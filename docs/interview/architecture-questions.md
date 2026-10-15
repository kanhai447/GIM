# Architecture Interview Questions

## Why keep Gateway calling Auth instead of verifying JWT locally?

V1 的决定是兼容现有链路，并把 token、撤销、用户状态和公开路径判断集中在 Auth。Gateway 先清洗身份头，再采用 Auth 返回结果。代价是 Auth 位于请求关键路径；未来若做本地验签，仍要解决撤销和用户禁用的一致性，不能只把公钥塞进 Gateway 就宣称等价。

## Why are Private and Group WebSockets separate?

两者的权限、fanout、数据模型和 session 语义不同。分开后 Hub、Client、Repository 和故障域都更清楚，也符合 V1 协议。代价是浏览器多一条连接；在没有明确规模收益前，不为“连接更少”牺牲边界清晰度。

## Why not use one Conversation table?

Private 是双方关系，Group 是成员集合和角色权限。强行统一会把大量 nullable 字段和条件分支带入热路径。V1 分表能给两类 history/session 建更直接的索引与约束。若未来需要跨域统一搜索，可以建立读模型，而不是先污染写模型。

## Why add session tables?

session 表承载最后消息、排序、未读、已读、置顶和隐藏恢复。理由是查询复杂度、扩展语义和删除模型，不是声称参考项目使用了错误的 `MAX(msg_preview)`；参考源码实际通过按时间子查询获取 preview。

## How is identity protected?

客户端提供的 `User-ID`、`Role`、`ValidPath` 在 Gateway 一律删除。Gateway 调用 Auth 后才写内部头。Admin 请求到达目标服务后还要校验 `Role == 1`。因此正式部署必须避免内部 API/RPC 暴露公网。

## Why does only Chat maintain Presence?

如果 Chat 与 Group 都写“上线/下线”，关闭任意一条 socket 都可能把仍在线的用户误判下线。让 Chat 成为唯一 owner 后，Presence 只与 Chat 多连接计数和 Redis TTL 相关，Group 连接生命周期不会干扰它。

## Why does Hub use a single event loop?

连接注册、注销和广播都串行修改 Hub 内部 map，可以把并发安全集中在一个所有权边界，而不是让业务 goroutine 共享普通 map。Client queue 仍允许消息并发产生，Hub 只负责调度。

## Why is writePump the only WebSocket writer?

Gorilla WebSocket 要求连接写入串行化。`writePump` 统一处理 data、Ping 和 Close，避免交错 frame 和 data race；业务代码只能向有界 queue 投递。

## Why Redis Pub/Sub for realtime delivery?

它延迟低、模型简单，适合把已持久化消息通知到其他 Chat/Group 实例。本地 Hub 再投递自己的连接。Pub/Sub 不是事实源，也不承担离线重放。

## Why not Kafka for realtime messaging?

V1 的实时路径需要按在线实例快速 fanout，而消息可靠性已经由 MySQL 保证。Kafka consumer group、分区顺序和连接路由会增加复杂度，但不能自动提供 receiver receipt。Kafka 在本项目只用于异步审计。

## Why is MySQL the message source of truth?

消息、幂等唯一键、session 和 unread 必须原子变化，关系型事务与索引能直接表达这组约束。Redis 投递失败或实例重启后，history/session 仍能重建客户端状态。

## Why have group_sessions?

它把每个成员的 last message、unread、read cursor、pin 和 hidden 状态显式化。写放大是可控代价，换来稳定查询和清晰的成员级语义。

## Why split FileObject and UserFile?

`FileObject` 表示按 SHA-256 去重的物理内容，`UserFile` 表示用户拥有的名称/引用。同一对象可被不同用户或名称安全引用，删除一条 ownership 不会误删共享内容。

## Why use etcd if Compose already has Docker DNS?

Docker DNS 解决容器寻址，etcd 验证了服务注册、租约和动态解析边界，并让 Gateway/RPC client 不硬编码业务实例。V1 是单实例，但实现和测试覆盖了 lease 替换与 shutdown cleanup。多实例跨节点 WS 路由仍不在 V1。

## Why is Admin a separate Vue app?

用户端和管理端具有不同依赖、路由、布局和发布入口。独立应用能减少权限 UI 与用户体验耦合，但安全仍由后端保证。原 Admin 的统计/weather/login statistic Mock 没有被伪装成真实后端服务；GIM dashboard 只展示真实聚合。

## What would change for production?

优先增加内部 TLS/网络策略、Secret Manager、可观测告警、数据库与 Kafka HA、备份恢复、资源限制、文件对象 GC、outbox/DLQ、容量规划和多节点 WS 路由。Kubernetes 是部署手段，不应早于这些明确问题。
