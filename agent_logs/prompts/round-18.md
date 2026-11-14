# GIM 第十八轮 — 修 K-3 + 打 gim-v2.1 tag + README 文档索引

## 0. 前提与授权

第十七轮已完成：docs/interview/design-questions.md 已生成并推送；HEAD = origin/main = 2b9d777，工作区干净；六个 tag 未移动；未打新 tag。

本轮目标（只做三件事，不扩范围）：

1. 修正 docs/known-issues.md 中 K-3 的过期表述。
2. 更新根 README.md 的文档索引，加入 architecture-deep-dive.md 和 interview/design-questions.md。
3. 打 annotated tag gim-v2.1，指向本轮最终通过验收的 commit，并 push。

本轮明确不做：

- 不写业务代码、不改 server/、web/、admin/ 源码。
- 不清理 A1/A2（deletion 表、HideSession 接线）。
- 不补 Group 1213 重试（A5）。
- 不加新功能、不打 gim-v2.0 之外的任何语义 tag。
- 不移动/删除已有六个 tag。
- 不放宽测试、不虚构结果、不 --force。

如果发现 K-3 修改需要连带改其他条目才能自洽，先停止报告，输出拟修改清单，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push、tag 结果。
- 每个可独立验收操作写 agent_logs/operations/day-21/；没有记录视为未完成。
- commit 前 git status、git diff --check、确认无 .env/reference//密钥进入 staged。
- tag 指向已推送的 commit，且该 commit 的文档状态与 tag message 一致。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-18.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = 2b9d777、六个 tag 存在。

## 3. 任务一：修正 K-3

### 3.1 先读

- 读 docs/known-issues.md 中 K-3 的完整条目。
- 读 K-15、K-16（好友上线提醒单机语义、offline best-effort）。
- 读 docs/06_WEBSOCKET_PROTOCOL.md §5.7 和 docs/v2-scope.md 中好友上线提醒的状态。
- 读 docs/interview/design-questions.md 3.13 节，确认它如何描述好友上线提醒。

### 3.2 修改

将 K-3 从“无 friend.online/offline 推送”修正为准确表述，建议结构：

```markdown
- **K-3**（修正于 2026-11-14）：好友上线提醒已实现（单机），由 Chat WS 推送 `friend.online`/`friend.offline`。
  - 历史：V1 阶段此条为“无好友上线推送”，第十三轮已实现单机版本。
  - 当前有效边界见 K-15（单机语义，多实例会重复/误报）和 K-16（offline best-effort）。
```

要求：

- 不删除 K-3 编号，不删除历史信息；用“修正于”保留可追溯性。
- 不修改 K-15、K-16 的内容。
- 如果 K-3 在其他文档（architecture-deep-dive.md、design-questions.md、v2-scope.md、validation-status.md）被引用为“无推送”，检查这些引用是否已过时：
  - 如果已过时，只记录到操作日志，本轮不改这些文档；如需改，先停止问我。
  - 如果引用是“K-3 见 known-issues”这类指针，无需改。
- 在 docs/known-issues.md 顶部或修正处注明核对日期。

### 3.3 验收

- K-3 表述与 docs/06 §5.7、v2-scope.md、validation-status.md 一致。
- 其他文档中如有“无好友上线推送”的过期表述，列入操作日志的“待修正清单”，不擅自改。
- 操作日志记录修改前后对比。

## 4. 任务二：更新根 README.md 文档索引

### 4.1 先读

- 读 README.md 现有的文档索引/目录说明部分。
- 确认 architecture-deep-dive.md、interview/design-questions.md、v2-scope.md、validation-status.md、known-issues.md 是否已在索引中。
- 确认 docs/interview/ 下其他文件（如 project-overview.md、resume-bullets.md）是否在索引中。

### 4.2 修改

在 README 的文档索引部分增加或整理，至少包含：

| 文档 | 用途 |
|---|---|
| docs/architecture-deep-dive.md | 源码级架构：进程、链路、16 个功能、数据模型、并发、配置、测试地图 |
| docs/interview/design-questions.md | 面试设计问答：16 个功能 + 横向问题 + 故障专题 + 简历表述 |
| docs/v2-scope.md | V2 范围与边界：已实现、V2 后期/不做、已知边界 |
| docs/validation-status.md | 验证状态：已验证 / 临时执行未入库 / NOT RUN |
| docs/known-issues.md | 已知边界与架构审计发现 |
| docs/interview/project-overview.md | 项目概览（如存在） |
| docs/interview/resume-bullets.md | 简历表述建议（如存在） |

要求：

- 只改索引，不改 README 其他内容。
- 链接使用相对路径，确认文件存在。
- 不新增“全量验证通过”“支持百万连接”等未验证声明。
- 不删除现有索引条目。

### 4.3 验收

- README 中所有新增链接的文件真实存在。
- 抽查 5 个链接，确认路径正确。
- 操作日志记录修改前后对比。

## 5. 任务三：打 gim-v2.1 tag

- 确认任务一、二的 commit 已提交并推送。
- 确认 HEAD 指向包含 K-3 修正与 README 索引的 commit。
- 执行：

```bash
git tag -a gim-v2.1 -m "GIM V2.1: V2.0 plus architecture deep-dive, interview design Q&A, and audit corrections (A4/A7/A8/A9/A11/K-3); no functional change"
git push origin main
git push origin gim-v2.1
```

- `git ls-remote --tags origin` 确认远端同时有 gim-v1、gim-v1.1、gim-v1.2、gim-v1.3、gim-v1.4、gim-v2.0、gim-v2.1，前六个 peeled commit 未变。
- 操作日志记录 tag hash、target commit、push 结果、旧 tag 是否保持不变。

禁止：git tag -f 移动旧 tag、git push --force 覆盖远端 tag、删除已有 tag。

## 6. 执行顺序与 commit

```text
落盘 round-18.md
  -> 任务一修 K-3 -> commit（docs: correct stale friend online reminder note）
  -> 任务二 README 文档索引 -> commit（docs: index architecture and interview docs in readme）
  -> 任务三打 tag + push
  -> 全量汇总
```

每个 commit 前：git status、git diff --check、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

建议 commit message：

```text
docs: correct stale friend online reminder note

docs: index architecture and interview docs in readme

chore(release): tag gim-v2.1
```

不压成一个 commit。本轮不改业务代码，不重跑全量测试；但需执行 `go build ./...`、`git diff --check` 确认无意外变更。

## 7. 本轮结束输出

- K-3 修改前后对比；是否发现其他文档过期引用（列入待修正清单，不本轮改）。
- README 文档索引新增/整理项；抽查 5 个链接结果。
- commit hash 与 push 结果。
- tag gim-v2.1 指向的 commit hash；前六个 tag 是否未移动。
- `git ls-remote --tags origin` 输出摘要（七个 tag 的 peeled commit）。
- 操作日志清单。
- 未验证项与剩余问题（应与 v2-scope.md 一致）。
- 下一步建议：是否写演示脚本 / 是否清理 A1/A2 / 是否补 Group 1213 / 到此为止。

## 8. 遇到以下情况停止并问我

- K-3 修改需要连带改 architecture-deep-dive.md、design-questions.md、v2-scope.md、validation-status.md 中多处引用才能自洽。
- 发现其他“无好友上线推送”的过期表述，需要你决定是否本轮一并修。
- README 索引与现有目录结构冲突，需要重组文档目录。
- tag 指向的 commit 与 K-3 修正状态不一致。
- push/tag 失败（认证/网络/non-fast-forward）。
- 任何需要改业务代码才能让文档一致的情况。
