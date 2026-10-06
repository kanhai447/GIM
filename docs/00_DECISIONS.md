# 00 已确认架构决策（Decision Log）

这是 Agent 判断“是否可以自行改变”的第一依据。原始参考项目由三个仓库组成：`fim_server`、`fim_web`、`fim_admin`。GIM 必须同时认识用户端 Web 与独立管理端 Admin，不能把 Admin 当成不存在。

| 决策 | GIM V1 结论 | 原因 |
|---|---|---|
| 项目名 | GIM | 与参考 FIM 区分 |
| 参考源码 | FIM Server + Web + Admin | 三个仓库共同构成原项目 |
| 实现方式 | 参考驱动重新实现 | FIM 只读参考，GIM 不复制/改名迁移 |
| 架构 | 保留微服务 | 与参考项目和面试目标一致 |
| 前端 | `web/` 用户端 + `admin/` 管理端 | 原项目有两个独立 Vue 应用 |
| Gateway 认证 | Gateway 调 Auth 验 JWT | 第一版按原项目，后续可优化本地验签 |
| HTTP Token Header | 兼容参考项目 `token` | 避免一次性重写两个前端；V2 可改 Bearer |
| Admin 鉴权 | Gateway 注入 User-ID/Role + 服务端 AdminMiddleware | 浏览器路由守卫仅作 UX，不能替代后端权限 |
| WS 鉴权 | URL query token/JWT | V1 兼容；一次性 Ticket 为 V2 |
| WS 数量 | Chat WS + Group WS 两条 | 第一版不合并 |
| WS 管理 | Hub + Client | 解决普通 map 和并发写问题 |
| WS 写入 | 单 Client 单 writePump | 禁止并发写 Conn |
| 心跳 | Gorilla Ping/Pong 控制帧 | 判断死连接 |
| Presence | 只由 Chat WS 维护 | 原源码 Chat/Group 各有本地连接表；全局在线态只允许一套权威来源 |
| 消息 ACK | 服务端持久化后 ACK | 明确“已发送到服务端” |
| 幂等 | sender + clientMsgId 唯一 | 网络重试不重复入库 |
| 私聊/群聊表 | 分开 | 保留原项目服务边界 |
| 最近会话 | chat_session/group_session | 原源码实时聚合查询复杂且存在删除语义问题；改为显式会话状态 |
| 未读 | session 内 unread + lastReadMsgId | 查询快，便于已读 |
| 文件上传 | 流式 + SHA-256 | 原源码 `io.ReadAll` 会随文件大小线性占内存 |
| 文件去重 | FileObject + UserFile | 原源码 hash 命中直接复用他人 FileModel，所有权/删除语义不完整 |
| Logs/Kafka | V1 保留，但核心 IM 前三天不被其阻塞 | 原 Admin 有真实日志管理 UI，Kafka 不再只是“堆技术栈” |
| 运行/操作日志格式 | 结构化 JSON，敏感字段脱敏 | 禁止原源码 HTML 拼接 + `v-html` + 明文密码/Token 日志 |
| Admin 首页 `/api/data/*` | 不视为现有真实后端能力 | 原 Admin 这三类接口只在 Mock 中存在；GIM 可隐藏或后续实现真实指标 |
| Admin 用户中心 | 非 V1 核心 | 原页面目前仅占位 |
| WebRTC | V2 | 不阻塞核心 IM |
| 多实例 WS 路由 | V2 | V1 先单实例验证并发模型 |
| GitHub 安全 | 禁止提交真实密钥、证书私钥、第三方 Secret、生产配置 | 原源码包含证书私钥与硬编码 Secret，必须清理后再建立 GIM Git 历史 |

任何修改本表的行为都应先得到用户确认。
