<template>
  <div>
    <div class="text-weight-medium q-mb-sm">{{ t('guia.titulo') }}</div>
    <div class="row q-col-gutter-md">
      <div v-for="c in tarjetas" :key="c.id" class="col-12" :class="tarjetas.length > 2 ? 'col-lg-4' : 'col-md-6'">
        <div class="guia-url column full-height">
          <div class="text-weight-medium">{{ c.titulo }}</div>
          <div class="text-caption texto-suave q-mt-xs col">{{ c.texto }}</div>
          <div class="bloque-codigo q-mt-sm">{{ c.url }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar') + ' ' + c.titulo" @click="copiar(c.url)" /></div>
          <template v-if="c.id === 'nativa'">
            <q-btn flat dense no-caps color="primary" icon="o_menu_book" class="self-start q-mt-xs" :label="t('guia.comoUsarla')" @click="emit('usar')" />
            <div class="text-caption texto-suave q-mt-xs">
              {{ t('guia.avanzado') }}
              <a href="#" class="enlace" @click.prevent="descargarOpenapi">{{ descargando ? '…' : t('guia.descargarOpenapi') }}</a>
            </div>
          </template>
        </div>
      </div>
    </div>
    <q-expansion-item dense icon="o_table_chart" :label="t('guia.tabla')" header-class="q-px-none text-body2" class="q-mt-sm">
      <div class="desplazable-x">
        <table class="guia-tabla text-body2">
          <thead><tr><th /><th>OpenAI</th><th>Anthropic</th><th>{{ t('ejemplos.nativa') }}</th></tr></thead>
          <tbody>
            <tr v-for="f in filas" :key="f[0]"><th scope="row">{{ t(f[0]) }}</th><td v-for="(c, i) in f.slice(1)" :key="i">{{ t(c) }}</td></tr>
          </tbody>
        </table>
      </div>
    </q-expansion-item>
  </div>
</template>

<script setup lang="ts">
// GuiaUrls explica qué URL usar según lo que hace la app, con la URL de cada contrato que acepta la API y una tabla
// de lo que permite cada uno. nombre es la API por aplicación (vacío: la API principal).
import { computed, ref } from 'vue'
import { api, avisarError, copiar } from '../api/cliente'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const props = defineProps<{ url: string; contratos: string[]; nombre?: string }>()
// usar pide mostrar los pasos de la API propia (los ejemplos de código con la URL y la clave).
const emit = defineEmits<{ usar: [] }>()

const tarjetas = computed(() => ([
  { id: 'openai', titulo: t('conectar.urlOpenai'), texto: t('guia.openai'), url: props.url + '/openai/v1' },
  { id: 'anthropic', titulo: t('conectar.urlAnthropic'), texto: t('guia.anthropic'), url: props.url + '/anthropic' },
  { id: 'nativa', titulo: t('conectar.apiPropia'), texto: t('guia.nativa'), url: props.url + '/api/v1' },
]).filter((c) => props.contratos.includes(c.id)))

const filas: Clave[][] = [
  ['guia.fSdk', 'guia.si', 'guia.si', 'guia.noHttp'],
  ['guia.fVivo', 'guia.si', 'guia.si', 'guia.siRetoma'],
  ['guia.fJson', 'guia.siResponseFormat', 'guia.no', 'guia.siEsquema'],
  ['guia.fArchivos', 'guia.siBase64', 'guia.siBase64', 'guia.si'],
  ['guia.fTareas', 'guia.no', 'guia.no', 'guia.si'],
  ['guia.fRespaldo', 'guia.soloEsfuerzo', 'guia.no', 'guia.si'],
  ['guia.fRegistro', 'guia.no', 'guia.no', 'guia.si'],
]

// descargarOpenapi guarda la descripción de la API propia con la URL de esta API; una API por aplicación no tiene lo
// que solo existe en la instancia (vaciar el historial).
const descargando = ref(false)
async function descargarOpenapi() {
  descargando.value = true
  try {
    const doc = await api.openapi()
    doc.servers = [{ url: props.url + '/api/v1' }]
    if (props.nombre) {
      for (const ops of Object.values(doc.paths ?? {}) as Record<string, { 'x-tramtai-solo-instancia'?: boolean }>[]) {
        for (const [metodo, op] of Object.entries(ops)) if (op?.['x-tramtai-solo-instancia']) delete ops[metodo]
      }
    }
    const enlace = document.createElement('a')
    enlace.href = URL.createObjectURL(new Blob([JSON.stringify(doc, null, 2)], { type: 'application/json' }))
    enlace.download = `tramtai-${props.nombre || 'principal'}-openapi.json`
    enlace.click()
    setTimeout(() => URL.revokeObjectURL(enlace.href), 1000)
  } catch (e) {
    avisarError(e)
  } finally {
    descargando.value = false
  }
}
</script>

<style scoped>
.guia-url {
  border: 1px solid var(--borde);
  border-radius: var(--radio-control);
  padding: 12px;
}
.guia-tabla {
  border-collapse: collapse;
  min-width: 560px;
  width: 100%;
}
.guia-tabla th,
.guia-tabla td {
  border-bottom: 1px solid var(--borde);
  padding: 6px 8px;
  text-align: left;
  vertical-align: top;
}
.guia-tabla thead th {
  font-weight: 600;
}
.guia-tabla tbody th {
  font-weight: 400;
  color: var(--texto-suave);
}
</style>
