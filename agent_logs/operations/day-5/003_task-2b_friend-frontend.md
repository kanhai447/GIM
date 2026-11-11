# 003 / Task 2B — 好友闭环前端 UI

- **日期**：2026-11-11（Asia/Shanghai）
- **授权范围**：`web/` 好友页面、API 封装、Pinia store、组件与测试；不改动聊天/群聊/文件既有逻辑。
- **开始状态**：HEAD `a97bee6`。
- **结束状态**：HEAD `0723004`（本日志所在提交另计）。

## 修改文件

| 文件 | 变更 |
|---|---|
| `web/src/types/friend.ts` | 新增好友/申请/搜索结果类型 |
| `web/src/api/friend.ts` | 新增 8 条好友路由封装 |
| `web/src/api/friend.test.ts` | 新增 2 组路由契约测试 |
| `web/src/stores/friend.ts` | 新增 store：加载、搜索、申请、同意/拒绝、备注、删除、在线计数、鉴权失效处理 |
| `web/src/stores/friend.test.ts` | 新增 6 组 store 测试 |
| `web/src/components/friend/FriendList.vue` | 新增好友列表（在线状态、备注、发消息、删除） |
| `web/src/components/friend/FriendRequestList.vue` | 新增申请列表（同意/拒绝） |
| `web/src/components/friend/friend_components.test.ts` | 新增 5 组组件测试（含 XSS 文本渲染断言） |
| `web/src/views/ContactsView.vue` | 新增 `/contacts` 页面：搜索、结果处理、双栏、详情与备注编辑 |
| `web/src/router/index.ts` | 新增 `contacts` 路由 |
| `web/src/layouts/MainLayout.vue` | 导航新增“通讯录”，退出时清理 friend store，更新侧栏阶段文案 |
| `web/README.md` | 补充页面清单与好友闭环说明，移除“Group/File remain later checkpoints”过期表述 |

## 行为

- 搜索：输入昵称/账号/用户 ID；已是好友显示“已是好友”，已发送显示“已申请”，否则“加好友”。
- 申请：默认只加载 pending；同意后刷新好友列表，拒绝后移除该条。
- 好友列表：按在线优先排序；显示 `@账号 · ID · 在线/离线`；支持备注编辑与双向删除确认。
- 打开会话：跳转 `/chat/:peerId`，复用既有私聊 WS 逻辑。
- 安全：所有用户输入使用文本插值；未引入 `v-html`；好友接口错误码 1113–1119 映射为可读提示；1203–1206/401 触发登录失效处理。
- 不做：`friend.online/offline` 实时提醒（V2），好友列表只在进入页面时读取 Presence。

## 执行命令与结果

```powershell
cd web
.\node_modules\.bin\vue-tsc.CMD --noEmit -p tsconfig.app.json   -> PASS
.\node_modules\.bin\vitest.CMD run                             -> 19 files / 103 tests PASS（原 16/90）
.\node_modules\.bin\vite.CMD build --outDir %TEMP%\...          -> PASS（23 个产物）

cd ..\admin（未修改，回归）
.\node_modules\.bin\vue-tsc.CMD --noEmit -p tsconfig.app.json   -> PASS
.\node_modules\.bin\vitest.CMD run                             -> 11 files / 25 tests PASS
```

## 未执行

- 好友闭环的浏览器端到端（注册两个用户 → 搜索 → 申请 → 同意 → 备注 → 删除）需要 Compose 全栈与 MySQL，本轮 **未执行**。
- 前端 e2e 脚本未新增好友场景；现有 `web/e2e/*.mjs` 需要运行中的后端。

## Git

- commit：`0723004 feat(web): add the contacts friend loop ui`
- push：见 Task 2 汇总提交后的统一推送记录。

## 下一步

Task 3：文档口径收缩。
