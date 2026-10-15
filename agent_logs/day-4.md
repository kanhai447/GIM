# Day 4

> 由 Agent 在 Day 4 执行过程中持续更新。

## 2026-10-13 Checkpoint 1 — Kafka Structured Logs + Audit Log Foundation

### Scope

- 完成结构化 Event schema/UUID、safe metadata、trusted request ID、统一 bounded Kafka producer、Logs consumer/MySQL repository/service/API 与 graceful lifecycle。
- Auth/User registration boundary、login/logout、Private Chat、Group lifecycle/message、File upload/delete 接入 post-commit best-effort audit；Kafka 故障不改变核心业务成功/ACK。
- 新增 007 audit_logs migration、event ID unique 幂等、time/user/type/service/request indexes 与 admin-only cursor/filter API。
- 未进入 Admin frontend、Docker Compose、performance、CI 或 Day 4 后续 Checkpoint。

### Acceptance

- 本机 Java/KRaft Kafka 4.2.2 真实 pipeline 连续 3 轮 PASS：produce→consume→persist→query、duplicate one row、consumer stop/restart recovery、Auth/Group/File schema events。
- unavailable broker non-blocking/bounded degradation PASS；business hook tests证明 Chat/Group ACK/delivery 与 File 结果不受 audit failure 影响。
- MySQL 001–007 EMPTY/UP/DOWN/RE-UP、audit schema/constraint/idempotency、5 类 EXPLAIN PASS。
- full Go suite、vet、gofmt、diff、Secret Scan、Gateway/Auth/Private Chat/Group/File regressions PASS。
- focused coverage：audit 47.7%、kafkaaudit 39.1%、consumer 64.8%、logs service 45.0%、logs HTTP 42.4%。
- Race：PENDING compatible environment。

### Architecture / Limitations

- topic `gim.audit.events`；business boundary best-effort，consumer at-least-once + DB unique，不宣称 exactly-once。
- metadata allowlist 禁止 password/JWT/body/message content/file content/storage path/credentials；Log Service 内部错误不递归 Kafka。
- 无 transactional outbox/DLQ/retention/Admin UI；broker 长时故障或 queue full 可能 audit loss。

### Git / Next

- implementation commit：`2ac7199ad6ee600460342c1c3cb801b893a2360e feat(logs): add kafka audit pipeline`。
- log/status commit：本记录所在提交，随后 push 并复核 equality/clean。
- Next Planned：Admin Integration，需新授权；Checkpoint 1 后严格停止。

## 2026-10-14 Checkpoint 2 — Admin Final Integration

### Scope

- 完成独立 Admin Vue 应用与 User / Chat / Group / File / Settings / Logs 正式服务集成；Dashboard 复用 Settings/Admin runtime 提供真实只读聚合，不新增 Mock data service。
- Admin API 保持 Gateway/Auth 身份覆盖及目标服务 `Role == 1` 双重授权；用户状态使用正式 `1=Active`、`2=Disabled`。
- 完成 Users Disable/Enable、Private Chat tombstone/session repair、Group message/dissolve、File metadata/download/reference delete、typed Settings、safe Logs list/detail/filter。
- 完成 Edge 100% zoom Users flex/table containment 修复、Login Arco 控件注册修复及 User status action 同步修复。

### Acceptance

- `Real Browser E2E: MANUAL USER-EXECUTED — PASS`；真实 Microsoft Edge 覆盖 Login、Dashboard、Users、Chat、Group、File、Settings、Logs、Logout 与 normal-user rejection。
- Computer Use blocked by browser URL verification safety layer, not bypassed；未声明 Automated Browser E2E PASS。
- Admin type-check、11 files / 25 tests、build PASS；Web type-check、16 files / 90 tests、build PASS。
- clean environment `go test ./... -count=1`、vet、gofmt、diff、Chat/Group/File regressions PASS。
- Migration 008 EMPTY/UP/DOWN/RE-UP 与 Admin query EXPLAIN PASS；全新 Kafka pipeline 连续 3 轮 PASS。
- no-token、normal-user、forged numeric/string role、expired/revoked token 均 REJECT；valid admin ALLOW。
- Secret Scan、XSS/privacy、physical storage path exclusion、reference independence PASS。
- `Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`；`Race Verification Pending`。

### Architecture / Limitations

