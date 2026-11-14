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

Checkpoint 4 只接受文本 `type=1`；V2 Phase 1 起同时接受图片 `type=2`（见第 5 节）。`clientMsgId` 为 1–64 字符且只允许字母、数字、`.`、`_`、`:`、`-`；文本 UTF-8 非空且默认最多 4096 bytes；typed message payload 默认最多 16384 bytes；图片 payload 只含元数据，不随文件大小增长。senderID 只取 Gateway 注入身份，不允许客户端提交。

receiver 离线不影响持久化和 ACK；在线 receiver 的所有 Chat Clients best-effort 收到一次 `chat.message`。跨实例统一走 Redis Pub/Sub；publish/本地队列失败不改变 ACK 语义，未来由 history 恢复。幂等重试不再次发布 `chat.message`。

receiver 是否在线不改变 unread：每条新消息都在事务中使 receiver session unread +1，只有 `PUT /api/chat/read` 改变 read state。Mark Read 不新增 WebSocket read receipt。

### Presence

**V2 / 未实现。** V1 服务端不发送 `friend.online` / `friend.offline`；在线状态只在 HTTP `GET /api/user/friends`、`GET /api/user/friend_info` 时从 Redis Presence 读取 `isOnline`。以下为 V2 目标格式（保留以便实现时对齐）：

- `friend.online`
- `friend.offline`（离线事件用于更新状态）

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

Checkpoint 3 已在相同连接层启用文本 `group.send/group.ack/group.message`；V2 Phase 1 起同一连接也接受图片 `type=2`。sender 只取连接绑定的可信 `User-ID`；客户端不能提交 sender ID。`clientMsgId` 与私聊相同，限制为 1–64 个安全字符；文本要求 UTF-8 非空、最多 4096 bytes，typed payload 最多 16384 bytes；图片 payload 只含元数据。

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

沿用参考 `ctype.MsgType` 的类型编号。V2 已实现 **文本 `type=1`**、**图片 `type=2`** 与 **普通文件 `type=3`**；其余类型仍未实现。

| type | 语义 | 状态 |
|---|---|---|
| `1` | 文本 | 已实现（V1） |
| `2` | 图片 | 已实现（V2 Phase 1） |
| `3` | 普通文件 | 已实现（V2 Phase 1） |
| 其他 | 视频文件、语音、回复/引用、撤回、@、图文 | V2 / 未实现；服务端返回 `2201`/`2403` |

### 5.1 图片消息 payload

`chat.send` 与 `group.send` 的 `msg` 使用同一结构：

