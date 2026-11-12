<template>
  <section class="welcome-panel">
    <div>
      <span class="eyebrow">GIM V1</span>
      <h1>通信从可靠的基础开始</h1>
      <p>登录、好友、私聊、群聊、文件与结构化审计已接入。消息先持久化再 ACK，会话与未读由服务端统一维护，断线后可从历史恢复。</p>
    </div>
    <div class="status-grid">
      <article><strong>私聊 / 群聊</strong><span>双 WebSocket · ACK · 幂等</span></article>
      <article><strong>好友</strong><span>搜索 · 申请 · 备注</span></article>
      <article><strong>文件</strong><span>流式上传 · SHA-256 去重</span></article>
      <article><strong>审计</strong><span>结构化事件 · Admin 查询</span></article>
    </div>
  </section>
</template>

<style scoped>
.welcome-panel { min-height: 420px; display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(280px, .8fr); gap: 34px; align-items: center; padding: clamp(28px, 6vw, 72px); border-radius: 20px; background: linear-gradient(135deg, #17243d, #315ea8); color: white; box-shadow: 0 24px 60px rgba(23,36,61,.16); }
h1 { max-width: 620px; margin: 14px 0; font-size: clamp(34px, 5vw, 60px); line-height: 1.06; }
p { max-width: 650px; color: #d9e5f9; line-height: 1.8; }
.status-grid { display: grid; gap: 12px; }
.status-grid article { display: flex; justify-content: space-between; gap: 12px; padding: 18px; border: 1px solid rgba(255,255,255,.16); border-radius: 14px; background: rgba(255,255,255,.08); }
.status-grid span { color: #bdd1f2; text-align: right; }
@media (max-width: 900px) { .welcome-panel { grid-template-columns: 1fr; } }
</style>
