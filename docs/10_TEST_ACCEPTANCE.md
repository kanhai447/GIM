# 10 测试与最终验收

## 1. 后端最低测试

- `go test ./...`
- 对 Hub/Client、幂等 Service、session unread 写单测/集成测试。
- 关键并发包运行 `go test -race`。
- migration 在空库执行一次，并在已有数据模拟升级一次。

## 2. WebSocket 场景

1. A/B 正常私聊。
2. B 离线，A 发消息；B 上线后历史和未读正确。
3. A 同时 3-5 个标签页，多端同步且上线只触发一次。
4. 同一个 clientMsgId 连续发送两次，只有一条 DB 消息。
5. 同一 sender 的 50 个并发相同 clientMsgId 由真实 MySQL UNIQUE 收敛到一行，所有成功结果返回同一 messageId。
6. ACK 丢失后重连重试返回同一 messageId，receiver 不重复收到 realtime event。
7. receiver 离线仍持久化并 ACK；在线多设备与跨实例连接各收到一次。
8. 断网/浏览器强关，心跳超时后 Presence 清理。
9. 多用户同时给同一用户发消息，无 concurrent write。
10. 群聊多人同时发；禁言用户被拒绝；管理员权限正确。
11. 前端 WS 自动重连后 session 与未读重新同步。

## 3. 文件场景

- 图片/普通文件上传与访问。
- 大文件不会 `ReadAll`。
- 超限拒绝。
- 同内容不同文件名秒传/去重正确。
- 用户 A/B 对同一物理文件有各自 user_file 关系。
- DB 写失败/磁盘失败不会留下不可控垃圾临时文件。

## 4. 数据库

对私聊历史、群历史、session 列表运行 EXPLAIN，确认命中设计索引。禁止用 `MAX(msg_preview)` 代表最新消息。

## 5. 最终交付

README 至少包含：项目介绍、架构、技术栈、目录、环境要求、一键依赖启动、数据库 migration、各服务启动顺序、前端启动、默认配置、测试、已知限制。

## 6. 简历真实性

只写真实实现/测试过的能力。若只有单节点，不写“分布式 WebSocket 路由”；若未做百万连接压测，不写“支持百万长连接”。可以写“基于 Hub + goroutine/channel 实现并发连接管理，并通过 race test/自建并发测试验证”。

---

# Admin 与日志补充验收

- 普通用户 token 调用任一 Admin API 必须失败。
- 管理员可查看用户、私聊记录、群/群消息、文件、日志，并可执行已有管理操作。
- Admin 不能依赖 `/api/data/*` Mock 才能进入核心管理页面；Mock-only Dashboard 必须明确标记/隐藏。
- 日志中搜索不到测试密码明文、JWT、Authorization/Cookie 等敏感字段。
- 并发登录/管理操作下日志内容不得串线；相关日志代码应执行 race test。
- Admin 日志详情不使用未清洗用户内容的 `v-html`。
- GIM Git 仓库扫描不得包含 TLS 私钥、真实第三方 Secret、真实 JWT Secret、生产数据库密码。
