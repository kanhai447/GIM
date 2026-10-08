# 01 项目总体架构

## 1. 原始参考项目事实

原项目不是两个工程，而是三个源码工程：

```text
fim_server-main      Go 后端
fim_web-master       用户端 Vue3 Web
fim_admin-master     管理端 Vue3 Admin
```

后端 README 所称“8 大服务”是逻辑域：Gateway、Auth、User、Chat、Group、File、Settings、Logs。User/Chat/Group/File/Settings 进一步包含 API/RPC 进程；基础设施包括 MySQL、Redis、etcd，日志链路还使用 Kafka/ZooKeeper。

## 2. GIM V1 目标拓扑

```text
                  +-------------------+
                  |  Vue3 User Web    |
                  | Element Plus/Pinia|
                  +---------+---------+
                            |
                     HTTP + Chat/Group WS
                            |
                            v
                       gim_gateway
                            ^
                            |
                  +---------+---------+
                  | Vue3 Admin Web    |
                  | Arco/ECharts      |
                  +-------------------+
                            |
                           HTTP
                            v
                  Gateway -> Auth 校验
                            |
       +----------+---------+---------+----------+----------+
       |          |                   |          |          |
    gim_user   gim_chat            gim_group   gim_file  gim_settings
    API/RPC    API/RPC             API/RPC     API/RPC   API/RPC
       \          |                   /            |
        +---------+----- gRPC/etcd --+-------------+
                            |
                         MySQL/Redis

                      业务/操作日志事件
                            |
                          Kafka
                            |
                        gim_logs API
                            |
                          MySQL
```

## 3. 请求链路

普通用户 HTTP：

```text
User Web -> Gateway -> Auth(authentication) -> Gateway -> Target API -> RPC/DB/Redis
```

管理端 HTTP：

```text
Admin Web -> Gateway -> Auth(authentication, role=1) -> Gateway 注入 Role ->
Target API AdminMiddleware -> Admin Logic
```

浏览器路由 `meta.role` 只用于隐藏/阻止页面导航，真正权限必须在服务端验证。

WebSocket：

```text
User Web -> Gateway WS proxy -> Auth validation -> Chat/Group WS endpoint -> Hub -> Client
```

## 4. 服务职责

### gim_gateway
服务发现、反向代理、统一调用 Auth、注入 User-ID/Role、HTTP/WS Upgrade 转发。V1 不改成本地验 JWT。

Gateway 通过 etcd 前缀 `/gim/services/{service}/{instance}` 查找 API 服务，resolver 返回端点列表以保留多实例扩展边界；Day 1 V1 转发选择排序后的首个有效 lease 注册端点。服务以 TTL lease + keepalive 注册并在优雅退出时 revoke。Gateway 路由使用 `auth/user/chat/group/file/settings/logs` 显式映射，禁止从外部路径模糊拼接任意 etcd key。

### gim_auth
注册、登录、JWT 签发/验证、logout blacklist、第三方登录兼容。日志不得记录密码、Token 或完整敏感 Header。

### gim_user
用户资料/配置、搜索、好友、好友验证、Admin 用户列表/限制/删除、基础信息 RPC、Presence 查询。

### gim_chat
私聊 WS、Hub/Client、消息持久化、ACK/幂等、历史、session、未读/已读、回复/引用/撤回；Admin 私聊记录查询/删除。

### gim_group
群资料/成员/角色/验证/禁言、群 WS、ACK/幂等、群 session、未读；Admin 群列表、群消息查看/删除。

### gim_file
上传/下载/预览、SHA-256、FileObject/UserFile、Admin 文件列表/删除。

### gim_settings
公开站点配置、Admin 系统配置修改。

### gim_logs
消费 Kafka 的安全结构化日志，提供 Admin 日志列表/读取/删除。V1 不允许复制原项目的 HTML 日志拼接方案。

## 5. 两个前端的处理原则

- `web/`：保留用户端页面和交互，重点改 WS 客户端、ACK、重连、未读。
- `admin/`：保留 Arco 管理后台，连接真实 Admin API；不要求重做 UI。
- Admin 首页原 `/api/data/statistic`、`/api/data/weather`、`/api/data/login_statistic` 只有 Mock，没有对应后端。GIM V1 不允许声称这些是已实现真实功能。可先隐藏 Mock-only 卡片，或在 Day 4 有余量时实现真实 IM 指标；天气不是核心。

## 6. 部署边界

原 docker-compose 只部署用户端 `fim_web`，没有部署 `fim_admin`。GIM 应补充 Admin 的构建/启动说明；正式环境只公开 Gateway 和前端入口，API/RPC 服务不直接暴露公网，避免绕过 Gateway 伪造 `Role` Header。

Gateway 在调用 Auth 前必须删除外部请求中的 `User-ID`、`Role`、`ValidPath`；公开请求保持无身份 Header，受保护请求只注入 Auth 返回的可信身份。内部 API 对这些 Header 的信任以网络隔离和仅公开 Gateway 为前提。
