import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '')
  return {
    plugins: [react()],
    server: {
      proxy: {
        '/api': {
          target: env.VYNODE_DEV_API || 'http://127.0.0.1:8096',
          changeOrigin: true,
          headers: { Origin: env.VYNODE_DEV_API || 'http://127.0.0.1:8096' },
        },
      },
    },
  }
})
