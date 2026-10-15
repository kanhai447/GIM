# GIM V1 Performance / Stability Baseline

> 这是本地单机基线，不是容量承诺。所有性能数字均来自 2026-10-15 的 Windows + Docker Desktop 隔离环境，待测实现提交为 `69955ce3e7caa032db60aa85bf88588e50e703f9`。

## Environment and method

- Host：13th Gen Intel Core i9-13980HX，24 cores / 32 logical processors，15.63 GiB RAM。
- Docker：Docker Desktop 4.69.0，Linux amd64 engine 29.4.0，32 vCPU、7.57 GiB memory。
- Deployment：GIM 完整 Compose 栈，Gateway、Auth、业务服务、MySQL、Redis、etcd、Kafka、Web、Admin 均运行；所有用户路径经 `Client -> Gateway -> Auth -> target service`。
- Harness：`server/tools/loadtest`，单一 Go 工具，支持 `prepare / cleanup / http / private / group / file`；参数化 base URL、并发、连接数、持续时间、消息速率、群规模、文件大小与设备数。
- Correctness：每档同时核验 ACK、`clientMsgId` 幂等、数据库行、session/unread、fanout 或文件 hash/object/reference；不是只采集 QPS。
- Resource：`scripts/perf-snapshot.ps1` 采集容器 CPU/RAM/PID、restart/OOM、Go goroutine/heap、DB pool、Redis clients、Kafka queue/drop/lag。
- Runtime secrets：仅从 `GIM_PERF_PASSWORD` 与 `GIM_PERF_MYSQL_DSN` 读取；输出和本文不包含密码、JWT、DSN、消息正文或物理 storage path。

本轮采用渐进负载，并在吞吐回落、tail latency 上升后停止扩大。以下 `maximum tested` 仅表示此机器上已验证的档位，不表示系统最大容量。

## HTTP

每档 8 秒，全部 0 failure、0 timeout、0 protocol/business error。

| workload | concurrency | throughput req/s | p50 ms | p95 ms | p99 ms |
|---|---:|---:|---:|---:|---:|
| bcrypt login | 1 / 5 / 10 / 25 | 5.62 / 25.62 / 46.25 / 99.98 | 173.43 / 190.94 / 213.90 / 246.11 | 180.60 / 209.74 / 226.15 / 254.92 | 184.22 / 212.53 / 233.29 / 298.01 |
| authenticated profile | 1 / 10 / 25 / 50 | 501.73 / 2772.67 / 2622.47 / 2157.69 | 1.95 / 3.61 / 8.98 / 19.62 | 2.71 / 4.86 / 15.43 / 52.19 | 3.44 / 5.79 / 18.96 / 77.51 |
| session list | 1 / 10 / 25 / 50 | 422.49 / 688.84 / 508.91 / 498.35 | 2.22 / 7.43 / 49.04 / 89.87 | 3.35 / 49.02 / 102.95 / 123.86 | 4.28 / 59.43 / 116.68 / 141.56 |

Profile 与 session list 都在 10 并发后出现吞吐回落与尾延迟增加；数据库 pool `WaitCount=0`，所以没有为漂亮数字盲调连接池。Login 是独立的 bcrypt CPU-heavy 路径，不能与轻量 API QPS 混读。

## Private WebSocket

Authenticated Gateway WS connection 阶梯为 25 / 50 / 100 / 200，全部连接成功，无意外断开；200 connections 的 connect p99 为 5.43 ms。`Maximum tested: 200 concurrent private WS connections; not claimed maximum capacity.`

100 connections 消息负载：

| rate / duration | attempts | persisted ACK | delivery | ACK p50/p95/p99 | errors |
|---|---:|---:|---:|---:|---:|
| 20 msg/s / 20s | 400 | 400 | 1040 / 1040 | 10.02 / 13.84 / 18.10 ms | 0 |
| 50 msg/s / 20s | 1000 | 1000 | 2600 / 2600 | 9.10 / 13.47 / 18.02 ms | 0 |

