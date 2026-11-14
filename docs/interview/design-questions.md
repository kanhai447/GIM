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

## 3. 按功能的设计问答

### 3.1 功能：注册、登录、注销、JWT、logout 黑名单

**一句话回答**
- Auth 服务统一做密码哈希、JWT 签发/校验与 Redis 撤销；Gateway 每请求调 Auth，业务服务只信任注入的 `User-ID/Role`。

**需求与约束**
- 注册/登录/注销；JWT 无状态校验；登出后 token 立即失效；V1 固定“Gateway → Auth → 业务服务”。

**方案**
- 入口 `auth/transport/http/handler.go:Register/Login/Authentication/Logout`；service `auth/service/service.go:Register/Login/Authenticate/Logout`；`auth/token/jwt.go:Issue/Parse`；`auth/revocation/redis.go:Revoke/IsRevoked/Fingerprint`；`auth/credential/password.go:Hash/Compare`；用户数据经 `auth/userclient` → User RPC。
- 登出把 token SHA-256 指纹写入 Redis，TTL=JWT 剩余时间。

**关键决策与理由**
- 选择集中 Auth，因为撤权立即生效、业务服务零 JWT 依赖；代价是每请求多一跳、Auth 是关键路径。
- 选择 JWT + Redis 黑名单，因为无状态校验快、登出又能即时失效；代价是每请求一次 Redis 查询。
- 选择 bcrypt + 统一 1201 错误，因为防彩虹表/账号枚举；代价是登录 CPU 成本更高。

**权衡**
- 可用性 vs 一致性：Auth/Redis 故障时业务请求失败（503/超时），不降级为“放行”。
- 无状态 vs 可撤销：JWT 本身不可撤销，用黑名单补齐。

**失败模式与恢复**
- Auth 进程崩溃：Gateway 返回 503/1303；客户端重试。
- Redis 不可用：`Authenticate` 返回内部错误，拒绝请求（不绕过黑名单）。
- DB 不可用：登录/注册失败；已签发 token 仍可用到过期（除登出检查）。
- 客户端崩溃：token 仍在有效期内；可重新登录，旧 token 不强制失效（V1 无单点会话）。

**常见追问与回答要点**
1. 为什么不用 Refresh Token？——V1 目标是简单闭环；短/长 token 轮换属 V2 安全增强。
2. JWT 被盗怎么办？——登出/改密后黑名单；V1 没有设备管理/强制下线列表。
3. 为什么登出只存指纹？——不把原始 JWT 写入 Redis/日志，降低泄露面。
4. 多端登录？——允许多 token；登出只失效当前 token。
5. 角色怎么传？——JWT role → Auth → Gateway `Role` header → 服务端 `adminapi.Identity` 二次校验。

**已知边界**
- 内部 API/RPC 无 mTLS，依赖网络边界（E-2）。
- `/api/auth/open_login` 在 Allowlist 但无路由（A6）；不属于核心功能。

### 3.2 功能：用户资料、Admin 用户管理

**一句话回答**
- 用户资料只读公开字段；Admin 用户列表/限制/禁用走 `Role==1` 二次校验，列表用游标分页并读 Presence 补 `isOnline`。

**需求与约束**
- 资料读取不能泄露 `pwd_hash/open_id`；Admin 需要搜索、分页、限制聊天/加好友/建群、禁用/启用。

**方案**
- 资料：`user/transport/http/profile.go:ServeHTTP` → `user/service/service.go:GetUserByID` → `user/repository/mysql/repository.go:GetByID`。
- Admin：`user/admin/transport/http/handler.go:List/UpdateRestrictions/Disable/Enable` → `user/admin/service.go` → `user/admin/mysql/repository.go`；`adminapi.Identity` 要求 `Role=="1"`。

**关键决策与理由**
- 选择服务端二次校验 Role，因为 Gateway 头仍可能被内部误用；代价是每个 Admin handler 重复鉴权。
- 选择游标分页（`beforeId`+limit+1），因为避免深 OFFSET；代价是不能跳页。
- 选择 `user_confs` 按需 upsert，因为注册不初始化配置（A3），减少注册事务；代价是查询要容忍缺行。

**权衡**
- 管理能力 vs 隐私：列表只返回安全字段；但仍能看用户统计，需权限边界。
- 一致性：限制写入后，登录/发消息读取最新 `user_confs`；无缓存，读放大可接受。

**失败模式与恢复**
- DB 失败：列表/更新 500，无部分写入（限制更新单事务）。
- Presence 查询失败：`isOnline` 降级 false，不影响列表。
- 误禁用：Admin 可 `PUT /api/user/users` 恢复；登录立即读 status。

**常见追问与回答要点**
1. 为什么禁用不是删除？——保留消息/好友关系与审计；登录返回 1202。
2. 限制聊天如何生效？——发私聊前 `friends.CanChat` 读 `user_confs.curtail_chat`。
3. 分页为什么不用 total？——深 COUNT 成本高；用 limit+1/hasMore。
4. Admin 能改密码吗？——V1 不做；避免管理员接触凭据。
5. 多个 Admin 并发改同一用户？——后写覆盖，无乐观锁；管理操作低频。

**已知边界**
- `user_confs` 无用户侧读写 API/UI（K-9）；资料编辑 `PUT /api/user/user_info` 未实现。
- Admin 限制字段只有四项，不是完整偏好设置。

### 3.3 功能：好友搜索、申请、同意/拒绝、列表、备注、删除、详情

**一句话回答**
- 好友关系用双向两行（A→B、B→A）+ 方向性 remark；申请用 pending 生成列唯一键防并发重复，同意在事务内写双向行。

**需求与约束**
- 搜索不能返回自己/已禁用用户；申请不能重复；同意/拒绝只能接收者操作；备注只影响自己方向。

