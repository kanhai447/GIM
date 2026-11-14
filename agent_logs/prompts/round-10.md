# GIM 第十轮 — 文件 Phase 收尾 + 撤回协议设计（不实现撤回代码）

## 0. 前提与授权

第九轮已完成：gim-v1.2 tag 已创建并推送；普通文件消息 type=3 闭环完成并推送；HEAD = origin/main = 4c4a2cc，工作区干净；gim-v1/gim-v1.1/gim-v1.2 均未移动；未打 gim-v2.0。

本轮目标：

- 补文件消息 Phase 的验证缺口：群聊文件浏览器 E2E + 全量 race。
- 只做撤回的协议设计（文档先行），不写撤回实现代码。
- 更新 docs/05/06/07、docs/known-issues.md、docs/validation-status.md、README.md。

本轮明确不做：

- 不实现撤回后端/前端代码。
- 不实现好友上线提醒、user_confs、语音/视频/@/图文混排。
- 不新增表/列/索引；撤回协议设计阶段不涉及 migration。
- 不重构 Hub/Client、幂等、session、Presence、好友闭环、图片/文件消息实现。
- 不放宽测试、不虚构结果、不 --force、不移动/删除 tag、不打新 tag。

如果必须偏离，先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push 结果；环境不可用记录 NOT RUN/SKIP。
- 每个可独立验收操作写 agent_logs/operations/day-13/；没有记录视为未完成。
- commit 前 git status、git diff --check、对应测试、确认无 .env/reference//密钥进入 staged。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-10.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = 4c4a2cc、三个 tag 存在。

## 3. 任务一：全量 race（Linux 容器）

使用与第八轮相同的 Linux 容器（golang:1.25-bookworm，挂载 module cache，GOPROXY=off）。

执行 `go test -race ./... -count=1`。

记录：镜像、命令、exit code、是否有 WARNING: DATA RACE。

重点覆盖 `internal/chat/...`、`internal/group/...`、`internal/file/...`、`internal/user/friend/...`。

如果发现 DATA RACE，先停止报告，不要擅自扩大修复范围。

环境不可用则记录 NOT RUN 及真实原因，不虚构。

## 4. 任务二：群聊文件浏览器 E2E

Compose 全栈可用时执行；不可用则记录 NOT RUN。

步骤（与第八轮群聊图片 E2E 对齐，替换为文件消息）：

- 注册 A、B、C、D；
- A 建群并邀请 B、C；
- A 上传 `report.pdf` 并通过 `group.send` 发送 type=3；
- B、C 收到 `group.message`，可通过下载按钮下载；
- D（非成员）下载被拒（`2503`）；
- B 退群后下载该群历史文件被拒（`2503`）；
- A（所有者）下载仍成功；
- 记录每步真实输出（脱敏）。

如果 E2E 发现缺陷，先停止报告根因与最小修复方案，等我确认后再改。

注意浏览器缓存问题（known-issues E-9）：E2E 下载使用 `cache: 'no-store'`，避免假失败。

## 5. 任务三：撤回协议设计（文档先行，不写实现）

### 5.1 阅读

- `server/internal/chat/message/`、`server/internal/group/message/` 中现有消息类型（type=1/2/3）的解析、持久化、历史 DTO、幂等实现。
- `server/internal/file/` 下载授权查询（`imageMsg.fileID` / `fileMsg.fileID`）。
- docs/06 现有消息类型表、docs/05 下载授权小节。

确认现有 msg JSON 结构是否有空间容纳 type=4 撤回消息，是否需要新增列（如 `recalled_at`）或仅用特殊消息表达。

### 5.2 在 docs/06_WEBSOCKET_PROTOCOL.md 新增“撤回消息”小节

必须包含：

- 类型：`msg.type = 4`，payload：

```json
{"type":4,"withdrawMsg":{"msgID":123}}
```

`msgID` 指向被撤回的原消息。

