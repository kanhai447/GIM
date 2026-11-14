# GIM 第十二轮 — 撤回收尾 + 好友上线提醒协议设计 + gim-v1.3 tag

## 0. 前提与授权

第十一轮已完成：撤回消息 type=4 后端/授权/前端/文档四个 commit 已推送；HEAD = origin/main = e3e37fd，工作区干净；gim-v1/gim-v1.1/gim-v1.2 未移动；未打新 tag。

本轮目标：

- 撤回收尾：并发撤回单元测试（K-14）+ 撤回窗口短配置 E2E + 撤回代码全量 race。
- 好友上线提醒协议设计（文档先行），不写实现。
- 打 annotated tag gim-v1.3，指向本轮最终通过验收的 commit，并 push。

本轮明确不做：

- 不实现好友上线提醒后端/前端代码。
- 不实现 user_confs、管理员撤回、语音/视频/@/图文混排。
- 不新增表/列/索引；撤回收尾阶段不改 schema。
- 不重构 Hub/Client、幂等、session、Presence、好友闭环、图片/文件/撤回实现。
- 不放宽测试、不虚构结果、不 --force、不移动/删除已有 tag、不打 gim-v2.0。

如果必须偏离（例如 K-14 并发撤回需要加列才能修复），先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push、tag 结果；环境不可用记录 NOT RUN/SKIP。
- 每个可独立验收操作写 agent_logs/operations/day-15/；没有记录视为未完成。
- commit 前 git status、git diff --check、对应测试、确认无 .env/reference//密钥进入 staged。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-12.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = e3e37fd、三个 tag 存在。

## 3. 任务一：撤回收尾

### 3.1 并发撤回单元测试（K-14）

背景：无唯一目标索引下，并发不同 clientMsgId 撤回同一消息可能产生两条撤回消息。顺序幂等已测试，并发未测。

要求：

- 写一个并发测试：两个 goroutine 同时用不同 clientMsgId 撤回同一 msgID，断言：
  - 不 panic、不返回系统错误；
  - 历史查询返回的占位数量与前端渲染逻辑一致（不重复渲染、不崩溃）；
  - 授权查询语义不受影响（撤回后非所有者下载仍被拒）；
  - 如果产生两条撤回消息，记录实际行为，不强行断言“只有一条”。
- 如果测试发现历史重复渲染或前端崩溃，先停止报告，评估是否需要加列/唯一索引；不要擅自改 schema。
- 把实际行为写入 docs/known-issues.md K-14，明确“当前为已知边界，修复需评估 schema 变更”。

### 3.2 撤回窗口短配置 E2E

- 设置 RECALL_WINDOW_SECONDS=2（或更短），重启 chat/group 服务。
- 浏览器 E2E：
  - A 发送消息；
  - 立即撤回 → 成功；
  - A 再发送一条，等待超过窗口（例如 3 秒）→ 前端撤回入口消失或点击后返回 2220/2421；
  - B 看到撤回占位，原文不可见。
- 记录真实输出（脱敏）。
- 环境不可用则记录 NOT RUN。
- 完成后恢复默认配置（120s），确认服务重启正常。

### 3.3 撤回代码全量 race

- Linux 容器 golang:1.25-bookworm，挂载 module cache，GOPROXY=off。
- `go test -race ./... -count=1`。
- 重点覆盖 internal/chat/...、internal/group/...、internal/file/...、internal/user/friend/...。
- 记录 exit code、是否有 WARNING: DATA RACE。
- 发现 DATA RACE 先停止报告，不扩大修复范围。

### 3.4 验收

- 并发撤回测试 PASS 或记录实际行为并写入 known-issues。
- 窗口 E2E PASS 或 NOT RUN（附原因）。
- race PASS 或 NOT RUN（附原因）。
- 操作日志记录每步命令与结果。

## 4. 任务二：好友上线提醒协议设计（文档先行，不写实现）

### 4.1 阅读

- `server/internal/chat/hub.go`、`client.go`、`presence/`：Chat Hub 0→1/1→0 的现有处理。
- `server/internal/user/friend/`：好友列表查询、isOnline 读取 Redis Presence 的方式。
- `web/src/stores/friend.ts`、`api/friend.ts`：好友列表状态管理。
- docs/06 现有事件、docs/08 Redis Presence key、docs/07 登录/断线流程。

### 4.2 设计

在 docs/06_WEBSOCKET_PROTOCOL.md 新增“好友上线提醒”小节，至少包含：

- 事件：`friend.online`、`friend.offline`，由 Chat WS 推送（Group WS 不参与）。
- Payload：

```json
{"event":"friend.online","data":{"userID":1002,"at":"2026-11-14T12:00:00Z"}}
```