**方案**
- 入口 `user/friend/transport/http/handler.go`（`/api/user/search`、`/api/user/friends`、`/api/user/friend_info`、`/api/user/valid*`）。
- service `friend/friend.go:Search/SendRequest/Respond/ListFriends/FriendInfo/UpdateRemark/DeleteFriend`；repo `friend/repository/mysql/repository.go`。
- 表：`friends`、`friend_verifies`；Presence 只读。

**关键决策与理由**
- 选择双向两行，因为查询“我的好友”只需 `user_id=?`，简单；代价是写入双份、删除双份。
- 选择 pending 生成列唯一键，因为同一对用户只能有一条待处理申请（含反向）；代价是 schema 多两个生成列。
- 选择 `SELECT ... FOR UPDATE` 锁申请行，因为并发同意只能一个成功；代价是事务持有行锁。

**权衡**
- 读性能 vs 写复杂度：双行读简单；写要事务保证两行一致。
- 安全 vs 体验：搜索/详情把“不存在/非好友”统一 1115，防枚举；代价是错误信息不精确。

**失败模式与恢复**
- 同意时进程崩溃：事务回滚，申请仍 pending，可重试。
- 重复申请：唯一键 1062 → 1116；前端提示已有申请。
- 删除好友：双向 DELETE 单事务；0 行 → 1115。
- Presence 失败：`isOnline=false`，不影响好友列表。

**常见追问与回答要点**
1. 为什么不用单行 A/B 加 low/high？——双行查询更直接；代价是双写。
2. 并发反向申请？——pending 生成列唯一键仲裁。
3. 备注为什么方向性？——A 对 B 的备注不应影响 B 的视图。
4. 被删除好友还能发消息吗？——`AreFriends` 失败 → 2203。
5. 好友申请实时推送？——V2 后期，当前好友申请走 HTTP；WS 推送未实现。

**已知边界**
- 只有接收方向申请列表，无“我发出的”（K-4）。
- 好友申请实时推送未实现；V2 后期项见 `v2-scope.md`。

### 3.4 功能：私聊文本消息（重点）

**一句话回答**
- WS `chat.send` 进入 `SendPrivateMessage`，以 `(send_user_id, client_msg_id)` 唯一键幂等，在一个事务写消息 + 双方 session，提交后先 ACK 再 Redis 推送。

**需求与约束**
- 只允许好友；文本 ≤4096B、payload ≤16KB；ACK 表示服务端已持久化；接收方离线可拉历史；V1 无 delivery ACK。

**方案**
- 入口 `chat/client.go:readPump` → `chat/message/inbound.go:Inbound.Handle` → `chat/message/service.go:SendPrivateMessage`。
- 校验 `CanChat/GetByID/AreFriends`；`repository/mysql/repository.go:PersistWithSessions` 事务 INSERT + 按 userID 升序 upsert 两侧 `chat_sessions`。
- ACK：`Hub.SendToClient`；推送：`delivery.Publish` → `LocalDeliveryHandler` → `Hub.SendToUser`。

**关键决策与理由**
- 选择“事务后 ACK”，因为 ACK 必须代表已落库；代价是 ACK 延迟受 DB 影响。
- 选择“先 ACK 后广播”，因为发送方感知不能被 Redis 阻塞；代价是接收方可能晚于 ACK 收到。
- 选择 session 表而非每次聚合消息表，因为未读/置顶/隐藏需要独立状态；代价是发送时多写两行、写放大。
- 选择 clientMsgId 而非服务端 ID 做幂等，因为客户端重试前还没有服务端 ID；代价是客户端必须生成稳定 UUID。

**权衡**
- 一致性：消息+session 同事务，读路径简单；代价是单事务锁竞争。
- 实时性：Pub/Sub at-most-once，丢推送可用历史补偿；代价是实时性不保证。
- 复杂度：session 冗余 last_message/unread，换查询性能。

**失败模式与恢复**
- ACK 前崩溃：事务回滚，客户端超时重试；不产生半条消息。
- ACK 后崩溃：消息已提交，历史可查；接收方靠历史/重连恢复。
- Redis 推送失败：只记 `realtime delivery degraded`，ACK 已发；历史兜底。
- 接收方离线：`SendToUser` 投递 0 个连接；上线后拉历史。
- DB 1213：Chat 有 3 次重试；仍失败返回 2204。
- ACK 丢失：客户端 failed → 同 clientMsgId 重试 → 服务端返回原消息 ACK。

**常见追问与回答要点**
1. 多端怎么办？——同一 userID 多个 Client，`SendToUser` 向所有连接投递；ACK 只回发送连接。
2. 重复发送怎么办？——唯一键 + 预查；同内容返回原消息，不同内容 2205。
3. 为什么不用消息队列？——V1 规模与复杂度取舍；Pub/Sub + 历史已够，可靠投递是 V2。
4. session 和动态聚合的区别？——session 维护未读/置顶/隐藏，读 O(1)；动态聚合每次扫消息表。
5. 为什么 ACK 不包含接收方已读？——V1 ACK 语义仅服务端持久化（K-2）。
6. 消息顺序怎么保证？——单会话按自增 messageId 排序；并发事务提交顺序可能不同，但 ID 单调。

**已知边界**
- 无接收方 delivery/read receipt（K-2）；pending 不跨刷新（K-5）。
- 推送 at-most-once，Redis 故障会丢实时消息，靠历史恢复。
- 群聊无 1213 重试（A5/K-10），私聊有。

### 3.5 功能：私聊图片消息

**一句话回答**
- 先 `POST /api/file/image` 上传拿 `user_files.id`，再发 `type=2`；服务端反查归属/MIME/size 并覆写 URL 等元数据，绝不信任客户端字段。

