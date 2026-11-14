# GIM 架构深度文档

> 基于仓库 `K:\it\gim`，代码基线 `2d849bc`（V2.0 收口后的文档提交；功能代码等同 `8e39668`）。本文只描述 GIM 自身源码，不引用 `reference/`。
>
> 约定：所有路径相对仓库根；函数引用格式为 `路径:函数名`。无法从代码确认的内容标注“推测”或“未找到”。验证口径见 `docs/v2-scope.md`，本文不写“全量验证通过/百万连接”。

## 目录

1. 进程与启动
2. HTTP 请求链路
3. WebSocket 连接链路
4. 按功能拆解
5. 数据模型与索引
6. 并发模型
7. 配置与部署
8. 测试地图
9. 已知边界与 V2 范围
10. 文件索引

---

# 1. 进程与启动

所有常驻服务都遵循同一骨架：`platform/process.LoadValues` → 各配置 `FromValues` → 打开依赖 → 构造 module/service → `rest.NewServer` + `RegisterRoutes` → etcd `registry.Register` → `signal.NotifyContext` → `ServeREST`。一次性命令（migrate、admin-bootstrap）不注册 etcd、不监听 HTTP。

| 入口 | main / run | 关键启动顺序 | 关键内部包 | etcd / 端口 / 健康检查 | 优雅关闭 |
|---|---|---|---|---|---|
| Gateway | `server/cmd/gateway/main.go:main` / `run` | `loadValues` → `platformetcd.FromValues/Open` → `gateway.New` → `mux`（`/healthz`、`/`）→ `http.Server.ListenAndServe` | `internal/gateway`、`gateway/route`、`gateway/proxy`、`platform/etcd`、`platform/process` | 不注册 etcd；监听 `GATEWAY_HOST:GATEWAY_PORT`（Compose 8080）；`/healthz` | `signal.NotifyContext` → `server.Shutdown(10s)` |
| Auth API | `server/cmd/auth-api/main.go:main` / `run` | `LoadValues` → `process.FromValues` → `kafkaaudit.NewProducer` → Redis/etcd/registration/rest 配置 → User RPC health → `auth.New` → `rest.NewServer` → `RegisterHealthz` + `authhttp.RegisterRoutes` → `registry.Register` → `ServeREST` | `internal/auth`、`auth/transport/http`、`platform/kafkaaudit`、`platform/redis`、`platform/discovery` | `AUTH` / `auth_api`；`AUTH_API_PORT`（8081）；`/healthz` | `ServeREST` 内置信号；defer 关闭 producer/registration/Redis/etcd |
| User API | `server/cmd/user-api/main.go:main` / `run` | 同骨架；额外 MySQL → `user.New`/`friend.New`/admin → 注册 `userhttp`、`useradminhttp`、`friendhttp` | `internal/user`、`user/friend`、`user/admin`、`platform/mysql`、`kafkaaudit` | `USER` / `user_api`；`USER_API_PORT`（8082）；`/healthz` | 同 Auth；audit producer defer close |
| User RPC | `server/cmd/user-rpc/main.go:main` / `run` | `LoadValues` → MySQL → `net.Listen` → `grpc.NewServer` → `usergrpc.Register` + gRPC health → `Serve` | `internal/user`、`user/transport/grpc`、`platform/mysql` | 不注册 etcd HTTP；监听 `USER_RPC_PORT`（9091）；gRPC health service | `gracefulStop(server, ShutdownTimeout)` |
| Chat API | `server/cmd/chat-api/main.go:main` / `run` | runtime/audit → Redis/etcd/registration/rest/presence/chat 配置 → MySQL + User RPC health → `presence.NewService` → `friendnotify.New` → `delivery.New` → `message.NewService` → `chat.NewWithInboundFactory`（组合 observer）→ delivery 订阅 → REST 路由 → `registry.Register` → 启动 Hub/Presence/notifier → `ServeREST` | `internal/chat`、`chat/presence`、`chat/friendnotify`、`chat/delivery`、`chat/message`、`platform/discovery` | `CHAT` / `chat_api`；`CHAT_API_PORT`（8083）；`/healthz`；WS `CHAT_WS_PATH=/api/chat/ws/chat` | `ServeREST` 后 `Hub.Shutdown` → `friendNotifier.Shutdown`（drain）→ `presenceService.Shutdown` → 等 delivery goroutine |
| Group API | `server/cmd/group-api/main.go:main` / `run` | runtime/audit → group/ws/mysql/redis/etcd 配置 → User RPC health → group module + message service + `groupws.NewWithInboundFactory` → delivery 订阅 → REST/WS 路由 → `registry.Register` → Hub/delivery 启动 → `ServeREST` | `internal/group`、`group/message`、`group/ws`、`group/ws/protocol`、`group/delivery` | `GROUP` / `group_api`；`GROUP_API_PORT`（8084）；`/healthz`；WS `GROUP_WS_PATH=/api/group/ws/chat` | `ServeREST` 后 `Hub.Shutdown` → 等 delivery goroutine；Group 不写 Presence |
| File API | `server/cmd/file-api/main.go:main` / `run` | runtime/audit → file 配置 → MySQL/etcd → `file.New(db,StorageRoot,MaxUploadBytes)`（storage+repository）→ REST + admin 路由 → `registry.Register` → `ServeREST` | `internal/file`、`file/service`、`file/repository/mysql`、`file/storage`、`file/admin` | `FILE` / `file_api`；`FILE_API_PORT`（8085）；`/healthz` | `ServeREST` 信号关闭；defer 关闭 MySQL/etcd/audit |
| Settings API | `server/cmd/settings-api/main.go:main` / `run` | runtime/audit → MySQL/etcd → `settings.New` + overview → REST 路由 → `registry.Register` → `ServeREST` | `internal/settings`、`settings/overview`、`platform/mysql` | `SETTINGS` / `settings_api`；`SETTINGS_API_PORT`（8086）；`/healthz` | `ServeREST`；defer 关闭依赖 |
| Logs API | `server/cmd/logs-api/main.go:main` / `run` | runtime → MySQL/etcd/Kafka 配置 → log service + audit consumer → REST 路由 → `registry.Register` → consumer goroutine + `ServeREST` | `internal/logs`、`logs/consumer`、`logs/service`、`platform/kafkaaudit` | `LOGS` / `logs_api`；`LOGS_API_PORT`（8087）；`/healthz` | `ServeREST` 后关闭 consumer（`Consumer.Close`）并等待 consumer goroutine |
| Migrate | `server/cmd/migrate/main.go:main` / `run` | flag/env → `mysqlclient.FromValues/OpenSQL` → `migrations.New` → `status/up/down/down-all` | `internal/platform/config`、`platform/database/mysql`、`migrations` | 无 etcd/HTTP；一次性命令 | 进程退出，DB 连接 defer close |
| Admin bootstrap | `server/cmd/admin-bootstrap/main.go:main` / `run` | `GIM_ADMIN_*` env 校验 → `process.LoadValues` → MySQL → bcrypt hash → `userservice.CreateUser`（RoleAdmin） | `auth/credential`、`user/service`、`user/repository/mysql`、`platform/mysql` | 无 etcd/HTTP；一次性命令 | 进程退出，连接 defer close |

**启动顺序要点**

1. `platform/process.LoadValues` 支持 `-env` / `GIM_ENV_FILE` / 进程环境（`server/internal/platform/process/*.go`）。
2. 常驻 API 先启动 audit producer，再打开 Redis/MySQL/etcd；启动失败直接返回，不进入监听。
3. Chat/Group 在注册 etcd 前完成 Hub/inbound 构造；Chat 额外启动 delivery 订阅、Presence worker、friendnotify worker。
4. 只有 Gateway、Web、Admin 对外暴露；内部 API 仅监听 Compose 内网。

# 2. HTTP 请求链路

```text
客户端
 -> Gateway Handler.ServeHTTP（清头/路由/Auth/身份注入/服务发现/反向代理）
 -> 目标服务 transport/http Handler
 -> service
 -> repository
 -> MySQL/Redis
```

## 2.1 Gateway 关键函数

| 步骤 | 位置 | 说明 |
|---|---|---|
| 清伪造身份头 | `server/internal/gateway/handler.go:scrubIdentityHeaders` | 删除客户端传入的 `User-ID`、`Role`、`ValidPath`、`X-Request-ID` |
| 生成/透传 RequestID | `handler.go:ServeHTTP` | `audit.NewRequestID`，写请求/响应头 |
| 路径解析 | `server/internal/gateway/route/route.go:Parse` | `/api/<service>/...` → `Target{PublicService, DiscoveryService}`；拒绝 `RawPath` |
| Auth 校验 | `server/internal/gateway/handler.go:ServeHTTP` → `authclient.Client.Authenticate` | 调用 Auth `/api/auth/authentication`，携带 token/path |
| 身份注入 | `handler.go:ServeHTTP` | 认证通过后写 `User-ID`、`Role` 到上游请求头 |
| 服务发现 | `server/internal/gateway/handler.go:ServeHTTP` → `resolver.Resolve` | 解析 `*_api` 的 etcd endpoint，取 `endpoints[0]` |
| 反向代理 | `server/internal/gateway/proxy/proxy.go:Forwarder.ServeHTTP` | `httputil` 风格转发；WS Upgrade 单独处理；超时/不可用映射 |

**服务映射**（`route.go:services`）：`auth→auth_api`、`user→user_api`、`chat→chat_api`、`group→group_api`、`file→file_api`、`settings→settings_api`、`logs→logs_api`。

## 2.2 代表性链路

### 登录：`POST /api/auth/login`

1. `server/internal/gateway/handler.go:ServeHTTP`：清头 → `route.Parse("auth")` → `authclient.Authenticate`（登录路径在 Allowlist，返回 `Authenticated=false` 也放行）→ 发现 `auth_api` → `proxy.Forwarder.ServeHTTP`。
2. `server/internal/auth/transport/http/handler.go:Login`：解码 body（account/password）→ `service.Login`。
3. `server/internal/auth/service/service.go:Login`：`Users.GetByAccount`（`auth/userclient` 走 User RPC）→ bcrypt 校验 → `Tokens.Issue` 生成 JWT → 返回 token/user。
4. `server/internal/user/transport/grpc/*.go` → `user/service.Service.GetUserByAccount` → `user/repository/mysql` 查 `users`。
5. 响应经 `httpresponse` 包装；失败由 `apperror`/transport 映射为业务码。

### 发送好友申请：`POST /api/user/friends`

1. Gateway：`ServeHTTP` → `route.Parse("user")` → Auth 校验（非公开路径）→ 注入 `User-ID` → 代理到 `user_api`。
2. `server/internal/user/friend/transport/http/handler.go:SendRequest`：`trustedUserID` 取 `User-ID`，解码 `{friendID,message}`。
3. `server/internal/user/friend/friend.go:Service.SendRequest`：校验非自己 → `Repository.CreateRequest`。
4. `server/internal/user/friend/repository/mysql/repository.go:CreateRequest`：事务内检查双向好友/待处理申请 → INSERT `friend_verifies`（唯一键处理并发）。
5. 前端：`web/src/stores/friend.ts:sendRequest` → `web/src/api/friend.ts` → `web/src/views/ContactsView.vue` / `FriendSearch` 组件刷新 `sentRequestIDs`。

### Admin 用户列表：`GET /api/user/users`

1. Gateway：Auth 校验；Admin 路径不在公开 Allowlist，需要有效 JWT；`Role` 透传给 user-api。
2. `server/internal/user/admin/transport/http/handler.go:List`：`adminapi` 校验 `Role==1`，解析 query（page/limit/keyword/status）。
3. `server/internal/user/admin/service.go:Service.List` → `Repository.List`（`server/internal/user/admin/mysql` 或 repository）。
4. `server/internal/user/admin/repository/mysql/*.go`：分页查 `users` + 限制字段，返回 Page。
5. 前端：`admin/src/views/UserManageView.vue` + `admin/src/api/admin.ts` 消费。

## 2.3 错误码产生与透传

- 业务错误：service 返回 `apperror.Business(code,msg)`（`server/internal/platform/apperror`），transport 的 `writeError`/`writeFailure` 用 `httpresponse.Failure` 输出 `{code,message}`。
- 参数/身份错误：transport 直接 `apperror.HTTP(400/401,...)`；缺失 `User-ID` 返回未认证。
- Gateway 自身：路径非法 `CodeInvalidPath`、服务不支持 `CodeUnsupportedService`、Auth 超时/不可用 `CodeAuthUnavailable`、发现失败 `CodeServiceUnavailable`（`gateway/handler.go:authError/discoveryError`）。
- 代理失败：`gateway/proxy/proxy.go` 将上游超时/不可用映射为 502/504（具体分支见 `proxy.go`）。
- 上游业务错误原样透传；Gateway 不重写业务 code。

# 3. WebSocket 连接链路

## 3.1 Chat WS

