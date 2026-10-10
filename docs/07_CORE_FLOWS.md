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
 -> DB transaction
    -> insert chat message
    -> upsert A->B session
    -> upsert B->A session + unread
 -> commit
 -> ACK A
 -> if B online locally: Hub.SendToUser(B)
 -> B writePump -> chat.message
```

关键顺序：**先成功落库，再 ACK/推送**。这样推送失败不会造成消息丢失，B 可从历史恢复。

## 3. 私聊已读

用户打开会话/看到指定 messageId -> HTTP `PUT /api/chat/read` -> 验证该消息属于 A/B -> 推进 A 的 lastReadMessageId -> unread 归零/重算。V1 不要求已读回执实时通知对方。

## 4. 断线与重连

read/write error 或 Pong 超时 -> unregister -> 若该用户在本 Chat 实例连接数 1->0，删除本实例 Presence contribution；其他实例仍有未过期 contribution 时用户保持全局在线。进程崩溃则由 Presence TTL 淘汰 stale contribution。前端退避重连；成功后重新拉 session/history/unread，不依赖服务端重放内存消息。

## 5. 群聊发送

```text
Sender -> Group readPump -> GroupMessageService
 -> member check
 -> group all-mute check
 -> member mute check
 -> idempotency
 -> transaction insert group_message
 -> update group_session for members
 -> commit
 -> ACK sender
 -> Group Hub broadcast online members
```

V1 群规模按参考项目限制；大群 session 扇出成本记录为 V2 优化点。

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