**需求与约束**
- 仅图片 MIME；大小上限 `IMAGE_MESSAGE_MAX_BYTES`（默认 10MiB）；接收方需有下载授权；历史/WS 同构。

**方案**
- 上传 `file/transport/http/handler.go:UploadImage` → `file/service/service.go:Upload`。
- 发送 `chat/message/service.go:normalizeImage`：`GetOwnedImage`（`user_files JOIN file_objects`，限本人）→ 校验 `image/` 前缀、size → 覆写 `url=/api/file/{id}`、MIME、size。
- 下载授权 `file/repository/mysql/repository.go:canAccessImage` 匹配 `imageMsg.fileID` 或 `fileMsg.fileID`。

**关键决策与理由**
- 选择“客户端只传 fileID”，因为 URL/文件名/大小可伪造；代价是发送前多一次 DB 查询。
- 选择上传与发送分离，因为大文件上传不应占用消息事务；代价是可能产生未引用文件（K-7）。
- 选择内容寻址 + `user_files` 引用，因为同一内容可复用；代价是授权需跨表 JSON 查询（E-8）。

**权衡**
- 安全 vs 延迟：服务端反查增加一次查询，换不可伪造。
- 存储 vs 去重：SHA-256 去重省空间；需要 GC/引用计数。

**失败模式与恢复**
- 上传失败：无消息；客户端可重试上传。
- 上传成功但发送失败：留下未引用 `user_files`；K-7 无 GC。
- 发送时文件被删/无权：2211。
- 接收方下载失败：2503；前端显示失败/重试。
- 撤回后：授权 `NOT EXISTS` 使非所有者 2503，所有者仍可下载。

**常见追问与回答要点**
1. 为什么不在消息里存二进制？——消息表膨胀、事务/推送成本高；对象存储/文件系统更合适。
2. 如何防伪造 URL？——服务端覆写，客户端 URL 被忽略。
3. 图片安全扫描？——V1 不做内容解码/病毒扫描（K-11）。
4. 缩略图？——未实现；预览直接用原图。
5. 多接收方授权？——私聊双方/群成员，按消息引用查询。

**已知边界**
- 宽高仅客户端提示，服务端不解码（K-11）。
- 下载授权 JSON 查询无索引/无限速（E-8）；浏览器缓存不回收（E-9）。

### 3.6 功能：私聊普通文件消息

**一句话回答**
- 与图片同构：上传拿 `user_files.id`，发 `type=3`；服务端反查 original_name/size/MIME 并覆写，接收方走鉴权下载。

**需求与约束**
- 不限 MIME 前缀；`FILE_MESSAGE_MAX_BYTES`（默认 25MiB）；文件名/大小以服务端为准；下载 `Content-Disposition: attachment`。

**方案**
- 上传 `file/transport/http/handler.go:UploadFile` → `file/service/service.go:Upload`。
- 发送 `chat/message/service.go:normalizeFile`：`GetOwnedFile` → 校验 size → 覆写 `url/fileName/size/mimeType`。
- 下载 `file/service/service.go:GetAccessible` → `http.ServeContent`，`nosniff` + attachment。

**关键决策与理由**
- 选择服务端回填文件名，因为客户端文件名可含路径/控制字符；`SanitizeFilename` 已处理上传名。
- 选择 attachment + nosniff，因为用户文件可能是 HTML/SVG，避免 XSS。
- 选择与图片共用 `canAccessImage` 查询，因为授权逻辑相同；代价是函数名不直观。

**权衡**
- 通用性 vs 安全：允许任意 MIME 提升可用性；靠下载头与鉴权降低风险。
- 存储成本 vs 体验：原文件保存，不做转码/预览。

**失败模式与恢复**
- 上传中断：临时文件 Abort；无 DB 引用。
- 发送时文件无权：2215；超限 2216。
- 下载时引用被删/撤回：2503。
- 物理对象丢失但 DB 有记录：下载 2504/500；需人工修复。

**常见追问与回答要点**
1. 大文件会 OOM 吗？——流式 `io.CopyBuffer` + `MaxBytesReader`，不 `ReadAll`。
2. 文件名如何防路径穿越？——`path.Base` + 控制字符校验 + 存储路径由 hash 生成。
3. 断点续传？——未实现；V2 候选。
4. 文件秒传？——SHA-256 命中复用物理对象，但仍建用户引用。
5. 删除文件会影响历史消息吗？——会 404；V1 无级联/软删。

**已知边界**
- 无孤儿 GC（K-7）；授权查询无索引/无限速（E-8）；浏览器缓存（E-9）。

### 3.7 功能：私聊撤回（重点）

**一句话回答**
- `type=4` + `withdrawMsg.msgID` 作为一条新消息落库；校验目标/本人/tombstone/已撤回/时间窗后，历史读时把原消息替换为 `[撤回]`，文件授权用 `NOT EXISTS` 排除被撤回原消息。

**需求与约束**
- 只能撤回自己消息；窗口 `RECALL_WINDOW_SECONDS`（默认 120s）；历史不返回原文；文件接收方失效；无 schema 变更。

**方案**
- 入口 `chat/message/inbound.go:Inbound.Handle` → `service.SendPrivateMessage` → `sendWithdraw`。
- 校验 `GetWithdrawBySenderAndTarget`（幂等）→ `GetByIDForRecall`（含 tombstone）→ 参与者/本人/tombstone/`IsRecalled`/窗口。
- 落库仍走 `PersistWithSessions`（type=4，preview `[撤回]`）。
- 历史/会话 `recalledSet` → `markRecalled`；文件授权 `canAccessImage` 的 `NOT EXISTS`。

