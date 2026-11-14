# 07 核心业务流程

## 1. 登录

```text
Web -> Gateway -> Auth.login -> User RPC/DB -> JWT -> Web
Web -> connect Chat WS + Group WS
Chat Hub 0->1 -> Redis Presence（好友提醒协议已定义，实现待下一轮；V1 只依赖 HTTP isOnline）
```

## 1.1 好友上线提醒（已实现，单机）

```text
用户 A 登录 -> Chat WS 建立
 -> Hub 本地连接 0->1（单机；多实例全局判断为 V2）
 -> Presence worker 写入/刷新本实例 contribution
 -> friendnotify observer 非阻塞入队（有界 channel）
 -> 通知 worker 异步查询 A 的好友列表（friends JOIN users，status=1）
 -> 通过 Hub.SendToUser 向在线好友推送 friend.online{userID:A, at}
A 最后一个本地 Chat WS 断开 -> Hub 1->0 -> 移除 Presence contribution
 -> 向在线好友推送 friend.offline
```

约束：Hub 回调只入队，不做网络 I/O；Group WS 不参与 Presence/好友事件；事件 best-effort，丢失时前端通过 `GET /api/user/friends` 的 `isOnline` 恢复；进程崩溃/TTL 过期不保证 offline 事件；多实例全局 0→1/1→0 为 V2。协议细节见 docs/06 §5.7。

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

## 2.1 图片消息

```text
Web 选择图片
 -> POST /api/file/image（multipart 字段 image）-> 服务端嗅探 MIME/大小/文件名 -> 存储或 SHA-256 复用
 -> 返回 user_files.id + url/mimeType/size
 -> Web 以 msg.type=2 + imageMsg.fileID 发送 chat.send / group.send
 -> 服务端 validate：type=2、fileID>0、width/height 合法
 -> 归属校验：user_files.id 属于 sender，并 JOIN file_objects
 -> MIME 必须 image/*；file_objects.size <= IMAGE_MESSAGE_MAX_BYTES
 -> 用 DB 的 url/mimeType/size 规范化 payload（客户端提交值不作为真相）
 -> 与文本相同的 message + session/unread 事务
 -> COMMIT -> ACK -> best-effort 推送 -> history 返回同一结构
```

上传失败不得发送消息。失败码：文件不存在/非本人 `2211`/`2412`、非图片 `2212`/`2413`、超限 `2213`/`2414`、参数非法 `2214`/`2415`；失败不落库、不 ACK。消息只存元数据，不存二进制；预览/下载仍走受保护的 `GET /api/file/:id`。

下载时 File 服务再次授权：所有者始终允许；私聊要求引用消息未 tombstone 且当前用户是发送者/接收者；群聊要求当前仍是群成员且消息未 tombstone。退群/被踢、tombstone 后、非参与者一律 `2503`。已下载到浏览器的副本属于客户端缓存，服务端不会回收。

## 2.2 普通文件消息

```text
Web 选择文件
 -> POST /api/file/file（multipart 字段 file）-> 流式存储 / SHA-256 复用
 -> 返回 user_files.id + fileName/mimeType/size
 -> Web 以 msg.type=3 + fileMsg.fileID 发送 chat.send / group.send
 -> 服务端 validate：type=3、fileID>0、size>=0
 -> 归属校验：user_files.id 属于 sender，并 JOIN file_objects
 -> 不限制 MIME 前缀；file_objects.size <= FILE_MESSAGE_MAX_BYTES
 -> 用 DB 的 url/fileName/size/mimeType 规范化 payload
 -> 与文本相同的 message + session/unread 事务
 -> COMMIT -> ACK -> best-effort 推送 -> history 返回同一结构
```

失败码：文件不存在/非本人 `2215`/`2416`、超限 `2216`/`2417`、参数非法 `2217`/`2418`；失败不落库、不 ACK。`msg_preview` 为 `[文件]`；下载授权与图片消息相同（所有者/私聊参与者/群成员，tombstone/退群/非参与者 `2503`）。

## 2.3 撤回流程（已实现，第十一轮）

```text
Web 选择“撤回”某条自己发送的消息
 -> chat.send / group.send，msg.type=4 + withdrawMsg.msgID
 -> 服务端校验：目标存在且属于当前会话/群、发送者是当前用户、
    未被 tombstone/撤回、距 createdAt <= RECALL_WINDOW_SECONDS、群聊仍为成员
 -> 落库一条撤回消息（不物理删除原消息）
 -> COMMIT -> ACK -> 推送 chat.message / group.message
 -> 历史/session 读取时把原消息替换为 type=4 占位（不返回原内容）
 -> 文件下载授权排除“被撤回消息引用的原消息”；所有者仍可下载
```

失败码：`2218–2221` / `2419–2422`（不存在、非本人、超窗口、已撤回）；群非成员沿用 `2404`。重复撤回同一 `msgID` 不重复插入，已有撤回消息时返回同一 ACK。

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

一名用户只需一条 Group Socket Manager，可有多个设备/tab Client；不是一群一个 socket。`group.send` 按目标 group 调用 `IsMember/GetMemberRole/IsMuted/GetGroup`；文本 `type=1` 与图片 `type=2` 走同一事务、ACK 与推送路径（图片额外做归属/MIME/大小校验）。Group Hub 没有 Presence transition，所有生命周期路径均不访问 `gim:presence:*`。

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

