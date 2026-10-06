# 14 Agent 操作日志模板

Agent 必须在项目根目录维护 `agent_logs/`。其中 `agent_logs/operations/` 是逐操作强制记录目录；以下模板均为项目交付物。

## agent_logs/AGENT_WORKLOG.md

```markdown
# GIM Agent Worklog

## [YYYY-MM-DD HH:mm] Day N / Phase X - <标题>

### 目标
- ...

### 开始状态
- branch: `...`
- HEAD: `...`
- working tree: clean / 有未提交项（说明来源）

### 修改
- `path/file.go`: ...
- `path/file.vue`: ...

### 数据/协议变化
- DB: ... / 无
- HTTP: ... / 无
- WebSocket: ... / 无
- Redis: ... / 无

### 关键操作
- `<command>` -> 成功/失败，关键输出摘要
- `<command>` -> ...

### 问题与修复
- 现象：...
- 根因：...
- 修复：...
- 验证：...

### 测试
- `go test ./...` -> PASS/FAIL（摘要）
- `go test -race ...` -> PASS/FAIL
- frontend build -> PASS/FAIL

### Git
- commit: `<hash> <subject>`
- push: `origin/<branch>` -> 成功/失败
- tag: `...` / 无

### 遗留
- ...

### 下一步
- ...
```

## agent_logs/day-N.md

```markdown
# Day N

## 今日目标
- ...

## 完成项
- [x] ...
- [ ] ...（原因）

## Phase 记录
### Phase X
- 开始：HH:mm
- 结束：HH:mm
- 主要改动：...
- 测试：...
- commit：...
- push：...

## 当日问题
1. ...

## 当日验收
- build: PASS/FAIL
- tests: PASS/FAIL
- race: PASS/FAIL/不适用
- frontend: PASS/FAIL/不适用
- GitHub checkpoint: SUCCESS/FAILED

## 次日入口
- ...
```

## 日志质量要求

好的日志应该回答：

- Agent 今天实际改了什么？
- 为什么这样改？
- 出过什么错，根因是什么？
- 用什么命令验证？
- 哪个 commit 可以回滚？
- GitHub 上哪个 checkpoint 对应当前阶段？

日志不是聊天记录，不要把大段思考过程、重复命令或敏感数据写进去。
