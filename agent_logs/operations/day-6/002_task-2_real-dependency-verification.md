# 002 / Task 2 — 真实依赖验证

- **日期**：2026-11-11（Asia/Shanghai）
- **开始状态**：`HEAD = origin/main = 14aebef`，working tree clean。
- **结束状态**：无代码改动；生成物全部在 `K:\it\gim\.tmp\`（Git ignored）。
- **结论**：集成测试、migration、好友端到端 **真实通过**；race 与 Compose 全栈 **NOT RUN**（原因见下）。

## 1. 环境盘点

| 依赖 | 状态 | 证据 |
|---|---|---|
| Docker daemon | **不可用** | `docker info` → `open //./pipe/dockerDesktopLinuxEngine: The system cannot find the file specified` |
| MySQL 8.0 | 可用（本机原生） | `mysqld` 进程 + `127.0.0.1:3306` listens；`TestLocalInfrastructure/mysql` PASS |
| Redis | 可用（本机原生） | `redis-server` 进程 + `127.0.0.1:6379`；`TestLocalInfrastructure/redis` PASS |
| etcd 3.6.10 | 本机二进制，本轮手动启动 | `etcd.exe --data-dir .tmp/etcd-verify/data --listen-client-urls http://127.0.0.1:2379 ...`；`127.0.0.1:2379` listens；`TestLocalInfrastructure/etcd` PASS |
| Kafka | **不可用** | 无 9092 监听、无 broker；Kafka 真实消费用例按设计 SKIP |

为避免污染开发库 `gim`，本轮使用隔离库：

```text
数据库：gim_verify_20261111183624（CREATE DATABASE 成功）
Redis：DB 15
env：K:\it\gim\.tmp\env.integration（由 .env.local 派生，仅替换 MYSQL_DATABASE/REDIS_DB；含真实密码，不提交、不输出）
```

## 2. Migration（空库）

```powershell
go run ./cmd/migrate -env K:\it\gim\.tmp\env.integration -command up
```

真实输出：

```text
001_identity applied
002_friend applied
003_chat applied
004_group applied
005_file applied
006_settings applied
007_audit_logs applied
008_admin_management applied
UP_EXIT=0
```

`-command status` 同样 exit 0。

`TestDatabaseMigrationsAndUserAuthRegression` 在独立库中覆盖：首次 UP、重复 UP、`DownAll`、dirty 状态阻止 UP、清理后 RE-UP、约束/索引/EXPLAIN、User/Auth 回归 —— 本轮 **PASS**。

## 3. 集成测试（真实执行）

```powershell
$env:GIM_ENV_FILE='K:\it\gim\.tmp\env.integration'
go test ./tests -count=1 -v
```

**结果：20 PASS / 1 SKIP / 0 FAIL（21.561s）**，子测试 21 PASS。

| 用例 | 结果 |
|---|---|
| TestLocalInfrastructure（MySQL/Redis/etcd） | PASS |
| TestDatabaseMigrationsAndUserAuthRegression | PASS |
| TestGatewayAuthUserChain | PASS |
| TestGatewayAuthChatWebSocketTunnel | PASS |
| TestGatewayAuthGroupWebSocketTunnelAndPresenceBoundary | PASS |
| TestDiscoveryLeaseKeepAliveCleanup | PASS |
| TestAuthRedisBlacklistTTL | PASS |
| TestChatMessageMySQLAndWebSocketReliability（4 子测试） | PASS |
| TestChatSessionUnreadReadAndHistoryMySQL（7 子测试） | PASS |
| TestChatPresenceRedisMultiInstanceTTL | PASS |
| TestChatPresenceServicesRecoverCrashAndPreserveOtherInstance | PASS |
| TestGroupFoundationMySQLRedisTransactionsAndConcurrency | PASS |
| TestGroupMessageReliabilitySessionsHistoryAndDelivery（7 子测试） | PASS |
| TestFileServiceMySQLFilesystemIntegration | PASS |
| TestFileRepositoryOwnershipAndReferenceCount | PASS |
| TestFileThroughGatewayAuthAndEtcd | PASS |
| TestAdminRepositoriesAndManagementActions | PASS |
| **TestFriendLifecycleMySQLAndConcurrency（含并发同意）** | **PASS**（1.05s） |
| TestAuditRepositoryIdempotencyFiltersAndPrivacy | PASS |
| TestKafkaAuditUnavailableBrokerKeepsPublishBounded | PASS |
| TestKafkaAuditProduceConsumePersistQueryRestartAndIdempotency | **SKIP**（无 Kafka；需 `GIM_KAFKA_INTEGRATION=1`） |

