import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { quasar, transformAssetUrls } from '@quasar/vite-plugin'

// En desarrollo (npm run dev) las llamadas van al servicio en marcha; el origen sigue siendo el mismo para el
// navegador, así que la cookie y la validación de Origin funcionan igual que en producción.
const servicio = 'http://127.0.0.1:17373'

export default defineConfig({
  plugins: [vue({ template: { transformAssetUrls } }), quasar()],
  build: {
    outDir: '../internal/recursos/web',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
  },
  server: {
    proxy: { '/api': servicio, '/ui': servicio, '/openai': servicio, '/anthropic': servicio },
  },
})