`friend.offline` 同构。

- 触发条件：
  - `friend.online`：Chat Hub 该用户连接数 0→1，且该用户有在线好友；
  - `friend.offline`：Chat Hub 该用户连接数 1→0。
- 通知范围：只通知当前用户的好友，不广播；好友关系来自 `friends` 表。
- 与 Presence 存储的关系：Redis `gim:presence:user:{uid}` 是权威存储，WS 事件只是通知；事件丢失时前端可通过 `GET /api/user/friends` 的 `isOnline` 恢复。
- 多端：同一用户多个连接时，只在该用户首个连接建立/最后一个连接断开时触发，避免多端误判。
- 离线事件：V1 是否必须实现 `friend.offline`，还是只做 `friend.online`？建议两者都设计，实现时按优先级。
- 错误/边界：好友关系不存在、被删除好友、用户被封禁时不推送。
- 兼容性：旧客户端忽略未知事件，不崩溃。
- 实现约束：不得让 Group WS 修改 Presence；不得在 Hub 事件循环内做网络 I/O；通知通过现有写队列异步发送。

同时更新：

- docs/05：好友列表 `isOnline` 仍是 HTTP 读取，WS 事件只是实时补充。
- docs/07：新增“好友上线提醒”流程（登录 → Chat WS 0→1 → 查好友 → 异步推送 friend.online）。
- docs/08：确认 Presence key 与 TTL 规则不变。
- docs/known-issues：新增“好友上线提醒协议已定义，实现待下一轮”。
- docs/validation-status：新增“好友上线提醒协议设计”状态（设计完成，实现 NOT RUN）。
- README：V2 进度，明确“好友上线提醒协议已定义，实现待下一轮”。

### 4.3 验收

- 协议文档先于实现提交。
- 操作日志记录设计决策、通知范围、多端语义、与 Presence 的关系。
- 如果发现现有 Chat Hub 无法在不改并发模型的情况下支持好友通知，先停止报告，不要擅自重构 Hub。

## 5. 任务三：打 gim-v1.3 tag

- 确认任务一、二的 commit 已提交并推送。
- 执行：

```bash
git tag -a gim-v1.3 -m "GIM V1.3: image/file/recall messages implemented and validated; friend online reminder protocol defined, implementation pending"
```

指向本轮最终通过验收的 commit。

- `git push origin main`，再 `git push origin gim-v1.3`。
- `git ls-remote --tags origin` 确认远端同时有 gim-v1、gim-v1.1、gim-v1.2、gim-v1.3，前三个 hash 未变。
- 操作日志记录 tag hash、push 结果、旧 tag 是否保持不变。

禁止：`git tag -f` 移动旧 tag、`git push --force` 覆盖远端 tag、删除已有 tag。

## 6. 执行顺序与 commit

```text
落盘 round-12.md
  -> 任务一撤回收尾（并发测试 + 窗口 E2E + race）-> commit（如有测试/文档变更）
  -> 任务二好友上线提醒协议设计 -> commit（docs: define friend online reminder protocol）
  -> 任务三打 tag + push
  -> 全量汇总
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

建议 commit message：

```text
test(chat): cover concurrent recall boundary

docs: define friend online reminder protocol

chore(release): tag gim-v1.3
```

不压成一个 commit。

## 7. 本轮结束输出

- 撤回收尾：并发测试结果、窗口 E2E 结果、race 结果（PASS/FAIL/SKIP/NOT RUN + 原因）。
- K-14 实际行为与 known-issues 更新。
- 好友上线提醒协议摘要：事件、payload、触发条件、通知范围、多端语义、与 Presence 关系、兼容性。
- 是否发现需要改 Chat Hub 并发模型；如有，报告影响与建议。
- 文档修改清单。
- 操作日志清单。
- commit hash 与 push 结果。
- tag gim-v1.3 指向的 commit hash；前三个 tag 是否未移动。
- 未验证项与剩余问题。
- 下一步建议：好友上线提醒实现 / user_confs / 性能 / 是否准备 gim-v2.0。

## 8. 遇到以下情况停止并问我

- 并发撤回测试发现历史重复渲染或前端崩溃，需要加列/唯一索引。
- 窗口短配置 E2E 发现服务端与前端窗口不一致，需要产品语义决定。
- race 发现 DATA RACE，修复超出撤回收尾范围。
- 好友上线提醒协议需要改 Chat Hub 并发模型或 Presence 写入方。
- 好友通知范围/多端语义需要产品决定（例如是否通知离线好友、是否只通知“互相好友”）。
- tag push 失败（认证/网络/non-fast-forward）。
- 环境不可用，是否允许本轮以“协议设计 + 文档 PASS，E2E/race NOT RUN”收尾。
