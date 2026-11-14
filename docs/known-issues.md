# 已知问题与待办

本文件记录**发现但未修复**的问题、明确延期到 V2 的能力，以及待补验证项。修复或验证完成后更新状态，不直接删除记录。

## 已验证（2026-11-12，第五轮）

| 编号 | 项目 | 证据 |
|---|---|---|
| V-1 | Linux Docker `go test -race ./...` | **PASS**；`golang:1.25-bookworm`，挂载 module cache、`GOPROXY=off`；exit 0，无 race warning |
| V-2 | Compose 全栈 build/up + healthy | **PASS**；17 services = 15 healthy + 2 一次性 job exit 0 |
| V-3 | Kafka 真实 produce/consume/persist/restart | **PASS**；`GIM_KAFKA_INTEGRATION=1`，broker `127.0.0.1:19092`，2 个用例通过 |
| V-5 | 浏览器好友 E2E | **PASS**；Chrome + Compose Web，11 步（注册→搜索→申请→同意→备注→删除） |
| V-7 | 聊天图片集成（MySQL/Redis/WS） | **PASS**；ACK、接收、历史、重复 clientMsgId 仅 1 行、归属/MIME/大小错误码、接收方下载授权 |
| V-8 | 浏览器图片 E2E | **PASS**；Chrome + Compose，上传→发送→接收→预览 |
| V-9 | 图片代码 `-race` | **PASS**（2026-11-14）；Linux Docker 全量 `go test -race ./...`，exit 0，无 race warning |
| V-10 | 群聊图片浏览器 E2E | **PASS**（2026-11-14）；建群邀请 B/C、发送、B/C 收到与预览、非成员 D `2503`、B 退群后 `2503`、owner 仍可下载 |
| V-11 | 文件下载授权专项测试 | **PASS**（真实 MySQL）：所有者/私聊接收方/群成员/非参与者/tombstone/退群/未知 fileID/文本不授权；文件消息同样覆盖 |
| V-13 | 普通文件消息集成 + E2E | **PASS**（2026-11-14）：type=3 ACK/接收/历史/幂等/错误码；浏览器上传 `report.pdf` → 接收方下载 → 非参与者 `2503` |
| V-14 | 群聊文件浏览器 E2E | **PASS**（2026-11-14）：B/C 收到并下载、非成员 D `2503`、B 退群后 `2503`、owner 仍可下载 |
| V-15 | 文件 Phase 全量 race | **PASS**（2026-11-14）：Linux Docker `go test -race ./...`，exit 0，无 race warning |
| V-16 | 撤回实现（后端/前端/历史/授权） | **PASS**（2026-11-14）：type=4 校验/ACK/推送/历史占位/文件下载撤销；单测 + 真实 MySQL/WS 集成 + 浏览器 E2E（文本+文件撤回） |
| V-17 | 撤回窗口短配置 E2E | **PASS**（2026-11-14）：`RECALL_WINDOW_SECONDS=3` 下窗口内撤回成功、4s 后 `2220` 且原消息保留；已恢复默认 120s |
| V-18 | 撤回代码全量 race | **PASS**（2026-11-14）：Linux Docker `go test -race ./...`，exit 0，无 race warning |
| V-19 | 好友上线提醒实现（单机） | **PASS**（2026-11-14）：单测 + 真实 MySQL/Redis/WS 集成（online/offline、多端只一次、非好友不通知）+ 浏览器 E2E + 全量 race |

## 待补验证 / 未覆盖

| 编号 | 项目 | 说明 |
|---|---|---|
| V-4 | 完整性能阶梯 / soak / restart / 文件吞吐 | 本轮只复跑 HTTP profile c10 与 Private 10 conn @10 msg/s；未做 25/50/100/200 连接阶梯、50 msg/s、5 分钟 soak、restart 稳定性与文件吞吐档位。历史数字仍是 2026-10-15 记录 |
| V-6 | 远端 CI 结果 | GitHub Actions 配置存在，但本环境无 API 访问；仅 local equivalent PASS，远端为 `NOT OBSERVED` |
| V-12 | 图片/文件消息性能 | 未单独测试图片与普通文件的上传/下载吞吐、缩略图或大图预览性能 |
| V-20 | 好友上线提醒多实例 | 全局 Presence 0→1/1→0 判断未实现（V2）；单机 E2E 已覆盖 |

## 业务/实现限制（V2 或后续）

