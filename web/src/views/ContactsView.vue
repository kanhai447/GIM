<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'

import FriendList from '@/components/friend/FriendList.vue'
import FriendRequestList from '@/components/friend/FriendRequestList.vue'
import { useFriendStore } from '@/stores/friend'
import type { FriendUser } from '@/types/friend'

const friendStore = useFriendStore()
const router = useRouter()
const keyword = ref('')
const selected = ref<FriendUser | null>(null)
const remarkDraft = ref('')

onMounted(() => {
  void friendStore.loadFriends()
  void friendStore.loadRequests()
})

function select(friend: FriendUser): void {
  selected.value = friend
  remarkDraft.value = friend.remark
}

async function runSearch(): Promise<void> {
  await friendStore.search(keyword.value)
}

async function sendRequest(userID: number): Promise<void> {
  if (await friendStore.sendRequest(userID)) ElMessage.success('好友申请已发送')
  else ElMessage.error(friendStore.lastError)
}

async function respond(requestID: number, status: 1 | 2): Promise<void> {
  if (await friendStore.respond(requestID, status)) ElMessage.success(status === 1 ? '已同意好友申请' : '已拒绝好友申请')
  else ElMessage.error(friendStore.lastError)
}

async function saveRemark(): Promise<void> {
  if (!selected.value) return
  if (await friendStore.updateRemark(selected.value.userID, remarkDraft.value)) {
    selected.value = friendStore.friends.find((friend) => friend.userID === selected.value!.userID) ?? null
    ElMessage.success('好友备注已保存')
  } else {
    ElMessage.error(friendStore.lastError)
  }
}

async function confirmRemove(friend: FriendUser): Promise<void> {
  try {
    await ElMessageBox.confirm(`确认删除好友“${friend.remark || friend.nickname}”？双方好友关系都会被移除。`, '删除好友', {
      type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消',
    })
  } catch {
    return
  }
  if (await friendStore.remove(friend.userID)) {
    if (selected.value?.userID === friend.userID) selected.value = null
    ElMessage.success('好友已删除')
  } else {
    ElMessage.error(friendStore.lastError)
  }
}

function openChat(friend: FriendUser): void {
  void router.push({ name: 'chat', params: { peerId: String(friend.userID) } })
}
</script>

<template>
  <main class="contacts-page">
    <section class="search-panel">
      <header>
        <div><p class="eyebrow">Find people</p><h2>找人</h2></div>
        <span>按昵称、账号或用户 ID 搜索</span>
      </header>
      <div class="search-row">
        <el-input v-model="keyword" placeholder="昵称 / 账号 / 用户 ID" clearable @keyup.enter="runSearch" />
        <el-button type="primary" :loading="friendStore.searching" @click="runSearch">搜索</el-button>
      </div>
      <ul v-if="friendStore.searchResults.length" class="search-list">
        <li v-for="result in friendStore.searchResults" :key="result.userID" class="search-item" :data-user-id="result.userID">
          <div class="search-main">
            <strong class="search-name">{{ result.nickname }}</strong>
            <span class="search-meta">@{{ result.account }} · ID {{ result.userID }}</span>
            <p v-if="result.abstract" class="search-abstract">{{ result.abstract }}</p>
          </div>
          <div class="search-actions">
            <span v-if="result.isFriend" class="tag">已是好友</span>
            <span v-else-if="friendStore.sentRequestIDs.includes(result.userID)" class="tag">已申请</span>
            <el-button v-else size="small" type="primary" @click="sendRequest(result.userID)">加好友</el-button>
          </div>
        </li>
      </ul>
      <p v-else-if="friendStore.searchKeyword && !friendStore.searching" class="search-empty">没有匹配的用户</p>
    </section>

    <p v-if="friendStore.lastError" class="contacts-error" role="alert">{{ friendStore.lastError }}</p>

    <div class="columns">
      <FriendList
        :friends="friendStore.friends"
        :loading="friendStore.loading"
        :selected-i-d="selected?.userID ?? null"
        @select="select"
        @remove="confirmRemove"
        @chat="openChat"
      />
      <FriendRequestList
        :requests="friendStore.requests"
        :loading="friendStore.requestsLoading"
        @accept="respond($event, 1)"
        @reject="respond($event, 2)"
      />
    </div>

    <section v-if="selected" class="detail-panel">
      <header>
        <div><p class="eyebrow">Contact</p><h2>{{ selected.remark || selected.nickname }}</h2></div>
        <span>{{ selected.isOnline ? '在线' : '离线' }}</span>
      </header>
      <dl class="detail-grid">
        <div><dt>账号</dt><dd>{{ selected.account }}</dd></div>
        <div><dt>用户 ID</dt><dd>{{ selected.userID }}</dd></div>
        <div><dt>昵称</dt><dd>{{ selected.nickname }}</dd></div>
        <div><dt>签名</dt><dd>{{ selected.abstract || '暂无' }}</dd></div>
      </dl>
      <div class="remark-row">
        <el-input v-model="remarkDraft" maxlength="32" placeholder="给好友设置备注（最多 32 个字符）" />
        <el-button type="primary" @click="saveRemark">保存备注</el-button>
      </div>
    </section>
  </main>
</template>

<style scoped>
.contacts-page { display: grid; gap: 20px; max-width: 1180px; margin: 0 auto; }
.columns { display: grid; grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr); gap: 20px; align-items: start; }
.search-panel, .detail-panel { padding: 22px 24px; border: 1px solid #e2e7ee; border-radius: 16px; background: #fff; box-shadow: 0 14px 38px rgba(39,55,84,.07); }
header { display: flex; align-items: end; justify-content: space-between; gap: 20px; padding-bottom: 16px; border-bottom: 1px solid #edf0f4; }
header h2, header p { margin: 0; } header span { color: #8993a4; font-size: 12px; }
.eyebrow { color: #7b8798; font-size: 11px; letter-spacing: .16em; text-transform: uppercase; }
.search-row, .remark-row { display: flex; gap: 10px; padding-top: 16px; }
.search-list { margin: 12px 0 0; padding: 0; list-style: none; }
.search-item { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px; padding: 12px 0; border-top: 1px solid #edf0f4; }
.search-main { min-width: 0; display: flex; flex-direction: column; gap: 3px; }
.search-name { overflow-wrap: anywhere; }
.search-meta, .search-abstract { color: #7b8798; font-size: 12px; }
.search-abstract { margin: 0; overflow-wrap: anywhere; }
.tag { padding: 3px 10px; border-radius: 999px; background: #eef2f7; color: #5a677d; font-size: 12px; }
.search-empty { margin: 12px 0 0; color: #8993a4; font-size: 13px; }
.contacts-error { margin: 0; padding: 10px 14px; border-radius: 8px; background: #fff3f3; color: #b33c3c; font-size: 13px; }
.detail-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 14px; margin: 18px 0 0; }
.detail-grid dt { color: #8993a4; font-size: 12px; }
.detail-grid dd { margin: 4px 0 0; color: #253046; overflow-wrap: anywhere; }
@media (max-width: 950px) { .columns { grid-template-columns: minmax(0, 1fr); } }
</style>
