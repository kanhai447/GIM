# 06 WebSocket 协议

V1 保留两条连接：Chat WS 和 Group WS。URL 路径以参考项目实际路由为基线；若路径调整，必须同步 Gateway 和前端配置。

## 1. 鉴权

V1 query token：

```text
wss://host/.../chat?token=<JWT>
wss://host/.../group?token=<JWT>
```

长期 JWT 暴露风险记录为 V2 Ticket 优化，不阻塞 V1。

## 2. Envelope

所有业务 JSON 帧统一：

```json
{
  "event":"chat.send",
  "requestId":"optional-request-id",
  "clientMsgId":"01J...",
  "data":{}
}
```

服务端事件：

```json
{
  "event":"chat.ack",
  "requestId":"...",
  "clientMsgId":"01J...",
  "data":{},
  "error":null
}
```

## 3. 私聊事件

### Client -> Server `chat.send`

```json
{
  "event":"chat.send",
  "clientMsgId":"uuid-or-ulid",
  "data":{
    "revUserID":1002,
    "msg":{"type":1,"textMsg":{"content":"你好"}}
  }
}
```

### Server -> Sender `chat.ack`

只有 `message + sender session + receiver session/unread` 事务成功提交，或命中已提交的同一幂等消息后发送：

```json
{
  "event":"chat.ack",
  "clientMsgId":"...",
  "data":{"messageId":123,"createdAt":"2026-10-06T16:00:00+08:00"}
}
```

若相同 clientMsgId 重试，返回同一 messageId，不能再次插入。

`chat.ack` 是 **Server persist ACK**：只表示服务端已经接受并持久化消息；不表示 receiver 在线、收到、展示或已读。ACK 只回发到发起 `chat.send` 的 Client。

### Server -> Recipient `chat.message`

`data` 使用历史消息接口同构的消息 DTO，避免前端维护两套结构。

```json
{
  "event":"chat.message",
  "clientMsgId":"uuid-or-ulid",
  "data":{
    "messageId":123,
    "sendUserID":1001,
    "revUserID":1002,
    "clientMsgId":"uuid-or-ulid",
    "msg":{"type":1,"textMsg":{"content":"你好"}},
    "createdAt":"2026-10-06T16:00:00+08:00"
  }
}
```

Checkpoint 4 只接受文本 `type=1`：`clientMsgId` 为 1–64 字符且只允许字母、数字、`.`、`_`、`:`、`-`；文本 UTF-8 非空且默认最多 4096 bytes，typed message payload 默认最多 16384 bytes。senderID 只取 Gateway 注入身份，不允许客户端提交。

receiver 离线不影响持久化和 ACK；在线 receiver 的所有 Chat Clients best-effort 收到一次 `chat.message`。跨实例统一走 Redis Pub/Sub；publish/本地队列失败不改变 ACK 语义，未来由 history 恢复。幂等重试不再次发布 `chat.message`。

receiver 是否在线不改变 unread：每条新消息都在事务中使 receiver session unread +1，只有 `PUT /api/chat/read` 改变 read state。Mark Read 不新增 WebSocket read receipt。

### Presence

- `friend.online`
- `friend.offline`（可实现；若参考 UI 只展示上线提醒，离线事件至少更新状态）

```json
{"event":"friend.online","data":{"userID":1002,"at":"..."}}
```

### 错误

`error` 事件要带 `clientMsgId`（若有关），使前端能把 sending 改 failed。

```json
{
  "event":"error",
  "clientMsgId":"uuid-or-ulid",
  "error":{"code":2201,"message":"消息格式无效"}
}
```

错误只返回稳定业务 code/message，不返回 SQL、DSN、RPC endpoint 或内部 `err.Error()`。单次 malformed JSON 优先返回 typed error，连接可继续处理后续合法 frame。

## 4. 群聊事件

Day 3 / Checkpoint 2 已实现连接层 endpoint：

```text
/api/group/ws/chat?token=<JWT>
```

连接经 Gateway -> Auth 后由 Group API 读取可信 `User-ID/Role`。同一用户允许多个 Client；连接本身不携带或绑定 `groupID`。应用帧严格使用本文件 Envelope；malformed payload 返回 `error` code `2401`，未知/尚未实现的事件返回 code `2402`，连接可继续。错误只包含稳定 code/message，不返回内部错误。

Checkpoint 3 已在相同连接层启用文本 `group.send/group.ack/group.message`。sender 只取连接绑定的可信 `User-ID`；客户端不能提交 sender ID。`clientMsgId` 与私聊相同，限制为 1–64 个安全字符；当前只接受 UTF-8 非空、最多 4096 bytes 的 `type=1` 文本，typed payload 最多 16384 bytes。

### `group.send`

```json
{
  "event":"group.send",
  "clientMsgId":"...",
  "data":{"groupID":2001,"msg":{"type":1,"textMsg":{"content":"hi"}}}
}
```

### `group.ack`

只有 `group_messages + 当前成员 group_sessions/unread` 事务成功提交，或命中已提交的同一幂等消息后，才只向 originating Client 返回：

```json
{"event":"group.ack","clientMsgId":"...","data":{"messageId":456,"groupID":2001,"createdAt":"..."}}
```

ACK 仅表示服务端已持久化/接受，不是成员 delivery ACK 或 read receipt。重复相同逻辑消息返回同一 messageId；同一 sender/clientMsgId 复用于不同 group/type/content 返回稳定冲突错误。

### `group.message`

事务内成员快照通过 `gim:group:delivery` 广播到各 Group API 实例的当前在线 Clients，消息 DTO 与 history 同构：

```json
{"event":"group.message","clientMsgId":"...","data":{"messageId":456,"groupID":2001,"sendUserID":1001,"clientMsgId":"...","msg":{"type":1,"textMsg":{"content":"hi"}},"createdAt":"..."}}
```

sender 所有设备（包括 originating Client）均可收到 message；originating Client 还单独收到 ACK，前端必须按 server messageId/clientMsgId 合并。离线不影响 unread；Redis publish/本地背压失败不改变 ACK，重连后从 session/history 恢复。幂等重试不再次发布。

## 5. 消息类型

沿用参考 `ctype.MsgType`：文本、图片、视频文件、普通文件、语音文件、撤回、回复、引用、@、图文。VoiceCall/VideoCall V1 不验收，可保留类型但不能成为核心依赖。

## 6. 撤回

V1 可沿用“撤回是一条特殊消息”的参考模型，`withdrawMsg.msgID` 指向原消息。服务端验证操作者、时间策略（若配置）后落库撤回事件并推送。历史 DTO 不应泄露不应展示的原内容。

## 7. Ping/Pong

使用 WebSocket 控制帧，不使用 JSON `ping` 代替协议心跳。业务层无需处理 ping 消息。

## 8. 连接关闭

服务端按标准 close code 关闭；前端非主动 logout 可重连。身份失效类 close 必须禁止无限重连并引导重新登录。

Group WS 的任一设备 normal/abnormal/timeout/slow/shutdown 只移除对应 Client；同用户 sibling 保持。Group WS 不产生全局 online/offline、不得刷新或删除 `gim:presence:*`，禁言或退出某个群也不关闭这个用户级 socket。
