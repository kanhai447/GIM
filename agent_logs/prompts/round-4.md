# GIM 第四轮 — 项目整理（文档一致性、README、代码整洁、日志结构、Git 整洁）

## 0. 前提与授权

第三轮已完成：HEAD = origin/main = afdb552，工作区干净，集成测试 20 PASS / 1 SKIP，好友 E2E 10 步 PASS，gim-v1 tag 存在。

本轮目标：把项目整理到“文档与实现一一对应、无死代码、无过期声明、日志结构统一、Git 整洁”。不补验证、不打 tag、不进入 V2。

授权范围：

- 文档一致性核对与更新（描述性文档以实现为准）。
- 重写根 README 和三个子 README 为当前事实。
- 代码整洁：删除无调用方代码、未读取配置键、过期注释；删除前必须有 grep 证据，删除后必须 build/test PASS。
- agent_logs/ 结构统一与历史误计修正。
- Git 整洁检查：.gitignore、无密钥、无 reference/ 进入历史。
- 每个任务 commit + push；本轮不打 tag。

明确不做：

- 不进入 V2。
- 不补 race/Compose/Kafka 验证（留第五轮）。
- 不修改 docs/00_DECISIONS.md。若发现决策表与现状不符，只在描述性文档中记录差异，不擅自改决策表。
- 不修复业务缺陷；发现缺陷记录到问题清单，留第五轮或单独修复。
- 不放宽测试、不虚构结果、不 --force、不移动/删除旧 tag。

如果必须偏离，先停止并问我。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push 结果。
- 每个可独立验收操作写 agent_logs/operations/day-7/；没有记录视为未完成。
- 删除代码/配置前必须有 grep 证据；删除后必须 go build ./...、go vet ./...、go test ./... -count=1 PASS。
- 前端改动后必须 vue-tsc、vitest run、vite build PASS。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-4.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = afdb552、gim-v1 存在。

## 3. 任务一：文档一致性核对

### 3.1 HTTP 路由

