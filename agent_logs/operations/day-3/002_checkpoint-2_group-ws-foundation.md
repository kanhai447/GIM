# Day 3 / Checkpoint 2 — Group WebSocket Foundation

- **时间:** 2026-10-12 16:26 +08:00
- **开始状态:** `main`，`HEAD == origin/main == 8b225633ef603bb03f78a9053e2d2feb422f7122`，working tree clean。
- **授权范围:** Group WebSocket endpoint、Hub/Client、readPump/writePump、heartbeat、multi-device、backpressure、Origin、Gateway/Auth tunnel、etcd/runtime 与 Presence 负向验证；不实现 Group Message、ACK、clientMsgId 业务、session/unread/history、前端或 File。

## Architecture

- 新增独立 `internal/group/ws`：用户级 `userID -> clientID -> Client` Hub、连接 Handler、typed protocol、Origin policy、lifecycle observer 和 module assembly。
- Group Hub 使用单 event loop 独占连接 map；register/unregister/count/delivery/shutdown 通过 command channel 串行处理。
- 同一用户支持多个 tab/设备，不会用 `map[userID]*Client` 覆盖 sibling；当前不建立 `groupID -> clients` 索引，也不做一群一 socket。
- `cmd/group-api` 在同一 go-zero REST runtime 注册既有 Group HTTP API 与 `/api/group/ws/chat`，复用 `group_api` etcd lease，并在服务退出时等待 Hub shutdown。

## Shared vs Independent

- 复审了 GIM 已封板 Chat Hub/Client/heartbeat 的不变量，但没有复制整个 Chat package、机械改包名或对 Chat 做共享 runtime 重构。
- 选择独立 Group 实现，避免 Checkpoint 2 为抽象而扩大 Private Chat 回归风险；只复用 Gorilla、Gateway/Auth、go-zero process 和既有 discovery Registry 等通用基础设施。
- Group Hub 刻意没有 Chat 的 Presence transition observer、Redis delivery bus 或 message service；两条 WebSocket 保持独立业务语义。

## Single Writer

- `readPump` 只读取 text frame、刷新 Pong deadline、解析 typed envelope 并调用 inbound handler。
- peer Ping 只向容量 4 的 control queue 排队 Pong；Pong、周期 Ping 和 data frame 均由唯一 `writePump` 写入。
- 生产 Group WS 搜索确认 `WriteMessage/WriteControl` 实际调用只在 `client.writePump`；无 `WriteJSON/NextWriter`。
- 100 个并发逻辑发送 stress 经 Hub -> bounded Send -> writePump，fake socket 未观察到 concurrent write。

## Heartbeat / Lifecycle

- 默认 read limit 1 MiB、pong wait 60s、ping period 50s、write wait 10s，配置校验强制 `pingPeriod < pongWait`。
- 初始 read deadline 在 readPump 建立；合法 Pong 只刷新 read deadline；缺失 Pong 触发 timeout unregister。
- `Client.stop` 是 Send/Conn 的唯一关闭点并受 `sync.Once` 保护；duplicate cleanup 不 panic，不会 send-on-closed-channel。
- writePump 独占 ticker 并 `defer Stop`；normal、abnormal、timeout、slow-client 和 shutdown 都收敛到 Hub 的单一生命周期所有权。
- 正式 `group-api` 使用真实 MySQL/Redis/User RPC/etcd 启动：内部 endpoint 无可信身份返回 401，`/gim/services/group_api/group-api-local` lease 可见；Ctrl+C 后 registration 与 8084/9091 listener 均清理。

## Multi-device / Backpressure

- 同用户 A/B 两个连接：A normal close、abnormal close或 timeout 后只移除 A，B 继续存活；最后连接关闭后用户连接数为 0。
- 每个 Client Send channel 有界；Hub 投递不阻塞，队列满时只移除对应 slow Client，不启动无限 goroutine、不影响 sibling。
- graceful shutdown 停止接受连接后关闭全部 Client，read/write pumps 与 tickers 在 timeout 内退出。

## Origin / Auth / Protocol

- endpoint 固定为 `/api/group/ws/chat?token=<JWT>`；Group 不维护第二套 JWT verifier，用户身份只取 Gateway/Auth 注入的 `User-ID/Role`。
- Gateway tunnel 测试提交伪造 `User-ID=999/Role=1`，Group 注册的仍是 token 用户，伪造用户连接数保持 0。
- `GROUP_WS_ALLOWED_ORIGINS` 使用精确 http/https canonical allowlist；allowed localhost PASS，missing/invalid/disallowed origin 拒绝，无 `CheckOrigin=true`。
- typed envelope 使用 `event/requestId/clientMsgId/data/error`；malformed 返回 `2401`，`group.send` 等未实现事件返回 `2402` 并保持连接，不写数据库、不 ACK。

