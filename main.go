// Package main
// Copyright 2025 wantnotshould. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.
package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/wantnotshould/sol"
)

// 配置常量
const (
	maxTexts         = 1000
	maxContentLength = 1000
	textLifetime     = time.Minute
)

// 数据结构
type entry struct {
	content   string
	createdAt time.Time
	used      bool
}

type textStore struct {
	sync.RWMutex
	data map[string]entry
}

var store = textStore{
	data: make(map[string]entry),
}

var (
	createCount uint64
	viewCount   uint64
)

// 计数操作
func incrementCreate() { atomic.AddUint64(&createCount, 1) }
func incrementView()   { atomic.AddUint64(&viewCount, 1) }
func countStats() (create, view uint64) {
	return atomic.LoadUint64(&createCount), atomic.LoadUint64(&viewCount)
}

// 模板文件嵌入
//
//go:embed templates/*.html
var templateFiles embed.FS
var templates = template.Must(template.ParseFS(templateFiles, "templates/*.html"))

var adminPassword string

// 初始化配置
func init() {
	adminPassword = os.Getenv("CLIPBOARD_PASSWORD")
	if adminPassword == "" {
		adminPassword = uuid.NewString()[:16]
		log.Printf("WARNING: CLIPBOARD_PASSWORD not set! Using auto-generated password: %s\n", adminPassword)
		log.Println("Set environment variable CLIPBOARD_PASSWORD for production!")
	}
}

func renderNotFound(c *sol.Context) {
	c.Writer.WriteHeader(http.StatusNotFound)
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	err := templates.ExecuteTemplate(c.Writer, "notfound.html", nil)
	if err != nil {
		fallback := "404 - Not Found\nOops, this has already been seen or it's expired... gone forever! 😅"
		http.Error(c.Writer, fallback, http.StatusNotFound)
	}
}

func main() {
	sl := sol.New()

	//  已读/路由不存在
	sl.NotFound(func(c *sol.Context) {
		renderNotFound(c)
	})

	// 根路由，展示文本链接
	sl.GET("/", func(c *sol.Context) {
		showResult := false
		resultURL := ""

		if id := c.QueryParam("s"); id != "" {
			store.RLock()
			if e, ok := store.data[id]; ok && !e.used && time.Since(e.createdAt) < textLifetime {
				showResult = true
				scheme := c.Scheme()
				resultURL = fmt.Sprintf("%s://%s/t/%s", scheme, c.Host(), id)
			}
			store.RUnlock()
		}

		createCnt, viewCnt := countStats()

		err := templates.ExecuteTemplate(c.Writer, "index.html", struct {
			HasResult        bool
			URL              string
			CreateCount      uint64
			ViewCount        uint64
			Year             int
			MaxContentLength int
		}{
			HasResult:        showResult,
			URL:              resultURL,
			CreateCount:      createCnt,
			ViewCount:        viewCnt,
			Year:             time.Now().Year(),
			MaxContentLength: maxContentLength,
		})
		if err != nil {
			http.Error(c.Writer, "template render error", http.StatusInternalServerError)
		}
	})

	// 添加文本
	sl.POST("/add", func(c *sol.Context) {
		if err := c.Request.ParseForm(); err != nil {
			http.Error(c.Writer, "form error", http.StatusBadRequest)
			return
		}

		content := strings.TrimSpace(c.Request.FormValue("content"))
		if content == "" {
			http.Error(c.Writer, "content can't be null", http.StatusBadRequest)
			return
		}

		if len(content) > maxContentLength {
			http.Error(c.Writer, fmt.Sprintf("sorry, your text is too long! Keep it under %d characters", maxContentLength), http.StatusBadRequest)
			return
		}

		currentCreate := atomic.LoadUint64(&createCount)
		if currentCreate >= maxTexts {
			http.Error(c.Writer, fmt.Sprintf("sorry, the text limit of %d has been reached. No more texts can be added.", maxTexts), http.StatusForbidden)
			return
		}

		store.Lock()
		defer store.Unlock()

		if atomic.LoadUint64(&createCount) >= maxTexts {
			http.Error(c.Writer, fmt.Sprintf("sorry, the text limit of %d has been reached. No more texts can be added.", maxTexts), http.StatusForbidden)
			return
		}

		id := uuid.New().String()[:10]
		store.data[id] = entry{
			content:   content,
			createdAt: time.Now(),
			used:      false,
		}

		incrementCreate()

		http.Redirect(c.Writer, c.Request, "/?s="+id, http.StatusSeeOther)
	})

	// 查看文本
	sl.GET("/t/:id", func(c *sol.Context) {
		id := c.Param("id")
		if len(id) < 8 {
			renderNotFound(c)
			return
		}

		store.Lock()
		e, exists := store.data[id]
		if !exists || e.used || time.Since(e.createdAt) > textLifetime {
			store.Unlock()
			renderNotFound(c)
			return
		}

		content := e.content
		e.used = true
		store.data[id] = e
		delete(store.data, id)
		store.Unlock()

		incrementView()

		err := templates.ExecuteTemplate(c.Writer, "view.html", struct {
			Content string
		}{
			Content: content,
		})
		if err != nil {
			http.Error(c.Writer, "template render error", http.StatusInternalServerError)
		}
	})

	// 重置数据
	sl.POST("/admin/reset", func(c *sol.Context) {
		if err := c.Request.ParseForm(); err != nil {
			http.Error(c.Writer, "Bad request", http.StatusBadRequest)
			return
		}

		pass := c.Request.FormValue("pass")
		if pass != adminPassword {
			http.Error(c.Writer, "Forbidden", http.StatusForbidden)
			return
		}

		store.Lock()
		store.data = make(map[string]entry)
		store.Unlock()

		atomic.StoreUint64(&createCount, 0)
		atomic.StoreUint64(&viewCount, 0)

		c.Status(http.StatusOK)
		c.Writer.Write([]byte("Reset successful! All texts cleared, counters reset."))
	})

	// 启动服务器
	port := flag.String("port", "8080", "server port")
	flag.Parse()

	sl.Run(":" + *port)
}
