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

## 2026-10-08 Day 1 / Phase 0.5 — Checkpoint 6 Core Integration Acceptance

### 目标与修改

- 对既有 Platform/User/Auth/Gateway/MySQL/Redis/etcd 做完整回归、安全、context 与资源生命周期验收。
- 补充真实链路错误签名/过期凭证、ValidPath 伪造、不可用 upstream 与 lease keepalive/cleanup 测试。
- 修复 discovery registration Close 未等待 keepalive goroutine 退出的问题；无业务协议、migration 或 Day 2 功能变化。

### 验收结果

- Register/Login/JWT/Gateway/Auth/User 主链路与 public/protected path -> PASS。
- 缺失、malformed、错误签名、过期、Logout 后凭证 -> 均返回预期 Auth 公开错误。
- `User-ID` / `Role` / `ValidPath` 伪造 -> 删除后注入普通用户可信身份，PASS。
- etcd 注册/发现/lease/keepalive/cleanup、服务不存在、lookup timeout、upstream 不可用 -> PASS，服务 key 无残留。
- 请求 context/timeout 正确传播；HTTP/MySQL/Redis/etcd/RPC/lease 资源关闭与公开错误安全审计 -> PASS。

### 测试

- `go test ./... -count=1`、core cover、`go vet ./...`、gofmt、Secret Scan、diff check -> PASS。
- 真实 MySQL/Redis/etcd/Gateway/Auth/User/lease 生命周期测试 -> PASS。
- Day 1 核心汇总 coverage：75.7% statements。
- race -> NOT RUN - environment limitation，本机 cgo 64 位工具链不可用。

### Git

- commit: `af53a76a1217918fb2f2008b068ff451908bbd95 test(day1): verify core auth gateway user integration`。
- push: `origin/main` -> SUCCESS。

### 下一步

- Checkpoint 6 已完成并推送；下一次进入 Checkpoint 7 migration，本轮不实施。

## 2026-10-08 Day 1 / Phase 1 — Checkpoint 7 正式数据库底座

### 目标与修改

- 新增 001–006 成对 UP/DOWN SQL、嵌入式轻量 migration Runner/CLI 和 dirty failure state。
- 正式建立 Identity/Friend/Private Chat/Group/File/Settings 共 16 张 V1 业务表，并将 User Domain/GORM record 与 Schema 对齐。
- 新增隔离 MySQL 集成验收：字段/default/collation/index、真实 duplicate constraint、11 类 EXPLAIN、User/Auth 注册登录、UP/DOWN/RE-UP。

### Reference 与自主改进

- 只读参考 FIM 相关 Model/查询的业务语义，没有复制 SQL、GORM Model 或配置。
- GIM 使用清晰 pending 状态、clientMsgId 幂等、独立 chat/group session、用户维度隐藏/删除、SHA-256 FileObject/UserFile 分层和公开 Settings 边界。
- V1 统一无数据库 FK，通过服务事务、唯一/非空/CHECK/索引保持一致性；明确好友接受、消息+session、建群+owner member、文件补偿边界。

### 验收

- EMPTY→UP、重复 UP、ALL DOWN、RE-UP：PASS；隔离测试库自动清理。
- 9 类 duplicate insert：全部被预期唯一约束拒绝。
- 11 类 EXPLAIN：索引与谓词/排序匹配；小数据量下两类历史查询由 optimizer 选择 PRIMARY 反向扫描，设计组合索引仍为 possible key。
- User/Auth 真实 Repository + gRPC 注册、登录、lookup：PASS。
- `go test ./... -count=1`、75.7% core coverage、vet、gofmt、Secret Scan：PASS。
- Race：NOT RUN - environment limitation（Windows cgo 64 位编译器限制）。

### Git / 下一步

- implementation commit：`b38a893 feat(db): add v1 schema and versioned migrations`，push SUCCESS。
- Checkpoint 7 完成即停止；Checkpoint 8 Web/Admin 仅在用户明确指令后进入。

## 2026-10-08 Day 1 / Phase 1 — Checkpoint 8 Web + Admin 基础工程

### 目标与修改

- 自主建立相互独立的 Web 与 Admin Vue 3 SPA；统一使用 pnpm、TypeScript、Vite、Pinia、Router 和 typed Axios API 层。
- Web 使用 Element Plus，完成登录/注册、Auth Store/session restore、protected guard、主布局与 Profile/Chat/Group/File 基础入口。
- Admin 使用 Arco Design + ECharts，复用现有 Auth，完成 role UI guard、管理布局、Dashboard 空状态及 User/Chat/Group/File/Settings/Logs 骨架。
- API 默认同源，并由 Vite 开发代理连接可配置 Gateway；无硬编码生产地址或 Secret。

### Reference 与自主实现

