# GIM 第十四轮 — 打 gim-v2.0 + V2 收口评估文档

## 0. 前提与授权

第十三轮已完成：好友上线提醒（单机）后端/前端/测试/文档均实现并验证；全量 race PASS；HEAD = origin/main = abc3e72，工作区干净；gim-v1/gim-v1.1/gim-v1.2/gim-v1.3/gim-v1.4 存在且未移动。

本轮目标：

- 打 annotated tag gim-v2.0，语义为“V2 核心四个 Phase 完成”，指向本轮最终功能/文档 commit。
- 新建 docs/v2-scope.md，明确 V2 已实现范围、V2 后期/不做项及理由、已知边界。
- 更新 README.md、docs/validation-status.md、docs/known-issues.md、CURRENT_STATE 相关引用，使 V2 范围表述一致。
- push main 与 tag。

本轮明确不做：

- 不实现新功能（user_confs、管理员撤回、多实例 Presence、好友申请实时推送、图片/文件吞吐性能均不做）。
- 不改业务代码；只改文档与 tag。
- 不重构、不重跑全量测试（如无代码变更）；只做必要的文档一致性检查。
- 不放宽测试、不虚构结果、不 --force、不移动/删除已有 tag。

如果必须偏离，先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push、tag 结果。
- 每个可独立验收操作写 agent_logs/operations/day-17/；没有记录视为未完成。
- commit 前 git status、git diff --check、确认无 .env/reference//密钥进入 staged。
- tag 指向已推送的 commit，且该 commit 的验证状态与 tag message 一致。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-14.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = abc3e72、五个旧 tag 存在。

## 3. 任务一：V2 收口评估文档

### 3.1 新建 docs/v2-scope.md

内容必须包含：

#### 3.1.1 V2 已实现（核心四 Phase）

| Phase | 类型 | 关键实现 | 验证证据 |
|---|---|---|---|
| 图片消息 | `msg.type=2` | 上传/发送/渲染/预览；复用 `file_objects`/`user_files`；授权联动 | 集成 + race + 群聊 E2E |
| 普通文件消息 | `msg.type=3` | 与图片同构；`FILE_MESSAGE_MAX_BYTES` | 集成 + race + 群聊 E2E |
| 撤回 | `msg.type=4` | 时间窗口 `RECALL_WINDOW_SECONDS`；历史替换；文件授权失效 | 集成 + 窗口 E2E + 并发测试 + race |
| 好友上线提醒（单机） | WS `friend.online`/`friend.offline` | observer + 有界 worker；只读 Presence；多端 0→1/1→0 | 集成 + E2E + race |

#### 3.1.2 V2 后期 / 不做

| 项目 | 状态 | 理由 |
|---|---|---|
| user_confs 读写 | 不做（V2 后期） | 偏好设置，面试价值低；不影响核心 IM |
| 管理员撤回 | 不做（V2 后期） | 需要独立的 Admin 撤回语义与审计；非核心 |
| 多实例 Presence 全局判断 | 不做（V2 后期） | 需要 Redis 全局 0→1/1→0 判断 + 多实例环境；当前单机语义已文档化 |
| 好友申请实时推送 | 不做（V2 后期） | 当前好友申请走 HTTP；WS 推送属体验优化 |
| 图片/文件吞吐与预览性能 | 不做（V2 后期） | 已有两档最小性能证据；完整阶梯属容量评估 |
| 语音/视频/@/图文混排 | 不做（V3 候选） | 工作量大，非当前目标 |
| WS Ticket、Gateway 本地验签 | 不做（V2 后期） | 当前 query token + Auth 链路已满足 V1/V2 语义 |
| Kafka outbox / 多实例 WS 路由 | 不做（V2 后期） | 架构扩展，需基础设施支持 |

#### 3.1.3 已知边界（引用 known-issues.md）

- K-14：并发不同 clientMsgId 撤回同一消息可能产生两条撤回消息；历史占位唯一、前端去重、授权不受影响；严格“一条”需 schema 决策。
- K-15：好友上线提醒单机语义；多实例可能重复/误报。
- K-16：offline 事件 best-effort；进程崩溃/TTL 过期不保证发送。
- E-8：文件授权查询 JSON_EXTRACT / NOT EXISTS 无索引；仅非所有者下载路径；无速率限制。
- E-9：浏览器已缓存副本不回收。

#### 3.1.4 验证状态摘要

链接 docs/validation-status.md，只列摘要：

- 单元测试、集成测试、前端测试、race、Compose 全栈、Kafka、E2E：PASS（附日期）。
- 完整性能阶梯、soak/restart、远端 CI：NOT RUN / NOT OBSERVED。

不写“全量验证通过”。

#### 3.1.5 tag 语义

