# Day 1 / Checkpoint 6 — Core Integration Acceptance

- **日期:** 2026-10-08
- **Phase:** Phase 0.5
- **目标:** 对既有 Platform/User/Auth/Gateway/MySQL/Redis/etcd 核心基础做真实回归、异常路径、安全与资源审计；不新增业务，不进入 Checkpoint 7 或 Day 2。

## 开始状态

- Branch：`main`。
- HEAD：`78d436e4d427e085b7a20f6db3bae1f47f67b7c5`，与 `origin/main` 同步。
- Working tree：clean。
- Checkpoint 5：已完成并推送。

## 验收范围

- 实际回归 Register/Login/JWT/Gateway/Auth/User 主链路。
- 分别验证缺失、malformed、错误签名、过期、注销凭证以及 public/protected path。
- 复核外部 `User-ID`、`Role`、`ValidPath` 删除与可信身份注入。
- 复核 etcd 注册、lease、keepalive、cleanup、服务不存在、lookup timeout 与 upstream 不可用。
- 审计请求 context、timeout、HTTP body、数据库/缓存/discovery/RPC/lease 资源关闭。
- 复核公开错误与配置/Secret 安全。

## 初始审计发现

1. 现有真实 Gateway/Auth/User 测试覆盖缺失凭证、通用非法凭证、Logout 与身份防伪造，但未在同一真实链路中分别断言错误签名和过期错误码。
   - 计划：只扩展现有集成测试，不新增业务能力。
2. discovery `Registration` 已维护 keepalive goroutine 的 `done` channel，但 `Close` 当前取消并 revoke 后未等待 goroutine 退出。
   - 计划：在调用方 context 边界内等待退出，并以真实 etcd lease/keepalive/cleanup 回归验证。
3. 生产请求路径中的 `context.Background()` 仅出现在 Gateway 进程启动、信号生命周期与 shutdown 根 context；请求链继续传播 `request.Context()`。其余命中均为测试根 context。

## FIM 参考边界

- 本 Checkpoint 不新增业务，也未再次读取或复制 FIM 实现；只验收 Checkpoint 1–5 已自主实现的 GIM 能力。
- 继续保持既有兼容语义：Gateway 调 Auth authentication、`Token` / `ValidPath`、可信 `User-ID` / `Role` 注入。
- `reference/` 仍只读、被忽略且不参与构建。

## 主链路与 Authentication 回归

- 真实本地 etcd 注册 `auth_api` / `user_api`，Gateway 通过 discovery 找到服务。
- Public：无凭证 Register -> Auth -> User RPC 成功；无凭证 Login 成功并签发 JWT。
- Protected：JWT -> Gateway -> Auth `/authentication` -> userID/role -> Gateway -> User API，公开资料返回成功。
- Auth 异常：缺少凭证 -> `1203`；malformed -> `1204`；错误签名 -> `1204`；过期 -> `1205`；Logout 后复用 -> `1206`，全部经真实 Gateway/Auth 链路通过。
- Gateway 没有 JWT 库依赖或本地 Parse 调用；所有合法 API 仍通过独立 Auth Client 请求 Auth。

## Header 安全回归

- 客户端同时伪造 `User-ID: 1`、`Role: 1`、`ValidPath: /api/auth/login`。
- Gateway 在认证前删除三者，Auth 收到的 ValidPath 来自实际 URL，业务上游收到 Auth 返回的普通用户 ID/role=2，且不收到外部 ValidPath。
- 管理员越权未发生，回归 PASS。

## Discovery 与异常回归

- `auth_api` / `user_api` 注册、发现：PASS。
- 新增真实 2 秒 lease 验收；等待 3 秒跨过自然 TTL 后 key 仍存在，证明 keepalive 生效：PASS。
- `Registration.Close` 取消 keepalive、等待 goroutine 退出并 revoke；关闭后 key 立即消失：PASS。
- 未注册 service：统一 `1303`；lookup timeout：统一 Gateway timeout；真实注册但不可连接的 upstream：统一 `1305`/502。
- 客户端响应不包含测试 endpoint 或 etcd cause；全部测试后 `/gim/services/` 无残留。

## Context / Timeout 审计

- HTTP Handler 使用 `request.Context()`；Gateway -> Auth 使用派生 timeout 和 `NewRequestWithContext`；Gateway -> Upstream 从原请求 context 克隆 timeout。
- User HTTP -> Service、Auth -> User RPC、User Repository `GORM.WithContext`、Redis、MySQL、etcd 均使用调用方 context。
- 生产 `context.Background()` 仅用于 Gateway 进程 startup/signal/shutdown 根生命周期，未覆盖请求 cancellation/deadline，判定合理。
- registry keepalive 使用显式服务生命周期 context，cleanup 使用有界 context。

