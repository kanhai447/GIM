# 16 原项目参考与 GIM 自主实现规则

## 1. 核心原则

FIM 是**参考项目**，GIM 是**重新实现的新项目**。Agent 的任务不是迁移 FIM，也不是给 FIM 换名字，而是先理解 FIM，再依据 GIM 设计文档自主编写。

## 2. 可以参考什么

允许参考：

- 服务边界和业务能力：Auth/User/Chat/Group/File/Settings/Logs/Gateway；
- 用户端和 Admin 的页面流程；
- HTTP 路由意图、请求/响应语义；
- WebSocket 的业务事件、消息类型和交互场景；
- 数据实体之间的关系；
- 好友申请、禁言、撤回、文件消息等业务规则；
- 原项目暴露出的错误场景和工程问题。

参考的目的，是回答“要实现什么”和“原方案哪里需要改”。

## 3. 禁止的做法

禁止：

1. `cp -r reference/fim_* server/` 或类似整目录复制；
2. 复制整个 `.go/.ts/.vue` 文件后只修改 package/module/project 名；
3. 批量 search/replace `fim` -> `gim` 生成新项目；
4. 保留原项目目录结构仅为了方便复制；
5. 直接沿用原项目已确认存在问题的全局 WS map、并发 `Conn.Write*`、共享可变 Pusher、`io.ReadAll` 上传、敏感日志等；
6. 把 reference 当作编译依赖或通过相对路径 import 原代码；
7. 把 reference 的 Secret、私钥、真实地址、作者信息复制进 GIM。

## 4. 每个模块的正确工作流程

例如实现私聊：

```text
1. 阅读 docs/06_WEBSOCKET_PROTOCOL.md 和 docs/07_CORE_FLOWS.md
2. 查看 FIM 私聊支持哪些消息和前端期望什么行为
3. 在操作日志写“参考行为”
4. 设计 GIM 的 Hub/Client/Service/Repository 边界
5. 在 server/gim_chat 新建自己的代码
6. 编写测试
7. 对比行为兼容性
8. 在日志写“与 FIM 的差异/优化”
```

其他模块同理。

## 5. 允许的小范围复用

若确有必要，可以复用极少量不具业务创造性的内容，例如公开协议常量名、枚举值、与兼容 API 强绑定的 DTO 字段名。必须满足：

- 复用量很小；
- 不包含秘密；
- 不复制大段业务逻辑；
- 在操作日志注明来源和原因；
- 优先由 Agent 重新定义为 GIM 自己的类型。

## 6. 前端要求

`web/` 和 `admin/` 同样是新项目。可以保持原 FIM 的主要 UI 信息架构和交互以节约时间，但组件、service、store、WebSocket 管理代码应在 GIM 中重新创建。

UI 不要求为了“原创”而全部换视觉设计；重点是代码结构、协议适配和状态管理由 GIM 自己实现。

## 7. 验收问题

每个 Phase 结束，Agent 必须能回答：

- 这一阶段参考了 FIM 的哪些行为？
- 哪些实现没有沿用？为什么？
- GIM 自己新增/重构了什么？
- 如果删除 `reference/`，GIM 是否还能独立构建、测试和运行？

最后一条必须始终为“是”。
