# Day 2 / Checkpoint 5 — Chat Session + Unread + Read State + History Cursor

- **时间:** 2026-10-11 09:20 +08:00
- **开始状态:** `main`，HEAD/origin/main 均为 `fdd114628c709af2d44ce86d843df6c4f4227824`，working tree clean。
- **目标:** 在不进入前端、Group 或 File 的前提下，将私聊消息与双方 session/unread 收敛到同一事务，并提供 session list、mark read、置顶与 message-ID cursor history。

## Transaction

- `PersistWithSessions` 在同一 MySQL transaction 中执行 message INSERT、sender session upsert、receiver session upsert；任何一步失败均回滚。
- 两条 session 按 `user_id` 固定顺序 upsert，降低 A→B 与 B→A 并发时的锁顺序风险。
- `last_message_id/last_message_at` 仅在新 message ID 更大时推进；receiver `unread_count = unread_count + 1` 使用数据库原子表达式，不做 Go 层 read-modify-write。
- 事务提交后 Service 才返回，Inbound 随后排队 ACK，再执行 Redis best-effort publish；ACK 现在明确表示 message + required session state 已提交。
- Day 1 的 `003_chat` 已包含所需表、字段、unique 和索引，本轮未创建无意义的 `007` migration，也未修改既有 migration。

## Sender / Receiver Session 与 Unread

- Sender session 更新 last message，unread 不增加，并保留 `is_top`、`last_read_message_id` 等用户状态。
- Receiver 无论 online/offline 都原子 unread +1；Presence 只控制 realtime fanout，不控制 unread。
- 新消息会将双方对应 session 的 `hidden_at` 清空，采用“新消息恢复可见”的 V1 语义。
- Session list 通过一次 `chat_sessions + users + chat_messages` JOIN 获取 peer 基础资料和 `last_message_id` 对应消息，不做逐 session User RPC/Message query。
- 排序在数据库中使用 `is_top DESC, last_message_at DESC, id DESC`；显式 pin/unpin 只修改当前用户。`friendId` only 的旧请求使用数据库原子 toggle 兼容参考前端。

## Idempotency

- `(send_user_id, client_msg_id)` UNIQUE 仍是最终幂等屏障。
- 并发 duplicate INSERT 命中唯一冲突后在 transaction 中读取原消息并返回 `createdNew=false`；不再次更新双方 session、不重复 unread、不再次 publish。
- 真实 MySQL 50 concurrent same clientMsgId：message=1、receiver unread +1、双方 session 指向同一 message ID、所有调用返回同一 server message ID，PASS。
- CP4 lost-ACK retry、offline ACK、same/multi-instance Redis delivery、duplicate no-redelivery 回归 PASS。

## Read State

- `PUT /api/chat/read` 只接受可信 Gateway `User-ID`；客户端仅提交 `peerUserID/lastReadMessageID`。
- transaction 先验证 read-through message 属于当前两人会话，再锁定当前用户 session；cursor 取 existing/requested 的最大值，永不倒退。
- unread 重算为 cursor 之后 peer→current user 的未删除入站消息数。读到部分历史保留剩余 unread，读到最新为 0。
- session 行锁与 send upsert 串行化：若新消息尚未提交，mark read 后该 send 再 +1；若新消息先提交，mark read 的 count 可见它。不会出现 cursor 停在旧消息但新消息被错误清零。
- Mark Read 是用户级 MySQL state，不是 peer read receipt；本轮没有新增 WS delivery/read ACK 或 UI。

## Sessions / Hidden / Top

- `GET /api/chat/session?limit=` 默认 20、最大 100，返回 peer `userID/nickname/avatar`、last message、last IDs/timestamps、unread、read cursor、isTop 和 count。
- `hidden_at` 只作用于 `(current user, peer)` 行；测试确认 A hide 不影响 B，A/B 后续新消息使对应 session 恢复可见。
- send/upsert 保留 isTop；显式 `isTop` set 与兼容 `friendId` toggle 均为当前用户范围。

## History

- `GET /api/chat/history?peerUserID=&beforeMessageID=&limit=`；兼容参考字段 `friendId`。
- 首次取最新 N 条，后续 `id < beforeMessageID`；DB DESC 取页，API oldest→newest 返回；默认 20、最大 100，并返回 `hasMore/nextBeforeMessageID`。
- cursor 必须属于当前用户与 peer 的会话；第三方会话 cursor 被拒绝。
- `chat_message_deletions` 使用 `NOT EXISTS` 且限定当前 user ID；A 隐藏一条历史不影响 B。个人删除不等于 recall，本轮未实现撤回。

## Failure Injection

- 真实 MySQL trigger 注入 message INSERT failure：message=0、sessions=0、无 ACK、无 publish，PASS。
- 注入 sender session failure：message 与两条 session 全回滚、无 ACK、无 publish，PASS。
- 注入 receiver session failure：已执行的 sender session 与 message 一并回滚、无 ACK、无 publish，PASS。
- Redis publisher 在 commit 后返回错误：message/双方 session 保留、sender ACK 成功、realtime 记录 degraded，PASS。

