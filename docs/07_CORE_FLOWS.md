# 07 核心业务流程

## 1. 登录

```text
Web -> Gateway -> Auth.login -> User RPC/DB -> JWT -> Web
Web -> connect Chat WS + Group WS
Chat Hub 0->1 -> Redis Presence -> notify online friends
```

## 2. 私聊发送

```text
A ChatSocket
 -> Chat readPump
 -> parse/validate
 -> ChatMessageService
 -> check user restriction
 -> check friendship
 -> lookup (A, clientMsgId)
    -> exists: return original ACK
 -> transaction
    -> INSERT chat message (DB UNIQUE is final idempotency guard)
       -> duplicate key: query original message; do not mutate session
    -> upsert A->B session（unread unchanged）
    -> upsert B->A session（unread atomic +1）
    -> last_message_id/at only advance for larger message ID
 -> COMMIT
 -> ACK originating Client A
 -> if createdNew: publish gim:chat:delivery
 -> every Chat instance receives event
 -> instance(s) with B Clients: Hub.SendToUser(B)
 -> B writePump -> chat.message (all active Chat Clients)
```

关键顺序：**message/session 事务提交，再 ACK，再 best-effort 推送**。Redis publish/receiver backpressure 失败不会回滚消息或把 ACK 变成失败；B 后续可从 history 恢复。幂等重试 `createdNew=false`，只重发同一 messageId 的 ACK，不再次增加 unread、修改 session 或推送 receiver。

## 3. 私聊已读

用户打开会话/看到指定 messageId -> HTTP `PUT /api/chat/read` -> 验证该消息属于 A/B -> 锁定 A->B session -> 单调推进 A 的 lastReadMessageId -> 按 cursor 后仍存在的入站消息重算 unread -> commit。并发新消息必须在该 session 行锁之后原子增加 unread，因此不会把 read cursor 之后的新消息误清零。V1 不向对方发送已读回执。

### 私聊会话与历史

`GET /api/chat/session` 直接读取显式 session 状态，使用数据库索引完成置顶/最近时间排序，并在一次 JOIN 中取得 peer 基础信息和 last message。`hidden_at` 只影响当前用户；后续新消息将 session 恢复可见。

`GET /api/chat/history` 使用 server message ID 作为 `beforeMessageID`，只返回当前用户与 peer 的双向消息，按 DB 降序取页后以 oldest -> newest 返回。用户维度 deletion 只过滤请求用户自己的历史，不等于 recall。

## 4. 断线与重连

read/write error 或 Pong 超时 -> unregister -> 若该用户在本 Chat 实例连接数 1->0，删除本实例 Presence contribution；其他实例仍有未过期 contribution 时用户保持全局在线。进程崩溃则由 Presence TTL 淘汰 stale contribution。前端退避重连；成功后重新拉 session/history/unread，不依赖服务端重放内存消息。

## 5. 群聊发送

### Group WebSocket connection foundation（Day 3 / Checkpoint 2）

```text
Client -> Gateway query token -> Auth -> trusted User-ID/Role -> Group WS
 -> Group Hub register userID/clientID
 -> readPump: typed envelope + Pong deadline refresh + inbound boundary
 -> writePump: data + Ping + queued Pong（唯一连接写者）
 -> normal/abnormal/timeout/slow/shutdown: unregister this Client only
```

一名用户只需一条 Group Socket Manager，可有多个设备/tab Client；不是一群一个 socket。当前 `group.send` 明确返回未实现错误。后续消息处理才按目标 group 调用 `IsMember/GetMemberRole/IsMuted/GetGroup`。Group Hub 没有 Presence transition，所有生命周期路径均不访问 `gim:presence:*`。

### 群领域与成员基础（Day 3 / Checkpoint 1）

```text
Create Group
 -> validate trusted operator and batch users
 -> transaction: INSERT groups + INSERT owner/member rows
 -> commit（任一 member 写失败则 group 不残留）

Apply Join
 -> active group + not-member + pending uniqueness
 -> open policy: transaction accepted request + member
 -> approval policy: create one pending request

Approve Join
 -> manager policy
 -> lock pending request and group
 -> transaction: INSERT member + pending -> accepted
 -> concurrent reviewers converge to one member and one state transition
```

owner/admin/member 权限判断集中在 Group Service policy；owner 不能普通退出，admin 不能操作 owner/其他 admin。临时个人禁言仅使用 Redis TTL，Group 不读写 Presence。

`My Groups` 使用 member->group JOIN 与成员数聚合；成员列表一次读取 membership，并通过 User 批量 RPC 获取公开资料，禁止逐成员 N+1。

```text
Sender -> Group readPump -> GroupMessageService
 -> active group + member check
 -> group all-mute check
 -> member mute check
 -> idempotency
 -> transaction lock active group + resolve sender member
 -> insert group_message（DB UNIQUE 是最终幂等屏障）
 -> snapshot current member IDs
 -> batch upsert group_session（sender +0；others unread atomic +1）
 -> commit
 -> ACK originating Client
 -> if createdNew: publish gim:group:delivery with target user IDs
 -> every Group instance routes to its local member Clients
```

duplicate retry 只返回同一 ACK，不再改变 session/unread 或发布。Redis publish 失败不撤销 commit/ACK。sender sibling devices 接收 `group.message`，不接收 originating ACK。Group delivery 与生命周期均不修改 Presence。

`GET /api/group/session` 通过显式 session + group + last message 一次 JOIN 返回；`GET /api/group/history/:id` 以 message ID cursor 分页并按当前用户隔离 deletion。`PUT /api/group/read` 先验证当前 membership 和目标消息所属群，再锁定当前用户 session，单调推进 read cursor 并重算 cursor 后由其他成员发送的 unread；与并发 send 通过同一 session 行锁串行化。

V1 群规模按参考项目限制；同步 session 扇出成本记录为大群 V2 优化点。

## 6. 好友申请通过

验证记录加锁/事务 -> 确认状态未处理 -> 防重复 friendship -> 建好友关系 -> 更新验证状态 -> commit。前端刷新好友列表。

## 7. 文件消息

```text
select file -> multipart upload
 -> stream to temp + SHA-256
 -> size/MIME validate
 -> find file_object
    exists: remove temp
    absent: atomic move + create file_object
 -> create user_file
 -> return URL/file metadata
 -> send normal chat/group message referencing file URL
```

上传与“发送文件消息”是两个步骤；上传成功不等于消息已发送。

## 8. 撤回

请求撤回 -> 查原消息 -> 校验发送者/管理员权限和可选时间限制 -> 生成 WithdrawMsg -> 持久化 -> ACK -> 推送。删除自己的历史记录与撤回是不同功能。
