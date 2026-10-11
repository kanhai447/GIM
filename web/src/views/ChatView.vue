<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import ChatComposer from '@/components/chat/ChatComposer.vue'
import ChatMessageList from '@/components/chat/ChatMessageList.vue'
import ChatSessionList from '@/components/chat/ChatSessionList.vue'
import { useChatStore } from '@/stores/chat'

const chat = useChatStore()
const route = useRoute()
const router = useRouter()

const routePeerID = computed(() => {
  const raw = route.params.peerId
  const parsed = typeof raw === 'string' ? Number(raw) : 0
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null
})

watch(routePeerID, (peerID) => {
  if (peerID) {
    void chat.openConversation(peerID)
  } else {
    chat.closeConversation()
  }
}, { immediate: true })

function selectPeer(peerID: number): void {
  void router.push({ name: 'chat', params: { peerId: String(peerID) } })
}

function updateActiveState(): void {
  chat.setConversationActive(document.visibilityState === 'visible' && document.hasFocus())
}

onMounted(() => {
  document.addEventListener('visibilitychange', updateActiveState)
  window.addEventListener('focus', updateActiveState)
  window.addEventListener('blur', updateActiveState)
  updateActiveState()
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', updateActiveState)
  window.removeEventListener('focus', updateActiveState)
  window.removeEventListener('blur', updateActiveState)
  chat.setConversationActive(false)
})
</script>

<template>
  <section class="chat-shell">
    <ChatSessionList
      :sessions="chat.sessions"
      :current-peer-i-d="chat.currentPeerID"
      :loading="chat.sessionsLoading"
      @select="selectPeer"
      @toggle-top="chat.setSessionTop"
    />
    <main v-if="chat.currentPeerID" class="conversation">
      <header class="conversation-header">
        <div>
          <strong>{{ chat.currentSession?.peer.nickname || `用户 ${chat.currentPeerID}` }}</strong>
          <span>私聊 · 服务端持久化 ACK</span>
        </div>
        <span class="connection-pill" :data-state="chat.connectionState">{{ chat.connectionState }}</span>
      </header>
      <ChatMessageList
        :messages="chat.currentMessages"
        :current-user-i-d="chat.currentUserID"
        :has-more="chat.currentHistory.hasMore"
        :loading="chat.currentHistory.loading"
        @load-older="chat.loadOlder()"
        @retry="chat.retryMessage"
      />
      <p v-if="chat.lastError" class="chat-error">{{ chat.lastError }}</p>
      <ChatComposer :disabled="!chat.currentPeerID" :connection-state="chat.connectionState" @send="chat.sendText" />
    </main>
    <main v-else class="conversation-empty">
      <div><strong>选择一个会话</strong><span>在左侧打开私聊，消息历史将按游标加载。</span></div>
    </main>
  </section>
</template>

<style scoped>
.chat-shell { height: calc(100vh - 130px); min-height: 520px; display: grid; grid-template-columns: minmax(240px, 31%) minmax(0, 1fr); overflow: hidden; border: 1px solid #e2e7ee; border-radius: 16px; background: #fff; box-shadow: 0 18px 45px rgba(39, 55, 84, .08); }
.conversation { min-width: 0; min-height: 0; display: grid; grid-template-rows: 72px minmax(0, 1fr) auto auto; }
.conversation-header { display: flex; align-items: center; justify-content: space-between; padding: 0 22px; border-bottom: 1px solid #edf0f4; }
.conversation-header div { display: flex; flex-direction: column; gap: 4px; }
.conversation-header span { color: #8d97a6; font-size: 11px; }
.connection-pill { padding: 5px 9px; border-radius: 12px; background: #f1f3f6; text-transform: capitalize; }
.connection-pill[data-state='connected'] { background: #e9f7ef; color: #24754d; }
.connection-pill[data-state='reconnecting'] { background: #fff6df; color: #9a6617; }
.chat-error { margin: 0; padding: 7px 18px; background: #fff3f3; color: #b33c3c; font-size: 12px; }
.conversation-empty { display: grid; place-items: center; background: linear-gradient(145deg, #f9fbfd, #f1f5fa); }
.conversation-empty div { display: flex; flex-direction: column; gap: 8px; color: #66758b; text-align: center; }
.conversation-empty strong { color: #25324a; font-size: 20px; }
@media (max-width: 820px) { .chat-shell { grid-template-columns: 210px minmax(0, 1fr); height: calc(100vh - 108px); } }
</style>