- 仅参考 FIM Web 的页面/API 语义和 FIM Admin 的真实模块范围；全部配置、组件、store、HTTP Client、路由和测试均自主编写。
- 避免原 Admin 错误 Logout URL、三个 Mock Dashboard endpoint 和 `v-html`；没有复制原前端，也没有进入 WebSocket/File/Kafka 业务。

### 验收

- Web：`pnpm install`、type-check、production build、4 files / 9 tests PASS。
- Admin：`pnpm install`、type-check、production build、5 files / 9 tests PASS；ECharts 按需运行验证 PASS。
- lint：两端均未配置；没有为本 Checkpoint 无依据新增工具链。
- 真实 etcd Gateway/Auth/User Register/Login/User Info/Logout/注销后拒绝链路 PASS；前端 API contract 路径和 `token` Header PASS。
- env/ignore/Secret Scan/diff check PASS；无 Token、JWT signing material、数据库/Redis凭据或 FIM Secret。

### Git / 下一步

- Web commit：`19512be8afe74cd03cb457b0b3a96770f4ac14e0`。
- Admin commit：`59490fae4e4590e3a91d49a26234c30564065267`。
- push：SUCCESS — `origin/main`。
- Checkpoint 8 完成后停止；下一次仅在用户明确指令下进入 Checkpoint 9。

## 2026-10-09 Day 1 / Phase 1 — Checkpoint 9 Final Acceptance & Freeze

### 目标与修复

- 对 Backend、Database、Web、Admin、安全配置、reference independence 与 Git 做 Day 1 最终验收。
- 补齐缺失的 Auth API、User API、User RPC 正式入口；复用现有 modules，main 仅负责配置、依赖 wiring、启动和关闭。
- 新增共享 process 配置与 go-zero REST 显式 lifecycle；修复 Windows proc shutdown no-op 导致 API Ctrl+C 不能运行 defer/lease revoke 的问题。
- migration 集成测试新增 dirty metadata fail-fast 断言；无 schema 或业务协议变化。

### Runtime / integration

- 正式四进程 + 真实 MySQL/Redis/临时 etcd：Register/Login/Gateway/Auth/User Info/Logout/旧 credential 拒绝 PASS。
- 缺失/malformed/错误签名/过期/注销、public/protected、Header spoofing、service missing、lookup timeout、upstream unavailable 回归 PASS。
- Auth/User API lease 等待超过 TTL 后保持；四进程优雅退出均 exit 0；service key immediate cleanup，端口全部关闭。
- 临时 etcd/data/binaries、隔离 migration 数据库和 3 条 smoke user 均已清理。

### Context / resource / security

- 业务请求 context 贯穿 Gateway/Auth/RPC/Repository；Background 仅用于 process lifecycle、startup/shutdown/migration。
- HTTP body、gRPC connection/listener/server、MySQL、Redis、etcd、lease/keepalive 都有明确关闭路径。
- Secret Scan/repository safety、本地敏感值比对、private key/JWT literal/OAuth assignment、reference dependency/symlink 检查全部 PASS。

### Tests

- Backend non-cached full test、真实 integration/migration、vet、gofmt：PASS。
- 核心 coverage：76.7% statements。
- Web：type-check/build + 4 files / 9 tests PASS；Admin：type-check/build + 5 files / 9 tests PASS；lint 均 NOT CONFIGURED。
- Race：NOT RUN - environment limitation；`cc1.exe` 不支持 64 位 mode。

### Git / freeze

- Runtime commit：`67d1077e20cda66169a755cae9f36c7996c8a09c feat(runtime): complete day1 service startup wiring`，push SUCCESS。
- Final acceptance/docs commit：本记录所在提交，push 后确认 `HEAD == origin/main`。
- `DAY 1 = COMPLETED`；Day 2 为 NOT STARTED，Next Authorized Work 为 NONE。

## 2026-10-09 Day 2 / Preflight — Concurrency and Readiness

### 目标与范围

- 只检查 Day 2 并发测试环境、Chat schema、Redis、JWT/WS handshake、Gateway Upgrade 和 FIM WebSocket 参考问题。
- 未实现 Chat WebSocket、Hub/Client、readPump/writePump、Presence、ACK 或 session/unread 业务。

### 开始状态

- branch：`main`。
- HEAD：`a638581ab538e1ada77e123ad694dc00c888e98b`。
- `git fetch origin` 后 `HEAD == origin/main`：PASS。
- working tree：CLEAN。

### 结果

