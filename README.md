# Netcatty Center

运维服务器组织中心。管理员在独立后台维护主机目录；之后 Netcatty 客户端用 **地址 + 密钥** 拉取这份目录。

当前仓库只包含服务端（Go + Gin）。Netcatty 客户端尚未改动。

## 能做什么

- 首次启动创建管理员账号
- 管理员增删改主机（连接元数据，不含密码 / 私钥）
- 签发、吊销客户端 API 密钥
- `GET /api/v1/catalog` 供客户端拉取

## 启动

需要 Go 1.24+。

```bash
cd D:\code\Netcatty-Center
go run ./cmd/center
```

浏览器打开 http://127.0.0.1:4780

数据文件默认写在 `data/center.sqlite`。

编译：

```bash
go test ./...
go build -o center.exe ./cmd/center
./center.exe
```

环境变量：

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `4780` | 监听端口 |
| `HOST` | `0.0.0.0` | 监听地址 |
| `DATA_DIR` | `./data` | SQLite 目录 |
| `NCC_PUBLIC_DIR` | `./public` | 管理后台静态文件 |
| `NCC_COOKIE_SECURE` | 未设置 | 设为 `1` 时管理后台 cookie 带 Secure（HTTPS） |

## 客户端约定（尚未接入）

配置两项：

1. **地址**：组织中心 URL，例如 `http://ops.example.com:4780`
2. **密钥**：后台「客户端密钥」里签发的 `ncc_...` 字符串

拉取目录：

```bash
curl -H "Authorization: Bearer ncc_你的密钥" http://127.0.0.1:4780/api/v1/catalog
```

返回示例：

```json
{
  "version": 1,
  "center": { "id": "...", "name": "Netcatty Center" },
  "generatedAt": 1710000000000,
  "hosts": [
    {
      "id": "...",
      "label": "prod-web-1",
      "hostname": "10.0.1.12",
      "port": 22,
      "username": "deploy",
      "group": "production/web",
      "tags": ["linux", "prod"],
      "os": "linux",
      "protocol": "ssh",
      "deviceType": "general",
      "notes": "",
      "updatedAt": 1710000000000
    }
  ]
}
```

健康检查（无需密钥）：`GET /api/v1/health`

## 目录结构

```
cmd/center/        入口
internal/config/   端口、数据目录
internal/store/    SQLite
internal/security/ 密码与 API 密钥
internal/httpapi/  Gin 路由（管理员会话 + catalog）
public/            管理后台页面
data/              运行时数据库（不入库）
```
