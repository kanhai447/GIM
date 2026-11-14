# GIM 第十五轮 — 生成源码级架构深度文档

## 0. 前提与授权

GIM V2.0 已收口：HEAD = origin/main = 2d849bc，gim-v2.0 指向 8e39668，六个 tag 均未移动。功能、验证、范围文档已闭合。

本轮唯一目标：生成 docs/architecture-deep-dive.md，把项目讲清楚到“每个文件处理什么、关联哪些文件、按功能怎么串起来”。

本轮明确不做：

- 不写业务代码、不改任何 server/、web/、admin/ 源码。
- 不修改现有协议、数据库、流程文档（docs/04–08），只新增一份深度文档；如发现现有文档与实现冲突，在新文档中标注差异，不改旧文档。
- 不读 reference/（本轮不需要参考 FIM，只读 GIM 自身）。
- 不虚构函数名、文件路径、调用关系；不确定的标注“推测”或“未找到”，并给出已核对路径。
- 不打 tag、不 push 业务变更。

如果发现必须改代码才能讲清楚（例如某函数职责混乱），先停止报告，不要改代码。

## 1. 红线

- 只读源码，不改业务代码。
- 不提交密钥、私钥、真实配置值。
- 每个章节必须带真实文件路径 + 函数/方法名；不得只写“服务层处理”。
- 文档中的结论必须能在代码中找到依据；找不到就写“未找到/推测”。
- 操作日志 agent_logs/operations/day-18/；没有记录视为未完成。
- commit 前 git status、git diff --check、确认无 .env/reference//密钥进入 staged。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-15.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = 2d849bc、六个 tag 存在。

## 3. 文档结构（必须按此章节）

新建 docs/architecture-deep-dive.md，包含以下章节。先输出完整大纲，再逐章填充；每章完成后自检文件路径真实性。

### 第 1 章：进程与启动

对 server/cmd/ 下每个入口（gateway、auth-api、user-api、user-rpc、chat-api、group-api、file-api、settings-api、logs-api、migrate、admin-bootstrap）：

- 入口文件路径、main 函数位置；
- 初始化顺序：配置加载 → DB/Redis/etcd/Kafka → 依赖装配 → 路由注册 → 启动监听 → 优雅关闭；
- 依赖了哪些内部包（列出 import 的关键包路径）；
- 是否注册到 etcd、监听端口、健康检查路径；
- 优雅关闭如何处理（信号、worker drain、Hub shutdown）。

### 第 2 章：HTTP 请求链路

以 Gateway 为起点，画出：

```text
客户端 -> Gateway handler -> Auth authentication -> 目标服务 handler -> service -> repository -> DB/Redis
```

必须列出：

- internal/gateway/handler.go 中关键函数：清伪造头、调 Auth、注入身份、路由、反向代理；
- internal/gateway/route/route.go 的服务映射；
- 以 3 个代表性接口为例（登录、发好友申请、Admin 用户列表），逐函数列出调用链，带文件路径和函数名；
- 错误码在哪些层产生、如何透传。

### 第 3 章：WebSocket 连接链路

对 Chat WS 和 Group WS 分别：

- Upgrade 入口文件与函数；
- Auth 校验位置；
- Hub register/unregister 流程；
- readPump / writePump 的职责、Ping/Pong、deadline、慢客户端摘除；
- 消息帧从 readPump 到 service 的调用链；
- ACK/推送从 service 到 writePump 的路径；
- Presence 写入位置（仅 Chat）；
- 好友通知 observer/worker 的位置与入队/消费链路。

### 第 4 章：按功能拆解（本文档主体）

每个功能一节，按固定结构：

```markdown
### 功能：<名称>

**涉及文件**
- 入口：
- 协议/DTO：
- service：
- repository：
- 前端：
- 测试：

**调用顺序**
1. ...（文件:函数）

**事务边界**
- ...

**并发/幂等/错误码**
- ...

**已知边界**
- 引用 known-issues 编号

**面试可讲的权衡**
- ...
```

必须覆盖的功能清单：

- 注册、登录、注销、JWT 校验、logout 黑名单
- 用户资料读取、Admin 用户列表/限制/禁用
- 好友搜索、发送申请、申请列表、同意/拒绝、好友列表、备注、删除、好友详情
- 私聊文本消息：发送、幂等、ACK、推送、历史
- 私聊图片消息：上传、发送、授权、渲染
- 私聊普通文件消息：上传、发送、授权、渲染
- 私聊撤回：校验、窗口、历史替换、文件授权联动
- 群创建、成员、角色、邀请、申请、禁言
- 群文本/图片/文件消息：发送、ACK、广播、session 扇出
- 群撤回：校验、历史替换、授权联动
- session 列表、未读、已读、置顶、删除语义
- 文件上传（流式 SHA-256、原子 rename、FileObject/UserFile）、下载授权
- 好友上线提醒：0→1/1→0、worker、多端、过滤
- Admin 私聊审计、群管理、文件管理、设置、日志查询
- 审计日志：每请求 LogEvent、脱敏、Kafka producer、consumer 幂等
- migration：版本化、runner、dirty 保护

