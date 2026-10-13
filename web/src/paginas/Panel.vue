<template>
  <div class="pagina apilado">
    <div class="row items-end q-col-gutter-md">
      <div class="col-12 col-sm">
        <div class="titulo-pagina">{{ t('menu.inicio') }}</div>
        <div class="texto-suave">{{ t('inicio.subtitulo', { url: instancia?.base ?? '…' }) }}</div>
      </div>
      <div class="col-12 col-sm-auto">
        <q-btn unelevated color="primary" icon="o_play_arrow" :label="t('menu.ejecutar')" no-caps @click="$emit('ir', 'ejecutar')" />
      </div>
    </div>

    <q-banner v-if="error" rounded class="error-caja">{{ error }}</q-banner>

    <div>
      <div class="titulo-seccion">{{ t('inicio.peticiones') }}</div>
      <div class="texto-suave text-body2 q-mb-sm">{{ t('inicio.peticionesTexto') }}</div>
      <div class="row q-col-gutter-md">
        <div v-for="c in contadores" :key="c.titulo" class="col-12 col-sm-4">
          <q-card v-ripple class="q-pa-md full-height cursor-pointer tarjeta-clicable" @click="verHistorial(c.filtro)">
            <div class="row items-center no-wrap">
              <q-avatar :color="c.color" text-color="white" :icon="c.icono" size="40px" class="q-mr-md" />
              <div class="col">
                <div class="row items-baseline q-gutter-x-sm">
                  <span class="text-h5 text-weight-bold">{{ c.valor }}</span>
                  <span class="text-weight-medium">{{ c.titulo }}</span>
                </div>
                <div class="texto-suave text-caption">{{ c.detalle }}</div>
              </div>
              <q-icon name="chevron_right" class="texto-suave" />
            </div>
          </q-card>
        </div>
      </div>
    </div>

    <div class="row q-col-gutter-md">
      <div class="col-12 col-md-7">
        <q-card class="full-height">
          <q-card-section v-if="pasos.every((p) => p.hecho)" class="row items-center no-wrap q-gutter-x-md full-height">
            <q-avatar color="positive" text-color="white" icon="check" size="44px" />
            <div>
              <div class="titulo-seccion">{{ t('inicio.todoListo') }}</div>
              <div class="texto-suave text-body2">{{ t('inicio.todoListoTexto') }}</div>
            </div>
          </q-card-section>
          <q-card-section v-else class="titulo-seccion">{{ t('inicio.primerosPasos') }}</q-card-section>
          <q-list v-if="!pasos.every((p) => p.hecho)">
            <q-item v-for="(p, i) in pasos" :key="p.pagina + i" class="q-py-md">
              <q-item-section avatar>
                <q-avatar size="32px" :color="p.hecho ? 'positive' : 'grey-3'" :text-color="p.hecho ? 'white' : 'grey-8'"
                  :icon="p.hecho ? 'check' : undefined">{{ p.hecho ? '' : i + 1 }}</q-avatar>
              </q-item-section>
              <q-item-section>
                <q-item-label class="text-weight-medium">{{ p.titulo }}</q-item-label>
                <q-item-label caption>{{ p.detalle }}</q-item-label>
              </q-item-section>
              <q-item-section side>
                <q-btn outline color="primary" no-caps :label="p.accion" @click="$emit('ir', p.pagina)" />
              </q-item-section>
            </q-item>
          </q-list>
        </q-card>
      </div>
      <div class="col-12 col-md-5">
        <q-card class="full-height">
          <q-card-section class="row items-center">
            <div class="titulo-seccion col">{{ t('menu.agentes') }}</div>
            <q-btn flat dense no-caps color="primary" :label="t('inicio.detalles')" @click="$emit('ir', 'agentes')" />
          </q-card-section>
          <q-list v-if="adaptadores">
            <q-item v-for="a in adaptadores" :key="a.agente">
              <q-item-section avatar><q-icon name="o_smart_toy" color="grey-7" /></q-item-section>
              <q-item-section>{{ nombreAgente(a.agente) }}</q-item-section>
              <q-item-section side>
                <q-chip dense :color="a.problema ? 'warning' : 'positive'" text-color="white" :icon="a.problema ? 'priority_high' : 'check'">
                  {{ estadoAgente(a) }}
                </q-chip>
              </q-item-section>
            </q-item>
          </q-list>
          <div v-else class="q-pa-md"><q-skeleton v-for="n in 4" :key="n" type="text" class="q-mb-sm" /></div>
        </q-card>
      </div>
    </div>

    <q-card>
      <q-card-section class="row items-center">
        <div class="titulo-seccion col">{{ t('inicio.recientes') }}</div>
        <q-btn flat dense no-caps color="primary" :label="t('inicio.verHistorial')" @click="$emit('ir', 'historial')" />
      </q-card-section>
      <q-list v-if="recientes.length" separator>
        <q-item v-for="trabajo in recientes" :key="trabajo.id" clickable @click="$emit('ir', 'historial')">
          <q-item-section avatar>
            <q-icon :name="estadoInfo(trabajo.estado).icono" :color="estadoInfo(trabajo.estado).color" />
          </q-item-section>
          <q-item-section>
            <q-item-label class="ellipsis">{{ textoDe(trabajo) }}</q-item-label>
            <q-item-label caption>{{ estadoInfo(trabajo.estado).etiqueta }} · {{ modeloDe(trabajo) }}</q-item-label>
          </q-item-section>
          <q-item-section side class="text-caption">{{ haceCuanto(trabajo.creado_en) }}</q-item-section>
        </q-item>
      </q-list>
      <div v-else class="vacio">
        <q-icon name="o_inbox" size="40px" />
        <div class="q-mt-sm">{{ t('inicio.sinTrabajos') }}</div>
      </div>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api, estadoInfo, filtroHistorial, haceCuanto, mensajeError, modeloDe, nombreAgente, sesion, terminales, textoDe, type Adaptador, type EstadoInstancia, type Trabajo } from '../api'