- 发送方式：沿用 `chat.send` / `group.send`，不新增独立 WS 事件；服务端校验后落库一条撤回消息，并推送 `chat.message` / `group.message`。
- 校验：
  - 原消息存在；
  - 发送者是当前用户（管理员撤回另议，本轮不设计管理员撤回）；
  - 在时间窗口内（`RECALL_WINDOW_SECONDS`，默认 120，可配置）；
  - 原消息未被撤回、未被 tombstone；
  - 群聊需校验当前用户仍是群成员。
- 错误码：Chat/Group 各自分配新码，写入错误码表，不冲突。至少区分：消息不存在、非本人消息、超过时间窗口、已撤回、群成员校验失败。
- 幂等：同一 msgID 重复撤回返回明确结果，不重复插入撤回消息。
- 历史 DTO：
  - 被撤回的原消息在历史中替换为撤回占位（type=4 或 recalled=true），不返回原内容；
  - 撤回消息本身在历史中可见；
  - 前端收到撤回消息后，按 msgID 定位并替换本地原消息。
- 文件授权联动：撤回后，原消息引用的文件不再授权非所有者下载；所有者仍可下载自己的 user_files。这与 tombstone 语义一致。
- 兼容性：type=1/2/3 不变；旧客户端收到 type=4 显示“消息已撤回”占位。
- 时间窗口：默认 120 秒，配置项 `RECALL_WINDOW_SECONDS`，写入 `.env.example` 和文档；测试环境可设短值。
- 管理员撤回：本轮不设计，标注 V2 后续；如需要，先停止问我。

### 5.3 更新其他文档

- `docs/05_HTTP_API.md`：错误码表新增撤回相关码；下载授权小节补充“撤回后的文件不再授权非所有者”。
- `docs/07_CORE_FLOWS.md`：新增“撤回流程”小节（发送 → 校验 → 落库 → ACK → 推送 → 历史替换 → 文件授权失效）。
- `docs/04_DATABASE_DESIGN.md`：说明撤回是否复用 msg JSON（建议复用，不新增列）；preview 显示 `[撤回]`。
- `docs/known-issues.md`：K-12（撤回后授权待 V2）改为“协议已定义，实现待第十一轮”；如需要新增列，标注“实现阶段评估”。
- `docs/validation-status.md`：新增“撤回协议设计”状态（设计完成，实现 NOT RUN）。
- `README.md`：V2 进度，明确“撤回协议已定义，实现待下一轮”。

### 5.4 验收

- 协议文档先于实现提交。
- 操作日志记录设计决策、与现有结构的兼容性、文件授权联动的语义。
- 如果发现现有 msg JSON 结构无法容纳撤回（例如需要 recalled_at 列才能正确过滤历史），先停止报告，不要擅自改 schema。

## 6. 执行顺序与 commit

```text
落盘 round-10.md
  -> 任务一 race -> 操作日志
  -> 任务二群聊文件 E2E -> 操作日志（如有修复则 commit）
  -> 任务三撤回协议设计 -> commit（docs: define recall message protocol）
  -> 文档与验证状态更新 -> commit（docs: record file phase closure and recall protocol）
  -> push
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit；本轮不打 tag。

## 7. 本轮结束输出

- race：PASS/FAIL/NOT RUN + 环境。
- 群聊文件 E2E：PASS/FAIL/NOT RUN + 步骤摘要。
- 撤回协议摘要：类型、payload、校验、错误码、历史 DTO、文件授权联动、时间窗口、兼容性。
- 是否发现需要 schema 变更；如有，报告影响与建议，不擅自修改。
- 文档修改清单。
- 操作日志清单。
- commit hash 与 push 结果。
- 未验证项与剩余问题。
- 下一步建议：撤回实现范围、好友上线提醒、user_confs、性能。

## 8. 遇到以下情况停止并问我

- race 发现 DATA RACE，修复超出本轮范围。
- 群聊文件 E2E 发现缺陷，修复涉及 Hub/广播/session 重构。
- 撤回协议需要新增表/列/索引才能正确实现。
- 撤回协议与现有 msg JSON 结构冲突。
- 撤回的文件授权语义与图片/文件消息现有授权查询冲突。
- 环境不可用，是否允许本轮以“race/E2E NOT RUN，协议设计 + 文档 PASS”收尾。
- push 失败（认证/网络/non-fast-forward）。
