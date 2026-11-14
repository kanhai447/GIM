# GIM 第十三轮 — 好友上线提醒实现 + gim-v1.4 tag

## 0. 前提与授权

第十二轮已完成：撤回收尾三项 PASS；好友上线提醒协议已在 docs/06 §5.7 定义；HEAD = origin/main = eace570，工作区干净；gim-v1/gim-v1.1/gim-v1.2/gim-v1.3 存在且未移动。

本轮目标：按 docs/06 §5.7 实现好友上线提醒（friend.online / friend.offline）的后端通知、前端实时更新、测试与文档；完成后打 annotated tag gim-v1.4 并 push。

授权范围：

- 后端：在 Chat Hub 的本地 0→1 / 1→0 回调基础上，新增有界异步 worker，查询好友并推送 friend.online/friend.offline；不改 Hub 内部并发结构，不在回调内做 I/O。
- 前端：Chat WS 接收事件，更新 stores/friend.ts 的 isOnline；事件丢失时仍可通过 HTTP 好友列表恢复。
- 测试：单测、集成、E2E、race。
- 文档：docs/06 §5.7 从“设计”改为“已实现”；同步 docs/05/07/08、README.md、docs/validation-status.md、docs/known-issues.md。
- 打 tag gim-v1.4 并 push。
- 操作日志 agent_logs/operations/day-16/。

明确不做：

- 不实现 user_confs、管理员撤回、语音/视频/@/图文混排。
- 不实现多实例全局 Presence 判断；本轮只做单机语义，worker 中预留接口并标注“多实例 V2”。
- 不新增表/列/索引；好友关系复用 friends 表。
- 不重构 Hub/Client、幂等、session、Presence 存储、图片/文件/撤回实现。
- 不放宽测试、不虚构结果、不 --force、不移动/删除已有 tag、不打 gim-v2.0。

如果必须偏离（例如需要改 Hub 并发模型、需要新增 Redis key、需要改 Presence 写入方），先停止，输出原因、影响、备选方案、推荐方案，等我决定。

## 1. 红线

- reference/ 只读，禁止复制。
- 不提交密钥、私钥、真实配置值。
- 日志不记录密码、JWT、Authorization、Cookie、Secret。
- 不虚构测试、commit、push、tag 结果；环境不可用记录 NOT RUN/SKIP。
- 每个可独立验收操作写 agent_logs/operations/day-16/；没有记录视为未完成。
- commit 前 git status、git diff --check、对应测试、确认无 .env/reference//密钥进入 staged。
- Group WS 不得修改 Presence，不得发送好友事件。

## 2. 先落盘本轮指令

- 将本指令全文写入 agent_logs/prompts/round-13.md。
- 输出文件行数和前 5 行确认。
- 执行 git status、git log --oneline -n 5、git tag，确认工作区干净、HEAD = eace570、四个 tag 存在。

## 3. 任务一：后端实现

### 3.1 阅读现状

- server/internal/chat/hub.go、lifecycle.go、presence/：现有 LocalConnectionTransitionObserver 的签名、调用时机、是否在 Hub 事件循环内。
- server/internal/chat/ 中 NewHub 的构造参数，确认“只接收一个 observer”的具体位置。
- server/internal/user/friend/：好友列表查询接口、friends 表结构、是否已有“查询某用户的所有好友 ID”的方法。
- server/cmd/chat-api/main.go：Hub 装配位置。
- docs/06 §5.7：协议约束。

### 3.2 设计（先写操作日志，再编码）

不修改 Hub 内部结构。在 chat-api 装配层组合两个 observer：

- 原 observer（Presence 写入）保持不变；
- 新增 friendNotifier，实现同一 observer 接口，内部把事件投递到有界 channel。

worker：

- 有界 buffered channel（容量配置化，默认 256）；
- 单个或少量 goroutine 消费；
- 收到 0→1：查该用户好友列表 → 对在线好友异步发送 friend.online；
- 收到 1→0：同理发送 friend.offline；
- 跳过：自己、已删除好友、被封禁/停用用户、查询失败；
- 发送失败记录日志（脱敏），不重试、不阻塞。

多端语义：observer 回调本身只在本地 0→1 / 1→0 触发；worker 不再判断连接数。

多实例预留：worker 内保留一个 globalPresenceCheck 接口占位（返回 bool），单机实现直接返回 true；多实例实现标注 V2，不本轮实现。

发送路径：复用现有 Hub SendToUser 或写队列，不直接 Conn.WriteMessage；如果 SendToUser 在 Hub 事件循环内，确认它已经是非阻塞 select。

不改 Presence 写入：gim:presence:user:{uid} 仍只由原 observer 维护；好友通知只读 Presence，不写。

### 3.3 实现

新增 server/internal/chat/friendnotify/（或项目现有风格下的等价位置），包含：

- observer.go：实现 observer 接口，投递事件；
- worker.go：有界 channel、消费循环、好友查询、发送；
- worker_test.go：单测。

修改 server/cmd/chat-api/main.go：装配两个 observer，启动 worker，优雅关闭时停止 worker。

协议层：friend.online/friend.offline 的 envelope 构造，复用现有 chat.message 的 envelope 风格；写入 docs/06 §5.7 的 payload 结构。

配置：worker channel 容量、worker 数量（如需要）写入配置，默认值合理；不新增必填 env 键。

### 3.4 测试

单测：

- observer 收到 0→1 / 1→0 后正确入队；
- worker 对好友列表正确过滤（自己、已删除、封禁）；
- 好友查询失败不 panic、不阻塞；
- channel 满时丢弃并记录，不阻塞 Hub 回调；
- 优雅关闭不丢事件、不 panic。

