# Architecture Decisions

| Decision | V1 choice | Why | Trade-off / next step |
| --- | --- | --- | --- |
| Authentication | `Gateway -> Auth -> Business Service` | 集中 token、撤销和用户状态判断，并保持参考协议兼容 | Auth 是请求链依赖；未来可评估 Gateway 本地验签，但不在 V1 |
| WS token | URL query token | 兼容浏览器和既有 V1 协议 | URL 可能进入代理记录；V2 应使用一次性 WS ticket |
| Socket topology | Private 与 Group 两条连接 | 隔离协议、Hub、权限和故障域 | 客户端连接更多；V1 不合并 |
| Presence owner | 仅 Chat WS | 避免两条 socket 互相误判在线状态 | 只开 Group 时不代表全局在线 |
| WS concurrency | Client pumps + Hub | 单写连接、并发安全、有界背压 | 慢消费者会被断开而不是无限缓存 |
| ACK | commit 后 server ACK | 语义清晰且可由数据库事实恢复 | 不代表接收方送达；无 delivery/read receipt |
| Idempotency | sender + `clientMsgId` 唯一键 | ACK 丢失时安全重试 | 客户端必须稳定复用 ID |
| Message model | Private/Group 分表 | 权限、索引、fanout 和会话语义不同 | 重复部分实现；不强制统一 Conversation |
| Session model | 独立 session 表 | 支持未读/已读、置顶、隐藏和稳定排序，降低动态聚合复杂度 | 写消息事务需要同步维护会话 |
| File storage | 流式 + SHA-256 内容寻址 | 控制内存、自然去重、支持原子提交 | V1 缺少孤儿物理对象 GC |
| Audit | bounded best-effort Kafka | 故障时不阻塞核心 IM，Admin 有真实日志链路 | 无 outbox/DLQ/exactly-once，长故障允许丢失 |
| Admin authorization | Gateway identity + target role check | 前端路由不是安全边界，防止绕过/伪造 | 内部 API 必须保持非公网 |
| Deployment | single-host Compose | 可复现、适合演示和本地验收 | 无 HA/Kubernetes/TLS/Secret Manager |
| Race verification | Linux Docker | 兼容 64-bit CGO，结果可复现 | Windows 32-bit MinGW native race 不可用 |

## Independence decision

FIM 三仓库只用于理解业务行为、接口语义、页面流程、数据关系与缺陷场景。GIM 的 `server/`、`web/`、`admin/`、migration、测试和部署均独立编写；`reference/` 被 Git/Docker 排除，不参与 import、运行或构建。这里的“独立”描述工程依赖与开发方法，不作超出证据的法律性声明。
