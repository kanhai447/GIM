# GIM Interview Project Overview

## 60-second introduction

GIM 是我围绕即时通信可靠性重新实现的 Go 微服务项目。它包含 Gateway、Auth、User、Chat、Group、File、Settings 和 Logs 服务，以及用户 Web 和独立 Admin。核心不是“做出一个聊天页面”，而是把鉴权、WebSocket 并发、消息持久化 ACK、幂等、会话未读、Presence、流式文件、异步审计、迁移和可复现验证做成一套能解释的工程闭环。

私聊和群聊使用独立 WebSocket、Hub 和 Client；每条连接只有一个写 goroutine。消息先在事务中写消息与会话，commit 后才 ACK，`clientMsgId` 加唯一约束处理 ACK 丢失重试。在线状态只由 Chat 连接维护，避免两条 socket 互相误判。文件边读边计算 SHA-256 并写临时文件，做内容寻址去重。审计用 Kafka 有界 best-effort 链路，故障时不阻塞核心业务。

项目通过 Compose 全栈、001–008 空库迁移、Web/Admin 单测与构建、真实 Gateway 冒烟、5 分钟 soak 和 Linux race。最大已测是 200 个私聊 WS 连接；这是已测基线，不是容量上限。

## What I would demo

1. 从 Compose 状态开始，展示内部服务不暴露宿主端口。
2. 登录两个用户，展示 Chat/Group 的独立 WS、ACK、未读和历史恢复。
3. 重发同一 `clientMsgId`，说明数据库唯一键与返回原消息。
4. 上传相同内容文件，说明流式处理、hash 去重和 ownership。
5. 在 Admin Logs 展示结构化事件不包含密码、token、正文和存储路径。
6. 打开性能基线和 operation log，说明 deadlock 如何被 soak 发现、如何最小修复、如何用 Linux race 复核。

## Engineering highlights

- 明确 ACK 语义，避免把“服务端已持久化”混同为“接收端已送达”。
- 数据库唯一键承担最终幂等，前端 reconciliation 处理 pending/ACK/delivery/history 合并。
- WebSocket 使用单写模型、有界队列、心跳和幂等清理，而不是并发 goroutine 直接写连接。
- session 表不是为了修复虚构的 `MAX(msg_preview)` bug，而是为未读、已读、置顶、隐藏和稳定查询复杂度建立可扩展模型。
- 真实 soak 找到 MySQL 1213，并只对安全事务边界做有限、可取消的重建重试。
- 文件不用 `io.ReadAll`；审计不使用共享可变事件对象，也不记录敏感内容。
- Admin API 同时依赖 Gateway 权威身份和目标服务角色检查，前端路由不作为安全边界。

## Honest gaps

- Chat/Group 当前仅支持文本，File 尚未进入消息协议。
- 没有 receiver delivery/read receipt，pending 不跨浏览器刷新。
- 用户端好友管理闭环尚未完成。
- Audit 无 outbox/DLQ/exactly-once；文件无物理孤儿 GC。
- Compose 是单机单节点，无 HA、内部 TLS、Secret Manager 或 Kubernetes。
- 压力重启仍可能发生 commit 已成功但 ACK/实时推送丢失；客户端重试和历史可恢复。

这些限制是 V1 边界，不应该在面试中隐藏。更好的回答是说明风险、现有恢复路径，以及下一版本的优先级。

## Evidence map

- 架构：[architecture/overview.md](../architecture/overview.md)
- 可靠性：[architecture/reliability.md](../architecture/reliability.md)
- 文件与审计：[architecture/file-and-audit.md](../architecture/file-and-audit.md)
- 性能：[performance/baseline.md](../performance/baseline.md)
- API/WS：[05_HTTP_API.md](../05_HTTP_API.md)、[06_WEBSOCKET_PROTOCOL.md](../06_WEBSOCKET_PROTOCOL.md)
- 最终验收：[Checkpoint 5](../../agent_logs/operations/day-4/005_checkpoint-5_final-acceptance.md)
