import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { PublicApiError } from '@/api/client'
import { friendApi } from '@/api/friend'
import type { FriendRequest, FriendSearchResult, FriendUser } from '@/types/friend'

const FRIEND_LIMIT = 50
const REQUEST_LIMIT = 20
const AUTH_ERROR_CODES = new Set([1203, 1204, 1205, 1206])

const FRIEND_ERROR_MESSAGES: Record<number, string> = {
  1101: '用户不存在',
  1113: '不能添加自己为好友',
  1114: '对方已经是好友',
  1115: '对方不是好友',
  1116: '已有待处理的好友申请',
  1117: '好友申请不存在',
  1118: '好友申请已处理',
  1119: '无权处理该好友申请',
}

function errorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof PublicApiError)) return fallback
  if (error.code !== undefined && FRIEND_ERROR_MESSAGES[error.code]) return FRIEND_ERROR_MESSAGES[error.code]!
  return error.message
}

export const useFriendStore = defineStore('friend', () => {
  const friends = ref<FriendUser[]>([])
  const requests = ref<FriendRequest[]>([])
  const searchResults = ref<FriendSearchResult[]>([])
  const loading = ref(false)
  const requestsLoading = ref(false)
  const searching = ref(false)
  const lastError = ref('')
  const searchKeyword = ref('')
  const sentRequestIDs = ref<number[]>([])
  let authFailureHandler: (() => void | Promise<void>) | null = null

  const onlineCount = computed(() => friends.value.filter((friend) => friend.isOnline).length)
  const friendIDs = computed(() => new Set(friends.value.map((friend) => friend.userID)))
  const pendingRequestCount = computed(() => requests.value.filter((request) => request.status === 0).length)

  function start(onAuthenticationFailure?: () => void | Promise<void>): void {
    authFailureHandler = onAuthenticationFailure ?? null
  }

  function stop(): void {
    friends.value = []
    requests.value = []
    searchResults.value = []
    sentRequestIDs.value = []
    loading.value = false
    requestsLoading.value = false
    searching.value = false
    lastError.value = ''
    searchKeyword.value = ''
    authFailureHandler = null
  }

  function isAuthenticationFailure(error: unknown): boolean {
    return error instanceof PublicApiError && (error.status === 401 || (error.code !== undefined && AUTH_ERROR_CODES.has(error.code)))
  }

  function handleFailure(error: unknown, fallback: string): string {
    if (isAuthenticationFailure(error)) {
      void authFailureHandler?.()
      return '登录状态已失效，请重新登录'
    }
    return errorMessage(error, fallback)
  }

  async function loadFriends(): Promise<void> {
    if (loading.value) return
    loading.value = true
    try {
      const page = await friendApi.friends(FRIEND_LIMIT)
      friends.value = [...page.list].sort((left, right) => {
        if (left.isOnline !== right.isOnline) return left.isOnline ? -1 : 1
        return left.userID - right.userID
      })
      lastError.value = ''
    } catch (error) {
      lastError.value = handleFailure(error, '好友列表加载失败')
    } finally {
      loading.value = false
    }
  }

  async function loadRequests(): Promise<void> {
    if (requestsLoading.value) return
    requestsLoading.value = true
    try {
      const page = await friendApi.requests(0, REQUEST_LIMIT)
      requests.value = page.list
      lastError.value = ''
    } catch (error) {
      lastError.value = handleFailure(error, '好友申请加载失败')
    } finally {
      requestsLoading.value = false
    }
  }

  async function search(keyword: string): Promise<void> {
    const trimmed = keyword.trim()
    searchKeyword.value = trimmed
    if (!trimmed) {
      searchResults.value = []
      lastError.value = '请输入搜索关键词'
      return
    }
    if (searching.value) return
    searching.value = true
    try {
      const page = await friendApi.search(trimmed)
      searchResults.value = page.list
      lastError.value = ''
    } catch (error) {
      searchResults.value = []
      lastError.value = handleFailure(error, '搜索失败，请稍后重试')
    } finally {
      searching.value = false
    }
  }

  async function sendRequest(userID: number): Promise<boolean> {
    try {
      await friendApi.sendRequest(userID)
      if (!sentRequestIDs.value.includes(userID)) sentRequestIDs.value = [...sentRequestIDs.value, userID]
      lastError.value = ''
      return true
    } catch (error) {
      lastError.value = handleFailure(error, '好友申请发送失败')
      return false
    }
  }

  // Accepting or rejecting removes the handled request from the pending list.
  async function respond(requestID: number, status: 1 | 2): Promise<boolean> {
    try {
      await friendApi.respond(requestID, status)
      requests.value = requests.value.filter((request) => request.id !== requestID)
      if (status === 1) await loadFriends()
      lastError.value = ''
      return true
    } catch (error) {
      lastError.value = handleFailure(error, status === 1 ? '同意好友申请失败' : '拒绝好友申请失败')
      return false
    }
  }

  async function updateRemark(userID: number, remark: string): Promise<boolean> {
    try {
      const updated = await friendApi.updateRemark(userID, remark)
      friends.value = friends.value.map((friend) => (friend.userID === userID ? updated : friend))
      lastError.value = ''
      return true
    } catch (error) {
      lastError.value = handleFailure(error, '好友备注保存失败')
      return false
    }
  }

  async function remove(userID: number): Promise<boolean> {
    try {
      await friendApi.remove(userID)
      friends.value = friends.value.filter((friend) => friend.userID !== userID)
      lastError.value = ''
      return true
    } catch (error) {
      lastError.value = handleFailure(error, '删除好友失败')
      return false
    }
  }

  return {
    friends, requests, searchResults, loading, requestsLoading, searching, lastError, searchKeyword, sentRequestIDs,
    onlineCount, friendIDs, pendingRequestCount,
    start, stop, loadFriends, loadRequests, search, sendRequest, respond, updateRemark, remove,
  }
})
