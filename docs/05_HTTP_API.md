# 05 HTTP API 契约

## 1. 统一响应

兼容参考前端：

```json
{"code":0,"msg":"成功","data":{}}
```

V1 不强制一次性改成 RESTful status 语义。网关/系统级错误仍应使用合理 HTTP status；业务响应保持 code 兼容。

客户端 Token Header V1 保持：`token: <jwt>`。

Gateway 接受 `/api/{service}/...`，其中 service 仅允许 `auth/user/chat/group/file/settings/logs` 的显式映射。非法路径使用 `1301`，不支持的服务使用 `1302`，服务未注册/发现不可用使用 `1303`，Auth 不可用/超时使用 `1304`，上游连接失败使用 `1305`，代理超时使用 `1306`。系统级错误使用对应的 4xx/5xx HTTP status，响应仍保持统一 envelope。

Gateway 不解析 JWT，也不维护第二套公开路径。每个合法 API 请求都调用 Auth authentication；Auth 返回 `public=true` 时无身份 Header 放行，返回 `authenticated=true` 时 Gateway 注入 `User-ID` / `Role`。任何客户端提供的 `User-ID`、`Role`、`ValidPath` 均先删除。

## 2. Auth

- `POST /api/auth/login` `{account,password}` -> `{token,user}`；兼容既有请求字段 `userName`，但 `account` 与 `userName` 同时存在时必须一致
- `POST /api/auth/register` `{account,nickname,pwd,rePwd}` -> 公开用户资料；不得返回密码哈希
- `POST /api/auth/authentication` 内部/网关认证，兼容原 Header `Token`/`ValidPath`，返回 `{userID,role,authenticated,public}`
- `POST /api/auth/logout`，从 `token` Header 读取 JWT，并返回 `{loggedOut:true}`
- `POST /api/auth/open_login` 可保留但非 V1 验收重点

JWT Claims 使用 `userID`、`role`、`jti`、`iat`、`exp`。签名 Secret 和过期时间分别从 `JWT_SECRET`、`JWT_EXPIRE_SECONDS` 读取；错误响应不得暴露验签细节。

`authentication` 的 V1 精确公开路径为：`/api/auth/login`、`/api/auth/register`、`/api/auth/open_login`、`/api/settings/info`、`/api/settings/open_login`。允许携带 query，但路径必须完全匹配；禁止 substring 或正则模糊放行。

## 3. User

保持参考路径：

- `GET /api/user/user_info`
- `PUT /api/user/user_info`
- `GET /api/user/friend_info?friendID=`
- `GET /api/user/friends`
- `PUT /api/user/friends` 修改备注
- `DELETE /api/user/friends`
- `GET /api/user/search`
- `POST /api/user/valid`
- `GET /api/user/valid`
- `PUT /api/user/valid_status`
- `POST /api/user/friends` 添加好友

`GET /api/user/user_info` 的最小公开资料包含 `userID/account/nickname/avatar/role/status`，不得返回 `pwd_hash`。调用方用户 ID 由后续 Gateway/Auth 链路通过可信 `User-ID` Header 注入；浏览器直接提供该 Header 不是安全边界。

好友返回必须包含 `isOnline`，值来自 Redis Presence。

## 4. Chat

保留原接口：

- `GET /api/chat/history`
- `GET /api/chat/session`
- `POST /api/chat/user_top`
- `DELETE /api/chat/chat`

新增：

### `PUT /api/chat/read`

请求：

```json
{"peerUserID":1002,"lastReadMessageID":12345}
```

语义：只允许推进 lastReadMessageID，不允许倒退；更新当前用户 session unread。返回最新 session 状态或空 data。

会话返回至少包含：peer 基础信息、lastMessage、lastMessageAt、unreadCount、isTop。

## 5. Group

保持参考项目已有 group 路由：创建/更新/删除、成员、角色、邀请、验证、搜索、禁言、历史、session、top、my 等。

新增：

### `PUT /api/group/read`

```json
{"groupID":2001,"lastReadMessageID":67890}
```

更新当前成员 group_session。

## 6. File

兼容：

- `POST /api/file/image`
- `POST /api/file/file`
- `GET /api/file/:id-or-uuid`

上传使用 multipart。后端必须限制最大大小并验证 MIME/扩展名策略；返回 URL 保持前端可直接用于消息。

## 7. 错误约定

优先复用参考项目现有 code。新增错误至少区分：未认证/无权限、参数错误、好友关系不存在、群成员不存在/禁言、重复 clientMsgId（应返回原消息而非失败）、消息不存在/无权撤回、文件超限/类型不允许、系统错误。

User 基础能力使用：`1001` 参数错误、`1101` 用户不存在、`1102` 账号已存在；数据库原始错误只作为服务端私有 cause，不进入 HTTP/RPC 响应。

Auth 使用：`1201` 账号或密码错误、`1202` 用户禁用、`1203` Token 缺失、`1204` Token 非法、`1205` Token 过期、`1206` Token 已注销。登录的账号不存在与密码错误统一返回 `1201`，避免账号枚举；User RPC/Redis 内部错误使用安全的通用系统错误。

## 8. API 兼容规则

Agent 不得为了“更 RESTful”随意改已有路径和字段。需要改时先提供兼容层或同步修改前端和本文件。

## 9. Admin API（来自真实 fim_admin + fim_server 源码）

所有以下接口都必须经过 Gateway/Auth，并由目标服务 AdminMiddleware 校验角色 1。

### User Admin
- `GET /api/user/users`
- `POST /api/user/curtail`
- `DELETE /api/user/users`

### Chat Admin
- `GET /api/chat/admin/history`
- `GET /api/chat/admin/session`（参考后端存在，Admin 当前页面主要使用 history）
- `DELETE /api/chat/admin/history`

### Group Admin
- `GET /api/group/groups`
- `DELETE /api/group/groups`
- `GET /api/group/messages/:id`
- `DELETE /api/group/messages`

### File Admin
- `GET /api/file/files`
- `DELETE /api/file/files`

### Settings Admin
- `PUT /api/settings/admin/info`

### Logs Admin
- `GET /api/logs/logs`
- `GET /api/logs/logs/:id`
- `DELETE /api/logs/logs`

## 10. Admin Mock-only 接口

以下路径存在于 `fim_admin` Mock，但在 `fim_server` 中没有对应服务/路由：

- `GET /api/data/statistic`
- `GET /api/data/weather`
- `GET /api/data/login_statistic`

Agent 不得自行创建 `gim_data` 微服务。V1 可隐藏这些 Dashboard 模块；若用户明确要求真实统计，再基于现有服务设计真实指标接口。