**关键决策与理由**
- 选择“撤回消息作为一等消息”，因为天然进入消息序列、多端同步、可 ACK；代价是占 unread/last_message。
- 选择读时投影而非改写原消息，因为避免批量更新历史/二次广播；代价是每次历史查询多一次 JSON 反查。
- 选择 `NOT EXISTS` 撤销文件授权，因为不改 schema、保留原消息审计；代价是无索引、非所有者下载变慢（E-8）。
- 选择“同 sender+target 先查已有撤回”，因为顺序重试幂等；代价是并发不同 clientMsgId 仍可能两条（K-14）。

**权衡**
- 审计 vs 存储：原消息保留，存储不回收；合规/追责友好。
- 一致性 vs 并发：无 target 唯一索引，换 schema 简单；代价是 K-14。

**失败模式与恢复**
- 撤回落库前崩溃：无撤回消息，原消息可见，客户端可重试。
- ACK 后推送失败：历史已替换，接收方拉历史可见。
- 文件下载并发撤回：取决于查询时序；撤回后新查询 2503。
- Redis 挂：撤回仍落库/ACK，推送失败靠历史。
- 目标被 Admin tombstone：返回 2221（视为不可撤回）。

**常见追问与回答要点**
1. 为什么原消息不物理删除？——审计/幂等/多端同步；代价是存储。
2. 历史接口怎么保证不泄露原文？——读侧 `RecalledTargets` 替换 payload，preview `[撤回]`。
3. 撤回后文件还能下载吗？——非所有者 2503，所有者仍可下载自己的 `user_files`。
4. 超过窗口怎么办？——2220；前端入口隐藏只是 UX，服务端权威。
5. 并发撤回两条怎么办？——历史占位仍唯一，前端 `applyWithdrawals` 合并；严格一条需唯一索引（K-14）。
6. 群撤回呢？——同构，见 3.10。

**已知边界**
- K-14 并发可能两条；K-13 无管理员撤回；K-12 已实现。
- 文件授权查询无索引/无限速（E-8）；浏览器缓存（E-9）。

### 3.8 功能：群创建、成员、角色、邀请、申请、禁言

**一句话回答**
- 群与成员用 `groups/group_members`，角色 1 owner/2 admin/3 member；建群/加成员/审批在事务内锁群行控制人数上限；禁言只放 Redis TTL。

**需求与约束**
- 群主/管理员/成员权限矩阵；入群策略 0–4；成员上限；禁言可过期；V1 单机。

**方案**
- 入口 `group/transport/http/handler.go:Create/Members/Invite/RemoveMember/UpdateRole/Apply/Review/Mute`；service `group/service/service.go` + `policy.go:canManageJoin/canInvite/canActOn`；repo `group/repository/mysql/repository.go`；禁言 `group/mute/store.go`。

**关键决策与理由**
- 选择锁 `groups` 行 + 重新 COUNT 控人数，因为单库内正确、成本低；代价是群行热点。
- 选择禁言 Redis TTL，因为天然过期、读快；代价是 Redis 丢失即禁言丢失、无审计历史。
- 选择邀请直接入群（无待接受），因为实现简单；代价是缺少被邀请人同意语义。

**权衡**
- 一致性 vs 并发：行锁保证不超员，但大群邀请串行化。
- 功能完整度：入群问题/答案未真正校验；`request_type=2` 未使用。

**失败模式与恢复**
- 审批中崩溃：事务回滚，申请仍 pending。
- 并发加入：唯一键 2303，人数上限 2313。
- Redis 挂：禁言不可用（2311），发送侧 `IsMuted` 失败按不可用处理。
- 踢人与在途发送：发送事务仍可能解析到旧成员，存在窗口。

**常见追问与回答要点**
1. 群主能退群吗？——不能（2310），只能解散/转让（V1 无转让）。
2. 管理员能踢管理员吗？——不能，`canActOn` 只能动成员。
3. 禁言为什么不入库？——临时状态、TTL 自然过期；代价是不可审计。
4. 全员禁言怎么开？——当前无写入入口（A10）。
5. 邀请需要确认吗？——V1 直接入群，V2 可加邀请确认。

**已知边界**
- A10 全员禁言无写入；邀请确认/入群问题未实现；禁言 key 随成员 ID 残留至 TTL。

### 3.9 功能：群文本/图片/文件消息

**一句话回答**
- 与私聊同构：`group.send` → `Service.Send`，事务内写 `group_messages` + 全群 `group_sessions`，提交后 ACK，再 Redis Pub/Sub 按成员扇出。

**需求与约束**
- 仅群成员；禁言/全员禁言；文本/图片/文件校验同私聊；每成员 session 未读；V1 无读扩散优化。

**方案**
- 入口 `group/ws/client.go:readPump` → `group/message/inbound.go:Inbound.Handle` → `service.go:Send`。
- `PersistWithSessions`：锁 active 群 → `group_members.id` → INSERT 消息 → `Pluck` 成员 → `upsertSessions`；`delivery.Publish` → `LocalDeliveryHandler` → `Hub.SendToUser`。

**关键决策与理由**
- 选择写扩散 session（每成员一行），因为未读/置顶查询 O(1)；代价是 O(成员数) 写放大。
- 选择事务后 ACK + 异步广播，因为不丢已确认消息；代价是实时性依赖 Redis。
- 选择 Redis Pub/Sub 而非持久队列，因为实现简单；代价是 at-most-once、无重投。

**权衡**
- 读 vs 写：大群读快、写慢；V1 接受。
- 一致性：成员快照在事务内，发送后新成员看不到该消息的实时推送，但历史可见。

**失败模式与恢复**
- 事务失败：无消息，客户端重试；1062 回查赢家。
- ACK 后广播失败：历史兜底。
- 接收方离线：上线拉历史。
- Redis 挂：实时推送失败，消息仍在。

