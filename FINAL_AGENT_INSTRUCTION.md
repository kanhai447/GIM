# GIM Final Agent Instruction（三仓库版）

你负责在四天计划内，基于三个 FIM 参考仓库完成 GIM：

- `reference/fim_server-main/`：Go 后端参考
- `reference/fim_web-master/`：用户端 Vue3 参考
- `reference/fim_admin-master/`：管理端 Vue3 参考

开始前必须完整阅读 `AGENTS.md`、`docs/00_DECISIONS.md` 至 `docs/16_REFERENCE_IMPLEMENTATION_RULES.md`、`docs/12_FOUR_DAY_PLAN.md`、`docs/13_GIT_GITHUB_WORKFLOW.md`。

## 不可违反的边界

- GIM 新项目必须包含 `server/ web/ admin/`。
- FIM 参考目录保持原名且只读。GIM 必须在新的 `server/web/admin` 中自主实现；禁止整文件/整目录复制、批量改名或机械 `fim -> gim` 替换。
- V1 Gateway 继续通过 Auth Service 验 JWT；不要擅自下沉本地验签。
- Chat/Group 两条 WebSocket 保留；Presence 只由 Chat WS 维护。
- WebSocket 必须 Hub/Client + readPump/writePump + Ping/Pong；业务层禁止直接并发写 Conn。
- 消息必须 clientMsgId + ACK + 服务端幂等；session/unread 按文档实现。
- 文件上传禁止整文件 `io.ReadAll`。
- Admin 核心管理 API 必须真实联调并有服务端角色校验。
- `/api/data/*` 是原 Admin Mock-only，不得冒充真实后端能力，也不得未经确认新建数据微服务。
- Kafka/Logs 可在前三天不阻塞核心 IM，但 Day 4 应完成可运行的安全结构化日志链路；禁止记录密码/Token/Secret，禁止未清洗 `v-html` 日志。
- 禁止提交 TLS 私钥、真实第三方 Secret、生产数据库凭证或其他秘密到 GitHub。
- 禁止虚构测试结果、push 结果或性能数据。

## 自主实现原则

原 FIM 只回答“原项目具备什么能力、交互是什么、哪些问题值得避免”。GIM 的代码结构、函数拆分、并发模型、错误处理、测试和配置必须依据 GIM 文档重新实现。遇到文档未规定的细节，应先设计再编码，不得以“直接复制原实现”为默认方案。

## 操作留痕

每完成一个独立操作，立即创建 `agent_logs/operations/day-N/NNN_<action>.md`，记录目标、修改文件、命令、结果、问题、测试、commit/push 状态。每日结束再汇总 `day-N.md` 和 `AGENT_WORKLOG.md`。

## Git

每个稳定 Phase 使用小而清晰的 commit；测试通过后再 push；push 失败必须记录真实错误，不得声称已上传。Day 1/2/3/4 分别建立稳定 checkpoint/tag，最终 `gim-v1` 只在全量验收通过后创建。
