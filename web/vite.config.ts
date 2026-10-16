import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { quasar, transformAssetUrls } from '@quasar/vite-plugin'

// En desarrollo (npm run dev) las llamadas van al servicio en marcha; el origen sigue siendo el mismo para el
// navegador, así que la cookie y la validación de Origin funcionan igual que en producción.
// TRAMTAI_SERVICIO apunta a otro servicio (por ejemplo, uno de prueba aislado en otro puerto).
const servicio = process.env.TRAMTAI_SERVICIO ?? 'http://127.0.0.1:17373'

// sinWoff quita la alternativa .woff de las fuentes de iconos: los navegadores y webviews actuales cargan la .woff2,
// y así la .woff (unos 350 KB) no se incorpora al ejecutable.
const sinWoff: Plugin = {
  name: 'tramtai-sin-woff',
  enforce: 'pre',
  transform(codigo, id) {
    if (/@quasar[\\/]extras[\\/].*\.css$/.test(id)) return codigo.replace(/,\s*url\([^)]*\.woff['"]?\)\s*format\(['"]woff['"]\)/g, '')
  },
}

export default defineConfig({
  plugins: [sinWoff, vue({ template: { transformAssetUrls } }), quasar()],
  build: {
    outDir: '../internal/recursos/web',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
  },
  server: {
    proxy: { '/api': servicio, '/ui': servicio, '/openai': servicio, '/anthropic': servicio },
  },
})