## 6. 好友申请与通过

```text
Search users -> GET /api/user/search（排除自己、只返回启用账号、标记 isFriend）
Send request -> POST /api/user/friends
  -> 事务内检查双向 friendship（已存在 -> 1114）
  -> INSERT friend_verifies(status=0)
  -> pending 生成列唯一键阻止同对用户反向/并发重复申请（1062 -> 1116）
Approve -> PUT /api/user/valid_status {status=1}
  -> 事务内 SELECT ... FOR UPDATE 申请行
  -> 校验 receiver == 当前用户（否则 1119）与 status == 0（否则 1118）
  -> INSERT 双向 friends（ON DUPLICATE KEY 幂等）
  -> UPDATE 申请 status=1
  -> commit
  -> 前端移除该申请并刷新好友列表
Reject -> 同一加锁与校验路径，只把 status 置 2，不写 friends
Re-apply after rejection -> 允许
  -> rejected 历史行保留；该行 pending_* 生成列已是 NULL，不占用 pending 唯一键
  -> 新 INSERT 生成新的 pending 行（status=0），再次申请成功
  -> 若已有新的 pending，则仍返回 1116；若已互为好友，返回 1114
```

并发两个管理员点击“同意”时，第二个请求阻塞在行锁上，提交后看到 `status=1` 并返回 `1118`，好友关系只会写入一次。`GET /api/user/friends` 从 Redis Presence 读取 `isOnline`；Presence 仍只由 Chat WS 写入，好友流程不改在线状态。

## 7. 文件消息

```text
select file -> multipart upload
 -> HTTP total request cap
 -> stream to same-filesystem temp + SHA-256 + first-512-byte MIME sniff
 -> streaming size cap / filename validate
 -> find file_object
    exists + physical exists: remove temp
    exists + physical missing: atomic repair from temp
    absent: atomic move to content-addressed path
 -> MySQL transaction resolve/create file_object + resolve/create user_file
 -> failure compensation: remove partial temp; delete only a newly published unreferenced object
 -> return URL/file metadata
 -> send normal chat/group message referencing file URL
```

上传与“发送文件消息”是两个步骤；上传成功不等于消息已发送。

相同用户、相同内容、相同规范化文件名返回原 UserFile；相同内容但不同文件名或不同用户创建各自 UserFile，始终只有一个 FileObject/物理对象。下载/详情/删除均先按可信用户身份检查 UserFile ownership。删除只移除用户引用；共享对象不受影响，物理 GC 不在当前 Checkpoint。

## 8. 撤回

请求撤回 -> 查原消息 -> 校验发送者/管理员权限和可选时间限制 -> 生成 WithdrawMsg -> 持久化 -> ACK -> 推送。删除自己的历史记录与撤回是不同功能。

## 9. 结构化审计日志

```text
business transaction commit
 -> build schema v1 event + UUID eventId + trusted requestId
 -> non-blocking bounded producer queue
 -> Kafka gim.audit.events (JSON; user/resource/event partition key)
 -> Logs Consumer validate
 -> MySQL INSERT audit_logs (event_id UNIQUE)
 -> commit Kafka offset only after persistence/idempotent duplicate resolution
 -> admin-only cursor/filter query
```

登录失败属于少量高价值失败事件，只保存 `failure_category`，不会保存账号是否存在的可枚举差异、输入账号或密码。注册事件由 Auth/User 注册边界唯一拥有，User 服务不重复发送同义事件。Chat/Group message event 只在消息事务成功且 `createdNew=true` 后发送，metadata 只有消息/发送方/接收方或群/类型/大小等标识，不含 content；重复 clientMsgId 不产生第二条审计事件。File 上传/删除只在 DB/物理流程成功后发送，不含文件正文、原始磁盘路径。

Kafka 故障是 observability degradation，不改变 HTTP 成功、消息 ACK、realtime delivery 或文件事务。消费者停机期间事件由 Kafka 保留；使用同 consumer group 恢复后继续持久化。Business DB 与 audit Kafka 不原子，V1 没有 outbox，长时间 broker 故障或有界队列溢出可能造成 audit loss，这是已知限制。

## 10. Admin 管理流程

```text
Admin Browser -> Gateway -> Auth authentication
 -> Gateway 覆盖 User-ID/Role
 -> target service 再验证 role=1 + valid user ID
 -> bounded cursor/filter query or transactional management action
 -> success response
 -> best-effort structured admin audit event
```

普通用户 token 和缺失可信身份的 `Role: 1` 均由目标服务拒绝。用户禁用保留历史关系，启用/禁用均使用 User Domain 的正式状态值并返回实际更新状态；Chat/Group 消息删除写 tombstone，并在同一事务修复 session last-message/unread；文件删除只移除 UserFile 引用；群解散保留历史消息。Admin Logs 详情只把已持久化 schema/allowlist metadata 渲染为普通文本 JSON，不使用 HTML 注入。

Admin Dashboard 只读聚合复用 Settings/Admin runtime，通过标量计数查询返回用户、消息、群、文件与审计事件数量；该接口同样经过 Gateway/Auth 和目标服务 Admin 身份校验，不引入 Mock `/api/data/*` 或新的微服务。