| tag | 指向 | 语义 |
|---|---|---|
| gim-v1 | 上一轮基线 | 文本 IM + 文件上传 + Admin + 审计 |
| gim-v1.1 | 项目整理 | 好友闭环 + C1/C2 修复 + 验证状态 |
| gim-v1.2 | 图片 Phase 1 | 图片消息 + 授权收尾 |
| gim-v1.3 | 文件 + 撤回收尾 | 图片 + 文件 + 撤回 + 撤回收尾 |
| gim-v1.4 | 好友上线提醒 | 图片 + 文件 + 撤回 + 好友上线提醒（单机） |
| gim-v2.0 | 本轮 | V2 核心四 Phase 完成；V2 后期项明确列出 |

tag 表格里的 hash 以实际为准，不硬编码错误值。

### 3.2 验收

- docs/v2-scope.md 内容与 docs/validation-status.md、docs/known-issues.md、README.md 一致。
- 不出现“全量验证通过”“支持百万连接”等未验证声明。
- 操作日志记录文档结构与引用关系。

## 4. 任务二：更新现有文档

- README.md：
  - 增加“V2 范围”小节，链接 docs/v2-scope.md。
  - 更新 tag 列表，加入 gim-v2.0。
  - 验证状态保持与 validation-status.md 一致；不夸大。
- docs/validation-status.md：
  - 增加“V2 收口（2026-11-14）”小节，链接 docs/v2-scope.md。
  - 确认 NOT RUN/NOT OBSERVED 项未被删除。
- docs/known-issues.md：
  - 顶部增加“V2 范围外的已知边界见 docs/v2-scope.md”引用。
  - 不删除已有条目。
- docs/05_HTTP_API.md、docs/06_WEBSOCKET_PROTOCOL.md、docs/07_CORE_FLOWS.md、docs/04_DATABASE_DESIGN.md：
  - 只在必要处增加“V2 范围外项见 docs/v2-scope.md”一行引用；不改协议内容。
- agent_logs/CURRENT_STATE.md：
  - 更新当前状态为“V2 核心四 Phase 完成，tag gim-v2.0 已创建”，并链接 v2-scope.md。
- 三个子 README（server/、web/、admin/）：
  - 如仍有“V2 待做”表述，统一改为“见 docs/v2-scope.md”；不逐个展开。

验收：文档之间无矛盾；不新增未验证声明。

## 5. 任务三：打 gim-v2.0 tag

- 确认任务一、二的 commit 已提交并推送。
- 确认 HEAD 指向包含 docs/v2-scope.md 的 commit。
- 执行：

```bash
git tag -a gim-v2.0 -m "GIM V2.0: image/file/recall/friend-online (single-instance) implemented and validated; user_confs, admin recall, multi-instance presence, friend-request WS push, full performance ladder explicitly out of V2 scope (see docs/v2-scope.md)"
git push origin main
git push origin gim-v2.0
```

- `git ls-remote --tags origin` 确认远端同时有 gim-v1、gim-v1.1、gim-v1.2、gim-v1.3、gim-v1.4、gim-v2.0，前五个 peeled commit 未变。
- 操作日志记录 tag hash、target commit、push 结果、旧 tag 是否保持不变。

禁止：git tag -f 移动旧 tag、git push --force 覆盖远端 tag、删除已有 tag。

## 6. 执行顺序与 commit

```text
落盘 round-14.md
  -> 任务一 docs/v2-scope.md -> commit（docs: define V2 scope and boundaries）
  -> 任务二更新现有文档 -> commit（docs: align V2 scope references）
  -> 任务三打 tag + push
  -> 全量汇总
```

每个 commit 前：git status、git diff --check、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

建议 commit message：

```text
docs: define V2 scope and boundaries

docs: align V2 scope references

chore(release): tag gim-v2.0
```

不压成一个 commit。本轮不改业务代码，因此不需要重跑全量测试；但需执行 `go build ./...`、`git diff --check` 确认无意外变更。

## 7. 本轮结束输出

- docs/v2-scope.md 摘要：已实现、V2 后期/不做、已知边界、验证状态、tag 语义。
- 现有文档修改清单。
- 是否出现文档矛盾；如何处理。
- commit hash 与 push 结果。
- tag gim-v2.0 指向的 commit hash；前五个 tag 是否未移动。
- git ls-remote --tags origin 输出摘要（六个 tag 的 peeled commit）。
- 操作日志清单。
- 未验证项清单（应与 v2-scope.md 一致）。
- 下一步建议：到此为止 / 补 user_confs / 管理员撤回 / 性能 / 多实例 Presence。

## 8. 遇到以下情况停止并问我

- 发现 README.md 或 validation-status.md 与 v2-scope.md 存在无法自动消解的冲突。
- 发现已有文档声称“全量验证通过”或“支持百万连接”，需要你决定是否删除/修正。
- tag 指向的 commit 与 v2-scope.md 中的验证状态不一致（例如 tag 指向了未包含文档的 commit）。
- push/tag 失败（认证/网络/non-fast-forward）。
- 任何需要改业务代码才能让文档一致的情况。
