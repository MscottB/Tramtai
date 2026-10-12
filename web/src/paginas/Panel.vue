<template>
  <div v-if="instancia" class="q-gutter-md">
    <div class="row q-col-gutter-md">
      <div class="col-12 col-md-6">
        <q-card class="full-height">
          <q-card-section>
            <div class="text-h6">Instancia {{ instancia.id }}</div>
            <div class="text-caption text-grey-7">{{ instancia.ambito === 'proyecto' ? instancia.raiz : 'Instancia global' }}</div>
          </q-card-section>
          <q-list dense>
            <q-item><q-item-section>Agente predeterminado</q-item-section><q-item-section side>{{ instancia.agente_predeterminado || '—' }}</q-item-section></q-item>
            <q-item><q-item-section>Perfil</q-item-section><q-item-section side>{{ instancia.perfil_predeterminado || '—' }}</q-item-section></q-item>
            <q-item>
              <q-item-section>
                <q-item-label caption>Archivo del token</q-item-label>
                <q-item-label class="codigo" style="word-break: break-all">{{ instancia.archivo_token }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
          <q-card-section class="q-gutter-xs">
            <q-chip v-for="(n, estado) in instancia.trabajos" :key="estado" :color="colorEstado(String(estado))" text-color="white" dense>
              {{ estado }} {{ n }}
            </q-chip>
            <span v-if="!Object.keys(instancia.trabajos).length" class="text-grey-7">Sin trabajos todavía</span>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12 col-md-6">
        <q-card class="full-height">
          <q-card-section><div class="text-h6">Conectar aplicaciones</div></q-card-section>
          <q-list dense>
            <q-item v-for="e in endpoints" :key="e.titulo">
              <q-item-section>
                <q-item-label caption>{{ e.titulo }}</q-item-label>
                <q-item-label class="codigo">{{ e.url }}</q-item-label>
              </q-item-section>
              <q-item-section side><q-btn flat dense round icon="content_copy" @click="copiar(e.url)" /></q-item-section>
            </q-item>
          </q-list>
          <q-card-section class="text-caption text-grey-7">
            Usa el token de la instancia como API key y un modelo como <span class="codigo">claude/sonnet</span> o
            <span class="codigo">perfil/rapido</span>. Para una aplicación concreta, crea un punto de acceso.
          </q-card-section>
        </q-card>
      </div>
    </div>
    <q-card>
      <q-card-section class="row items-center">
        <div class="text-h6 col">Trabajos recientes</div>
        <q-btn flat color="primary" label="Ver todos" @click="$emit('ir', 'trabajos')" />
        <q-btn color="primary" icon="play_arrow" label="Ejecutar" @click="$emit('ir', 'ejecutar')" />
      </q-card-section>
      <q-list separator>
        <q-item v-for="t in recientes" :key="t.id">
          <q-item-section avatar><q-badge :color="colorEstado(t.estado)">{{ t.estado }}</q-badge></q-item-section>
          <q-item-section>
            <q-item-label class="ellipsis">{{ t.solicitud.mensajes?.at(-1)?.contenido }}</q-item-label>
            <q-item-label caption>{{ modeloDe(t) }} · {{ fecha(t.creado_en) }}</q-item-label>
          </q-item-section>
        </q-item>
        <q-item v-if="!recientes.length"><q-item-section class="text-grey-7">Sin trabajos</q-item-section></q-item>
      </q-list>
    </q-card>
  </div>
  <q-banner v-else-if="error" class="bg-negative text-white">{{ error }}</q-banner>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, colorEstado, fecha, mensajeError, modeloDe, type EstadoInstancia, type Trabajo } from '../api'

defineEmits<{ ir: [pagina: string] }>()
const $q = useQuasar()
const instancia = ref<EstadoInstancia>()
const recientes = ref<Trabajo[]>([])
const error = ref('')
const endpoints = computed(() => {
  const base = instancia.value?.base ?? location.origin
  return [
    { titulo: 'OpenAI (base_url)', url: base + '/openai/v1' },
    { titulo: 'Anthropic (base_url)', url: base + '/anthropic' },
    { titulo: 'API nativa', url: base + '/api/v1' },
  ]
})

async function cargar() {
  try {
    ;[instancia.value, recientes.value] = await Promise.all([api.instancia(), api.trabajos(5)])
    error.value = ''
  } catch (e) {
    error.value = mensajeError(e)
  }
}

async function copiar(texto: string) {
  await navigator.clipboard.writeText(texto)
  $q.notify({ message: 'Copiado', timeout: 1000 })
}

let intervalo: number
onMounted(() => {
  cargar()
  intervalo = window.setInterval(cargar, 5000)
})
onUnmounted(() => clearInterval(intervalo))
</script>