每个功能必须能回答：请求从哪个文件进来、经过哪些函数、在哪落库、失败怎么返回、前端哪个组件消费。

### 第 5 章：数据模型与索引

对 server/migrations/ 中每张表：

- 表名、字段、索引、唯一约束；
- 被哪些 repository 读写（文件路径）；
- 事务边界（哪些操作在同一事务）；
- 与 docs/04_DATABASE_DESIGN.md 的一致性；如不一致，标注差异。

### 第 6 章：并发模型

- Hub/Client 单写模型：clients map 唯一写者、Send 有界、慢客户端摘除；
- 消息发送锁顺序（session 按 user_id 升序）、1213 有界重试；
- 幂等：clientMsgId + 唯一约束；
- Presence：只由 Chat WS 写、TTL 刷新；
- 好友通知 worker：有界、异步、不阻塞 Hub；
- 审计 producer：有界、失败退化；
- 列出每个共享可变状态及其保护方式（channel、mutex、单 goroutine）。

### 第 7 章：配置与部署

- .env.example / .env.compose.example 每个键：被哪个文件读取、默认值、是否必填；
- compose.yaml 每个服务：镜像、依赖、健康检查、启动顺序；
- 内部端口与外部入口边界；
- migration 与 storage-init 一次性 job 的作用。

### 第 8 章：测试地图

对 server/、web/、admin/ 的测试文件：

- 每个测试文件覆盖什么功能；
- 哪些是 gated（需要 GIM_ENV_FILE）、哪些默认 skip；
- 哪些场景只在单测覆盖、哪些有集成/E2E；
- 与 docs/validation-status.md 对齐；不一致处标注。

### 第 9 章：已知边界与 V2 范围

- 引用 docs/v2-scope.md、docs/known-issues.md、docs/validation-status.md；
- 列出 K-14、K-15、K-16、E-8、E-9 及其代码位置；
- 不重复展开，只给指针和一句话说明。

### 第 10 章：文件索引

按目录列出 server/internal/ 每个子包的一句话职责 + 关键文件；web/src/、admin/src/ 同理。这是“查文件用”的索引，不展开细节。

## 4. 质量要求

- 文件路径必须真实：每写一个路径，用 rg 或文件存在性检查确认；不确定的标注“未找到”。
- 函数名必须真实：引用函数时带文件路径和函数名，不写“某函数”。
- 调用链必须可追踪：每步写 文件:函数，不写“服务层处理”。
- 不复制现有文档：可以引用 docs/04–08 的结论，但必须补充“代码里在哪、怎么串”。
- 不虚构：找不到实现、找不到调用方、文档与代码冲突，全部标注，不猜。
- 控制篇幅：第 4 章是主体，可以长；第 1、2、3、5–10 章简洁，以表格和调用链为主。
- 不写“全量验证通过”“支持百万连接”：与 v2-scope.md 口径一致。

## 5. 执行方式（分批生成，避免一次性过长）

建议分 3 个 commit：

```text
commit 1：第 1–3 章（进程、HTTP、WS 链路）
commit 2：第 4 章（按功能拆解，可分 2–3 次提交，如私聊/群聊/文件与撤回/好友与 Admin）
commit 3：第 5–10 章（数据、并发、配置、测试、边界、索引）
```

每个 commit 前：

- 自检本章文件路径真实性（可用 rg 抽查）；
- git status、git diff --check；
- 确认操作日志 agent_logs/operations/day-18/ 有对应记录。

不压成一个 commit；本轮不打 tag。

## 6. 本轮结束输出

- docs/architecture-deep-dive.md 的章节完成情况。
- 每章覆盖的文件/包数量。
- 发现的“文档与实现不一致”清单（只记录，不修改旧文档）。
- 发现的“找不到实现/调用方”清单。
- 文件路径抽查结果（抽 10 个路径，确认存在）。
- commit hash 与 push 结果。
- 操作日志清单。
- 文档中标注“推测/未找到”的位置清单。
- 下一步建议：是否需要补充某章、是否进入面试设计问答文档。

## 7. 遇到以下情况停止并问我

- 某功能在代码中找不到完整实现，无法写出调用链。
- 现有 docs/04–08 与实现存在无法判断“谁对”的冲突。
- 需要读 reference/ 才能解释某段 GIM 代码（说明 GIM 可能复制了参考实现，需要你判断）。
- 文档规模超出预期，需要你决定是否拆分文件（例如按 architecture/ 目录分多份）。
- 发现业务代码缺陷，需要修复才能讲清楚（本轮不改代码，只报告）。
