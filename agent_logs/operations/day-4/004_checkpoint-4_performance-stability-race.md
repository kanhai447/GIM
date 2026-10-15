# Day 4 / Checkpoint 4 — Performance + Stability + Race Verification

- **日期：** 2026-10-15（Asia/Shanghai）
- **授权范围：** 仅 Performance / Stability / Race Verification；不新增业务功能，不进入 Day 4 Final Acceptance、Final README、面试材料或 Kubernetes。
- **开始状态：** `main`，`HEAD == origin/main == f86b552c5e1ca15a0dd51c6d5e647b25cb6d5e98`，working tree clean。

## Environment

- Windows host：13th Gen Intel Core i9-13980HX，24 cores / 32 logical processors，15.63 GiB RAM；预检时约 6.5 GiB host RAM free。
- Docker Desktop 4.69.0 / Docker Engine 29.4.0，Linux amd64；Docker VM 32 CPUs、约 7.57 GiB memory。
- Compose 恢复后 16 个长期服务全部 healthy，migration/storage-init 保持既有 completed 状态；预检无 OOMKilled。
- Docker Desktop 首次恢复被 4.69.0 Model Runner / Secrets Engine 的损坏 Windows AF_UNIX runtime socket 阻塞。没有 factory reset 或删除 Compose 数据；精确保留损坏 runtime 目录为 `run.stale-*` / `docker-secrets-engine.stale-*`，并按 Docker 官方 settings contract 显式设置用户级 `EnableInference=false` 后恢复 Linux engine。
- `git fetch origin` 实际成功；恢复后再次确认 `HEAD == origin/main`。
- 压测资源门禁：渐进负载；若出现 host/Docker memory 紧张、OOM、持续 CPU 饱和、容器重启、DB/Redis 无响应或错误率明显上升，停止增加负载并记录 safe maximum tested。

## Harness

已完成并通过聚焦单测/vet：

- `server/tools/loadtest/` 是单一可复现 Go 工具，提供 `prepare / cleanup / http / private / group / file` 子命令。
- 所有用户请求均通过公开 Gateway；MySQL 直连仅用于创建当前 V1 尚无公开批量接口的好友 fixture、核验持久化/session/unread/dedup，以及清理带前缀测试数据，不计入性能路径。
- HTTP 统计 total/success/failure/timeout/protocol/business、throughput、p50/p95/p99；Private/Group 额外核验 ACK、`clientMsgId` 幂等、delivery fanout、unread、重连；File 使用 `io.Pipe` 流式生成内容并核验 hash/object/reference。
- 凭据只从 `GIM_PERF_PASSWORD`、`GIM_PERF_MYSQL_DSN` 运行时环境读取；manifest 只记录临时账号 ID/名称、组 ID 与非秘密端点，不保存密码、JWT、DSN 或消息正文。
- 增加 `scripts/perf-snapshot.ps1`，统一采集容器 CPU/RAM/PID、restart/OOM、内部 Go runtime/DB/audit 队列、MySQL connection、Redis clients 和 Kafka lag。
- 增加 opt-in `internal/platform/debugserver`：默认关闭，只接受显式 loopback IP，未通过 Gateway 暴露；Compose 仅在 Chat/Group/File/Logs 容器内监听 `127.0.0.1:6060`。
- 边界修正：截止瞬间不误计超时、上传提前失败主动关闭 pipe、unread 使用加法等式避免 `uint64` 下溢。
- 验证：`go test` 与 `go vet` 覆盖 loadtest/debugserver/database/kafkaaudit 及四个接入服务；`docker compose config --quiet`、`git diff --check` 通过。

## HTTP baseline

均为 Gateway 路径、8 秒/档、0 failure / 0 timeout / 0 protocol/business error：

| workload | concurrency | throughput req/s | p50 ms | p95 ms | p99 ms |
|---|---:|---:|---:|---:|---:|
| login | 1 / 5 / 10 / 25 | 5.62 / 25.62 / 46.25 / 99.98 | 173.43 / 190.94 / 213.90 / 246.11 | 180.60 / 209.74 / 226.15 / 254.92 | 184.22 / 212.53 / 233.29 / 298.01 |
| profile | 1 / 10 / 25 / 50 | 501.73 / 2772.67 / 2622.47 / 2157.69 | 1.95 / 3.61 / 8.98 / 19.62 | 2.71 / 4.86 / 15.43 / 52.19 | 3.44 / 5.79 / 18.96 / 77.51 |
| session | 1 / 10 / 25 / 50 | 422.49 / 688.84 / 508.91 / 498.35 | 2.22 / 7.43 / 49.04 / 89.87 | 3.35 / 49.02 / 102.95 / 123.86 | 4.28 / 59.43 / 116.68 / 141.56 |

