# 001 / Task 1 — 架构文档：进程、HTTP、WS 与私聊链路

- **日期**：2026-11-14（Asia/Shanghai）
- **开始状态**：`HEAD = 2d849bc`，工作区干净，六个 tag 存在。
- **结果**：新建 `docs/architecture-deep-dive.md`，完成第 1–3 章、第 6 章、第 9 章，以及第 4 章私聊/文件/好友/会话/上线提醒小节；未改业务代码。

## 覆盖内容

- 第 1 章：11 个 `server/cmd/` 入口的 main/run、启动顺序、关键 import、etcd/端口/健康检查、优雅关闭。
- 第 2 章：Gateway `ServeHTTP` 清头 → route.Parse → Auth → 身份注入 → 服务发现 → 反向代理；登录/好友申请/Admin 用户列表三条调用链；错误码层级。
- 第 3 章：Chat/Group WS 的 Upgrade、Auth、Hub register/unregister、readPump/writePump、慢客户端、ACK/推送、Presence、friendnotify。
- 第 4 章（私聊部分）：认证、用户/Admin、好友、私聊文本/图片/文件/撤回、session、文件上传下载、好友上线提醒。
- 第 6 章：Hub 单写、锁顺序、1213 重试（Chat 有/Group 未找到）、幂等唯一索引、Presence、friendnotify、审计 producer、共享状态表。
- 第 9 章：K-14/K-15/K-16/E-8/E-9 代码指针。

## 路径抽查（10 个）

`server/cmd/gateway/main.go`、`server/internal/gateway/handler.go`、`server/internal/chat/hub.go`、`server/internal/chat/client.go`、`server/internal/chat/friendnotify/notifier.go`、`server/internal/group/ws/hub.go`、`server/internal/chat/presence/redis.go`、`server/internal/user/admin/mysql/repository.go`、`server/internal/platform/kafkaaudit/producer.go`、`server/internal/chat/message/inbound.go` 均存在。

## 已记录的不确定/差异

- Group 消息仓库未找到 1213 重试。
- `DELETE /api/chat/chat` 未找到 HTTP 入口；`chat_message_deletions` 未找到生产写入方。
- `/api/auth/open_login`、`/api/settings/open_login` 在 Allowlist 但未找到路由。

## Git

- commit：`docs: document process, HTTP and WebSocket architecture`（本记录所在提交）。
