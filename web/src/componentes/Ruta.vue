<template>
  <ol class="ruta q-ma-none q-pa-none" :class="claseRuta" :aria-label="t('ruta.titulo')">
    <span v-if="!terminado" class="ruta__tranvia" :style="{ left: ((actual + 0.5) / 5) * 100 + '%' }" aria-hidden="true" />
    <li v-for="(p, i) in paradas" :key="p" class="ruta__parada" :class="claseParada(i)"
      :aria-current="i === actual ? 'step' : undefined">
      <span class="ruta__punto" aria-hidden="true" />
      <span>{{ p }}</span>
    </li>
  </ol>
</template>

<script setup lang="ts">
// Ruta dibuja una petición como un trayecto: cola, preparación, ejecución, validación y llegada. Mientras avanza,
// un tranvía se desliza hasta la estación actual; al terminar, la llegada toma el color del resultado.
import { computed } from 'vue'
import { estadoInfo } from '../api'
import { t } from '../i18n'

const props = defineProps<{ estado: string }>()

const posicion: Record<string, number> = {
  en_cola: 0, preparando_entrada: 1, esperando_agente: 1, ejecutando: 2, cancelacion_solicitada: 2, validando_salida: 3, completado: 4,
}
const terminado = computed(() => ['completado', 'fallido', 'cancelado', 'interrumpido'].includes(props.estado))
const actual = computed(() => (terminado.value ? 4 : (posicion[props.estado] ?? 0)))
const paradas = computed(() => [t('ruta.cola'), t('ruta.preparacion'), t('ruta.ejecucion'), t('ruta.validacion'),
  props.estado === 'completado' || !terminado.value ? t('ruta.llegada') : estadoInfo(props.estado).etiqueta])
const claseRuta = computed(() => ({
  'ruta--ok': props.estado === 'completado',
  'ruta--error': props.estado === 'fallido' || props.estado === 'interrumpido',
  'ruta--cancelada': props.estado === 'cancelado',
}))

function claseParada(i: number) {
  if (terminado.value) return i === 4 ? 'ruta__parada--fin' : 'ruta__parada--pasada'
  return i < actual.value ? 'ruta__parada--pasada' : i === actual.value ? 'ruta__parada--actual' : ''
}
</script>
