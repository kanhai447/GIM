# 13 Git 与 GitHub 阶段交付规则

目标：让 GIM 在四天开发过程中始终存在可回滚、可审计、可展示的 Git 历史，并把通过验收的阶段性版本上传到 GitHub。

## 1. 初始化检查

Agent 在第一次修改代码前执行并记录：

```bash
git status
git branch --show-current
git remote -v
git log --oneline -n 5
```

如果当前目录尚未初始化 Git，可在 GIM 新项目根目录初始化；`reference/fim_*` 是否纳入仓库需遵循用户仓库安排，默认不把无关大体积历史再次嵌套提交。

如果已有 Git 仓库，不得删除 `.git`、不得重新 `git init` 覆盖现状。

## 2. 分支策略

优先使用现有仓库约定。若没有约定，建议：

```text
main                 稳定/用户控制
dev/gim-v1           四天开发工作分支
```

Agent 默认在 `dev/gim-v1` 工作并阶段性 push。除非用户明确授权，不自动 merge/push 到 `main`。

## 3. 提交粒度

- 一个可独立解释和验收的变更一个 commit。
- 不允许一个 commit 同时混入无关格式化、重命名和核心业务重构。
- 每个 Phase 至少一个 commit。
- commit 前必须先看 `git diff --stat` 和 `git diff`/等价审阅。

推荐 Conventional Commits：

```text
chore(day1): establish runnable GIM baseline
feat(db): add chat and group session schema
feat(chat): add websocket hub and client pumps
feat(chat): add ack and idempotent message send
feat(group): add reliable group websocket messaging
feat(file): stream uploads and hash deduplication
feat(web): integrate websocket ack and reconnect
fix(chat): prevent duplicate unread increments
chore(release): add tests docs and v1 guide
```

## 4. 每日 checkpoint

至少：

- Day 1 -> push stable baseline -> tag `gim-day1-baseline`
- Day 2 -> push private chat stable -> tag `gim-day2-chat`
- Day 3 -> push group/file stable -> tag `gim-day3-group-file`
- Day 4 -> push V1 -> tag `gim-v1`

Tag 只能指向已通过当天最低验收的 commit。测试失败时可以 commit WIP 修复过程，但不得打稳定 tag。

## 5. Push 前检查

每次 push 前执行并记录：

```bash
git status
git diff --check
```

并执行当前阶段测试。还需确认：

- `.env` 未被跟踪；
- GitHub Token/JWT/密码/私钥未出现在 staged diff；
- `node_modules/`、上传临时目录、数据库 volume、日志垃圾文件未被误提交；
- 操作日志中的敏感值已脱敏；
- 本次已完成操作在 `agent_logs/operations/day-N/` 中存在对应记录，且当日/总日志已同步；
- 文档与当前协议一致。

可使用 `git diff --cached` 审查 staged 内容。

## 6. Push 规则

推荐：

```bash
git push -u origin dev/gim-v1
```

后续：

```bash
git push origin dev/gim-v1
git push origin <tag>
```

Agent 必须记录实际 remote、branch、commit hash 和 push 输出。

禁止：

```text
git push --force
git push -f
擅自删除远端 branch/tag
擅自重写 main 历史
```

如果遇到 non-fast-forward，先 fetch 并分析差异；不得强推覆盖用户提交。

## 7. GitHub 不可用时

以下情况 Agent 不得伪造完成：

- 没有 remote；
- GitHub 未认证；
- 网络不可用；
- 无 push 权限；
- 远端存在冲突需用户决策。

此时：

1. 完成本地测试；
2. 完成本地 commit；
3. 在日志写明 push 命令、错误和当前 commit hash；
4. 告知用户需要处理的最小事项；
5. 不删除本地 checkpoint。

## 8. 回滚与修复

- 优先新增修复 commit，不通过重写历史“假装从未出错”。
- 对已 push 的共享 commit，使用 `git revert` 或正常修复 commit。
- 不对用户未提交改动执行 destructive reset/checkout。

## 9. README 中的 GitHub 信息

项目最终 README 可以包含仓库结构、开发里程碑和稳定 tag，但不得暴露 token、私有 remote 凭证或内部账号信息。

---

# 三仓库复审后的 GitHub 安全门禁

原 `fim_server` 中存在不应进入新仓库历史的 TLS 私钥和硬编码凭证/Secret。由于 GIM 不迁移 FIM 源码，`reference/` 默认只在本地且加入 `.gitignore`；GIM 只提交自主编写的代码和脱敏配置。

至少检查：

```text
*.key
*.pem
AccessSecret
DataSource 中的真实密码
第三方 app secret/key
真实公网/内网生产地址
.env 中真实凭证
Token/JWT 样本
```

允许提交：`.env.example`、脱敏 YAML 示例、开发默认假值。

`reference/` 默认建议本地保留并加入 `.gitignore`，避免把原项目大仓库和历史敏感文件再次嵌套推到你的 GIM GitHub。原则上不要把 reference 入仓；如用户明确要求，也必须先彻底脱敏，并且 reference 仍不能作为 GIM 源码目录被编译或发布。
