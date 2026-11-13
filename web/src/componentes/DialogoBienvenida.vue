<template>
  <!-- La bienvenida: ofrece el recorrido y, si ningún agente está listo, «Prepara tus agentes» (los instala uno tras otro
       en un solo panel y termina diciendo cuáles quedaron listos). -->
  <q-dialog :model-value="modelValue" :persistent="fase === 'preparando'" aria-labelledby="titulo-bienvenida" @update:model-value="emit('update:modelValue', $event)" @hide="emit('cerrada', hecha)">
    <q-card style="width: 560px; max-width: 95vw">
      <q-card-section>
        <h2 id="titulo-bienvenida" class="titulo-seccion">{{ fase === 'oferta' ? t('bienvenida.titulo') : t('bienvenida.agentesTitulo') }}</h2>
        <div v-if="fase === 'oferta'" class="texto-suave text-body2 q-mt-xs">{{ t('bienvenida.texto') }}</div>
      </q-card-section>

      <q-card-section v-if="fase === 'oferta' && opciones.length" class="q-pt-none">
        <div class="aviso">
          <div class="text-weight-medium">{{ t('bienvenida.agentesTitulo') }}</div>
          <div class="text-body2 q-mt-xs">{{ t('bienvenida.agentesTexto') }}</div>
          <div v-if="cuenta" class="text-caption texto-suave q-mt-xs"><q-icon name="o_dns" class="q-mr-xs" aria-hidden="true" />{{ t('preparar.cuentaServicio', { cuenta }) }}</div>
          <div class="column q-mt-sm" role="group" :aria-label="t('bienvenida.agentesElegidos')">
            <q-checkbox v-for="o in opciones" :key="o.agente" v-model="elegidos[o.agente]" dense class="q-py-xs" :label="nombreAgente(o.agente)" />
          </div>
          <div v-if="elegidos.antigravity" class="text-caption texto-suave q-mt-xs">{{ t('bienvenida.notaAntigravity') }}</div>
          <q-btn unelevated no-caps color="primary" class="q-mt-sm" icon="o_download" :label="t('bienvenida.preparar')" :disable="!marcados.length" @click="preparar" />
        </div>
      </q-card-section>

      <q-card-section v-if="fase === 'preparando'" class="q-pt-none">
        <PanelPreparacion :agentes="lanzados" accion="instalar" @fin="(a, e) => (estados[a] = e)" @listo="alAcabar" @cerrar="alAcabar" />
      </q-card-section>

      <q-card-section v-if="fase === 'resumen'" class="q-pt-none column q-gutter-y-sm" role="status">
        <div v-if="resumen.listos.length" class="text-body1 text-weight-medium">{{ t('bienvenida.listo', { agentes: resumen.listos.map(nombreAgente).join(', ') }) }}</div>
        <div v-else class="text-body1 text-weight-medium">{{ t('bienvenida.ninguno') }}</div>
        <template v-if="resumen.pendientes.length">
          <ul class="q-my-none q-pl-md text-body2">
            <li v-for="p in resumen.pendientes" :key="p.agente">{{ t('bienvenida.pendiente', { agente: nombreAgente(p.agente), razon: p.razon }) }}</li>
          </ul>
          <div class="text-body2">{{ t('bienvenida.terminarEnAgentes') }}</div>
        </template>
      </q-card-section>

      <q-card-actions v-if="fase !== 'preparando'" align="right" class="q-px-md q-pb-md">
        <q-btn v-close-popup flat no-caps color="primary" :label="fase === 'oferta' ? t('bienvenida.ahoraNo') : t('comun.cerrar')" />
        <q-btn v-if="fase === 'resumen' && resumen.pendientes.length" v-close-popup outline no-caps color="primary" :label="t('bienvenida.irAgentes')" @click="emit('ir', 'agentes')" />
        <q-btn v-close-popup :unelevated="!opciones.length || fase === 'resumen'" :outline="!!opciones.length && fase === 'oferta'" no-caps color="primary" :label="t('bienvenida.empezar')" @click="empezar" />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { api } from '../api/cliente'
import type { EstadoPreparacion } from '../api/tipos'
import { adaptadoresConocidos, nombreAgente } from '../agentes'
import { marcarBienvenida, sesion } from '../estado'
import { opcionesAgentes } from '../bienvenida'
import { resumenFinal } from '../preparacion'
import { esServidor } from '../servidor'
import { t } from '../i18n'
import PanelPreparacion from './PanelPreparacion.vue'

const props = defineProps<{ modelValue: boolean }>()
// cerrada: el diálogo se cerró; hecha dice si el usuario ya hizo algo (el recorrido o preparar agentes), y entonces no se vuelve a ofrecer.
const emit = defineEmits<{ 'update:modelValue': [abierto: boolean]; cerrada: [hecha: boolean]; recorrido: []; ir: [pagina: string] }>()

const fase = ref<'oferta' | 'preparando' | 'resumen'>('oferta')
const hecha = ref(false)
const opciones = computed(() => opcionesAgentes(adaptadoresConocidos.value))
const elegidos = reactive<Record<string, boolean>>({})
watch(opciones, (o) => o.forEach((x) => (elegidos[x.agente] ??= x.marcado)), { immediate: true })
const marcados = computed(() => opciones.value.filter((o) => elegidos[o.agente]).map((o) => o.agente))
// lanzados: los agentes de esta preparación, fijos aunque las opciones cambien mientras se instalan.
const lanzados = ref<string[]>([])
const estados = reactive<Record<string, EstadoPreparacion | null>>({})
const resumen = computed(() => resumenFinal(lanzados.value, estados))
// cuenta: en el servidor se prepara para la cuenta del servicio, y la bienvenida lo dice antes de empezar.
const cuenta = ref('')
watch(() => props.modelValue, (abierto) => {
  if (abierto && esServidor(sesion.value) && !cuenta.value) api.acerca().then((a) => (cuenta.value = a.cuenta ?? '')).catch(() => {})
}, { immediate: true })

function preparar() {
  lanzados.value = [...marcados.value]
  hecha.value = true
  marcarBienvenida('hecha')
  fase.value = 'preparando'
}
function alAcabar(finales?: Record<string, EstadoPreparacion | null>) {
  if (finales) Object.assign(estados, finales)
  fase.value = 'resumen'
}
function empezar() {
  hecha.value = true
  emit('recorrido')
}
</script>