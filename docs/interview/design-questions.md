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
5. 好友申请实时推送？——V2 后期，当前 HTTP（K-3 历史表述已过时，实际以 K-15 为准；好友申请推送仍未做）。

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
