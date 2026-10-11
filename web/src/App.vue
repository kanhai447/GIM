<script setup lang="ts">
import { watch } from 'vue'
import { useRouter } from 'vue-router'

import { useAuthStore } from '@/stores/auth'
import { useChatStore } from '@/stores/chat'

const auth = useAuthStore()
const chat = useChatStore()
const router = useRouter()

watch(
  () => [auth.authenticated, auth.user?.userID, auth.token] as const,
  ([authenticated, userID, token]) => {
    if (!authenticated || !userID || !token) {
      chat.stop()
      return
    }
    chat.start(userID, token, async () => {
      auth.reset()
      await router.replace({ name: 'login', query: { reason: 'session-expired' } })
    })
  },
  { immediate: true },
)
</script>

<template><router-view /></template>
