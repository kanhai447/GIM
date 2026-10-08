# Day 1 / Checkpoint 4 — Auth 注册、登录、JWT、Logout

- **日期:** 2026-10-08
- **Phase:** Phase 0.5
- **目标:** 自主实现可供后续 Gateway 调用的 Auth 注册、登录、JWT、authentication 与 Logout 基础能力；本 Checkpoint 不实现 Gateway。

## FIM Auth 参考内容

- 参考 FIM 的五个 Auth 路由语义，其中本次实现注册、登录、authentication 与 Logout；第三方登录仍不进入 V1 本 Checkpoint。
- 参考 FIM authentication 使用 `Token` / `ValidPath` Header，并向 Gateway 返回用户 ID 与角色的接口语义。
- 参考 FIM 注册调用 User RPC、第三方登录读取 Settings RPC 的服务边界；本次普通注册/登录只依赖 User RPC，Settings RPC 边界留给未实现的第三方登录。
- 参考 FIM 注销状态保存到 Redis 且 TTL 等于凭证剩余寿命的行为。
- 未复制 FIM 文件、目录或实现，未读取或复用 FIM 配置值，`reference/` 不参与 GIM 编译。

## GIM 自主实现与改进

- Auth 使用 `HTTP -> Auth Service -> User RPC / Redis` 分层，并以显式依赖注入组装。
- 注册由 Auth 完成 bcrypt 强哈希，User RPC 只接收哈希结果；响应只含公开用户资料。
- 登录统一“账号或口令错误”外部提示，区分用户禁用；内部 User RPC cause 不进入响应。
- JWT 固定 HS256，Claims 包含 `userID`、`role`、`jti`、`iat`、`exp`，并验证算法、签名、格式与自然过期时间。
- 签名材料与有效期只从本地安全配置加载，签名材料长度不足时拒绝启动；相关值在本日志中记为 `[REDACTED]`。
- authentication 对公开路径使用精确集合匹配，允许 query 但不使用正则或 substring；受保护路径校验签名、过期与注销状态。
- Logout 对原始凭证计算 SHA-256 fingerprint，通过公共 Redis Key Helper 生成 `gim:auth:logout:{tokenHash}`，值为 `1`，TTL 为 JWT 剩余寿命；Redis 不保存原始凭证。
- 所有 User RPC 与 Redis 调用都传递调用方 `context.Context`；生产 Auth 路径不存在无意义的 `context.Background()`。
- 相比 FIM，未直接查询 Auth 自有用户表、未使用 bcrypt 最低成本、未用正则白名单、未把原始凭证拼入 Redis key、未记录口令或完整认证材料，也未使用请求间共享的可变日志对象。

## 模块结构与修改文件

- `server/internal/auth/config/`：Auth 安全配置解析与测试。
- `server/internal/auth/credential/`：bcrypt 哈希/校验与测试。
- `server/internal/auth/token/`：JWT 签发、解析、Claims 与错误分类测试。
- `server/internal/auth/userclient/`：User gRPC 适配、稳定错误映射与 context 测试。
- `server/internal/auth/revocation/`：SHA-256 fingerprint 与 Redis 注销存储。
- `server/internal/auth/service/`：注册、登录、authentication、Logout、精确白名单及业务测试。
- `server/internal/auth/transport/http/`：四个 Auth HTTP Handler、兼容字段/Header 与响应安全测试。
- `server/internal/auth/module.go`、`module_test.go`：显式模块组装与安全配置测试。
- `server/internal/platform/rediskeys/`：统一 Auth 注销 key helper 与测试。
- `server/tests/auth_redis_integration_test.go`：本地真实 Redis key、存在性与 TTL 集成测试。
- `server/go.mod`、`server/go.sum`：JWT 与 bcrypt 直接依赖。
- `docs/05_HTTP_API.md`、`docs/08_REDIS_CONFIG.md`：同步 Auth 请求/响应、错误码、白名单和注销 key/TTL 契约。
- `agent_logs/CURRENT_STATE.md`、`agent_logs/day-1.md`、`agent_logs/AGENT_WORKLOG.md`：Checkpoint 状态与测试事实。