## Presence Boundary

- **Group WS does NOT modify global Presence；Chat WS remains the only Presence authority.**
- Group WS package没有 Redis client、Presence observer、MarkOnline/MarkOffline 或 `gim:presence:*` 访问。
- 真实 Redis 测试覆盖 connect、disconnect、heartbeat timeout、slow removal、Hub shutdown；sentinel ZSET member/score 保持不变，四个 Group 测试用户 Presence key 始终不存在。
- mute 不影响连接；被踢出单个群也不会关闭用户级 Group socket，后续 `group.send` 才执行 membership/mute 校验。

## Membership Boundary

- 连接不要求用户属于某个特定群；后续消息服务必须复用 Checkpoint 1 的 `GetGroup/IsMember/GetMemberRole/IsMuted` 边界。
- 本 Checkpoint 没有 group_messages insert、clientMsgId 幂等、ACK、Pub/Sub、group_sessions、unread 或 history。

## Gateway / Regression

- 真实临时 etcd + Auth + Gateway + Group go-zero endpoint：valid JWT connect、双设备、Ping/Pong tunnel、close cleanup、one timeout/sibling survives、missing JWT、revoked JWT、Header spoofing 全部 PASS。
- Group Foundation 真实 MySQL/Redis create/membership/approve/role/mute transaction/concurrency suite PASS。
- Private Chat message/session/Gateway tunnel focused regression PASS；全量真实依赖 `go test ./... -count=1` 覆盖 Chat Hub/heartbeat/Presence/reliability/session 全部 PASS。

## Tests

- `go test ./... -count=1`（真实 MySQL/Redis/隔离 etcd）：PASS。
- `go vet ./...`：PASS；`gofmt -l .`：PASS；`git diff --check`：PASS。
- `go test ./internal/group/ws/... -count=10`：PASS，生命周期/stress 无 flaky。
- Group WS coverage：package 76.3%，protocol 100%，汇总 77.3%。
- `TestGatewayAuthGroupWebSocketTunnelAndPresenceBoundary`：PASS，包含真实 Redis Presence negative test。
- `TestGroupFoundationMySQLRedisTransactionsAndConcurrency`：PASS。
- Private Chat message/session/Gateway tunnel focused regression：PASS。
- Single writer、multi-device、timeout、abnormal close、slow client、duplicate cleanup、peer Ping、Origin/Auth、typed error、server+Hub shutdown：PASS。

## Race

- `Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`；按授权未重复运行已知由 32-bit MinGW 导致失败的完整 race。
- 本 Checkpoint 新增并发 Hub/Client，已用 event-loop ownership、single writer、`sync.Once`、有界 channel、100-way send 与 10 轮 stress 验证。
- `Race Verification Pending`：兼容 64 位 C 工具链/CI 必须覆盖 Chat + Group。

## Reference / Independent Implementation

- **FIM referenced:** `/api/group/ws/chat` 路径、用户级一条 Group WS 服务多个群、同用户多连接与群事件行为。
- **GIM independently implemented:** 新的 Group Hub/Client/pumps/config/protocol/runtime/tests 全部在 GIM 目录自主编写，没有复制 reference 文件或依赖 reference 编译。
- **GIM improvements:** FIM 使用 package 全局普通 map、无 heartbeat、`CheckOrigin=true`、handler/广播/错误路径直接并发写 Conn，并在请求链使用 `context.Background()`；GIM 使用 Hub 单所有者、单 writePump、deadline、safe Origin、可信 Gateway identity、有界 backpressure、typed safe errors 和可等待 shutdown。

## Data / Protocol / Files

- Database/Redis schema：无变化；Group WS 不创建 Redis key。
- WebSocket：新增 Group connection endpoint 和 connection-level typed error `2401/2402`；`group.send/ack/message` 仍未实现。
- Frontend：无变化。
- 主要文件：`.env.example`、`server/internal/group/ws/**`、`server/internal/group/config/config.go`、`server/cmd/group-api/main.go`、`server/tests/group_gateway_websocket_integration_test.go`、相关 docs/README。

## Git / Next

- Implementation commit：`c3bb003aeb12ed7bdc06a88fea118ee3790316dd feat(group): add websocket connection foundation`。
- Log/status commit：本记录所在提交。
- Push：implementation commit 已成功推送至 `origin/main`；log/status commit 随后推送并执行最终等值/clean 复核。
- Next Planned：Group Message Reliability + Session/Unread；需要用户另行明确授权。
- 本轮严格停止，不进入消息持久化、ACK、clientMsgId、session/unread/history、Group frontend 或 File。