- Admin 只经 Gateway 调用真实 API；认证链仍为 `Gateway -> Auth -> Business Service`，业务服务端角色校验是最终安全边界。
- audit 为 post-commit best-effort；无 transactional outbox、DLQ、retention 或 exactly-once；overview 当前为请求时计数。
- Vite build 保留非阻塞 chunk-size warning；兼容 64-bit CGO 环境/CI 仍需 race 验证。

### Git / Freeze

- Backend/Admin/API/test：`e18e562 feat(admin): add management services and audit integration`。
- Admin frontend/fixes：`08a4f2e feat(admin): integrate management console and fixes`。
- Log/status：本记录所在提交，随后 push 并复核 `HEAD == origin/main` / clean。
- `ADMIN V1 = FUNCTIONAL`；`Day 4 / Checkpoint 2 = COMPLETED`。
- Next Authorized Work：NONE。Next Planned：Docker Compose Full Stack Deployment；未进入下一 Checkpoint。

## 2026-10-15 Checkpoint 3 — Docker Compose Full Stack Deployment

### Scope

- 新增单一 `compose.yaml`，运行 MySQL、Redis、etcd、Kafka KRaft、migration、User/Group RPC、Auth/User/Chat/Group/File/Settings/Logs API、Gateway、Web 与 Admin。
- Go 共享多阶段镜像；Web/Admin production build 由非特权 Nginx 提供，并通过同源 `/api` 代理 Gateway/WS。
- 新增容器环境适配、minimal health/readiness、File named volume、一次性正式 Admin bootstrap 与部署文档；没有新增业务能力或改变 Chat/Group/File 协议。

### Acceptance

- clean `down -v -> up`、persistent `down -> up`、migration 001–008 idempotency、File restart/down persistence、Chat/Group/File/Logs restart、etcd lease replacement/cleanup、MySQL recovery：PASS。
- Compose API/Auth、Private Chat、Group Chat、File、Admin focused real-browser/API smoke：PASS。
- Kafka 正常链路 `business event -> consumer -> MySQL -> Admin Logs API` PASS；Kafka 停止时核心业务继续成功，恢复后新 audit 继续消费。
- 最终 long-running containers 全部 healthy；migration/storage-init exited 0；Gateway/Web/Admin HTTP 200。
- backend full test/vet/gofmt、Web 16/90、Admin 11/25、两端 production build、Compose config/build、核心 no-cache build、Secret Scan/reference independence：PASS。
- `Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`；`Race Verification Pending`。

### Architecture / Limitations

- 单 bridge network；MySQL/Redis/etcd/Kafka/File 使用 named volumes；内部依赖走 Docker DNS，业务 API 路由继续使用 etcd discovery。
- `.env.compose.local` ignored/runtime-only；模板只含 placeholder；Admin bootstrap 不自动运行或重置密码。
- development/single-host only；基础设施单节点，无 HA、内部 TLS、production secret manager、resource hard limits 或 Kubernetes。
- audit 仍为 best-effort；Private Chat smoke 因当前无 friend-management HTTP 流程使用最小对称数据库 fixture。

### Git / Freeze

- implementation commit：`36e694e chore(deploy): add full stack docker compose`。
- docs/status commit：本记录所在提交，随后 push 并复核 `HEAD == origin/main` / clean。
- `DOCKER COMPOSE = FUNCTIONAL`；Next Authorized Work：NONE。
- Next Planned：Performance / Stability / CI Race；严格未进入下一 Checkpoint。

## 2026-10-15 Checkpoint 4 — Performance / Stability / Race Verification

### Scope

- 新增单一可复现 Go load harness，覆盖 Gateway HTTP、Private/Group authenticated WS、File streaming/dedup、fixture correctness 与 prefix cleanup。
- 新增 loopback-only opt-in runtime metrics/pprof、资源快照脚本、Linux Docker full/focused/integration race runner 与 GitHub Actions backend/frontend/race workflow。
- 基于 soak 证据只修复 Private Chat MySQL 1213 deadlock：最多 3 次重建 transaction，10/20ms context-aware backoff；未移除事务、ACK、unread、鉴权或 audit。

### Acceptance