| 环节 | 位置 | 说明 |
|---|---|---|
| Upgrade 入口 | `server/internal/chat/handler.go:Handler.ServeHTTP` | 校验 Origin（`chat.NewOriginPolicy`）、读取可信 `User-ID`/`Role`（Gateway 注入；测试可直连）、`websocket.Upgrader.Upgrade` |
| Auth | Gateway `handler.ServeHTTP` → Auth `/api/auth/authentication` | WS 握手也走 Gateway 代理；Chat 只信任 `User-ID` |
| Client 构造 | `server/internal/chat/client.go:newClient` | `newClientID`、read limit、heartbeat、send/control channel |
| Register | `server/internal/chat/hub.go:Hub.Register` → `registerClient` | 本地 0→1 时调用 `transitions.LocalUserOnline` |
| readPump | `server/internal/chat/client.go:Client.readPump` | 设置 read deadline/pong/ping handler；读帧 → `protocol.Decode` → `inbound.Handle`；失败分类后 `hub.Unregister` |
| writePump | `server/internal/chat/client.go:Client.writePump` | ticker 发 Ping；写 `send`/`control`；写超时/失败 → `hub.Unregister` |
| 慢客户端 | `server/internal/chat/hub.go:routeMessage` | 每 client `send` 非阻塞 select；满则 `Dropped++` 并 `removeClient(..., DisconnectSlow)` |
| 消息入站 | `server/internal/chat/message/inbound.go:Inbound.Handle` | 解析 `chat.send` → `service.SendPrivateMessage` |
| ACK | `inbound.go` → `hub.SendToClient` | 事务提交后发送 `chat.ack` |
| 推送 | `inbound.go` → `delivery.Publish` → `LocalDeliveryHandler` → `hub.SendToUser` | Redis Pub/Sub 到所有 Chat 实例，本实例按 userID 投递 |
| Presence | `server/internal/chat/presence/service.go` | 实现 `LocalConnectionTransitionObserver`，回调入队后异步写 Redis ZSET |
| 好友通知 | `server/internal/chat/friendnotify/notifier.go` | 同一 observer 接口；回调非阻塞入队，worker 查好友 → `Hub.SendToUser` 发 `friend.online/offline` |

**Chat 发送调用链（文本）**

1. `server/internal/chat/client.go:readPump` 读帧 → `protocol.Decode`。
2. `server/internal/chat/message/inbound.go:Inbound.Handle` → `service.SendPrivateMessage`。
3. `server/internal/chat/message/service.go:SendPrivateMessage`：校验 → 幂等查询 → 好友/接收者校验 → `sends.PersistWithSessions`。
4. `server/internal/chat/message/repository/mysql/repository.go:PersistWithSessions`：事务写 `chat_messages` + 双方 `chat_sessions`。
5. `inbound.go`：`hub.SendToClient(sender, ack)` → 发送方 writePump。
6. `delivery.Publish` → `server/internal/chat/message/inbound.go:LocalDeliveryHandler` → `hub.SendToUser(receiver)` → 接收方 writePump。

## 3.2 Group WS

| 环节 | 位置 | 说明 |
|---|---|---|
| Upgrade | `server/internal/group/ws/handler.go:Handler.ServeHTTP` | Origin/身份校验、Upgrade、`Hub.Register` |
| Auth | Gateway/Auth；Group handler 读取可信 `User-ID` | 与 Chat 相同链路 |
| Hub | `server/internal/group/ws/hub.go` | 单 goroutine 写 clients map；register/unregister；`SendToUser` 非阻塞 |
| readPump/writePump | `server/internal/group/ws/client.go` | 与 Chat 同构；Ping/Pong、deadline、慢客户端摘除 |
| 入站 | `server/internal/group/message/inbound.go:Inbound.Handle` | `group.send` → `Service.Send` |
| ACK/广播 | `inbound.go` → `Hub.SendToClient` / `delivery.Publish` → `LocalDeliveryHandler` → `Hub.SendToUser` | 群消息按成员 userID 扇出；不写 Presence |
| 好友通知 | 无 | Group WS 不参与好友事件/Presence |

**Group 发送调用链**

`group/ws/client.go:readPump` → `group/message/inbound.go:Inbound.Handle` → `group/message/service.go:Send` → `repository.PersistWithSessions`（`group_messages` + 每成员 `group_sessions`）→ ACK 给发送者 → `delivery.Publish` → 各实例 `LocalDeliveryHandler` → `Hub.SendToUser`。

## 3.3 连接关闭

- Chat `client.go:readPump`/`writePump` 退出时调用 `hub.Unregister`；`removeClient` 在本地 1→0 时回调 `LocalUserOffline`。
- Group `client.go:readPump`/`writePump` 退出时调用 `hub.requestUnregister`（只投递命令、不等待），由 Group Hub 事件循环 `dropClient` 删除连接；Group 无 Presence 回调。
- Chat `Hub.Shutdown` 会停止所有 client 并对每个 user 调 offline；随后 `friendNotifier.Shutdown` drain 已入队事件，`presenceService.Shutdown` 清 Redis contribution。
- 慢客户端被 `routeMessage` 摘除时同样走 `removeClient`，触发 1→0（若为最后一个连接）。

# 4. 按功能拆解

本章每个功能都按“入口 → service → repository → 前端”列出真实调用链。路径相对仓库根，函数名均已按源码核对；无法确认处标注“未找到”。

## 4.1 认证：注册、登录、注销、JWT 校验、logout 黑名单

**涉及文件**
- 入口：`server/internal/gateway/handler.go:ServeHTTP`；`server/internal/auth/transport/http/handler.go:RegisterRoutes`（`/api/auth/register|login|authentication|logout`）
- 协议/DTO：`auth/transport/http/handler.go` 的 `registerRequest`、`loginRequest`；`auth/service/service.go` 的 `PublicUser`、`LoginResult`、`AuthenticationResult`
- service：`auth/service/service.go:Register/Login/Authenticate/Logout`；`auth/service/allowlist.go:DefaultAllowlist/IsPublic`
- repository：`auth/token/jwt.go:Issue/Parse`；`auth/revocation/redis.go:Revoke/IsRevoked/Fingerprint`；`auth/credential/password.go:Hash/Compare`；`auth/userclient/client.go` → User RPC
- 前端：`web/src/api/auth.ts`、`web/src/stores/auth.ts`、`web/src/utils/session.ts`、`web/src/api/client.ts`
- 测试：`auth/service/service_test.go`、`auth/token/jwt_test.go`、`auth/revocation/redis_test.go`、`server/tests/auth_redis_integration_test.go`、`server/tests/gateway_auth_user_integration_test.go`

**调用顺序**
1. `gateway/handler.go:ServeHTTP` → `scrubIdentityHeaders` → `route.Parse` → `authclient.Client.Authenticate` → Auth `POST /api/auth/authentication`。
2. `auth/transport/http/handler.go:Authentication` → `service.Authenticate`：`allowlist.IsPublic` → `token.Manager.Parse` → `revocations.IsRevoked(Fingerprint(rawToken))`。
3. 登录：`handler.Login` → `Service.Login` → `users.GetByAccount`（User RPC）→ `passwords.Compare` → 状态校验（禁用 `1202`）→ `tokens.Issue`。
4. 注册：`Service.Register` → `passwords.Hash`（bcrypt）→ `users.Create`（只传 hash）。
5. 注销：`handler.Logout`（header `token`）→ `Service.Logout` → `Parse` 取剩余 TTL → `Revoke`（Redis `SET`）。
6. Gateway 认证成功后写 `User-ID`/`Role` 再代理。

**事务边界**
- 注册单行 INSERT；logout 单 Redis `SET`；JWT 校验无状态 + Redis 存在性判断；均无 DB 事务。

**并发/幂等/错误码**
- `1201` 凭证错误（不存在/密码错统一，防枚举）、`1202` 禁用、`1203` 缺 token、`1204` 无效、`1205` 过期、`1206` 已注销。
- 重复 logout 幂等（TTL 覆盖）；黑名单 key 为 token SHA-256，不存原文。

**已知边界**
- E-2（内部 API 无 mTLS，依赖只暴露 Gateway）；`/api/auth/open_login` 在 Allowlist 但未找到路由（第 9 章）。

**面试可讲的权衡**
- 中心 Auth 多一跳，换取撤权立即生效、业务服务零 JWT 依赖；登录失败统一码防账号枚举。

## 4.2 用户资料、Admin 用户列表/限制/禁用

**涉及文件**
- 入口：`server/internal/user/transport/http/profile.go:ServeHTTP`；`user/admin/transport/http/handler.go:List/UpdateRestrictions/Disable/Enable`
- 协议/DTO：`profileResponse`；`user/admin` 的 `Query/User/Restrictions/Page`
- service：`user/service/service.go:GetUserByID`；`user/admin/service.go:List/UpdateRestrictions/Disable/Enable`
- repository：`user/repository/mysql/repository.go:GetByID`；`user/admin/mysql/repository.go:List/UpdateRestrictions/UpdateStatus`
- 前端：`web/src/views/ProfileView.vue`；Admin：`admin/src/views/UserManageView.vue`、`admin/src/api/admin.ts`
- 测试：`user/transport/http/profile_test.go`、`user/admin/service_test.go`、`user/admin/transport/http/handler_test.go`、`server/tests/admin_integration_test.go`

**调用顺序**
1. 资料：`profile.ServeHTTP` → 读 `User-ID` → `users.GetUserByID` → `repository.GetByID` → `profileResponse`（不含 hash）。
2. Admin 列表：`handler.List` → `adminapi.Identity`（`Role==1`）→ `CursorLimit` → `admin.Service.List` → `admin/mysql.Repository.List`（`users LEFT JOIN user_confs` + 消息/群成员 COUNT 子查询）→ 逐行 `presence.IsOnline`。
3. 限制：`UpdateRestrictions` → upsert `user_confs`（事务内校验用户存在）。
4. 禁用/启用：`Service.Disable/Enable` → `UpdateStatus`；`actorID==target` 拒绝。

**事务边界**
- `UpdateRestrictions` 单事务；`UpdateStatus` 单 UPDATE，0 行时再 COUNT 区分“未变/不存在”。

**并发/幂等/错误码**
- `1110` 参数、`1111` 无权限、`1112` 不存在；禁用幂等；禁用后登录 `1202`。

**已知边界**
- K-9：`user_confs` 只有 Admin 写入，无用户侧读写 API/UI。

**面试可讲的权衡**
- 服务端二次校验 Role，不信任 Gateway；游标分页避免深翻页 COUNT。

## 4.3 好友：搜索、申请、同意/拒绝、列表、备注、删除、详情

**涉及文件**
- 入口：`server/internal/user/friend/transport/http/handler.go:RegisterRoutes`（`GET /api/user/search`、`GET/POST/PUT/DELETE /api/user/friends`、`GET /api/user/friend_info`、`GET /api/user/valid`、`PUT /api/user/valid_status`）
- 协议/DTO：`user/friend/friend.go` 的 `Friend/SearchResult/Request/FriendRow/VerificationRow`
- service：`friend/friend.go:Search/SendRequest/ListRequests/Respond/ListFriends/FriendInfo/UpdateRemark/DeleteFriend`
- repository：`friend/repository/mysql/repository.go:SearchUsers/ListFriends/GetFriend/FriendshipFlags/CreateRequest/ListRequests/Respond/UpdateRemark/DeleteFriend`
- 前端：`web/src/api/friend.ts`、`web/src/stores/friend.ts`、`web/src/components/friend/FriendList.vue`、`FriendRequestList.vue`、`web/src/views/ContactsView.vue`
- 测试：`friend/service_test.go`、`friend/transport/http/handler_test.go`、`server/tests/friend_integration_test.go`、`web/src/stores/friend.test.ts`

**调用顺序**
1. 搜索：`handler.Search` → `Service.Search` → `Repository.SearchUsers`（`status=1 AND id<>?` + account/nickname LIKE/ID）→ `FriendshipFlags`。
2. 申请：`Service.SendRequest` → `validatePair` → `directory.GetUserByID` → `Repository.CreateRequest`（事务：双向好友检查 → INSERT `friend_verifies`；1062 → `ErrDuplicateRequest`）。
3. 同意/拒绝：`Service.Respond` → `Repository.Respond`（`SELECT … FOR UPDATE` → 校验接收者/状态 → 接受时双向 `INSERT friends` → 更新申请）。
4. 列表：`Repository.ListFriends`（`friends JOIN users`）→ `Service.friendDTO` → `Service.online`（Presence 500ms 超时，失败降级 false）。
5. 备注/删除：单方向 UPDATE / 双向 DELETE。

**事务边界**
- `CreateRequest`、`Respond`、`DeleteFriend` 有事务；搜索/列表/备注无。

**并发/幂等/错误码**
- `1113` 自己、`1114` 已是好友、`1115` 不是好友、`1116` 待处理、`1117` 不存在、`1118` 已处理、`1119` 非接收方。
- 并发申请由 pending 唯一键仲裁；并发同意由行锁串行化。