拐点：profile 在 10 并发、session 在 10 并发达到本轮吞吐峰值；继续到 25/50 后吞吐回落且尾延迟增加。因此停止扩大 HTTP 并发。登录为 bcrypt CPU 成本路径，25 并发仍无错误，但本轮不把约 100 req/s 声称为容量上限。

## Private Chat WS

Gateway 鉴权连接阶梯 25/50/100/200，全部连接成功、无意外掉线；connect p95 分别 3.38/4.66/2.96/3.64 ms，200 连接 p99=5.43 ms。消息档位为 100 connections：

- 20 msg/s × 20s：400/400 unique ACK，p50/p95/p99=10.02/13.84/18.10 ms；20 次相同 `clientMsgId` retry 均返回一致 ACK；400 行持久化，1040/1040 delivery，unread delta 正确。
- 50 msg/s × 20s：1000/1000 unique ACK，p50/p95/p99=9.10/13.47/18.02 ms；50 次 retry 一致；1000 行持久化，2600/2600 delivery，unread delta 正确。
- 全部档位 correctness pass，0 ACK conflict / timeout / business error / protocol error / unexpected drop。

## Group

Gateway 路径，全部 correctness pass、0 failure/timeout/drop、幂等 ACK/message row/unread/fanout 一致：

- 3/5/10 人、单设备、10 msg/s × 15s：每档 150 unique ACK；fanout 分别 450/750/1500，全部完整。
- 5 人、每成员 2 设备、20 msg/s × 15s：300 unique ACK，3000/3000 delivery，15 次 duplicate retry 一致。
- 稳态 ACK p50 约 11 ms、p95 约 14–15 ms、p99 约 19–21 ms。首次 3 人档观察到一次 p95=2.10s/p99=2.55s 的启动期离群点但无错误、丢失或积压；后续三档未复现，保留在报告中并将在 soak/re-run 继续观察。

## File

全程经 Gateway，生成器使用 `io.Pipe`，不在客户端或服务端 `io.ReadAll` 大文件；全部 0 failure/timeout 且 hash/object/reference 正确，UserFile 引用通过公开 API 清理：

- 1 MiB，concurrency 1/5/10：upload 39.33/63.78/60.75 MiB/s，p95 25.43/78.39/164.10 ms；download 141.20/198.29/182.72 MiB/s。
- 10 MiB，concurrency 1/5：upload 122.00/211.75 MiB/s，p95 81.97/236.13 ms；download 150.15/263.10 MiB/s。
- 相同文件名 + 相同内容并发 10：10 次成功上传仅形成 1 个 UserFile reference / 1 个 FileObject / 1 个 object UID；upload 118.22 MiB/s，p95=83.93 ms。
- 10 MiB 内容因内容寻址对象已在第一档建立，后续并发仍完整流式读取、计算 hash 并复用对象；数字反映本机 loopback + 已存在对象路径，不外推网络带宽。

## Kafka

正常链路在阶梯后 consumer offset 4421/4421、lag=0，四个 instrumented service producer queue=0/drop=0。故障注入分两次：

- 停止本地 Kafka 后，10 msg/s 私聊仍可持久化、ACK 与实时投递；producer 不阻塞业务，队列最高观察到 138/1024。
- 配置为 `capacity=1024 / publish timeout=750ms / max attempts=3 / retry backoff=100ms`。第一次 15 秒 outage 的 146 条已接受消息中，110 条在恢复后进入 Kafka，36 条超过重试边界被 drop；第二次 10 秒 outage 的 100 条业务消息全部成功，部分审计同样在恢复前到达 drop 边界。Chat producer 累计 drop=51。
- Kafka 恢复后 producer queue=0，consumer 最终 offset=log-end-offset、lag=0；审计落库随第一轮由 4297 增至 4407，正好对应 110 条最终进入 Kafka 的事件。
- 结论：核心 IM 与审计正确解耦；V1 明确是 bounded best-effort audit，长 outage 会有可观测丢失，尚无 disk spool/outbox，不声称 exactly-once。

## Resource observation

空闲基线（2026-10-15 15:48 +08:00）：所有长期容器 healthy、无 OOM；Gateway/各 Go 服务约 9–12 MiB，MySQL 467 MiB、Kafka 451 MiB；Kafka lag=0。Chat/Group/File/Logs goroutine 分别为 36/35/25/31，heap alloc 约 4.6–4.8 MiB。MySQL `Threads_connected=9 / Threads_running=2 / Aborted_connects=0`，Redis `connected_clients=9 / blocked_clients=0`。MySQL pool 已显式配置 `maxOpen=25 / maxIdle=10 / maxLifetime=30m`，不属于“完全依赖默认值”；只有实测证明瓶颈后才允许调整。