- HTTP login/profile/session、200 Private WS、100 connections 50 msg/s、3/5/10-member Group、2-device fanout、1/10 MiB File 与 concurrent dedup：correctness + latency/throughput baseline PASS。
- Kafka normal/degraded、修复后完整 5-minute Chat/Group soak、Chat/Group controlled restart 与资源观测 PASS；最终 soak 为 Private 1500/1500 ACK/rows、2100/2100 delivery，Group 600/600 ACK/rows、6000/6000 delivery，均无错误、重连或 drop；高压 restart 的 persisted-before-ACK/live-delivery gap 如实列为 V1 限制。
- Linux Docker focused/full unit/real-dependency integration race 均 PASS，无 race warning；Windows native race 仍因 32-bit MinGW 不可用，但项目 Race Verification 已在兼容 Linux 环境解除 PENDING。GitHub Actions 已 push，remote result 因当前无可用 Actions/API 访问而 NOT OBSERVED，local equivalent PASS。
- Backend full tests/vet/gofmt、Web 16/90 + type-check/build、Admin 11/25 + type-check/build、Secret Scan/repository safety/diff PASS。
- perf users/groups/messages/file references 及两个已确认孤立物理对象已清理；16 个长期 Compose 服务最终保持 healthy，无 OOM/restart。

### Baseline / Limits

- Maximum tested：200 concurrent Private WS；100 connections、50 msg/s 时 1000/1000 ACK，ACK p95=13.47ms。该数字不是最大容量声明。
- 修复后最终完整 5 分钟 Private 1500/1500 rows/ACK、2100/2100 delivery，Group 600/600 rows/ACK、6000/6000 delivery；未发现 goroutine/memory/connection runaway。
- V1 audit 是 bounded best-effort；服务进程终止存在 DB commit 与 ACK/live publish 间窗口；无 outbox、receiver delivery ACK 或 exactly-once。

### Git / Freeze

- implementation/perf/CI：`69955ce test(perf): add reproducible stability and race verification`。
- docs/status：本记录所在提交，随后 push 并复核 equality/clean。
- `Performance Baseline = COMPLETED`；`Stability = PASS`；`Race = PASS in compatible Linux environment`。
- Next Authorized Work：NONE。Next Planned：Day 4 Final Acceptance + Documentation / Interview Package；严格停止。

## 2026-10-15 Checkpoint 5 — Final Acceptance / Documentation / Interview Package

### Scope

- 不新增业务能力；对 Backend、Web、Admin、Compose smoke、001–008 migration、安全边界、测试数据和仓库卫生执行最终复核。
- 重构根 README；新增 architecture/reliability/file-audit/decisions 与 interview package；修正 deployment 中过期的 load/race 描述。
- 清理明确 E2E/perf 临时数据和可重建工作区产物，最后以 `docker compose down` 保留 volumes 收口。

### Acceptance

- Backend `go test ./... -count=1`、`go vet ./...`、gofmt、diff：PASS。
- Web type-check、16 files / 90 tests、build：PASS；Admin type-check、11 files / 25 tests、build：PASS（仅已知 chunk warning）。
- Compose focused smoke：Login、Private、Group、File、Admin Logs 全部 PASS；临时用户、群、消息、File 引用和 Admin 已清理。
- 隔离空库 migration 001–008 UP/repeated UP/DOWN/dirty protection/RE-UP、schema/constraints/EXPLAIN/User-Auth regression：PASS；主库 8 applied / 0 dirty。
- Secret Scan、repository safety、identity header、Admin second authorization、WS pre-upgrade auth、File path/MIME/size/ownership/privacy、unsafe HTML：PASS。
- 10 个明确历史 E2E 账号与 2 个测试群及关联数据已事务清理；普通开发数据未触碰。
- 文档相对链接、最终状态措辞、FIM independence：PASS。

### Shutdown / Freeze

- 停止前 16 个长期服务全部 healthy。
- 执行 `docker compose down`（未加 `-v`）；五个 GIM named volumes 已确认保留。
- Windows Native Race：NOT AVAILABLE（32-bit MinGW）；Linux Docker Race：PASS；Project Race Verification：PASS in compatible Linux environment。
- GitHub Actions remote：NOT OBSERVED；local equivalent PASS，不阻塞最终验收。
- `Day 4 / Checkpoint 5 = COMPLETED`。
- `DAY 4 = COMPLETED`。
- `GIM V1 = COMPLETED`。
- Next Authorized Work：NONE；V1 冻结后 STOP。
