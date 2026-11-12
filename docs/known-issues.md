# 已知问题与待办

本文件记录**发现但未修复**的问题、明确延期到 V2 的能力，以及待补验证项。修复或验证完成后更新状态，不直接删除记录。

## 已验证（2026-11-12，第五轮）

| 编号 | 项目 | 证据 |
|---|---|---|
| V-1 | Linux Docker `go test -race ./...` | **PASS**；`golang:1.25-bookworm`，挂载 module cache、`GOPROXY=off`；exit 0，无 race warning |
| V-2 | Compose 全栈 build/up + healthy | **PASS**；17 services = 15 healthy + 2 一次性 job exit 0 |
| V-3 | Kafka 真实 produce/consume/persist/restart | **PASS**；`GIM_KAFKA_INTEGRATION=1`，broker `127.0.0.1:19092`，2 个用例通过 |
| V-5 | 浏览器好友 E2E | **PASS**；Chrome + Compose Web，11 步（注册→搜索→申请→同意→备注→删除） |

## 待补验证 / 未覆盖

| 编号 | 项目 | 说明 |
|---|---|---|
| V-4 | 完整性能阶梯 / soak / restart / 文件吞吐 | 本轮只复跑 HTTP profile c10 与 Private 10 conn @10 msg/s；未做 25/50/100/200 连接阶梯、50 msg/s、5 分钟 soak、restart 稳定性与文件吞吐档位。历史数字仍是 2026-10-15 记录 |
| V-6 | 远端 CI 结果 | GitHub Actions 配置存在，但本环境无 API 访问；仅 local equivalent PASS，远端为 `NOT OBSERVED` |

## 业务/实现限制（V2 或后续）

| 编号 | 问题 | 说明 |
|---|---|---|
| K-1 | 仅支持文本消息 | 图片/视频/文件/语音/回复/引用/撤回/@/图文均未进入消息协议 |
| K-2 | 无接收方 delivery/read receipt | ACK 仅表示服务端持久化/接受 |
| K-3 | 好友 `isOnline` 非实时 | 只在 HTTP 列表/详情请求时读取 Redis Presence，无 `friend.online/offline` 推送 |
| K-4 | 好友申请只提供接收方向 | 未提供“我发出的申请”历史列表；`POST /api/user/valid` 旧路径未实现 |
| K-5 | pending 消息不跨刷新持久化 | 前端只在当前页面内保留 pending/ACK 状态 |
| K-6 | File 未与消息协议集成 | 上传成功不等于发送文件消息 |
| K-7 | 文件孤儿对象无 GC | 删除最后一个引用不删除物理对象 |
| K-8 | Audit 无 outbox/DLQ | 长时间 Kafka 故障允许丢失审计事件 |
| K-9 | `user_confs` 无读写 API/UI | 仅 Admin 限制更新会按需写入四项限制字段 |
| K-10 | 群消息无 1213 重试 | 群写入按成员 ID 升序降低风险，但未像私聊那样做有界重试 |

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

## 使用约定

- 新发现的问题先登记编号与影响，再决定是否在本轮修复。
- 修复/验证后更新状态（已验证/关闭/降级），保留记录与证据链接。
