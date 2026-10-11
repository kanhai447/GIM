# Day 2

## 授权状态

- Day 2：COMPLETED（按四日计划的 Chat WS + Presence + 私聊可靠性范围）。
- 当前 Checkpoint：Private Chat Final Acceptance & Freeze COMPLETED。
- 已完成：Pre-Day2 Concurrency Preflight。
- 已完成：Chat Hub/Client、readPump/writePump、多连接模型、Chat endpoint/runtime 和 Gateway real WS tunnel。
- 已完成：Chat WebSocket Heartbeat、deadline、dead/slow/normal/abnormal disconnect cleanup 和 multi-device lifecycle。
- 已完成：Redis-backed Chat Presence、多设备 transition、多实例 contribution、TTL/crash recovery 与 shutdown cleanup。
- 已完成：typed private chat send、MySQL persist、clientMsgId 幂等、Server ACK、offline/online/multi-device/multi-instance realtime delivery。
- 已完成：message/session transaction、unread、Mark Read、Session List/top/hidden 与 message-ID cursor History。
- 已完成：用户端 Chat Socket/Store/API、pending/sent/failed、稳定 clientMsgId、reconnect/retry、session/unread/read/history 与真实浏览器双用户联调。
- 已完成：Checkpoint 7 最终真实浏览器、失败场景、并发、生命周期、安全与稳定性验收；`PRIVATE CHAT V1 = COMPLETED`。

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
- `agent_logs/operations/day-2/005_checkpoint-4_private-message-ack.md`
- `agent_logs/operations/day-2/006_checkpoint-5_chat-session-unread-read.md`
- `agent_logs/operations/day-2/007_checkpoint-6_private-chat-frontend.md`
- `agent_logs/operations/day-2/008_checkpoint-7_private-chat-final-acceptance.md`

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

## Checkpoint 4 — Private Message Persistence + clientMsgId + Server ACK

- 时间：2026-10-10 18:42 +08:00。
- 正式启用 typed `chat.send`、`chat.ack`、`chat.message`、`error`；sender 只取可信 Client identity，CP4 只实现 text。
- 接收者通过 User RPC + caller-derived timeout 校验；按 docs 校验 `curtail_chat` 与好友关系。
- MySQL Repository 使用既有 `(send_user_id, client_msg_id)` UNIQUE 作为最终幂等保障；duplicate 查询原消息并 ACK 同一 messageId。
- ACK 只表示 message persisted/accepted，只发 originating Client；不是 delivery/read receipt，receiver offline 仍 ACK。
- 只有新插入消息触发 realtime delivery；重复 retry 不重投 receiver。
- 所有私聊 realtime delivery 统一经 `gim:chat:delivery` Redis Pub/Sub，各实例只投本地 receiver Clients；同实例无第二条直投路径。
- 真实隔离 MySQL 50 并发：1 row / same messageId；lost ACK retry、offline、same-instance、two remote devices、cross-instance、duplicate no-redelivery：PASS。
- 完整本地 MySQL/Redis/临时 etcd `go test ./... -count=1`、vet、gofmt、Secret Scan、Gateway/Presence regressions：PASS。
- Implementation commit：`6da24e4e161d5fc66c7d843eb676ff367e47f431`。
- 未实现 chat_sessions、unread、mark read、history API 或前端 socket manager。
- Race：NOT AVAILABLE；兼容环境/CI verification PENDING。

## Checkpoint 5 — Chat Session + Unread + Read State + History Cursor

- 时间：2026-10-11 09:21 +08:00。
- message insert、sender session 和 receiver session/unread 在同一 MySQL transaction；ACK/publish 均只发生在 commit 后。
- 两条 session 固定锁顺序，last message 只前进，receiver 在线/离线统一数据库 unread 原子 +1；重复 clientMsgId 不重复 session/unread/publish。
- 真实 MySQL 50 duplicate：1 message、unread +1、same messageId；50 distinct：50 messages、unread +50、last ID 最大且无 lost update。
- Mark Read 验证会话归属并单调推进 read cursor；partial/stale/latest/duplicate/concurrent new-message 均保持 unread 一致；不实现 peer read receipt。
- `GET /api/chat/session` 单 JOIN 返回 peer/last message/unread/read/top，数据库完成 top/time 排序；hidden 只影响当前用户，新消息恢复可见。
- `GET /api/chat/history` 使用 server message-ID cursor、默认 20/最大 100、oldest→newest 返回；当前用户 deletion 不影响 peer，个人删除不等于 recall。
- `PUT /api/chat/read` 与 `POST /api/chat/user_top` 使用 Gateway 可信身份；top 支持显式 set 和 legacy `friendId` atomic toggle。
- message/sender-session/receiver-session failure injection 全部回滚且无 ACK/publish；Redis publish after commit failure 保留 DB 与 ACK。
- 无 migration 变化；复用 Day 1 `003_chat` schema/index。Session list EXPLAIN 选择设计排序索引；history/unread 小数据 optimizer 选择更窄 PK/unique，同时列出设计 cursor 索引为 possible keys，未新增无意义索引。
- 非缓存全量测试、真实 MySQL/Redis/临时 etcd、CP4 Pub/Sub、Presence/Gateway、vet、gofmt、diff check 与 Secret Scan PASS；CP5 真实 MySQL 套件连续 3 次 PASS。
- Message/Session/MySQL coverage 71.1%，Chat HTTP transport 56.3%。
- Implementation commit：`3dac219ca23021aee96d0f4b824800e88b4914c4`。
- Race：NOT AVAILABLE；compatible environment/CI verification PENDING。