**常见追问与回答要点**
1. 大群 1000 人怎么办？——当前 O(N) 写放大；需要异步扇出/读扩散，属优化。
2. 多设备会重复吗？——每连接一份，未做设备去重。
3. 发送者会收到自己的消息吗？——会，广播包含发送者。
4. 群成员被踢后还能收吗？——在途消息可能仍推给已连接客户端，历史权限以当前成员为准。
5. 群消息顺序？——按自增 messageId 排序。

**已知边界**
- 无 1213 重试（A5/K-10）；Pub/Sub 丢实时消息；session 写放大。

### 3.10 功能：群撤回

**一句话回答**
- 复用 `type=4`，额外校验群归属与“当前仍是成员”；历史/会话替换、文件授权排除，和私聊同构。

**需求与约束**
- 目标属于本群；只能撤回自己消息；窗口/已撤回/tombstone 同私聊；退群后不能撤回。

**方案**
- `group/message/service.go:sendWithdraw` → `GetByIDForRecall/IsRecalled/GetWithdrawBySenderAndTarget` → `PersistWithSessions`；历史 `recalledSet/markRecalled`；授权 `file/.../canAccessImage` 的 `NOT EXISTS`。

**关键决策与理由**
- 选择把成员校验放在最后，因为目标/作者/窗口更基础；代价是退群用户得到 2404 而非 2419。
- 选择与私聊相同的读时投影，因为复用语义；代价是群历史查询多一次 JSON 反查。
- 选择不删除文件引用，因为审计与 owner 下载；代价是文件仍存在。

**权衡**
- 一致性与权限：退群后不能撤回，避免“离群仍操作”；和“作者永久可撤回”语义不同。

**失败模式与恢复**
- 同私聊：撤回消息事务失败无副作用；推送失败历史兜底。
- 管理员删除原消息：目标 tombstone → 2422。
- 并发不同 clientMsgId：K-14。

**常见追问与回答要点**
1. 退群后能撤回旧消息吗？——不能（2404）。
2. 群解散后？——群成员关系删除，撤回失败。
3. 群主能撤回别人消息吗？——不能，只有本人。
4. 撤回后群文件还能下载吗？——非成员/非所有者 2503，owner 仍可。
5. 和私聊撤回代码复用吗？——同构但独立 package/错误码 2419–2423。

**已知边界**
- K-14、K-13；无管理员撤回；文件授权无索引（E-8）。

### 3.11 功能：Session 列表、未读、已读、置顶、删除（重点）

**一句话回答**
- 每个用户/会话一行 `chat_sessions`（群为 `group_sessions`），冗余 last_message/unread/is_top/hidden_at；已读用消息行+session 行锁 + 重算未读。

**需求与约束**
- 会话列表按置顶/时间排序；未读准确；已读单调；置顶；隐藏会话；V1 不做 OFFSET 深分页。

**方案**
- 入口 `chat/message/transport/http/handler.go:Sessions/Read/Top`；service `ListSessions/MarkRead/SetSessionTop/ToggleSessionTop/HideSession`；repo `repository/mysql/session.go`。
- 列表 JOIN users/last message；`MarkRead` 锁消息行 → 锁 session → max(lastRead) → COUNT 未读 → 更新。
- 新消息 `upsertSession` 增加 unread、清 hidden_at。

**关键决策与理由**
- 选择 session 表而非动态聚合，因为未读/置顶/隐藏是独立状态；代价是写放大。
- 选择已读游标单调 max + COUNT 重算，因为并发消息下不会把新消息误清零；代价是每次已读一次 COUNT。
- 选择隐藏而非删除，因为对端不受影响；代价是表里保留 hidden 行。

**权衡**
- 读性能 vs 写成本：读 O(1)，写多两行/多行。
- 准确 vs 性能：未读 COUNT 保证准确；大会话可改为增量，V1 未做。

**失败模式与恢复**
- 已读请求失败：客户端重试；游标只前进不后退。
- 新消息与已读并发：session 行锁 + 事务顺序保证不丢未读/不误清零（仍可能出现“已读后新消息”正常未读）。
- 隐藏后新消息：`upsertSession` 清 hidden_at 重新出现。
- 进程崩溃：已提交的 session 状态保留；未提交回滚。

**常见追问与回答要点**
1. 为什么不每次 COUNT 消息表？——列表会 N 次聚合；session 读快。
2. 未读会不会不准？——发送事务原子 +1，已读重算；并发下有行锁。
3. 置顶有数量限制吗？——V1 没有，产品可加。
4. 删除会话是删除消息吗？——不是，仅 `hidden_at`；A1/A2 说明单条删除表未使用、HideSession 无 HTTP 入口。
5. 多端已读同步？——当前无跨端 read receipt；各端拉 session。

**已知边界**
- K-5 pending 不跨刷新；A1/A2 删除表未使用、HideSession 未接线；无跨端已读回执。

### 3.12 功能：文件上传与下载授权（重点）

**一句话回答**
- 流式读取 + SHA-256 内容寻址 + 临时文件原子 rename；DB 里 `file_objects`（物理对象）+ `user_files`（用户引用）；下载按所有者/私聊参与者/群成员鉴权。

**需求与约束**
- 不能 `io.ReadAll`；大小限制；同一内容去重；不泄露存储路径；下载要防越权；V1 无病毒扫描。

**方案**
- 入口 `file/transport/http/handler.go:upload/Download`；service `Upload/GetAccessible/Open`；storage `local.go:CreateTemp/Commit/Abort`；repo `Attach/FindAccessibleByID/canAccessImage`。
- 上传：`MaxBytesReader` → `io.CopyBuffer(io.MultiWriter(temp,sha256,prefix))` → hash 去重 → `Commit`（Sync/Rename）→ `Attach` 事务。

