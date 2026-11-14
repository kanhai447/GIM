# 001 / Task 1 — 面试问答：使用说明、一分钟介绍、横向问题

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：新建 `docs/interview/design-questions.md` 第 0–2 节；未改业务代码/旧文档。

## 内容

- 第 0 节：用途、与 `architecture-deep-dive.md` 的关系、阅读方式、诚实边界。
- 第 1 节：项目定位/技术栈/服务边界/V2 范围；三个亮点（消息可靠性、文件流式与授权、好友上线提醒单机）；三个主动边界（多实例 Presence、完整性能阶梯、K-17 E2E）。
- 第 2 节：9 个横向问题（Gateway 信任边界、Hub 单写、幂等/ACK、事务、锁顺序/1213、错误码、migration/索引、部署、V2 范围），每节按“一句话→展开→追问→边界”。

## 口径

- 性能只写 `validation-status.md` 的两档：HTTP c10/10s、Private 10 conn/10s@10 msg/s；不写容量结论。
- E2E 明确“部分临时脚本未入库（K-17）”。
- 引用文件：`gateway/handler.go`、`chat/hub.go`、`chat/client.go`、`chat/message/service.go`、`chat/message/repository/mysql/repository.go`、`file/service/service.go`、`file/repository/mysql/repository.go`、`chat/friendnotify/notifier.go`、`server/migrations/runner.go` 等。

## Git

- commit：`docs: add interview design questions overview`（本记录所在提交）。
