# GIM V2 范围与收口评估

- **更新日期**：2026-11-14（Asia/Shanghai）
- **状态**：V2 核心四个 Phase 已完成并验证；本文档定义 V2 的边界，防止 README/面试材料夸大。
- **相关文档**：`docs/validation-status.md`、`docs/known-issues.md`、`README.md`、`docs/06_WEBSOCKET_PROTOCOL.md` §5.6/§5.7。

## 1. V2 已实现（核心四 Phase）

| Phase | 类型 | 关键实现 | 验证证据 |
|---|---|---|---|
| 图片消息 | `msg.type=2` | `POST /api/file/image` 上传；`imageMsg.fileID` 经 `chat.send`/`group.send`；服务端校验归属/`image/*`/大小并回填元数据；Web 上传/发送/渲染/预览；复用 `file_objects`/`user_files`；下载授权联动 | 后端单测 + 真实 MySQL/Redis/WS 集成 + 全量 race + 群聊图片 E2E |
| 普通文件消息 | `msg.type=3` | 与图片同构；`POST /api/file/file`；`FILE_MESSAGE_MAX_BYTES`（默认 25 MiB，不限 MIME 前缀）；Web 文件卡片/鉴权下载 | 后端单测 + 真实集成 + race + 群聊文件 E2E |
| 撤回 | `msg.type=4` | `withdrawMsg.msgID`；`RECALL_WINDOW_SECONDS`（默认 120s）；校验目标/本人/tombstone/已撤回/窗口/群成员；历史 `[撤回]` 占位；文件授权对非所有者失效 | 单测 + 真实集成 + 短窗口 E2E + 并发测试 + race |
| 好友上线提醒（单机） | WS `friend.online`/`friend.offline` | Chat Hub 本地 0→1/1→0 → 有界 worker → `friends JOIN users(status=1)` → `Hub.SendToUser`；只读 Presence；跳过自己/已删除/禁用；前端实时更新 `isOnline` | 单测 + 真实集成 + 浏览器 E2E（在线/离线/重登）+ race |

共同约束：无 schema 变更（图片/文件复用既有表，撤回复用 `msg` JSON）；Group WS 不修改 Presence；Presence 仍只由 Chat WS 维护。

## 2. V2 后期 / 不做

| 项目 | 状态 | 理由 |
|---|---|---|
| `user_confs` 读写 | 不做（V2 后期） | 偏好设置，面试价值低；不影响核心 IM |
| 管理员撤回 | 不做（V2 后期） | 需要独立的 Admin 撤回语义与审计；非核心 |
| 多实例 Presence 全局判断 | 不做（V2 后期） | 需要 Redis 全局 0→1/1→0 判断 + 多实例环境；当前单机语义已文档化 |
| 好友申请实时推送 | 不做（V2 后期） | 当前好友申请走 HTTP；WS 推送属体验优化 |
| 图片/文件吞吐与预览性能 | 不做（V2 后期） | 已有两档最小性能证据；完整阶梯属容量评估 |
| 语音/视频/@/图文混排 | 不做（V3 候选） | 工作量大，非当前目标 |
| WS Ticket、Gateway 本地验签 | 不做（V2 后期） | 当前 query token + Auth 链路已满足 V1/V2 语义 |
| Kafka outbox / 多实例 WS 路由 | 不做（V2 后期） | 架构扩展，需基础设施支持 |

这些项目不阻塞 V2.0；如后续实现，必须新增验证证据并更新本文档与 `docs/validation-status.md`。

## 3. 已知边界（引用 known-issues.md）

- **K-14**：并发不同 `clientMsgId` 撤回同一消息可能产生两条撤回消息；历史占位唯一、前端合并去重、文件授权不受影响；严格“一条”需评估唯一索引/列（schema 变更）。
- **K-15**：好友上线提醒为单机语义（本地 0→1/1→0）；多实例可能重复/误报，`GlobalPresenceCheck` 仅为 V2 占位。
- **K-16**：`friend.offline` 为 best-effort；进程崩溃/TTL 过期无 Hub 回调，不保证发送；有界队列满会丢弃；HTTP `isOnline` 是恢复路径。
- **E-8**：文件授权查询包含 `JSON_EXTRACT` / `NOT EXISTS`，无索引；仅影响非所有者下载路径；下载接口无速率限制。
- **E-9**：浏览器已缓存的图片/文件副本无法回收；撤回/退群只阻止新的服务端下载。

完整列表见 `docs/known-issues.md`。

## 4. 验证状态摘要

详细命令与结果见 `docs/validation-status.md`。

| 项目 | 结果 |
|---|---|
| 后端单元测试 | **PASS**（2026-11-14，70 packages） |
| 真实依赖集成（MySQL/Redis/WS/etcd/Kafka） | **PASS**（图片、文件、撤回、好友提醒；2026-11-14） |
| 前端类型检查/单测/构建 | **PASS**（2026-11-14，21 files / 135 tests） |
| 全量 race | **PASS**（2026-11-14，Linux Docker，无 DATA RACE） |
| Compose 全栈 | **PASS**（2026-11-14，17 services：15 healthy + 2 exit 0） |
| 浏览器 E2E | **PASS**（图片、文件、撤回、好友上线；2026-11-14） |
| Kafka 审计链路 | **PASS**（历史轮次验证；当前未回归） |
| 完整性能阶梯（25/50/100/200 连接、50 msg/s） | **NOT RUN** |
| 5 分钟 soak / restart 稳定性 | **NOT RUN** |
| 图片/文件吞吐与预览性能 | **NOT RUN** |
| 远端 GitHub Actions | **NOT OBSERVED**（本环境无 API 访问） |

> 结论措辞：可以说“V2 核心四 Phase 已实现并通过单测/集成/前端/E2E/race 验证”；**不得**说“全量验证通过”“支持百万连接”“生产级容量已验证”。

## 5. Tag 语义

| tag | 指向 commit | 语义 |
|---|---|---|
| `gim-v1` | `09994cb` | 上一轮基线：文本 IM + 文件上传 + Admin + 审计 |
| `gim-v1.1` | `55bbb8e` | 项目整理 + 好友闭环 + C1/C2 修复 + 验证状态 |
| `gim-v1.2` | `c97618f` | 图片 Phase 1 + 文件授权收尾 |
| `gim-v1.3` | `32a4a55` | 图片 + 文件 + 撤回 + 撤回收尾 |
| `gim-v1.4` | `886d55e` | 图片 + 文件 + 撤回 + 好友上线提醒（单机） |
| `gim-v2.0` | 本轮收口 commit（含本文档；以 `git rev-list -n 1 gim-v2.0` 为准） | V2 核心四 Phase 完成；V2 后期项明确列为范围外 |

## 6. 面试/简历表述建议

- 可写：图片/文件/撤回/好友上线提醒的完整链路、协议与错误码、幂等与事务、历史占位、文件授权、有界异步通知、真实依赖集成与 E2E/race。
- 不可写：多实例 Presence、百万连接、完整性能阶梯、管理员撤回、`user_confs`、语音/视频。
- 面试被问“为什么不做 X”时，引用第 2 节的状态与理由，而不是声称“已支持”。
