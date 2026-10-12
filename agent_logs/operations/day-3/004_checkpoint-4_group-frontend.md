# Day 3 / Checkpoint 4 — Group Frontend Integration

- **时间:** 2026-10-13 07:50 +08:00
- **开始状态:** `main`，`HEAD == origin/main == 69c16a93a3e7e99267379f072e9311f4df0dcd70`，working tree clean。
- **授权范围:** Group Web 前端、typed Group Socket、Group Store、Group UI、自动化测试与真实浏览器联调；严格未进入 File、Admin 或 Day 4。

## Architecture

- 新增独立分层：`GroupView/components -> Group Store -> GroupSocketClient`，HTTP Group API 独立走 Gateway；没有把 Chat/Group 合并成一条连接或一个 Store。
- 登录用户只创建一条 Group socket，与既有一条 Chat socket 并存；`App.vue` 统一处理两条连接的启动、logout 停止和认证失效清理。
- `My Groups` 与 `Group Sessions` 使用不同 API/状态集合；Group session 保持后端 `isTop DESC, lastMessageAt DESC` 权威顺序，实时消息只做相同语义的即时本地重排并进行有界服务端校准。
- `/group/:groupId?` 支持刷新恢复：认证恢复后重建 Chat/Group 两条 socket，并加载 group info、member cache、history 与 persisted read state。

## Socket / Protocol / Reliability

- Group Socket 使用 `/api/group/ws/chat?token=JWT`，按页面来源自动选择 `ws/wss`；从不记录 URL、token、payload 或消息正文。
- lifecycle 为 `disconnected/connecting/connected/reconnecting`；单一 reconnect timer、1/2/4/8/16/30 秒有界指数退避、成功归零、logout/认证失败永久停止。
- `group.send/group.ack/group.message/error` 均为严格 TypeScript 类型；runtime parser 拒绝 malformed/未知事件并安全忽略，无 `any`。
- 每条发送生成稳定 UUID `clientMsgId`；pending 在内存中，ACK timeout/可重试错误后复用同一 ID，刷新不保留未 ACK pending。
- ACK-first/message-first 两种事件顺序都按 `messageId/clientMsgId` 合并；重复 realtime、重复 ACK 与 history overlap 不增加第二条 UI 消息。
- `pending -> sent` 只由 persist ACK 或同构 `group.message` 完成；typed error 进入 `failed`。禁言可重试但保持 socket；非成员/群解散/clientMsgId conflict 标记不可重试并刷新 Group 权威状态。

## Sessions / History / Read / UI

- Group session 展示群名、头像、同源 preview、时间、unread 与 top；realtime 更新 preview/time/order，reconnect/refresh 后用后端 session 覆盖校准。
- 非当前群 realtime 本地 unread `+1`；当前路由同时满足 document visible + window focused 时才自动 Mark Read。HTTP read cursor 为用户级 DB 权威，多设备通过 refresh 共享。
- History 首屏 20 条，后续使用 `beforeMessageID`；统一合并 history/pending/ACK/realtime，并保持 oldest -> newest。
- 群信息展示名称、头像、当前角色、成员数、简介与成员列表；sender 只查当前 member cache，不逐消息调用 User API。
- 提供最小建群入口和 owner/admin 可见的 5 分钟禁言/解除入口；后端仍是角色/禁言权限唯一安全边界。
- composer 只实现 text；没有伪造 image/file/voice 能力。消息使用 Vue 文本插值，`<b>hello group</b>` 显示为字面文本；无 `v-html/innerHTML`。

## Automated Tests

- `pnpm test`: PASS，12 files / 77 tests。
  - Group Socket 12 cases：单例连接、状态、单 timer、退避上限/归零、logout、auth close、connected-only send、typed parse/unknown ignore、URL。
  - Group Store 17 cases：分离列表、刷新恢复、cursor、稳定 ID、ACK/message 双顺序、duplicate、lost ACK、mute/nonmember、active/background unread/read、create/mute、logout/auth failure。
  - Reconcile 5 cases；Group component 3 cases（状态、session/unread/top、HTML literal）。
  - 既有 Chat socket/store/components、Auth/API/router 全部回归 PASS。
- `pnpm type-check`: PASS。
- `pnpm build`: PASS，Vite 1736 modules transformed；lint NOT CONFIGURED。
- `node --check web/e2e/group-chat-smoke.mjs`: PASS。

## Real Chrome A/B/C E2E