## Checkpoint 6 — Private Chat Frontend Integration

- 时间：2026-10-11 18:12 +08:00。
- 分层为 typed Chat API、无 DOM Chat Socket Client、Pinia Chat Store 和纯展示/交互组件；同一登录用户只有一条 Chat WS。
- 连接状态为 disconnected/connecting/connected/reconnecting；1/2/4/8/16/30 秒上限 + jitter，单 reconnect timer；logout/auth failure 会停止重连并清理状态。
- 断线发送进入内存 pending queue；UUID clientMsgId 在 ACK timeout/manual retry/reconnect retry 中保持不变，ACK 映射 server messageId 并转 sent，typed error 只失败对应消息。
- realtime/history/pending 以 messageId/clientMsgId reconcile；duplicate ACK/message 不重复 UI。ACK 仍只代表服务端事务提交，不是 delivery/read。
- session 以后端排序为准；其他 peer realtime 先本地 unread +1，再单 debounce refresh；当前 route 且页面 visible/focused 才自动 mark read。
- history 使用 beforeMessageID、oldest→newest merge、load older；路由为 `/chat/:peerId?`。页面安全插值 text，不使用 v-html，不持久化 history/pending。
- Web type-check、8 files / 35 tests、production build、Go 全量非缓存 test/vet、diff check、Secret Scan PASS；lint NOT CONFIGURED。
- 真实 MySQL/Redis/etcd/Gateway/Auth/User/Chat/Vite + 两个隔离 Chrome context：pending→sent、B realtime/unread/read、刷新恢复、receiver 双设备、20+ cursor pagination、Chat API 重启 reconnect 全部 PASS。
- 临时测试用户/好友/session/messages、进程、bin/log/etcd data 均精确清理，最终相关端口监听 0。
- Implementation commit：`80456b3e27e2ebc0dc56d03f17c1ee58cf0c6f6d`。
- Private Chat V1：FUNCTIONAL。Race 仍为 NOT AVAILABLE；兼容环境/CI verification PENDING。

## Checkpoint 7 — Private Chat Final Acceptance & Freeze

- 时间：2026-10-11 21:39 +08:00。
- Chrome 双用户双向消息、pending→ACK→sent、session/unread/read、刷新、双设备共享 read、20+ cursor history、Chat API 重启 reconnect、logout/revoked auth failure 与 HTML-like text 安全渲染全部 PASS。
- ACK lost 由前端 same-clientMsgId Store integration + 真实后端 lost-ACK/idempotency 联合证明；50 duplicate、50 distinct、partial/stale read、new-message/read race、hidden/deletion user scope、Redis multi-instance/degraded delivery 全部 PASS。
- 真实 MySQL/Redis/etcd 全量 Go suite PASS；核心 message/session 3 轮、Presence 3 轮、Hub/heartbeat/single-writer/shutdown 10 轮 PASS。
- Web type-check、8 files / 40 tests、production build PASS；lint NOT CONFIGURED。Go vet/gofmt/diff check 与 Secret Scan PASS。
- 临时用户、数据库数据、Presence contribution、进程、bin/log/etcd data 均清理；无测试端口残留。
- 无业务 Bug 或 production code 修改；仅增加验收测试和日志。DB/API/WS/Redis protocol 无变化。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Race Verification Pending compatible environment/CI。
- `PRIVATE CHAT V1 = COMPLETED`。Next Authorized Work：NONE。Next Planned：Day 3 — Group WS + File Service + User Web protocol adaptation（等待明确授权）。