- Race：Go 1.25.2 / windows-amd64 / CGO=1；当前 MinGW.org GCC 6.3.0 target `mingw32/i586`。`go test -race ./...` 在 runtime/cgo 构建阶段报 `64-bit mode not compiled in`；本机无可直接切换的 64 位编译器。**Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT**，保留兼容环境/CI 强制验证要求，不阻塞 Day 2。
- Schema：`chat_messages`、`send_user_id`、`rev_user_id`、`client_msg_id`、sender/client unique、`chat_sessions`、user/peer unique、last message/read、unread、top 全部符合文档；`hidden_at` 与 `chat_message_deletions` 保持用户维度隐藏语义，未发现 Day 1 遗漏。
- Redis：现有 go-redis client/lifecycle 可复用；集中 key helper 边界已存在。Presence/可选设备 key 尚未实现，须在下一 Checkpoint 集中加入，不得硬编码。本轮未实现 Presence。
- Auth：Gateway 已读取 WS query token，继续通过 Auth authentication 完成 JWT/blacklist 校验并注入可信身份；不在 Gateway 或 Chat 复制本地验签，不改 WS ticket。
- Gateway：`chat -> chat_api` 显式路由、Upgrade detection、Upgrade 请求跳过普通 proxy timeout、标准 ReverseProxy 条件已具备；focused tests PASS。真实 tunnel 测试等待最小 Chat endpoint。
- FIM reference：确认普通全局 map、多个路径直接写 Conn、无 writePump、无 Ping/Pong、`CheckOrigin=true`、多设备通知/投递语义不一致等问题；未复制代码。
- Day 2 blocker：NONE。

### 测试

- `go test -race ./...` -> NOT AVAILABLE（environment/toolchain build failure）。
- Auth/Gateway/Redis focused tests -> PASS。
- `go test ./... -count=1` -> PASS。

### 数据/协议变化

- DB：无。
- HTTP：无。
- WebSocket：无。
- Redis：无 key/data 写入；仅记录后续 helper 要求。

### Git / 下一步

- commit：本记录所在 `chore(day2): record concurrency preflight` 提交。
- push：完成 Secret Scan 后推送 `origin/main`，最终结果由本轮 Git 复核确认。
- Next Checkpoint：Chat WebSocket Hub / Client foundation。

## 2026-10-09 Day 2 / Checkpoint 1 — Chat WebSocket Hub / Client Foundation

### 目标与实现

- 自主实现 Chat Hub event loop、单用户多 Client 模型、幂等 register/unregister、随机 ClientID、有界 Send、readPump/唯一 writePump、slow-client backpressure 和 shutdown。
- 新增严格 typed envelope boundary、配置化精确 Origin allowlist 和 `/api/chat/ws/chat` Upgrade handler。
- 新增正式 Chat API 入口，复用既有 go-zero runtime、etcd lease/keepalive/cleanup 和 Gateway/Auth 认证边界。
- 未实现 Heartbeat、Presence/Redis online、ACK/clientMsgId 处理、消息持久化、session/unread 或前端 socket。

### Reference 与改进

- 只读参考 FIM endpoint、私聊连接和消息流向；没有复制代码或引入 reference 依赖。
- GIM 避免全局普通 map、并发直写 Conn、无 writePump、无生命周期所有者、Origin 全放行和单用户连接覆盖问题。
- 生产代码由 writePump 独占 WebSocket 数据写入；Hub event loop 独占连接 map。

### 验收

- Hub/Client、双连接、关闭其一保留另一连接、出站 typed frame、断开清理、slow-client、Origin 单元/集成测试：PASS。
- 真实临时 etcd 下 Gateway → Auth → Upgrade → go-zero Chat API tunnel：valid JWT PASS，missing/logout JWT 均 401，关闭清理 PASS。
- 正式 Chat binary 注册 `/gim/services/chat_api/chat-api-local`，Ctrl+C 后 service key 立即清理且端口关闭：PASS。
- `go test ./... -count=1`、`go vet ./...`、gofmt、diff check、Secret Scan：PASS。
- Chat foundation coverage：78.6% statements。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；兼容 64 位工具链/CI 验证仍 PENDING。

### Git / 下一步

- implementation commit：`b2e6f8cc2b10bc9cff834e0bf45e92cf7eacd932 feat(chat): add websocket hub and client foundation`。
- log/status commit：本记录所在提交。
- 下一 Checkpoint：Chat WebSocket Heartbeat + Connection Lifecycle；本轮完成后停止。

## 2026-10-10 Day 2 / Checkpoint 2 — Chat WebSocket Heartbeat + Connection Lifecycle

### 目标与实现

- 增加配置化 read limit、pong wait、ping period、write wait；安全默认 1 MiB / 60s / 50s / 10s。
- readPump 设置 initial deadline、Pong refresh、无写 Ping/Close handlers；writePump 独占 data/Ping/Pong control 写和 ticker。
- Hub 新增 total connection count；disconnect reason 区分 normal/timeout/abnormal/slow/shutdown，安全日志只记录 userID/opaque ClientID/reason。
- Hub 触发唯一 `Client.stop` 关闭 Send/Conn；stopOnce、幂等 unregister 和不关闭的有界 control queue 避免 double close/send-on-closed。
- 未实现 Presence、Redis online、ACK、消息持久化、session/unread 或前端 socket。

### FIM reference / 改进

