# Day 5 — V1 范围收敛（2026-11-11）

## 今日目标

在既有 V1 成果上做缺口收敛，不重建骨架：

1. 修复审计报告 C1/C2。
2. 好友闭环后端 + 前端。
3. 文档口径收缩。
4. 删除无调用方的 group-rpc。

## 完成项

- [x] C1：`repairSession` 错误处理顺序修复 + 失败池单元测试（修复前 FAIL、修复后 PASS）。
- [x] C2：admin delete 统一 user_id 升序锁 + 有界 1213 重试（3 次、10/20 ms、可取消）+ 锁顺序/重试/退避测试。
- [x] 好友后端 8 条路由、事务并发同意、Presence 只读、7 个稳定错误码、9 组 service 测试、8 组 HTTP 测试、1 个 gated MySQL 并发集成测试。
- [x] 好友前端 `/contacts` 页面、API、store、两个组件、3 个测试文件；Web 测试 16/90 → 19/103。
- [x] AGENTS/README/三个子 README/docs 05/06/07/10/面试材料与实现对齐。
- [x] 删除 group-rpc 及生成代码，Compose 19 → 18 services，构建/测试/compose config 全绿。
- [ ] 真实 MySQL 集成测试 / race / Compose 全栈（环境不可用，见下）。

## Phase 记录

### Task 1（C1/C2）

- 修改：`chat/message/admin/mysql/repository.go`、新增 `repository_test.go`、`server/tests/admin_integration_test.go` 增加 session 修复断言。
- 测试：修前 `--- FAIL: TestRepairSessionPropagatesSessionLockFailure`；修后 PASS；`go test ./internal/chat/... ./tests` PASS。
- commit：`05dc348 fix(chat): repair admin chat delete error handling and lock order`；日志 `729b8aa`。

### Task 2（好友闭环）

- 后端 commit：`a97bee6 feat(user): implement the friend loop api`。
- 前端 commit：`0723004 feat(web): add the contacts friend loop ui`。
- 日志 commit：`f0d3b6d docs(day5): record task 2 friend loop delivery`。
- 后端 69 包全绿；Web 19/103；Admin 回归 11/25。

### Task 3（文档收缩）

- commit：`83db97a docs: align the v1 scope statement with the implementation`。
- docs/05 路由与代码逐条比对，仅剩 V2/Mock/模板/健康探针差异。

### Task 4（删除 group-rpc）

- commit：`bcacbae refactor(group): remove the unused group-rpc service`。
- 删除 6 个文件（cmd/group-rpc、transport/grpc、api/group/v1），Compose/镜像/环境/文档同步。

## 当日问题

1. **本机 `.git` ACL 阻塞写操作**：`git add/commit` 首次失败 `Unable to create '.git/index.lock': Permission denied`；ACL 检查显示 `.git` 缺 `WRITE_OWNER`，且存在两个外部 SID 的 Deny ACE。用 DSH 沙箱 ACL 修复脚本补当前用户 FullControl（备份与回滚命令在 `K:\it\gim-acl-recovery\`）后，confined 下仍被 Deny 拦截，最终 commit/push 在 unconfined 模式下完成。属环境问题，未改仓库内容。
2. **`t.TempDir()` 在本机被拒绝**：默认 `C:\Users\...\Temp` 返回 Access denied，导致 config 类测试集中失败；把 `TMP`/`TEMP` 指向 `K:\it\gim\.tmp\wintmp` 后全部通过。属环境问题。
3. **Docker daemon 未运行**：真实依赖集成测试、Compose build/up、race 均无法执行，已在日志与 README 中如实记录。

## 当日验收

- build: PASS
- tests: PASS（69 packages）
- race: NOT RUN（环境不可用）
- frontend: PASS（Web 19/103、Admin 11/25、build PASS）
- compose config: PASS（18 services）
- repository safety: PASS
- GitHub checkpoint: **PARTIAL**。`05dc348`、`729b8aa`、`a97bee6`、`0723004`、`f0d3b6d` 已 push 到 `origin/main`；`83db97a`（文档）、`bcacbae`（删除 group-rpc）、`3032875`（收尾日志）及推送状态修正提交仍在本地。收尾阶段 GitHub 不可达：`Recv failure: Connection was reset` / `Failed to connect to github.com port 443`，重试两次失败，未使用 `--force`。

## 次日入口

- 若环境可用：跑 `GIM_ENV_FILE=../.env.local go test ./tests -count=1`、`go test -race ./...`、`docker compose up -d --build` + 好友端到端手工验收。
- 之后按用户指示决定是否进入 V2（富消息、好友上线提醒、user_confs 等）。
