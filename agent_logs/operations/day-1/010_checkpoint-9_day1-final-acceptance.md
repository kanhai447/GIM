# Day 1 / Checkpoint 9 — Final Acceptance & Freeze

- **日期:** 2026-10-09
- **Phase:** Phase 1
- **状态:** COMPLETED
- **目标:** 最终验收 Day 1 Backend、Database、Web、Admin、安全配置和 Git 状态；只修复 Day 1 范围内必要问题，完成冻结，不进入 Day 2。

## 开始状态

- Branch：`main`。
- HEAD：`90fd194fdd26128ee684e6e59554d4cb41bc5c2f`。
- `HEAD == origin/main`：PASS（完成 `git fetch origin` 后确认）。
- Working tree：开始时 clean。
- `.env.local`、`reference/`：均未跟踪。

## Runtime validation / fixes

- 审计发现 Gateway 与 migration 已有正式命令，但 Auth API、User API、User RPC 只有模块和测试 wiring，不能满足真实多进程启动验收。
- 新增 `cmd/auth-api`、`cmd/user-api`、`cmd/user-rpc`：main 只负责配置、依赖组装、服务启动和资源生命周期，不包含业务逻辑。
- Auth API 复用现有 Auth module，连接配置指定的 User RPC，启动前执行 gRPC health check；使用 Redis blacklist，并将 HTTP endpoint 注册到 etcd。
- User API/RPC 共用现有 User module 与 MySQL Repository；API 注册到 etcd，RPC 使用内部配置地址和 gRPC health service。
- 新增公共 process 配置：环境文件、启动/停止 timeout、内部 RPC 地址与 go-zero REST 配置全部来自安全配置，无硬编码生产地址或凭据。
- Windows 上 go-zero 1.10.1 的 proc shutdown hook 是 no-op；首次 Ctrl+C 验收发现 API 进程被控制台直接终止，注册只能等待 TTL 清理。GIM 新增 context 驱动的底层 `http.Server.Shutdown`，有界等待并保留 go-zero REST 路由；复验四进程均 exit 0，key 立即 revoke。
- 本地忽略配置补充本轮开发端口、lease/timeout 和有效 token expiry；未写入 Git。
- Runtime commit：`67d1077e20cda66169a755cae9f36c7996c8a09c feat(runtime): complete day1 service startup wiring`，push SUCCESS。

## Multi-process smoke test

- 使用正式构建二进制分别启动：User RPC、User API、Auth API、Gateway；依赖真实本地 MySQL、Redis 和临时隔离 etcd。
- 真实 HTTP 链路：Register → Login → credential issued → Gateway → Auth authentication → User API → Logout → 相同旧 credential 再访问。
- 结果：注册/登录/资料均成功；Logout 后旧 credential 返回 HTTP 401 / code 1206。
- 伪造 `User-ID: 1`、`Role: 1`、`ValidPath`：Gateway 删除后由 Auth 注入真实普通用户 role 2，PASS。
- 无 credential：HTTP 401 / 1203；malformed：HTTP 401 / 1204。
- 不存在服务：HTTP 503 / 1303；注册不可用 upstream 后：HTTP 502 / 1305，响应不泄露 endpoint。
- Auth/User API lease 等待超过 TTL 后 key 仍存在，keepalive PASS；四进程 Ctrl+C 均 exit 0，退出后 service key 立即为 0，所有端口关闭。
- 临时 etcd 进程、数据目录、构建产物和 3 条 smoke test 用户数据已清理。

## Auth / Gateway security

- `TestGatewayAuthUserChain` 随全量测试实际覆盖 public register/login、protected User API、缺失、malformed、错误签名、过期、logout 后 credential、Header spoofing、服务不存在和不可用 upstream。
- Gateway 仍只转发 credential 给 Auth `/authentication`，自身不解析或验证签名。
- Auth 的 public allowlist 精确匹配；Logout Redis fingerprint + remaining TTL 机制无变化并通过真实 Redis 集成。

## Discovery / shutdown

- Auth API、User API：etcd lease 注册、keepalive、显式 revoke、goroutine cleanup PASS。
- User RPC：当前架构使用配置的内部地址，不经过 Gateway discovery；health service、graceful stop 和 connection close PASS。
- Gateway discovery、lookup timeout、service missing、upstream unavailable 的安全公共错误测试 PASS。
- HTTP、gRPC、MySQL、Redis、etcd client、lease/keepalive 均有显式关闭路径。

## Context / resource audit

- 请求链 HTTP → Gateway → Auth → User RPC → Repository → MySQL/Redis/etcd 继续传播调用方 context、deadline 和 cancellation。
- 生产代码中的 `context.Background()` 仅存在于进程 startup/lifecycle/signal/shutdown/migration，不用于替换业务请求 context。
- Gateway Auth HTTP response body 明确关闭；ReverseProxy 由标准库管理 upstream body；gRPC connection 和 listener 在进程退出时关闭。

