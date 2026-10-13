# Day 3 / Checkpoint 6 — File Frontend Integration + Day 3 Final Acceptance

- **时间:** 2026-10-13 13:52 +08:00
- **开始状态:** `main`，`HEAD == origin/main == 7c2008219907d1c142ccb82659bbab0f556ee6ed`，working tree clean。
- **授权范围:** File 用户端接入与 Day 3 总验收；严格未实现 Chat/Group 文件消息、Admin File/Chat、Kafka、Docker Compose 或 Day 4。

## File Frontend

- 建立 `FileView/components -> File Store -> File API -> Gateway` 分层；组件不直接调用 Axios，文件列表不写 LocalStorage。
- 新增公开 typed model：UserFile metadata、upload result、cursor page、独立 upload task；production 代码无 `any`。
- `/files` 替换 placeholder，展示原文件名、server MIME、格式化大小、创建时间、下载与删除；空列表是正常 empty state。
- 初始列表固定 20 条，`beforeId` load-more；按 UserFile ID 合并并按 ID 倒序，重复上传返回同 ID 时不产生重复 UI 行。

## Upload / Progress / Errors

- multipart 仅调用 Gateway `/api/file/file`；多文件各自维护 waiting/uploading/success/failed、百分比和安全失败信息，单任务失败不污染其他任务。
- 前端 50 MiB 预检查会在读取/请求前失败；后端 50 MiB 流式 cap 仍是最终安全边界。
- 真实 Chrome 首轮暴露 Vue 响应式问题：普通 task 入数组后继续修改 raw 引用，接口 200 但 DOM 停在 uploading。改为按 task ID 不可变替换并增加同步 watch 回归测试，最终 progress/success 可见。
- 2502/2503/2504、认证失败、网络和 generic error 映射为安全文案；不显示 SQL、堆栈、DSN 或 storage path。认证错误委托 App 既有全局 invalidation。
- Abort/cancel 未实现，记录为后续可选能力；不为非强制项扩大本 Checkpoint。

## Download / Delete / Security

- 下载走 shared authenticated Axios client、`responseType=blob`、临时 `<a download>` 和 `URL.revokeObjectURL`；没有 `window.open` 或公开 URL 绕过 token。
- 以服务端 `Content-Disposition: attachment` 作为成功边界；HTTP 200 business-error envelope 会解析为 PublicApiError，不会被保存成“文件”。文件名取公开 metadata，浏览器可按操作系统规则进一步安全化。
- 删除前有明确确认，只删除当前账号 UserFile 引用；文案明确共享内容不会被误删，成功后只移除对应行。
- filename/MIME 均用 Vue 文本插值；`<img src=x onerror=alert(1)>.txt` 保持字面文本，无 `v-html/innerHTML`。V1 不提供 HTML/SVG/PDF/image/text inline preview。

## Automated Frontend Tests

- `pnpm test`: **16 files / 90 tests PASS**。覆盖 API/cursor/multipart/progress/download envelope、size/date formatter、Blob URL revoke、list/load more、duplicate reconciliation、多任务、oversize、download/delete、ownership/auth delegation、empty/row/progress/failure/action 和恶意文件名文本渲染。
- `pnpm type-check`: PASS。
- `pnpm build`: PASS，1743 modules transformed。
- `node --check e2e/file-management-smoke.mjs`: PASS。
- lint: **NOT CONFIGURED**；未临时引入 ESLint。

## Real Chrome File A/B E2E

- 真实 MySQL、Redis、隔离 etcd、User RPC/API、Auth、Chat、Group、File、Gateway、Vite 与已安装 Chrome 全栈运行。
- A 上传 HTML-like filename 与二进制；list/refresh 恢复；认证 Blob 下载与源内容 SHA-256 相等；确认删除后 UserFile 行消失，其他文件仍可下载。
- 12 MiB 文件真实出现 uploading/progress/success，下载 SHA-256 相等；50 MiB + 1 byte sparse fixture 在前端显示“未开始上传”，没有发送正文。
- 21 个独立小文件上传后刷新只取首 20 条，点击 load-more 使用 `beforeId` 得到 >20 且 ID 合并无重复。
- B 对 A UserFile 的 detail/download/delete 均得到统一 code 2503，A 引用和下载保持正常；未泄露 owner/object/storage 信息。
- A/B 上传相同内容但不同文件名，各有独立 UserFile；A 删除后 B 下载内容仍相等，证明共享物理对象未被误删。
- 最终脚本两次完整 PASS（修复后及下载-envelope 加固后的最终复验）；Chrome、服务、fixture 与下载临时文件已清理。

## Private Chat Final Regression

- 真实 Chrome A/B + Gateway/Auth/Chat/MySQL/Redis：双向顺序、pending -> persist ACK -> sent、receiver realtime/unread/read、refresh/history、20+ cursor、双设备 exact-once/shared read、Chat API restart/reconnect、logout、revoked token terminal、HTML-like text全部 PASS。
- 最终结果：`PRIVATE CHAT V1 = COMPLETED / REGRESSION PASS`。

## Group Chat Final Regression

