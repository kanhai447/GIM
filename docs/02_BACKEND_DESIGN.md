# 02 后端详细设计

## 1. 目录策略

GIM 后端从新的 Go 工程开始自主编写。参考 FIM 只用于理解服务职责、接口语义和业务行为，**不得复制参考代码作为基线再重构**。最终建议：

```text
server/
  gim_auth/
  gim_user/
  gim_chat/
  gim_group/
  gim_file/
  gim_settings/
  gim_gateway/
  common/
  core/
  utils/
```

Go module 从第一天直接使用 GIM 命名；不存在 `fim_* -> gim_*` 的批量改名阶段。先搭最小可构建的 GIM skeleton，再逐服务实现并保持 build green。

## 2. Hub / Client

Chat 和 Group 各有独立 Hub。

```go
type Hub struct {
    clients    map[uint]map[string]*Client
    register   chan *Client
    unregister chan *Client
    command    chan any // 可选，用于序列化查询/发送操作
}

type Client struct {
    UserID   uint
    ClientID string
    Conn     *websocket.Conn
    Send     chan []byte
    Hub      *Hub
}
```

约束：

- `Hub.Run()` 是 clients map 的唯一写入者。
- 业务层不得持有/写 `*websocket.Conn`。
- `readPump` 只读；`writePump` 是该 Conn 唯一写者。
- `Send` channel 必须有有限缓冲；满时按策略断开慢客户端，不能无限阻塞 Hub。
- `unregister` 必须幂等，重复关闭不得 panic。

## 3. 心跳

采用 Gorilla Ping/Pong 控制帧：

- `pongWait` 例如 60s；
- `pingPeriod < pongWait`，例如 50s；
- readPump 设置 read deadline；
- PongHandler 刷新 deadline；
- writePump 定时发送 Ping；
- 超时触发 unregister。

精确数值配置化，测试环境可缩短。

## 4. Presence

全局在线状态只由 Chat Hub 维护：

```text
Chat user local connections: 0 -> 1 : online
Chat user local connections: 1 -> 0 : offline
```

Redis 按用户记录各 Chat API 实例的 Presence contribution：

```text
gim:presence:user:{uid} = ZSET(instanceID -> expiresAt)
```

Hub 仍以本实例 `userID -> clientID -> Client` 为连接计数真相；本实例 `0->1` 新增/续租 contribution，`1->0` 删除 contribution。全局在线等价于至少存在一个未过期 contribution。独立 Presence worker 负责合理周期刷新并在 Redis 故障时重试，避免在 Hub event loop 中执行网络 I/O；进程崩溃后的 stale contribution 由过期时间清理。

Group Hub 不写这个 key，也不参与全局 Presence。好友列表从 Redis 查询 Presence；好友上线提醒只允许基于 Chat Hub 的真实用户级 transition 扩展，本阶段不实现通知广播。

## 5. 私聊发送 Service

WebSocket Handler 只负责 envelope 解析和调用：

```text
HandleChatSend(ctx, userID, clientMsgId, payload)
```

Service 负责：参数验证 -> 接收者 RPC 校验 -> 好友校验 -> 幂等检查 -> 构造消息 -> 持久化，并返回 `createdNew`。WebSocket transport 只在 Service 成功后向发起 Client 排队 ACK；只有 `createdNew=true` 才触发 receiver realtime delivery，幂等重试只重发 ACK。

ACK 只能在数据库持久化成功后返回，且仅代表服务端已接受/持久化，不代表 receiver 在线、收到或已读。ACK 只发回发起此次发送的 Client，不广播到 sender 其他设备。

Checkpoint 4 只持久化 `chat_messages`；Checkpoint 5 引入 session/unread 时，必须把 message insert 与双方 session update 纳入同一事务。当前实时私聊统一发布到 `gim:chat:delivery`，各 Chat API 实例只向本实例 receiver Clients 投递；同实例也不额外直投，避免本地路径与 Pub/Sub 重复。

## 6. 群聊发送 Service

参数验证 -> 群成员检查 -> 全员禁言/个人禁言 -> clientMsgId 幂等 -> 落库 -> 更新每个成员 session/unread -> ACK -> Hub 广播当前在线成员。

群成员较多时 V1 可同步更新；需把“未来异步扇出/大群优化”写入已知限制，不在 V1 提前引 Kafka。

## 7. Context

HTTP handler 传入的 ctx 应继续传到 RPC/DB；禁止业务路径无理由 `context.Background()`。后台生命周期任务除外。

## 8. 事务边界

至少以下操作需要事务：

- 私聊消息 insert + session 更新；
- 群消息 insert + session 更新；
- 好友验证同意 -> 建好友关系 + 更新验证状态；
- 创建群 -> group + creator member + 邀请成员；
- 文件物理对象与 user_file 关系建立（结合文件落盘补偿）。

## 9. 安全和配置

- `CheckOrigin` 不得永久 `return true`；允许 origin 配置化。
- 密码只保存强哈希；参考项目仅作为行为参考，不继承不安全实现。
- 上传路径、JWT secret、MySQL/Redis/etcd 地址全部从配置读取。
- 前端输入仍需后端验证，不能依赖 TS 类型。

---

# 三仓库复审后的补充后端要求

## A. Admin API 必须保留服务端权限校验

参考源码中的 User/Chat/Group/File/Settings/Logs 均存在管理员接口和 `AdminMiddleware`。GIM 继续使用 Gateway 认证后注入 `User-ID`/`Role`，目标服务验证 `Role == 1`。浏览器端角色判断不能作为安全边界。

正式部署不得直接暴露这些内部 API 服务端口，否则外部请求可绕过 Gateway 并伪造 Role Header。

## B. 日志系统必须重构后再保留

原 Kafka 日志链路保留，但禁止复用原共享可变 `Pusher`。正确方向：

```text
Kafka producer client（可共享）
        |
每个 HTTP 请求创建独立 LogEvent/Builder
        |
结构化 JSON + 字段脱敏
        |
Kafka
        |
Logs Consumer
        |
MySQL
```

不得记录：明文密码、JWT、Authorization、Cookie、第三方 Secret、完整敏感请求头。

Admin 日志详情必须安全渲染结构化 JSON，禁止将用户输入拼接 HTML 后 `v-html`。

## C. 原会话聚合查询的结论修正

实际私聊源码通过子查询拿最新 preview，不是简单 `MAX(msg_preview)`。GIM 改 session 表的理由应表述为：降低动态聚合复杂度、支持未读/已读、降低大消息表查询成本、统一删除/置顶状态，而不是声称原 SQL 一定取错 preview。

群会话删除过滤存在用户隔离语义问题，应在迁移 session 时修正。
