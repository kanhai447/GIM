# GIM Current State

- **Current Day:** Day 1
- **Current Phase:** Phase 0.5
- **Current Checkpoint:** Checkpoint 4 — Auth 注册、登录、JWT、Logout（COMPLETED，等待 commit/push）
- **Last Completed Checkpoint:** Checkpoint 3 — User API / RPC 基础能力
- **Last Stable Commit:** `3aa0150ffbeecadbcc872344c70b67cf24a0ce0a`
- **Last Push Status:** SUCCESS — Checkpoint 3 pushed to `origin/main`

## Completed

- Pre-Day1 Git/GitHub、安全配置与本地基础设施验证。
- `reference/`、`.env.local`、密钥、本地数据和构建产物忽略规则验证。
- MySQL、Redis、etcd、Go、Node/pnpm 环境验证。
- GIM 独立 Go module 与 `server/`、`web/`、`admin/` 空骨架。
- 可重复执行且不回显 Secret 的仓库安全检查脚本。
- 安全 dotenv 配置读取、统一应用错误与 V1 HTTP 响应基础模块。
- 可注入的 MySQL/GORM、Redis、etcd 客户端及真实本地连接测试。
- User Domain、GORM Repository、共享 Service、公开资料 HTTP Handler 与内部 gRPC 基础契约。
- Auth 注册/登录、bcrypt、JWT Claims/验证、精确公开路径、authentication、Logout Redis blacklist 与 User RPC 适配。

## In Progress

- Checkpoint 4：staged diff、Secret Scan、commit 与 push 交付。

## Not Started

- Checkpoint 5：Gateway 与 Auth 鉴权链路。
- Checkpoint 6：Gateway -> Auth -> User 集成测试。
- Checkpoint 7：Day 1 数据库底座与 migration。
- Checkpoint 8：Web/Admin 基础工程与 build。
- Checkpoint 9：Day 1 全量验收。

## Test Status

- Pre-Day1 Verification：PASS。
- Checkpoint 1：安全脚本、`gofmt`、`go test ./...`、`go vet ./...`、reference 零依赖与目录边界检查均 PASS。
- Checkpoint 2A：普通单测、覆盖率、vet、gofmt 和安全脚本 PASS；race 因本机 cgo 64 位编译器不可用而未能执行。
- Checkpoint 2B：全量单测、覆盖率、vet、gofmt、安全脚本和 MySQL/Redis/etcd 真实集成测试 PASS；race 仍为 NOT RUN。
- Checkpoint 3：全量单测、User 覆盖率、vet、gofmt、安全脚本、HTTP Handler 与内存 gRPC 集成测试 PASS；race 因相同环境限制为 NOT RUN。
- Checkpoint 4：全量单测、Auth 72.8% 汇总覆盖率、vet、gofmt、安全脚本、真实 Redis blacklist TTL 集成测试 PASS；race 因相同环境限制为 NOT RUN。

## Known Issues

- `protoc 3.9.0` 较旧，后续首次生成 RPC 代码时需要验证与当前 Go 插件兼容性。
- 本机 cgo C 编译器不支持 64 位 race 构建；Day 2 并发验收前需要准备兼容环境。
- users 正式 migration 尚未创建；按 Day 1 边界留在 Checkpoint 7，当前 Repository 只经过单元测试。
- 本机 etcd 当前未运行；额外基础设施复核中的 MySQL/Redis 通过、etcd 健康检查连接被拒绝。Auth Checkpoint 不依赖 etcd 进程。

## Next Action

- 完成 Checkpoint 4 staged Secret Scan、commit 与 push 后停止；下一步为 Checkpoint 5 Gateway，但本轮不得进入。
