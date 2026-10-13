<script setup lang="ts">
import { watch } from 'vue'
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/file'
import { useGroupStore } from '@/stores/group'

const auth = useAuthStore()
const chat = useChatStore()
const file = useFileStore()
const group = useGroupStore()
const router = useRouter()
let invalidatingAuthentication = false

async function invalidateAuthentication(): Promise<void> {
  if (invalidatingAuthentication) return
  invalidatingAuthentication = true
  try {
    chat.stop()
    file.stop()
    group.stop()
    auth.reset()
    await router.replace({ name: 'login', query: { reason: 'session-expired' } })
  } finally {
    invalidatingAuthentication = false
  }
}

watch(
  () => [auth.authenticated, auth.user?.userID, auth.token] as const,
  ([authenticated, userID, token]) => {
    if (!authenticated || !userID || !token) {
      chat.stop()
      file.stop()
      group.stop()
      return
    }
    chat.start(userID, token, invalidateAuthentication)
    file.start(invalidateAuthentication)
    group.start(userID, token, invalidateAuthentication)
  },
  { immediate: true },
)
</script>

<template><router-view /></template>
