# Day 2 / Operation 005 — Private Message Persistence + clientMsgId + Server ACK

- **时间：** 2026-10-10 18:42 +08:00
- **状态：** COMPLETED
- **范围：** 只实现文本私聊的 typed inbound、接收者/好友/聊天限制校验、MySQL 持久化、clientMsgId 幂等、Server ACK 和 best-effort receiver realtime delivery；未实现 session/unread/read/history/frontend/Group/File。

## 开始状态

- Branch：`main`。
- 开始 HEAD：`6c9b1d4f722eb47df060c334cddf6803913dc278`。
- `git fetch origin`：SUCCESS。
- 开始时 `HEAD == origin/main`：PASS。
- Working tree：CLEAN。
- Chat WS Foundation、Heartbeat/Lifecycle、Presence：COMPLETED。

## Protocol

- Client -> Server：`chat.send`，沿用文档 envelope 顶层 `event/requestId/clientMsgId/data`；data 为 typed `revUserID + msg`，没有任意 `map[string]any`。
- Checkpoint 4 只接受 `msg.type=1` 文本及 `textMsg.content`；senderID 不在输入 DTO，唯一来源是 Gateway 注入身份绑定的 `Client.UserID`。
- Server -> originating Client：`chat.ack`，包含同一 `clientMsgId`、server `messageId`、`createdAt`。
- Server -> receiver Clients：`chat.message`，包含 messageId、sendUserID、revUserID、clientMsgId、typed msg、createdAt；server messageId 是持久身份/排序游标。
- 失败：`error` envelope 使用稳定 `code/message`，相关消息保留 `clientMsgId`；不暴露 SQL、DSN、RPC endpoint、stack 或 raw internal error。
- 单次 malformed JSON 返回 typed error 后可继续处理合法 frame；readPump 不直接写 Conn，所有出站仍为 Hub -> bounded Client.Send -> unique writePump。

## Validation / security

- `clientMsgId` 必填，1–64 字符，只允许字母、数字、`.`、`_`、`:`、`-`；服务端不替客户端生成。
- receiverID > 0、sender != receiver、只允许 text、UTF-8 正文非空、默认正文 <= 4096 bytes、typed payload <= 16384 bytes。
- 接收者存在/启用状态通过现有 User RPC `GetUserByID`，使用 WebSocket caller context 派生的 2s timeout；没有业务路径 `context.Background()`。
- 按 docs 执行 sender `user_confs.curtail_chat` 与好友关系校验；好友查询兼容正式 A->B/B->A 双行模型，任一方向存在即可判定。
- 日志只记录 messageId/senderID/receiverID 等安全元数据；不记录正文、JWT、Authorization、Cookie 或凭证。

## Persistence / repository / service

- 新增正式 `Message.Repository`：`Create`、`GetBySenderAndClientMsgID`、`GetByID`；Repository 只处理 DB，不依赖 WS/Hub/ACK。
- `Message.Service.SendPrivateMessage` 集中参数、限制、接收者、好友、幂等和持久化语义，transport 根据 `CreatedNew` 生成 ACK/delivery。
- MySQL GORM Repository 映射现有 `chat_messages`，写入 msg_type/msg_preview/typed JSON；没有 migration 变化。
- 当前 CP4 ACK 基于 `chat_messages` 成功持久化。`chat_sessions` 行数验收为 0；CP5 引入 session 后必须把 message + sender session + receiver session/unread 纳入同一事务。

## Idempotency

- 最终保障是现有 MySQL UNIQUE `(send_user_id, client_msg_id)`，不是只靠 `SELECT -> INSERT`。
- 正常路径可先查原消息；并发 miss 后各自尝试 INSERT，MySQL 1062 映射为 duplicate，再按 sender/clientMsgId 查询原消息。
- 同一 sender + clientMsgId + 同一逻辑 payload 返回同一 messageId；同一 key 被复用于不同 receiver/type/content 返回稳定 idempotency conflict。
- 真实隔离 MySQL 50 goroutine 并发同 key：数据库恰好 1 行，所有 50 个成功结果的 messageId 相同，PASS。

## ACK

**ACK = 服务端已经成功接受并持久化该消息。**

ACK 不代表 receiver 在线、收到、展示或已读；不是 Delivery ACK，也不是 Read Receipt。ACK 只排队到发起 `chat.send` 的 Client，不广播到 sender 其他设备。若 sender ACK queue 已满，现有 slow-client policy 可关闭连接；消息已在 DB，重连以同 clientMsgId 重试可恢复同一 ACK。

## Delivery / multi-instance

- receiver realtime delivery 与 ACK 分离；receiver 离线不拒绝、不回滚，sender 仍收到 persist ACK。
- 新消息统一发布集中 helper `rediskeys.ChatDeliveryChannel()` -> `gim:chat:delivery`；所有 Chat 实例订阅，只有持有 receiver 本地 Client 的实例调用 `Hub.SendToUser`。
- 同实例也只走 Pub/Sub，不额外本地直投，因此不会出现 local + bus 双投；同用户所有本地 Chat Clients 各排队一次。
- Pub/Sub 是 best-effort，不是消息存储；publish timeout/failure 不撤销 DB 或 ACK，只记录安全 degraded 元数据。可靠性事实源是 MySQL，后续 history 恢复。
- receiver slow/backpressure 继续复用 Hub 非阻塞 drop/disconnect policy，不阻塞 sender ACK 或 Hub 其他用户。

