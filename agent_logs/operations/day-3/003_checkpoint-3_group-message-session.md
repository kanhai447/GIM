# Day 3 / Operation 003 — Group Message Reliability + Session / Unread / History

- **时间：** 2026-10-12 22:32 +08:00
- **状态：** COMPLETED
- **范围：** 仅 Day 3 / Checkpoint 3；文本群消息、幂等 ACK、事务 session/unread、read/history 和 Redis 多实例实时投递。不含 Group Frontend、File、Admin 或 Day 4。

## 开始状态

- Branch：`main`。
- HEAD / `origin/main`：`a2691b85c48ed6296623c379bcd33743494abf3a`，working tree clean。
- Day 1、Day 2、Private Chat V1、Day 3 Checkpoint 1/2 均为稳定完成状态。
- 已完整复读 `AGENTS.md`、`START_AGENT.md`、`FINAL_AGENT_INSTRUCTION.md` 与 `docs/00`–`16`，并核对 CP1/CP2、私聊消息/session 操作记录。

## Reference 与自主实现

- 只读参考 FIM 的群 WS 请求形状、文本消息 JSON、member/全员禁言/个人禁言检查、`/api/group/history/:id`、`/api/group/session` 与群消息预览行为。
- 未复制 FIM 文件、handler、全局连接 map、直接 Conn.Write 或动态消息表聚合实现。
- GIM 自主实现 typed protocol、可信连接 sender、Service/Repository/Inbound 分层、DB UNIQUE 最终幂等、消息+成员 sessions 单事务、显式 unread/read cursor、用户维度 deletion、单 writePump 与独立 Redis 跨实例 fanout。删除 `reference/` 后仍可独立构建测试。

## 协议与校验

- 启用 `group.send` / `group.ack` / `group.message` / `error`；沿用顶层 `event/requestId/clientMsgId/data`。
- 当前只接受 `type=1` 文本：clientMsgId 1–64 安全字符，UTF-8 非空、正文最多 4096 bytes、typed payload 最多 16384 bytes。
- sender 唯一来源是 Group Client.UserID；不接收客户端 sender ID。
- 发送前复用 CP1 Group Service 检查 active group、membership、all-mute、membership-ID-scoped Redis mute。owner/admin 不受 all-mute，个人 mute 仍生效。
- stable error code：2403–2411；不返回 SQL、DSN、Redis endpoint 或内部 cause。

## Persistence / Idempotency / Transaction

- 复用既有 `004_group` schema，无 migration 变化。
- Repository transaction 使用 active group shared lock，允许并发发送并阻止 dissolve 跨越持久化边界；事务内再次确认 sender membership、插入 group message、取得当前成员快照并按 user ID 顺序批量 upsert sessions。
- sender unread +0；其他事务时成员 online/offline 均数据库原子 +1；last message 只向更大 ID 推进，保留 top/read cursor，清除 hidden。
- `(send_user_id,client_msg_id)` UNIQUE 是最终屏障。50 个同键请求收敛为一行/同 messageId/unread +1；50 个不同键全部落库、无 lost increment、last message 不回退。
- MySQL REPEATABLE READ 下唯一冲突事务可能看不到刚提交胜者；实现明确结束失败事务，再在新事务读取胜者。重复不更新 session、不发布；不同逻辑 payload 返回 2407。

## ACK / Delivery

- ACK 仅在 message + 全体 required sessions transaction commit 后排队到 originating Client；duplicate retry 重发同一 ACK。
- ACK 只表示 Server persist/accept，不代表成员在线、收到、展示或已读。
- 新消息发布独立 `gim:group:delivery`，携带 transaction-time target user IDs；所有 Group API instance 订阅并只路由本实例 Clients，同实例没有额外直投。
- sender originating Client 收 ACK + `group.message`，sender sibling 只收 `group.message`；receiver 多设备/跨实例各一次。前端后续按 messageId/clientMsgId merge。
- Redis publish 失败发生在 commit/ACK 后：DB/session/ACK 保留，仅安全记录 degraded；history 是恢复路径。

