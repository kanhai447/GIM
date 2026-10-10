# Day 2

## 授权状态

- Day 2：AUTHORIZED。
- 当前 Checkpoint：Chat Presence COMPLETED。
- 已完成：Pre-Day2 Concurrency Preflight。
- 已完成：Chat Hub/Client、readPump/writePump、多连接模型、Chat endpoint/runtime 和 Gateway real WS tunnel。
- 已完成：Chat WebSocket Heartbeat、deadline、dead/slow/normal/abnormal disconnect cleanup 和 multi-device lifecycle。
- 已完成：Redis-backed Chat Presence、多设备 transition、多实例 contribution、TTL/crash recovery 与 shutdown cleanup。

## Pre-Day2 Concurrency Preflight

- 时间：2026-10-09 10:43 +08:00。
- 范围：仅检查 race 环境、Chat schema、Redis、JWT/WS handshake、Gateway Upgrade 和 FIM WebSocket 参考问题。
- 业务代码：无修改；未建立 WebSocket、Hub、Client、readPump、writePump 或 Presence。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT。Go 1.25.2 / windows-amd64 / CGO=1；当前 MinGW.org GCC 6.3.0 为 `mingw32/i586`，`go test -race ./...` 报 `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。Day 2 后续必须在兼容环境/CI 验证，不阻塞开发。
- Chat Schema：READY；幂等 unique、session unique、last message/read、unread、top、用户维度隐藏/删除字段全部符合文档。
- Redis：Client READY；Presence/可选 connection tracking helper 待下一 Checkpoint 在 `rediskeys` 集中新增。
- Auth/JWT：READY；WS query token 由 Gateway 调 Auth 验证，继续 V1 `?token=JWT`，不引入 ticket 或本地重复验签。
- Gateway：READY；显式 Chat route、Upgrade detection、Upgrade 无普通 proxy timeout 和标准 ReverseProxy 条件齐备，focused tests PASS。
- Reference：确认全局普通 map、直接并发 Conn write、无 writePump/Ping/Pong、CheckOrigin 过宽和多设备 Presence/投递语义问题；未复制代码。
- 非 race 回归：`go test ./... -count=1` PASS。
- Day 2 blocker：NONE。

## Operation Log

- `agent_logs/operations/day-2/001_pre-day2_concurrency-preflight.md`
- `agent_logs/operations/day-2/002_checkpoint-1_chat-ws-foundation.md`
- `agent_logs/operations/day-2/003_checkpoint-2_chat-heartbeat-lifecycle.md`
- `agent_logs/operations/day-2/004_checkpoint-3_chat-presence.md`

## Checkpoint 1 — Chat WebSocket Hub / Client Foundation

- 时间：2026-10-09 18:49 +08:00。
- Hub 使用单 event loop 管理 `userID -> clientID -> Client`，支持同用户多 tab/多设备；没有全局共享 clients map。
- 每个 Client 有随机 opaque ClientID、有界 Send、readPump 和唯一 writePump；生产代码的 WebSocket 数据写入只发生在 writePump。
- slow-client 队列满时非阻塞移除该连接，不拖住 Hub 或其他用户。
- endpoint 为 `/api/chat/ws/chat`；Chat 信任 Gateway 注入的 User-ID/Role，不重复解析 JWT。
- Origin 使用显式配置 allowlist，allowed/disallowed/missing 测试均符合预期。
- 正式 Chat API 入口复用 go-zero REST、etcd lease/keepalive/cleanup 和 graceful shutdown。
- Gateway/Auth/Chat 真实 WebSocket tunnel：valid JWT 连接 PASS；missing JWT 与 logout JWT 拒绝 PASS；连接关闭清理 PASS。
- `go test ./... -count=1`、vet、gofmt、diff check、Secret Scan 全部 PASS；Chat foundation coverage 78.6%。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Race Verification Pending compatible environment/CI。
- 未实现 Heartbeat、Presence、ACK、clientMsgId 业务、消息持久化、session/unread 或前端 socket。

## Checkpoint 2 — Chat WebSocket Heartbeat + Connection Lifecycle

- 时间：2026-10-10 10:01 +08:00。
- 生产默认：read limit 1 MiB、pong wait 60s、ping period 50s、write wait 10s；配置强制 `pingPeriod < pongWait`。
- readPump 设置 read deadline/PongHandler；writePump 独占 data、Ping 及 peer-Ping 的 Pong 写入和 heartbeat ticker。
- missing Pong timeout、normal/abnormal close、slow client、double cleanup 与 server/Hub shutdown 均完成清理测试。
- 同用户两个设备 `2 -> 1 -> 0`；一个 timeout 不影响健康 sibling。
- Hub 新增 total connection count，但未产生 Redis/Presence 副作用。
- Gateway heartbeat tunnel 在真实临时 etcd 下保持超过 pongWait：PASS。
- 全量测试、vet、gofmt、diff check、Secret Scan PASS；Chat heartbeat/lifecycle coverage 81.5%。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Race Verification Pending compatible environment/CI。
- 未实现 Presence、Redis online、ACK、持久化、session/unread 或前端 socket。

## Checkpoint 3 — Chat Presence

- 时间：2026-10-10 10:21 +08:00。
- Hub 只在本实例用户连接 `0->1` / `1->0` 发出 local contribution transition；`1->N` / `N->1`、duplicate unregister 不产生重复上下线。
- 独立 Presence Service worker 管 Redis I/O、context timeout、retry、refresh 和 shutdown；Hub event loop 不执行网络调用。
- Redis key 为集中 helper 生成的 `gim:presence:user:{uid}`；ZSET member 是稳定 `CHAT_API_INSTANCE_ID`，score 为过期毫秒时间。
- 多实例：任一有效 contribution 即 ONLINE；A 离开或 crash/TTL 淘汰时 B 存活则保持 ONLINE；最后 contribution 删除才 OFFLINE。
- 生产默认 TTL/refresh/retry/operation timeout 为 90s/30s/5s/1s；不按每个 WebSocket Ping 写 Redis。
- normal、heartbeat timeout、slow client、duplicate cleanup 和 Hub shutdown 全部统一走 connection count transition。
- Chat WS 是全局 Presence 唯一权威；Group WS 不得修改 `gim:presence:*`。本轮无好友广播、Group、DB online 或前端 Presence。
- 全量 test/vet/gofmt/Secret Scan PASS；Presence coverage 75.8%；关键生命周期 10 轮 PASS；真实 Redis multi-instance/TTL 与 Gateway/Auth/Chat Presence tunnel PASS。
- Implementation commit：`6bd5c4506bdccf237c3652d0af1e06587a691c41`。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Race Verification Pending compatible environment/CI。

## Next Checkpoint

Private Message Persistence + clientMsgId + ACK