- 只读确认 FIM 无 Ping/Pong、deadline/dead detection，仍有全局 map 与多路径直接写 Conn；未复制实现。
- GIM 使用 Hub event loop、单 writePump、配置化 heartbeat、原因分类和可等待的 pump/ticker 生命周期测试。

### 验收

- Pong refresh、missing Pong timeout、normal/abnormal close、slow client、double cleanup、shutdown：PASS。
- 同用户两个连接 timeout-one/keep-sibling，connection count `2 -> 1 -> 0`：PASS。
- 9 个核心 heartbeat/lifecycle 测试组合连续 10 轮：PASS。
- 真实 etcd Gateway/Auth/Chat tunnel：Ping/Pong 保持超过 pongWait、关闭清理、missing/logout token 回归 PASS。
- `go test ./... -count=1`、vet、gofmt、diff check、Secret Scan：PASS。
- Chat heartbeat/lifecycle coverage：81.5% statements。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；兼容环境/CI verification PENDING。

### Git / 下一步

- implementation commit：`2b90df522337932bb57dfb0889caf39f54da80d3 feat(chat): add websocket heartbeat and connection lifecycle`。
- log/status commit：本记录所在提交。
- 下一 Checkpoint：Chat Presence；本轮完成后停止。

## 2026-10-10 Day 2 / Checkpoint 3 — Chat Presence

### 目标与实现

- 在既有 Hub 多连接事实源上，只对本实例 `0->1` / `1->0` 产生 local Presence contribution transition；多 tab/设备中间变化不重复上下线。
- 新增独立 Presence Service/Store、集中 Redis key helper、`IsOnline` 查询、context timeout、dirty-state retry、独立 refresh 和有界高优先 shutdown。
- Redis 使用用户维度 ZSET：instance ID member + expiresAt score；至少一个未过期 member 即全局 ONLINE，避免任一 Chat 实例最后连接断开误删其他实例状态。
- Chat API 复用 discovery registration instance ID 和现有 Redis Client；Hub shutdown 后清理本实例 contribution，crash/清理失败由 TTL 兜底。
- 同步更新 Backend/Core Flow/Redis 设计；明确 Chat WS 是唯一全局 Presence authority，Group WS 不修改 Presence。

### Reference 与改进

- 只读参考 FIM Chat 的多连接、Redis online Hash 和好友上线行为；没有复制代码或引入 reference 依赖。
- GIM 避免 FIM 全局普通 map、每连接 HSet、无 TTL/多实例 contribution 及多路径并发写 Conn；未提前复制好友广播。

### 验收

- `0->1` online、`1->N` no duplicate、`N->1` no offline、`1->0` offline、duplicate unregister：PASS。
- heartbeat timeout sibling survives/final offline、slow-client、shutdown、Redis timeout/unavailable/recovery：PASS；关键组合 10 轮稳定。
- 真实 Redis 双实例、A disconnect、A crash TTL/B refresh、last B offline、测试 key cleanup：PASS。
- 真实 etcd + Gateway/Auth/Chat/Redis：query JWT connect、双设备、唯一/最后关闭、heartbeat sibling timeout Presence：PASS。
- `go test ./... -count=1`、vet、gofmt、diff check、Secret Scan：PASS；Presence coverage 75.8%。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；兼容环境/CI verification PENDING。

### 数据/协议

- DB：无；不写 users.online、message/session/unread。
- HTTP/WS envelope：无变化；无好友广播或 Presence WS event。
- Redis：新增 `gim:presence:user:{uid}` ZSET instance contribution 与 TTL/refresh/retry 配置。

### Git / 下一步

- implementation commit：`6bd5c4506bdccf237c3652d0af1e06587a691c41 feat(chat): add redis backed user presence`。
- log/status commit：本记录所在提交。
- 下一 Checkpoint：Private Message Persistence + clientMsgId + ACK；本轮完成后停止。

## 2026-10-10 Day 2 / Checkpoint 4 — Private Message Persistence + clientMsgId + Server ACK

### 目标与实现

- 新增 typed `chat.send` / `chat.ack` / `chat.message` / `error`，只实现 text；sender 只来自 Gateway identity/Client.UserID。
- 新增 Message Service、MySQL Repository、User RPC receiver adapter、curtail_chat 与 friendship policy repository。
- clientMsgId 使用 MySQL `(send_user_id, client_msg_id)` UNIQUE 最终收敛；duplicate 查询原消息并返回同一 messageId。
- ACK 只表示 persist/accepted 且只发 originating Client；receiver offline、Pub/Sub failure 或 receiver backpressure 不改变 ACK 语义。
- 新消息统一走 `gim:chat:delivery` Redis Pub/Sub，所有实例只投本地 receiver Clients；同实例无 direct + bus 双路径，duplicate retry 不 republish。
- malformed JSON 返回 typed error 并保持连接；错误不泄露 SQL/DSN/RPC/internal detail，日志不记录正文或凭证。
- 未实现 chat_sessions、unread、read state、history API、frontend socket、Group 或 File。

