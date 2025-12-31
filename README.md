# 粘贴板

一个基于 [Sol](https://github.com/wantnotshould/sol) 的在线粘贴板工具

## 快速开始

```bash
git clone https://github.com/wantnotshould/clipboard.git

cd clipboard

# 直接运行（默认监听 8080 端口）
go run .

# 或指定端口
go run . -port 8080
```

## 管理员功能

**服务启动时会自动生成一个随机管理员密码（如果未设置环境变量），用于清空所有数据和重置计数器。**

### 设置固定密码（推荐生产环境）

```bash
CLIPBOARD_PASSWORD=your_password go run .
```

### 重置所有数据

向 `/admin/reset` 发送 POST 请求，带表单参数 `pass=your_password` 即可清空所有文本和计数。

```bash
curl -X POST -d "pass=your_password" http://localhost:8080/admin/reset
```

## 安全声明

- 本服务不适合传输极高敏感度信息（如银行卡完整信息、大额转账指令等）
- 请务必核对链接发送者身份，谨防钓鱼诈骗
- 开发者对用户传输内容及后果不承担任何责任

## 许可证

MIT License

欢迎 Star、Fork、贡献代码或提出建议！