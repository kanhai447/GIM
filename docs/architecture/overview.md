# GIM V1 Architecture Overview

## Scope

GIM V1 是单主机、单节点基础设施上的可运行 IM 系统。它验证服务拆分、统一鉴权、双 WebSocket 实时链路、消息可靠性、文件流式处理、Admin 安全边界和异步审计；不声称具备生产级多活或无限扩展能力。

## Runtime topology

公开入口只有 Web、Admin 和 Gateway。MySQL、Redis、etcd、Kafka 的 Compose 调试端口只绑定 loopback，业务 API/RPC 不直接映射宿主机。

| Boundary | Responsibility | Stateful dependency |
| --- | --- | --- |
| Gateway | 路由、Auth 调用、身份头清洗/注入、HTTP/WS 反向代理 | etcd |
| Auth | 注册/登录/注销、JWT 验证、Redis 撤销 | User RPC、Redis |
| User | 用户资料、配置和 Admin 用户管理 | MySQL、User RPC |
| Chat | 私聊 WS、Presence、消息/会话/历史 | MySQL、Redis、User RPC |
| Group | 群管理、群聊 WS、消息/会话/历史 | MySQL、Redis、User/Group RPC |
| File | 流式上传、去重、列表、下载、引用删除 | MySQL、File volume |
| Settings | 公共配置、用户配置、Admin overview/settings | MySQL |
| Logs | Kafka consumer、审计落库、Admin 查询 | Kafka、MySQL |

etcd 保存服务租约，Gateway 和内部 RPC client 在请求时解析健康实例。Compose 中所有应用使用 Docker DNS 连接依赖，但业务 API 路由仍经过 etcd discovery。

## Authentication and authorization

1. 客户端只向 Gateway 发送 `token`；WebSocket 为兼容 V1 使用 query token。
2. Gateway 先删除客户端提供的 `User-ID`、`Role`、`ValidPath`。
3. Gateway 调用 Auth `authentication`，Auth 校验 JWT、有效期、撤销状态和用户状态。
4. Gateway 只把 Auth 返回的身份写入上游请求。
5. 业务服务从可信内部头读取 actor；Admin API 还在各目标服务验证 `Role == 1`。
6. WebSocket 在 Gateway/Auth 通过后才到 Chat/Group，业务 handler 在 Upgrade 前再次校验内部身份与 Origin。

正式部署必须保持内部 API/RPC 不直接暴露公网，否则攻击者可能绕过 Gateway 伪造内部身份头。

## Private and group message flow

Private Chat 与 Group Chat 使用不同 URL、Hub、Client、Repository 和数据表。发送流程保持相同可靠性骨架：

1. `readPump` 解析并校验 envelope、`clientMsgId`、目标和文本 payload。
2. service 在数据库事务内写消息、更新双方/成员会话与未读状态。
3. 唯一键将同一 sender 的同一 `clientMsgId` 收敛为同一条消息。
4. commit 成功后通过 Client send queue 返回 ACK，并向在线目标发布 delivery。
5. 离线或发布失败不回滚已经持久化的数据；历史、会话和未读是恢复来源。

ACK 的精确定义是“服务端已持久化/接受”。V1 没有接收端 delivery receipt 或 read receipt。

## Presence ownership

只有 Chat WebSocket 修改全局 Presence。Hub 维护同一用户的连接计数；第一条连接写 Redis TTL，活动连接周期刷新，最后一条连接离开才删除 Presence。Group WebSocket 不触碰全局在线状态，避免两条独立 socket 互相把用户误判下线。

## Data model

正式 schema 仅由 001–008 migration 管理，不依赖 AutoMigrate。Private/Group 使用分离的 message/session/deletion 表；session 表承担最后消息、排序、未读、置顶和隐藏语义，避免每次从大消息表动态聚合。File 使用 `file_objects` 与 `user_files` 分离内容对象和用户引用。Audit 以 `event_id` 唯一键支持消费者幂等。

## Deployment and verification

`compose.yaml` 启动 MySQL、Redis、etcd、Kafka、migration、RPC、API、Gateway、Web 和 Admin。migration 是一次性 job，storage-init 准备同文件系统的 temp/object 目录。完整启动、持久重启、服务恢复、文件持久性、Kafka degraded、性能 soak 和 Linux race 均有操作记录。

相关文档：

- [可靠性设计](reliability.md)
- [文件与审计](file-and-audit.md)
- [架构决策](decisions.md)
- [Compose 部署](../deployment/docker-compose.md)
- [性能基线](../performance/baseline.md)
- [HTTP API](../05_HTTP_API.md)
- [WebSocket 协议](../06_WEBSOCKET_PROTOCOL.md)