**已知边界**
- K-4（只有接收方向申请列表）；K-3/K-15/K-16（在线状态推送边界）。

**面试可讲的权衡**
- 双向好友行换查询简单，代价是双写；`GetFriend` 把“不存在/非好友”统一 `1115` 防枚举。

## 4.4 私聊文本：发送、幂等、ACK、推送、历史

**涉及文件**
- 入口：WS `chat/handler.go:ServeHTTP` → `chat/client.go:readPump` → `chat/message/inbound.go:Inbound.Handle`；历史 `chat/message/transport/http/handler.go:History`
- 协议/DTO：`chat/protocol/envelope.go`（`Envelope/MessagePayload/ChatSendData/AckData/PrivateMessage`）
- service：`chat/message/service.go:SendPrivateMessage/History`
- repository：`chat/message/repository/mysql/repository.go:PersistWithSessions/GetBySenderAndClientMsgID/GetByID`；`session.go:upsertSession`；`history.go:ListHistory`；`friendship.go:AreFriends/CanChat`
- 前端：`web/src/stores/chat.ts`、`web/src/service/ws/chat_socket.ts`、`web/src/api/chat.ts`、`web/src/components/chat/*`
- 测试：`chat/message/service_test.go`、`chat/message/inbound_test.go`、`chat/hub_test.go`、`server/tests/chat_message_integration_test.go`

**调用顺序**
1. `readPump` → `protocol.Decode` → `Inbound.Handle` → `DecodeData[ChatSendData]` → `SendPrivateMessage`。
2. `validate`（sender≠receiver、clientMsgId 格式、文本 ≤4096B、payload ≤16KB）。
3. `GetBySenderAndClientMsgID`；命中且 `sameLogicalMessage` → 返回原消息；不同 → `2205`。
4. `friends.CanChat`（`user_confs.curtail_chat`）→ `recipients.GetByID`（User RPC）→ `friends.AreFriends`。
5. `sends.PersistWithSessions`：事务 INSERT `chat_messages`（1062 回查）→ 按 userID 升序 upsert 两条 `chat_sessions`（接收方 unread+1）。
6. `Inbound` 事务后发 `chat.ack`（`hub.SendToClient`）；仅 `CreatedNew` 时 `delivery.Publish` → `LocalDeliveryHandler` → `hub.SendToUser` 推送接收方。
7. 历史：`Service.History`（游标 `belongsToConversation`）→ `ListHistory`（conversation_low/high_id + id DESC）→ `recalledSet/markRecalled` → 升序返回。

**事务边界**
- 消息 + 双方 session 同一 MySQL 事务；ACK 在提交后；Redis 推送在 ACK 后，失败只记日志。

**并发/幂等/错误码**
- `2201` 参数、`2202` 接收者不存在、`2203` 非好友、`2204` 服务不可用、`2205` 幂等冲突、`2206` 被限制。
- `(send_user_id, client_msg_id)` 唯一索引最终兜底；1213 死锁有界重试 3 次（10/20ms）。

**已知边界**
- K-2（无接收方 delivery ACK）、K-5（pending 不跨刷新）、K-10（群聊无 1213 重试）。

**面试可讲的权衡**
- ACK 表示“服务端已持久化”；先 ACK 后广播，发送方感知不被推送通道阻塞。

## 4.5 私聊图片

**涉及文件**
- 入口：`file/transport/http/handler.go:UploadImage`（`POST /api/file/image`）→ WS type=2 → `Inbound.Handle`
- 协议/DTO：`protocol.ImageMessage{fileID,url,width,height,size,mimeType}`
- service：`chat/message/service.go:normalizeImage`；`file/service/service.go:Upload`
- repository：`chat/message/repository/mysql/repository.go:GetOwnedImage`；`file/repository/mysql/repository.go:FindAccessibleByID/canAccessImage`
- 前端：`web/src/stores/chat.ts:sendImage`、`web/src/api/file.ts`、`web/src/components/chat/MessageBubble.vue`、`ChatComposer.vue`、`web/src/utils/message.ts`
- 测试：`chat/message/service_test.go`、`server/tests/image_message_integration_test.go`、`server/tests/file_authorization_integration_test.go`

**调用顺序**
1. `sendImage` → `fileApi.uploadImage`（multipart `image`）→ `Service.Upload` → 得到 `user_files.id`。
2. WS `chat.send` type=2 → `validateImagePayload`（fileID>0、尺寸 0..20000）→ `normalizeImage`：`GetOwnedImage`（`user_files JOIN file_objects`，限本人）。
3. 非 `image/` MIME → `2212`；`size > MaxImageBytes` → `2213`；服务端覆写 `url=/api/file/{id}`、MIME、size 后落库/ACK/推送。

**事务边界**
- 上传：文件系统 + `Attach` 事务（见 4.9）；发送与上传分离。

**并发/幂等/错误码**
- `2211` 文件无效/无权、`2212` 非图片、`2213` 超限、`2214` payload 非法、`2503` 下载不可用；幂等键仍为 clientMsgId。

**已知边界**
- K-11（宽高仅客户端提示）；E-8（授权查询无索引）；E-9（浏览器缓存）。

**面试可讲的权衡**
- 客户端只传 fileID，URL/MIME/size 由服务端反查，杜绝伪造外链。

## 4.6 私聊普通文件

**涉及文件**
- 入口：`file/transport/http/handler.go:UploadFile`（`POST /api/file/file`）→ WS type=3
- 协议/DTO：`protocol.FileMessage{fileID,url,fileName,size,mimeType}`
- service：`chat/message/service.go:validateFilePayload/normalizeFile`
- repository：`chat/message/repository/mysql/repository.go:GetOwnedFile`；`file/repository/mysql/repository.go:canAccessImage`（同时匹配 `fileMsg.fileID`）
- 前端：`web/src/stores/chat.ts:sendFile`、`web/src/api/file.ts`、`web/src/components/file/*`、`web/src/views/FileView.vue`
- 测试：`chat/message/service_test.go`、`server/tests/file_message_integration_test.go`、`server/tests/file_text_authorization_integration_test.go`

**调用顺序**
1. `sendFile` → `fileApi.upload`（multipart `file`）→ `user_files.id`。
2. WS type=3 → `validateFilePayload`（fileID>0、size≥0）→ `normalizeFile`：`GetOwnedFile` 校验归属 → `size > MaxFileBytes` → `2216`。
3. 服务端回填 `url/fileName/size/mimeType` → 落库/ACK/推送。
4. 接收端点击文件 → `fileApi.download(id)` → `GET /api/file/:id` → `GetAccessible`。

**事务边界**
- 同图片；上传与发送分离。

**并发/幂等/错误码**
- `2215` 文件无效、`2216` 超限、`2217` payload 非法、`2503`；幂等同文本。

**已知边界**
- K-7（无孤儿 GC）；E-8/E-9。

**面试可讲的权衡**
- 文件名取 `user_files.original_name`，下载 `Content-Disposition: attachment` + `nosniff`，避免把用户文件当 HTML 执行。

## 4.7 私聊撤回

**涉及文件**
- 入口：WS `chat.send` type=4 → `Inbound.Handle` → `SendPrivateMessage` → `sendWithdraw`
- 协议/DTO：`protocol.WithdrawMessage{msgID}`
- service：`chat/message/service.go:sendWithdraw`；读侧 `recalledSet/markRecalled`
- repository：`chat/message/repository/mysql/repository.go:GetByIDForRecall/IsRecalled/GetWithdrawBySenderAndTarget/RecalledTargets`；授权联动 `file/repository/mysql/repository.go:canAccessImage` 的 `NOT EXISTS`
- 前端：`web/src/stores/chat.ts:recallMessage/finishRecall/markRecalledLocally`、`web/src/stores/chat_reconcile.ts:applyWithdrawals`、`web/src/utils/message.ts`
- 测试：`chat/message/service_test.go`（含并发）、`server/tests/recall_integration_test.go`

**调用顺序**
1. `sendWithdraw` → `GetWithdrawBySenderAndTarget`（已有 → 返回同一 ACK）。
2. `GetByIDForRecall`（含 tombstone）→ 不存在/非会话 → `2218`。
3. 非发送者 → `2219`；tombstoned 或 `IsRecalled` → `2221`；超过 `RecallWindow` → `2220`。
4. `PersistWithSessions`（type=4，preview `[撤回]`）→ ACK + 推送。
5. 历史/会话读侧 `RecalledTargets` 批量查询 → `markRecalled` 替换原消息。
6. 文件下载：被撤回原消息不满足授权 `NOT EXISTS` → 非所有者 `2503`。

**事务边界**
- 撤回消息 + session 同事务；“查窗口/查已撤回”和写入不在同一事务，无 target 唯一索引。

**并发/幂等/错误码**
- `2218/2219/2220/2221/2222`；顺序重复撤回幂等；并发不同 clientMsgId 可能两条（K-14）。

**已知边界**
- K-12（已实现）、K-13（无管理员撤回）、K-14（并发重复）。

**面试可讲的权衡**
- 不物理删除原消息，用“撤回行 + 读侧替换”保留审计与幂等；代价是历史/下载多一次 JSON 反查。

## 4.8 Session：列表、未读、已读、置顶、隐藏

**涉及文件**
- 入口：`chat/message/transport/http/handler.go:Sessions/Read/Top`（`GET /api/chat/session`、`PUT /api/chat/read`、`POST /api/chat/user_top`）
- service：`chat/message/service.go:ListSessions/MarkRead/SetSessionTop/ToggleSessionTop/HideSession`
- repository：`chat/message/repository/mysql/session.go:ListSessions/MarkRead/SetSessionTop/ToggleSessionTop/HideSession/upsertSession`
- 前端：`web/src/stores/chat.ts:refreshSessions/openConversation/markConversationRead/setSessionTop`、`ChatSessionList.vue`、`ChatView.vue`
- 测试：`server/tests/chat_session_integration_test.go`、`chat/message/transport/http/handler_test.go`

**调用顺序**
1. 列表：`ListSessions`（`hidden_at IS NULL`；JOIN users 取 peer；JOIN 最后一条消息；`ORDER BY is_top DESC,last_message_at DESC,id DESC`）→ `recalledSet` 修正占位。
2. 已读：`MarkRead` → `FOR UPDATE` 消息行校验会话 → 锁 session → `max(旧,请求)` → COUNT 未读 → 更新 last_read/unread。
3. 置顶：`SetSessionTop` / `ToggleSessionTop`（`IF(is_top=1,0,1)`）。
4. 隐藏：`HideSession` 写 `hidden_at`；新消息 upsert 清空 `hidden_at`。
5. 前端 `readInFlight/readRequestedThrough` 合并并发已读；`updateLocalSession` 本地 +1 未读。

**事务边界**
- `MarkRead` 单事务；置顶/隐藏单 UPDATE。

**并发/幂等/错误码**
- `2207/2208/2209/2210`；重复已读幂等（max）；行锁避免并发覆盖。

**已知边界**
- K-5；`DELETE /api/chat/chat` **未找到** HTTP 入口（`HideSession` 未接线）；`chat_message_deletions` **未找到写入方**，读侧 `NOT EXISTS` 目前无数据来源。

**面试可讲的权衡**
- session 表替代大消息表动态聚合，支持置顶/未读/隐藏；隐藏而非删除，保证对端不受影响。

## 4.9 文件上传与下载授权

**涉及文件**
- 入口：`file/transport/http/handler.go:upload/Download/List/Detail/Delete`
- service：`file/service/service.go:Upload/Get/GetAccessible/List/Delete/Open/SanitizeFilename/ContentDisposition`
- repository：`file/repository/mysql/repository.go:GetBySHA256/Attach/FindAccessibleByID/canAccessImage/DeleteReference`
- storage：`file/storage/local.go:NewLocal/CreateTemp/Commit/Abort/Open/Delete/Exists`
- 领域：`file/domain/file.go:FileObject/UserFile/FileMetadata`
- 前端：`web/src/api/file.ts`、`web/src/stores/file.ts`、`web/src/components/file/FileUploadPanel.vue`、`UserFileList.vue`、`web/src/views/FileView.vue`
- 测试：`file/service/service_test.go`、`file/storage/local_test.go`、`server/tests/file_service_integration_test.go`、`server/tests/file_gateway_integration_test.go`

**调用顺序**
1. `handler.upload`：身份校验 → Content-Length 预检 → multipart 解析 → `MaxBytesReader` → 逐 part 传给 `Service.Upload`。
2. `Service.Upload`：`SanitizeFilename` → `storage.CreateTemp` → `io.CopyBuffer(io.MultiWriter(temp,sha256,prefixWriter(512)), io.LimitReader(...))`。
3. 超限 `2502`；`hashLocks.lock(hash)` 进程内去重 → `GetBySHA256` → `storage.Exists` / `Commit`（`Sync`→`Close`→`os.Rename`→`Chmod`）。
4. `repository.Attach` 事务：`INSERT file_objects ... ON DUPLICATE KEY` → `SELECT ... FOR UPDATE` → `INSERT user_files ... ON DUPLICATE KEY` → 回读。
5. 失败补偿：本次创建且 DB 无引用时 `storage.Delete`；成功发审计。
6. 下载：`Download` → `GetAccessible` → `Open` → `Content-Type/Length/Content-Disposition(attachment)/nosniff` → `http.ServeContent`。