好友并发用例真实断言：两个 goroutine 同时同意同一申请 → 恰好 1 个成功、1 个返回 `CodeRequestHandled`，`friends` 恰好 2 行。

## 4. Race — NOT RUN

三个具体阻塞项，均非“未尝试”：

1. 系统 PATH 默认 32-bit MinGW：`go test -race ./internal/chat/` → `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`。
2. 发现 64-bit MinGW `C:\Program Files\mingw64\bin\gcc.exe`（8.1.0），但安装不完整：`fatal error: stddef.h: No such file or directory`（缺 libc headers）。
3. Docker 不可用，无法使用此前记录中的 `golang:1.25-bookworm` Linux race 环境。

因此本轮 **不声称 race PASS**，Linux Docker race 仍为历史记录、未复验。

## 5. Compose 全栈 — NOT RUN

`docker compose config --quiet` PASS（18 services），但 `docker compose up -d --build` 需要 daemon；本轮 **未执行**，未产生 `compose ps` healthy 证据。

## 6. 好友端到端（真实服务，非 Compose）

由于 Docker 不可用，改用本机原生依赖 + 真实服务进程完成端到端：
`gim-user-rpc`、`gim-user-api`、`gim-auth-api`、`gim-gateway` 四个二进制（构建于 `.tmp/bin`），全部指向隔离库/Redis DB 15/本机 etcd；四个进程 + `/healthz` 均 200。

经 Gateway 的真实 HTTP 流程（输出已脱敏，不含 token）：

```text
STEP1  register A code=0 id=3; B code=0 id=4
STEP2  login A tokenIssued=True; B tokenIssued=True
STEP3  search code=0 hits=1 foundB=True isFriend=False
STEP4  apply code=0 requestID=2 status=0
STEP5  valid-list code=0 count=1 requesterID=3 status=0
STEP6  accept code=0 status=1
STEP7  friends A code=0 count=1 hasB=True isOnlineField=True isOnline=False
STEP8  remark code=0 remark=e2e buddy
STEP9  friend-info code=0 userID=4 remark=e2e buddy
STEP10 delete code=0 deleted=True; A-friends=0 B-friends=0
```

说明：`isOnline=False` 符合预期——本轮没有建立 Chat WS 连接，Presence 只由 Chat WS 写入；字段存在且来自 Redis。

端到端结束后已停止 4 个服务进程（端口 8080/8081/8082/9091 已释放）；etcd 保留给后续测试。

## 7. 顺带采集的“拒绝后再申请”现场证据

用真实服务链路（同一套进程）验证，详见 Task 3：

```text
E1 apply#1 code=0 id=3 status=0
E2 apply#2-while-pending code=1116 msg=已有待处理的好友申请
E3 reject code=0 status=2
E4 apply-after-reject code=0 newRequestID=5 status=0
E5 accept#2 code=0 status=1
E6 apply-when-friends code=1114 msg=对方已经是好友
```

即：**拒绝后允许再次申请**，pending 唯一键只约束未处理申请。

## 8. 本轮任务二结论

- PASS：migration（空库 UP/重复/DOWN/RE-UP，集成测试覆盖）、20 个集成用例、好友真实端到端。
- SKIP：Kafka 真实消费用例（无 broker）。
- NOT RUN：race、Compose build/up（Docker 不可用）。
- 未发现需要修复的缺陷；未产生代码改动，因此本任务无 commit。
