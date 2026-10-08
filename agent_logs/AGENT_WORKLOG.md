# GIM Agent Worklog

> 由 Agent 追加维护，禁止覆盖历史。

## 2026-10-07 Day 1 / Phase 0 — Checkpoint 1 独立工程骨架

### 目标

- 建立 GIM 自有 `server/`、`web/`、`admin/` 边界和可重复仓库安全检查。

### 开始状态

- branch: `main`
- HEAD: `c1dc8d47a0e294b66bd7097227b2b0e5fd09540f`
- working tree: 存在此前已生成但未提交的 `agent_logs/operations/pre-day1/`。

### 修改

- 新建独立 Go module、最小可编译 package、Server/Web/Admin 空骨架。
- 新建不回显 Secret 的仓库安全检查脚本。
- 新建 `agent_logs/CURRENT_STATE.md`。

### 数据/协议变化

- DB: 无。
- HTTP: 无。
- WebSocket: 无。
- Redis: 无。

### 测试

- repository safety script -> PASS。
- `gofmt -l .` -> PASS。
- `go test ./...` -> PASS。
- `go vet ./...` -> PASS。
- reference dependency scan -> PASS。

### Git

- commit: `81340911fe58df2ccaee3e3fc540b6c6c7cbf662 chore(day1): establish independent project skeleton`。
- push: `origin/main` -> SUCCESS。
- tag: 无。

### 遗留与下一步

- 完成 Checkpoint 1 commit/push 后实施 Checkpoint 2 公共基础模块。

## 2026-10-07 Day 1 / Phase 0.5 — Checkpoint 2A 公共 HTTP 基础

### 目标与修改

- 自主实现 dotenv 配置读取、统一应用错误和 V1 HTTP 响应。
- 未新增业务路由、数据库访问或 WebSocket 实现。

### 参考与调整

- 保留 FIM `{code,msg,data}` 与业务 HTTP 200 兼容行为。
- 不沿用直接返回 `err.Error()`；未知错误统一隐藏内部细节。

### 测试

- `go test ./...` -> PASS。
- `go test -cover ./internal/platform/...` -> PASS。
- `go vet ./...`、`gofmt -l .`、安全脚本 -> PASS。
- `go test -race ...` -> NOT AVAILABLE，本机 cgo C 编译器不支持 64 位模式。

### Git

- commit: `e6811c9df4248c3af9e84b905da6bdb498958e2a feat(server): add safe config and HTTP foundations`。
- push: `origin/main` -> SUCCESS。

### 下一步

- 完成 Checkpoint 2A commit/push，然后实施 Checkpoint 2B 基础设施客户端。

## 2026-10-07 Day 1 / Phase 0.5 — Checkpoint 2B 基础设施客户端

### 目标与修改

- 新增可注入的 MySQL/GORM、Redis、etcd 客户端与 context 健康检查、Close 生命周期。
- 新增真实本地基础设施集成测试；未执行 migration 或业务数据写入。

### 参考与调整

- 参考 FIM 基础连接需求和 ServiceContext 依赖关系。
- 不沿用 panic、无效 timeout context、反复创建且不关闭 etcd client 的方式。

### 测试

- `go test ./...`、cover、vet、gofmt、安全脚本 -> PASS。
- MySQL connect/Ping/错误凭据脱敏/Close -> PASS。
- Redis Ping/Close -> PASS。
- etcd Status/Close -> PASS。
- race -> NOT RUN，本机 cgo 64 位工具链不可用。

### Git

- commit: `b255dd3221a7fc83f15aeb3a15cfaf70c7257b93 feat(platform): add mysql redis and etcd clients`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 2B 已完成并推送；停止，下一次进入 Checkpoint 3。

## 2026-10-07 Day 1 / Phase 0.5 — Checkpoint 3 User API / RPC 基础能力

### 目标与修改

