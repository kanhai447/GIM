# Day 2 / Checkpoint 7 — Private Chat Final Acceptance & Freeze

- **时间:** 2026-10-11 21:39 +08:00
- **开始状态:** `main`，HEAD/origin/main 均为 `85f504956dd50f6ba177208840f6fdacfd7c522c`，working tree clean。
- **范围:** 只做 Private Chat V1 regression、真实浏览器、失败场景、并发、协议、资源生命周期与安全验收；未进入 Group、File、Admin Chat 或 Day 3。
- **结论:** `PRIVATE CHAT V1 = COMPLETED`。无业务代码 Bug、数据库 migration 或 HTTP/WS/Redis 协议变化；仅增强验收测试与日志。

## E2E

- 使用本机 MySQL/Redis、隔离官方 etcd、User RPC、User/Auth/Chat API、Gateway、Vite 与预装 Chrome headless，两个独立用户 context + receiver 第二页面真实联调。
- A→B 首条消息观察到 sender `pending -> sent`；B realtime/session preview/unread=1，打开会话后 history 正确且 unread=0，刷新后 session/history/read state 恢复。
- A→B、B→A、A→B 双向顺序在两端一致；每条文本只显示一次，发送者/接收者样式正确，session last message 未倒退，sender 未增加自己的 unread。
- sender `sent` 仅由 `chat.ack` 驱动；UI 文案为“已发送”，没有 delivered/read 含义。
- `/chat/:peerId` 刷新后 Auth restore、Chat Socket、session、current peer 与 history 均恢复。
- 创建超过一页历史：首屏 latest 20，`beforeMessageID` 加载更早后总数 >20；oldest→newest、跨页无重复，realtime/pending/ACK 合并无重复。

## Reliability

- 前端 Store 测试模拟 ACK timeout/lost processing 后以同一 `clientMsgId` retry，最终 ACK 映射原 logical message，不新增 UI 行。
- 真实后端集成覆盖已持久化但 ACK 丢失后的 same-ID retry：DB 1 row、same messageId、receiver unread 只 +1、duplicate 不 republish；与前端 Store 证据联合验收 ACK lost 场景。
- 真实 MySQL 50 个相同 sender/clientMsgId：1 message、所有结果 same messageId、receiver unread +1、realtime once，PASS。
- 真实 MySQL 50 个不同 clientMsgId：50 messages、unread +50、无 lost update、last_message_id 为最大值，PASS。
- Redis publish failure 发生在事务提交后时，message/session 保留、sender ACK 成功、realtime degraded；history 可恢复，PASS。

## Reconnect

- Chrome 连接后强制停止并重启本轮 Chat API：页面捕获 `reconnecting`，服务恢复后自动 `connected`，随后再次发送/接收成功且无重复历史。
- Socket 单测确认同一时刻只有一个 reconnect timer，退避 1/2/4/8/16/30 秒并封顶 30 秒；本轮新增“成功连接后下一次故障回到 1 秒”断言。
- Logout 真实浏览器：先停止 Chat Store/socket，再调用 Auth logout，清除 local session 并返回 Login；等待超过一个 reconnect interval 未创建新 Chat WS。Chat 服务日志记录 active connection `reason=normal`。
- revoked token 真实浏览器：Auth logout 后保留页面旧凭据并 reload，可能出现一次初始握手，但随 profile/handshake 失败清空 session、进入 Login，之后 socket 数稳定无 401 reconnect loop。expired(1205) 与 revoked(1206) HTTP code 均新增 Store 终止重连测试。

## Multi-device

- Receiver B1/B2 同时在线，A 新消息在 B1 会话和 B2 session 各出现一次，无重复。
- B1 当前 visible/focused 会话 Mark Read 后，B2 reload session 看到共享 MySQL unread=0；未要求主动 read-state WS 同步。
- 页面 inactive 时收到 current-peer message 不自动 Mark Read；active 时自动读到 latest。后端 partial/stale read 与 new-message/read race 均回归 PASS。

## Presence

- Hub/heartbeat 回归确认 B1+B2 online、关闭/timeout 一个 sibling 仍 online、最后连接离开才 offline。
- 真实 Redis multi-instance/TTL 测试连续 3 轮 PASS；正常 shutdown 清 contribution，crash 由 TTL 淘汰且其他 instance contribution 保持在线。
- 浏览器 context 全部关闭后，临时用户 `gim:presence:user:{uid}` 的 ZCARD 均为 0；Chat restart/reconnect 后 contribution 可重新建立，无 stale 测试 contribution。

## Session

- session peer nickname/avatar、preview、last time、unread/read、top 与后端 top/time 排序回归 PASS；不为最终验收新增 hidden UI。
- hidden 仅作用当前用户，新消息恢复可见；peer session 不受影响。
- `chat_message_deletions` 仅过滤当前用户 history，不等于 recall，peer history 保留。
- partial read（读到倒数第二条保留 unread=1）、stale read 不倒退、latest read 清零、并发 new-message + mark-read 不误清新未读，PASS。

## Multi-instance

