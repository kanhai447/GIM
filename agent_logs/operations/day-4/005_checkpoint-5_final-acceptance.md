# Day 4 / Checkpoint 5：V1 最终验收、文档与面试材料

## 操作状态

- 状态：COMPLETED
- 日期：2026-10-15
- 范围：仅执行 GIM V1 最终验收、文档冻结、面试材料整理与发布收口；未新增业务能力。

## 开始前基线

- 分支：`main`。
- 开始时 `HEAD == origin/main == 1cf870b18b5833fdb02e108e36dbf4b108b62822`，工作树 clean。
- Compose：16 个常驻服务 healthy；`migration` 与 `storage-init` 均成功退出（0）。
- Day 1、Day 2、Day 3、Day 4 Checkpoint 1–4 均已完成。

## 修改文件

- 重写项目入口：`README.md`。
- 新增 `docs/architecture/{overview,reliability,file-and-audit,decisions}.md`。
- 新增 `docs/interview/{project-overview,architecture-questions,reliability-questions,resume-bullets}.md`。
- 更新 `docs/deployment/docker-compose.md`，移除过期的 load/race pending 说明。
- 更新 `agent_logs/day-4.md`、`agent_logs/AGENT_WORKLOG.md`、`agent_logs/CURRENT_STATE.md` 和本记录。

没有修改后端/前端业务代码、数据库 migration、HTTP API 或 WebSocket 协议。

## Backend / Frontend acceptance

- Backend `go test ./... -count=1`：PASS。
- Backend `go vet ./...`：PASS。
- repository-wide `gofmt -l`：无输出；`git diff --check`：PASS。
- Web `pnpm type-check`：PASS；Vitest 16 files / 90 tests：PASS；production build：PASS。
- Admin `pnpm type-check`：PASS；Vitest 11 files / 25 tests：PASS；production build：PASS。
- Admin build 保留已知的非阻塞 chunk-size warning。

## Compose focused smoke

所有请求均经过公开 Gateway：

- Login：PASS。
- Private Chat：4 个认证 WS；12/12 unique message ACK、12/12 delivery、12 条持久化，4 次 duplicate retry 返回一致 ACK；0 failure/timeout/protocol/business error/drop。
- Group Chat：3 个认证 WS；12/12 ACK、36/36 fanout delivery、12 条持久化，4 次 duplicate retry；0 failure/timeout/protocol/business error/drop。
- File：4 KiB 流式 upload/download/hash correctness/reference delete：PASS。
- Admin Logs：一次性 Admin 经 Gateway login，`GET /api/logs/logs` 返回 5 条安全结构化事件，logout：PASS。
- 首次手写 Admin 请求误用 `Authorization` 而不是当前兼容 `token` header，按设计收到 1203；临时账号立即清理。改用前端真实协议后 PASS。这是验收调用修正，不是产品故障。

本轮 `perf-user-final-*` 用户、群、消息、File 引用和两个 `final-admin-*` 临时管理员均已精确清理并验证为 0。

## Migration acceptance

`TestDatabaseMigrationsAndUserAuthRegression` 在自动创建的隔离空库中完成：

- 001–008 ordered UP：PASS，`schema_migrations` 为 8 applied / 0 dirty。
- schema、明文密码列缺失、唯一索引和 16 类关键 EXPLAIN：PASS。
- repeated UP、DOWN ALL、dirty-state protection、RE-UP：PASS。
- User/Auth regression：PASS。
- 隔离数据库由测试自动删除；未对主开发库执行 destructive down。

## Security acceptance

- `scripts/check-repository-safety.ps1`：PASS；本地 env、`reference/`、私钥/证书、依赖、构建物和数据目录 ignore 覆盖完整。
- tracked path 无 `.env.local` / `.env.compose.local` / `reference/` / private key；模板仅有 `CHANGE_ME`。
- high-confidence Secret Scan 仅命中明确的测试构造值，无真实 JWT、Authorization bearer、DSN、数据库密码、E2E 密码或 token。
- README/docs 无本机绝对路径；Web/Admin runtime 无 `v-html`、`innerHTML`、`outerHTML` 或 `insertAdjacentHTML`。
- Gateway 在 Auth 前删除客户端 `User-ID`、`Role`、`ValidPath`，只注入 Auth 权威身份；伪造 Admin role 回归包含在 full suite。
- Admin Users/Chat/Group/File/Settings/Logs 目标 handler 均执行服务端管理员校验。
- WS query token 经 Gateway/Auth 后，Chat/Group handler 在 Upgrade 前校验内部身份和 Origin。
- File 使用 `MaxBytesReader`、流式处理、服务端 MIME 探测、ownership、storage root 路径约束；公开 DTO 不含物理路径。
- Audit 字段白名单和 privacy tests 不记录密码、JWT、Authorization、Cookie、消息正文、文件内容、DSN 或 storage path。