- 新增 User Domain、GORM Repository、共享 Service、go-zero HTTP Handler 与 gRPC Adapter。
- RPC 提供 CreateUser、GetUserByID、GetUserByAccount；公开 UserInfo 不含 password hash。
- 同步 users account/status 模型约束和 HTTP 错误码文档；未执行 migration。

### 参考与调整

- 参考 FIM User 字段语义、`/api/user/user_info` 路径及 User RPC 服务调用关系。
- GIM 不在 User 内 Hash 密码，不返回整模型 JSON bytes，不暴露数据库原始错误；HTTP/RPC 共用 Service。

### 测试

- `go test ./...`、User cover、vet、gofmt、安全脚本 -> PASS。
- Handler 公开字段与内存 gRPC 生成客户端调用 -> PASS。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。

### Git

- commit: `3aa0150ffbeecadbcc872344c70b67cf24a0ce0a feat(user): add user api rpc and domain foundation`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 3 已完成并推送；停止，下一次进入 Checkpoint 4 Auth。

## 2026-10-08 Day 1 / Phase 0.5 — Checkpoint 4 Auth 服务

### 目标与修改

- 自主实现 Auth 注册、登录、bcrypt、JWT、authentication、Logout 与 Redis blacklist。
- 新增 User RPC 适配、统一 Redis key helper、四个 HTTP Handler 和真实 Redis TTL 集成测试。
- 未实现 Gateway、第三方登录、migration 或任何 Day 2 内容。

### 参考与调整

- 参考 FIM Auth 路由、`Token`/`ValidPath`、User/Settings RPC 边界和 Logout TTL 语义。
- GIM 改为 Auth 侧强哈希、User RPC 登录查询、精确公开路径、`jti/iat/exp` Claims、SHA-256 fingerprint key、调用方 context 传播和安全错误映射。

### 测试

- `go test ./...`、Auth cover、vet、gofmt、安全脚本、diff check -> PASS。
- Auth + Redis key helper 汇总 coverage：72.8% statements。
- 真实本地 Redis blacklist 写入、查询与 TTL -> PASS。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。
- 额外基础设施复核：MySQL/Redis PASS；本机 etcd 未运行，因此既有 etcd health test 连接被拒绝。

### Git

- commit: `519282dbdfdbead9efeb556e6013dd14104d1c08 feat(auth): add registration jwt authentication and logout`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 4 已完成并推送；下一 Checkpoint 为 Gateway，不在本轮实施。

## 2026-10-08 Day 1 / Phase 0.5 — Checkpoint 5 Gateway 鉴权代理

### 目标与修改

- 自主实现 Gateway 显式路由、etcd 服务发现/lease 注册、独立 Auth Client、可信身份 Header 注入和 HTTP Reverse Proxy。
- 增加 Gateway 可执行入口与真实 etcd 下 Gateway/Auth/User 注册、登录、认证、资料访问、注销集成测试。
- 未实现 Chat/Group WebSocket、Hub、Presence、ACK、File、Kafka、Docker 或 Day 2 内容。

### 参考与调整

- 参考 FIM `/api/{service}/...`、etcd 查询、Auth authentication、身份 Header 和 ReverseProxy 业务语义。
- GIM 改为显式服务表、可扩展多端点 Resolver、lease/keepalive 注册、调用方 context/timeout、身份 Header 先删除后可信注入、安全错误映射和单一 Auth allowlist。

### 测试

- `go test ./...`、Gateway cover、vet、gofmt、安全脚本、diff check -> PASS。
- Gateway 相关汇总 coverage（含集成测试）：79.5% statements。
- 真实 etcd Gateway -> Auth -> User 注册/登录/认证/User API/Logout，以及身份 Header 防伪造 -> PASS。
- etcd health/status 与测试注册清理 -> PASS。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。

### Git

- commit: `eb9eebc6bfe4ee41c388d6eb6297cbfca43316bf feat(gateway): add auth-aware service proxy`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 5 已完成并推送；下一次进入 Checkpoint 6，本轮不实施。