## Session / Read / History

- `GET /api/group/session?limit=`：有效 membership + active group，一次 JOIN groups/last message，无 N+1；top/time/id 排序。
- `PUT /api/group/read`：member-only，目标 message 必须属于 group；锁 session、cursor 单调、按 cursor 后他人未删除消息重算 unread。partial/latest/stale/duplicate 与并发 send 语义通过。
- `GET /api/group/history/:id?beforeMessageID=&limit=`：member-only、message-ID cursor、oldest→newest、cursor group isolation、当前用户 deletion isolation。退出/被踢/解散通过 Group Service active/member boundary 拒绝。

## Failure Injection / Concurrency / Presence

- 真实 MySQL trigger 注入 group_message insert failure：事务无消息/session mutation，无 ACK/Publish。
- 注入 group_session batch insert failure：message 与此前 session updates 全回滚，无 ACK/Publish。
- 注入 commit 后 Redis publish failure：消息/session 保留，originating ACK 保留。
- 真实 Redis 两个 subscriber + 实际两套 Group Hub/WS server：跨实例 sender sibling、receiver 双设备各收一次；origin 收 ACK + message。
- Group delivery、WS 生命周期、mute 均未创建/修改/删除 `gim:presence:*`；Presence 仍只属于 Chat WS。

## Tests / Quality

- `go test ./tests -run TestGroupMessageReliabilitySessionsHistoryAndDelivery -count=3`：PASS。
- 真实 MySQL：50 duplicate、50 distinct、all/personal mute、member/leave boundary、session JOIN、partial/stale/latest read、两页 cursor、deletion isolation、failure injection、三类 EXPLAIN：PASS。
- 真实 Redis + actual WebSocket：两 instance、sender sibling、receiver two devices、publish degraded、Presence negative：PASS。
- Group foundation + Private Chat focused regressions：PASS。
- 临时 etcd 下 `GIM_ENV_FILE=[REDACTED] go test ./... -count=1`：PASS（`server/tests` 14.266s）。
- `go vet ./...`、`gofmt -l .`、`git diff --check`、repository Secret Scan：PASS。
- 新 Group message/delivery/protocol focused coverpkg：69.1%；包含全部 Group foundation/WS 的本次 integration-only coverpkg：32.3%（非本轮旧包未执行导致，不作为旧 checkpoint coverage 回退）。
- Race：NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；已知 32-bit MinGW 不支持 windows/amd64 race，compatible environment/CI PENDING，按既有结论未重复运行。
- 临时 etcd/coverage 文件已精确清理；2379/2380 无监听残留。

## Query Plans

- group session：`ix_group_sessions_user_sort`。
- group history：`ix_group_messages_group_cursor`。
- membership：`ux_group_members_group_user`。
- session upsert：`ux_group_sessions_user_group` 由 UNIQUE duplicate-key 路径使用。

## 修改文件

- `server/internal/group/message/**`
- `server/internal/group/delivery/redis.go`
- `server/internal/group/ws/{module.go,protocol/envelope.go}`
- `server/internal/platform/rediskeys/keys.go`
- `server/cmd/group-api/main.go`
- `server/tests/group_message_integration_test.go`
- `docs/{02_BACKEND_DESIGN,04_DATABASE_DESIGN,05_HTTP_API,06_WEBSOCKET_PROTOCOL,07_CORE_FLOWS,08_REDIS_CONFIG}.md`
- 本 operation、`agent_logs/CURRENT_STATE.md`、`agent_logs/day-3.md`、`agent_logs/AGENT_WORKLOG.md`

## Git / 下一步

- Implementation commit：`547afe835f84760adf9733f1661050706ef8b32f feat(group): add reliable group messaging and sessions`。
- Log/status commit：本记录所在 `docs(day3): record group message checkpoint`。
- Push：implementation 与本 log/status commit 已 push `origin/main`；最终复核 `HEAD == origin/main` 且 working tree clean。
- 下一入口：Group Frontend Integration；需用户另行授权。本轮停止，不进入 Group Frontend、File、Admin 或 Day 4。
