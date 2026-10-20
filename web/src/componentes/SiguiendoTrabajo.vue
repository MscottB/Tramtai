<template>
  <q-card v-if="tranvia" flat bordered class="siguiendo">
    <q-card-section class="row items-start no-wrap q-gutter-x-sm">
      <div class="col">
        <div class="text-weight-medium">{{ t('vivo.siguiendo') }}</div>
        <div class="text-caption texto-suave">
          {{ tranvia.api ? 'API ' + tranvia.api : t('mapa.principal') }} · {{ nombreAgente(tranvia.agente) }}<span v-if="tranvia.modelo"> · {{ tranvia.modelo }}</span>
          · {{ t('vivo.hace', { s: segundos }) }}
        </div>
      </div>
      <q-btn flat round dense icon="close" :aria-label="t('vivo.dejar')" @click="$emit('dejar')" />
    </q-card-section>
    <q-card-section class="q-pt-none">
      <Ruta :estado="tranvia.estado" />
      <div class="solo-lectores" aria-live="polite">{{ t('vivo.tranvia', { agente: nombreAgente(tranvia.agente), etapa: t(claveEtapa(tranvia)) }) }}</div>
    </q-card-section>
    <q-card-actions class="q-px-md q-pb-md q-pt-none">
      <q-btn outline color="primary" no-caps icon="o_history" :label="t('vivo.verHistorial')" @click="verHistorial" />
      <q-btn flat no-caps color="primary" :label="t('vivo.dejar')" @click="$emit('dejar')" />
    </q-card-actions>
  </q-card>
</template>

<script setup lang="ts">
// SiguiendoTrabajo muestra el recorrido en vivo de la petición elegida en el mapa de Inicio: de dónde salió, qué
// agente la lleva y en qué parada va. Al terminar se queda con su resultado hasta que se deja de seguir.
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Ruta from './Ruta.vue'
import { nombreAgente } from '../agentes'
import { trabajoAbrir } from '../estado'
import { t } from '../i18n'
import { claveEtapa, type Tranvia } from '../vivo'

const props = defineProps<{ tranvia?: Tranvia }>()
const emit = defineEmits<{ dejar: []; ir: [pagina: string] }>()

const ahora = ref(Date.now())
let reloj: number
onMounted(() => (reloj = window.setInterval(() => (ahora.value = Date.now()), 1000)))
onUnmounted(() => window.clearInterval(reloj))
const segundos = computed(() => (props.tranvia ? Math.max(0, Math.round(((props.tranvia.terminadoEn ?? ahora.value) - props.tranvia.desde) / 1000)) : 0))

function verHistorial() {
  if (!props.tranvia) return
  trabajoAbrir.value = props.tranvia.id
  emit('ir', 'historial')
}
</script>
