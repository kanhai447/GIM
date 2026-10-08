import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const gatewayTarget = env.VITE_DEV_GATEWAY_TARGET?.trim()

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: gatewayTarget
      ? {
          proxy: {
            '/api': {
              target: gatewayTarget,
              changeOrigin: true,
            },
          },
        }
      : undefined,
    test: {
      environment: 'node',
      clearMocks: true,
    },
  }
})