**事务边界**
- 文件系统写入不在 DB 事务；`Attach` 是对象+引用唯一原子事务；采用“先落盘再写库 + 补偿”，容忍孤儿对象。

**并发/幂等/错误码**
- `2501` 参数、`2502` 超限、`2503` 不存在/无权、`2504` 不可用。
- 并发同内容：进程内 hash lock + `ON DUPLICATE KEY` + `FOR UPDATE` 三层；同用户同内容同名复用 `user_files` 行。

**已知边界**
- K-7（无孤儿 GC）；E-8（JSON 授权无索引/无限速）；E-9（浏览器缓存）。

**面试可讲的权衡**
- 内容寻址解耦“对象/引用”，删除只删引用；代价是 GC 与跨表 JSON 授权查询。

## 4.10 好友上线提醒

**涉及文件**
- 入口：`cmd/chat-api/main.go:run`（`friendnotify.New/SetSender/Start/Shutdown`，`transitionObservers{presenceService, friendNotifier}`）；`chat/hub.go:registerClient/removeClient`
- 协议/DTO：`chat/protocol/envelope.go:EventFriendOnline/EventFriendOffline/FriendPresenceData`
- service：`chat/friendnotify/notifier.go:LocalUserOnline/LocalUserOffline/enqueue/run/handle/notificationEnvelope`；`chat/presence/service.go:setDesired/flush`；`chat/presence/redis.go:MarkOnline/MarkOffline/IsOnline`
- repository：`chat/message/repository/mysql/friendship.go:FriendIDs`
- 前端：`web/src/service/ws/chat_socket.ts:parseChatServerEnvelope` → `web/src/stores/chat.ts:handleEnvelope` → `web/src/stores/friend.ts:applyPresence` → `web/src/components/friend/FriendList.vue`
- 测试：`chat/friendnotify/notifier_test.go`、`chat/hub_test.go`、`server/tests/friend_presence_integration_test.go`、`chat/presence/service_test.go`、`server/tests/chat_presence_redis_integration_test.go`

**调用顺序**
1. `hub.registerClient`：仅 `len(userClients)==0` 调 `transitions.LocalUserOnline`；`removeClient` 在 `len==0` 调 offline（多端去重）。
2. `transitionObservers` 扇出：`presence.Service.setDesired`（内存 signal）与 `friendnotify.Notifier.enqueue`（非阻塞，满则丢弃计数）。
3. notifier worker `handle`：`GlobalPresenceCheck.ShouldNotify`（单机恒 true）→ `FriendLookup.FriendIDs`（`friends JOIN users status=1`，排除自己/禁用）→ `notificationEnvelope` → 每个好友 `Hub.SendToUser`。
4. Hub `routeMessage` → 每 client `send` → `writePump`。
5. presence `flush`：`ZADD gim:presence:user:{id} score=expiresAt member=instanceID` + `PEXPIRE`；offline Lua `ZREM` 空则 `DEL`；`IsOnline` 先清过期再 `ZCARD`。
6. 前端 `applyPresence` 只更新已存在好友行。

**事务边界**
- 无 DB 事务；Redis 单命令/Lua 原子；通知逐好友 best-effort。

**并发/幂等/错误码**
- Hub 单 goroutine 独占连接 map；notifier 有界队列（默认 256）+ worker（默认 1）+ `Shutdown` drain；presence 用 desired/dirty 合并突发变更。无专用错误码，失败仅日志。

**已知边界**
- K-15（单机）、K-16（offline best-effort）、V-20；K-3（HTTP isOnline 仍为权威）。

**面试可讲的权衡**
- 0→1/1→0 判定放在 Hub 事件循环（权威、天然多端去重），但 Hub 内绝不做 I/O，只做 O(1) 入队；接受 offline 丢事件换在线路径零阻塞。

## 4.11 群创建、成员、角色、邀请、申请、禁言

**涉及文件**
- 入口：`server/internal/group/transport/http/handler.go`（`Create/Members/Invite/RemoveMember/UpdateRole/Apply/Review/Mute`）
- 协议/DTO：同文件 `createRequest/inviteRequest/roleRequest/applyRequest/reviewRequest/muteRequest/groupResponse/memberResponse`
- service：`group/service/service.go`（`CreateGroup/GroupInfo/ListMembers/InviteMembers/ApplyToJoin/ReviewJoinRequest/LeaveGroup/KickMember/UpdateMemberRole/MuteMember/UnmuteMember/DissolveGroup`）；`group/service/policy.go`（`canManageJoin/canInvite/canActOn`）
- repository：`group/repository/mysql/repository.go`（`CreateGroup/AddMembers/JoinOpenGroup/CreateJoinRequest/ApproveJoinRequest/RejectJoinRequest/RemoveMember/UpdateRole`）
- 禁言：`group/mute/store.go:RedisStore.Mute`（Redis key `GroupProhibition(memberID)` + TTL）
- 前端：用户端群页面未逐页核对；Admin 群列表见 `admin/src/views/GroupsView.vue`
- 测试：`group/service/policy_test.go`、`group/transport/http/handler_test.go`、`server/tests/group_foundation_integration_test.go`

**调用顺序**
1. `Handler.Create` → `trustedUserID` → `decodeJSON` → `Service.CreateGroup`：校验名称/策略/人数 → `uniqueUserIDs` → `lookupUsers`（User RPC）→ `Repository.CreateGroup`。
2. `CreateGroup` 单事务：INSERT `groups` → INSERT 群主 `group_members(role=1)` + 受邀成员（`member_limit` 校验）。
3. `Handler.Members` → `Service.ListMembers` → `requireMember`（`GetGroupByID` + `GetMember`）→ `ListMembers` → `lookupUsers` 补资料。
4. `Invite` → `Service.InviteMembers` → `canInvite`（管理员，或成员且 `allow_member_invite`）→ 逐个 `IsMember` → `AddMembers` 事务：锁群行 → COUNT 成员 → 上限 → 批量插入。
5. `Apply` → `ApplyToJoin` → `GetGroupByID` → `IsMember` → 按 `JoinPolicy`：Closed `2312`；Open `JoinOpenGroup` 事务；Approval/Question/CorrectAnswer `CreateJoinRequest(status=0)`。
6. `Review` → `ReviewJoinRequest` → `requireMember` → `canManageJoin` → `ApproveJoinRequest/RejectJoinRequest` 事务：锁 `group_verifies` → 校验 pending → 锁群/人数 → 插入成员 → 更新状态。
7. `RemoveMember`：`memberId` 空/自己 → `LeaveGroup`（群主 `2310`）；否则 `KickMember` → `canActOn` → `RemoveMember`。
8. `UpdateRole`：仅群主可改，目标不能是群主，只允许 2/3。
9. `Mute`：`prohibitionTime=nil` → `UnmuteMember`；否则 `MuteMember` → `membersForModeration` → `canActOn` → Redis `Mute`（≤30 天）。
10. 发送侧禁言：`group/message/service.go:Send` → `policy.IsMuted` → `requireMember` → Redis `Exists`。

**事务边界**
- 显式事务：`CreateGroup/AddMembers/JoinOpenGroup/ApproveJoinRequest/RejectJoinRequest/DissolveGroup`。
- `RemoveMember/UpdateRole` 单条 SQL；禁言 Redis 写与 MySQL 无跨存储事务。

**并发/幂等/错误码**
- 成员上限“锁群行 + 重新 COUNT”；`ux_group_members_group_user` 兜底 1062 → `2303`。
- `ux_group_verifies_pending` 兜底重复申请 → `2306`。
- `canActOn`：管理员只能动成员，任何人不能动群主。
- `2301` 参数、`2302` 群不存在、`2303` 已是成员、`2304` 非成员、`2305` 无权限、`2306` 重复申请、`2307` 申请不存在、`2308` 状态无效、`2309` 目标用户不存在、`2310` 群主不能退群、`2311` 禁言存储不可用、`2312` 不允许申请、`2313` 人数上限。

**已知边界**
- `groups.is_all_muted` 只在发送时读取，**未找到**修改它的 HTTP/Service 入口。
- 邀请直接入群，无“待接受邀请”；`group_verifies.request_type=2` 未使用。
- Question/CorrectAnswer 与审批同路径，`verification_question/answer` 未参与校验。
- 退群/踢人不清理 Redis 禁言 key（按 memberID，残留至 TTL）。
- 发送事务不锁成员行，踢人与在途发送存在窗口。

**面试可讲的权衡**
- 锁群行控人数：单库正确、成本低，但群行成为热点；大群需分片/限流。
- 禁言用 Redis TTL：天然过期、读快；Redis 丢失即禁言丢失，且无审计历史。
- 群解散置 `status=2` 并删成员、不删消息：保留审计，历史数据膨胀。

## 4.12 群文本/图片/文件消息：发送、ACK、广播、session 扇出

**涉及文件**
- 入口：`group/ws/handler.go:ServeHTTP` → `group/ws/client.go:readPump` → `group/message/inbound.go:Inbound.Handle`
- 协议/DTO：`group/ws/protocol/envelope.go`（`Envelope/GroupSendData/MessagePayload/AckData/GroupMessage`）
- service：`group/message/service.go:Send/normalizeImage/normalizeFile/previewOf/sameLogicalMessage`
- repository：`group/message/repository/mysql/repository.go:PersistWithSessions/upsertSessions/GetOwnedImage/GetOwnedFile`；`session.go`；`history.go`
- 投递：`group/delivery/redis.go:Bus.Publish/Subscribe`；`group/message/inbound.go:LocalDeliveryHandler`
- 前端：`web/src/stores/group.ts`、`web/src/service/ws/group_socket.ts`、`web/src/components/group/*`、`web/src/views/GroupView.vue`
- 测试：`group/message/service_test.go`、`inbound_test.go`、`group/ws/hub_test.go`、`handler_integration_test.go`、`server/tests/group_message_integration_test.go`、`group_gateway_websocket_integration_test.go`

**调用顺序**
1. `Handler.ServeHTTP` 校验 `User-ID`/`Role` → Upgrade → `newClient` → `Hub.Register` → writePump/readPump。
2. `readPump` 读帧 → `protocol.Decode` → `Inbound.Handle`（只收 `group.send`）→ `Service.Send`。
3. `validate`（groupID/clientMsgId/文本 ≤4096B/payload ≤16KB/图片/文件 DTO）→ `GetBySenderAndClientMsgID` 幂等预查。
4. `GetGroupByID` → `IsMember` → 全员禁言 `memberRole` → `IsMuted`；图片 `GetOwnedImage`、文件 `GetOwnedFile`，服务端覆写 URL/MIME/size/name。
5. `PersistWithSessions` 事务：`SELECT ... FOR SHARE` 锁 active 群 → 查 `group_members.id` → INSERT `group_messages` → `Pluck` 全群成员 → `upsertSessions`。
6. `upsertSessions`：每成员 `INSERT ... ON DUPLICATE KEY UPDATE`（last_message_id=GREATEST、unread+1、hidden_at=NULL），返回 `TargetUserIDs`。
7. 1062 → `errDuplicateRace` → 结束事务后重查赢家，`CreatedNew=false`。
8. `Inbound` 先发 `group.ack`（`Hub.SendToClient`）；仅 `CreatedNew` 时 `delivery.Publish`。
9. 各实例订阅 → `LocalDeliveryHandler` → 对 `TargetUserIDs` 逐个 `Hub.SendToUser` → `client.send` → `writePump`。

**事务边界**
- 消息 + 全群 session 同一事务；ACK 在提交后；Redis 广播在提交后，失败只记 `group realtime delivery degraded`。

**并发/幂等/错误码**
- 幂等唯一键 `(send_user_id, client_msg_id)`；`sameLogicalMessage` 不一致 → `2407`。
- 重复竞态“事务失败后新事务读胜者”，避免 RR 旧快照。
- 群共享锁允许并发发送，阻止解散/改群跨过持久化边界。
- `2403` 参数、`2404` 非成员、`2405` 禁言、`2406` 不可用、`2407` 幂等冲突；图片 `2412–2415`；文件 `2416–2418`。

**已知边界**
- Redis Pub/Sub 是 at-most-once，订阅者掉线/失败丢实时消息，靠历史补偿。
- 每成员一行 session，群越大写放大越明显。
- 成员快照取事务内，发送后新入群成员不会收到该消息。
- 多设备每连接一份，未做设备去重。
- Group 无 1213 重试（第 6 章）。

