<template>
  <div class="apilado vista-tema">
    <div v-if="!sinCabecera">
      <h2 class="titulo-seccion q-ma-none">{{ t(tema.titulo) }}</h2>
      <div class="texto-suave q-mt-xs">{{ t(tema.resumen) }}</div>
    </div>
    <template v-for="(b, i) in bloques" :key="i">
      <BloqueDiagrama v-if="b.tipo === 'diagrama'" :bloque="b" />
      <BloquePasos v-else-if="b.tipo === 'pasos'" :bloque="b" />
      <BloqueCaptura v-else-if="b.tipo === 'captura'" :bloque="b" />
      <BloqueEjemplo v-else-if="b.tipo === 'ejemplo'" :bloque="b" />
      <BloqueTabla v-else-if="b.tipo === 'tabla'" :bloque="b" />
      <BloquePreguntas v-else-if="b.tipo === 'preguntas'" :bloque="b" :ancla="ancla" @tema="(id) => $emit('ir', 'ayuda/' + id)" />
      <BloqueErrores v-else-if="b.tipo === 'errores'" :bloque="b" :ancla="ancla" />
      <BloqueNota v-else-if="b.tipo === 'nota'" :bloque="b" />
      <BloqueRecorrido v-else-if="b.tipo === 'recorrido' && tema.paginaApp" :bloque="b" @mostrar="$emit('recorrido', tema.paginaApp)" />
      <div v-else-if="b.tipo === 'ir'">
        <q-btn outline color="primary" no-caps icon-right="arrow_forward" :label="t('ayuda.ir', { pagina: t(menu[b.pagina]) })" @click="$emit('ir', b.pagina)" />
      </div>
    </template>
    <BloqueNota :bloque="{ tipo: 'nota', texto: tema.terminal }" />
    <template v-if="contexto === 'app'">
      <div v-if="relacionados.length">
        <div class="text-weight-medium q-mb-sm">{{ t('ayuda.ui.relacionados') }}</div>
        <TarjetasTemas :temas="relacionados" @elegir="(id) => $emit('ir', 'ayuda/' + id)" />
      </div>
      <div>
        <q-btn flat dense no-caps color="primary" icon-right="open_in_new" :label="t('ayuda.ui.abrirWeb')" type="a"
          :href="'https://mscottb.github.io/Tramtai/ayuda/' + tema.id + '.html'" target="_blank" rel="noopener" />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// VistaTema dibuja un tema con sus bloques, en la página Ayuda (contexto app) o en el panel del botón ?. Al cambiar
// ancla, lleva la vista a esa pregunta o error.
import { computed, nextTick, onMounted, watch } from 'vue'
import { t } from '../../i18n'
import type { Clave } from '../../idiomas/es'
import { temaPorId } from '../../ayuda/indice'
import { bloquesVisibles } from '../../ayuda/vista'
import type { Pagina, Tema } from '../../ayuda/tipos'
import BloqueDiagrama from './BloqueDiagrama.vue'
import BloquePasos from './BloquePasos.vue'
import BloqueCaptura from './BloqueCaptura.vue'
import BloqueEjemplo from './BloqueEjemplo.vue'
import BloqueTabla from './BloqueTabla.vue'
import BloquePreguntas from './BloquePreguntas.vue'
import BloqueErrores from './BloqueErrores.vue'
import BloqueNota from './BloqueNota.vue'
import BloqueRecorrido from './BloqueRecorrido.vue'
import TarjetasTemas from './TarjetasTemas.vue'

const props = defineProps<{ tema: Tema; contexto: 'app' | 'panel'; paginaActual?: Pagina; ancla?: string; sinCabecera?: boolean }>()
defineEmits<{ ir: [destino: string]; recorrido: [pagina: Pagina] }>()

const menu: Record<Pagina, Clave> = { inicio: 'menu.inicio', agentes: 'menu.agentes', ejecutar: 'menu.ejecutar', conectar: 'menu.conectar',
  historial: 'menu.historial', espacios: 'menu.espacios', configuracion: 'menu.configuracion' }
const bloques = computed(() => bloquesVisibles(props.tema, props.contexto, props.paginaActual))
const relacionados = computed(() => props.tema.relacionados.map(temaPorId).filter((x): x is Tema => !!x))

function irAlAncla() {
  if (props.ancla) nextTick(() => document.getElementById('ayuda-' + props.ancla)?.scrollIntoView({ behavior: 'smooth', block: 'center' }))
}
onMounted(irAlAncla)
watch(() => [props.tema.id, props.ancla], irAlAncla)
</script>
