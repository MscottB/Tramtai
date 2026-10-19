<template>
  <div v-if="texto">
    <div v-if="!json" class="row justify-end q-mb-xs">
      <q-btn-toggle v-model="modo" dense flat no-caps size="sm" toggle-color="primary" :aria-label="t('salida.formato')"
        :options="[{ label: t('salida.markdown'), value: 'markdown' }, { label: t('salida.texto'), value: 'texto' }]" />
    </div>
    <!-- El HTML sale de marked y pasa por DOMPurify: sin scripts, sin imágenes externas y con los enlaces en otra pestaña. -->
    <div v-if="!json && modo === 'markdown'" class="markdown" :class="clase" v-html="html" />
    <pre v-else class="texto-plano" :class="[clase, { json }]">{{ texto }}</pre>
  </div>
</template>

<script setup lang="ts">
// SalidaTrabajo muestra la respuesta de un agente: con formato Markdown, o como texto tal cual llegó; JSON siempre
// como texto. La comparten Probar y el detalle del Historial.
import DOMPurify from 'dompurify'
import { Marked } from 'marked'
import { computed, ref, watch } from 'vue'
import { esJSON } from '../formato'
import { t } from '../i18n'

// caja: el estilo con fondo y borde del Historial; sin ella, el de Probar.
const props = defineProps<{ texto?: string; caja?: boolean }>()
const clase = computed(() => (props.caja ? 'bloque-texto' : 'salida'))
const json = computed(() => esJSON(props.texto))
// Solo HTML: sin SVG ni MathML, que traen enlaces e imágenes por otra vía.
const html = computed(() => DOMPurify.sanitize(conversor.parse(props.texto ?? '', { async: false }),
  { USE_PROFILES: { html: true }, FORBID_TAGS: ['img', 'style', 'form'] }))
const modo = preferencia
watch(modo, (m) => {
  try {
    localStorage.setItem(clave, m)
  } catch {
    // sin almacenamiento (ventana privada): la elección vale hasta recargar
  }
})
</script>

<script lang="ts">
const clave = 'tramtai.salida'
function leer(): 'markdown' | 'texto' {
  try {
    return localStorage.getItem(clave) === 'texto' ? 'texto' : 'markdown'
  } catch {
    return 'markdown'
  }
}
// preferencia es compartida: cambiarla en Probar también cambia el Historial.
const preferencia = ref<'markdown' | 'texto'>(leer())

const escapar = (s: string) => s.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)
// Una imagen externa podría rastrear a quien abre la respuesta: se muestra su texto alternativo.
const conversor = new Marked({ gfm: true, breaks: true, renderer: { image: ({ text }) => escapar(text) } })
DOMPurify.addHook('afterSanitizeAttributes', (nodo) => {
  if (nodo.tagName.toLowerCase() === 'a') {
    nodo.setAttribute('target', '_blank')
    nodo.setAttribute('rel', 'noopener noreferrer')
  }
})
</script>