- 真实 Chrome A/B/C：create/invite、ACK/message merge、三成员 realtime/unread/read、refresh/history、mute/unmute/receive while muted、双设备、cursor、Group API restart/reconnect、Chat socket unaffected、logout/revoked auth、HTML text全部 PASS。
- 最终结果从 FUNCTIONAL 升级为 `GROUP CHAT V1 = COMPLETED`。

## File Backend Final Regression

- 真实 MySQL/filesystem、File unit、Gateway/Auth/etcd 集成全量 PASS；上传/list/detail/download/delete、ownership、dedup、restart persistence、missing physical、compensation/failure injection 和 Content-Disposition 路径仍由既有 suite 覆盖。
- File focused integration 在显式 `GIM_ENV_FILE` 下连续 3 轮 PASS；最终 `FILE V1 = COMPLETED`。

## Gateway / etcd / MySQL / Redis

- Gateway `/api/auth|user|chat|group|file` 均由真实 browser/integration 经过 Auth；Chat WS、Group WS 与 File HTTP 同时运行互不冲突，伪造身份既有回归 PASS。
- etcd 实际观察到 `auth_api/user_api/chat_api/group_api/file_api` 五个 lease key；应用 shutdown 后 `/gim/services/` 为 0，最终 etcd 2379/2380 关闭且 data 清理。
- migration status 为 001–006 全 applied；重复 `up` 两次无错误、无新 migration 或 schema 改动。
- Redis 当前范围保持 Auth revoke、Chat Presence/delivery、Group mute/delivery；File 无 Redis dependency。结束时 `gim:*` key 总数 0（presence/mute/revoked 均 0）。

## Backend / Stability / Quality

- 显式真实配置 `GIM_ENV_FILE` 的 `go test ./... -count=1`: PASS；普通全量单元同样 PASS。
- File 真实 integration 3 轮 PASS；Chat message/session + Group foundation/message 真实 integration 3 轮 PASS。
- Chat/Group lifecycle/heartbeat/shutdown/concurrent writer focused **10 轮** PASS。
- `go vet ./...`、`go fmt ./...`（无输出）、`git diff --check` PASS。
- 测试后 fixture user/group/message/session/file reference 计数归零，Redis `gim:*` 为 0，8080–8085/9091/5173/2379/2380 无监听，`.tmp/day3-cp6` 已删除。

## Security / Reference Independence

- repository safety script PASS：`.env.local`/private key/reference/build/data ignore 有效，无 forbidden tracked path；candidate secret scan无新增秘密。
- production File frontend 无 token/payload console log、`v-html`、`innerHTML`、filesystem/storage path；Blob URL 有释放路径。
- `git ls-files reference` 为空，tracked symlink 为空；`server/web/admin` 不 import、runtime read、symlink 或 build-depend on reference。reference 未修改。
- **FIM referenced:** 用户端上传入口、`src/url` 响应兼容、File 管理基本交互；只用于行为/协议理解。
- **GIM independently implemented:** typed File API/Store/components/tests/E2E 全部在 GIM `web/` 自主编写；增加 authenticated Blob、business-envelope 边界、任务 reconciliation、ownership UX 与完整 A/B 安全验收。

## Race / Known Limitations

- **Race: NOT AVAILABLE IN CURRENT LOCAL ENVIRONMENT；Race Verification Pending。** 沿用已确认的 32-bit MinGW `cc1.exe: 64-bit mode not compiled in`，未伪造或重复无意义执行；需 compatible 64-bit toolchain/CI。
- Private/Group pending 只在内存，未 ACK 消息不跨刷新；已持久化消息由 history 恢复。
- V1 无 receiver delivery ACK/read receipt；Chat/Group 当前只支持 text，File 尚未接入消息协议。
- File 为单实例本地 storage + process-local hash lock；延迟 physical orphan GC 未实现。
- Browser authenticated download 会在客户端内存物化 Blob；后端仍为 streaming。Upload cancel 未实现。

## Files / Database / Protocol

- 新增：`web/src/types/file.ts`、`web/src/api/file*.ts`、`web/src/stores/file*.ts`、`web/src/utils/file*.ts`、`web/src/components/file/*`、`web/e2e/file-management-smoke.mjs`。
- 修改：`web/src/views/FileView.vue`、`web/src/App.vue`、`web/src/layouts/MainLayout.vue`、`web/package.json`。
- Database/migration、后端 HTTP/WS/Redis protocol：无变化；前端接入 Checkpoint 5 已有 File 契约。

## Git / Freeze

- Implementation commit：`a9e1182 feat(web): integrate file management`。
- Acceptance/log commit：`docs(day3): finalize day3`（本 operation/state 所在提交）。
- Push 与最终 `HEAD == origin/main` / clean 在提交后复核。
- `PRIVATE CHAT V1 = COMPLETED`；`GROUP CHAT V1 = COMPLETED`；`FILE V1 = COMPLETED`；`DAY 3 = COMPLETED`。
- `DAY 4 = NOT STARTED`；`Next Authorized Work = NONE`。本轮到此停止。
