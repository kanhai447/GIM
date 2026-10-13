# Day 4 / Checkpoint 1 — Kafka Structured Logs + Audit Log Foundation

- **时间:** 2026-10-13 +08:00
- **开始状态:** `main`，`HEAD == origin/main == 386856e554dc6194f7fddc1f013049e5cfd06c05`，working tree clean。
- **授权范围:** Kafka 异步结构化审计、Logs consumer/MySQL/API 与 Auth/Chat/Group/File hooks；未进入 Admin frontend、Docker Compose、performance、CI 或 Day 4 后续 Checkpoint。

## Architecture

```text
committed business operation
 -> audit.Publisher abstraction
 -> process-shared bounded kafkaaudit.Producer
 -> gim.audit.events
 -> Logs Consumer
 -> audit_logs
 -> admin-only Logs HTTP API
```

- 业务服务不直接调用 Kafka；统一 producer 是进程级单例，一个 worker 串行处理有界 queue，不存在 shared mutable event/buffer 或逐事件 goroutine。
- Logs Service 按 Domain -> Repository -> Consumer -> Service -> HTTP Transport 分层；`cmd/logs-api` 负责 wiring、etcd registration 和生命周期，不承载 SQL/consumer 业务实现。
- Kafka 只承载审计/业务日志，不替代业务 DB、Chat/Group realtime、Presence 或 Auth 状态。

## Event Schema

- schema v1 字段：`schema_version,event_id,event_type,service,action,user_id,role,resource_type,resource_id,request_id,trace_id,status,error_code,ip,method,path,occurred_at,metadata`。
- `event_id` 使用 UUID；`occurred_at` 统一 UTC；producer key 优先 `user_id`，其次 resource，最后 event type。
- 固定事件类别：user registered；auth login success/failed/logout；private/group message accepted；group created/member joined/member kicked/dissolved；file uploaded/user file deleted。
- Gateway 清除外部 `X-Request-ID` 并生成可信 UUID，写入 downstream request 和 response。本轮无可信 proxy allowlist，故不读取可伪造的 forwarded IP，IP 字段保持空。

## Sensitive Data Policy

- metadata 使用显式 key allowlist、JSON scalar-only、8 KiB cap；Event validation 再校验 schema/type/status/role/长度/metadata。
- 禁止 password/hash、JWT/refresh token、Authorization/Cookie、OAuth/DB/Redis Secret、DSN/private key、request body、完整 user/config、聊天正文、文件正文和 storage path。
- Auth 失败不记录 account，仅记录安全的 failure category；Chat/Group 只记录 message/sender/receiver-or-group/type/clientMsgId；File 只记录 UserFile/public object UID/size/server MIME/dedup flags。
- malformed Kafka payload 的普通日志只包含 partition/offset；Log Service 自身错误不回发 Kafka，避免递归。

## Kafka Failure Semantics

- `segmentio/kafka-go v0.4.51` 单一 client；topic 为 `gim.audit.events`，consumer group 默认 `gim-logs-v1`。
- producer queue 默认 1024；enqueue non-blocking；queue full 明确 drop，Kafka write 每次默认 750 ms、最多 3 次、100 ms 退避。
- 业务 DB 与 Kafka 是非原子 best-effort boundary；Kafka 长时间不可用可能丢 audit，但不改变已经成功的 HTTP、ACK、realtime 或 File 结果。
- 无 transactional outbox，不宣称 exactly-once。Kafka/consumer 侧采用 at-least-once，DB event ID unique 收敛重复。
- shutdown 停止接收、在 deadline 内 drain/cancel/close；consumer 停止 fetch、完成已持久化事件 commit 后关闭 reader；DB/etcd 注册按现有 lifecycle 关闭。

## Consumer Idempotency

- `FetchMessage -> JSON decode/validate -> Repository.Insert -> CommitMessages`；DB 成功或 duplicate event ID 被安全解析后才提交 offset。
- DB、fetch、commit 暂时失败均退避重试；DB 失败时不提交。malformed poison payload 安全丢弃并提交，避免永久 crash loop。
- `audit_logs.event_id UNIQUE` 是最终幂等屏障；真实 Kafka 三轮均验证 duplicate publish 最终只存一行。
- Consumer 停机后 producer 写入事件，再以同 group 重启，事件被恢复消费并持久化。

## Authorization

- Logs 服务同时要求有效可信 `User-ID` 和 `Role == 1`；普通用户、仅伪造 role 而无身份均 403。
- Gateway 对 `/api/logs/*` 仍执行 Auth，并覆盖外部 `User-ID/Role/X-Request-ID`。测试证明普通用户伪造 `Role: 1` 后 downstream 仍为真实 role 2。
- 正式部署 Logs API 继续仅在内部网络暴露；前端路由/按钮不构成安全边界。

## Business Integration

- **Auth/User registration boundary:** register success owner 为 Auth，产生唯一 `user.registered`，User 不重复产生同义事件；login success/failure、logout 已接入。
- **Private Chat:** 新消息事务 commit、ACK/realtime 路径后且 `createdNew=true` 才 emit；duplicate clientMsgId 不重复记录；Kafka failure 不改变 ACK。
- **Group:** create、join/invite/approve、kick、dissolve 与新 group message 已接入；message audit 不含正文；Kafka failure 不改变 ACK/delivery。
- **File:** upload 物理/DB流程成功后与 UserFile delete 成功后 emit；Kafka failure 不回滚引用或物理对象流程。
- **Gateway:** 只增加可信 request ID propagation，不把全量 access log/body Kafka 化。

## Logs API