**面试可讲的权衡**
- 事务后 ACK + 异步广播：不丢已确认消息，实时性受 Redis 影响。
- 唯一键最终幂等 + 预查快速幂等：正确性与性能分离。
- session 扇出换查询性能：读简单，写 O(成员数)，大群需异步/批量。

## 4.13 群撤回

**涉及文件**
- 入口：`group/ws/inbound.go` → `group/message/inbound.go:Handle`（type=4）
- service：`group/message/service.go:Send/sendWithdraw/markRecalled/recalledSet`
- repository：`group/message/repository/mysql/repository.go:GetByIDForRecall/IsRecalled/GetWithdrawBySenderAndTarget/RecalledTargets`
- 读路径：`service.go:History/ListSessions`、`repository/mysql/history.go`、`session.go`
- 前端：`web/src/stores/group.ts:recallMessage`、`web/src/stores/group_reconcile.ts:applyGroupWithdrawals`
- 测试：`group/message/service_test.go`、`server/tests/recall_integration_test.go`（私聊为主，群撤回以单测覆盖）

**调用顺序**
1. `group.send` type=4 → `validate`（`WithdrawMsg.MsgID>0`，否则 `2423`）。
2. 幂等预查 `GetBySenderAndClientMsgID`；`sendWithdraw` 先 `GetWithdrawBySenderAndTarget`。
3. `GetByIDForRecall`（含 tombstone）→ 不存在/跨群 `2419` → 非本人 `2420` → tombstone/已撤回 `2422` → 窗口 `2421` → `IsMember`（退群 `2404`）。
4. `PersistWithSessions` 写 type=4 + 全群 session。
5. 历史/会话 `RecalledTargets` 批量查询 → `markRecalled` 替换原消息。
6. 文件授权 `file/repository/mysql/repository.go:canAccessImage` 的 `NOT EXISTS` 使非成员/非所有者下载 `2503`。

**事务边界**
- 校验全部在事务外；撤回消息 + 全群 session 同一事务；原消息行不修改，读时投影。

**并发/幂等/错误码**
- 同 clientMsgId / 同 sender+target 串行幂等；并发不同 clientMsgId 仍可能两条（K-14）。
- `2419/2420/2421/2422/2423`；群成员最后校验。
- 管理员删除撤回记录后 `IsRecalled` 不再命中，同一目标可再次撤回（语义缝隙）。

**已知边界**
- K-14；K-13（无管理员撤回）；撤回不删除文件/引用。
- 撤回消息占 `last_message_id`，会话预览变 `[撤回]`。

**面试可讲的权衡**
- 读时投影避免批量改写历史；代价是每次历史/会话多一次 JSON 反查（E-8）。
- 撤回作为一等消息：进入序列、可多端同步；代价是占 unread/last_message。

## 4.14 Admin：私聊审计、群管理、文件管理、设置、日志查询

### 4.14.1 Admin 私聊审计

**涉及文件**
- 入口：`chat/message/admin/transport/http/handler.go`（`GET/DELETE /api/chat/admin/history`）
- service/repo：`chat/message/admin/service.go:List/Delete` → `chat/message/admin/mysql/repository.go:List/Delete/deleteOnce/repairSession`
- 鉴权：`platform/adminapi/adminapi.go:Identity`（严格 `Role=="1"`）
- 前端：`admin/src/views/ChatsView.vue`、`admin/src/api/admin.ts`

**调用顺序**
`List`（游标 + 用户过滤）→ `Delete` 事务：`FOR UPDATE` 锁消息 → `admin_deleted_at=UTC` → 按 userID 升序 `repairSession` 两侧 session（重算 last_message/unread 或隐藏）。

**事务边界**
- 列表只读；删除（软删 + 两侧 session 修复）单事务；1213 重试 3 次；审计在提交后 best-effort。

**并发/幂等/错误码**
- 错误码 `2230/2231/2232`（Chat Admin 独立段；撤回使用 `2218–2222`）；重复删除第二次 404；行锁顺序与消息持久化一致。

**已知边界**
- 读取私聊不产生审计事件；删除不通知在线用户；大会话修复 O(消息数)。

**面试可讲的权衡**
- Tombstone 保留审计/恢复能力，代价是所有查询带 `admin_deleted_at IS NULL`；同步修 session 强一致但增加删除延迟。

### 4.14.2 Admin 群管理

**涉及文件**
- 入口：`group/admin/transport/http/handler.go`（群列表/解散/群消息列表/删消息）
- service/repo：`group/admin/service.go` → `group/admin/mysql/repository.go`
- 前端：`admin/src/views/GroupsView.vue`、`admin/src/api/admin.ts`

**调用顺序**
`List`（群 + owner + member/message 计数 + Presence 在线数）→ `Dissolve` 事务（status=2 + 删 members + 隐藏 sessions）→ `DeleteMessage` 事务（tombstone + repairGroupSession）。

**事务边界**
- 解散/删消息各单事务；审计在提交后。

**并发/幂等/错误码**
- 错误码 `2320/2321/2322`；重复解散/删除 404；`repairGroupSession` 对每 session 加 `FOR UPDATE`。

**已知边界**
- 解散不删消息/文件、不通知在线成员；群 WS 无“解散”事件；MemberOnlineCount 逐用户查 Presence。

**面试可讲的权衡**
- 状态位解散保留审计但产生孤儿数据；在线人数实时算无额外存储但 N 次 Redis 查询。

### 4.14.3 Admin 文件管理

**涉及文件**
- 入口：`file/admin/transport/http/handler.go`（列表/详情/删除/下载）
- service/repo：`file/admin/service.go` → `file/admin/mysql/repository.go`
- 前端：`admin/src/views/FilesView.vue`

**调用顺序**
`List`（user_files JOIN file_objects JOIN users）→ `Delete` 单条 DELETE（只删引用，不删 file_objects）→ `Download` 流式 `ServeContent`。

**事务边界**
- 删除单语句；下载无事务。

**并发/幂等/错误码**
- 错误码 `2520/2521/2522`；`RowsAffected` 判存在，重复删除 404；共享对象删一个引用不影响其他用户。

**已知边界**
- 无物理 GC；删引用后历史消息 URL 404；管理员下载无二次审批/水印；下载不审计。

**面试可讲的权衡**
- 引用计数式删除安全、支持共享对象；代价是存储膨胀与离线 GC。

### 4.14.4 Admin 设置

**涉及文件**
- 入口：`settings/transport/http/handler.go`（`/api/settings/info` 公开、`/api/settings/admin/info` PUT、`/api/settings/admin/overview`）
- service/repo：`settings/service/service.go:Get/Update` → `settings/repository/mysql`；overview 独立
- 前端：`admin/src/views/SettingsView.vue`、`DashboardView.vue`

**调用顺序**
`Update` 校验名称/版本/公告 → 固定 key JSON → `UpsertPublic` → 审计 `admin.settings.updated`。

**事务边界**
- 5 个 key 批量 upsert；service 无显式事务（repository 是否单事务未逐行核对）。

**并发/幂等/错误码**
- 错误码 `2701/2703`；Upsert 幂等；并发后写覆盖，无版本号。

**已知边界**
- 只允许公开业务设置；无变更历史；无配置版本/回滚。

**面试可讲的权衡**
- 固定 key + JSON 类型安全、可控；扩展需改代码，换取简单。

### 4.14.5 Admin 日志查询

**涉及文件**
- 入口：`logs/transport/http/handler.go`（`GET /api/logs/logs`、`:id`）
- service/repo：`logs/service/service.go:List/Get` → `logs/repository/mysql/repository.go`
- 前端：`admin/src/views/LogsView.vue`

**调用顺序**
`isAdmin`（`Role=="1"`）→ 参数校验 → 按 `occurred_at DESC,id DESC` 复合游标查询。

**事务边界**
- 只读，无事务。

**并发/幂等/错误码**
- 错误码 `2601/2603/2604`；查询本身不产生审计事件。

**已知边界**
- 查询不留痕；无导出/脱敏二次处理。

**面试可讲的权衡**
- 复合游标避免深翻页；代价是查询条件/索引耦合。

## 4.15 审计日志：LogEvent、脱敏、Kafka producer、consumer 幂等


**涉及文件**
- 事件：`platform/audit/event.go`（`Event/NewEvent/Validate/PartitionKey`）
- 脱敏：`platform/audit/metadata.go`（`ValidateMetadata/safeMetadataKey`）
- 请求关联：`platform/audit/request.go`（`HTTPContext/EnrichHTTP/RequestIDHeader`）
- 发射：`platform/audit/emit.go:EmitBestEffort`
- 生产：`platform/kafkaaudit/producer.go`、`config.go`
- 消费/落库：`logs/consumer/consumer.go`、`logs/repository/mysql/repository.go:Insert/List/Get`
- 查询：`logs/service/service.go`、`logs/transport/http/handler.go`
- 前端：`admin/src/views/LogsView.vue`
- 测试：`platform/audit/event_test.go`、`platform/kafkaaudit/producer_test.go`、`logs/consumer/consumer_test.go`、`server/tests/audit_kafka_integration_test.go`、`audit_repository_integration_test.go`

**调用顺序**
1. Gateway `ServeHTTP` 生成 `X-Request-ID` → 业务 handler 成功后 `audit.NewEvent` + `EnrichHTTP` → `EmitBestEffort`。
2. `Producer.Publish`：`event.Validate` → 非阻塞入队（满则 `ErrQueueFull`/`dropped++`）→ 单 goroutine `write`（JSON、`RequiredAcks=all`、`BatchSize=1`、`Async=false`、重试 `MaxAttempts`）。
3. `logs/consumer.Run`：`FetchMessage` → `Unmarshal` + `Validate` → `repository.Insert`（`audit_logs` 唯一 `event_id`，1062 → `(false,nil)`）→ `CommitMessages`。
4. Admin 查询走 `logs` HTTP。

**事务边界**
- 审计与业务非同一事务；Kafka 不是 outbox；消费“先落库后 commit”，至少一次 + 唯一键幂等。

**并发/幂等/错误码**
- 事件 UUID；唯一键 `event_id` 幂等；队列满/发送失败丢弃并计数；metadata allowlist 18 个 key，拒绝 password/token/authorization/cookie/secret 等。
- **未找到“每请求自动 LogEvent”中间件**：只有显式 emit 的业务动作产生事件。
- `Event.IP` 未在 `EnrichHTTP` 填充，当前 HTTP 审计 IP 为空。

**已知边界**
- 无 outbox/DLQ；Kafka 不可用或队列满即丢。
- 消费者 `FirstOffset`，新消费组重放靠幂等。
- 消费失败无限重试会阻塞分区；无失败隔离。
- 查询本身不审计。

**面试可讲的权衡**
- 审计异步化不阻塞 IM；代价是可能丢事件，需要监控 `Dropped`。
- allowlist metadata 比通用脱敏更安全，但新增字段要改代码。
- 至少一次 + 唯一键简单可恢复；代价是重复消费与 offset 阻塞。

## 4.16 Migration：版本化、runner、dirty 保护

**涉及文件**
- runner：`server/migrations/runner.go`（`New/Status/Up/Down/DownAll/applyUp/applyDown/ensureTable/load/parseFilename`）
- SQL：`server/migrations/001_identity` … `008_admin_management`
- CLI：`server/cmd/migrate/main.go:run`
- 测试：`server/migrations/runner_test.go`、`server/tests/database_migration_integration_test.go`

**调用顺序**
1. `cmd/migrate`：flag/env → `mysqlclient.FromValues/OpenSQL`（2min）→ `migrations.New` → `status/up/down/down-all`。
2. `load`：读取 `*.up.sql/*.down.sql`，要求 3 位版本、连续、up/down 成对、名称一致。
3. `applyUp`：先 INSERT `schema_migrations(version,name,dirty=1)` → 执行 Up SQL → 更新 `dirty=0,applied_at`。
4. `applyDown`：执行 Down SQL → 删除版本行；`DownAll` 统计后全部 down。
5. 已应用且 `dirty=1` 时 `Up` 直接报错，要求人工审查。

**事务边界**
- DDL 隐式提交，`applyUp/applyDown` 不在事务中；dirty 依赖“先写标记再执行”。
- 无 advisory lock；并发 `up` 可能冲突。

**并发/幂等/错误码**
- `schema_migrations.version` 主键防重复；未知版本/名称不匹配报错；`Down` 不检查 dirty。
- CLI 失败 `exit 1`，无业务错误码；`parseFilename` 只接受 3 位版本。

**已知边界**
- 无自动 dirty 修复；依赖人工。
- 只覆盖 MySQL；多语句 SQL 依赖 `multiStatements`。
- runner 自建 `schema_migrations` 不在 docs/04 索引清单（第 5 章差异）。

# 5. 数据模型与索引

来源：`server/migrations/001_identity` … `008_admin_management` 与 `server/migrations/runner.go`。字段为“主要字段”摘要；完整 DDL 以 migration 为准。

