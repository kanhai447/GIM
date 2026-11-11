<script setup lang="ts">
import type { FriendUser } from '@/types/friend'

defineProps<{ friends: FriendUser[]; loading: boolean; selectedID: number | null }>()
defineEmits<{ select: [friend: FriendUser]; remove: [friend: FriendUser]; chat: [friend: FriendUser] }>()

function displayName(friend: FriendUser): string {
  return friend.remark || friend.nickname
}
</script>

<template>
  <section class="friend-panel">
    <header>
      <div><p class="eyebrow">Contacts</p><h2>好友</h2></div>
      <span>{{ friends.length }} 位 · 在线 {{ friends.filter((friend) => friend.isOnline).length }}</span>
    </header>
    <div v-if="loading" class="empty-state">正在加载好友…</div>
    <div v-else-if="!friends.length" class="empty-state"><strong>还没有好友</strong><span>搜索用户并发送好友申请后会显示在这里。</span></div>
    <ul v-else class="friend-list">
      <li
        v-for="friend in friends"
        :key="friend.userID"
        class="friend-row"
        :class="{ selected: selectedID === friend.userID }"
        :data-user-id="friend.userID"
        @click="$emit('select', friend)"
      >
        <span class="online-dot" :class="{ online: friend.isOnline }" :title="friend.isOnline ? '在线' : '离线'" />
        <div class="friend-main">
          <strong class="friend-name">{{ displayName(friend) }}</strong>
          <span class="friend-meta">@{{ friend.account }} · ID {{ friend.userID }} · {{ friend.isOnline ? '在线' : '离线' }}</span>
          <span v-if="friend.remark" class="friend-remark">备注：{{ friend.remark }}</span>
        </div>
        <div class="friend-actions">
          <el-button size="small" @click.stop="$emit('chat', friend)">发消息</el-button>
          <el-button size="small" type="danger" plain @click.stop="$emit('remove', friend)">删除</el-button>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.friend-panel { padding: 22px 24px; border: 1px solid #e2e7ee; border-radius: 16px; background: #fff; box-shadow: 0 14px 38px rgba(39,55,84,.07); }
header { display: flex; align-items: end; justify-content: space-between; gap: 20px; padding-bottom: 16px; border-bottom: 1px solid #edf0f4; }
header h2, header p { margin: 0; } header span { color: #8993a4; font-size: 12px; }
.eyebrow { color: #7b8798; font-size: 11px; letter-spacing: .16em; text-transform: uppercase; }
.friend-list { margin: 0; padding: 0; list-style: none; }
.friend-row { display: grid; grid-template-columns: 12px minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 14px 8px; border-bottom: 1px solid #edf0f4; border-radius: 10px; cursor: pointer; }
.friend-row.selected { background: #f2f7ff; }
.online-dot { width: 9px; height: 9px; border-radius: 50%; background: #c3cad5; }
.online-dot.online { background: #37b26c; box-shadow: 0 0 0 3px rgba(55,178,108,.16); }
.friend-main { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.friend-name { overflow-wrap: anywhere; }
.friend-meta, .friend-remark { color: #7b8798; font-size: 12px; }
.friend-remark { color: #45536a; }
.friend-actions { display: flex; gap: 2px; }
.empty-state { min-height: 180px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 7px; color: #8993a4; }
.empty-state strong { color: #45536a; }
@media (max-width: 950px) { .friend-row { grid-template-columns: 12px minmax(0, 1fr); } .friend-actions { grid-column: 2; } }
</style>
