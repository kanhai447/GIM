# 003 / Task 1C — Kafka 真实消费验证

- **日期**：2026-11-12（Asia/Shanghai）
- **前置**：Compose 全栈已启动，`kafka` service healthy（`apache/kafka:4.2.2` KRaft，宿主机映射 `127.0.0.1:19092`）。
- **结果**：Kafka 两个集成用例 **PASS**（含真实 produce → consume → MySQL 落库 → 查询 → consumer restart → event_id 幂等）。

## 环境

使用 `.tmp/env.compose-host`（由 `.env.local` 派生，仅替换为 Compose 宿主端口）：

```text
MYSQL_HOST=127.0.0.1  MYSQL_PORT=13306
REDIS_HOST=127.0.0.1  REDIS_PORT=16379
ETCD_ENDPOINTS=http://127.0.0.1:12379
KAFKA_BROKERS=127.0.0.1:19092
```

该文件含本机开发凭据，位于 Git ignored 的 `.tmp/`，本轮结束删除，不提交、不输出。

## 命令与真实结果

```powershell
$env:GIM_ENV_FILE='K:\it\gim\.tmp\env.compose-host'
$env:GIM_KAFKA_INTEGRATION='1'
go test ./tests -run 'TestKafkaAudit' -count=1 -v
```

```text
=== RUN   TestKafkaAuditProduceConsumePersistQueryRestartAndIdempotency
--- PASS: TestKafkaAuditProduceConsumePersistQueryRestartAndIdempotency (1.59s)
=== RUN   TestKafkaAuditUnavailableBrokerKeepsPublishBounded
--- PASS: TestKafkaAuditUnavailableBrokerKeepsPublishBounded (0.01s)
PASS
ok  github.com/kanhai447/GIM/server/tests  2.116s
```

测试内部校验：producer → Kafka → consumer → MySQL `audit_logs` 写入、Admin 查询过滤、consumer 重启后继续消费、`event_id` 重复投递幂等；以及 broker 不可用时 publish 有界退化。

## 问题与修复

- 无。首轮 Compose 启动后 Kafka healthcheck 已通过，直接执行测试即 PASS。

## 验收

- Kafka 真实消费：**PASS**。

## 日志

- 本文件；未修改仓库代码。