| 表 | 主要字段 | 索引 / 唯一 / CHECK | 读写 repository | 事务边界 |
|---|---|---|---|---|
| `users` | id, account, pwd_hash, nickname, avatar, abstract, role, status, open_id, 时间戳 | UNIQUE `ux_users_account`、`ux_users_open_id`；CHECK role∈(1,2)、status∈(1,2) | 读写 `user/repository/mysql/repository.go`；读 `user/friend/repository/mysql`、`user/admin/mysql`、`settings/overview/mysql` | 单语句；注册只写 `users` |
| `user_confs` | user_id, recall_message, friend_online, secure_link, curtail_chat, add_user, create_group, in_group_chat 等 | UNIQUE `ux_user_confs_user`；10 个 CHECK | 读 `chat/message/repository/mysql/friendship.go:CanChat`；读写 `user/admin/mysql/repository.go` | Admin 限制 upsert 单事务；注册不初始化 |
| `friends` | user_id, friend_id, remark, 时间戳 | UNIQUE `ux_friends_user_friend`；`ix_friends_friend_user`；CHECK 双方不同 | 读写 `user/friend/repository/mysql/repository.go`；读 `chat/message/.../friendship.go` | 申请/审批/删除单事务；双向两行 |
| `friend_verifies` | requester_id, receiver_id, status, message, pending_* 生成列 | UNIQUE `ux_friend_verifies_pending_pair`；2 查询索引；2 CHECK | 读写 `user/friend/repository/mysql/repository.go` | 发起/审批单事务 |
| `chat_messages` | send_user_id, rev_user_id, client_msg_id, msg_type, msg_preview, msg(JSON), admin_deleted_at, conversation_* 生成列 | UNIQUE `ux_chat_messages_sender_client`；4 索引；3 CHECK | 读写 `chat/message/repository/mysql/repository.go`；读 file 授权/Admin 计数/overview；读写 tombstone `chat/message/admin/mysql` | 消息+双方 session 单事务；1213 重试 |
| `chat_sessions` | user_id, peer_user_id, last_message_id/at, last_read_message_id, unread_count, is_top, hidden_at | UNIQUE `ux_chat_sessions_user_peer`；2 排序索引；2 CHECK | 读写 `chat/message/repository/mysql/session.go`；Admin 修复 | 与消息同事务；MarkRead 行锁 |
| `chat_message_deletions` | user_id, message_id, created_at | PK(user_id,message_id)；`ix_..._message` | **只读**（history/session/admin repair 的 `NOT EXISTS`）；**未找到生产写入方** | 无写入事务 |
| `groups` | name, owner_id, avatar, status, is_search, join_policy, allow_member_invite, is_all_muted, member_limit | `ix_groups_owner_status`；7 CHECK；无 UNIQUE | 读写 `group/repository/mysql/repository.go`；读 group message/admin/overview | 建群/解散单事务；发送共享锁 |
| `group_members` | group_id, user_id, member_nickname, role, joined_at | UNIQUE `ux_group_members_group_user`；2 索引；1 CHECK | 读写 `group/repository/mysql`；读 group message/admin/user admin | 加成员/审批单事务；锁群行 |
| `group_verifies` | group_id, user_id, reviewer_id, status, request_type, message, answer | UNIQUE `ux_group_verifies_pending`；2 索引；2 CHECK | 读写 `group/repository/mysql` | 审批单事务；`FOR UPDATE` |
| `group_messages` | group_id, send_user_id, group_member_id, client_msg_id, msg_type, msg(JSON), admin_deleted_at | UNIQUE `ux_group_messages_sender_client`；3 索引；2 CHECK | 读写 `group/message/repository/mysql`；读 file 授权/overview；tombstone group admin | 消息+全群 session 单事务；无 1213 重试 |
| `group_sessions` | user_id, group_id, last_message_id/at, last_read_message_id, unread_count, is_top, hidden_at | UNIQUE `ux_group_sessions_user_group`；2 索引；1 CHECK | 读写 `group/message/repository/mysql/session.go`；group admin | 与群消息同事务批量 upsert |
| `group_message_deletions` | user_id, message_id, group_id | PK(user_id,message_id)；2 索引 | **只读**（`NOT EXISTS`）；**未找到生产写入方** | 无写入事务 |
| `file_objects` | uid, sha256, size, storage_path, mime_type | UNIQUE `ux_file_objects_uid`、`ux_file_objects_sha256`；1 CHECK | 读写 `file/repository/mysql`；读 chat/group message、file admin、overview | `Attach` 单事务（对象+引用） |
| `user_files` | user_id, file_object_id, original_name | UNIQUE `ux_user_files_user_object_name`；2 索引 | 读写 `file/repository/mysql`；file admin 删除；chat/group 查找；overview 计数 | 上传附件单事务；删除引用单语句 |
| `settings` | setting_key, setting_value(JSON), is_public, description | UNIQUE `ux_settings_key`；`ix_settings_public_key`；2 CHECK | 读写 `settings/repository/mysql` | Upsert 批量；未逐条核对是否单事务 |
| `audit_logs` | event_id, event_type, service, action, user_id, role, resource_*, request_id, status, occurred_at, metadata(JSON) | UNIQUE `ux_audit_logs_event_id`；5 cursor 索引；4 CHECK | 读写 `logs/repository/mysql` | Insert 自动提交；event_id 1062 幂等 |
| `schema_migrations` | version, name, dirty, applied_at | PK(version)；CHECK dirty∈(0,1) | `server/migrations/runner.go` | 先 dirty=1 → DDL → 清 0 |

## 5.1 与 docs/04 的差异（只记录，不改旧文档）

> 以下 D1–D9 已于第十六轮按 migration 修正 docs/04（核对日期 2026-11-14）；本表保留差异发现记录。

| # | docs/04 表述 | 实际 schema/代码 |
|---|---|---|
| D1 | `user_confs` 列名 `curtail_add_user/curtail_create_group/curtail_in_group_chat` | 001 实际为 `add_user/create_group/in_group_chat` |
| D2 | `friend_verifies.message` “最多 64 字符” | 002 为 `VARCHAR(128)` |
| D3 | `file_objects.path`、`user_files.file_name` | 005 实际 `storage_path`、`original_name` |
| D4 | `friends` 草案 `send_user_id/rev_user_id/...` | 002 实际 `user_id/friend_id/remark` 双行；§7.4 已自我取代 |
| D5 | §7.8 “用户与配置原子创建” | 注册只写 `users`，**未找到** `user_confs` 初始化；`CanChat` 用 `COALESCE(MAX(...),0)` 容忍缺行 |
| D6 | `chat_message_deletions` 表达“用户隐藏单条历史” | 生产代码无 INSERT，仅读；隐藏走 `chat_sessions.hidden_at` |
| D7 | §7.11 索引清单 | 与 001–008 一致；清单未含 runner 自建 `schema_migrations` |
| D8 | §3 `chat_sessions` 未列 `hidden_at` | 003 与 §7.5 有 `hidden_at`，以 §7.5 为准 |
| D9 | `server/migrations/README.md` “released sequence ends at 007” | 实际已有 008（README 过期） |

## 5.2 事务与一致性要点

- 消息/会话：Chat 消息 + 双方 `chat_sessions` 同一事务；Group 消息 + 全群 `group_sessions` 同一事务。
- 好友：发起申请、审批（双向好友 + 状态）、删除均单事务；并发由唯一键/行锁仲裁。
- 文件：文件系统写入不在 DB 事务；`file_objects` + `user_files` 的 `Attach` 是唯一原子事务，失败补偿物理对象。
- 撤回：撤回消息 + session 同事务；原消息不改写，历史/会话/下载读时投影。
- 删除表：`chat_message_deletions`/`group_message_deletions` 生产代码只读，属“预留/未接线”，与 docs/04 的删除语义存在差异（D6）。

# 6. 并发模型

## 6.1 Hub/Client 单写模型

- Chat `clients map[uint64]map[string]*Client` 在 `server/internal/chat/hub.go:run` 内创建，只有 Hub 事件循环可读写；外部通过 `Hub.Register/Unregister/SendToClient/SendToUser` 投递命令（`hub.go:submit`）。
- Group `clients` 在 `server/internal/group/ws/hub.go:eventLoop` 内创建，只有事件循环调用 `addClient/dropClient/deliver`。
- 每个 Client 的 `send` 是 `make(chan []byte, SendBuffer)`（默认 64）；`Hub.routeMessage` / Group `deliver` 用 `select { case client.send <- payload: ... default: Dropped++; removeClient(..., DisconnectSlow) }`，**业务侧永不阻塞在慢连接**。
- `Client.stop` 用 `sync.Once` 只关闭一次 `send`/连接；`control` 队列 cap 4 只用于 Ping/Pong 控制帧。

## 6.2 锁顺序与死锁重试

- Chat `server/internal/chat/message/repository/mysql/repository.go:persistWithSessionsOnce`：构造发送方/接收方两条 session update 后按 `UserID` 升序排序，再在同一事务内 upsert `chat_sessions`。
- Group `server/internal/group/message/repository/mysql/repository.go:upsertSessions`：`sort.Slice` 按 userID 升序，单条多值 `INSERT ... ON DUPLICATE KEY UPDATE`。
- Chat `Repository.PersistWithSessions`：`maxDeadlockAttempts=3`，仅 MySQL `1213`（`isDeadlock`）重试，退避 `attempt*10ms`，ctx 取消立即返回。
- **Group 消息仓库未找到 1213 重试**：只处理 1062（`errDuplicateRace`），无 `isDeadlock`/重试；这是当前实现差异（见第 9 章）。
- `MarkRead`（Chat/Group `repository/mysql/session.go`）先 `SELECT ... FOR UPDATE` 消息行，再锁 session 行；unread 用 COUNT 重算。

## 6.3 幂等

| 项 | Chat | Group |
|---|---|---|
| 表 | `chat_messages` | `group_messages` |
| 唯一索引 | `ux_chat_messages_sender_client (send_user_id, client_msg_id)`（`003_chat.up.sql`） | `ux_group_messages_sender_client (send_user_id, client_msg_id)`（`004_group.up.sql`） |
| 预查询 | `Repository.GetBySenderAndClientMsgID` | 同名方法 |
| 并发兜底 | 事务内 INSERT 1062 → 同事务查询已有行，`CreatedNew=false` | INSERT 1062 → `errDuplicateRace`，结束事务后重查赢家 |
| 逻辑校验 | `Service.SendPrivateMessage` + `sameLogicalMessage`，不一致 `2205` | `Service.Send` 同构，不一致 `2407` |
| 撤回幂等 | `sendWithdraw` 先查 sender+target；同 clientMsgId 走通用幂等 | `sendWithdraw` 同构 |

## 6.4 Presence

- 唯一写入方：`server/internal/chat/presence/service.go`，由 `cmd/chat-api/main.go` 装配；Group WS 无 transition、不写。
- 配置：`CHAT_PRESENCE_TTL` 默认 90s、`REFRESH_INTERVAL` 30s、`RETRY_INTERVAL` 5s、`OPERATION_TIMEOUT` 1s（`chat/presence/config.go`）。
- `Service.run` 周期 `markActiveDirty` + `flush`；`RedisStore.MarkOnline` 用 ZADD（member=instanceID，score=expiresAt）+ PEXPIRE；`MarkOffline` Lua ZREM 空则 DEL；`IsOnline` 先清过期再 ZCARD。
- Hub 回调只更新内存 `desired` 并 signal（`wake` cap 1），Redis I/O 在 worker。

## 6.5 好友通知

- `server/internal/chat/friendnotify/notifier.go:enqueue` 在 `RWMutex.RLock` 下非阻塞入队；满则 `dropped` 原子计数并日志。
- worker 数 1..8、队列 256（上限 65536）、单次 I/O 2s（上限 30s）；`Shutdown` 置 closed → close(events) → wait drain。
- worker 通过 `Hub.SendToUser` 投递，仍受 Hub `send` 有界队列约束（慢客户端会被摘除）。

## 6.6 审计 producer

- `server/internal/platform/kafkaaudit/producer.go`：`Publish` 非阻塞 `select queue <- event / default`，满则 `dropped++` 并返回 `ErrQueueFull`；业务侧 `audit.EmitBestEffort` 只记日志，不影响请求。
- 单 `run` goroutine 消费；`write` 按 `MaxAttempts` 重试（默认 3）、每次 `PublishTimeout`（750ms），失败计数并放弃；`Close` 关闭 queue 并等待。
- 默认 `QueueCapacity=1024`；`KAFKA_BROKERS` 为空时 producer 禁用（`Enabled=false`）。

## 6.7 共享可变状态

