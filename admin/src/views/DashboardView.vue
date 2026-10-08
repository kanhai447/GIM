<script setup lang="ts">
import { ref } from 'vue'
import { Message } from '@arco-design/web-vue'

const chartReady = ref(false)

async function verifyCharts(): Promise<void> {
  try {
    const runtime = await import('@/utils/charts')
    const charts = await runtime.loadChartRuntime()
    chartReady.value = typeof charts.init === 'function'
    Message.success('ECharts 按需运行时可用')
  } catch {
    Message.error('可视化运行时加载失败')
  }
}
</script>

<template>
  <section class="dashboard-grid">
    <a-card :bordered="false" class="welcome-card">
      <span class="eyebrow">GIM Admin</span>
      <h1>欢迎进入管理控制台</h1>
      <p>当前提供稳定的身份、路由和管理模块入口。真实统计将在服务端指标接口完成后接入，不展示 Mock 数据。</p>
    </a-card>
    <a-card :bordered="false" title="运行状态" class="status-card">
      <a-space direction="vertical" fill>
        <a-alert type="success">Gateway / Auth / User 基础链路已就绪</a-alert>
        <a-alert type="info">Dashboard 真实指标尚未实现</a-alert>
        <a-button type="outline" @click="verifyCharts">验证 ECharts 按需运行时</a-button>
        <a-tag v-if="chartReady" color="green">ECharts runtime ready</a-tag>
      </a-space>
    </a-card>
    <a-card :bordered="false" title="数据概览" class="empty-card">
      <a-empty description="暂无真实统计数据；未调用 FIM Mock-only /api/data/*" />
    </a-card>
  </section>
</template>

<style scoped>
.dashboard-grid { display: grid; grid-template-columns: 1.35fr .65fr; gap: 20px; }
.welcome-card { min-height: 280px; padding: 28px; background: linear-gradient(135deg, #223451, #315ea8); color: white; }
.eyebrow { color: #9ec2ff; font-size: 11px; font-weight: 700; letter-spacing: .16em; text-transform: uppercase; }
.welcome-card h1 { max-width: 650px; margin: 18px 0 12px; font-size: clamp(34px, 4vw, 52px); line-height: 1.08; }
.welcome-card p { max-width: 680px; color: #d5e3f8; line-height: 1.8; }
.status-card { min-height: 280px; }
.empty-card { grid-column: 1 / -1; min-height: 260px; }
@media (max-width: 960px) { .dashboard-grid { grid-template-columns: 1fr; } .empty-card { grid-column: auto; } }
</style>
