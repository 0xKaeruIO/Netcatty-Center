# Netcatty Center

运维服务器组织中心。管理员在独立后台维护主机目录；之后 Netcatty 客户端用 **地址 + 密钥** 拉取这份目录。

服务端在本仓库（Go + Gin）。Netcatty 客户端同步 catalog 时会把 `groups` 和主机 `group` 挂到中心名称下，保留同一套目录树。

## 能做什么

- 用启动参数或环境变量指定管理员账号（不写入数据库）
- 管理员增删改主机（含登录密码、密钥、启动命令 / Expect-Send 规则，以及目录可见范围）
- 主机按 `group` 路径（如 `production/web`）做树状目录展示，支持展开全部 / 折叠全部；空分组也会随 catalog 的 `groups` 下发
- 可从 JSON 导入主机（含密码、私钥、启动命令 / Expect-Send 规则、分组等后台可配字段），示例见 `examples/hosts-import.json`
- 可将当前主机目录导出为同样的明文 JSON（含密码、私钥），可再导回后台
- 签发、吊销、重新启用或彻底删除客户端 API 密钥；密钥分 **只读**（默认）和 **可读可写**。只读可拉取机器列表并开启/加入/关闭分享，不能改机器列表
- `GET /api/v1/catalog` 按密钥可见范围下发主机

## 启动

需要 Go 1.24+。

```bash
cd D:\code\Netcatty-Center
go run ./cmd/center --admin-user admin --admin-password change-me
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
| `NCC_ADMIN_USER` | 未设置 | 管理员用户名（不入库；也可 `--admin-user`） |
| `NCC_ADMIN_PASSWORD` | 未设置 | 管理员密码（不入库；也可 `--admin-password`） |

修改管理员账号：改启动参数或环境变量后重启进程，不必改数据库。两个值必须一起提供。

未提供启动参数时，仍可走首次初始化，把管理员写进 SQLite（兼容旧部署）。

## 客户端约定

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
  "groups": ["production", "production/web"],
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
      "password": "",
      "privateKey": "",
      "passphrase": "",
      "startupCommand": "tmux attach || tmux",
      "startupCommandRunMode": "paste",
      "startupCommandRules": [],
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
examples/          主机 JSON 导入示例
data/              运行时数据库（不入库）
```
