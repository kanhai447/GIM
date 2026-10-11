<script setup lang="ts">
import type { ChatSession } from '@/types/chat'

defineProps<{ sessions: ChatSession[]; currentPeerID: number | null; loading: boolean }>()
const emit = defineEmits<{ select: [peerID: number]; toggleTop: [peerID: number, isTop: boolean] }>()

function preview(session: ChatSession): string {
  return session.lastMessage?.msg.textMsg.content || '暂无消息'
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
}
</script>

<template>
  <aside class="chat-sessions" aria-label="私聊会话">
    <header>
      <div><strong>消息</strong><span>{{ sessions.length }} 个会话</span></div>
      <span v-if="loading" class="loading-label">同步中</span>
    </header>
    <div v-if="sessions.length" class="session-items">
      <div
        v-for="session in sessions"
        :key="session.peer.userID"
        class="session-item"
        :class="{ active: session.peer.userID === currentPeerID }"
        role="button"
        tabindex="0"
        @click="emit('select', session.peer.userID)"
        @keydown.enter="emit('select', session.peer.userID)"
      >
        <el-avatar :src="session.peer.avatar || undefined">{{ session.peer.nickname.slice(0, 1) }}</el-avatar>
        <span class="session-copy">
          <span class="session-title">
            <strong>{{ session.peer.nickname || `用户 ${session.peer.userID}` }}</strong>
            <time>{{ formatTime(session.lastMessageAt) }}</time>
          </span>
          <span class="session-preview">{{ preview(session) }}</span>
        </span>
        <span v-if="session.unreadCount" class="unread-badge" :aria-label="`${session.unreadCount} 条未读`">
          {{ session.unreadCount > 99 ? '99+' : session.unreadCount }}
        </span>
        <button
          type="button"
          class="top-button"
          :title="session.isTop ? '取消置顶' : '置顶会话'"
          @click.stop="emit('toggleTop', session.peer.userID, !session.isTop)"
        >{{ session.isTop ? '已置顶' : '置顶' }}</button>
      </div>
    </div>
    <div v-else class="empty-sessions">{{ loading ? '正在加载会话…' : '暂无私聊会话' }}</div>
  </aside>
</template>

<style scoped>
.chat-sessions { min-width: 0; border-right: 1px solid #e7ebf1; background: #fff; }
header { height: 72px; display: flex; align-items: center; justify-content: space-between; padding: 0 18px; border-bottom: 1px solid #edf0f4; }
header div { display: flex; flex-direction: column; gap: 3px; }
header strong { font-size: 18px; }
header span, .loading-label { color: #8993a4; font-size: 11px; }
.session-items { overflow-y: auto; max-height: calc(100vh - 202px); }
.session-item { position: relative; width: 100%; display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 11px; padding: 14px 16px; border-bottom: 1px solid #f1f3f6; background: transparent; color: inherit; text-align: left; cursor: pointer; }
.session-item:hover, .session-item.active { background: #eef4ff; }
.session-copy { min-width: 0; display: flex; flex-direction: column; gap: 6px; }
.session-title { display: flex; justify-content: space-between; gap: 8px; }
.session-title strong, .session-preview { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.session-title time { color: #98a2b2; font-size: 10px; white-space: nowrap; }
.session-preview { max-width: calc(100% - 28px); color: #788497; font-size: 12px; }
.unread-badge { position: absolute; right: 14px; bottom: 13px; min-width: 20px; height: 20px; padding: 0 6px; border-radius: 10px; background: #e5484d; color: #fff; font-size: 11px; line-height: 20px; text-align: center; }
.top-button { position: absolute; right: 12px; top: 34px; border: 0; background: transparent; color: #5e7eae; font-size: 10px; cursor: pointer; opacity: 0; }
.session-item:hover .top-button, .top-button:focus-visible { opacity: 1; }
.session-item:has(.unread-badge) .top-button { display: none; }
.empty-sessions { padding: 36px 18px; color: #909aaa; text-align: center; font-size: 13px; }
</style>