| 状态 | 位置 | 保护 |
|---|---|---|
| Chat/Group `clients` map | `chat/hub.go:run`、`group/ws/hub.go:eventLoop` | 单 goroutine + command channel |
| `Hub.commands/stop/done` | 两个 Hub | 有界 channel + `sync.Once` |
| `Client.send` | 两个 `client.go` | 有界 channel；写者 Hub，读者 writePump |
| `Client.control` | 两个 `client.go` | cap 4 channel |
| `Client.stopOnce/connectOnce` | 两个 `client.go` | `sync.Once` |
| Presence `desired/dirty` | `chat/presence/service.go` | `sync.Mutex` + wake cap 1 |
| friendnotify `closed/sender/events/dropped` | `chat/friendnotify/notifier.go` | `RWMutex`、有界 channel、`atomic.Uint64` |
| Audit `queue/closed/dropped` | `kafkaaudit/producer.go` | `RWMutex` + `atomic.Bool`；单消费 goroutine |
| Session unread/lastRead | `chat_sessions`、`group_sessions` | DB 行锁 + `ON DUPLICATE KEY UPDATE` |

# 7. 配置与部署

配置全部通过 dotenv 文件/进程环境读取（`server/internal/platform/config`），**不存在** `server/configs/*.yaml`。`-env`/`GIM_ENV_FILE` 优先，否则进程环境。

## 7.1 `.env.example` / `.env.compose.example` 键

| 分组 | 键 | 读取/默认/必填 |
|---|---|---|
| MySQL | `MYSQL_HOST/PORT/USER/PASSWORD/DATABASE` | `platform/database/mysql.FromValues`；必填（`CHANGE_ME` 视为缺失） |
| | `MYSQL_MAX_OPEN_CONNS/MAX_IDLE_CONNS/CONN_MAX_LIFETIME` | 默认 25/10/30m |
| Redis | `REDIS_HOST/PORT/PASSWORD/DB/POOL_SIZE` | `platform/redis.FromValues`；HOST/PORT 必填，PASSWORD 可空但 `CHANGE_ME` 报错，DB/POOL 默认 0/20 |
| etcd/生命周期 | `ETCD_ENDPOINTS/ETCD_DIAL_TIMEOUT/ETCD_SERVICE_TTL` | `platform/etcd`、`discovery`；ENDPOINTS 必填，默认 5s/15s |
| | `SERVICE_STARTUP_TIMEOUT/SERVICE_SHUTDOWN_TIMEOUT` | `platform/process`；默认 10s/10s |
| JWT | `JWT_SECRET/JWT_EXPIRE_SECONDS` | `auth/config`；必填，SECRET≥32B |
| Gateway | `GATEWAY_HOST/PORT/DISCOVERY_TIMEOUT/AUTH_TIMEOUT/PROXY_TIMEOUT` | `gateway/config`；HOST/PORT 必填，超时默认 2s/3s/15s |
| 内部 API | `INTERNAL_API_HOST`、`AUTH_API_PORT`…`LOGS_API_PORT`、`*_API_INSTANCE_ID` | `process.RESTConfig`；HOST/PORT 必填，INSTANCE_ID 可缺省 `<service>-local` |
| RPC | `USER_RPC_PORT`（`USER_RPC_HOST` 可选） | `process.RPCListenAddress`；PORT 必填 |
| Chat WS/消息 | `CHAT_WS_PATH/ALLOWED_ORIGINS/SEND_BUFFER/READ_LIMIT_BYTES/PONG_WAIT/PING_PERIOD/WRITE_WAIT` | `chat/config`；PATH/ORIGINS 必填，其余默认 64/1MiB/60s/50s/10s |
| | `CHAT_MESSAGE_MAX_TEXT_BYTES/MAX_PAYLOAD_BYTES/DEPENDENCY_TIMEOUT` | 默认 4096/16384/2s |
| | `CHAT_DELIVERY_OPERATION_TIMEOUT` | 默认 1s |
| | `IMAGE_MESSAGE_MAX_BYTES/FILE_MESSAGE_MAX_BYTES/RECALL_WINDOW_SECONDS` | 默认 10MiB/25MiB/120s |
| | `FRIEND_NOTIFY_BUFFER/WORKERS/OPERATION_TIMEOUT` | 可选，默认 256/1/2s（示例中注释） |
| Presence | `CHAT_PRESENCE_TTL/REFRESH_INTERVAL/RETRY_INTERVAL/OPERATION_TIMEOUT` | `chat/presence/config`；默认 90s/30s/5s/1s（refresh/retry < ttl） |
| Group WS | `GROUP_WS_PATH/ALLOWED_ORIGINS/SEND_BUFFER/READ_LIMIT_BYTES/PONG_WAIT/PING_PERIOD/WRITE_WAIT` | `group/config`；PATH/ORIGINS 必填，其余默认 64/1MiB/60s/50s/10s |
| | `GROUP_DEPENDENCY_TIMEOUT/GROUP_REDIS_OPERATION_TIMEOUT` | 默认 2s/1s |
| File | `FILE_STORAGE_ROOT/FILE_MAX_UPLOAD_BYTES` | `file/config`；ROOT 必填，MAX 默认 50MiB（≤1GiB） |
| Kafka | `KAFKA_BROKERS/AUDIT_TOPIC/CONSUMER_GROUP/QUEUE_CAPACITY/PUBLISH_TIMEOUT/RETRY_BACKOFF/MAX_ATTEMPTS` | `kafkaaudit`；BROKERS 空则禁用，其余默认 `gim.audit.events`/`gim-logs-v1`/1024/750ms/100ms/3 |
| Compose 专用 | `MYSQL_ROOT_PASSWORD`、`*_API_HOST`、`INTERNAL_BIND_HOST`、`FILE_STORAGE_ROOT=/var/lib/gim/files`、`KAFKA_BROKERS=kafka:29092` | 仅 `compose.yaml`/容器环境使用；Go 服务不读 `MYSQL_ROOT_PASSWORD` |

`*_API_HOST` 在 Compose 中为服务名（`auth-api` 等）；`INTERNAL_BIND_HOST=0.0.0.0` 让容器内监听所有接口。`FRIEND_NOTIFY_OPERATION_TIMEOUT` 在 `.env.compose.example` 中只有注释，未显式赋值。

## 7.2 compose.yaml 服务

| 服务 | 镜像/build | 依赖 | 健康检查 | 端口 | 说明 |
|---|---|---|---|---|---|
| mysql | `mysql:8.4` | — | `mysqladmin ping` | `127.0.0.1:13306:3306` | 数据卷 |
| redis | `redis:7.4-alpine` | — | `redis-cli ping` | `127.0.0.1:16379:6379` | Presence/delivery/禁言 |
| etcd | `registry.k8s.io/etcd:3.5.17-0` | — | `etcdctl endpoint health` | `127.0.0.1:12379:2379` | 服务发现 |
| kafka | `apache/kafka:4.2.2` | — | topics list | `127.0.0.1:19092:9092` | 审计 |
| migration | `gim-server:compose`，`/app/gim-migrate -command up` | mysql healthy | — | — | 一次性，exit 0 |
| admin-bootstrap | 同镜像，profile `tools` | migration completed | — | — | 手工创建首个 Admin |
| storage-init | `alpine:3.22` | — | — | — | 建 `temp/objects` 并 chown 65532 |
| user-rpc | `gim-server:compose` | migration | `nc 9091` | 内部 | 唯一 gRPC 服务 |
| user-api | 同镜像 | migration/redis/etcd/kafka | `/healthz` 8082 | 内部 | 资料/好友/Admin |
| auth-api | 同镜像 | user-rpc/redis/etcd/kafka | `/healthz` 8081 | 内部 | 注册/登录/鉴权 |
| chat-api | 同镜像，`GIM_DEBUG_ADDR=127.0.0.1:6060` | migration/user-rpc/redis/etcd/kafka | `/healthz` 8083 | 内部 | Chat WS + 消息 + Presence + friendnotify |
| group-api | 同镜像，debug | migration/user-rpc/redis/etcd/kafka | `/healthz` 8084 | 内部 | Group WS + 群消息 |
| file-api | 同镜像，debug + `file_storage` 卷 | migration/storage-init/etcd/kafka | `/healthz` 8085 | 内部 | 上传/下载 |
| settings-api | 同镜像 | migration/etcd/kafka | `/healthz` 8086 | 内部 | 设置/概览 |
| logs-api | 同镜像，debug | migration/etcd/kafka | `/healthz` 8087 | 内部 | 审计消费/查询 |
| gateway | 同镜像 | 7 个 API healthy | `/healthz` 8080 | `127.0.0.1:8080:8080` | 唯一 API 入口 |
| web | `docker/frontend.Dockerfile`（APP_DIR=web） | gateway healthy | `wget /` | `127.0.0.1:5173:8080` | 用户端 |
| admin | 同 Dockerfile（APP_DIR=admin） | gateway healthy | `wget /` | `127.0.0.1:4174:8080` | 管理端 |

启动顺序：mysql/redis/etcd/kafka → migration →（storage-init）→ user-rpc → 各 API → gateway → web/admin。

## 7.3 内外边界

- 对外仅 gateway（8080）、web（5173）、admin（4174），均绑定 `127.0.0.1`。
- 7 个内部 API 与 user-rpc 无 `ports`，仅在 `gim` 网络内按服务名访问。
- mysql/redis/etcd/kafka 的 loopback 端口仅用于本机调试。
- Admin 前端路由守卫不是安全边界；`adminapi.Identity` 依赖 Gateway 注入的 `Role`，内部 API 不得直接暴露。

## 7.4 一次性 job

| job | 作用 |
|---|---|
| `migration` | 执行 `gim-migrate -command up`，按 001–008 建表；成功后其他服务才启动 |
| `storage-init` | 在 `file_storage` 卷创建 `temp/objects` 并 chown `65532:65532`，供非 root file-api 写 |
| `admin-bootstrap` | `tools` profile 手工执行，创建首个 `Role=RoleAdmin` 账号；不更新既有账号、不打印凭据 |

# 8. 测试地图

统计（2026-11-14）：Go `*_test.go` 共 **109** 个（`server/tests` 23、`server/internal` 82、`server/tools/loadtest` 3、`server/migrations` 1）；Web vitest **21** 文件；Admin vitest **11** 文件；浏览器 E2E **4** 个 `.mjs`（`web/e2e/`）。

## 8.1 `server/tests/`（全部 gated）

所有文件默认 `t.Skip`，需要 `GIM_ENV_FILE` 指向真实 MySQL/Redis/etcd；Kafka 用例还需 `GIM_KAFKA_INTEGRATION=1`。

| 文件 | 覆盖 |
|---|---|
| `infrastructure_test.go` | MySQL/Redis/etcd 连通、凭据不回显 |
| `auth_redis_integration_test.go` | Auth + Redis 撤销/TTL |
| `gateway_auth_user_integration_test.go` | Gateway→Auth→User、discovery 生命周期 |
| `chat_gateway_websocket_integration_test.go` | 私聊 WS 全链路 |
| `chat_message_integration_test.go` | 文本 ACK/幂等/历史/会话 |
| `chat_session_integration_test.go` | session/未读/置顶/已读/隐藏 |
| `chat_presence_redis_integration_test.go` | Presence 读写/多端 |
| `group_foundation_integration_test.go` | 群 CRUD/成员/申请/EXPLAIN 索引 |
| `group_gateway_websocket_integration_test.go` | 群聊 WS 全链路 |
| `group_message_integration_test.go` | 群消息/会话/未读/删除 |
| `file_service_integration_test.go` | 上传、SHA-256 去重、下载 |
| `file_gateway_integration_test.go` | 经 Gateway 的上传/下载 |
| `file_authorization_integration_test.go` | 下载授权（owner/私聊/群成员/tombstone/退群） |
| `file_text_authorization_integration_test.go` | 文本不授予图片下载 |
| `image_message_integration_test.go` | `type=2` |
| `file_message_integration_test.go` | `type=3` |
| `recall_integration_test.go` | `type=4`、窗口、幂等、授权联动 |
| `friend_integration_test.go` | 好友生命周期 + 并发审批 |
| `friend_presence_integration_test.go` | 好友 online/offline、多端、过滤 |
| `admin_integration_test.go` | Admin 鉴权、用户限制 |
| `audit_repository_integration_test.go` | 审计落库/查询/幂等 |
| `audit_kafka_integration_test.go` | Kafka 生产→消费→落库→重启幂等（双门禁） |
| `database_migration_integration_test.go` | 001–008、dirty、DownAll、表清单 |

## 8.2 `server/internal/` 单测（默认运行）

