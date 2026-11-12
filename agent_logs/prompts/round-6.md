# GIM 第六轮 — 补推 main 与 gim-v1.1 tag，确认远端一致

## 0. 前提与授权

第五轮已完成：race、Compose 全栈、Kafka 真实消费、浏览器好友 E2E、性能两档均真实 PASS；docs/validation-status.md、docs/known-issues.md、README.md 已更新；本地 annotated tag gim-v1.1 已创建，指向 55bbb8e。

当前状态：HEAD = 76d2032，origin/main = bce78cc，本地领先 2 个 commit；gim-v1.1 仅本地；gim-v1 未移动。push 因网络不可达失败，已如实记录。

本轮目标：网络恢复后补推 main 与 gim-v1.1，确认远端状态一致；若仍失败，如实记录并保留本地 tag。

授权范围：

- git fetch 确认远端无新提交。
- 补推 main 和 gim-v1.1。
- 用 git ls-remote 确认远端 tag 与 main 状态。
- 更新 agent_logs/CURRENT_STATE.md 的 push 状态，写 agent_logs/operations/day-9/ 记录。

明确不做：

- 不进入 V2。
- 不改业务代码，不改文档内容（除 push 状态记录外）。
- 不打新 tag，不移动/删除 gim-v1 或 gim-v1.1。
- 不 --force，不自动 merge/rebase 远端新提交。
- 不虚构 push/tag 结果。

如果必须偏离，先停止并问我。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push、tag 结果。
- 每个可独立验收操作写 agent_logs/operations/day-9/；没有记录视为未完成。
- push 失败记录真实错误，不 --force，不删除/移动远端 tag。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-6.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag、git remote -v，确认：
  - 工作区干净；
  - HEAD = 76d2032（以实际为准）；
  - gim-v1 和 gim-v1.1 均存在；
  - origin 指向 github.com/kanhai447/GIM.git。

## 3. 任务一：fetch 并确认远端状态

- 执行 git fetch origin。
- 执行 git log --oneline origin/main..HEAD，确认待推送 commit 为 55bbb8e、76d2032（以实际为准）。
- 执行 git log --oneline HEAD..origin/main，确认远端没有本地缺失的新提交。
- 如果远端有本地缺失的新提交：
  - 停止，输出远端差异，不要自动 merge/rebase，等我决定。
- 如果 fetch 失败（网络/认证）：
  - 记录真实错误；
  - 不继续 push；
  - 保留本地状态；
  - 写操作日志；
  - 结束本轮并报告。

## 4. 任务二：补推 main 与 tag

### 4.1 推送 main

- 确认工作区干净、git diff --check 无输出。
- 执行 git push origin main。
- 记录真实输出：
  - 成功：记录 old..new main -> main；
  - 失败：记录完整错误，不重试 --force。

### 4.2 推送 tag

- 若 main 推送成功，执行 git push origin gim-v1.1。
- 记录真实输出：
  - 成功：记录 tag 推送结果；
  - 失败：记录完整错误，保留本地 tag。
- 如果 main 推送失败，不要继续推 tag；先报告 main 失败原因。

### 4.3 验收

- git log --oneline origin/main..HEAD 为空，或记录真实失败。
- git ls-remote --tags origin 输出中同时包含 gim-v1 和 gim-v1.1，且 gim-v1 hash 仍为 09994cb。
- git ls-remote origin refs/heads/main 与本地 HEAD 一致，或记录真实差异。

## 5. 任务三：更新状态与操作日志

- 更新 agent_logs/CURRENT_STATE.md：
  - 写明 HEAD、origin/main、远端 tag 状态；
  - 若 push 成功，写“已推送”；若失败，写真实错误与待重试命令。
- 新建 agent_logs/operations/day-9/001_task-1_push-main-and-tag.md，记录：
  - 目标；
  - 执行的命令；
  - 真实输出摘要；
  - 成功/失败；
  - commit hash、tag hash；
  - 下一步。
- 若本轮有状态文件修改，提交一个 commit：
  - 建议 message：docs(day9): record push status for main and gim-v1.1
  - 提交前 git status、git diff --check，确认只含状态记录文件。
- 如果 push 仍失败，该 commit 也仅本地，记录在操作日志中。
- 若 push 成功，再尝试推送该状态 commit；若失败，记录真实错误，不 --force。

## 6. 执行顺序

```text
落盘 round-6.md
  -> 任务一 fetch + 确认远端
  -> 远端有新提交：停止报告
  -> 任务二 push main -> push tag
  -> 任务三更新 CURRENT_STATE + 操作日志 + 可选状态 commit
  -> 汇总
```

## 7. 本轮结束输出

- fetch 结果：远端是否有新提交。
- git push origin main 真实结果：成功/失败，错误原文。
- git push origin gim-v1.1 真实结果：成功/失败，错误原文。
- git ls-remote --tags origin 输出：远端是否存在 gim-v1、gim-v1.1，hash 是否与本地一致。
- origin/main 与本地 HEAD 是否一致。
- 本地 gim-v1 是否仍为 09994cb，未被移动。
- 操作日志路径。
- 如有状态 commit，记录 hash 与 push 状态。
- 下一步建议：若 push 成功，可进入 V2 或补完整性能；若仍失败，保留本地并等待网络恢复。

## 8. 遇到以下情况停止并问我

- git fetch 发现远端有本地缺失的新提交。
- push 出现 non-fast-forward。
- push 认证失败，需要调整远端/凭据。
- 远端已存在 gim-v1.1 且 hash 与本地不同。
- 发现 gim-v1 被误移动或远端 tag 冲突。
- 任何需要 --force 才能继续的情况。
