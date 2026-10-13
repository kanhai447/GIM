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
