<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'

import GroupComposer from '@/components/group/GroupComposer.vue'
import GroupInfoPanel from '@/components/group/GroupInfoPanel.vue'
import GroupMessageList from '@/components/group/GroupMessageList.vue'
import GroupSidebar from '@/components/group/GroupSidebar.vue'
import { useGroupStore } from '@/stores/group'
import type { CreateGroupInput } from '@/types/group'

const group = useGroupStore()
const route = useRoute()
const router = useRouter()

const routeGroupID = computed(() => {
  const raw = route.params.groupId
  const parsed = typeof raw === 'string' ? Number(raw) : 0
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : null
})

watch(routeGroupID, (groupID) => {
  if (groupID) void group.openGroup(groupID)
  else group.closeGroup()
}, { immediate: true })

watch(() => group.currentUnavailable, (unavailable) => {
  if (unavailable && routeGroupID.value) void router.replace({ name: 'group' })
})

function selectGroup(groupID: number): void {
  void router.push({ name: 'group', params: { groupId: String(groupID) } })
}

async function createGroup(input: CreateGroupInput): Promise<void> {
  const created = await group.createGroup(input)
  if (!created) return
  ElMessage.success('群聊已创建')
  selectGroup(created.groupId)
}

async function setMute(memberID: number, muted: boolean): Promise<void> {
  const success = await group.setMemberMute(memberID, muted)
  if (success) ElMessage.success(muted ? '已禁言 5 分钟' : '已解除禁言')
  else ElMessage.error(group.lastError || '操作失败')
}

function updateActiveState(): void {
  group.setGroupActive(Boolean(routeGroupID.value) && document.visibilityState === 'visible' && document.hasFocus())
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
  group.setGroupActive(false)
})
</script>

<template>
  <section class="group-shell" :class="{ 'with-info': Boolean(group.currentGroup) }">
    <GroupSidebar
      :sessions="group.sessions"
      :groups="group.groups"
      :current-group-i-d="group.currentGroupID"
      :loading="group.sessionsLoading || group.groupsLoading"
      @select="selectGroup"
      @create="createGroup"
    />
    <main v-if="group.currentGroupID" class="conversation">
      <header class="conversation-header">
        <div><strong>{{ group.currentGroup?.title || `群 ${group.currentGroupID}` }}</strong><span>群聊 · 服务端持久化 ACK</span></div>
        <span class="connection-pill" :data-state="group.connectionState">{{ group.connectionState }}</span>
      </header>
      <GroupMessageList
        :messages="group.currentMessages"
        :current-user-i-d="group.currentUserID"
        :has-more="group.currentHistory.hasMore"
        :loading="group.currentHistory.loading"
        :sender-name="group.senderName"
        @load-older="group.loadOlder()"
        @retry="group.retryMessage"
      />
      <p v-if="group.lastError" class="group-error">{{ group.lastError }}</p>
      <GroupComposer :disabled="group.currentUnavailable" :connection-state="group.connectionState" @send="group.sendText" />
    </main>
    <main v-else class="conversation-empty"><div><strong>选择一个群聊</strong><span>群会话和“我的群”独立展示，历史按消息游标加载。</span></div></main>
    <GroupInfoPanel v-if="group.currentGroup" :group="group.currentGroup" :members="group.currentMembers" :current-user-i-d="group.currentUserID" @mute="setMute" />
  </section>
</template>

<style scoped>
.group-shell { height: calc(100vh - 130px); min-height: 520px; display: grid; grid-template-columns: minmax(235px, 29%) minmax(0, 1fr); overflow: hidden; border: 1px solid #e2e7ee; border-radius: 16px; background: #fff; box-shadow: 0 18px 45px rgba(39, 55, 84, .08); }
.group-shell.with-info { grid-template-columns: minmax(220px, 25%) minmax(0, 1fr) minmax(190px, 22%); }
.conversation { min-width: 0; min-height: 0; display: grid; grid-template-rows: 72px minmax(0, 1fr) auto auto; }
.conversation-header { display: flex; align-items: center; justify-content: space-between; padding: 0 20px; border-bottom: 1px solid #edf0f4; }
.conversation-header div { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.conversation-header strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conversation-header span { color: #8d97a6; font-size: 11px; }
.connection-pill { padding: 5px 9px; border-radius: 12px; background: #f1f3f6; text-transform: capitalize; }
.connection-pill[data-state='connected'] { background: #e9f7ef; color: #24754d; }
.connection-pill[data-state='reconnecting'] { background: #fff6df; color: #9a6617; }
.group-error { margin: 0; padding: 7px 18px; background: #fff3f3; color: #b33c3c; font-size: 12px; }
.conversation-empty { display: grid; place-items: center; background: linear-gradient(145deg, #f9fbfd, #f1f5fa); }
.conversation-empty div { display: flex; flex-direction: column; gap: 8px; color: #66758b; text-align: center; }
.conversation-empty strong { color: #25324a; font-size: 20px; }
@media (max-width: 1050px) { .group-shell.with-info { grid-template-columns: 220px minmax(0, 1fr); } .group-shell.with-info :deep(.info-panel) { display: none; } }
</style>