| 编号 | 问题 | 说明 |
|---|---|---|
| K-1 | 视频/语音/回复/引用/@/图文未实现 | 文本/图片/普通文件/撤回已实现（`type=1/2/3/4`）；其余富消息仍未进入协议 |
| K-2 | 无接收方 delivery/read receipt | ACK 仅表示服务端持久化/接受 |
| K-3 | 好友 `isOnline` 非实时 | 只在 HTTP 列表/详情请求时读取 Redis Presence，无 `friend.online/offline` 推送 |
| K-4 | 好友申请只提供接收方向 | 未提供“我发出的申请”历史列表；`POST /api/user/valid` 旧路径未实现 |
| K-5 | pending 消息不跨刷新持久化 | 前端只在当前页面内保留 pending/ACK 状态 |
| K-6 | 语音/视频未与消息协议集成 | 图片（`type=2`）与普通文件（`type=3`）已集成；语音/视频仍未进入协议 |
| K-7 | 文件孤儿对象无 GC | 删除最后一个引用不删除物理对象 |
| K-8 | Audit 无 outbox/DLQ | 长时间 Kafka 故障允许丢失审计事件 |
| K-9 | `user_confs` 无读写 API/UI | 仅 Admin 限制更新会按需写入四项限制字段 |
| K-10 | 群消息无 1213 重试 | 群写入按成员 ID 升序降低风险，但未像私聊那样做有界重试 |
| K-11 | 图片尺寸仅客户端提示 | 服务端不解码图片，`width/height` 只做 1..20000 合法性校验；不用于安全判断 |
| K-12 | 撤回已实现 | type=4、`RECALL_WINDOW_SECONDS`、历史占位、文件授权撤销均已实现；已知边界见 K-14 |
| K-13 | 管理员撤回未设计 | 仅实现用户撤回自己的消息；管理员撤回（含原因/审计）留待 V2 后续 |
| K-14 | 并发重复撤回无唯一约束 | 顺序重复撤回幂等；并发不同 clientMsgId 撤回同一消息在无目标唯一索引下可能产生两条撤回消息。并发测试本次实际观察到 1 条（调度相关，不强行断言）；历史占位仍唯一、前端合并去重、授权不受影响。修复“严格一条”需评估唯一索引/列，属 schema 变更 |
| K-15 | 好友上线提醒仅单机 | 已实现单机 0→1/1→0 通知；多实例全局判断未实现（V2，`GlobalPresenceCheck` 占位），多实例下可能重复/误报 |
| K-16 | 好友 offline 事件 best-effort | 进程崩溃/TTL 过期无 Hub 回调，不保证发送 `friend.offline`；channel 满会丢弃；前端通过 HTTP `isOnline` 恢复 |

## 工程/运维限制

| 编号 | 问题 | 说明 |
|---|---|---|
| E-1 | Compose 为单机单节点 | 无 HA、Kubernetes、内部 TLS、Secret Manager |
| E-2 | 内部 API/RPC 无 mTLS | 依赖“只暴露 Gateway + 私有网络”的边界 |
| E-3 | CI 不运行真实依赖集成测试 | `go test ./...` 会跳过 `server/tests`；race job 同样跳过；Kafka 用例需额外开关 |
| E-4 | Admin 构建有 chunk-size warning | 非阻塞 |
| E-5 | Windows native race 不可用 | 32-bit MinGW 不兼容；64-bit MinGW 缺 libc headers；Linux Docker 为验证路径（本轮已 PASS） |
| E-6 | 远端 GitHub Actions 结果未观察 | 本环境无 API 访问，仅 local equivalent PASS |
| E-7 | 未使用导出函数无自动扫描 | 当前以“包级可达性 + 手工 grep”为准；`internal/chat.UnavailableInboundHandler` 仅测试使用，保留为测试替身 |
| E-8 | 图片/文件下载授权跨读消息表 | 非所有者下载时在 File 服务内查询 `chat_messages`/`group_messages` 的 JSON 引用，并用 `NOT EXISTS` 排除被撤回原消息；保持 `SELECT 1 … LIMIT 1`，但 `JSON_EXTRACT` 无索引，且下载接口无速率限制；仅影响非所有者下载路径 |
| E-9 | 浏览器可能缓存已下载图片 | 退群/tombstone 只阻止新的服务端下载；浏览器已缓存的副本无法回收（客户端缓存语义，已记入 docs/05） |

## 使用约定

- 新发现的问题先登记编号与影响，再决定是否在本轮修复。
- 修复/验证后更新状态（已验证/关闭/降级），保留记录与证据链接。
