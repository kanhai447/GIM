# GIM 第五轮 — 补验证、更新验证状态、打 gim-v1.1 tag

## 0. 前提与授权

第四轮已完成：HEAD = origin/main = bce78cc，工作区干净，文档一致性、README、代码整洁、agent_logs 结构、Git 整洁均已完成。gim-v1 tag 存在，本轮不移动。

本轮目标：尽量补 race / Compose / Kafka / 性能验证；无论结果如何，更新 docs/validation-status.md 和 docs/known-issues.md，然后打 gim-v1.1 tag 并 push。

授权范围：

- 尝试执行 race、Compose 全栈、Kafka 真实消费、性能基线复跑、浏览器 E2E；环境不可用则如实记录 NOT RUN/SKIP。
- 更新 docs/validation-status.md、docs/known-issues.md、README.md 的验证状态与 tag 说明。
- 新建 annotated tag gim-v1.1，指向本轮最终通过验收的 commit；不移动、不删除 gim-v1。
- push main 和 tag。

明确不做：

- 不进入 V2。
- 不重构业务代码；只有验证发现必须修的最小缺陷才改，且先报告。
- 不放宽断言、不跳过测试、不虚构结果。
- 不 --force、不移动/删除旧 tag。

如果必须偏离，先停止并问我。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push、tag 结果。
- 每个可独立验收操作写 agent_logs/operations/day-8/；没有记录视为未完成。
- push/tag 失败记录真实错误，不 --force，不删除远端 tag。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-5.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = bce78cc、gim-v1 存在。

## 3. 任务一：补验证（逐项尝试，如实记录）

### 3.1 race

- 先尝试本机 `go test -race ./...`；若 32-bit MinGW 报错，记录真实错误。
- 若 Docker 可用，在 Linux 容器内执行 `go test -race ./...`，记录镜像、命令、结果。
- 若都不可用，记录 NOT RUN 及原因。
- 重点包：`server/internal/chat/...`、`server/internal/user/friend/...`。

### 3.2 Compose 全栈

- 若 Docker daemon 可用：`docker compose up -d --build`，等待 healthy，记录 `docker compose ps` 真实状态。
- 跑一次好友 E2E（可复用第三轮脚本），记录真实输出。
- 若 daemon 不可用，记录 NOT RUN 及原因。

### 3.3 Kafka 真实消费

- 若有 broker：设置 `GIM_KAFKA_INTEGRATION=1` 和 broker 地址，跑 Kafka 集成用例。
- 若无 broker：记录 SKIP 及原因。

### 3.4 性能基线

- 若 Compose 全栈可用，复跑 docs/performance/baseline.md 中的最小场景，记录当前环境与结果。
- 若不可用，记录 NOT RUN，并明确“历史数字不作为当前结论”。

### 3.5 浏览器 E2E

- 若前端静态站点可启动且有浏览器环境，跑一次好友流程 E2E；否则记录 NOT RUN。
- 不要求本轮必须做，优先级低于 race/Compose。

### 3.6 验收

- 每项真实结果：PASS / FAIL / SKIP / NOT RUN，附命令与环境。
- 若 FAIL，先报告根因和最小修复方案，等我确认后再改。

## 4. 任务二：更新验证状态与已知问题

更新 docs/validation-status.md：

- 每项状态、命令/环境、证据/备注。
- 明确区分“已验证”“NOT RUN”“SKIP”。
- 若某项本轮 PASS，更新证据；若仍 NOT RUN，保留并更新原因。

更新 docs/known-issues.md：

- 把已验证项从“待验证”移到“已验证”或删除。
- 保留 V2 功能限制与工程限制。

更新 README.md：

- 验证状态小节链接 validation-status.md。
- tag 说明：gim-v1 是上一轮基线，gim-v1.1 是本轮整理 + 验证状态更新后的版本；明确 gim-v1.1 是否包含 race/Compose 验证。

操作日志记录变更。

## 5. 任务三：打 gim-v1.1 tag

- 确认任务一、二的 commit 已提交并推送。
- 确认 gim-v1 仍指向原 commit，未被移动。
- 执行：

```bash
git tag -a gim-v1.1 -m "GIM V1.1: project tidy-up, friend loop, C1/C2 fixes, validation status"
```

指向本轮最终 commit（不是硬编码 bce78cc；如果本轮有文档 commit，tag 指向最终 commit）。

- `git push origin main`，再 `git push origin gim-v1.1`。
- 确认 `git tag` 同时存在 gim-v1 和 gim-v1.1，gim-v1 hash 不变。
- 若 race/Compose/Kafka 仍 NOT RUN，tag message 和 README 必须如实写“功能完成 + 集成测试 PASS + race/Compose/Kafka 未验证”，不得写“全量验证通过”。
- 操作日志记录 tag hash、push 结果、旧 tag 是否保持不变。

禁止：`git tag -f` 移动旧 tag、`git push --force` 覆盖远端 tag、删除 gim-v1。

## 6. 执行顺序与 commit

```text
落盘 round-5.md
  -> 任务一补验证（逐项尝试，记录真实结果）
  -> 如发现缺陷：报告 -> 确认 -> 最小修复 -> 测试 -> commit
  -> 任务二更新 validation-status / known-issues / README -> commit
  -> 任务三打 tag + push
  -> 全量汇总
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

建议 commit message：

```text
docs: update validation status after round-5 checks

chore(release): tag gim-v1.1
```

## 7. 本轮结束输出

- 已完成/未完成。
- 验证真实结果：race / Compose / Kafka / 性能 / 浏览器 E2E 每项 PASS/FAIL/SKIP/NOT RUN + 原因。
- 修改文件清单。
- commit 列表与 hash。
- push 结果：main、tag 是否成功，真实错误如有。
- tag gim-v1.1 指向的 commit hash；gim-v1 是否保持不变。
- 操作日志清单。
- 未验证项清单。
- 下一步建议：V2 优先级或继续补验证。

## 8. 遇到以下情况停止并问我

- race/Compose/Kafka/性能全部不可用，是否允许本轮以“文档更新 + tag”收尾。
- 验证发现业务缺陷，修复超出“最小修复”。
- tag 推送失败（认证/网络），是否保留本地 tag。
- 发现旧 tag gim-v1 被误移动或远端有冲突。
- docs/00_DECISIONS.md 需要修改才能与 tag 说明一致。
