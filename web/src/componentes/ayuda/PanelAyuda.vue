<template>
  <q-drawer :model-value="panel.abierto" side="right" overlay bordered :width="ancho" :breakpoint="0" class="panel-ayuda"
    @update:model-value="(v: boolean) => !v && cerrarAyuda()">
    <div v-if="tema" class="q-pa-md apilado" role="region" aria-labelledby="panel-ayuda-titulo">
      <div class="row items-start no-wrap q-gutter-x-sm">
        <h2 id="panel-ayuda-titulo" ref="titulo" tabindex="-1" class="titulo-seccion q-ma-none col">{{ t(tema.titulo) }}</h2>
        <q-btn flat round dense icon="close" :aria-label="t('comun.cerrar')" @click="cerrarAyuda" />
      </div>
      <div class="texto-suave">{{ t(tema.resumen) }}</div>
      <VistaTema :tema="tema" contexto="panel" sin-cabecera :pagina-actual="pagina" :ancla="panel.ancla" @ir="ir" @recorrido="mostrar" />
      <div><q-btn outline color="primary" no-caps icon-right="arrow_forward" :label="t('ayuda.ui.abrirEnAyuda')" @click="ir('ayuda/' + tema.id)" /></div>
    </div>
  </q-drawer>
</template>

<script setup lang="ts">
// PanelAyuda muestra un tema junto a la página, sin salir de ella: se abre con el ? de la cabecera, «¿Qué hago?» o
// «Saber más», y se cierra con la X o Esc. En el móvil ocupa toda la pantalla.
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useQuasar } from 'quasar'
import { t } from '../../i18n'
import { temaPorId } from '../../ayuda/indice'
import { cerrarAyuda, escDelPanel, panel } from '../../ayuda/panel'
import type { Pagina } from '../../ayuda/tipos'
import VistaTema from './VistaTema.vue'

// recorridoActivo: con un recorrido en marcha Esc es suyo (useRecorrido la captura y la detiene), no del panel ni del diálogo de debajo.
const props = defineProps<{ pagina?: Pagina; recorridoActivo?: boolean }>()
const emit = defineEmits<{ ir: [destino: string]; recorrido: [pagina: Pagina] }>()

const $q = useQuasar()
const ancho = computed(() => ($q.screen.width < 600 ? $q.screen.width : 420))
const tema = computed(() => temaPorId(panel.tema))
const titulo = ref<HTMLElement | null>(null)

watch(() => [panel.abierto, panel.tema], () => {
  if (panel.abierto) nextTick(() => titulo.value?.focus())
})

// ir lleva a otra página (o a la Ayuda completa) y cierra el panel.
function ir(destino: string) {
  cerrarAyuda()
  emit('ir', destino)
}

// mostrar cierra el panel antes de empezar el recorrido de la página, para que no quede encima de lo que se señala.
function mostrar(pagina: Pagina) {
  cerrarAyuda()
  emit('recorrido', pagina)
}

// Esc cierra solo el panel: se atrapa antes que el diálogo que haya debajo (el detalle del Historial), que si no se
// cerraría con la misma tecla. Quasar escucha keyup para Esc, así que se detienen las dos.
function alTeclear(e: KeyboardEvent) {
  if (!escDelPanel(e, panel.abierto, !!props.recorridoActivo, !!document.querySelector('.q-menu'))) return
  e.stopImmediatePropagation()
  if (e.type === 'keyup') cerrarAyuda()
}
onMounted(() => {
  window.addEventListener('keydown', alTeclear, true)
  window.addEventListener('keyup', alTeclear, true)
})
onUnmounted(() => {
  window.removeEventListener('keydown', alTeclear, true)
  window.removeEventListener('keyup', alTeclear, true)
})
</script>
