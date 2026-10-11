<script setup lang="ts">
import type { ChatMessageItem } from '@/types/chat'

defineProps<{ messages: ChatMessageItem[]; currentUserID: number; hasMore: boolean; loading: boolean }>()
const emit = defineEmits<{ loadOlder: []; retry: [clientMsgId: string] }>()

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit' }).format(date)
}
</script>

<template>
  <section class="message-region" aria-live="polite">
    <div class="older-row">
      <button v-if="hasMore" type="button" :disabled="loading" @click="emit('loadOlder')">
        {{ loading ? '加载中…' : '加载更早消息' }}
      </button>
      <span v-else-if="messages.length">已经到最早的消息</span>
    </div>
    <div v-if="messages.length" class="messages">
      <article
        v-for="message in messages"
        :key="message.localID"
        class="message"
        :class="{ mine: message.sendUserID === currentUserID }"
        :data-status="message.status"
      >
        <div class="bubble">{{ message.msg.textMsg.content }}</div>
        <div class="message-meta">
          <time>{{ formatTime(message.createdAt) }}</time>
          <span v-if="message.status === 'pending'" class="pending-label">发送中</span>
          <span v-else-if="message.status === 'sent' && message.sendUserID === currentUserID" class="sent-label">已发送</span>
          <button
            v-else-if="message.status === 'failed'"
            type="button"
            class="failed-label"
            :title="message.errorMessage"
            @click="emit('retry', message.clientMsgId)"
          >发送失败 · 重试</button>
        </div>
      </article>
    </div>
    <div v-else class="empty-messages">还没有消息，发一句问候吧。</div>
  </section>
</template>

<style scoped>
.message-region { min-height: 0; overflow-y: auto; padding: 18px 24px; background: linear-gradient(180deg, #f8fafc, #f4f7fb); }
.older-row { min-height: 28px; color: #9aa4b3; font-size: 11px; text-align: center; }
.older-row button { border: 0; background: transparent; color: #4774bd; cursor: pointer; }
.messages { display: flex; flex-direction: column; gap: 15px; }
.message { display: flex; flex-direction: column; align-items: flex-start; }
.message.mine { align-items: flex-end; }
.bubble { max-width: min(72%, 620px); padding: 11px 14px; border: 1px solid #e3e8ef; border-radius: 4px 15px 15px; background: #fff; color: #25324a; line-height: 1.55; overflow-wrap: anywhere; white-space: pre-wrap; box-shadow: 0 5px 14px rgba(40, 57, 86, .05); }
.mine .bubble { border-color: #315ea8; border-radius: 15px 4px 15px 15px; background: #315ea8; color: #fff; }
.message-meta { display: flex; gap: 7px; align-items: center; margin-top: 5px; color: #9aa4b3; font-size: 10px; }
.pending-label { color: #b47a27; }
.sent-label { color: #6680a7; }
.failed-label { border: 0; padding: 0; background: transparent; color: #d24343; font-size: 10px; cursor: pointer; }
.empty-messages { display: grid; place-items: center; min-height: 260px; color: #99a3b2; }
</style>