## 资源关闭审计与修复

- Auth Client 对 HTTP response body 使用 `defer Close`；ReverseProxy 由标准库管理上游 body；集成请求 body 全部关闭。
- MySQL/Redis/etcd 客户端均有显式 Close，初始化失败会回收已创建资源；RPC connection/server/listener 均在 cleanup 关闭。
- 发现并修复：`Registration.Close` 原先取消 keepalive 并 revoke，但未等待已跟踪的 goroutine 退出。现于调用方 context 内等待 `done`，超时返回安全 discovery error。
- 真实 lease 生命周期测试覆盖 keepalive、Close 等待路径、revoke 与无残留。

## 错误与配置安全

- 统一响应层对未知错误返回公开 500；Auth/Gateway/discovery/proxy 只返回稳定业务错误，内部 cause 可 unwrap 但不进入 HTTP body。
- 未发现 DSN、SQL/Redis/etcd 原始错误、JWT Secret、完整 JWT、Go stack 或上游 endpoint 对客户端泄露。
- 生产代码无硬编码 HTTP 上游 URL；服务 endpoint 由本地配置和 etcd 注册提供。
- `.env.local`、`reference/`、密钥文件均未跟踪；`.env.example` 仅含 `CHANGE_ME` 与 localhost 开发占位值。
- repository safety/Secret Scan：PASS；未发现 FIM 生产地址或 Secret 进入 GIM 跟踪文件。

## 修改文件

- `server/internal/platform/discovery/registry.go`：Close 在有界 context 内等待 keepalive goroutine 退出。
- `server/tests/gateway_auth_user_integration_test.go`：补充错误签名、过期、ValidPath 伪造、真实不可用 upstream、lease/keepalive/cleanup 回归。
- `agent_logs/operations/day-1/007_checkpoint-6_core-integration-acceptance.md`：本验收记录。
- `agent_logs/CURRENT_STATE.md`、`agent_logs/day-1.md`、`agent_logs/AGENT_WORKLOG.md`：同步状态。

## 测试命令与结果

- `go test ./... -count=1` -> PASS。
- `GIM_ENV_FILE=[REDACTED] go test ./tests -run 'Test(GatewayAuthUserChain|DiscoveryLeaseKeepAliveCleanup|AuthRedisBlacklistTTL|LocalInfrastructure)' -count=1 -v` -> PASS。
- `go test -cover ./internal/auth/... ./internal/user/... ./internal/gateway/... ./internal/platform/...` -> PASS。
- 真实集成参与的 Day 1 核心汇总 coverage：**75.7% statements**。
- 关键包：Auth Service 76.3%，JWT 75.7%，User Service 77.5%，Gateway 72.7%，Auth Client 83.1%，Proxy 89.3%，Route 100.0%；discovery 独立单测 46.1%。
- `go vet ./...` -> PASS。
- 全仓 `gofmt -l` -> PASS（无输出）。
- `git diff --check` -> PASS。
- repository safety/Secret Scan -> PASS。
- etcd 3.6.10 health/status -> PASS，单节点 leader，无 endpoint error；服务 key cleanup -> PASS。
- `go test -race ./internal/auth/... ./internal/user/... ./internal/gateway/... ./internal/platform/discovery` -> **NOT RUN - environment limitation**：本机 cgo C 编译器不支持 64 位模式。

## 问题与处理

1. Authentication 的错误签名、过期凭证此前仅有 Auth/JWT 单元测试，没有真实 Gateway/Auth 回归证据。
   - 处理：使用测试专用材料签发错误签名和已过期 JWT，经真实链路分别断言 `1204` / `1205`，不记录完整值。
2. `Registration.Close` 未等待 keepalive goroutine 退出。
   - 处理：在调用方 deadline 内等待 `done`；真实 etcd 回归证明 keepalive 与 cleanup 正常。
3. race 仍被本机 64 位 cgo 工具链阻断。
   - 处理：按要求记录 NOT RUN，不在本 Checkpoint 修改工具链。

## Git

- **Commit:** `af53a76a1217918fb2f2008b068ff451908bbd95`。
- **Push:** SUCCESS — `origin/main`。

## 下一步

- Checkpoint 6 验收、commit 与 push 已完成；在稳定节点停止。
- 下一 Checkpoint：Checkpoint 7 — Day 1 数据库底座与 migration；本次不进入。
