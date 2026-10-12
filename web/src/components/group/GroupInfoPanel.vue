<script setup lang="ts">
import type { GroupMember, GroupSummary } from '@/types/group'

defineProps<{ group: GroupSummary; members: GroupMember[]; currentUserID: number }>()
const emit = defineEmits<{ mute: [memberID: number, muted: boolean] }>()

function roleName(role: number): string {
  return role === 1 ? '群主' : role === 2 ? '管理员' : '成员'
}
</script>

<template>
  <aside class="info-panel">
    <div class="group-card">
      <el-avatar :src="group.avatar || undefined" :size="54">{{ group.title.slice(0, 1) }}</el-avatar>
      <div><strong>{{ group.title }}</strong><span>{{ group.memberCount }} 人 · {{ roleName(group.role) }}</span></div>
    </div>
    <p class="abstract">{{ group.abstract || '暂无群简介' }}</p>
    <div class="labels"><span v-if="group.isProhibition">全员禁言</span><span>群号 {{ group.groupId }}</span></div>
    <h3>成员</h3>
    <ul>
      <li v-for="member in members" :key="member.userId">
        <el-avatar :src="member.avatar || undefined" :size="28">{{ member.userNickname.slice(0, 1) }}</el-avatar>
        <span class="member-copy"><strong>{{ member.memberNickname || member.userNickname }}</strong><small>{{ roleName(member.role) }} · {{ member.userId }}</small></span>
        <span v-if="member.userId === currentUserID" class="self">我</span>
        <span v-else-if="member.role === 1"></span>
        <span v-else-if="group.role === 1 || (group.role === 2 && member.role === 3)" class="actions">
          <button type="button" @click="emit('mute', member.userId, true)">禁言</button>
          <button type="button" @click="emit('mute', member.userId, false)">解除</button>
        </span>
      </li>
    </ul>
  </aside>
</template>

<style scoped>
.info-panel { min-width: 0; overflow-y: auto; border-left: 1px solid #e7ebf1; background: #fff; padding: 18px 15px; }
.group-card { display: flex; gap: 11px; align-items: center; }
.group-card div { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.group-card strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.group-card span, .abstract { color: #7c8798; font-size: 11px; }
.abstract { line-height: 1.55; }
.labels { display: flex; flex-wrap: wrap; gap: 6px; }
.labels span { border-radius: 10px; padding: 3px 7px; background: #edf2fa; color: #526b94; font-size: 9px; }
h3 { margin: 20px 0 8px; color: #5f6c80; font-size: 12px; }
ul { display: flex; flex-direction: column; gap: 3px; margin: 0; padding: 0; list-style: none; }
li { display: grid; grid-template-columns: 30px minmax(0, 1fr) auto; gap: 7px; align-items: center; padding: 7px 2px; }
.member-copy { min-width: 0; display: flex; flex-direction: column; }
.member-copy strong { overflow: hidden; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.member-copy small, .self { color: #8d97a6; font-size: 9px; }
.actions { display: flex; gap: 3px; }
.actions button { border: 0; padding: 2px; background: transparent; color: #5573a5; font-size: 9px; cursor: pointer; }
</style>