| 包组 | 覆盖重点 |
|---|---|
| `platform/*` | apperror/httpresponse/config/process/discovery/etcd/redis/mysql/debugserver/audit/kafkaaudit/rediskeys/adminapi |
| `auth/*` | config、credential、token、service/allowlist、revocation、userclient、module、HTTP handler |
| `gateway/*` | config、route、proxy、authclient、module、handler |
| `user/*` | domain、repository/mysql、service、gRPC、profile、friend service/handler、admin service/handler |
| `chat/*` | hub、client lifecycle、handler、origin、presence、message service/inbound/HTTP/repository、message/admin、protocol、friendnotify |
| `group/*` | service/policy、message service/inbound/HTTP、ws hub/client/origin/envelope/handler、admin HTTP、config |
| `file/*` | storage、service、config、HTTP、admin HTTP |
| `settings/*`、`logs/*` | service、HTTP、consumer、overview |
| `migrations`、`tools/loadtest` | runner 解析/失败语义；loadtest HTTP/file/stats |

**门禁**：`server/internal` 无 build tag/Short 跳过，依赖以 fake/httptest 替代；`server/tests` 全部 gated；Kafka 双门禁。

## 8.3 Web / Admin 单测

- Web（21）：`api/client|auth.contract|friend|file`、`router/guard`、`stores/auth|chat|chat_reconcile|group|group_reconcile|file|friend`、`service/ws/chat_socket|group_socket`、`components/chat/MessageBubble|chat_components`、`components/file/file_components`、`components/friend/friend_components`、`components/group/group_components`、`utils/file|message`。
- Admin（11）：`api/client|auth.contract|admin.contract`、`router/guard`、`stores/auth`、`layouts/AdminLayout`、`ui/arco`、`utils/charts`、`views/LoginView|DashboardView|userStatusAction`。

## 8.4 浏览器 E2E（已入库 4 个 smoke）

| 文件 | 覆盖 | 运行 |
|---|---|---|
| `web/e2e/private-chat-smoke.mjs` | 双账号私聊冒烟 | `pnpm test:e2e:chat`，需账号环境变量 |
| `web/e2e/group-chat-smoke.mjs` | 群聊冒烟 | `pnpm test:e2e:group` |
| `web/e2e/file-management-smoke.mjs` | 文件管理冒烟 | `pnpm test:e2e:file` |
| `web/e2e/admin-compose-smoke.mjs` | Admin 登录/概览 | `node e2e/admin-compose-smoke.mjs` |

**注意**：图片、文件消息、撤回、好友上线、短窗口撤回等轮次的 E2E 为临时脚本，未入库；仓库内没有这些场景的可复现脚本。这与 `docs/validation-status.md` 中“E2E PASS”记录存在可复现性差异（见 8.5）。

## 8.5 与 validation-status 的出入（只记录）

1. validation-status 记录了好友/图片/文件/撤回/Presence 的浏览器 E2E PASS，但仓库只有 4 个通用 smoke，无对应脚本。
2. “web 21 files / 135 tests、admin 11 files / 25 tests”：文件数 21/11 已核对；用例数未运行验证。
3. “69/70 packages”“集成 20 PASS / 1 SKIP”：未逐包/逐用例核对；`server/tests` 有 23 个文件、多个 Test，SKIP 数取决于两个环境变量。
4. CI（`.github/workflows/ci.yml`）只跑默认单测/vet/gofmt/前端/race，不跑 `server/tests`、Kafka、浏览器 E2E。

# 9. 已知边界与 V2 范围

范围口径以 `docs/v2-scope.md` 为准；边界明细以 `docs/known-issues.md` 为准；验证结果以 `docs/validation-status.md` 为准。本节只给代码指针，不重复展开。

| 编号 | 一句话 | 代码位置 |
|---|---|---|
| K-14 | 并发不同 clientMsgId 撤回同一消息可能落两条撤回消息；历史占位唯一、前端合并、授权不受影响；严格一条需 schema 决策 | `server/internal/chat/message/service.go:sendWithdraw` 先查 `GetWithdrawBySenderAndTarget` 再落库；`server/internal/chat/message/repository/mysql/repository.go:GetWithdrawBySenderAndTarget` 仅按 sender+target 查询，`chat_messages` 无 target 唯一索引；`web/src/stores/chat_reconcile.ts:applyWithdrawals` 合并重复行 |
| K-15 | 好友上线提醒是单机语义，多实例可能重复/误报 | `server/internal/chat/friendnotify/notifier.go:singleInstancePresence.ShouldNotify` 恒返回 true；`GlobalPresenceCheck` 为 V2 占位；Hub 本地 0→1/1→0 在 `server/internal/chat/hub.go:registerClient` / `removeClient` |
| K-16 | `friend.offline` best-effort；崩溃/TTL 过期不保证发送；队列满丢弃 | `server/internal/chat/friendnotify/notifier.go:enqueue`（非阻塞、满则丢弃）、`Shutdown`（drain）；`server/internal/chat/presence/service.go` 的 TTL/刷新；进程崩溃无 Hub 回调属运行语义，代码无补偿 |
| E-8 | 文件授权查询使用 `JSON_EXTRACT` / `NOT EXISTS`，无索引；仅非所有者下载路径；无限速 | `server/internal/file/repository/mysql/repository.go:canAccessImage`（chat/group 两条查询） |
| E-9 | 浏览器已缓存的图片/文件副本不回收；撤回/退群只阻止新的服务端下载 | 浏览器缓存行为，仓库内无对应代码；服务端授权见 `server/internal/file/service/service.go:GetAccessible` 与 `server/internal/file/transport/http/handler.go:Download` |

V2 范围外项（`user_confs`、管理员撤回、多实例 Presence、好友申请实时推送、完整性能阶梯、语音/视频/@/图文等）见 `docs/v2-scope.md` 第 2 节。

## 9.1 文档与实现差异汇总（只记录，不改旧文档）

| # | 现象 | 证据/位置 |
|---|---|---|
| A1 | ~~`chat_message_deletions` / `group_message_deletions` 生产代码只读，未找到写入方；隐藏会话实际走 `hidden_at`~~ **已记录（K/A1）** | `chat/message/repository/mysql/history.go`、`session.go`；`group/message/repository/mysql/history.go`、`session.go` |
| A2 | ~~`DELETE /api/chat/chat` 未找到 HTTP 入口；`Service.HideSession` 未接线~~ **已记录（K/A2）** | `chat/message/service.go:HideSession`，`transport/http/handler.go` 无路由 |
| A3 | ~~注册不初始化 `user_confs`；`CanChat` 用 `COALESCE(MAX(...),0)` 容忍~~ **已记录（K/A3）** | `user/service/service.go:CreateUser`、`chat/message/repository/mysql/friendship.go:CanChat` |
| A4 | ~~`chat/message/admin` 使用 `2220/2221/2222`，与撤回错误码数值重叠~~ **已修复（第十六轮）**：Chat Admin 改为 `2230/2231/2232`；Group Admin 保持 `2320–2322` | `chat/message/admin/transport/http/handler.go` 常量；撤回在 `chat/message/service.go` |
| A5 | ~~Group 消息仓库无 1213 死锁重试（Chat 有 3 次）~~ **已记录（K/A5）** | `group/message/repository/mysql/repository.go`；Chat 见 `chat/message/repository/mysql/repository.go:PersistWithSessions` |
| A6 | ~~`/api/auth/open_login` 在 Auth Allowlist，但未找到路由注册；`/api/settings/open_login` 实际存在~~ **已记录（K/A6）** | `auth/service/allowlist.go:DefaultAllowlist`；`settings/transport/http/handler.go:OpenLogin` |
| A7 | ~~validation-status 的多个浏览器 E2E 无入库脚本~~ **已澄清（第十六轮）**：好友/图片/文件/群聊/撤回/窗口/好友上线为临时脚本；仓库仅 4 个 smoke，validation-status 已加“E2E 证据链说明” | `web/e2e/`；`docs/validation-status.md` |
| A8 | ~~`server/migrations/README.md` 称迁移到 007，实际有 008~~ **已修正（第十六轮）** | `server/migrations/README.md` |
| A9 | ~~`schema_migrations` 未列入 docs/04 索引清单~~ **已补充（第十六轮）** | `docs/04_DATABASE_DESIGN.md` §7.1/§7.11 |
| A10 | ~~`groups.is_all_muted` 只读，未找到修改入口~~ **已记录（K/A10）** | `group/message/service.go:Send`；`group/transport/http` 无对应路由 |
| A11 | ~~docs/04 的 `user_confs`/`friend_verifies`/`file_*`/`friends` 若干列名与 migration 不一致（D1–D5）~~ **已修正（第十六轮）** | 第 5.1 节 |


# 10. 文件索引

## 10.1 `server/internal/`

| 子包 | 一句话职责 | 关键文件 |
|---|---|---|
| `platform/` | 配置、MySQL/Redis/etcd、发现注册、进程生命周期/健康、错误/响应、审计/Kafka、Redis key、Admin 鉴权、debug | `config/env.go`、`database/mysql/client.go`、`discovery/registry.go`、`process/rest.go`、`apperror/error.go`、`httpresponse/response.go`、`audit/event.go`、`kafkaaudit/producer.go`、`rediskeys/keys.go`、`adminapi/adminapi.go` |
| `auth/` | 注册/登录/注销、密码、JWT、撤销、HTTP、User RPC 客户端 | `service/service.go`、`credential/password.go`、`token/jwt.go`、`revocation/redis.go`、`transport/http/handler.go`、`userclient/client.go` |
| `user/` | 资料、好友、Admin 用户 | `domain/user.go`、`repository/mysql/repository.go`、`service/service.go`、`transport/grpc/server.go`、`transport/http/profile.go`、`friend/friend.go`、`friend/repository/mysql/repository.go`、`admin/service.go`、`admin/mysql/repository.go` |
| `chat/` | 私聊 Hub/Client、Presence、消息/会话/撤回、好友提醒、Admin 消息 | `hub.go`、`client.go`、`handler.go`、`presence/service.go`、`message/service.go`、`message/inbound.go`、`message/repository/mysql/*`、`friendnotify/notifier.go`、`protocol/envelope.go` |
| `group/` | 群/成员/申请/禁言、Group WS、群消息/会话、Admin | `domain/group.go`、`repository/mysql/repository.go`、`service/service.go`、`ws/hub.go`、`ws/client.go`、`message/service.go`、`message/repository/mysql/*`、`mute/store.go`、`admin/service.go` |
| `file/` | 流式上传、SHA-256 去重、所有权/授权下载、本地存储、Admin | `domain/file.go`、`repository/mysql/repository.go`、`service/service.go`、`storage/local.go`、`transport/http/handler.go`、`admin/service.go` |
| `settings/` | 公开设置、Admin 概览 | `domain/settings.go`、`repository/mysql/repository.go`、`service/service.go`、`transport/http/handler.go`、`overview/service.go` |
| `logs/` | 审计查询、Kafka 消费落库 | `domain/log.go`、`repository/mysql/repository.go`、`service/service.go`、`consumer/consumer.go`、`transport/http/handler.go` |
| `gateway/` | 路由、反代、Auth 链、服务发现 | `route/route.go`、`proxy/proxy.go`、`authclient/client.go`、`handler.go`、`config/config.go` |

## 10.2 `server/` 其余

- `cmd/`（11）：`gateway`、`auth-api`、`user-api`、`user-rpc`、`chat-api`、`group-api`、`file-api`、`settings-api`、`logs-api`、`migrate`、`admin-bootstrap`。
- `api/user/v1/`：唯一 gRPC 契约（`user.proto`、`user.pb.go`、`user_grpc.pb.go`）。
- `tests/`：23 个 gated 集成测试；`admin_integration_test.go:openAdminTestDatabase` 提供共享 DB helper。
- `migrations/`：001–008 up/down + `runner.go` + `runner_test.go` + `README.md`。

## 10.3 `web/src/`

| 目录/文件 | 职责 |
|---|---|
| `main.ts` / `App.vue` | 应用装配、Pinia/Router |
| `api/` | Axios 客户端与 auth/chat/friend/file/group 接口 |
| `service/ws/` | `chat_socket.ts`、`group_socket.ts`（两条独立 WS） |
| `stores/` | auth/chat/group/friend/file + `chat_reconcile.ts`/`group_reconcile.ts` |
| `router/`、`layouts/` | 路由守卫、`MainLayout.vue` |
| `views/` | Login/Register/Home/Chat/Contacts/Group/File/Profile |
| `components/` | chat（Composer/MessageList/SessionList/MessageBubble）、file、friend、group |
| `types/`、`utils/`、`styles/` | 类型、消息/文件/session 工具、全局样式 |

## 10.4 `admin/src/`

| 目录/文件 | 职责 |
|---|---|
| `api/` | `client.ts`、`auth.ts`、`admin.ts`（用户/群/消息/文件/日志/概览） |
| `router/` | 路由 + 前端角色守卫（非安全边界） |
| `stores/auth.ts` | 管理员登录态 |
| `layouts/AdminLayout.vue` | 侧边栏/顶栏 |
| `ui/arco.ts`、`utils/` | 组件注册、charts/format/session |
| `views/` | Dashboard/Users/Groups/Chats/Files/Logs/Settings/Login/Forbidden + `userStatusAction.ts` |
| `components/` | **空目录**（未找到组件源文件，页面直接写在 views） |
