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
            <a class="enlace text-caption q-mt-xs" href="https://mscottb.github.io/Tramtai/desarrolladores.html" target="_blank" rel="noopener">{{ t('guia.desarrolladores') }}</a>
            <div class="text-caption texto-suave q-mt-xs">
              {{ t('guia.avanzado') }}
              <a href="#" class="enlace" :aria-busy="descargando" @click.prevent="descargarOpenapi">{{ t('guia.descargarOpenapi') }}</a>
            </div>
          </template>
        </div>
      </div>
    </div>
    <q-expansion-item dense default-opened icon="o_table_chart" :label="t('guia.tabla')" header-class="q-px-none text-body2" class="q-mt-sm">
      <TablaApis class="q-pb-sm" />
    </q-expansion-item>
  </div>
</template>

<script setup lang="ts">
// GuiaUrls explica qué URL usar según lo que hace la app, con la URL de cada contrato que acepta la API y una tabla
// de lo que permite cada uno (TablaApis). nombre es la API por aplicación (vacío: la API principal).
import { computed, ref } from 'vue'
import { api, avisarError, copiar } from '../api/cliente'
import { t } from '../i18n'
import TablaApis from './TablaApis.vue'

const props = defineProps<{ url: string; contratos: string[]; nombre?: string }>()
// usar pide mostrar los pasos de la API propia (los ejemplos de código con la URL y la clave).
const emit = defineEmits<{ usar: [] }>()

const tarjetas = computed(() => ([
  { id: 'openai', titulo: t('conectar.urlOpenai'), texto: t('guia.openai'), url: props.url + '/openai/v1' },
  { id: 'anthropic', titulo: t('conectar.urlAnthropic'), texto: t('guia.anthropic'), url: props.url + '/anthropic' },
  { id: 'nativa', titulo: t('conectar.apiPropia'), texto: t('guia.nativa'), url: props.url + '/api/v1' },
]).filter((c) => props.contratos.includes(c.id)))

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
</style>