## Retry / duplicate delivery

- 模拟首次持久化后 ACK 丢失，再次调用相同 clientMsgId：`CreatedNew=false`、同 messageId、ACK success，PASS。
- 只有 `CreatedNew=true` 才 publish receiver event；duplicate retry 只重发 sender ACK，因此 DB 一行且 receiver UI 不会因 GIM routing 再收到第二条。
- 真实 WebSocket 重复发送相同 frame：sender 两次 ACK messageId 相同，三个 receiver connections 在第二次均无重复 event，PASS。

## Online / offline / multi-device / multi-instance results

- Offline receiver：DB persist PASS，sender ACK PASS，receiver realtime delivery none，发送不失败。
- Online same-instance receiver：收到一次 `chat.message`，messageId 与 ACK 相同。
- Receiver two clients on second instance：两个 Client 各收到一次，未随机只投一个、未重复。
- Instance A sender + Instance B receiver：Redis publish -> B subscriber -> B Hub -> writePump，PASS。
- Sender sibling device：未收到 originating Client 的 ACK，PASS。
- Malformed frame -> typed error -> same connection subsequent valid chat.send，PASS。

## FIM comparison

- 只读参考 FIM 的私聊请求形状、文本 msg JSON、好友/curtail_chat 校验、消息预览与 receiver 推送行为；未复制文件、handler 或 DB 实现。
- FIM Chat 使用 package 全局普通 map、多路径直接写 Conn、普通 Create 后直接推送，缺少 clientMsgId UNIQUE 幂等/正式 persist ACK，重试与并发可靠性边界不清晰。
- GIM 自主实现 typed DTO、可信 sender、Service/Repository 分层、DB UNIQUE 最终幂等、originating-client ACK、single writePump、createdNew 去重投递和 Redis 跨实例 best-effort fanout。

## Tests

- `go test ./... -count=1`：PASS。
- 设置本地安全 `GIM_ENV_FILE`，临时单节点 etcd + 本地 MySQL/Redis 的完整 `go test ./... -count=1`：PASS；`server/tests` 包含 migration/Auth/Gateway/Presence/Chat message real integrations，PASS（9.598s）。
- 真实 MySQL Repository：create/query by key/query by ID/duplicate rejected/cleanup，PASS。
- 真实 MySQL 50-way concurrent idempotency：1 row / one messageId，PASS。
- 真实 Redis + 两 Chat instances + 5 个 sender/receiver WS connections：ACK、same/cross-instance、multi-device、offline、duplicate no-redelivery、sender sibling isolation，PASS。
- lost ACK retry：same messageId，PASS。
- Gateway/Auth/Chat WebSocket tunnel：临时 etcd 下 PASS。
- Presence real Redis multi-instance/TTL regression：PASS。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS（无输出）。
- `git diff --check`：PASS。
- Message service coverage：**69.5% statements**；Chat User RPC adapter：**71.4% statements**。
- `scripts/check-repository-safety.ps1` / Secret Scan：PASS。
- 临时 etcd/data 与隔离 MySQL test database 均已清理；`etcd_process=NONE`。

## Race

**Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT。**

按要求未重复运行已知必然在 Windows cgo/runtime 阶段失败的 `go test -race ./...`。本轮包含 Hub、Redis subscriber 与并发 duplicate，故继续明确：

**Race Verification Pending：必须在兼容 64 位 C 工具链或 CI 环境执行 `go test -race ./...`。**

## 数据/协议变化

- Database migration：无；使用既有 `chat_messages` 及 UNIQUE；未写 `chat_sessions`/unread/read state。
- User RPC schema：无变化；复用 `GetUserByID`。
- WebSocket：正式启用 `chat.send`、`chat.ack`、`chat.message`、`error`。
- Redis：新增集中 Pub/Sub channel helper `gim:chat:delivery`，无持久 key/value。
- HTTP/history/frontend/Group/File：无变化。

## 修改文件

- `.env.example`
- `docs/00_DECISIONS.md`、`02_BACKEND_DESIGN.md`、`06_WEBSOCKET_PROTOCOL.md`、`07_CORE_FLOWS.md`、`08_REDIS_CONFIG.md`、`10_TEST_ACCEPTANCE.md`
- `server/cmd/chat-api/main.go`
- `server/internal/chat/client.go`、`module.go`、`config/**`、`protocol/**`
- `server/internal/chat/message/**`
- `server/internal/chat/delivery/**`
- `server/internal/chat/userclient/**`
- `server/internal/platform/rediskeys/**`
- `server/tests/chat_message_integration_test.go`
- `agent_logs/operations/day-2/005_checkpoint-4_private-message-ack.md`
- `agent_logs/CURRENT_STATE.md`、`agent_logs/day-2.md`、`agent_logs/AGENT_WORKLOG.md`

## Git / 下一步

- Implementation commit：`6da24e4e161d5fc66c7d843eb676ff367e47f431 feat(chat): add idempotent private messaging and server ack`。
- Log/status commit：本记录所在 `docs(day2): record private message reliability checkpoint` 提交。
- Push：两个提交完成后推送 `origin/main`，最终结果以 Git 复核为准。
- Day 2 blocker：NONE（race 是兼容环境验收缺口，不阻塞下一 Checkpoint）。
- Next Checkpoint：**Chat Session + Unread + Read State**（等待下一条明确指令）。
- 本操作到此停止，不进入 session/unread/read/history/frontend/Group/File。
