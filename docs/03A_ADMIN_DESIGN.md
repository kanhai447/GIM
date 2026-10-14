# 03A 管理端 Admin 详细设计

## 1. 原源码技术栈

`fim_admin` 是独立 Vue3 + TypeScript + Pinia + Vue Router 应用，UI 使用 Arco Design，统计图使用 ECharts。它不是 `fim_web` 的一个页面。

GIM 目录应为：

```text
gim/
  server/
  web/
  admin/
```

## 2. 原项目真实已对接的管理能力

以下页面存在真实后端接口，应纳入 GIM：

1. 用户管理：用户列表、在线状态、消息数、群数量、限制聊天/加好友/建群/进群、删除。
2. 私聊审计：管理员查看/筛选私聊记录、删除记录。
3. 群聊管理：群列表、成员/在线数/消息数、群消息查看与删除。
4. 文件管理：文件列表、预览/下载、删除。
5. 系统设置：读取公开配置、管理员修改配置。
6. 日志管理：Kafka 日志消费后的结构化列表与详情。GIM V1 保留审计记录，不沿用参考项目的已读/无条件删除语义。

## 3. 原项目不完整的页面

### 首页数据

以下接口只存在于 Admin Mock，Server 中没有 `data_api`：

```text
GET /api/data/statistic
GET /api/data/weather
GET /api/data/login_statistic
```

因此 GIM V1 不得直接照抄后声称“真实统计完成”。优先方案：

- 先隐藏/替换天气和假统计；
- 若 Day 4 有余量，实现真实 IM 指标，例如用户总数、在线数、群数、消息数、近 7 日登录/注册；
- 不为了三个 Dashboard 接口新建第九个微服务，除非用户明确确认。

### 用户中心
原页面仅是占位，可留到 V2。

## 4. 权限

前端 `meta.role` 只用于交互。所有管理 API 必须经过 Gateway/Auth，并在服务端 AdminMiddleware 验证 `Role == 1`。

正式部署只能由 Gateway 对外暴露 API 服务；不能把内部 API 端口暴露公网后依赖可伪造的 `Role` Header。

## 5. 日志展示安全

原 Admin 使用 `v-html` 直接渲染后端拼接的日志 HTML，这是 GIM 禁止复制的模式。

GIM 日志 API 返回结构化字段，Admin 使用 Vue 组件正常渲染文本/JSON，不使用未清洗的 `v-html` 显示请求体、Header 或用户输入。

## 6. GIM Day 4 集成结果

- User/Chat/Group/File/Settings/Logs 页面均只调用 Gateway 下真实 API，使用最大 100 的 cursor 分页。
- Dashboard 通过现有 Settings/Admin aggregation runtime 的只读 `/api/settings/admin/overview` 展示真实用户、消息、群、文件和审计计数，不请求 `/api/data/*`，也不新增微服务。
- 消息详情同样使用 Vue 文本插值显示 JSON，不执行消息中的 HTML。
- 文件下载使用管理员专用受保护路由；响应不包含物理 storage path。
- Settings 表单只有公开业务 allowlist，不提供任意 key 编辑器，避免将部署 Secret 写入数据库。
- destructive action 都有显式确认；后端成功后 best-effort 发送结构化管理员审计事件，Kafka 故障不反转已提交管理事务。