## Concurrency / Multi-device / Multi-instance

- 真实 MySQL 50 distinct clientMsgId：50 messages、receiver unread 完整 +50、last_message_id 为最大 server ID，无 lost update，PASS。
- partial/latest/stale/duplicate mark read 与 concurrent new-message + mark-read 均 PASS；整套真实 MySQL CP5 测试连续 3 次 PASS。
- session/read/history 事实源均为 MySQL；不同设备/实例读取相同用户级 state。前端/WS 不主动广播 read state，留到后续集成。

## FIM Comparison

- 参考了 FIM 的 `/api/chat/history`、`/api/chat/session`、`/api/chat/user_top` 路径、双向会话历史、批量用户信息意图与个人删除行为。
- 未复制 FIM 的临时消息表聚合、OFFSET 深分页、独立 top 表或业务代码。
- GIM 显式 session 表用于降低动态聚合复杂度、支持 unread/read/top/hidden 并统一用户维度状态；没有声称 FIM 使用错误的 `MAX(msg_preview)`，实际参考实现使用按时间取最新 preview 的子查询。
- GIM history deletion 始终绑定当前 user，避免用户维度过滤遗漏影响他人视图。删除 `reference/` 后 GIM 仍可独立构建、测试和运行。

## HTTP / WS / DB 变化

- HTTP 新启用：`GET /api/chat/session`、`GET /api/chat/history`、`PUT /api/chat/read`、`POST /api/chat/user_top`。
- WS event/payload 无新增；`chat.ack` 语义收紧为 message/session transaction committed，仍不代表 delivered/read。
- DB schema/migration 无变化；开始正式使用既有 `chat_sessions` 与 `chat_message_deletions` 查询语义。
- Redis key/协议无变化；publish 继续只在 commit 后且为 best-effort。

## Tests

- `go test ./... -count=1`：最终代码包级非缓存测试 PASS。
- 设置安全 `GIM_ENV_FILE`，临时官方 etcd + 本地 MySQL/Redis 的 `go test ./... -count=1`：PASS；`server/tests` 10.382s，含 Gateway/Auth/Chat tunnel、Presence、Pub/Sub 与 migration 回归。
- `go test ./tests -run TestChatSessionUnreadReadAndHistoryMySQL -count=3`：PASS（3.685s）。
- 真实 MySQL CP5 覆盖率：Message/Session Service + MySQL Repository 71.1%；HTTP transport 56.3%。
- `go vet ./...`：PASS。
- `gofmt -l .`：PASS（无输出）。
- `git diff --check`：PASS。
- Repository Secret Scan：PASS；敏感配置仅显示 `[REDACTED]`。
- EXPLAIN：session list 实际选择 `ix_chat_sessions_user_sort`；小数据集 history/unread 分别选择 PK/unique 的窄扫描，但设计索引 `ix_chat_messages_conversation_cursor` / `ix_chat_messages_receiver_sender_cursor` 均列入 possible keys。按文档不因小样本 optimizer 选择新增索引。

## 问题与修复

1. 初次聚焦集成命令使用相对 `GIM_ENV_FILE`，测试包工作目录无法找到文件；改用解析后的绝对路径后真实测试执行。
2. Session JOIN 初版将未导出的嵌入结构直接 Scan，基础字段未映射；改为 exported embedded field 后真实 session list 测试通过。
3. 首次全量基础设施回归因本机 etcd 未启动而 4 项连接失败；启动忽略目录中的官方临时 etcd 3.6.10 后全量 PASS。测试后进程和临时 data 已清理；首次删除 binary 遇到 Windows 句柄释放延迟，等待 2 秒重试后清理成功。
4. Race 按 Pre-Day2 结论未重复浪费时间：`Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`；兼容环境/CI 验证仍 PENDING。

## 修改文件

- `server/internal/chat/message/{repository.go,service.go}`
- `server/internal/chat/message/repository/mysql/{repository.go,session.go,history.go}`
- `server/internal/chat/message/transport/http/{handler.go,handler_test.go}`
- `server/cmd/chat-api/main.go`
- `server/tests/{chat_message_integration_test.go,chat_session_integration_test.go}`
- `docs/{02_BACKEND_DESIGN,04_DATABASE_DESIGN,05_HTTP_API,06_WEBSOCKET_PROTOCOL,07_CORE_FLOWS,10_TEST_ACCEPTANCE}.md`

## Git

- implementation commit: `3dac219ca23021aee96d0f4b824800e88b4914c4` — `feat(chat): add sessions unread and read state`
- log/status commit: 本 operation/CURRENT_STATE/day-2/worklog 所在提交 — `docs(day2): record chat session checkpoint`
- push: 两个提交完成后 push `origin/main`；实际输出与最终 HEAD equality 由本轮最终 Git 复核记录。

## 下一步

- 完成本轮两次 commit/push 与最终 clean/equality 核对后停止。
- 下一授权入口：Private Chat Frontend Integration；本轮不开始 frontend socket/reconnect/pending/unread UI、Group WS 或 File。
