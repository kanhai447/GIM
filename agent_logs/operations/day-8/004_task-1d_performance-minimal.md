# 004 / Task 1D — 性能基线最小场景复跑

- **日期**：2026-11-12（Asia/Shanghai）
- **前置**：Compose 全栈 healthy；使用 `server/tools/loadtest`，全部请求经 Gateway。
- **结果**：HTTP profile 与 Private WS 两个最小场景 **PASS**，数据已清理。

## 环境与方法

```text
Host：Windows，Docker Desktop 29.4.0（Linux containers）
Stack：docker compose 全栈（17 services）
工具：go run ./tools/loadtest（prepare / http / private / cleanup）
鉴权：GIM_PERF_PASSWORD（运行时随机，未输出）
DB：GIM_PERF_MYSQL_DSN（Compose MySQL 宿主端口 13306，未输出）
Manifest：K:\it\gim\.tmp\perf\manifest.json（ignored）
```

## 场景与真实结果

### HTTP `profile`（并发 10，10 秒）

```json
{
  "workload": "http.profile",
  "attempts": 27281, "successes": 27281, "failures": 0,
  "timeouts": 0, "protocolErrors": 0, "businessErrors": 0,
  "throughputPerSecond": 2728.08,
  "p50Ms": 3.642, "p95Ms": 4.834, "p99Ms": 5.758,
  "gatewayPath": true, "maximumTestedNotCapacity": true
}
```

### Private WebSocket（10 connections，10 秒，目标 10 msg/s）

```json
{
  "workload": "private.websocket",
  "ack": { "attempts": 100, "successes": 100, "failures": 0,
           "p50Ms": 11.511, "p95Ms": 22.859, "p99Ms": 44.779 },
  "connect": { "attempts": 10, "successes": 10, "failures": 0 },
  "sentUnique": 100, "persistedUnique": 100,
  "deliveryExpected": 100, "deliveryObserved": 100,
  "duplicateRetries": 5, "duplicateAcks": 5,
  "reconnects": 0, "unexpectedDrops": 0,
  "correctnessPass": true, "deliveryCorrect": true, "unreadDeltaCorrect": true,
  "gatewayPath": true, "maximumTestedNotCapacity": true
}
```

### 清理

```json
{ "operation": "cleanup", "prefix": "perf-user-20261112-212155",
  "usersRemoved": 20, "groupsRemoved": 3,
  "physicalFileGC": "not implemented by V1; ..." }
```

## 结论与边界

- 本机单机 Compose 环境下两个最小场景 0 failure / 0 timeout / 0 protocol/business error，正确性校验通过。
- 这是**当前环境的一次复跑**，不等同于 `docs/performance/baseline.md` 的历史完整档位；不构成容量承诺。
- 未复跑：50 connections / 200 connections 阶梯、5 分钟 soak、restart 稳定性、文件吞吐档位。

## 问题与修复

- 首轮 HTTP 负载因 prepare 与 workload 使用了不同随机 `GIM_PERF_PASSWORD` 而登录失败（`code=1201`）。修正为同一 shell 内使用同一密码重新 prepare 后 PASS；旧 manifest 已先 cleanup。

## 验收

- 性能最小场景：**PASS**（限上述两个档位）。