集成（环境可用则跑）：

- A 登录（Chat WS）→ B 收到 friend.online；
- A 关闭全部 Chat 连接 → B 收到 friend.offline；
- A 多端登录 → B 只收到一次 friend.online；A 关闭一端 → B 不收到 friend.offline；A 关闭最后一端 → B 收到 friend.offline；
- 非好友 C 不收到 A 的事件。

go build/vet/gofmt/test PASS。

## 4. 任务二：前端实现

### 4.1 接收与更新

Chat WS 的 onmessage 增加 friend.online/friend.offline 分支；Group WS 不处理。

更新 stores/friend.ts：

- 按 userID 更新对应好友的 isOnline；
- 未知 userID（例如新加好友但本地列表未刷新）忽略，不报错；
- 事件丢失时，GET /api/user/friends 仍是权威恢复路径。

好友列表/通讯录实时反映在线状态；不新增轮询。

### 4.2 测试

vitest：

- friend.online 更新对应好友；
- friend.offline 更新对应好友；
- 未知 userID 不报错；
- 事件与 HTTP 列表合并时不重复、不冲突。

vue-tsc、vite build PASS。

E2E（环境可用则跑）：

- A、B 互为好友；
- A 登录 → B 的好友列表显示 A 在线；
- A 关闭浏览器 → B 的好友列表显示 A 离线；
- A 重新登录 → B 再次看到在线。

不可用则记录 NOT RUN。

## 5. 任务三：全量 race

- Linux 容器 golang:1.25-bookworm，挂载 module cache，GOPROXY=off。
- go test -race ./... -count=1。
- 重点覆盖 internal/chat/friendnotify/...、internal/chat/...、internal/user/friend/...。
- 记录 exit code、是否有 WARNING: DATA RACE。
- 发现 DATA RACE 先停止报告，不扩大修复范围。

## 6. 任务四：文档与验证状态

- docs/06_WEBSOCKET_PROTOCOL.md：§5.7 从“设计”改为“已实现”，补充实际 payload、触发时机、过滤规则、多端语义、多实例限制（V2）。
- docs/05_HTTP_API.md：好友列表 isOnline 与 WS 事件的关系确认。
- docs/07_CORE_FLOWS.md：§1.1 好友上线提醒流程从“设计”改为“已实现”。
- docs/08_REDIS_CONFIG.md：确认 Presence key/TTL 未变；好友通知只读。
- docs/known-issues.md：
  - V-19 好友实现移入已验证；
  - 新增：多实例全局 Presence 判断未实现（V2）；
  - 新增：事件 best-effort，进程崩溃/TTL 过期不保证 offline 事件；
  - K-14/K-15 保持。
- docs/validation-status.md：好友上线提醒单测/集成/前端/E2E/race 真实状态。
- README.md：V2 进度“图片 + 文件 + 撤回 + 好友上线提醒已实现；user_confs、管理员撤回、多实例 Presence 待”。

## 7. 任务五：打 gim-v1.4 tag

确认任务一至四的 commit 已提交并推送。

执行：

```bash
git tag -a gim-v1.4 -m "GIM V1.4: image/file/recall/friend-online messages implemented and validated; user_confs, admin recall, multi-instance presence pending"
```

指向本轮最终通过验收的功能/文档 commit。

- git push origin main，再 git push origin gim-v1.4。
- git ls-remote --tags origin 确认远端同时有 gim-v1、gim-v1.1、gim-v1.2、gim-v1.3、gim-v1.4，前四个 hash 未变。
- 操作日志记录 tag hash、push 结果、旧 tag 是否保持不变。

禁止：git tag -f 移动旧 tag、git push --force 覆盖远端 tag、删除已有 tag。

## 8. 执行顺序与 commit

```text
落盘 round-13.md
  -> 任务一后端 -> 单测/集成 -> commit（feat(chat): notify friends on presence transitions）
  -> 任务二前端 -> 测试 -> commit（feat(web): apply friend online events）
  -> 任务三 race -> 操作日志
  -> 任务四文档与验证状态 -> commit（docs: record friend online reminder support）
  -> 任务五打 tag + push
  -> 全量汇总
```

每个 commit 前：git status、git diff --check、对应测试、确认操作日志存在、确认无 .env/reference//密钥进入 staged。

不压成一个 commit。

## 9. 本轮结束输出

- 已完成/未完成。
- 后端修改文件与测试真实结果（PASS/FAIL/SKIP/NOT RUN）。
- 前端修改文件与测试真实结果。
- 集成/E2E/race 是否执行，结果或原因。
- 实现与协议 §5.7 的差异（如有）。
- 多实例预留接口的现状与限制。
- 文档修改清单。
- 操作日志清单。
- commit hash 与 push 结果。
- tag gim-v1.4 指向的 commit hash；前四个 tag 是否未移动。
- 未验证项与剩余问题。
- 下一步建议：user_confs / 管理员撤回 / 性能 / 是否准备 gim-v2.0。

## 10. 遇到以下情况停止并问我

- 需要修改 Hub 内部并发模型才能实现好友通知。
- 需要新增 Redis key 或修改 Presence 写入方。
- 好友查询接口不存在，需要新增 RPC/查询。
- 好友过滤语义需要产品决定（例如是否通知“单向好友”、是否通知被封禁用户）。
- worker channel 容量/丢弃策略需要产品决定。
- race 发现 DATA RACE，修复超出好友通知范围。
- 集成/E2E 环境不可用，是否允许以“单测 + 前端测试 PASS，集成/E2E NOT RUN”收尾。
- tag push 失败（认证/网络/non-fast-forward）。
