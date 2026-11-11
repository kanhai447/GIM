# Day 6 — 补推、真实依赖验证与遗留清理（2026-11-11）

## 今日目标

1. 补推 round 2 遗留的 4 个本地 commit。
2. 在真实依赖环境验证集成测试、migration、好友端到端；race/Compose 视环境记录。
3. 明确“拒绝后再申请”语义并补测试与文档。
4. 清理 4 个无用 RPC 环境键、核查 group api 残留、确认 1113–1119 错误码。

## 完成项

- [x] 4 个本地 commit 推送成功：`f0d3b6d..14aebef`，本地与远端一致。
- [x] 真实依赖验证：
  - MySQL 8.0 / Redis 本机原生可用；etcd 3.6.10 本机二进制启动；Kafka 不可用。
  - 隔离库 `gim_verify_*`（Redis DB 15），空库 migration 001–008 UP 成功。
  - 集成测试 **20 PASS / 1 SKIP / 0 FAIL**（Kafka 真实消费用例因无 broker SKIP）。
  - 好友端到端 10 步全部通过（真实 Gateway→Auth→User API→User RPC→MySQL）。
- [x] “拒绝后再申请”语义：现场验证 + MySQL 集成测试 + docs/05/07 明确；无需改代码。
- [x] `.env.example` 删除 4 个无读取的 `*_RPC_PORT`；`server/api/group/v1` 已不存在且无 import；docs/05 新增 11xx 错误码分配表，1113–1119 无冲突。
- [x] 全量验收：后端 69 包 PASS，Web 19/103 + build，Admin 11/25 + build，compose config PASS（17 services），safety PASS。
- [ ] race：NOT RUN（无 64-bit C 工具链 / 无 Linux Docker）。
- [ ] Compose build/up：NOT RUN（Docker daemon 不可用）。
- [ ] Kafka 真实消费链路：SKIP（无 broker）。

## Phase 记录

### Task 1（补推）

- `git fetch` 无远端新提交；`git push origin main` → `f0d3b6d..14aebef`；`origin/main..HEAD` 为空。
- 无新增 commit。

### Task 2（真实依赖验证）

- 环境：本机 MySQL/Redis + 手动 etcd；Docker/Kafka 缺失。
- migration：`go run ./cmd/migrate -env .tmp/env.integration -command up` → 001–008 applied。
- 集成：`GIM_ENV_FILE=.tmp/env.integration go test ./tests -count=1 -v` → 20 PASS / 1 SKIP / 0 FAIL（21.561s）。
- 好友 E2E：真实四服务链路，STEP1–STEP10 全部 code=0，好友列表出现、`isOnline` 字段存在、备注、删除双方消失。
- 结束清理：停止 4 个服务与临时 etcd，DROP 隔离库，删除含凭据的临时 env 与二进制。

### Task 3（拒绝后再申请）

- 现场证据：pending 重复 → `1116`；拒绝后可再次申请（新 request id）；已好友 → `1114`。
- 测试扩展：`TestFriendLifecycleMySQLAndConcurrency` 新增 rejected 历史保留、re-apply 成功、重复 pending、accept 后 2 行、已好友拒绝等断言。
- 真实 MySQL 运行 PASS（1.18s）。
- commit：`d3be2c9 test(user): cover reject-then-reapply semantics`。

### Task 4（清理与错误码）

- `.env.example` 删除 `AUTH/CHAT/FILE/SETTINGS_RPC_PORT`，保留 `USER_RPC_*`。
- `server/api/group/v1` 不存在、无 import；`server/api` 仅剩 `user/v1`。
- docs/05 新增 11xx 分配表，1110–1112 Admin、1113–1119 Friend，无重复。
- 修正历史 Compose 服务数误计（18→17，原记录 19/18 有误）。
- commit：`df82d59 chore: drop unused rpc env keys and document friend error codes`。

## 当日问题

1. **Docker 不可用**：无法 Compose build/up、无法 Linux race；已改用本机原生依赖 + 真实服务进程完成集成与 E2E。
2. **race 工具链不可用**：32-bit MinGW 报 `64-bit mode not compiled in`；`C:\Program Files\mingw64` 的 8.1.0 缺少 libc headers（`stddef.h` 找不到）；Docker 亦不可用。
3. **配置路径陷阱**：`GIM_ENV_FILE=..\.env.local` 在 `go test ./tests` 下相对包目录解析失败；本轮改用绝对路径。
4. **PowerShell 写 UTF-8 BOM** 导致 migrate 报 `invalid configuration at line 1`；改用无 BOM 写入后通过。
5. **Compose 服务数误计**：round-2 记录的 19/18 有误，实际删除前 18、删除后 17；已修正历史记录。

## 当日验收

- build：PASS
- unit tests：PASS（69 packages）
- integration tests：**20 PASS / 1 SKIP / 0 FAIL**（真实 MySQL/Redis/etcd）
- race：NOT RUN（工具链/Docker 限制）
- frontend：Web 19/103 + build PASS；Admin 11/25 + build PASS
- compose config：PASS（17 services）
- repository safety：PASS
- 好友 E2E：PASS（10 步）
- GitHub checkpoint：任务 1–4 commit 已推送（`14aebef..df82d59`）；本日志提交随后推送。

## 次日入口

- 若 Docker 可用：`docker compose up -d --build`、Linux `go test -race ./...`、Kafka `GIM_KAFKA_INTEGRATION=1` 用例。
- 之后按用户指示决定是否进入 V2。
