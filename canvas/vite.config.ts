import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': resolve(process.cwd(), 'src') } },
  server: { host: 'localhost', port: 16302, strictPort: true, open: true },
})
