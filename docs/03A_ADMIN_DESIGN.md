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
6. 日志管理：Kafka 日志消费后的列表、详情已读、删除。

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