**关键决策与理由**
- 选择“先落盘再写库 + 补偿”，因为大文件不能长时间占事务；代价是可能孤儿对象（K-7）。
- 选择 SHA-256 内容寻址，因为天然去重/秒传；代价是需要引用计数/GC。
- 选择 `user_files` 引用而非对象直接归用户，因为共享对象；代价是授权多一层 JOIN。
- 选择下载时鉴权而非公开 URL，因为消息/群成员关系是权限源；代价是每次下载查消息表（E-8）。

**权衡**
- 性能 vs 安全：授权查询无索引，非所有者下载慢；换不可越权。
- 存储 vs 成本：共享对象省空间；无 GC 会膨胀。

**失败模式与恢复**
- 上传中断：temp Abort，无 DB 记录。
- DB 失败：物理对象可能残留（K-7）；上传返回错误。
- 下载时 DB 失败：2504/500；不返回文件。
- 文件被删/撤回：2503。
- 浏览器缓存：E-9，服务端无法回收。

**常见追问与回答要点**
1. 为什么用 SHA-256 不用 MD5？——抗碰撞；代价是计算稍慢。
2. 原子 rename 跨文件系统？——temp 与 objects 同卷；跨卷 rename 会失败，部署需保证。
3. 如何防越权？——私有消息参与者/群成员 + `admin_deleted_at`/撤回 `NOT EXISTS`。
4. 秒传安全吗？——只复用物理对象，仍建用户引用；不泄露他人文件。
5. 删除引用会删对象吗？——不会，K-7 无 GC。

**已知边界**
- K-7 无 GC；E-8 授权无索引/无限速；E-9 浏览器缓存；无内容扫描/缩略图。

### 3.13 功能：好友上线提醒（重点）

**一句话回答**
- Chat Hub 本地 0→1/1→0 回调只做非阻塞入队；有界 worker 查好友并 `Hub.SendToUser` 推 `friend.online/offline`；单机语义，Presence 仍是权威。

**需求与约束**
- 只通知好友；多端只首个/最后连接触发；不阻塞 Hub；不改 Presence 写入；V1 单机。

**方案**
- `chat/hub.go:registerClient/removeClient` → `cmd/chat-api` 的 `transitionObservers` → `chat/friendnotify/notifier.go:enqueue` → worker `handle` → `FriendIDs`（`friends JOIN users status=1`）→ `protocol.FriendPresenceData` → `Hub.SendToUser`。
- Presence：`chat/presence/service.go` 异步写 Redis ZSET；Group 不参与。

**关键决策与理由**
- 选择 Hub 只入队、worker 做 I/O，因为 Hub 事件循环不能被网络/DB 阻塞；代价是事件可能延迟/丢弃。
- 选择本地 0→1/1→0，因为天然多端去重；代价是多实例会重复/误报（K-15）。
- 选择只通知好友、跳过禁用/已删除，因为隐私与无效连接；代价是查询失败时不推送。
- 选择 best-effort，因为 Presence/HTTP 是权威；代价是崩溃/TTL 无 offline（K-16）。

**权衡**
- 实时性 vs 稳定性：异步队列满丢弃，换 Hub 不阻塞。
- 单机简单 vs 多实例正确：`GlobalPresenceCheck` 占位，V2 接 Redis 全局判断。

**失败模式与恢复**
- 进程崩溃：无 offline 事件；前端 HTTP `isOnline` 恢复。
- Redis 挂：Presence 写入重试；通知仍可发（只读好友表）。
- 好友查询失败：跳过该事件，不重试。
- 队列满：丢弃并计数，不阻塞。
- 多实例：可能重复/误报（K-15）。

**常见追问与回答要点**
1. 多端怎么只发一次？——Hub 本地连接计数 0→1/1→0。
2. 多实例怎么办？——需要 Redis 全局 Presence 判断，V2；当前 `GlobalPresenceCheck` 恒 true。
3. 如何防止刷事件？——有界队列 + 丢弃计数；未做限流。
4. 好友离线时发吗？——`SendToUser` 投递 0 个连接，不重试。
5. 为什么 Group WS 不参与？——Presence 唯一写入方是 Chat，避免两套 socket 误判。
6. 事件丢了怎么办？——HTTP `GET /api/user/friends` 的 `isOnline` 恢复。

**已知边界**
- K-15 单机；K-16 offline best-effort；V-20 多实例 NOT RUN。

### 3.14 功能：Admin 私聊审计、群管理、文件管理、设置、日志（重点）

**一句话回答**
- Admin 统一经 Gateway/Auth，服务端 `adminapi.Identity` 强制 `Role==1`；删除用 tombstone + 修复 session，群解散置 status 并隐藏 session，文件只删引用，设置固定 key，日志只读。

**需求与约束**
- 前端守卫不是安全边界；所有 Admin API 必须服务端校验；删除要可审计/可恢复；不能泄露 Secret。

**方案**
- 私聊：`chat/message/admin/transport/http/handler.go` → `service.List/Delete` → `mysql/repository.go:deleteOnce/repairSession`；错误码 2230–2232。
- 群：`group/admin/...` → `Dissolve/DeleteMessage/repairGroupSession`；错误码 2320–2322。
- 文件：`file/admin/...` 只删 `user_files` 引用。
- 设置：`settings/...` 固定 key upsert；日志：`logs/...` 只读查询。

**关键决策与理由**
- 选择 tombstone 而非物理删除，因为审计/恢复；代价是查询都带 `admin_deleted_at IS NULL`。
- 选择同步修复 session，因为列表不能指向已删消息；代价是删除事务变重。
- 选择 Admin 独立错误码 2230–2232/2320–2322，因为避免与撤回/消息码冲突（A4 修复）。
- 选择服务端二次校验 Role，因为 Gateway 头不是充分信任；代价是重复代码。

