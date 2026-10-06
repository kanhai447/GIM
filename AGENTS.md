# GIM 项目 Agent 总指令

本文件是 GIM 项目最高优先级开发约束。修改代码前必须先阅读本文件、`START_AGENT.md` 和 `docs/` 下全部文档。

## 1. 项目定位

GIM 是一个用于后端面试/简历展示的即时通信系统。参考源码：

- 后端：`fim_server-main`
- 用户端：`fim_web-master`
- 管理端：`fim_admin-master`

三个 FIM 仓库只作为**只读参考实现**。GIM 必须由 Agent 在新的 `server/`、`web/`、`admin/` 目录中自主编写。允许参考业务流程、接口语义、页面交互、数据关系和实现思路；禁止把 FIM 当成模板仓库直接迁移、整文件复制、批量改包名或机械执行 `fim -> gim` 替换。

新项目目标是：理解 FIM 后重新实现同类能力，并针对并发安全、消息可靠性、会话查询、未读、在线状态、文件上传和日志安全做工程化设计。最终代码必须能够由 GIM 自身的设计文档解释，而不是依赖“原项目就是这么写的”。

用户主要掌握后端，因此后端必须可解释、可测试；前端以稳定可用和协议一致为目标，不做无意义 UI 重写。

## 2. V1 固定技术栈

后端：Go、go-zero、GORM、MySQL、Redis、etcd、gRPC/go-zero RPC、Gateway、JWT、Gorilla WebSocket。

前端：Vue 3、TypeScript、Pinia、Vue Router、Axios、Element Plus、Vite。

服务边界保留：Auth / User / Chat / Group / File / Settings / Gateway / Logs。Kafka/Logs 不阻塞 Day 1–3 的核心 IM，但作为完整 V1 的 Admin 日志链路在 Day 4 完成。

## 3. 已敲定、不得擅自修改的设计

1. V1 继续采用 `Gateway -> Auth Service -> Business Service` 的认证链路；不要提前改成 Gateway 本地验 JWT。
2. V1 WebSocket 继续使用 URL query 中的 JWT/token 以兼容参考项目；一次性 WS Ticket 属于 V2。
3. 私聊和群聊保持两条 WebSocket，不在 V1 合并成单连接。
4. 私聊和群聊数据模型保持分离，不强制统一 Conversation 表。
5. 私聊和群聊各自使用 Hub/Client；禁止普通全局 map 被多个 goroutine 直接读写。
6. 每个 WebSocket Client 必须 `readPump + writePump`，业务代码禁止直接调用 `Conn.WriteMessage/WriteJSON`。
7. 在线 Presence 只由 Chat WebSocket 维护；Group WebSocket 不修改全局在线状态，避免两套 socket 互相误判。
8. 服务端 ACK 代表“服务端已成功持久化/接受消息”，V1 不实现接收方 delivery ACK。
9. `clientMsgId` 用于发送幂等；同一发送者的同一 clientMsgId 只能生成一条消息。
10. 最近会话新增 session 表维护；该优化用于支持未读/已读、置顶、删除语义和降低大消息表动态聚合复杂度，不得把它描述成修复原项目 `MAX(msg_preview)` bug（完整源码实际使用的是按时间取最新 preview 的子查询）。
11. 文件上传必须流式处理，不允许大文件 `io.ReadAll`。
12. V1 暂不要求 WebRTC、Kubernetes、多实例 WebSocket 跨节点路由；Kafka 日志链路按 Day 4 计划完成，但不得阻塞 Day 1–3 核心 IM。

## 4. 开发行为约束

- 从空的 GIM 工程骨架开始实现；不得先复制 FIM 再重构，也不得通过批量改名得到 GIM。每个模块先阅读参考行为，再依据 GIM 文档独立实现。
- 每次只实施一个 Phase；不得越过验收继续下一 Phase。
- 改协议/表结构前先对照文档，不得自行发明不兼容字段。
- 需要偏离文档时，先输出：原因、影响、备选方案、推荐方案，等待用户决定。
- 每个 Phase 结束必须输出：修改文件、数据库变化、API/WS 协议变化、测试命令、测试结果、剩余问题。
- **每完成一个可独立验收的操作，必须立即在项目根目录 `agent_logs/operations/` 下新增一份操作记录；未生成对应操作记录的工作一律视为未完成，不得进入下一操作或声称已完成。**
- 保留 Git 可回滚点，建议每个 Phase 一个独立提交。
- 不要为了“技术栈丰富”添加未被需求驱动的中间件。


## 4A. 参考源码使用红线

- `reference/` 永远只读，不得在其中开发 GIM。
- 禁止 `cp -r`、`rsync`、整目录拖拽、整文件复制后改名、批量 search/replace `fim` 为 `gim`。
- 禁止为了赶进度直接保留原实现中的全局 map WebSocket、共享可变日志对象、`io.ReadAll` 文件上传等已知问题。
- 可以参考：业务功能、HTTP/WS 语义、页面流程、服务边界、数据库关系、错误场景、消息类型。
- 可以保留兼容接口，但实现代码应按 GIM 文档重新组织和编写。
- 每个 Phase 的操作日志必须写明“参考了原项目的什么行为/接口”，并说明 GIM 的自主实现或改进点。
- 如果确需复用极少量通用常量/类型定义，应在操作日志中明确列出，并保证不包含秘密、作者私有信息或大段业务实现。

## 5. V1 必须完成

注册/登录/注销/JWT；用户资料和配置；好友搜索/申请/验证/删除/备注；在线状态和上线提醒；私聊、群聊；文本/图片/视频文件/普通文件/语音文件/回复/引用/撤回/@/图文；历史记录；最近会话；置顶；未读；文件上传/预览/Hash 去重；WS 心跳、断线清理、自动重连；消息 ACK 和幂等。

## 三仓库复审后的强制补充规则

1. 原项目参考源码共有三个仓库：`fim_server-main`、`fim_web-master`、`fim_admin-master`。不得忽略 Admin。
2. GIM 新项目必须区分 `server/`、`web/`、`admin/`；Admin 是独立 Vue 应用。
3. 在任何 GIM 首次 commit 前，先扫描 reference 源码中的证书私钥、JWT Secret、数据库密码、第三方登录 Secret、生产地址；禁止把这些真实值复制进 GIM Git 历史。
4. 原 Admin 的 `/api/data/statistic`、`/api/data/weather`、`/api/data/login_statistic` 仅为 Mock，Server 无对应接口。禁止把 Mock 当成已实现后端能力，禁止未经用户确认新增 `gim_data` 微服务。
5. 原 Kafka/Logs 链路有真实 Admin UI，GIM V1 保留日志能力，但必须修复共享可变 Pusher、敏感字段记录、HTML/v-html 渲染等问题。
6. 日志严禁记录明文密码、JWT、Authorization、Cookie、第三方 Secret、数据库凭证。所有日志事件必须结构化并按请求隔离。
7. Admin 前端路由角色判断不是安全边界；所有 Admin API 必须经过 Gateway/Auth，并由目标服务端验证角色。
8. 正式部署时内部 API/RPC 服务不得直接暴露公网，避免绕过 Gateway 伪造 `Role` Header。
9. 实际私聊 session 源码使用子查询获取最新 preview，不允许在说明中继续声称它是简单 `MAX(msg_preview)` bug；session 表优化的理由应基于复杂度、未读/已读、扩展性和删除语义。