### Reference 与改进

- 只读参考 FIM 文本 msg、好友/限制校验和 receiver 推送语义；没有复制 handler/repository。
- GIM 补齐 DB UNIQUE 幂等、persist ACK、originating-client targeting、single writePump、createdNew duplicate suppression 与跨实例 Redis fanout。

### 验收

- 真实 MySQL Repository create/query/duplicate/get by ID 与 test database cleanup：PASS。
- 50-way concurrent same clientMsgId：数据库 1 行，所有结果 same messageId，PASS。
- lost ACK retry、offline ACK、same-instance receiver、Instance B 两设备、sender sibling no ACK、duplicate no-redelivery：PASS。
- 完整本地 MySQL/Redis/临时 etcd `go test ./... -count=1`：PASS；Gateway/Auth/Chat WS 与 Presence real Redis regression PASS。
- `go vet ./...`、gofmt、diff check、Secret Scan：PASS。
- Message coverage 69.5%；User RPC adapter coverage 71.4%。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible environment/CI PENDING。

### 数据/协议

- DB migration 无变化；只写 chat_messages，不写 chat_sessions/unread。
- Redis 新增集中 Pub/Sub channel `gim:chat:delivery`；不是持久消息源。
- 正式启用 private chat send/ack/message/error envelope。

### Git / 下一步

- implementation commit：`6da24e4e161d5fc66c7d843eb676ff367e47f431 feat(chat): add idempotent private messaging and server ack`。
- log/status commit：本记录所在提交。
- 下一 Checkpoint：Chat Session + Unread + Read State；本轮完成后停止。

## 2026-10-11 Day 2 / Checkpoint 5 — Chat Session + Unread + Read State + History Cursor

### 目标与实现

- 将 message INSERT、sender session、receiver session/unread 收敛为单一 MySQL transaction；固定 session 行更新顺序，ACK/publish 只发生在 commit 后。
- receiver 在线/离线统一 unread 原子 +1；重复 clientMsgId 读取原消息，不重复 unread/session/publish；last message 只随更大 server ID 推进。
- 新增 Session Repository：list/get/mark read/set/toggle top/hide；新消息恢复 hidden session，send 不覆盖 isTop/read cursor。
- 新增 History Repository：规范化双方 ID + server message-ID cursor + current-user deletion filter；API oldest→newest，默认 20/最大 100。
- 新增 Chat HTTP transport：`GET /api/chat/session`、`GET /api/chat/history`、`PUT /api/chat/read`、`POST /api/chat/user_top`；身份只取 Gateway Header。
- Session list 使用一次 `chat_sessions/users/chat_messages` JOIN，避免 N+1；未新增 User RPC batch 或跨服务协议。

### Reference 与改进

- 参考 FIM 路径、双向历史、批量用户信息意图、top 与个人删除行为；未复制源码。
- GIM 用显式 session 支持 unread/read/top/hidden 与索引排序，理由是复杂度、查询成本和用户状态语义；没有把 FIM 最新 preview 子查询错误描述为 `MAX(msg_preview)` bug。
- deletion 严格绑定当前 user；Mark Read 是用户级数据库状态，不是 peer read receipt。

### 验收

- 真实 MySQL 50 same clientMsgId：message=1、receiver unread +1、same messageId；50 distinct：50 messages、unread +50、last ID 最大，PASS。
- partial/stale/latest/duplicate/concurrent read、session order/top/hidden/reappear、两页 history/no duplicate/conversation isolation/per-user deletion/limit/empty：PASS。
- message/sender-session/receiver-session failure injection 全 transaction rollback 且 no ACK/no publish；Redis post-commit failure 保留 DB + ACK，PASS。
- CP5 真实 MySQL suite 连续 3 次 PASS；CP4 Redis Pub/Sub、Presence、Gateway/Auth/Chat tunnel 与 migration 回归 PASS。
- `go test ./... -count=1`、`go vet ./...`、gofmt、diff check、Secret Scan：PASS。
- Message/Session/MySQL coverage 71.1%；Chat HTTP transport coverage 56.3%。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible environment/CI verification PENDING。

### 数据/协议

- DB migration：无；开始使用既有 chat_sessions/chat_message_deletions。
- HTTP：启用 session/history/read/user_top。
- WS：无新增 event；ACK 语义收紧为 message/session transaction committed，仍非 delivered/read。
- Redis：无 key 变化；delivery 继续 commit 后 best-effort Pub/Sub。

### Git / 下一步

- implementation commit：`3dac219ca23021aee96d0f4b824800e88b4914c4 feat(chat): add sessions unread and read state`。
- log/status commit：本记录所在提交。
- 下一 Checkpoint：Private Chat Frontend Integration；本轮完成后停止。