import { t } from '../i18n'

const emit = defineEmits<{ ir: [pagina: string] }>()

function verHistorial(filtro: string) {
  filtroHistorial.value = filtro
  emit('ir', 'historial')
}
const instancia = ref<EstadoInstancia>()
const recientes = ref<Trabajo[]>([])
const adaptadores = ref<Adaptador[]>()
const puntos = ref(0)
const error = ref('')

const total = (estados: string[]) => estados.reduce((n, e) => n + (instancia.value?.trabajos[e] ?? 0), 0)
const contadores = computed(() => [
  { titulo: t('inicio.completados'), detalle: t('inicio.completadosDetalle'), filtro: 'ok', valor: total(['completado']), icono: 'o_check_circle', color: 'positive' },
  { titulo: t('inicio.enCurso'), detalle: t('inicio.enCursoDetalle'), filtro: 'curso',
    valor: total(Object.keys(instancia.value?.trabajos ?? {}).filter((e) => !terminales.includes(e))), icono: 'o_autorenew', color: 'primary' },
  { titulo: t('inicio.conError'), detalle: t('inicio.conErrorDetalle'), filtro: 'error', valor: total(['fallido', 'interrumpido']), icono: 'o_error', color: 'negative' },
])

// estadoAgente resume el problema que informa el servicio (en español) en una etiqueta traducida.
function estadoAgente(a: Adaptador) {
  if (!a.problema) return t('agente.listo')
  if (/no se encontró|envoltorio/.test(a.problema)) return t('agente.noInstalado')
  if (/falta/i.test(a.problema)) return t('agente.faltaClave')
  return t('agente.revisar')
}

const pasos = computed(() => {
  const listos = adaptadores.value?.filter((a) => !a.problema).length ?? 0
  const lista = [
    { titulo: t('inicio.paso1'), detalle: adaptadores.value ? t('inicio.paso1Detalle', { listos, total: adaptadores.value.length }) : t('inicio.comprobando'),
      hecho: listos > 0, accion: t('menu.agentes'), pagina: 'agentes' },
    { titulo: t('inicio.paso2'), detalle: t('inicio.paso2Detalle'), hecho: total(['completado']) > 0, accion: t('menu.ejecutar'), pagina: 'ejecutar' },
    { titulo: t('inicio.paso3'), detalle: t('inicio.paso3Detalle'), hecho: puntos.value > 0, accion: t('inicio.paso3Accion'), pagina: 'conectar' },
  ]
  if (sesion.value?.autoarranque !== undefined) {
    lista.push({ titulo: t('inicio.paso4'), detalle: t('inicio.paso4Detalle'), hecho: sesion.value.autoarranque,
      accion: t('inicio.paso4Accion'), pagina: 'configuracion' })
  }
  return lista
})

async function cargar() {
  try {
    ;[instancia.value, recientes.value] = await Promise.all([api.instancia(), api.trabajos(6)])
    error.value = ''
  } catch (e) {
    error.value = mensajeError(e)
  }
}

let intervalo: number
onMounted(() => {
  cargar()
  api.capacidades().then((c) => (adaptadores.value = c.adaptadores)).catch(() => (adaptadores.value = []))
  api.puntos().then((p) => (puntos.value = p.length)).catch(() => {})
  intervalo = window.setInterval(cargar, 5000)
})
onUnmounted(() => clearInterval(intervalo))
</script>
