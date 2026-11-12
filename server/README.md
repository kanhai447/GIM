# GIM Server

`server/` 是 GIM 的独立 Go 后端，不 import/编译 `reference/` 中的任何代码。

## 目录

```text
cmd/         11 个进程入口（服务 + migrate/admin-bootstrap 工具）
internal/    GIM 自有实现（domain/service/repository/transport/platform）
api/         生成的 gRPC 契约（当前仅 user）
migrations/  MySQL 版本化迁移（001-008，含 up/down）
tests/       真实依赖集成测试（GIM_ENV_FILE 门控）
tools/       负载工具
```

## 进程入口

| cmd | 作用 |
|---|---|
| `gateway` | 唯一公网 HTTP/WS 入口：调用 Auth、清洗/注入身份、反向代理 |
| `auth-api` | 注册 / 登录 / 注销 / authentication |
| `user-api` | 用户资料、好友闭环、Admin 用户管理 |
| `user-rpc` | 内部 User gRPC（Auth/Chat/Group 调用） |
| `chat-api` | 私聊 HTTP、Chat WS、Presence、Redis 实时投递 |
| `group-api` | 群 HTTP、群消息/session、Group WS |
| `file-api` | 流式上传、内容寻址存储、鉴权下载/删除 |
| `settings-api` | 公开配置、Admin 配置与只读概览 |
| `logs-api` | Kafka consumer 与 Admin 审计查询 |
| `migrate` | 一次性 migration（`up` / `down -steps N` / `status`） |
| `admin-bootstrap` | 一次性创建首个管理员（从环境变量读取，不写死密码） |

V1 已实现：文本私聊/群聊、好友闭环、文件所有权、Admin 管理、bounded Kafka 审计。富消息、好友上线提醒、`user_confs` 编辑、delivery/read receipt 为 V2。

## 依赖与配置

需要 MySQL、Redis、etcd；Kafka 可选（未配置时审计 producer/consumer 自动禁用，核心 IM 不受影响）。
运行配置来自被 Git 忽略的根 `.env.local`；仓库只提交 `.env.example` 中的 `CHANGE_ME` 占位。

## 启动顺序

```powershell
cd server
$env:GIM_ENV_FILE = '..\.env.local'

go run ./cmd/migrate -env ..\.env.local -command up
go run ./cmd/user-rpc
go run ./cmd/user-api
go run ./cmd/auth-api
go run ./cmd/chat-api
go run ./cmd/group-api
go run ./cmd/file-api
go run ./cmd/settings-api
go run ./cmd/logs-api
go run ./cmd/gateway
```

- `user-api`/`auth-api`/`chat-api`/`group-api`/`file-api`/`settings-api`/`logs-api` 在 etcd `/gim/services/{service}/{instance}` 注册 lease，优雅退出时 revoke。
- Auth/Chat/Group 通过 `INTERNAL_API_HOST:USER_RPC_PORT` 调用 User RPC；RPC 是内部配置依赖，不经 Gateway 发现。
- 正式部署只暴露 Gateway；内部 API/RPC 不得直接映射公网，否则可绕过 Gateway 伪造身份头。

## 测试

```powershell
go build ./...
go vet ./...
gofmt -l .
go test ./... -count=1

# 真实依赖集成测试（MySQL/Redis/etcd；Kafka 用例需 GIM_KAFKA_INTEGRATION=1）
$env:GIM_ENV_FILE = '<absolute path to env file>'
go test ./tests -count=1 -v

# Linux/64-bit C 工具链下的 race
go test -race ./... -count=1
```

当前验证状态见 `docs/validation-status.md`；未修复问题见 `docs/known-issues.md`。

## 网络边界与安全

- Gateway 删除客户端提供的 `User-ID`、`Role`、`ValidPath` 与 `X-Request-ID`，调用 Auth 后只注入 Auth 返回的身份。
- Admin 接口在目标服务再次校验 `Role == 1`；前端路由不是安全边界。
- 日志不记录密码、JWT、Authorization、Cookie、Secret、消息正文或物理存储路径。
- 运行时秘密只来自环境变量/被忽略的 env 文件，不进入仓库与 Settings 表。

## 数据库

- 001–008 版本化 migration，禁止 AutoMigrate 改生产结构。
- `admin_deleted_at` 为管理端 tombstone；不物理删除消息行。
- 表/索引/约束清单见 `docs/04_DATABASE_DESIGN.md` §7.11。
