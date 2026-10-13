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

Day 3 / Checkpoint 2 的 Group WS 使用独立 `group/ws` Hub/Client，不重构已封板的 Chat Hub。Group Hub 只维护 `userID -> clientID -> Client` 用户级连接，不在连接时绑定单个群，也不建立未被消息业务需要的订阅索引。成员与禁言校验留在后续具体 `group.send` 处理链。Group Hub 没有 Presence observer、Redis client 或 online/offline transition；连接、超时、慢客户端和 shutdown 均不得影响全局 Presence。

## 3. 心跳

采用 Gorilla Ping/Pong 控制帧：

- `pongWait` 例如 60s；
- `pingPeriod < pongWait`，例如 50s；
- readPump 设置 read deadline；
- PongHandler 刷新 deadline；
- writePump 定时发送 Ping；
- 超时触发 unregister。

精确数值配置化，测试环境可缩短。

Group WS 第一版即使用 1 MiB read limit、60s pong wait、50s ping period、10s write wait 的生产默认值。peer Ping 只向有界 control queue 排队 Pong，由同一个 writePump 写出；readPump、Hub 和 inbound handler 均不直接写连接。

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

Checkpoint 5 起，message insert 与 sender/receiver 两条 session upsert 在同一 MySQL 事务中提交；ACK 只能在该事务提交后排队。双方 session 行按 `user_id` 固定顺序更新以降低反向并发死锁风险，receiver 使用数据库原子增量累计 unread，sender 不增加自己的 unread；`last_message_id/at` 只在更大 message ID 到达时推进。重复 `(sender_id, client_msg_id)` 读取原消息，不再次修改 session/unread 或投递。当前实时私聊统一发布到 `gim:chat:delivery`，各 Chat API 实例只向本实例 receiver Clients 投递；同实例也不额外直投，避免本地路径与 Pub/Sub 重复。

Session list 由 Chat Repository 一次 JOIN `chat_sessions/users/chat_messages` 获取 peer 基础资料和 `last_message_id` 对应消息，避免逐会话 User RPC/Message N+1。History 使用规范化会话双方 ID + message ID cursor；当前用户的 `chat_message_deletions` 仅过滤自己的视图。Mark Read 在事务内锁定用户自己的 session，单调推进 `last_read_message_id` 并按剩余入站消息重算 unread；它不是发送给 peer 的已读回执。

## 6. 群聊发送 Service

参数验证 -> 群成员检查 -> 全员禁言/个人禁言 -> clientMsgId 幂等 -> 落库 -> 更新每个成员 session/unread -> ACK -> Hub 广播当前在线成员。

群成员较多时 V1 可同步更新；需把“未来异步扇出/大群优化”写入已知限制，不在 V1 提前引 Kafka。

Day 3 / Checkpoint 3 的文本群消息已按上述边界实现：Group Service 负责 active/member/all-mute/personal-mute policy，消息 Repository 在同一 MySQL 事务中锁定 active group、确认 sender membership、插入 `group_messages`，并按事务内当前成员快照批量 upsert `group_sessions`。sender unread 不增加，其他成员无论在线与否均原子 `+1`；重复 `(send_user_id, client_msg_id)` 在新事务读取胜者，不更新 session、不重复发布。

事务提交后只向 originating Client 排队 persist ACK，再将携带成员快照的事件发布到独立 `gim:group:delivery`。每个 Group API 实例只向自己的本地 Clients 投递；sender 其他设备和 originating Client 都可收到 `group.message`。Redis/本地慢客户端失败不撤销 MySQL 或 ACK，history 是恢复事实源。群 session 一次 JOIN groups/last_message，history 使用 group/message-ID cursor 并按当前用户过滤 deletion；Mark Read 锁定用户自己的 session、单调推进并重算 cursor 后的他人消息 unread。

在 Group WebSocket 之前，Group foundation 先提供独立的 Domain/Repository/Service：角色枚举禁止魔法数字；创建群保证 group 与 owner membership 原子；批准申请保证 request 状态与 member 原子；owner/admin/member policy 集中测试；成员列表通过 User 批量 RPC 补齐公开资料。后续 Group Message Service 只能复用 `IsMember/GetMemberRole/IsMuted/GetGroup`，不得自行复制 membership SQL。临时个人禁言以 membership ID 为 key 使用 Redis TTL，不写 MySQL 过期时间。

## 7. Context

HTTP handler 传入的 ctx 应继续传到 RPC/DB；禁止业务路径无理由 `context.Background()`。后台生命周期任务除外。

## 8. 事务边界

至少以下操作需要事务：

- 私聊消息 insert + session 更新；
- 群消息 insert + session 更新；
- 好友验证同意 -> 建好友关系 + 更新验证状态；
- 创建群 -> group + creator member + 邀请成员；
- 文件物理对象与 user_file 关系建立（结合文件落盘补偿）。

### File Service Backend Foundation（Day 3 / Checkpoint 5）

File 使用 `UserFile -> FileObject -> Local Storage Object` 三层模型。Domain 与 Repository 不依赖 multipart；HTTP 只负责读取可信 `User-ID`、建立有界 multipart reader 和输出安全 DTO，Service 负责流式 SHA-256/MIME、同 hash 串行化、物理提交、数据库事务和补偿。

上传从 storage root 内的 `temp/` 写入，因此最终发布与 temp 位于同一文件系统；最终路径固定为 `objects/{sha[0:2]}/{sha[2:4]}/{sha256}`，不使用用户文件名。服务启动创建并验证 `temp/objects` 可写，同时清理上一进程遗留的 `upload-*.part`。当前 V1 部署边界是一个 File API 实例配一个本地 storage root；多实例共享对象存储/分布式 hash lock 属于后续部署演进。

物理文件发布后，Repository 在一个 MySQL 事务内以 `file_objects.sha256 UNIQUE` 解析/创建 FileObject，再以 `(user_id,file_object_id,original_name) UNIQUE` 解析/创建 UserFile。DB 失败时只删除本请求刚发布且数据库中仍不存在对象的文件；已有共享对象永不因第二个用户引用失败而删除。进程崩溃可能留下按 hash 定址但尚无 DB 行的对象，后续相同内容上传会原位采用并补齐数据库关系，不形成随机不可寻址垃圾。

V1 删除只删除当前用户的 UserFile；不立即删除 FileObject/物理文件，避免误删共享内容。孤儿统计与延迟 GC 留到后续管理能力。下载必须先按当前用户所有权解析 UserFile，再由内部 storage path 打开；API 不接受或返回真实路径。

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
