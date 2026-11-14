# 002 / Task 2 — README 文档索引

- **日期**：2026-11-14（Asia/Shanghai）
- **结果**：README“文档入口”由 5 条 bullet 整理为表格，新增架构深度文档、V2 范围、面试设计问答、简历表述、架构/可靠性问答。

## 修改前

```text
- [HTTP API]...、[WebSocket 协议]...、[核心流程]...
- [数据库设计]...、[Redis/配置]...
- [验证状态]...、[测试验收]...、[已知问题]...
- [Compose 部署]...、[性能基线]...
- [面试材料](docs/interview/project-overview.md)
```

## 修改后

表格 17 行，包含：

- HTTP API、WebSocket 协议、核心流程
- 数据库设计、Redis/配置
- 架构深度文档（`docs/architecture-deep-dive.md`）
- V2 范围（`docs/v2-scope.md`）
- 验证状态、测试验收、已知问题
- 面试设计问答（`docs/interview/design-questions.md`）
- 项目概览、简历表述、架构问答、可靠性问答
- Compose 部署、性能基线

原有链接全部保留；仅改“文档入口”一节。

## 链接抽查

17 个链接均 `Test-Path=True`（HTTP/WS/流程/DB/Redis/架构深度/V2/验证/验收/known-issues/设计问答/项目概览/简历/架构问答/可靠性问答/Compose/性能）。

## 口径

- 未新增“全量验证通过/百万连接”等声明。
- 性能基线注明“非容量承诺”。

## Git

- commit：`docs: index architecture and interview docs in readme`（本记录所在提交）。
