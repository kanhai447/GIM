<template>
  <section class="welcome-panel">
    <div>
      <span class="eyebrow">Day 1 foundation</span>
      <h1>通信从可靠的基础开始</h1>
      <p>身份、路由与服务端数据底座已就绪。实时私聊、群聊和文件能力会在后续 Checkpoint 按可靠性设计逐步接入。</p>
    </div>
    <div class="status-grid">
      <article><strong>Auth</strong><span>已接入</span></article>
      <article><strong>Chat</strong><span>待 Day 2</span></article>
      <article><strong>Group / File</strong><span>待 Day 3</span></article>
    </div>
  </section>
</template>

<style scoped>
.welcome-panel { min-height: 420px; display: grid; grid-template-columns: minmax(0, 1.4fr) minmax(280px, .8fr); gap: 34px; align-items: center; padding: clamp(28px, 6vw, 72px); border-radius: 20px; background: linear-gradient(135deg, #17243d, #315ea8); color: white; box-shadow: 0 24px 60px rgba(23,36,61,.16); }
h1 { max-width: 620px; margin: 14px 0; font-size: clamp(34px, 5vw, 60px); line-height: 1.06; }
p { max-width: 650px; color: #d9e5f9; line-height: 1.8; }
.status-grid { display: grid; gap: 12px; }
.status-grid article { display: flex; justify-content: space-between; padding: 18px; border: 1px solid rgba(255,255,255,.16); border-radius: 14px; background: rgba(255,255,255,.08); }
.status-grid span { color: #bdd1f2; }
@media (max-width: 900px) { .welcome-panel { grid-template-columns: 1fr; } }
</style>
