<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Message } from '@arco-design/web-vue'

import { adminApi, type AdminOverview } from '@/api/admin'
import { PublicApiError } from '@/api/client'

const chartReady = ref(false)
const loading = ref(false)
const summary = ref<AdminOverview | null>(null)

async function load(): Promise<void> {
  loading.value = true
  try {
    summary.value = await adminApi.overview()
  } catch (error) {
    Message.error(error instanceof PublicApiError ? error.message : '概览加载失败')
  } finally {
    loading.value = false
  }
}

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

onMounted(load)
</script>

<template>
  <section class="dashboard-grid">
    <a-card :bordered="false" class="welcome-card">
      <span class="eyebrow">GIM Admin</span>
      <h1>欢迎进入管理控制台</h1>
      <p>概览来自 Settings/Admin 只读聚合接口，不使用 FIM Mock-only /api/data/*。</p>
    </a-card>
    <a-card :bordered="false" title="运行状态" class="status-card">
      <a-space direction="vertical" fill>
        <a-alert type="success">Gateway / Auth / Admin API 基础链路已就绪</a-alert>
        <a-alert type="info">Dashboard 展示当前数据库的真实聚合计数</a-alert>
        <a-button type="outline" @click="verifyCharts">验证 ECharts 按需运行时</a-button>
        <a-tag v-if="chartReady" color="green">ECharts runtime ready</a-tag>
      </a-space>
    </a-card>
    <a-card :bordered="false" title="数据概览" class="overview-card" :loading="loading">
      <div v-if="summary" class="metric-grid">
        <div><strong>{{ summary.usersTotal }}</strong><span>用户总数</span></div>
        <div><strong>{{ summary.usersActive }}</strong><span>启用用户</span></div>
        <div><strong>{{ summary.usersDisabled }}</strong><span>禁用用户</span></div>
        <div><strong>{{ summary.privateMessages }}</strong><span>私聊消息</span></div>
        <div><strong>{{ summary.groupsActive }}</strong><span>活跃群聊</span></div>
        <div><strong>{{ summary.groupMessages }}</strong><span>群聊消息</span></div>
        <div><strong>{{ summary.userFiles }}</strong><span>文件引用</span></div>
        <div><strong>{{ summary.auditEvents }}</strong><span>审计事件</span></div>
      </div>
      <a-empty v-else-if="!loading" description="暂无概览数据" />
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
.overview-card { grid-column: 1 / -1; min-height: 260px; }
.metric-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
.metric-grid > div { display: flex; min-height: 110px; flex-direction: column; justify-content: center; padding: 18px; border: 1px solid #e5e8ef; border-radius: 10px; background: #f7f8fa; }
.metric-grid strong { color: #1d4f91; font-size: 30px; }
.metric-grid span { margin-top: 8px; color: #86909c; }
@media (max-width: 960px) { .dashboard-grid { grid-template-columns: 1fr; } .overview-card { grid-column: auto; } .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
