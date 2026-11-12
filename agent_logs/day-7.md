# Day 7 — 项目整理（文档一致性、README、代码整洁、日志结构、Git 整洁）（2026-11-12）

## 今日目标

把项目整理到“文档与实现一一对应、无死代码、无过期声明、日志结构统一、Git 整洁”；不补验证、不打 tag、不进入 V2。

## 完成项

- [x] 落盘本轮指令 `agent_logs/prompts/round-4.md`（176 行）。
- [x] 文档一致性：docs/05 补 Chat WS 与 `/healthz`、确认全部 `/api` 路由有实现或 V2/Mock 标注；docs/04 新增 17 表/16 UNIQUE/30 索引/40 CHECK 的 §7.11 清单；docs/06 标注 `friend.online/offline` 为 V2 并新增事件清单；docs/01 修正服务职责、补 `cmd/` 入口表与 17 service 说明。
- [x] README 重写：根 README + server/web/admin 三个子 README 全部改为当前事实；新增 `docs/validation-status.md`、`docs/known-issues.md`。
- [x] 代码整洁：删除 `internal/project`（0 import）、`FeaturePlaceholder.vue`（0 引用）、`ModulePlaceholder.vue`（0 引用）、无调用方的 `chat.New`；删除 4 个无读取配置键（`APP_ENV`/`LOG_LEVEL`/`WEB_ORIGIN`/`ADMIN_ORIGIN`）；改写 HomeView/AdminLayout 的过期 Day 文案。
- [x] agent_logs：新增 operations 索引与字段/编号约定；day-6/day-7 记录补齐目标/问题段；复核 Compose 计数修正（18→17）与“16 长期服务”表述。
- [x] Git 整洁：`.gitignore` 覆盖全部敏感/构建路径；历史无 `.key/.pem/reference/`；HEAD 无密钥标记；`reference/` 未跟踪；safety 脚本 PASS。

## 未完成 / 明确不做

- 未补 race/Compose/Kafka/性能验证（留第五轮）。
- 未进入 V2，未打新 tag，未移动 `gim-v1`。

## Phase 记录

### Task 1（文档一致性）

- 修改：`docs/01`、`docs/04`、`docs/05`、`docs/06`。
- 比对：路由 53 文档路径 vs 47 代码路径，剩余差异均为 V2/Mock/模板；schema 17 表全部覆盖。
- commit：`c180523 docs: align contracts with implementation`。

### Task 2（README）

- 修改：`README.md`、`server/README.md`、`web/README.md`、`admin/README.md`；新增 `docs/validation-status.md`、`docs/known-issues.md`。
- commit：`830956d docs: rewrite readmes to current state`。

### Task 3（代码整洁）

- 删除：`server/internal/project/doc.go`、`web/src/components/FeaturePlaceholder.vue`、`admin/src/components/ModulePlaceholder.vue`；删除 `chat.New`。
- 配置：删除 `APP_ENV`、`LOG_LEVEL`、`WEB_ORIGIN`、`ADMIN_ORIGIN`。
- 文案：HomeView、AdminLayout。
- commit：`6b6a03d chore: remove dead code and unused config`。

### Task 4（agent_logs）

- 新增 `agent_logs/operations/README.md`；补齐 day-6/day-7 记录字段。
- commit：`425dd05 docs(logs): normalize operation logs`。

### Task 5（Git 整洁）

- 检查命令与结果见 `operations/day-7/005_task-5_git-hygiene.md`。
- commit：`chore: verify repository hygiene`（本记录所在提交）。

## 当日问题

1. **`git grep` 命中项**只有两处“字符串本身”：`docs/13_GIT_GITHUB_WORKFLOW.md` 的 `AccessSecret` 检查项与 `agent_logs/prompts/round-4.md` 的指令文本，均非真实密钥。
2. **配置键扫描误报**：`*_API_PORT`/`*_RPC_PORT` 等动态构造键最初被误判为无读取；修正扫描逻辑后只删除 4 个真正无读取的键。
3. **day-4 编号重复**：按 checkpoint 并行编号导致；不重命名（避免破坏历史链接），改为在 operations 索引中给出映射。
4. **未使用导出函数**：无自动化工具；`chat.New` 经全仓 grep 确认后删除，其余登记为 known-issues E-7。

## 当日验收

```text
go build ./...                 PASS
go vet ./...                   PASS
gofmt -l .                     无输出
go test ./... -count=1         PASS（69 packages）
Web: vue-tsc / vitest 19 files 103 tests / build   PASS
Admin: vue-tsc / vitest 11 files 25 tests / build PASS
docker compose config --quiet  PASS（17 services）
check-repository-safety.ps1    PASS
```

## 次日入口

- 第五轮：race（Linux Docker/64-bit 工具链）、Compose 全栈 build/up + healthy、Kafka 集成（`GIM_KAFKA_INTEGRATION=1`）、性能基线复跑；随后按结果决定是否创建 `gim-v1.1`。
