# GIM 面试设计问答

> 对应代码基线：`9d252df`；配套深挖文档：`docs/architecture-deep-dive.md`。
> 本文档讲“为什么这样设计、被追问怎么答”；文件路径/函数/编号的“在哪、怎么串”以深度文档为准。

## 目录

- [0. 使用说明](#0-使用说明)
- [1. 一分钟项目介绍](#1-一分钟项目介绍)
- [2. 横向问题](#2-横向问题)
- [3. 按功能的设计问答](#3-按功能的设计问答)
  - 3.1 认证
  - 3.2 用户资料与 Admin 用户管理
  - 3.3 好友闭环
  - 3.4 私聊文本消息
  - 3.5 私聊图片消息
  - 3.6 私聊普通文件消息
  - 3.7 私聊撤回
  - 3.8 群创建、成员、角色、邀请、申请、禁言
  - 3.9 群文本/图片/文件消息
  - 3.10 群撤回
  - 3.11 Session 列表、未读、已读、置顶、删除
  - 3.12 文件上传与下载授权
  - 3.13 好友上线提醒
  - 3.14 Admin：私聊审计、群管理、文件管理、设置、日志
  - 3.15 审计日志
  - 3.16 Migration 与 runner
- [4. 故障与边界专题](#4-故障与边界专题)
- [5. 主动说明的边界](#5-主动说明的边界)
- [6. 简历表述建议](#6-简历表述建议)

## 0. 使用说明

- **用途**：面试前按功能复习；被追问时先给“一句话回答”，再展开到文件/函数和取舍。
- **与深度文档的关系**：`docs/architecture-deep-dive.md` 回答“是什么、在哪、怎么串”；本文回答“为什么、被问怎么答、失败怎么办”。
- **阅读方式**：先读第 1、2 节建立全局；再按岗位/项目经历挑第 3 节的功能；最后用第 4、5 节准备故障与边界。
- **诚实边界**：每节末尾列已知边界。主动说“没做/没验证”比被问出来更好；不要背“百万连接/全量压测”。

## 1. 一分钟项目介绍

**项目定位**：GIM 是一个用于后端面试展示的即时通信系统，覆盖注册登录、好友、私聊、群聊、文件、撤回、好友上线提醒、Admin 与审计；不是生产级分布式 IM。

**技术栈/边界**：Go + go-zero REST + GORM + MySQL 8 + Redis + etcd + gRPC（仅 User）+ Gorilla WebSocket；Vue 3 + TS + Pinia；Admin 独立 Vue。服务边界为 Gateway / Auth / User / Chat / Group / File / Settings / Logs；V2 范围见 `docs/v2-scope.md`。

**三个可讲亮点**

1. **消息可靠性**：`clientMsgId` 唯一约束 + 事务后 ACK + 消息/双方 session 同事务；重复发送返回原消息（`chat/message/service.go:SendPrivateMessage`、`repository/mysql/repository.go:PersistWithSessions`）。
2. **文件流式与授权**：上传 `io.CopyBuffer` + SHA-256 内容寻址 + 临时文件原子 rename；下载按所有者/私聊参与者/群成员鉴权（`file/service/service.go:Upload`、`file/repository/mysql/repository.go:canAccessImage`）。
3. **好友上线提醒（单机）**：Hub 本地 0→1/1→0 → 有界 worker → 只通知好友；Presence 仍是权威（`chat/friendnotify/notifier.go`、`chat/presence/service.go`）。

**三个主动说明的边界**

- 多实例 Presence 全局判断未做（K-15）；当前单机语义。
- 完整性能阶梯/soak/restart/远端 CI 未跑；只有 HTTP c10 与 Private 10 conn 两档最小结果（`docs/validation-status.md`）。
- 部分 E2E 脚本为临时执行未入库（K-17），仓库只有 4 个 smoke。

## 2. 横向问题

### 2.1 Gateway 信任边界

**一句话**：V1 让 Gateway 调 Auth 中心校验，业务服务只信任 Gateway 注入的 `User-ID/Role`，不自行解析 JWT。

**展开**：`gateway/handler.go:scrubIdentityHeaders` 先删客户端伪造的身份头；`authclient.Client.Authenticate` 调 Auth `/api/auth/authentication`；成功后 `ServeHTTP` 写入 `User-ID/Role`；`proxy.Forwarder.ServeHTTP` 转发。`route/route.go:Parse` 只允许已知服务前缀。

**追问**：为什么不让 Gateway 本地验签？——V1 固定决策是集中 Auth，换取撤权立即生效、业务服务零 JWT 依赖；代价是每请求多一跳，且 Auth 成为关键路径。内部服务为何不能暴露？——否则可绕过 Gateway 直接伪造 `Role`；Admin 前端守卫不是安全边界，服务端 `adminapi.Identity` 还会二次校验 Role。

**边界**：内部 API/RPC 无 mTLS，依赖“只暴露 Gateway + 私有网络”（E-1/E-2）。

### 2.2 并发模型：Hub 单写 + 双 pump

**一句话**：每个 Chat/Group Hub 用单 goroutine 独占连接 map；readPump 只读、writePump 只写，业务层禁止直接写 Conn。

**展开**：`chat/hub.go:run` 内创建 `clients`，只有 `registerClient/removeClient/routeMessage` 读写；外部通过 `Hub.submit` 投递命令。`client.go:readPump` 解析帧交给 inbound；`writePump` 从有界 `send` 取消息写帧。`routeMessage` 对 `client.send` 用非阻塞 select，满则 `Dropped++` 并 `removeClient(..., DisconnectSlow)`。

**追问**：为什么不用全局 map + mutex？——锁竞争和死锁风险高，单写者把并发收敛到一个事件循环。慢客户端怎么办？——有界队列 + 主动摘除，避免一个客户端拖垮 Hub。业务直接写 Conn 有什么问题？——Gorilla 连接不支持并发写，会 panic/交错帧。

**边界**：慢客户端被摘除会触发 1→0，可能提前发 offline；这是可用性优先的取舍（K-16）。

### 2.3 幂等与 ACK

**一句话**：`clientMsgId` 由发送方生成，DB 唯一键兜底；ACK 只在消息+session 事务提交后发送，重复请求返回原消息。

**展开**：`chat/message/service.go:SendPrivateMessage` 先 `GetBySenderAndClientMsgID` 预查；`sameLogicalMessage` 不同内容返回 2205；`PersistWithSessions` 在事务内 INSERT，1062 冲突回查。ACK 由 `chat/message/inbound.go:Inbound.Handle` 在 service 返回后通过 `Hub.SendToClient` 发回。

**追问**：ACK 丢了怎么办？——客户端超时置 failed，可用同一 `clientMsgId` 重试；服务端命中已有消息返回原 ACK，不重复落库。为什么不用服务端生成 ID 做幂等？——服务端 ID 无法在重试前关联请求；`clientMsgId` 是客户端可复用的幂等键。重复发送不同内容？——返回 2205，不覆盖原消息。群聊也一样吗？——同构，唯一键 `(send_user_id, client_msg_id)`。

**边界**：ACK 只代表服务端持久化，不代表接收方已读（K-2）；pending 不跨刷新（K-5）。

### 2.4 事务边界

**一句话**：消息 + session、好友申请/同意、文件对象 + 用户引用三类操作放在同一事务；跨文件系统/Kafka/Redis 的操作明确不做分布式事务。

**展开**：Chat `persistWithSessionsOnce` 在一个事务写 `chat_messages` + 双方 `chat_sessions`；Group `PersistWithSessions` 写消息 + 全群 session；好友 `CreateRequest/Respond` 事务内写申请/双向好友；文件 `Attach` 事务内写 `file_objects` + `user_files`。

**追问**：为什么文件系统不在事务？——大文件长时间占事务不可接受；采用“先落盘再写库 + 失败补偿”，容忍孤儿对象（K-7）。为什么 Redis 推送不在事务？——Redis 不是消息存储，事务提交后 best-effort 推送，失败靠历史恢复。群发消息为什么不在一个事务里写所有成员？——是在同一个事务，但因此写放大 O(成员数)，大群需要异步/批量优化。

**边界**：跨服务（Auth/User/File）没有分布式事务；失败靠补偿/重试/最终一致。

### 2.5 锁顺序与死锁

**一句话**：session 更新按 `user_id` 升序，Chat 对 MySQL 1213 做 3 次有界重试；Admin 删除消息也用同一顺序。

**展开**：Chat `repository/mysql/repository.go:persistWithSessionsOnce` 构造两条 session 后按 UserID 排序；Group `upsertSessions` 排序后单条多值 upsert；Chat `PersistWithSessions` 捕获 1213 重试（最多 3 次、10/20ms 退避）；Admin 删除按 `sessionLockOrder` 升序锁 session。

**追问**：为什么升序能降低死锁？——所有事务以相同顺序拿行锁，破坏循环等待。Group 有重试吗？——**没有**，只有 1062 处理；这是 A5/K-10 的已知边界。Admin 删除和发送并发会死锁吗？——可能竞争同一 session，但顺序一致 + Chat 侧重试降低概率。

**边界**：Group 缺 1213 重试（A5/K-10）；重试不是无限，极端竞争仍可能失败。

### 2.6 错误码体系

**一句话**：按领域分段（11xx 用户/好友、22xx 私聊、24xx 群聊、25xx 文件等）；Admin 使用独立段，前端按 HTTP 状态 + 业务码提示。

**展开**：`docs/05_HTTP_API.md` 是错误码表；Chat 撤回 `2218–2222`；Chat Admin 第十六轮改为 `2230–2232`；Group Admin `2320–2322`；Group 撤回 `2419–2423`。业务错误由 `apperror.Business` 产生，`httpresponse.Failure` 输出 `{code,message}`，Gateway 透传业务错误。

**追问**：HTTP 200 还是 4xx？——业务错误通常 HTTP 200 + code，身份/权限用 401/403；这样前端统一处理。前端如何消费？——Axios 拦截器把 code 映射为提示，401/1203 清 session；具体码分支有限。为什么 Admin 要独立段？——避免与撤回/消息错误码语义冲突（A4 修复）。未知错误？——统一 500 + code 7，不泄露内部错误。

**边界**：没有统一的错误码 registry 文件，常量分散在各 transport/service；新增码需人工同步 docs/05。

### 2.7 Migration 与索引

**一句话**：001–008 版本化 SQL + 自研 runner，先写 dirty 再执行，禁止 AutoMigrate；关键唯一约束是幂等与并发的最终防线。

**展开**：`server/migrations/runner.go:Up` 对每个未应用版本 `applyUp`（先 dirty=1，执行 SQL，成功后清 0）；`schema_migrations` 记录版本/名称/时间；`clientMsgId`、`(user_id, peer)` session、`(group_id,user_id)` 成员等唯一键在 001–004。

**追问**：为什么不用 GORM AutoMigrate？——生产结构不可控、无法审查/回滚。dirty 怎么恢复？——人工审查后执行对应 Down 或手工修复，runner 拒绝继续 Up。并发 runner 会怎样？——没有 advisory lock，可能冲突；当前单实例部署可接受。索引够吗？——核心查询有 conversation/group cursor 索引；文件授权 JSON 查询无索引（E-8）。

**边界**：无迁移锁、无 checksum；DDL 非事务；008 只加列不加索引。

### 2.8 部署边界

**一句话**：Compose 单机 17 个服务，只有 Gateway/Web/Admin 对外，内部 API/RPC 只在私有网络；migration/storage-init 是一次性 job。

**展开**：`compose.yaml` 中 gateway 8080、web 5173、admin 4174 绑定 `127.0.0.1`；mysql/redis/etcd/kafka 的 loopback 端口仅调试；7 个 API 和 user-rpc 无 ports。`migration` 执行 `gim-migrate up` 后其他服务才启动；`storage-init` 准备文件卷权限。

**追问**：为什么内部服务不暴露？——防止绕过 Gateway 伪造 `User-ID/Role`。为什么用 Compose 而不是 K8s？——面试项目优先可复现；K8s/HA 是 V2 后期（E-1）。服务发现怎么做？——etcd + `registry.Register`，Gateway `resolver.Resolve` 取 `endpoints[0]`。健康检查？——各 API `/healthz`，user-rpc gRPC health。

**边界**：单机、无 HA/多实例 WS 路由；E-1/E-2。

### 2.9 V2 范围

**一句话**：V2 核心四 Phase 已完成（图片、普通文件、撤回、好友上线提醒单机）；其余明确列为 V2 后期/不做，不冒充完成。

**展开**：`docs/v2-scope.md` 第 1 节列四 Phase 与验证证据；第 2 节列 `user_confs`、管理员撤回、多实例 Presence、好友申请实时推送、完整性能阶梯、语音/视频/@/图文、WS Ticket、Kafka outbox 等范围外项及理由。

**追问**：为什么不做多实例 Presence？——需要 Redis 全局 0→1/1→0 与多实例环境；当前单机语义已文档化（K-15）。为什么不做管理员撤回？——需要独立 Admin 语义与审计，非核心（K-13）。性能怎么样？——只有 HTTP c10/10s 与 Private 10 conn/10s@10 msg/s 两档最小结果，不构成容量结论。E2E 都能复现吗？——部分临时脚本未入库（K-17）。

**边界**：范围外项未实现；不要把 V2.0 讲成“全部 V2 功能完成”。

（第 3–6 节见后续提交）