## 协议与数据变化

- DB：无 migration、无表结构变化、无业务数据写入。
- HTTP：实现 `POST /api/auth/register`、`POST /api/auth/login`、`POST /api/auth/authentication`、`POST /api/auth/logout`。
- 注册：`{account,nickname,pwd,rePwd}`，返回公开用户资料；不返回凭据哈希。
- 登录：首选 `{account,password}`，兼容既有 `{userName,password}`，返回 token 与公开用户资料。
- authentication：兼容 `Token` / `ValidPath`，返回 `userID/role/authenticated/public`。
- Redis：新增统一 `gim:auth:logout:{tokenHash}` 契约，TTL 为剩余有效时间。
- WebSocket：无变化。

## 测试命令与结果

- `go test ./...` -> PASS。
- `go test -cover ./internal/auth/... ./internal/platform/rediskeys` -> PASS。
- 汇总 Auth + Redis key helper coverage：**72.8% statements**。
- 关键包：Auth Service 76.3%，JWT 75.7%，credential 83.3%，revocation 91.7%，Redis key helper 100.0%。
- `go vet ./...` -> PASS。
- 全仓 `gofmt -l` -> PASS（无输出）。
- `GIM_ENV_FILE=[REDACTED] go test ./tests -run TestAuthRedisBlacklistTTL -count=1 -v` -> PASS；真实本地 Redis 的写入、查询、TTL 与清理均通过。
- repository safety script -> PASS；本地配置、`reference/`、密钥类文件仍未跟踪，签名材料只显示 `[REDACTED]`。
- `git diff --check` -> PASS。
- `go test -race ./internal/auth/... ./internal/platform/rediskeys` -> **NOT RUN - environment limitation**：本机 cgo C 编译器不支持 64 位模式。

### 覆盖的验收场景

- 正常注册、重复账号、bcrypt 结果不同于明文、公开响应不含哈希/口令材料、Handler 错误不泄露私有 cause。
- 正确登录、错误口令、账号不存在、禁用用户、JWT Claims。
- 正常 JWT、malformed、错误签名、错误算法、过期拒绝。
- Logout 前有效、Logout 后拒绝、剩余 TTL、fingerprint key 不含原始凭证。
- 公共路径精确匹配、受保护路径缺少 token 拒绝、有效 token 返回 userID/role。
- User RPC 与 Redis context 传播、Redis 内部错误安全映射。

## 问题、根因与处理

1. 首次 `go mod tidy -compat=1.25.0` 被 PowerShell 将版本参数当数值处理，依赖校验和未生成。
   - 处理：改用显式字符串参数重新执行 tidy；随后全量测试通过。
2. 首次真实 Redis 测试使用相对本地配置路径；Go 测试包运行目录导致文件未找到。
   - 处理：改用已解析的仓库内路径重新运行；Auth Redis 测试通过，未回显配置内容。
3. 同次额外基础设施复核中 MySQL/Redis 通过，但本机 etcd 当前未启动，既有 etcd 健康检查连接被拒绝。
   - 处理：Auth 不依赖 etcd 本地进程完成本 Checkpoint；未在 Checkpoint 4 扩展范围修复基础设施。
4. 首次汇总 coverage 命令的 PowerShell 参数传递未生成 profile。
   - 处理：使用明确的忽略目录 profile 路径重跑，得到 72.8% 汇总结果。
5. race 仍被本机 64 位 cgo 工具链阻断。
   - 处理：按要求记录 NOT RUN，不在本 Checkpoint 更换工具链。

## Git

- **Commit hash:** `519282dbdfdbead9efeb556e6013dd14104d1c08`。
- **Push:** SUCCESS — `origin/main`。

## 下一步

- Checkpoint 4 已完成测试、Secret Scan、commit 和 push；在稳定节点停止。
- 下一 Checkpoint：Checkpoint 5 — Gateway 与 Auth 鉴权链路；本次未实现 Gateway。