## 2026-10-11 Day 2 / Checkpoint 6 — Private Chat Frontend Integration

### 目标与实现

- 新增 typed Chat API、无 DOM Chat Socket Client、Pinia Chat Store、消息 reconcile 工具和私聊组件；页面不直接持有或操作 WebSocket。
- 单登录用户单 Chat WS，支持同源/配置化 ws/wss URL、typed server frame、安全忽略 malformed frame、四态连接与 30 秒上限指数退避。
- login/session restore 自动连接；logout 先停 socket/reconnect/ACK timers 并清内存，再 Auth logout；auth failure 清状态并回登录页。
- 断线发送采用内存 pending queue；每个逻辑消息只生成一次 UUID clientMsgId，ACK timeout、手动 retry 和 reconnect retry 都复用该 ID。
- ACK 将 pending 映射 server messageId/sent；typed error 只失败相关消息；duplicate ACK/message/history page 通过 messageId/clientMsgId reconcile 不重复展示。
- session list 信任服务端 top/time 排序；other-peer realtime 本地 unread +1 后单 debounce refresh；current route 只在 visible+focused 时自动 mark read。
- history 使用 beforeMessageID cursor、oldest→newest merge 和 load older；UI 提供 pending/sent/failed/retry、unread/top/preview/time 和安全 text rendering。

### Reference 与改进

- 只读参考 FIM 的 session sidebar、history/top/text composer 与登录后 Chat WS 路径；未复制任何文件或业务实现。
- GIM 没有延续全局 Store 中裸 WebSocket、业务直接 send、无 ACK/reconnect/pending、HTML 图文输入等模式；按 GIM 文档独立实现 typed lifecycle/reliability/reconcile。

### 验收

- Web `pnpm type-check`、`pnpm test`（8 files / 35 tests）、`pnpm build`：PASS；lint NOT CONFIGURED。
- Socket/Store/component 覆盖 connect/backoff/logout/auth failure、stable ID/retry、ACK mapping、error/timeout、dedupe、unread/read、cursor merge 和 UI 状态。
- 真实 Chrome 双用户 + MySQL/Redis/etcd/Gateway/Auth/User/Chat/Vite：pending→sent、realtime、未读→打开清零、刷新恢复、receiver 双设备、20+ 分页、Chat API 重启 reconnect：PASS。
- `go test ./... -count=1`、`go vet ./...`、diff check、Secret Scan：PASS。
- 测试用户/消息与全部 CP6 临时进程/文件均清理；无端口残留。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible environment/CI PENDING。

### 数据/协议

- DB migration、服务端 HTTP/WS、Redis：无变化。
- 前端新增 `/chat/:peerId?`、`VITE_WS_BASE_URL`、Chat API/Socket/Store/Components 和可重复 Chrome smoke。
- pending 不跨刷新持久化；已提交消息通过 history 恢复。这是 V1 已记录限制。

### Git / 下一步

- implementation commit：`80456b3e27e2ebc0dc56d03f17c1ee58cf0c6f6d feat(web): integrate reliable private chat`。
- log/status commit：本记录所在提交。
- Private Chat V1：FUNCTIONAL。
- 下一 Checkpoint：Day 2 Final Private Chat Acceptance；本轮不进入 Group/File/Admin/Day 3。

## 2026-10-11 Day 2 / Checkpoint 7 — Private Chat Final Acceptance & Freeze

### Scope / Changes

- 只做 Private Chat V1 最终 regression、真实浏览器、失败场景、并发、生命周期与安全验收；未进入 Group、File、Admin Chat 或 Day 3。
- 无业务 production code、migration、HTTP/WS/Redis protocol 变化。
- 增强 Web 验收测试：成功重连后 backoff reset、1205/1206 auth terminal、inactive conversation 不自动 read、HTML-like text 字面渲染，以及真实浏览器双向/双设备/shared read/logout/revoked token。

### Acceptance

- 真实 Chrome + MySQL/Redis/etcd/User/Auth/Chat/Gateway/Vite：双向顺序、pending→sent、realtime/unread/read、刷新、双设备、20+ cursor history、Chat restart reconnect、logout/auth failure 与纯文本渲染 PASS。
- ACK lost/same-ID retry、50 duplicate、50 distinct、partial/stale/latest/concurrent read、hidden/deletion user scope、Redis multi-instance and degraded delivery、Gateway/Auth/Origin/single-writer/Presence 全部回归 PASS。
- Go full integration PASS；message/session 3 轮、Presence 3 轮、Hub/heartbeat/single-writer/shutdown 10 轮 PASS；vet/gofmt/diff/Secret Scan PASS。
- Web type-check、8 files / 40 tests、production build PASS；lint NOT CONFIGURED。
- 临时用户/业务数据、Presence contribution、服务、浏览器、bin/log/etcd data 精确清理。

### Freeze

