import { readdirSync } from 'node:fs'
import { resolve } from 'node:path'
import react from '@vitejs/plugin-react'
import { defineConfig, type Plugin } from 'vite'

// Expose /plugins/index.json with local plugin files from public/plugins.
function localPluginsManifest(): Plugin {
    const pluginsDir = resolve(process.cwd(), 'public/plugins')
    const listLocalPlugins = () => {
        try {
            return readdirSync(pluginsDir)
                .filter((file) => file.endsWith('.js'))
                .sort()
                .map((file) => `/plugins/${file}`)
        } catch {
            return []
        }
    }
    return {
        name: 'local-plugins-manifest',
        configureServer(server) {
            server.middlewares.use('/plugins/index.json', (_req, res) => {
                res.setHeader('Content-Type', 'application/json')
                res.end(JSON.stringify(listLocalPlugins()))
            })
        },
        generateBundle() {
            this.emitFile({ type: 'asset', fileName: 'plugins/index.json', source: JSON.stringify(listLocalPlugins()) })
        },
    }
}

export default defineConfig({
  plugins: [react(), localPluginsManifest()],
  resolve: { alias: { '@': resolve(process.cwd(), 'src') } },
  define: {
    __APP_VERSION__: JSON.stringify('0.1.0'),
    __APP_RELEASES__: JSON.stringify([]),
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (id.includes('antd') || id.includes('@ant-design') || id.includes('@rc-component')) return 'antd'
          if (id.includes('lucide-react')) return 'icons'
          if (id.includes('codemirror')) return 'codemirror'
          if (id.includes('zustand') || id.includes('i18next') || id.includes('react-i18next')) return 'state'
          if (id.includes('react') || id.includes('react-dom') || id.includes('react-router')) return 'react'
          return undefined
        },
      },
    },
  },
  server: { host: 'localhost', port: 16302, strictPort: true, open: true },
})
