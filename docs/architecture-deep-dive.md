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
| File API | `server/cmd/file-api/main.go:main` / `run` | runtime/audit → file 配置 → MySQL/etcd → `file.New`（storage+repository）→ REST + admin 路由 → `registry.Register` → `ServeREST` | `internal/file`、`file/service`、`file/repository/mysql`、`file/storage`、`file/admin` | `FILE` / `file_api`；`FILE_API_PORT`（8085）；`/healthz` | `ServeREST` 信号关闭；defer 关闭 MySQL/etcd/audit |
| Settings API | `server/cmd/settings-api/main.go:main` / `run` | runtime/audit → MySQL/etcd → settings module + overview → REST 路由 → `registry.Register` → `ServeREST` | `internal/settings`、`settings/overview`、`platform/mysql` | `SETTINGS` / `settings_api`；`SETTINGS_API_PORT`（8086）；`/healthz` | `ServeREST`；defer 关闭依赖 |
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

（群聊、Admin、审计、migration 见后续小节）

# 5. 数据模型与索引

（待填充）

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

（待填充）

# 8. 测试地图

（待填充）

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

# 10. 文件索引

（待填充）
