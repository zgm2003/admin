import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': resolve(process.cwd(), 'src') } },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('antd') || id.includes('@ant-design/icons')) return 'antd'
          if (id.includes('lucide-react')) return 'icons'
          if (id.includes('zustand') || id.includes('i18next') || id.includes('react-i18next')) return 'state'
          if (id.includes('react') || id.includes('react-dom') || id.includes('react-router')) return 'react'
          return undefined
        },
      },
    },
  },
  server: { host: 'localhost', port: 16302, strictPort: true, open: true },
})