```json
{
  "type": 2,
  "imageMsg": {
    "fileID": 123,
    "url": "/api/file/123",
    "width": 800,
    "height": 600,
    "size": 123456,
    "mimeType": "image/png"
  }
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `fileID` | 是 | `user_files.id`（上传接口返回的 `id`），必须是发送者自己的引用 |
| `url` | 否 | 服务端回填为 `/api/file/{fileID}`；客户端提交值被忽略 |
| `width` / `height` | 否 | 客户端显示提示，若提供必须为 `1..20000`；服务端不解码图片，不据此做安全判断 |
| `size` | 否 | 服务端以 `file_objects.size` 为准回填，客户端提交值被忽略 |
| `mimeType` | 否 | 服务端以 `file_objects.mime_type`（首 512 bytes 嗅探结果）为准回填 |

服务端在事务前校验：

1. `fileID > 0`，且 `user_files.user_id = senderID`；不存在或不属于发送者统一返回 `2211`/`2412`（不区分，避免枚举）。
2. `file_objects.mime_type` 必须以 `image/` 开头，否则 `2212`/`2413`。
3. `file_objects.size <= IMAGE_MESSAGE_MAX_BYTES`（默认 10 MiB），否则 `2213`/`2414`。
4. `type=2` 必须携带 `imageMsg.fileID`；`width`/`height` 若提供必须为合法正数，否则 `2214`/`2415`。

校验通过后，服务端用数据库元数据规范化 payload（`url`、`mimeType`、`size`）再落库；`msg` 只存元数据，不存二进制，不返回原始文件路径。历史查询 `GET /api/chat/history`、`GET /api/group/history/:id` 与 `chat.message`/`group.message` 使用同一结构。消息 DTO 不含二进制；接收方（私聊对端、群成员）通过 `GET /api/file/{fileID}` 读取，文件服务按“所有者或消息参与者”授权：私聊要求消息未 tombstone 且当前用户是发送者/接收者，群聊要求当前仍是群成员且消息未 tombstone，否则统一 `2503`（详见 `docs/05_HTTP_API.md`）。

### 5.2 ACK 与幂等

图片与文本走同一事务（insert message + 更新 session/unread），ACK 语义不变：事务成功提交后才返回 `messageId`/`createdAt`。`clientMsgId` 幂等比较包含 `type` 与 `fileID`：同一 sender/clientMsgId 改成另一张图片返回 `2205`/`2407`，不能生成第二条消息。普通文件消息同样适用（比较 `fileMsg.fileID`）。

### 5.3 图片消息错误码

| 场景 | Chat | Group |
|---|---|---|
| 文件不存在或不属于发送者 | `2211` | `2412` |
| 文件不是图片（MIME 非 `image/*`） | `2212` | `2413` |
| 图片超过大小限制 | `2213` | `2414` |
| 图片参数缺失/非法 | `2214` | `2415` |
| 未知消息类型 | `2201` | `2403` |
| 同一 clientMsgId 复用于不同逻辑消息 | `2205` | `2407` |

错误只返回稳定 code/message，不返回 SQL、DSN、存储路径或内部错误；校验失败不落库、不发送成功 ACK。

### 5.4 兼容性

- `type=1` 文本的字段、校验、ACK、历史、前端渲染完全不变。
- `type=2` 是新增类型，不新增表/列；复用 `chat_messages.msg` / `group_messages.msg` 现有 JSON 列。
- `type=3` 同样是新增类型，复用同一 JSON 列与 `user_files`/`file_objects`。
- 旧客户端收到 `type=2`/`type=3` 可能无法渲染；GIM Web 对无法识别的消息显示占位，不崩溃、不影响其他消息。
- 旧服务端收到 `type=2`/`type=3` 返回 `2201`/`2403`（当前行为）。
- 图片上传与发消息是两个步骤：先 `POST /api/file/image`（multipart 字段 `image`）拿到 `user_files.id`，再发送图片消息；上传失败不得发送。

### 5.5 普通文件消息

`chat.send` 与 `group.send` 的 `msg` 使用同一结构：

```json
{
  "type": 3,
  "fileMsg": {
    "fileID": 123,
    "url": "/api/file/123",
    "fileName": "report.pdf",
    "size": 123456,
    "mimeType": "application/pdf"
  }
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `fileID` | 是 | `user_files.id`（上传接口返回的 `id`），必须是发送者自己的引用 |
| `url` | 否 | 服务端回填为 `/api/file/{fileID}`；客户端提交值被忽略 |
| `fileName` | 否 | 服务端以 `user_files.original_name` 回填；客户端提交值被忽略 |
| `size` | 否 | 服务端以 `file_objects.size` 回填；客户端提交值被忽略 |
| `mimeType` | 否 | 服务端以 `file_objects.mime_type`（嗅探结果）回填 |

服务端在事务前校验：

1. `fileID > 0`，且 `user_files.user_id = senderID`；不存在或不属于发送者统一返回 `2215`/`2416`（不区分，避免枚举）。
2. **不限制 MIME 前缀**（普通文件允许任意类型；上传接口自身仍做大小/文件名/流式限制）。
3. `file_objects.size <= FILE_MESSAGE_MAX_BYTES`（默认 25 MiB），否则 `2216`/`2417`。
4. `type=3` 必须携带 `fileMsg.fileID`；`size` 若提供必须为非负数，否则 `2217`/`2418`。

校验通过后，服务端用数据库元数据规范化 payload（`url`、`fileName`、`size`、`mimeType`）再落库；`msg` 只存元数据，不存二进制，不返回原始文件路径。历史与 WS 推送同构。下载复用文件授权语义（所有者、私聊参与者、群成员；tombstone/退群/非参与者 `2503`）；授权查询同时匹配 `imageMsg.fileID` 与 `fileMsg.fileID`。

#### 普通文件消息错误码

| 场景 | Chat | Group |
|---|---|---|
| 文件不存在或不属于发送者 | `2215` | `2416` |
| 文件超过 `FILE_MESSAGE_MAX_BYTES` | `2216` | `2417` |
| 文件消息参数缺失/非法 | `2217` | `2418` |
| 未知消息类型 | `2201` | `2403` |
| 同一 clientMsgId 复用于不同逻辑消息 | `2205` | `2407` |

## 6. 撤回

**V2 / 未实现**。参考实现把撤回建模为一条特殊消息（`withdrawMsg.msgID` 指向原消息）；GIM V1 没有撤回消息类型、没有撤回路由/事件，`chat_message_deletions` 只表达“仅自己隐藏历史”，与撤回是不同语义。

## 7. Ping/Pong

使用 WebSocket 控制帧，不使用 JSON `ping` 代替协议心跳。业务层无需处理 ping 消息。

## 8. 连接关闭

服务端按标准 close code 关闭；前端非主动 logout 可重连。身份失效类 close 必须禁止无限重连并引导重新登录。

Group WS 的任一设备 normal/abnormal/timeout/slow/shutdown 只移除对应 Client；同用户 sibling 保持。Group WS 不产生全局 online/offline、不得刷新或删除 `gim:presence:*`，禁言或退出某个群也不关闭这个用户级 socket。

## 9. V1 事件清单

| 连接 | Client → Server | Server → Client | 状态 |
|---|---|---|---|
| Chat WS | `chat.send` | `chat.ack`, `chat.message`, `error` | 已实现（文本 `type=1`、图片 `type=2`） |
| Group WS | `group.send` | `group.ack`, `group.message`, `error` | 已实现（文本 `type=1`、图片 `type=2`） |
| Chat WS | — | `friend.online`, `friend.offline` | V2 / 未实现 |
| 两条连接 | 视频/普通文件/语音/回复/引用/撤回/@/图文 | 同左 | V2 / 未实现（图片已实现） |

Control frame Ping/Pong 不属于业务事件，见第 7 节。