阶梯后快照：所有容器 healthy、无 OOM/restart；Chat/Group/File/Logs heap alloc 约 4.0–5.5 MiB，goroutine 48/38/36/31；Chat DB pool peak 后保留 10 idle，`WaitCount=0`，Group/File 也为 0；MySQL `Threads_connected=40 / running=2 / Aborted_connects=0`；Redis `connected_clients=27 / blocked_clients=0`；Kafka lag=0。Kafka 在大量 audit 事件刚完成时瞬时约 158% CPU、803 MiB，需在 soak 与静置后继续观察是否回落。

## Soak

并行 5 分钟：Private 50 connections / 5 msg/s，Group 10 members / 2 msg/s。中段与初期相比，Chat goroutine 均为 143、Group 均为 59；容器内存 Chat 18.85→20.49 MiB、Group 18.51→18.38 MiB，heap 随 GC 正常波动；DB WaitCount=0、Kafka queue/lag=0、Redis blocked=0、无 OOM/restart，未发现 goroutine 或内存单调泄漏。

- Group：600/600 ACK，600 rows，6000/6000 fanout，12 次 duplicate retry 一致，p50/p95/p99=11.65/23.17/309.64 ms，0 error/drop/reconnect。
- Private（修复前）：1500 sends，1495 rows/ACK，2093/2093 accepted-message fanout，14 次 `2204`，最终同 clientMsgId retry 使 9 次恢复、5 条仍未接受；无重复行、unread 正确。该结果触发下述 deadlock 诊断与必要修复。
- 修复后同参 2 分钟：600/600 ACK/rows，840/840 fanout，12 次 duplicate retry 一致，p50/p95/p99=10.59/25.82/233.66 ms，0 error、0 final retry、0 drop/reconnect，correctness pass。

## Restart stability

轻量负载（10 connections、2 msg/s、45 秒）分别重启 Chat/Group API：

- 两条路径均 10/10 sockets 自动重连；每条在 restart window 有 1 次 write failure，按真实 Web store 语义用原 `clientMsgId` 最终重试。
- Chat：90/90 rows/ACK、90/90 delivery，unread 正确；Group multi-device：90/90 rows/ACK、900/900 fanout，unread 正确；均无重复行/ACK conflict。
- recovery 消息端到端 p99 分别约 35.5s/29.5s（从首次发送计时），属于重启恢复延迟，不能与稳态 ACK 延迟混用。
- 更高的 50 connections / 10 msg/s Chat 探索档在 restart window 暴露 37 个 2204、2 条已持久化但 ACK/live delivery 在进程终止前丢失；history/unread 保留，V1 无 receiver delivery ACK/outbox。轻量验收档通过，但该高压重启结果作为已知限制保留。

## Bottlenecks

- HTTP profile/session 在并发 10 后吞吐回落且 tail latency 上升，单机拐点明确；未出现 pool wait，主要是本机 Gateway/服务/DB 综合饱和，不盲调连接池。
- Private soak 的通用 2204 经安全 cause 分类稳定定位为 MySQL `1213` deadlock：修复前同参 2 分钟 599 sends 中有 7 次 2204，最终仅 597 rows。
- Kafka audit 在短 outage 内由内存队列吸收，但每事件 bounded retry 后会 drop；这是明确可观测的 V1 降级边界。
- 高压服务 restart 允许“DB 已提交但 ACK/Redis live publish 未完成”窗口；session/unread/history 保证最终可查，实时 delivery 不是 exactly-once。

## Optimizations

- 唯一业务路径修复：`PersistWithSessions` 只对 MySQL 1213 做最多 3 次重试（10/20ms context-aware backoff），每次重建 GORM row/transaction；不吞掉其他错误，不调整 pool 或 timeout。
- 增加 `classifyCause`，最终发送失败只记录 sender/receiver ID、公共 code 和归类后的 `mysql_N / grpc_code / deadline`，不记录 SQL、消息正文、JWT、密码或凭据；有单测保证 MySQL message 文本不进入分类结果。
- 修复后完全同参 2 分钟 600/600 成功且无最终 2204，证明修复针对实测瓶颈有效。

## Race

- Windows native：已知本机 32-bit MinGW 不支持 windows/amd64 race，本轮不修改用户 GCC。
- Linux Docker `golang:1.25-bookworm`：
  - focused：`go test -race` 覆盖 Chat/Group/File/Audit/KafkaAudit/Logs/loadtest，PASS，无 race warning。
  - full unit：`go test -race ./... -count=1`，PASS；无 `GIM_ENV_FILE` 时现有真实依赖 tests 按设计 skip。
  - real integration：停止应用层容器并等待 etcd 15s lease 清理，仅保留 MySQL/Redis/etcd/Kafka；容器 `/tmp` 动态生成 root 测试配置后 `go test -race ./tests -count=1`，PASS（24.628s），随后全栈恢复 16/16 healthy。