- `GET /api/logs/logs` 支持 `beforeId/limit/from/to/service/eventType/userId/status/requestId`，默认 50、最大 100，稳定 `occurred_at DESC,id DESC` cursor。
- `GET /api/logs/logs/:id` 返回单条已持久化安全字段。非法 filter/cursor/limit 返回 2601；非管理员返回 2603；不存在详情返回 2604。
- Checkpoint 1 不提供 delete/retention 接口，避免沿用参考项目的无条件删除语义；Admin frontend 留待下一 Checkpoint。

## Migration / EXPLAIN

- 新增 `007_audit_logs.up.sql/down.sql`，未修改 001–006。
- 表包含 JSON metadata、CHECK 约束、`event_id UNIQUE` 和 time/user/type/service/request 查询索引。
- 真实隔离 MySQL EMPTY/UP/DOWN/RE-UP PASS；schema/constraint/duplicate event ID PASS。
- EXPLAIN 分别命中 `ix_audit_logs_occurred`、`ix_audit_logs_user_occurred`、`ix_audit_logs_type_occurred`、`ix_audit_logs_service_occurred`、`ix_audit_logs_request`。

## Kafka Integration / Stability

- Docker content store 在三种 Kafka image 上出现 layer size validation，未生成容器；随后使用本机 Java 24 + Maven 官方 Kafka 4.2.2 runtime 启动临时单节点 KRaft broker。未新增 Compose 文件或项目容器化。
- 真实 broker + real producer/consumer + real MySQL 连续 **3 轮 PASS**；每轮内部再覆盖 3 组 produce/duplicate/persist/query，并覆盖 Auth/Group/File schema event。
- consumer stop -> publish -> same-group restart -> recovery PASS；unavailable TCP broker 的 business-side Publish 非阻塞且 bounded close/drop PASS。
- 临时 Kafka/etcd 进程与本轮创建的 `.tmp` runtime/data/log 已停止并删除。

## Regression

- Private Chat message/ACK/delivery、session/unread/read/history、50 duplicate/50 distinct 与 rollback：PASS。
- Group send/ACK/delivery、session/unread/read/history、multi-instance/multi-device、Redis degraded：PASS。
- File stream upload/dedup/ownership/download/delete、Gateway/Auth/etcd：PASS。
- Gateway/Auth/User chain、Chat/Group WS tunnel、identity/request-ID spoof protection：PASS。
- 首次 full suite 因本机 etcd 未启动而只在 integration package 失败；启动现有本地 etcd 后相同全量 suite PASS，属于测试环境依赖而非代码失败。

## Verification

- `go test ./... -count=1` with real local config: PASS。
- Kafka focused real integration `-count=3`: PASS。
- MySQL migration/repository integration: PASS。
- explicit Chat/Group/File/Gateway regression set: PASS。
- `go vet ./...`: PASS；repository-wide gofmt check: PASS；`git diff --check`: PASS。
- focused coverage：audit 47.7%、kafkaaudit 39.1%、consumer 64.8%、logs service 45.0%、logs HTTP 42.4%；MySQL repository 由 real integration 覆盖。
- repository safety script PASS；changed-file high-risk secret scan 只有 `.env.example` placeholder 和代码 identifier false positive，经逐项复核 PASS。

## Reference Use / Independent Implementation

- **FIM referenced:** Kafka/Logs 的业务目的、Admin logs list/detail 行为与既有服务边界。
- **GIM independently implemented/improved:** versioned Event schema、metadata allowlist、bounded producer、best-effort degradation、post-persist commit、event ID idempotency、cursor/filter API、server-side role verification、trusted request ID、privacy tests 与 migration/index 均在 GIM 自主编写；未复制 reference 文件或共享 mutable Pusher。

## Race

- **PENDING compatible environment。** 未重复运行已知不可用的 Windows race；当前仅有 32-bit MinGW，既有错误为 `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。Day 4 后续 CI Checkpoint 必须执行 producer/consumer 与既有 Chat/Group race。

## Known Limitations

- 无 transactional outbox；business DB commit 与 audit Kafka publish 不原子，broker 长时间不可用或 queue 满可能丢审计事件。
- 暂无 DLQ；malformed event 被安全提交丢弃并仅记录 partition/offset。
- 暂无 audit retention/archive/delete policy、Admin Logs frontend、metrics/alert dashboard。
- 一个 topic 不提供全局业务顺序；partition key 只提供同 key 局部顺序倾向。
- IP 在未建立可信 proxy allowlist 前不采集；TraceID 预留但未引入 OpenTelemetry。

## Files / Database / Protocol

- 新增 `server/internal/platform/audit/**`、`server/internal/platform/kafkaaudit/**`、`server/internal/logs/**`、`server/cmd/logs-api`、007 migration 与 audit integration tests。
- 修改 Auth/Chat/Group/File/Gateway wiring/hooks、`.env.example`、Go module 与 backend/database/API/core-flow/config docs。
- Database：新增 `audit_logs`。HTTP：新增正式 Logs list/detail query contract。WS envelope/ACK semantics、Redis key 与前端：无变化。

## Git / Freeze

- Implementation commit：`2ac7199ad6ee600460342c1c3cb801b893a2360e feat(logs): add kafka audit pipeline`。
- Log/status commit：`88afb79d27a3a1e20962c90f3ed1b41acc2db15b docs(day4): record kafka audit checkpoint`。
- 最终状态措辞校正随其后的 state-only commit 推送；该提交不修改实现或验收结论。
- Push 后执行最终 `HEAD == origin/main` / clean 复核。
- Next 条件：具备进入 **Admin Integration** 的后端 API/数据基础，但必须等待新授权；本轮严格停止于 Checkpoint 1。