每档插入同 `clientMsgId` retry；ACK 返回原 message ID，数据库、unread 与 realtime 均未重复。

## Group WebSocket

单设备、10 msg/s、15 秒：3 / 5 / 10 members 均收到 150 ACK，fanout 分别为 450 / 750 / 1500，全部完整。5 members × 2 devices、20 msg/s、15 秒时为 300 / 300 ACK、3000 / 3000 delivery；duplicate retry、session、unread 均正确。

稳态 ACK p50 约 11 ms、p95 约 14–15 ms、p99 约 19–21 ms。首次 3-member 启动档出现一次 p95=2.10s / p99=2.55s 离群点，无错误、丢失或积压，后续档未复现，仍作为已知启动抖动保留。

## File

上传生成和服务端处理均为 streaming；所有档 0 failure / timeout，下载内容、hash、FileObject/UserFile 关系正确。

| payload / concurrency | upload MiB/s | upload p95 ms | download MiB/s |
|---|---:|---:|---:|
| 1 MiB / 1, 5, 10 | 39.33 / 63.78 / 60.75 | 25.43 / 78.39 / 164.10 | 141.20 / 198.29 / 182.72 |
| 10 MiB / 1, 5 | 122.00 / 211.75 | 81.97 / 236.13 | 150.15 / 263.10 |

相同文件名与内容并发 10 次全部成功，只形成 1 个 UserFile reference、1 个 FileObject、1 个 object UID；upload 118.22 MiB/s、p95 83.93 ms。10 MiB 后续档会复用已存在内容对象，因此这些是本机 loopback/content-addressed 路径数字，不外推为公网吞吐。

## Kafka audit

正常负载保持 audit 开启；测试后 producer queue=0、consumer lag=0。受控停止 Kafka 时，Chat 业务仍持久化、ACK、实时投递，producer queue 有界于 1024。

15 秒 outage 中 146 条业务消息全部成功，110 条 audit 在恢复后持久化，36 条超过有限重试边界被丢弃；第二次 10 秒 outage 的 100 条业务消息也全部成功。恢复后 queue=0、lag=0。V1 audit 明确是 bounded best-effort，没有 outbox/disk spool，不声明 exactly-once。

## Resources

- Idle：Go 服务约 9–12 MiB；MySQL 467 MiB；Kafka 451 MiB。Chat/Group/File/Logs goroutine 36/35/25/31，heap alloc 约 4.6–4.8 MiB。
- 阶梯负载后：Chat/Group/File/Logs heap 约 4.0–5.5 MiB，goroutine 48/38/36/31；各 DB pool `WaitCount=0`；MySQL connected/running=40/2；Redis connected/blocked=27/0；Kafka lag=0。
- Kafka audit burst 后曾瞬时约 158% CPU、803 MiB，静置后 CPU 与 backlog 回落；最终清理后快照所有 16 个长期服务运行且 healthy、无 OOM/restart，Gateway/Chat/Group/File/Logs 分别约 11.1/16.5/14.1/13.7/15.2 MiB，MySQL 624.6 MiB，Redis 17.1 MiB，Kafka 1.00 GiB。
- DB pool 已配置 max open 25、max idle 10、lifetime 30m；实测无 pool wait，因此未调整。

## Soak and restart

修复前 5 分钟并行 soak（Private 50 connections / 5 msg/s，Group 10 members / 2 msg/s）发现并定位了 Private MySQL 1213 deadlock；2 分钟同参修复验证先达到 600/600 ACK/rows。随后又完成一轮修复后的完整 5 分钟并行 soak：Private 1500/1500 ACK/rows、2100/2100 delivery，ACK p50/p95/p99=14.929/48.574/95.200 ms；Group 600/600 ACK/rows、6000/6000 delivery，ACK p50/p95/p99=13.026/40.439/94.958 ms。两者均 0 failure/timeout/protocol/business error、0 final-unacked retry、0 reconnect/drop，幂等与 unread 正确。

