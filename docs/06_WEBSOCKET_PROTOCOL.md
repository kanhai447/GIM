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

只有消息成功持久化或命中已持久化的同一幂等消息后发送：

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

### `group.send`

```json
{
  "event":"group.send",
  "clientMsgId":"...",
  "data":{"groupID":2001,"msg":{"type":1,"textMsg":{"content":"hi"}}}
}
```

### `group.ack`

事务成功后返回 messageId。

### `group.message`

广播给当前在线群成员；发送者可以通过 ACK 本地确认，也可以统一接收 message 事件，前端必须避免重复插入（按 server messageId/clientMsgId 去重）。

## 5. 消息类型

沿用参考 `ctype.MsgType`：文本、图片、视频文件、普通文件、语音文件、撤回、回复、引用、@、图文。VoiceCall/VideoCall V1 不验收，可保留类型但不能成为核心依赖。

## 6. 撤回

V1 可沿用“撤回是一条特殊消息”的参考模型，`withdrawMsg.msgID` 指向原消息。服务端验证操作者、时间策略（若配置）后落库撤回事件并推送。历史 DTO 不应泄露不应展示的原内容。

## 7. Ping/Pong

使用 WebSocket 控制帧，不使用 JSON `ping` 代替协议心跳。业务层无需处理 ping 消息。

## 8. 连接关闭

服务端按标准 close code 关闭；前端非主动 logout 可重连。身份失效类 close 必须禁止无限重连并引导重新登录。
