# Reliability and Performance Interview Questions

## What exactly does ACK mean?

ACK 表示服务端已经成功接受并持久化消息。它不表示接收端在线、收到或已读。这个边界让客户端能可靠判断是否应该以相同 `clientMsgId` 重试，同时避免虚构端到端保证。

## How do you handle an ACK lost after commit?

客户端重发同一个 `clientMsgId`。数据库的 `(sender_id, client_msg_id)` 唯一键阻止重复插入，Repository 读取原行并返回一致 ACK。若客户端错误地用同一 ID 发送不同目标或 payload，服务端拒绝语义冲突。

## Can a message be committed but not delivered live?

可以。进程可能在 commit 后、ACK/广播前终止，或者接收方不在线。数据库 history、session 和 unread 仍是事实来源；重试不会重复写入。要缩小跨系统窗口，可引入 transactional outbox，但 V1 没有宣称 realtime exactly-once。

## What if Redis Pub/Sub loses a delivery?

Pub/Sub 只承担实时通知。消息在发布前已经写入 MySQL，并且 session/unread 已提交；接收方重连后通过 session 和 cursor history 恢复。V1 接受实时通知可丢，不把 Redis 包装成可靠队列。

## Why not claim exactly-once?

网络会让“对方是否收到响应”不可判定，commit 与实时推送也跨系统。项目提供的是 at-least-once retry + server-side idempotency，得到“数据库只保留一个逻辑消息”；receiver receipt 和跨系统 exactly-once 需要额外协议与成本。

## Why one writePump per WebSocket?

Gorilla WebSocket 不允许多个并发 writer 安全写同一连接。让所有业务把 frame 送入有界 queue，由 `writePump` 串行写，可以避免竞态、交错 frame 和 goroutine 阻塞扩散。Hub 也不直接写 socket。

## How are slow consumers handled?

send queue 有固定容量。消费者持续跟不上时，系统选择关闭并清理连接，让它通过重连和历史恢复，而不是无限缓存拖垮整个 Hub。这是可用性与单连接完整性之间的明确取舍。

## How are unread counts kept correct?

消息行和 session 更新在同一事务内；发送者不增加未读，其他参与者递增。read 操作采用单调游标，旧的并发 read 不能把状态回退。测试覆盖 partial/stale read、新消息与 read 竞态、隐藏恢复和用户范围删除。

## What did the soak test find?

私聊高压 soak 暴露 MySQL 1213 deadlock。修复没有放宽事务或吞掉错误，而是仅识别 1213，最多重建事务 3 次，使用 10/20 ms context-aware backoff。修复后完整 5 分钟 Private 和 Group soak 均无错误、重连或 drop。

## How should performance numbers be presented?

说明环境、负载、正确性断言和延迟，不把“最大已测”说成容量上限。当前可引用：

- 最大已测 200 concurrent Private WS。
- 100 connections / 50 msg/s：1000/1000 ACK，ACK p95 13.47 ms。
- 5 分钟 Private 1500/1500、Group 600/600 ACK/rows，交付计数全部匹配。
- File 1 MiB c10 为 60.75 MiB/s，10 MiB c5 为 211.75 MiB/s。

## Where was the load test run, and why not in the cloud?

基线运行在 Windows 宿主 + Docker Desktop Linux containers，目标是给同一仓库提供可重复的本地工程回归，不是 production capacity certification。云上容量规划需要固定机型、网络、HA 拓扑、监控和多轮统计，属于后续部署演进。

## Is 200 WebSocket connections the capacity limit?

不是。它只是当前 harness 和本机环境的 maximum tested load。没有继续升压到饱和点，因此不能推断容量上限。

## Why is Login throughput lower than Profile?

Login 需要 bcrypt 密码验证，属于有意的 CPU 密集安全操作；Profile 主要是已认证后的索引读取。当前实测 Login c25 约 99.98 req/s、p95 254.92 ms，Profile c10 约 2772.67 req/s、p95 4.86 ms。

## Why is audit best-effort?

V1 优先保证 Kafka 故障不能拖死核心 IM，所以 producer 使用有界队列、超时、有限重试和 drop 计数。代价是长故障可能丢审计。合规场景应改为 outbox、DLQ/重放、持久 spool、告警和明确保留策略。

## What does the race result prove?

Linux Docker full、focused 和真实依赖 integration 的 `go test -race` 都通过，说明这些执行路径未检测到数据竞争；它不能证明不存在所有并发 bug。Windows native 因 32-bit MinGW 与 windows/amd64 race 不兼容而不可用，所以项目结论明确写为“PASS in compatible Linux environment”。
