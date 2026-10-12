<script setup lang="ts">
import { ref } from 'vue'

import type { CreateGroupInput, GroupSession, GroupSummary } from '@/types/group'

defineProps<{
  sessions: GroupSession[]
  groups: GroupSummary[]
  currentGroupID: number | null
  loading: boolean
}>()
const emit = defineEmits<{
  select: [groupID: number]
  create: [input: CreateGroupInput]
}>()

const creating = ref(false)
const groupName = ref('')
const memberIDs = ref('')

function preview(session: GroupSession): string {
  return session.lastMessage?.msg.textMsg.content || '暂无消息'
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
}

function submitCreate(): void {
  const name = groupName.value.trim()
  if (!name) return
  const userIdList = [...new Set(memberIDs.value.split(',').map((value) => Number(value.trim())).filter((value) => Number.isSafeInteger(value) && value > 0))]
  emit('create', { name, userIdList, size: Math.max(100, userIdList.length + 1), isSearch: true })
  groupName.value = ''
  memberIDs.value = ''
  creating.value = false
}
</script>

<template>
  <aside class="group-sidebar" aria-label="群聊导航">
    <header>
      <div><strong>群聊</strong><span>会话与我的群分开维护</span></div>
      <button type="button" class="create-button" @click="creating = !creating">{{ creating ? '取消' : '建群' }}</button>
    </header>
    <form v-if="creating" class="create-form" @submit.prevent="submitCreate">
      <input v-model="groupName" maxlength="40" placeholder="群名称" aria-label="群名称">
      <input v-model="memberIDs" placeholder="成员 ID，逗号分隔" aria-label="邀请成员 ID">
      <button type="submit" :disabled="!groupName.trim()">创建</button>
    </form>

    <section class="list-section">
      <h2>群会话 <span v-if="loading">同步中</span></h2>
      <button
        v-for="session in sessions"
        :key="`session:${session.group.groupID}`"
        type="button"
        class="list-item"
        :class="{ active: session.group.groupID === currentGroupID }"
        @click="emit('select', session.group.groupID)"
      >
        <el-avatar :src="session.group.avatar || undefined">{{ session.group.name.slice(0, 1) }}</el-avatar>
        <span class="item-copy">
          <span class="item-title"><strong>{{ session.group.name }}</strong><time>{{ formatTime(session.lastMessageAt) }}</time></span>
          <span class="preview">{{ preview(session) }}</span>
        </span>
        <span v-if="session.isTop" class="top-mark">置顶</span>
        <span v-if="session.unreadCount" class="unread">{{ session.unreadCount > 99 ? '99+' : session.unreadCount }}</span>
      </button>
      <p v-if="!sessions.length" class="empty">暂无群会话</p>
    </section>

    <section class="list-section my-groups">
      <h2>我的群 <span>{{ groups.length }}</span></h2>
      <button
        v-for="group in groups"
        :key="`group:${group.groupId}`"
        type="button"
        class="list-item compact"
        :class="{ active: group.groupId === currentGroupID }"
        @click="emit('select', group.groupId)"
      >
        <el-avatar :src="group.avatar || undefined" :size="34">{{ group.title.slice(0, 1) }}</el-avatar>
        <span class="item-copy"><strong>{{ group.title }}</strong><span class="preview">{{ group.memberCount }} 人 · {{ group.role === 1 ? '群主' : group.role === 2 ? '管理员' : '成员' }}</span></span>
      </button>
      <p v-if="!groups.length" class="empty">还没有加入群聊</p>
    </section>
  </aside>
</template>

<style scoped>
.group-sidebar { min-width: 0; overflow-y: auto; border-right: 1px solid #e7ebf1; background: #fff; }
header { height: 72px; display: flex; align-items: center; justify-content: space-between; padding: 0 16px; border-bottom: 1px solid #edf0f4; }
header div { display: flex; flex-direction: column; gap: 3px; }
header strong { font-size: 18px; }
header span, h2 span { color: #8993a4; font-size: 10px; font-weight: 400; }
.create-button, .create-form button { border: 0; border-radius: 7px; padding: 7px 10px; background: #315ea8; color: #fff; cursor: pointer; }
.create-form { display: grid; gap: 7px; padding: 10px 14px; background: #f7f9fc; }
.create-form input { min-width: 0; border: 1px solid #dfe5ed; border-radius: 7px; padding: 8px 9px; }
.create-form button:disabled { opacity: .5; cursor: not-allowed; }
.list-section h2 { margin: 0; padding: 14px 16px 8px; color: #6e7b8e; font-size: 12px; text-transform: uppercase; letter-spacing: .08em; }
.list-item { position: relative; width: 100%; display: grid; grid-template-columns: 42px minmax(0, 1fr); gap: 10px; padding: 12px 15px; border: 0; border-bottom: 1px solid #f1f3f6; background: transparent; color: inherit; text-align: left; cursor: pointer; }
.list-item:hover, .list-item.active { background: #eef4ff; }
.list-item.compact { grid-template-columns: 36px minmax(0, 1fr); padding-top: 9px; padding-bottom: 9px; }
.item-copy { min-width: 0; display: flex; flex-direction: column; gap: 5px; }
.item-title { display: flex; justify-content: space-between; gap: 7px; }
.item-title time { color: #98a2b2; font-size: 9px; white-space: nowrap; }
.item-title strong, .preview { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview { color: #788497; font-size: 11px; }
.unread { position: absolute; right: 12px; bottom: 10px; min-width: 20px; height: 20px; padding: 0 6px; border-radius: 10px; background: #e5484d; color: #fff; font-size: 10px; line-height: 20px; text-align: center; }
.top-mark { position: absolute; right: 12px; top: 31px; color: #5477ad; font-size: 9px; }
.list-item:has(.unread) .top-mark { display: none; }
.empty { margin: 0; padding: 12px 16px 18px; color: #9aa4b3; font-size: 12px; }
.my-groups { border-top: 5px solid #f3f5f8; }
</style>
