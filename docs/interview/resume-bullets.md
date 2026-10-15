# Resume Bullets

以下要点应按目标岗位取舍，避免一次堆入全部数字。

## Backend-focused

- 独立实现 Go 即时通信微服务，覆盖 Gateway/Auth/User/Chat/Group/File/Settings/Logs，使用 go-zero、GORM、MySQL、Redis、etcd、gRPC、WebSocket 与 Kafka，并通过 Docker Compose 提供可复现全栈环境。
- 设计 Private/Group 双 WebSocket 单写模型：Hub + `readPump/writePump` + 有界队列 + 心跳/断线清理；Linux Docker full/focused/真实依赖 race 验证均通过。
- 建立“事务提交后 ACK + `clientMsgId` 唯一键幂等 + session/unread 同事务维护”的消息可靠性链路；处理 ACK 丢失、重复发送、离线恢复和读游标竞态。
- 用 5 分钟真实负载定位 MySQL 1213 deadlock，并实现仅针对安全事务的最多 3 次重建重试；修复后 Private 1500/1500、Group 600/600 消息持久化与 ACK 全部匹配。
- 实现流式文件上传与 SHA-256 内容寻址去重，限制 50 MiB、服务端 MIME 探测、原子提交、ownership 和失败补偿，避免大文件 `io.ReadAll`。
- 构建 Kafka 结构化审计链路，以有界 producer 保证 Kafka 故障不阻塞核心 IM，消费者通过 `eventId` 唯一键幂等落库，并在目标服务执行 Admin 二次鉴权。

## Full-stack / delivery

- 交付 Vue 3 用户端与独立 Admin，使用 TypeScript、Pinia、Router、Axios、Element Plus/Arco/ECharts；最终 Web 16 files / 90 tests、Admin 11 files / 25 tests、类型检查和生产构建通过。
- 搭建 MySQL/Redis/etcd/Kafka KRaft/12 个应用进程/Web/Admin 的单一 Compose 栈，包含 001–008 migration、健康依赖、非特权镜像、named volumes、恢复与持久性验证。
- 建立负载工具、资源快照、Linux Docker race runner 和 GitHub Actions workflow；记录 200 个 Private WS 最大已测值与 100 连接 50 msg/s 下 13.47 ms ACK p95，明确不将其描述为容量上限。

## Short version

- Go IM 微服务：事务后 ACK、`clientMsgId` 幂等、双 WebSocket 单写模型、session/unread、Redis Presence、流式文件去重与 Kafka 审计。
- 通过 Compose 全栈、空库 001–008 migration、5 分钟 soak 和 Linux race；压力测试发现并修复 MySQL 1213 deadlock。

## Claims to avoid

- 不写“百万连接”“零丢失”“exactly-once”“生产级高可用”。
- 不把 server ACK 写成接收端送达/已读。
- 不把 200 个连接写成系统上限。
- 不声称富消息、好友闭环、Kubernetes、outbox、文件 GC 已完成。
- 不声称远端 GitHub Actions PASS；收口时只能确认 local equivalent PASS、remote NOT OBSERVED。