第一轮资源观测中 Chat goroutine 起点/中点均 143、Group 均 59，内存约 19–20 MiB，随 GC 波动。最终完整复验连接关闭后 Chat/Group goroutine 为 40/39、heap alloc 4.42/7.29 MiB，DB pool `WaitCount=0`，Redis blocked=0，Kafka queue/lag=0；16 个长期容器均 healthy，RestartCount=0、OOMKilled=false。

轻量 restart（10 connections、2 msg/s、45 秒）分别重启 Chat/Group：两者均 10/10 sockets 自动重连；原 `clientMsgId` 重试后 Chat 90/90 rows/ACK、90/90 delivery，Group 90/90 rows/ACK、900/900 delivery，无 duplicate storm。恢复消息端到端 p99 约 35.5s/29.5s。

更高的 50 connections / 10 msg/s Chat 探索档在进程终止窗口出现 37 个 2204 和 2 条“已提交但 ACK/live delivery 未完成”。history/session/unread 保留；V1 没有 receiver delivery ACK/outbox，因此不声称进程重启下实时 exactly-once。

## Measured bottleneck and fix

Private soak 的通用 2204 通过只记录安全类别的诊断稳定定位为 MySQL 1213 deadlock。修复前同参 2 分钟为 599 sends、7 次 2204、597 rows；修复仅对 1213 做最多 3 次事务重建重试，10/20 ms context-aware backoff，不吞其他错误。

修复后同参 2 分钟达到 600/600 ACK/rows、840/840 delivery，0 error / final retry / drop / reconnect，p50/p95/p99=10.59/25.82/233.66 ms。完整回归与 race 均通过。

## Race and CI

- Windows native race：NOT AVAILABLE；本机 32-bit MinGW 不支持 windows/amd64 race，未修改用户 GCC。
- Linux Docker focused high-risk race：PASS，覆盖 Chat、Group、File、Audit、Kafka producer/consumer、Logs、load harness。
- Linux Docker full unit race：`go test -race ./... -count=1` PASS，无 race warning。
- Linux Docker real-dependency integration race：应用层容器停止、保留 MySQL/Redis/etcd/Kafka，隔离临时配置执行 `go test -race ./tests -count=1`，PASS（24.628s）；全栈随后恢复。
- GitHub Actions workflow 已新增 backend test/vet/gofmt、Web/Admin type-check/test/build、full + focused Linux race。提交已 push，但宿主未安装 `gh`，当前网络工具也无法读取该仓库 Actions/API，因此 `CI remote result = NOT OBSERVED`；只声明 local equivalent PASS。CI 未运行真实外部依赖 integration race，本地 Docker 已覆盖该范围。

## Regression, cleanup, and limits

- Backend：full tests、vet、repository gofmt、diff check PASS。
- Web：type-check、16 files / 90 tests、build PASS。
- Admin：type-check、11 files / 25 tests、build PASS；仅保留既有 chunk-size warning。
- Repository safety 与 Secret Scan PASS。
- Cleanup：`perf-user-*`、`perf-group-*`、private/group perf messages、sessions/unread/file references 与两个已核实无引用的物理对象已清理；最终补强复验及三次准备失败留下的 `perf-user-final-*` 也按精确前缀清理，数据库核验 remaining=0。共享 audit topic 的历史事件按既有 retention 保留，不做危险的 topic truncate/Redis flush。

可信结论：在上述本机 Compose 环境，已验证 200 条并发 Private WS 连接；100 connections、50 msg/s 下 1000/1000 持久化 ACK，ACK p95 13.47 ms；修复后的完整 5 分钟 Chat/Group soak 2100/2100 与 6000/6000 delivery，未观察到连接、goroutine、DB pool 或内存 runaway。以上均为 `maximum tested`，不是最大容量声明。