**权衡**
- 权限 vs 便利：所有操作要身份；Admin 前端只是 UI。
- 删除一致性 vs 延迟：同步修 session 强一致；大会话慢。

**失败模式与恢复**
- 删除事务失败：回滚，消息仍可见；可重试。
- 会话修复失败：整个事务回滚；不会半删除。
- 审计失败：best-effort，不影响删除。
- Redis Presence 失败：群在线人数降级，不影响主流程。

**常见追问与回答要点**
1. 为什么删除不是物理删？——审计/恢复；代价是存储。
2. Admin 能看私聊内容吗？——能，这是审计功能；读取本身不产生 LogEvent。
3. 删除消息会撤回已推送内容吗？——不会；客户端已收到的无法回收。
4. 解散群会删消息吗？——不删，只 status=2 + 删成员 + 隐藏 session。
5. 文件删除会删物理对象吗？——不会，只删引用（K-7）。
6. 错误码怎么区分？——Chat Admin 2230–2232，Group Admin 2320–2322。

**已知边界**
- A1/A2 删除表/HideSession 未接线；K-7 无 GC；私聊读取不审计；管理员撤回未设计（K-13）。

### 3.15 功能：审计日志（重点）

**一句话回答**
- 业务 handler 显式构造 `audit.Event`，经有界 producer 发 Kafka；consumer 先落库 `audit_logs` 再 commit，`event_id` 唯一键保证至少一次下的幂等。

**需求与约束**
- 不阻塞业务；脱敏；每请求 requestId；Kafka 不可用不能拖垮 IM；V1 无 outbox/DLQ。

**方案**
- `platform/audit/event.go:NewEvent/Validate`、`metadata.go:ValidateMetadata`、`request.go:EnrichHTTP`、`emit.go:EmitBestEffort`。
- `platform/kafkaaudit/producer.go:Publish/run`；`logs/consumer/consumer.go:Run`；`logs/repository/mysql/repository.go:Insert`。
- Gateway 生成 `X-Request-ID`；业务成功后才 emit。

**关键决策与理由**
- 选择异步 best-effort，因为审计不能阻塞 IM；代价是 Kafka 故障/队列满会丢事件（K-8）。
- 选择 metadata allowlist，因为通用脱敏容易漏；代价是新增字段要改代码。
- 选择至少一次 + `event_id` 唯一键，因为实现简单、可恢复；代价是重复消费。
- 选择显式 emit，因为只审计关键业务动作；代价是普通读请求无审计。

**权衡**
- 完整性 vs 可用性：允许丢审计，保业务；需要监控 `Dropped`。
- 安全 vs 灵活性：allowlist 安全，但扩展成本高。

**失败模式与恢复**
- Kafka 不可用：producer 重试 `MaxAttempts` 后丢弃；业务成功。
- consumer 崩溃：offset 未提交，重启重放；唯一键幂等。
- DB 失败：consumer 无限重试（阻塞分区），不丢但延迟。
- 队列满：`ErrQueueFull` + 计数；不阻塞。
- 进程崩溃：内存队列丢失，无 outbox（K-8）。

**常见追问与回答要点**
1. 能保证审计不丢吗？——不能，V1 best-effort；要可靠需 outbox/DLQ。
2. 密码/JWT 会进日志吗？——allowlist + 敏感 key 拒绝；登录失败只记安全信息。
3. 每请求都有审计吗？——不是；只有显式 emit 的业务动作。
4. 为什么先落库后 commit？——至少一次，重复靠唯一键；代价是可能重复消费。
5. Kafka 挂了业务会失败吗？——不会，`EmitBestEffort` 只记日志。
6. IP 有吗？——`Event.IP` 存在但 `EnrichHTTP` 未填；无可信代理列表。

**已知边界**
- K-8 无 outbox/DLQ；E-3 CI 不跑真实 Kafka；查询本身不审计；IP 为空。

### 3.16 功能：Migration 与 runner

**一句话回答**
- 001–008 嵌入式 SQL 是唯一 schema 真相；runner 先写 `schema_migrations.dirty=1` 再执行，成功后清 0，失败拒绝继续 Up；应用不使用 AutoMigrate。

**需求与约束**
- 可重复、可审查、可回滚；MySQL DDL 隐式提交；单实例部署；V1 无迁移锁。

**方案**
- `server/migrations/runner.go:New/Status/Up/Down/DownAll/applyUp/applyDown`；`server/cmd/migrate/main.go:run`；`schema_migrations(version,name,dirty,applied_at)`。

**关键决策与理由**
- 选择自研 runner，因为依赖少、命名/版本校验可控；代价是无 advisory lock/checksum。
- 选择 dirty 标记，因为能阻止半应用继续 Up；代价是恢复靠人工。
- 选择连续 3 位版本，因为防跳号/重名；代价是合并分支易冲突。
- 选择嵌入式 SQL，因为部署单二进制；代价是改迁移要重编译。

**权衡**
- 安全 vs 自动化：dirty 保护简单，但没有自动修复。
- 单实例 vs 并发：无锁；多实例迁移需外部协调。

**失败模式与恢复**
- 执行中途失败：dirty=1，后续 Up 拒绝；人工审查后 Down/修复。
- 并发 Up：可能同时看到 pending；一个版本插入冲突，需人工处理。
- DB 不可用：CLI 失败 exit 1；不影响已运行服务。
- Down 半失败：版本行未删，需人工核对。

**常见追问与回答要点**
1. 为什么不用 golang-migrate？——自研可控、依赖少；缺锁/校验和是代价。
2. dirty 怎么修？——人工确认 schema 后执行对应 Down 或手工修，再清 dirty。
3. 迁移能回滚吗？——有 Down，但 DDL 非事务；需备份。
4. 多实例同时迁移？——当前无锁，不推荐。
5. 008 做了什么？——只加 `admin_deleted_at`，不加索引。

