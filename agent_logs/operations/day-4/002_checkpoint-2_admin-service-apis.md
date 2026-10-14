# Day 4 / Checkpoint 2 — Admin Service APIs

## 时间

- 2026-10-14 10:52 +08:00

## 目标

- 在 User/Chat/Group/File/Settings 各目标服务内实现管理员 API，并继续由 Logs 提供管理员查询。
- 所有 API 同时验证 Gateway/Auth 注入的非零 `User-ID` 与 `Role == 1`。
- 为管理动作保留数据一致性、幂等事实和结构化审计。

## 参考行为与自主实现

- 参考了 FIM Admin 的用户限制、私聊记录、群/群消息、文件、设置和日志页面/API 语义。
- 未复制 FIM Handler/Logic/Model；GIM 使用独立的 Repository/Service/HTTP Transport，实现 bounded cursor、目标服务二次鉴权和安全响应 DTO。
- 未沿用 FIM 的物理消息删除、共享可变 Pusher、任意 Settings Secret、文件物理删除等风险实现。

## 修改

- 新增 `internal/platform/adminapi`：统一解析可信管理员身份、有界 cursor/limit、严格 JSON body。
- 新增 User Admin：安全列表/在线状态/统计、限制策略、账号禁用（保留历史，禁止管理员自禁用）。
- 新增 Chat Admin：筛选列表与 tombstone 删除；事务修复 session last-message/unread。
- 新增 Group Admin：群/在线成员/消息统计、群消息列表与 tombstone 删除、管理员解散群。
- 新增 File Admin：安全元数据列表、受保护下载、UserFile 引用删除；不返回 storage path、不删除共享物理对象。
- 新增 Settings Service/API：只允许公开站点业务配置，不接受任意 key，不保存 Kafka/DB/Redis/JWT/OAuth Secret。
- 管理动作成功后发送独立结构化 `admin.*` audit event；Kafka 失败仍不反转已提交动作。

## Migration

- 新增 `008_admin_management.up/down.sql`。
- `chat_messages/group_messages.admin_deleted_at DATETIME(3) NULL` 保留 clientMsgId 幂等事实。
- 用户 history/session/read 查询过滤 tombstone；session 修复与 tombstone 同事务。
- 未增加低选择性的 tombstone 单列索引；既有 conversation/group cursor 索引仍先缩小查询范围。

## 测试

- `go test ./... -count=1`（workspace GOCACHE）→ PASS。
- `go test ./tests -run TestDatabaseMigrationsAndUserAuthRegression -count=1 -v`（真实 MySQL）→ PASS；覆盖 008 UP/DOWN/RE-UP 和既有 EXPLAIN。
- `go test ./tests -run TestAdminRepositoriesAndManagementActions -count=1 -v`（真实 MySQL）→ PASS；覆盖用户限制/禁用、Chat/Group tombstone、群解散、文件引用删除、Settings round-trip。
- 首轮 Group list 因保留字 alias `groups` 触发 MySQL 1064；改为 alias `g` 后通过。
- 首轮扫描 `MemberIDs` 出现 GORM unsupported warning；添加 `gorm:"-"` 后无警告通过。

## 协议与安全

- 管理列表默认 30/50、最大 100；不允许无限 limit。
- 普通用户与只有 `Role: 1` 但无有效 `User-ID` 的请求均由目标服务 403。
- Admin 文件下载设置 attachment、nosniff、sandbox CSP。
- Response 不包含 password hash、OpenID、storage path、部署 Secret 或原始 SQL/错误。

## Git

- commit：待 Checkpoint 2 完整验收后创建实现提交。
- push：待完整验收。

## 下一步

- 完成 Admin 浏览器联调、全量回归、Secret Scan 和最终状态日志。