- Redis Pub/Sub 同实例/跨实例统一 delivery path；receiver 每个 active Client 一次，origin sender sibling 不获 ACK，duplicate clientMsgId 不 republish。
- Presence 使用按 instance 的 ZSET contribution；Delivery 使用集中 channel helper `gim:chat:delivery`，无散落硬编码 key。
- Gateway/Auth/Chat tunnel 全量回归 PASS：valid query JWT Upgrade 成功；missing JWT、logout/revoked JWT 拒绝；伪造 User-ID/Role 不可信；Chat 未维护第二套 JWT verifier。

## Security

- Origin 使用精确 allowlist；development Origin allowed、非 allowlist/missing Origin rejected；未出现 `CheckOrigin=true`。
- 生产 Chat websocket 搜索结果中，数据帧与 Ping/Pong control 只在 `client.writePump` 写；无第二业务写协程。
- HTML-like text `<b>hello-…</b>` 在 Chrome 与组件测试中均显示为字面文本，DOM 内无 `b` 元素；`web/src` 无 `v-html`。
- lifecycle 日志仅含 user ID、opaque client ID 和 reason；不记录 JWT、Authorization、password 或 message content。
- Repository safety 与 tracked secret pattern scan PASS；临时凭据只存在未跟踪 `.env.local`/进程环境，未写 operation log。

## Stability

- `go test ./tests -run '^(TestChatMessageMySQLAndWebSocketReliability|TestChatSessionUnreadReadAndHistoryMySQL)$' -count=3`：PASS。
- `go test ./tests -run '^(TestChatPresenceRedisMultiInstanceTTL|TestChatPresenceServicesRecoverCrashAndPreserveOtherInstance)$' -count=3`：PASS。
- Hub/heartbeat/single-writer/shutdown 9 项 focused tests `-count=10`：PASS。
- 最终真实环境 `go test ./... -count=1`：PASS；首次运行时因本轮真实 `user_api` 仍注册在同一 etcd，missing-service case 得到真实服务响应（200/code 2207）。停止应用注册后重跑全量 PASS，确认是测试环境互扰而非产品 Bug。
- `go vet ./...`：PASS；`gofmt -l .`：PASS；`git diff --check`：PASS。
- Web `pnpm type-check`：PASS；`pnpm test`：8 files / 40 tests PASS；`pnpm build`：PASS（1719 modules）。`lint: NOT CONFIGURED`。

## Lifecycle / Resource Audit

- readPump、唯一 writePump、heartbeat ticker、Presence worker、Redis subscriber、frontend reconnect/ACK/session timers 与 server shutdown 均有 owner/stop path；focused shutdown tests 无 hang。
- 本轮 fixture 用户在确认 ID/account 后事务清理；好友、messages、sessions、deletions、config 与 users 均清除。
- 浏览器、GIM 服务、Vite、隔离 etcd、临时 bin/log/data 已清理；Presence test key 无残留，相关临时端口无监听。

## Reference / Independent Implementation

- 本轮只读复核 FIM 的私聊路径、双向消息/session/history/top 与多设备行为；没有复制 reference 代码或新增 reference 依赖。
- 最终审计继续避免 FIM 的普通全局 map、并发直写 Conn、缺少 writePump/Ping-Pong、宽松 Origin 与不清晰多设备 Presence；GIM 依靠 Hub event loop、单 writer、typed ACK、MySQL 幂等/session/read cursor 与 Redis instance contribution 自主实现。

## Known Limitations

- **Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Race Verification Pending。** 本机 MinGW.org GCC 6.3.0 `mingw32/i586` 不支持 windows/amd64 race，必须在兼容 64 位 C 工具链或 CI 执行 `go test -race ./...`。
- pending queue 仅在内存；未 ACK pending 不跨浏览器刷新。已持久化消息由 history 恢复，不引入 IndexedDB。
- V1 ACK 只有 server persist/transaction commit 语义；无 delivery receipt、无 read receipt。
- 当前 Private Chat V1 只实现 text message；图片/文件/语音/撤回/回复等属于后续明确阶段。

## Files / Protocol / Database

- Tests：`web/e2e/private-chat-smoke.mjs`、`web/src/service/ws/chat_socket.test.ts`、`web/src/stores/chat.test.ts`、`web/src/components/chat/chat_components.test.ts`。
- Logs/state：本 operation、`agent_logs/CURRENT_STATE.md`、`agent_logs/day-2.md`、`agent_logs/AGENT_WORKLOG.md`。
- Database migration：无。HTTP/WS/Redis protocol：无。业务 production code：无修改。

## Git / Freeze

- Test commit：`8e8d69483113aee237ac7fda928353a0573e2a8f` — `test(chat): complete private chat v1 acceptance`。
- Documentation/state commit：`docs(day2): finalize private chat v1 acceptance`（实际 hash 以最终 Git 复核为准）。
- Push：两个提交完成后推送 `origin/main`，以最终 equality/clean 复核为准。
- Day 2 当前计划目标已完成；`Next Authorized Work: NONE`。
- Next Planned：Day 3 — Group WS + File Service + User Web protocol adaptation；必须等待用户明确授权。