**已知边界**
- 无迁移锁/checksum；`migrations/README` 已改为 008；`schema_migrations` 由 runner 建表（A9 已补文档）。

## 4. 故障与边界专题

### 4.1 ACK 后进程崩溃，消息会丢吗？
- 不会。ACK 只在 `PersistWithSessions` 提交后发送；已提交消息在 MySQL，重启后历史可查。未提交则客户端重试。
- 注意：ACK 丢失会导致客户端 failed，但重试命中唯一键返回原消息。

### 4.2 接收方离线，消息怎么恢复？
- 持久化在 `chat_messages/group_messages`；接收方上线打开会话拉 `GET /api/chat/history` / group history，按 messageId 游标分页。
- Redis Pub/Sub 只负责实时，不承担存储。

### 4.3 推送失败，发送方会知道吗？
- 不会。ACK 只代表持久化；推送失败日志 `realtime delivery degraded`，接收方靠历史恢复。V1 没有 delivery receipt（K-2）。

### 4.4 Redis 挂了，Presence 和好友通知会怎样？
- Presence 写入重试，`isOnline` 可能降级 false；好友通知 worker 仍查 MySQL 好友并尝试推送，但接收方连接在 Hub 内存，不依赖 Redis。事件丢失靠 HTTP `isOnline` 恢复。
- Redis 是 Presence 权威，但 IM 消息不依赖 Redis 持久化。

### 4.5 Kafka 不可用，审计会怎样？
- producer 重试后丢弃并计数，业务不失败；无 outbox/DLQ（K-8）。审计完整性降级，IM 不受影响。

### 4.6 并发发消息和 Admin 删除同一会话，会死锁吗？
- 可能竞争同一 session 行；两边都按 userID 升序，Chat 发送有 1213 三次重试，Admin 删除也有重试。极端竞争仍可能失败，返回错误可重试。

### 4.7 并发不同 clientMsgId 撤回同一消息，会怎样？
- 两个请求都可能通过 `IsRecalled` 检查并各写一条撤回消息（无 target 唯一索引），即 K-14。历史读侧仍只把原消息替换一次，前端 `applyWithdrawals` 合并撤回行；文件授权不受影响。严格一条需唯一索引/列。

### 4.8 文件上传中途失败，会留下垃圾吗？
- 临时文件 `Abort`；若进程崩溃，temp 可能残留；DB 无引用。已提交物理对象但 DB 失败时也可能残留，K-7 无 GC。当前接受人工/后续 GC。

### 4.9 退群后还能下载群历史图片吗？浏览器缓存呢？
- 服务端：`canAccessImage` 要求当前仍是 `group_members`，退群后非所有者 2503；撤回/tombstone 也被排除。
- 浏览器：已缓存副本无法回收（E-9）；只能保证新的服务端下载被拒。

### 4.10 多实例部署，好友上线提醒会怎样？
- 每个实例只看到本地连接，0→1/1→0 可能重复/误报；`GlobalPresenceCheck` 当前恒 true，K-15。需要 Redis 全局 Presence 判断（V2），当前不声称多实例正确。

## 5. 主动说明的边界

| 边界 | 一句话解释/后续 |
|---|---|
| 多实例 Presence 未做（K-15） | 当前单机；后续接 Redis 全局 0→1/1→0。 |
| 完整性能阶梯/soak/restart/远端 CI 未跑 | 只有 HTTP c10 与 Private 10 conn 两档最小结果，不构成容量结论。 |
| 部分 E2E 脚本未入库（K-17） | 临时脚本执行后删除；仓库仅 4 个 smoke，复现需重写。 |
| Group 无 1213 重试（A5/K-10） | 目前只处理 1062；高竞争下可能失败，后续可复用 Chat 的重试。 |
| 删除表未使用、HideSession 未接线（A1/A2） | 表/服务预留，HTTP 未暴露；当前隐藏走 `hidden_at`。 |
| 撤回并发可能两条（K-14） | 历史占位唯一、前端合并；严格一条需 schema 变更。 |
| offline 事件 best-effort（K-16） | 崩溃/TTL 无事件；HTTP `isOnline` 兜底。 |
| 文件授权无索引/无限速（E-8） | 仅非所有者下载路径；后续可加索引/限流。 |

## 6. 简历表述建议

**可写**：真实实现的功能、真实验证项、可解释的设计（消息+session 事务、幂等、文件流式/授权、撤回、好友提醒、审计脱敏）。

**不可写**：百万连接、全量压测、分布式 WS 路由、多实例 Presence、生产级容量。

**Bullet 示例（均可在代码/文档找到依据）**

1. 设计并实现私聊/群聊消息链路：`clientMsgId` 唯一约束幂等、消息+session 同事务、提交后 ACK、Redis Pub/Sub 实时推送与历史补偿。
2. 实现流式文件上传（`io.CopyBuffer` + SHA-256 内容寻址 + 原子 rename）与按消息/群成员关系的鉴权下载，支持图片/普通文件消息。
3. 实现撤回消息 `type=4`：窗口校验、历史 `[撤回]` 占位、文件授权 `NOT EXISTS` 排除，前端多端占位合并。
4. 实现好友上线提醒（单机）：Hub 本地 0→1/1→0、有界异步 worker、只通知好友，Presence 与 HTTP `isOnline` 兜底。
5. 实现 Admin + Kafka 审计链路：服务端 Role 二次校验、metadata allowlist 脱敏、至少一次消费 + `event_id` 幂等落库。

> 面试时主动补一句：多实例 Presence、完整性能阶梯、部分 E2E 脚本未入库是已知边界，详见 `docs/v2-scope.md` 与 `docs/known-issues.md`。