## Data and workspace cleanup

- 发现 10 个历史 `gim_cp4_*` / `gim_cp6_file_*` E2E 账号及 2 个 CP4 群；确认来自仓库固定测试前缀后，在单事务内删除其消息、会话、成员、文件引用、审计、用户配置和账号，复核全部为 0。
- 第一个 cleanup SQL 因 MySQL temporary table reopen 限制在执行删除前失败并回滚；改为单次 JOIN 后事务 PASS。
- `perf-user-*`、`final-admin-*`、`perf-group-*`、perf Chat/Group messages、perf UserFile references：0。
- 主库 migration：8 applied / 0 dirty。
- 删除可重建的 `.tmp/`、`server/tmp/`、`web/dist/`、`admin/dist/`；无 tracked temp/coverage/log/binary/IDE artifact。
- `.local-data/` 与 `server/storage/` 可能包含真实开发数据，仅确认处于 ignore，未盲目删除。
- V1 不实现物理孤儿对象 GC；已删除测试 File metadata/reference，但 named volume 内无引用内容对象按已知限制保留。

## Documentation acceptance

- README 已包含 Features、Architecture、Tech Stack、Reliability、Security、Performance、Quick Start、Testing、Repository Structure、Known Limitations、Roadmap 与 Reference/Independence。
- 架构文档覆盖 Auth/Gateway、双 WS、ACK/幂等/session/unread、Presence、File、Audit 和关键 trade-off。
- 面试包包含 60 秒介绍、demo 路径、架构/可靠性/性能问答、简历 bullets、禁止夸大项和真实缺口。
- 检查范围内相对 Markdown 链接均可解析；README/docs 无过期的 race-pending 标记。

## Compose shutdown

- 停止前 16 个常驻服务全部 healthy。
- 执行 `docker compose down`，未使用 `-v`。
- 容器和 network 已移除；`gim_mysql_data`、`gim_redis_data`、`gim_etcd_data`、`gim_kafka_data`、`gim_file_storage` 五个 volume 全部保留。

## Database / API / WS changes

- Database schema：无变化。
- HTTP API：无变化。
- WebSocket protocol：无变化。
- 数据变化仅为精确清理明确测试前缀数据；未删除普通开发用户数据。

## Race / CI

- Windows Native：NOT AVAILABLE，原因是当前 32-bit MinGW 与 windows/amd64 race 不兼容。
- Linux Docker：full / focused / real-dependency integration 全部 PASS。
- Project Race Verification：PASS in compatible Linux environment。
- GitHub Actions workflow 已存在；remote result 在收口环境中 NOT OBSERVED，local equivalent PASS。该项按用户约定不阻塞 Final Acceptance。

## Remaining limitations

- Chat/Group 仅文本；无 receiver delivery/read receipt；pending 不跨刷新；File 未接入消息协议。
- 用户端好友管理 HTTP/UI 闭环未完成，Private smoke 使用对称好友 fixture。
- File 物理孤儿对象延迟 GC；浏览器下载物化 Blob。
- Audit 为 bounded best-effort，无 outbox、DLQ、disk spool 或 exactly-once。
- 高压重启存在 DB commit 与 ACK/live publish 窗口。
- single-host/single-node，无 HA、Kubernetes、内部 TLS、生产 Secret Manager。
- Admin chunk warning；remote CI NOT OBSERVED。

## Reference source statement

本操作未从 `reference/` 复制或迁移代码。最终材料仅说明此前参考过 FIM 三仓库的业务行为、接口语义、页面流程、数据关系与问题场景；GIM 的实现、测试、可靠性改进和文档均在独立目录中自主完成，且不依赖参考仓库参与 import、构建或运行。

## Freeze

- `Day 4 / Checkpoint 5 = COMPLETED`。
- `Day 4 = COMPLETED`。
- `GIM V1 = COMPLETED`。
- Final commit / push：本记录所在提交完成后复核 `HEAD == origin/main` 与 clean。
- 下一阶段：无授权工作；到此停止。