- 真实本地 MySQL、Redis、etcd、User RPC/API、Auth API、Chat API、Group API、Gateway、Vite 与已安装 Chrome（Playwright Core 驱动三个独立 browser context）运行。
- A 创建群并邀请 B/C；A 发送 `hello-group-*`，浏览器观察到 pending -> sent 且仅一条；B/C 均实时获得 session preview/unread=1。
- B 打开群后 unread=0，刷新后 history/info/member/read state 恢复；C 保持非当前群并保留 unread。
- A 禁言 B：B 发送进入 failed 且显示安全中文；Group socket 保持 connected；A 在 B 被禁言时发送，B 仍正常接收；解除后 B 发送成功。
- HTML-like text 在 B 显示字面内容且 DOM 无 `b` 元素。
- B1/B2 两设备均各看到一次新消息；B1 read 后 B2 refresh，服务端共享 unread=0。
- 发送 21 条分页消息；刷新首屏精确 20，`beforeMessageID` 加载后消息数 >20 且无重复。
- 人工停止 Group API 后 A/B 观察 `reconnecting -> connected`；A 的 Chat socket 全程未关闭；Group 恢复后新消息发送与 B realtime PASS。
- A logout 后 Chat/Group 两条 socket 均关闭且无重连；B token 经 Auth logout 撤销后刷新返回 Login，后续未进入 reconnect loop。
- 最终脚本结果：`PASS`，临时浏览器/服务/etcd 全部关闭；`gim_cp4_*`、`probe` 测试用户及相关 group/message/session/member 数据已删除，复核计数 0。

## E2E Issues / Fixes

1. 首轮 registration 得到 Vite 404：根因是 Vite 默认只读 `web/` 环境文件，未加载根目录 `.env.local` 的 proxy target；以显式本地 `VITE_DEV_GATEWAY_TARGET` 重启，产品配置/代码无需修改。
2. 首次 pagination assert 在 reload 后只等待 WS connected，早于 HTTP history render；修正 smoke 同步点为等待最新消息出现再断言 20。
3. 首次 restart smoke 只等待 A socket，B subscriber 尚未稳定即发送；修正为等待 A/B 均 connected 并留 750ms 订阅稳定窗口，随后 PASS。消息持久化/ACK 未失败。

## Backend / Security / Regression

- 后端 production code、DB migration、HTTP/WS/Redis protocol：无修改。
- Group + Private Chat 真实 MySQL/Redis focused integration：PASS。
- `go test ./... -count=1`: PASS；`go vet ./...`: PASS；`gofmt -l .`: PASS。
- repository safety、tracked secret scan、`git diff --check`: PASS；Group 新代码无 `any`、`v-html`、`innerHTML`、console payload/token log。
- Race：`NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT`；沿用已确认的 32-bit MinGW `cc1.exe: 64-bit mode not compiled in` 限制，未重复运行；compatible environment/CI PENDING。

## Reference / Independent Implementation

- **FIM referenced:** Group API 路径、单用户级 Group socket 服务多个群、群会话/群详情/成员入口与主要 UI 行为。
- **GIM independently implemented:** Group types/API/socket/store/reconcile/components/tests/E2E 均在 GIM `web/` 中自主编写，没有复制 reference 文件或依赖 reference 编译。
- **GIM improvements:** FIM 缺少完整自动重连、pending ACK、typed event、双事件顺序去重、focus-aware read 与安全 lifecycle；GIM 增加独立 socket manager、稳定 clientMsgId、persist ACK 状态机、服务端权威 reconciliation、多设备 read 与安全文本渲染。

## Files / Data / Protocol

- 新增：`web/src/types/group.ts`、`web/src/api/group.ts`、`web/src/service/ws/group_socket*.ts`、`web/src/stores/group*.ts`、`web/src/components/group/*`、`web/e2e/group-chat-smoke.mjs`。
- 修改：`web/src/App.vue`、`web/src/router/index.ts`、`web/src/views/GroupView.vue`、`web/src/layouts/MainLayout.vue`、`web/package.json`。
- Database：无 schema 变化；临时 E2E 数据已删除。
- HTTP/WS/Redis：无服务端协议变化；前端实现既有契约。

## Git

- Implementation commit：`0a17af6 feat(web): integrate reliable group chat`。
- Log/status commit：本记录所在 `docs(day3): record group frontend integration` 提交。
- Push：implementation commit 已成功推送 `origin/main`；本 log/status commit 随后推送并执行 equality/clean 复核。

## Result / Boundary

- Day 3 / Checkpoint 4 Group Frontend Integration：验收通过。
- `GROUP CHAT V1 = FUNCTIONAL`（当前文本消息范围；文件/图片/语音仍属于后续 File checkpoint）。
- Private Chat V1：REGRESSION PASS。
- 下一计划：File Service checkpoint，必须等待用户另行授权；未进入 File、Admin 或 Day 4。
