<script setup lang="ts">
import type { FriendRequest } from '@/types/friend'

defineProps<{ requests: FriendRequest[]; loading: boolean }>()
defineEmits<{ accept: [id: number]; reject: [id: number] }>()
</script>

<template>
  <section class="request-panel">
    <header>
      <div><p class="eyebrow">Requests</p><h2>好友申请</h2></div>
      <span>{{ requests.length }} 条待处理</span>
    </header>
    <div v-if="loading" class="empty-state">正在加载申请…</div>
    <div v-else-if="!requests.length" class="empty-state"><strong>暂无待处理申请</strong><span>收到的好友申请会显示在这里。</span></div>
    <ul v-else class="request-list">
      <li v-for="request in requests" :key="request.id" class="request-row" :data-request-id="request.id">
        <div class="request-main">
          <strong class="request-name">{{ request.requester.nickname }}</strong>
          <span class="request-meta">ID {{ request.requester.userID }}</span>
          <p v-if="request.message" class="request-message">{{ request.message }}</p>
        </div>
        <div class="request-actions">
          <el-button size="small" type="primary" @click="$emit('accept', request.id)">同意</el-button>
          <el-button size="small" plain @click="$emit('reject', request.id)">拒绝</el-button>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.request-panel { padding: 22px 24px; border: 1px solid #e2e7ee; border-radius: 16px; background: #fff; box-shadow: 0 14px 38px rgba(39,55,84,.07); }
header { display: flex; align-items: end; justify-content: space-between; gap: 20px; padding-bottom: 16px; border-bottom: 1px solid #edf0f4; }
header h2, header p { margin: 0; } header span { color: #8993a4; font-size: 12px; }
.eyebrow { color: #7b8798; font-size: 11px; letter-spacing: .16em; text-transform: uppercase; }
.request-list { margin: 0; padding: 0; list-style: none; }
.request-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 14px 0; border-bottom: 1px solid #edf0f4; }
.request-main { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.request-name { overflow-wrap: anywhere; }
.request-meta, .request-message { color: #7b8798; font-size: 12px; }
.request-message { margin: 0; overflow-wrap: anywhere; }
.request-actions { display: flex; gap: 2px; }
.empty-state { min-height: 160px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 7px; color: #8993a4; }
.empty-state strong { color: #45536a; }
</style>