- `PRIVATE CHAT V1 = COMPLETED`。
- Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Race Verification Pending compatible environment/CI。
- V1 limitations：pending 不跨刷新；无 delivery/read receipt；当前仅 text message。
- Next Authorized Work：NONE。Next Planned：Day 3 — Group WS + File Service + User Web protocol adaptation，等待用户明确授权。

## 2026-10-12 Day 3 / Checkpoint 1 — Group Domain + Membership + API Foundation

### 目标与实现

- 复用既有 `004_group` schema，自主实现 Group Domain/Repository/Service/Policy、正式 API/RPC commands 与稳定错误边界；无 migration 变化。
- 创建群使用可信 Gateway identity，group + owner/invited memberships 同事务；open join 与 approve 保证 verify state + member 原子。
- 实现 group info、my groups、member list、invite、apply/approve/reject、leave/kick、role、dissolve；owner/admin/member 权限集中在 policy。
- 成员列表通过新增 User `BatchGetUsers` RPC 一次补齐公开资料；My Groups 使用 SQL JOIN + 聚合，避免明显 N+1。
- 临时禁言使用集中 `gim:group:prohibition:{memberId}` Redis TTL；支持 mute/unmute/expiry/cross-group isolation 和 unavailable safe error；不触碰 Presence。
- 内部 Group RPC 仅暴露 GetGroup/IsMember/GetMemberRole/IsMuted；未实现任何 Group WS 或群消息能力。

### Reference 与改进

- 只读参考 FIM 群创建、角色、申请审批、邀请、退出踢人、禁言与 Web API 字段；未复制文件、业务代码或依赖 reference。
- GIM 修复 FIM 多步骤创建/审批部分成功风险，采用 transaction + row lock + unique constraint；权限集中 policy；Redis key 集中；context 贯穿 DB/RPC/Redis；成员资料改为 batch RPC。

### 验收

- Group 真实 MySQL/Redis integration 连续 3 次 PASS；create/approve failure injection rollback、concurrent approve/add member、duplicate pending、reject/open join、权限、TTL/expiry、Presence isolation 和四类 EXPLAIN PASS。
- Group coverpkg 67.9%；MySQL Repository 77.4%；Service 67.0%。
- Private Chat message/session focused regression PASS；全量 `go test ./... -count=1` PASS。
- `go vet ./...`、`gofmt -l .`、`git diff --check`、repository safety 与 candidate secret pattern scan PASS。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible environment/CI PENDING。

### 数据/协议

- DB migration：无；使用现有 groups/group_members/group_verifies 与索引。
- HTTP：启用 Group CP1 create/info/dissolve/my/member/invite/leave-kick/role/apply-review/mute。
- RPC：新增 Group membership minimum RPC；User 新增 BatchGetUsers。
- Redis：新增集中 Group prohibition key helper；无 Presence 写入。
- WS/Frontend：无变化。

### Git / 下一步

- implementation commit：`860ba8eb16dac77ad751fecb0c62d9e509f74cb9 feat(group): add group domain and membership foundation`。
- log/status commit：本记录所在提交。
- push：implementation commit 已成功推送至 `origin/main`；log/status commit 随后推送并执行最终等值/clean 复核。
- 下一 Checkpoint：Group WebSocket Foundation，需用户另行授权；本轮完成后停止。

## 2026-10-12 Day 3 / Checkpoint 2 — Group WebSocket Foundation

### 目标与范围

- 实现独立 Group WebSocket endpoint、Hub/Client、多设备、readPump/唯一 writePump、heartbeat/deadline、backpressure、Origin、Gateway/Auth tunnel、etcd runtime 与 graceful shutdown。
- 严格不实现 Group message persistence、ACK、clientMsgId 业务、session/unread/history、前端或 File；Group WS 不参与 Presence。

### 修改

- 新增 `server/internal/group/ws/**` 与 typed protocol/tests；Hub event loop 独占 `userID -> clientID -> Client` map。
- Group config 增加 `/api/group/ws/chat`、有界 queue、Origin allowlist、1 MiB/60s/50s/10s heartbeat 配置。
- `cmd/group-api` 同时注册 HTTP API/WS route，启动并等待 Group Hub，复用 `group_api` etcd lease。
- 新增真实 Gateway/Auth/Group/Redis/etcd integration；同步 WS/core flow/Redis/README 文档。

### 验证

- 真实 tunnel：valid/missing/revoked JWT、spoofed identity、双设备、Ping/Pong、timeout sibling、close cleanup PASS。
- Redis Presence sentinel：connect/disconnect/timeout/slow/shutdown 全程未修改 `gim:presence:*`，PASS。
- Group API foundation、Private Chat message/session/Gateway 与全量真实依赖回归 PASS。
- Group WS 10 轮 stress PASS；coverage 76.3%，protocol 100%，汇总 77.3%。
- `go test ./... -count=1`、vet、gofmt、diff、Secret Scan PASS。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Chat + Group compatible environment/CI PENDING。

