# 15 三仓库源码复审结果

本文件记录基于 `fim_server-main`、`fim_web-master`、`fim_admin-master` 的实际源码观察，优先级高于早期仅基于文档的推测。

## 1. 范围修正

原项目由三个工程构成：Server、用户 Web、Admin。Admin 是独立前端，调用同一 Gateway/后端，不是独立后端服务。

## 2. WebSocket

Chat 和 Group 各维护一套全局普通 map，且多个业务路径直接对 `*websocket.Conn` 调用 `WriteMessage/WriteJSON`。这确认了 GIM 必须采用 Hub/Client + 单 writePump。用户端也确认缺少完整自动重连和 ACK。

Chat WS 将 `online` Presence 写 Redis；Group WS 只维护本地连接表。GIM 继续规定只有 Chat WS 能修改全局 Presence。

## 3. 会话查询

实际私聊代码并不是简单 `max(msg_preview)`，而是用相关子查询获取最新 preview，因此早期“MAX preview 必错”的表述需要撤回。

但现有实现仍然每次从消息表聚合最近会话，SQL 复杂、扩展性差；显式 `chat_session/group_session` 仍是 GIM 优化方向。

群会话现有代码存在更实际的问题：用户删除群消息的过滤查询没有按当前 `user_id` 限定，且最新 preview 子查询没有同步应用用户删除过滤，删除语义可能影响错误的会话预览。Session 模型可一并解决。

## 4. 文件服务

确认原文件/图片上传使用 `io.ReadAll` + MD5 + `os.WriteFile`。Hash 命中时直接返回已有 `FileModel`，不会建立新的用户-文件拥有关系；同时 FileModel 删除钩子会删除物理文件。这使跨用户秒传、管理员删除、消息引用之间的所有权/生命周期语义不完整。

GIM 必须采用流式 Hash、临时文件、FileObject/UserFile 或等价引用模型。目录权限也不能沿用 `MkdirAll(..., 0666)`，目录需要执行位。

## 5. Admin

真实接口包括 User/Chat/Group/File/Settings/Logs 管理；Dashboard `/api/data/*` 只有 Mock，后端不存在。不要把 Mock 统计当成 GIM 后端需求；如需统计，应按 GIM 自身设计重新实现。

## 6. 日志系统

原 Kafka -> Logs API -> MySQL 链路是真实存在的，Admin 有日志列表 UI，因此 Kafka/Logs 在有 Admin 的完整项目中具有业务价值。

但原实现有必须修复的问题：

- `ServiceContext` 共享一个可变 `*Pusher` 给并发请求，可能造成数据竞争和日志串线；
- 登录密码错误时会把明文密码写入操作日志；
- 请求 Header 可能包含 Token，原中间件会整体记录；
- 日志内容拼接成 HTML，Admin 用 `v-html` 渲染，存在注入风险；
- 消费端在函数内部临时创建 mutex，无法在并发 Consume 之间形成有效互斥。

GIM 应改为“共享 Kafka producer、每个请求创建独立结构化 LogEvent”，并对 `password/token/authorization/cookie/secret` 等字段统一脱敏。

## 7. GitHub 安全

原 Server 仓库包含不应复制进 GIM Git 历史的敏感材料，包括 TLS 私钥文件、JWT/数据库/第三方登录硬编码配置等。Agent 阅读 reference 时必须识别 secrets，但 GIM 不复制 reference 源码。新项目只创建 `.env.example`/示例配置和自主实现代码。不要先复制后删除；reference 默认不进入 GIM Git。

## 8. Admin 部署

原 `deploy/docker-compose.yaml` 和 Nginx 配置只部署用户端 Web，没有 Admin 前端。GIM 若把 Admin 作为 V1，应补充其构建/开发启动说明；正式发布是否同域 `/admin` 或独立域名可在 Day 4 决定，但不应改变 API 权限模型。
