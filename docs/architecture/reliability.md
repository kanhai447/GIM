# Messaging Reliability

## Reliability contract

GIM V1 的目标是：同一发送者的重试不制造重复消息；服务端只对已接受并持久化的消息 ACK；连接断开后可由历史、会话和未读恢复。V1 不承诺接收端已展示，也不承诺端到端 exactly-once。

## Single-writer WebSocket model

每个 Chat/Group Client 固定使用：

- `readPump`：读取、限长、解析、心跳 deadline 和业务入站。
- `writePump`：唯一连接写者，串行处理 send queue、Ping 与关闭。
- 有界 send queue：避免任意业务 goroutine 直接 `WriteMessage/WriteJSON` 和无限内存增长。
- Hub 事件循环：串行处理 register/unregister/broadcast，避免普通全局 map 的并发读写。

连接关闭路径使用幂等清理；慢消费者不会无限阻塞 Hub。心跳超时、写失败或服务 shutdown 都会进入同一注销路径。

## Persistence, ACK and delivery

```text
validate -> begin transaction -> insert/idempotency lookup
         -> update sessions/unread -> commit
         -> ACK sender -> live delivery
```

- transaction 失败：无 ACK，调用方可以使用相同 `clientMsgId` 重试。
- commit 成功：消息和会话成为事实来源。
- ACK 丢失：重试命中唯一键并返回原消息，不创建新行。
- live delivery 失败：不撤销数据库事实；接收方之后通过 session/history/unread 恢复。
- 进程在 commit 后、ACK/推送前终止：可能出现“已入库但实时反馈丢失”的窗口，这是 V1 明确限制。

## Idempotency

Private 与 Group 消息都以 `(send_user_id, client_msg_id)` 建唯一约束。Repository 将 duplicate-key 解释为幂等重试并读取既有行，同时校验目标/群组和 payload 语义，防止同一 key 被用于不同消息。前端 reconciliation 以 `clientMsgId` 将 pending、ACK、delivery 和历史合并。

## Sessions, unread and read

- 消息事务同时维护发送者和接收者/群成员 session。
- 新消息恢复被隐藏会话并更新最后消息。
- 未读只对非发送者递增。
- read cursor/last-read 采用单调语义，旧请求不能把状态回退。
- 置顶、隐藏和用户范围 tombstone 独立于消息物理行。

## Deadlock policy

5 分钟 soak 暴露 MySQL 1213 后，只对明确可安全重试的事务增加最多 3 次重建事务。退避为 10/20 ms 且监听 context；每次尝试重新读取与写入，绝不复用已失败 transaction。其他错误不被伪装成重试成功。

## Presence and reconnect

Presence 只由 Chat 连接维护。Redis key 带 TTL，连接存活时刷新；Redis 短暂不可用不会让 WS Hub 崩溃，恢复后继续刷新。前端 Chat/Group socket manager 各自执行退避重连；pending 状态仅在当前页面内保留，不跨刷新持久化。

## Verification evidence

- 100 Private 连接、50 msg/s：1000/1000 ACK、2600/2600 delivery。
- 5 分钟 Private：1500/1500 rows/ACK、2100/2100 delivery。
- 5 分钟 Group：600/600 rows/ACK、6000/6000 delivery。
- controlled restart 证明持久化恢复，并保留 commit-to-ACK gap 的真实限制。
- Linux Docker full/focused/real-dependency race：PASS。

详细数据见 [性能基线](../performance/baseline.md)。
