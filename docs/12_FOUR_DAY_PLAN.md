# 12 GIM 四天实施计划（三仓库版）

四天计划以真实代码/测试为完成依据。每个独立操作完成后立即写 `agent_logs/operations/day-N/*.md`。

## Day 1：三仓库基线、安全清理、数据库底座

覆盖 Phase 0、0.5、1。

### 目标
- 审计 Server/Web/Admin 三仓库，明确 Admin 真实接口与 Mock-only Dashboard。
- 在 GIM Git 历史产生前排除原仓库 TLS 私钥、第三方 Secret、JWT/数据库真实配置。
- 建立只读 `reference/` 和全新 `server/ web/ admin/`；GIM 代码从空骨架自主编写，禁止从 FIM 复制后改名。
- 启动 MySQL、Redis、etcd；跑通 Gateway -> Auth 登录和核心服务。
- migration、索引、client_msg_id、chat_session/group_session 基础结构。

### 最低验收
- 登录经 Gateway/Auth 成功。
- 用户 Web 可启动；Admin 可启动到登录/路由层。
- Secret scan 无私钥/真实 Secret。
- 空库 migration 成功。

建议 tag：`gim-day1-baseline`。

## Day 2：Chat WS + Presence + 私聊可靠性

覆盖 Phase 2、3。

### 目标
Hub/Client、readPump/writePump、Ping/Pong、多端 Presence、好友上线链路、私聊 ACK/幂等、chat_session、未读/read。

### 验收
并发写无 race；同 clientMsgId 不重复；离线历史/未读正确；Group WS 不影响 Presence。

建议 tag：`gim-day2-chat`。

## Day 3：Group WS + 文件服务 + 用户 Web 协议适配

覆盖 Phase 4、5、6、Phase 7 的用户端主要部分。

### 目标
群 Hub/可靠消息/session；禁言/权限/撤回兼容；文件流式 SHA-256 与所有权模型；用户 Web 自动重连、ACK、未读基本联调。

### 验收
群聊并发稳定；文件不整包入内存；私聊无回归；用户 Web 完成核心 IM 流程。

建议 tag：`gim-day3-core`。

## Day 4：Admin、Kafka Logs、安全、全量验收

覆盖 Phase 7.5、8。

### 目标
- 参考原 Admin 的页面与接口语义，自主实现并联调独立 `admin/`。
- 用户、私聊、群、文件、设置、日志管理页面连接真实 API。
- Kafka 日志链路可运行；重构共享 Pusher 为每请求独立结构化事件。
- password/token/cookie/secret 统一脱敏。
- Admin 日志详情不使用危险 `v-html`。
- 原 `/api/data/*` Mock-only Dashboard 隐藏/明确标识；若时间充裕再实现真实 IM 指标，禁止为了它擅自新建服务。
- 全量 build/test/race、README、Docker/启动说明、GitHub push、面试材料。

### 最低验收
- 普通用户无法调用 Admin API；管理员核心管理页可用。
- 日志中没有密码/JWT 等敏感信息，且并发日志不串线。
- 用户 Web 与 Admin 均 build 成功。
- GitHub 中无 TLS 私钥和真实 Secret。
- README 能说明三工程启动方式。

建议 tag：`gim-v1`。

# 每日开始输出
1. 当日 Phase；2. 目标；3. 文件计划；4. DB/API/WS 变化；5. 测试计划；6. Git checkpoint。

# 每日结束输出
1. 已完成/未完成；2. 修改文件；3. migration/API/WS 变化；4. 实际测试命令与结果；5. 问题/修复；6. 操作日志清单；7. commit hash；8. push 结果；9. tag；10. 遗留问题。