## Database acceptance

- 六个 migration：001 identity、002 friend、003 chat、004 group、005 file、006 settings，顺序与 UP/DOWN 配对完整。
- 隔离 MySQL：EMPTY → UP → duplicate UP → DOWN ALL → dirty metadata fail-fast → clear → RE-UP，PASS；临时数据库自动删除。
- 16 张业务表、schema_migrations、9 类唯一约束、11 类 EXPLAIN、无数据库 FK、User/Auth Repository 回归 PASS。
- User Domain/GORM 与 users migration 对齐；生产 schema 不依赖 AutoMigrate。
- 已确认 chat/group `client_msg_id`、sessions、unread、last read/message、top，以及 file SHA-256/user references 字段存在；未实现相关 Day 2/3 业务。

## Backend tests

- `go test ./... -count=1`（真实本地配置）：PASS。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS（无输出）。
- 核心 package coverage（含真实 integration/migration）：**76.7% statements**。
- migration dirty protection 专项：PASS。

## Web acceptance

- `pnpm type-check`：PASS。
- `pnpm test`：PASS，4 files / 9 tests。
- `pnpm build`：PASS。
- lint：NOT CONFIGURED。
- Login/Register/Auth Store/session restore/Router Guard/User Info/Logout/Main/Profile 及 Chat/Group/File 占位入口均存在；未实现 WebSocket、Presence、ACK、reconnect 或文件业务。

## Admin acceptance

- `pnpm type-check`：PASS。
- `pnpm test`：PASS，5 files / 9 tests。
- `pnpm build`：PASS。
- lint：NOT CONFIGURED。
- Login/Forbidden/Auth Guard/Admin Layout/Dashboard/User/Chat/Group/File/Settings/Logs 骨架存在；Logout 使用正确路径。
- 未调用 Mock Dashboard endpoints，不使用 `v-html`；ECharts 按需加载测试 PASS。

## Protocol check

- Web/Admin 与后端统一 `{code,msg,data}`、Login/Register/User Info/Logout、`token` Header、`userID`/`role` 数字类型一致。
- 前端 contract tests 验证精确 URL、请求 DTO、response envelope 和 Header；真实多进程调用再次验证运行时契约。

## Secret Scan / reference independence

- repository safety script：PASS；`.env.local`、`reference/`、private key、node_modules、dist、local data 均未跟踪。
- 本地真实签名材料与数据库凭据只在内存中和候选 Git 文件比对，均无匹配；无 private key marker、完整 JWT literal、OAuth secret-like assignment。
- `.env.example` 只含 `CHANGE_ME`、localhost 与非敏感 timeout；无真实凭据。
- `server/`、`web/`、`admin/` 无 reference import/runtime read/symlink/module/package dependency；Go dependency list 无 reference/FIM 项。
- `reference/` 最新文件时间仍为原始 2025 时间，Git ignore/tracking 状态无变化；本 Checkpoint 没有写入参考目录。

## Race status

- `go test -race ./...`：**Race: NOT RUN - environment limitation**。
- 原因：当前 Windows cgo C compiler 报 `64-bit mode not compiled in`；按要求未在本 Checkpoint 修工具链。
- Day 2 涉及 WebSocket 并发前，此问题优先级提升，需准备兼容 64 位 race 环境。

## Problems / fixes

1. 缺少 Auth API/User API/User RPC 正式入口：新增最小 runtime wiring、health 和资源关闭。
2. Windows go-zero REST signal hook 不执行 graceful shutdown：增加跨平台 context-driven bounded shutdown，并用真实进程 exit 0 + etcd immediate cleanup 验证。
3. migration dirty 状态此前缺少显式回归：在隔离 MySQL 测试中补 fail-fast 断言。
4. Coverage 前两次命令因 PowerShell 原生参数绑定未生成有效结果：改为显式插值参数后完整重跑，最终有效结果 76.7%。
5. Race 被既有本机工具链阻断：如实保留 Known Issue，无虚报。

## Known issues

- Race detector 当前环境不可运行；Day 2 并发开发前应优先解决。
- 本机 `protoc 3.9.0` 较旧，后续新增 RPC 契约时需验证插件兼容性。
- Gateway V1 选择排序后的首个 lease endpoint；负载均衡和主动健康探测不属于 Day 1。

## Git / freeze

- Runtime commit：`67d1077e20cda66169a755cae9f36c7996c8a09c`，push SUCCESS。
- Final acceptance/docs commit：本日志所在提交；完成后 push `origin/main`。
- Checkpoint 9 完成后 `DAY 1 = COMPLETED`，Next Authorized Work 为 NONE。
- Day 2 仅为计划项，状态保持 NOT STARTED。