- 提取 server/internal/*/transport/http/handler.go 中所有 rest.Route{Path: ...}。
- 与 docs/05_HTTP_API.md 逐条比对。
- 对未实现的路由，在 docs/05 中标注 “V2 / 未实现”，不删除条目。
- 对已实现但文档缺失的路由，补进 docs/05。
- 错误码表：确认 1113–1119 和 11xx 分配表存在，且与代码中的错误码一致。

### 3.2 数据库

- 提取 server/migrations/*.sql 中的表、索引、唯一约束。
- 与 docs/04_DATABASE_DESIGN.md 逐条比对。
- 不一致处：以实现为准更新文档；若发现实现缺索引/约束，记录到问题清单，不在本轮修。

### 3.3 WebSocket

- 提取 chat/ws、group/ws 中实际处理的事件名。
- 与 docs/06_WEBSOCKET_PROTOCOL.md 比对。
- 未实现的事件（如 friend.online/offline、富消息）标注 V2。

### 3.4 架构与拓扑

- 确认 docs/01_PROJECT_ARCHITECTURE.md 和 README.md 拓扑图无 group-rpc 残留。
- 确认 cmd/ 下实际入口与文档一致。
- 确认 compose.yaml 服务数与文档一致（当前应为 17）。

### 3.5 验收

- docs/05、04、06、01 与实现逐条一致，未实现项有 V2 标注。
- 操作日志记录比对结果和修改文件。

## 4. 任务二：README 重写

### 4.1 根 README.md

包含：项目介绍、V1 范围（已实现/未实现）、架构简图、技术栈、目录结构、环境要求、依赖启动、migration、各服务启动顺序、前端启动、默认配置、测试命令、验证状态（链接 docs/validation-status.md，本轮只建占位或写“待第五轮补”）、tag 说明（gim-v1 基线，gim-v1.1 待第五轮）、已知限制、GitHub 安全说明。

### 4.2 三个子 README

- server/README.md：目录说明、各 cmd 启动命令、测试命令、migration 命令；删除所有“later checkpoint”“remain later work”类表述。
- web/README.md：启动、构建、测试、页面清单（含好友/通讯录）；删除过期表述。
- admin/README.md：启动、构建、测试、页面清单；修正“Dashboard metrics remain intentionally empty”为已接 /api/settings/admin/overview。

### 4.3 验收

- 三个子 README 的启动命令与实际 cmd/、package.json 一致。
- 无过期表述。
- 操作日志记录修改文件。

## 5. 任务三：代码整洁

### 5.1 孤儿包与死代码

- 对 server/internal/ 每个子包执行 grep -r "<包名>" server/ --include=*.go，确认有调用方。
- 对无调用方的导出函数、未注册的 handler、未使用的类型，列出清单。
- 删除前记录 grep 证据；删除后 build/vet/test PASS。
- 如果删除影响面大或不确定，记录到问题清单，不删。

### 5.2 未使用配置键

- 对 .env.example、.env.compose.example、各服务 etc/*.yaml 逐键 grep 代码读取。
- 删除无读取的键；保留实际使用的键（如 USER_RPC_PORT）。
- 删除后 go build ./...、docker compose config --quiet PASS。

### 5.3 过期注释

- grep -r "TODO\|FIXME\|Day [0-9] checkpoint\|later work\|remain later" server/ web/ admin/。
- 能转为明确问题记录的，写入 docs/known-issues.md（新建）或现有问题清单；能直接删除的过期注释删除。
- 不删除有实际意义的 TODO，转为 known-issues 条目。

### 5.4 验收

- 无孤儿包、无未读取配置键、无过期 checkpoint 注释。
- go build/vet/test、前端 build/test PASS。
- 操作日志记录删除清单、grep 证据、测试结果。

## 6. 任务四：agent_logs/ 结构统一

- 检查 day-1 到 day-7 目录命名、文件编号是否一致。
- CURRENT_STATE.md 只写当前状态，不写历史；历史放 day-N.md 和 operations/。
- 修正历史误计（如 Compose 服务数、commit 数），确保 day-N.md 与 operations/ 一致。
- 确认每份操作记录包含：目标、修改文件、命令、结果、测试、问题、commit、push 状态。
- 不删除历史记录；只修正事实错误，并在修正处注明“修正于 day-7”。

## 7. 任务五：Git 整洁

- .gitignore 确认覆盖 .env.local、.env.compose.local、.tmp/、node_modules/、构建产物、reference/、上传临时目录。
- git log --diff-filter=A --name-only 确认历史无 .key、.pem、reference/ 被加入。
- git grep -n "BEGIN .*PRIVATE KEY\|AccessSecret\|password.*=.*[^C]HANGE_ME" 确认 HEAD 无密钥。
- scripts/check-repository-safety.ps1 PASS。
- 确认 reference/ 未被跟踪。
- 操作日志记录检查命令与结果。

## 8. 执行顺序与 commit

```text
落盘 round-4.md
  -> 任务一文档一致性 -> commit（docs: align contracts with implementation）
  -> 任务二 README -> commit（docs: rewrite readmes to current state）
  -> 任务三代码整洁 -> commit（chore: remove dead code and unused config）
  -> 任务四 agent_logs -> commit（docs(logs): normalize operation logs）
  -> 任务五 Git 整洁 -> commit（chore: verify repository hygiene）
  -> 全量汇总
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit。本轮不打 tag。

## 9. 本轮结束输出

- 已完成/未完成。
- 文档一致性比对结果：哪些一致、哪些补了 V2 标注、哪些以实现为准更新。
- 删除的代码/配置清单 + grep 证据。
- README 修改文件。
- agent_logs 修正项。
- Git 整洁检查结果。
- 实际测试命令与结果。
- 操作日志清单。
- commit hash 与 push 结果。
- 问题清单（发现但未修的业务缺陷、待第五轮验证项）。
- 下一步建议。

## 10. 遇到以下情况停止并问我

- 文档与实现冲突，且无法判断以实现为准还是文档为准。
- 删除代码影响面大或不确定是否有调用方。
- 发现业务缺陷，修复超出“整理”范围。
- docs/00_DECISIONS.md 与现状冲突，需要你决定是否记录差异。
- push 失败（认证/网络/non-fast-forward）。
- 测试出现 FAIL 且原因不明。