- race 脚本第一次发现 Debian login shell PATH 丢失，改为绝对 `/usr/local/go/bin/go`。
- 第一次 real integration race 发现 Kafka 测试复用生产 topic + 新 group + FirstOffset，约 9000 条历史 backlog 导致 10s 超时，并非 race。测试改为每次唯一临时 topic 且 cleanup 删除，复跑 PASS；保留 consumer 首次消费、重启续读和 event-id 幂等语义。
- CI：新增 GitHub Actions backend test/vet/gofmt、web/admin install/type-check/test/build、full + focused Linux race；remote run 待 push 后观察。

## Correctness

- 每一档均同时核验 attempts/success/failure/timeout/protocol/business error；Private/Group 进一步核验 ACK、message rows、same-clientMsgId 幂等、session/unread、fanout 与 multi-device exact-once per active client。
- Private 20/50 msg/s、Group 3/5/10 members 和 2-device、File 1/10 MiB 与 concurrent dedup、Kafka normal/degraded 均完成持久化一致性验证；修复后 soak 600/600 rows/ACK、840/840 delivery。
- 重启验收档 Chat 90/90 rows/ACK/delivery、Group 90/90 rows/ACK 且 900/900 delivery，无 duplicate storm；高压探索档的已提交/未 ACK 窗口保留为限制，不伪装为通过。
- Backend full test/vet/gofmt、Web 16 files / 90 tests/type-check/build、Admin 11 files / 25 tests/type-check/build、Compose config、repository safety 与 `git diff --check` 均 PASS。
- 测试数据清理后核验：`perf-user-*`、`pc-*`、`gc-*` 与本轮两个无引用 FileObject 均为 0；对应两个物理对象已按精确 hash path 删除。16 个长期 Compose 服务保持 running/healthy，migration/storage-init 为既有 exited 0。

## Security

- harness/CI/race/performance docs 未包含真实 Secret、token、password、DSN、消息正文或物理 storage path；测试密码和 DSN 仅运行时环境注入。
- 所有最终用户路径性能数字均经过 Gateway -> Auth -> target service；MySQL 直连仅用于 fixture、correctness verification 与 prefix cleanup，不计入性能路径。
- debug metrics/pprof 默认关闭，只允许显式 loopback IP，Compose 监听容器内 `127.0.0.1:6060`，未添加 Gateway route。
- deadlock cause log 只输出业务 ID、公共 code 与 `mysql_1213` 等安全分类；单测确认驱动错误文本/SQL 不进入日志。
- CI 使用无本机 `.env.local` 依赖的 unit/race/frontend 流程；Secret Scan 与 repository safety PASS。

## Known Limitations

- 结果是单机 Windows + Docker Desktop baseline，不是生产容量或最大容量；HTTP 每档 8 秒，长期趋势主要由 5 分钟 soak 覆盖。
- 200 Private WS 是本轮 safe maximum tested，不表示连接上限；未继续到 300/500，以避免在 7.57 GiB Docker VM 中无意义追极限。
- V1 audit 是 bounded best-effort；broker outage 超过有限重试会 drop，无 outbox/disk spool/DLQ/exactly-once。
- 服务进程终止可能发生 DB 已提交但 ACK/Redis live publish 未完成；history/session/unread 最终可查，但无 receiver delivery ACK 或 realtime exactly-once。
- 首个小群启动档出现一次 2 秒级 tail outlier，未在后续档复现。
- CI workflow 不运行真实 MySQL/Redis/etcd/Kafka integration race；该范围已由本地 Linux Docker real-dependency race PASS 覆盖。
- Kafka CLI 在当前容器 advertised-listener 下无法从容器内管理残留的临时 consumer group；这些 group 不包含业务数据。共享 audit topic 的 perf events按正常 retention 保留，未做危险 topic truncate。
- Admin build 仍有既有的非阻塞 chunk-size warning。

## Reference use / independent implementation

- 参考 FIM 的只有既有用户路径/两条 WebSocket/文件/管理审计业务语义。
- 本轮工具、资源观测、race/CI 与报告均针对当前 GIM 自主实现，不复制 reference 代码，也不将 reference 作为运行依赖。

## Git

- implementation/perf/CI commit：`69955ce3e7caa032db60aa85bf88588e50e703f9 test(perf): add reproducible stability and race verification`。
- docs/status commit：本记录所在提交。
- push：文档提交后执行并复核 `HEAD == origin/main` / clean。