### Git / 下一步

- implementation commit：`c3bb003aeb12ed7bdc06a88fea118ee3790316dd feat(group): add websocket connection foundation`。
- log/status commit：本记录所在提交。
- push：implementation commit 已成功推送至 `origin/main`；log/status commit 随后推送并执行最终等值/clean 复核。
- Next Planned：Group Message Reliability + Session/Unread，需用户明确授权；本轮到此停止。
## 2026-10-12 Day 3 / Checkpoint 3 — Group Message Reliability + Session / Unread / History

### Scope / Changes

- 只实现文本群消息 typed protocol、Group policy 复用、MySQL 幂等事务、group session/unread/read/history 与独立 Redis realtime fanout。
- 无 migration；复用 `004_group` 的 UNIQUE/index/session/deletion schema。
- 未进入 Group Frontend、File、Admin 或 Day 4。

### Reliability / Delivery

- active group shared lock + sender membership + message insert + current-member snapshot + batch session upsert 同事务；sender unread +0，其他成员 +1。
- MySQL UNIQUE 最终幂等；duplicate transaction 结束后新事务读取胜者，避免 REPEATABLE READ 旧快照；duplicate 不修改 session/publish。
- commit 后 originating ACK；`gim:group:delivery` 跨实例投递本地全部目标 Clients。sender sibling 与 receiver 多设备均收到 message，Redis failure 保留 DB/ACK。
- session JOIN 无 N+1；read cursor 单调并与 send 行锁串行；history cursor/member/deletion 隔离。

### Acceptance

- 真实 MySQL/Redis focused integration 3 轮 PASS：50 duplicate、50 distinct、mute/membership、session/read/history、failure injection、EXPLAIN。
- actual two-instance Group WebSocket + sender sibling + receiver two devices PASS；Presence negative PASS。
- 临时 etcd 下 `go test ./... -count=1` PASS；Group foundation/Private Chat regression PASS。
- `go vet ./...`、gofmt、diff、Secret Scan PASS；focused coverage 69.1%。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible CI PENDING。

### Git / Next

- implementation commit：`547afe835f84760adf9733f1661050706ef8b32f feat(group): add reliable group messaging and sessions`。
- log/status commit：本记录所在提交。
- push：implementation 与本 log/status commit 已 push `origin/main`；最终 HEAD equality/clean 已复核。
- 下一入口：Group Frontend Integration，需新授权。

## 2026-10-13 Day 3 / Checkpoint 4 — Group Frontend Integration

### Scope / Changes

- 自主实现 Group typed types/API、单例 socket manager、Pinia store、reconcile、UI components、`/group/:groupId?` 和真实 Chrome smoke；未修改后端、migration 或协议。
- Chat/Group 两条连接和 Store 保持独立；`App.vue` 统一启动、logout stop 与 auth invalidation，任一连接认证失败会清空 session 并回 Login。
- My Groups 与 Group Sessions 分离；Group session/realtime/unread/read/history/info/member cache/minimal create/mute UX 完成。

### Reliability / UX

- single reconnect timer + 30s cap、stable UUID clientMsgId、pending/sent/failed、persist ACK、lost ACK same-ID retry。
- ACK-first/message-first、duplicate realtime、history overlap 按 messageId/clientMsgId 收敛；未知事件安全忽略。
- active route + visible + focused 才 Mark Read；inactive group 本地 unread 增加，reconnect/refresh 用后端权威 session 校准。
- mute/all-mute 显示安全失败且 socket 继续接收；nonmember/dissolved 禁止重试并刷新/退出；文本使用 Vue 插值，无 v-html/innerHTML。

### Acceptance

- Web `pnpm test` 12 files / 77 tests、`pnpm type-check`、`pnpm build` PASS；lint NOT CONFIGURED。
- 真实 Chrome A/B/C：create/invite、pending→sent、realtime/unread/read、refresh、mute/unmute、receive while muted、HTML literal、B1/B2 exact-once/shared read、20+ cursor、Group restart/reconnect、Chat unaffected、logout/revoked auth PASS。
- Group + Private Chat 真实 MySQL/Redis focused integration 与后端 `go test ./... -count=1` PASS；vet/gofmt/diff/Secret Scan PASS。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；compatible 64-bit toolchain/CI PENDING。
- 测试用户/群/消息/session、浏览器、服务与临时 etcd 已清理。

### Git / Freeze

- implementation commit：`0a17af6 feat(web): integrate reliable group chat`，已推送 `origin/main`。
- log/status commit：本记录所在提交，随后推送并复核 equality/clean。
- `GROUP CHAT V1 = FUNCTIONAL`（text scope）；Private Chat regression PASS。
- Next Authorized Work：NONE。Next Planned：File Service checkpoint；严格未进入 File、Admin 或 Day 4。
