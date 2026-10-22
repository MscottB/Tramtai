<template>
  <div class="pagina apilado">
    <CabeceraPagina tema="inicio" :titulo="t('menu.inicio')">
      <template #descripcion>
        <div class="row items-center q-gutter-x-sm">
          <span>{{ t('inicio.direccion') }}</span>
          <span class="direccion">{{ instancia?.base ?? '…' }}</span>
          <q-btn v-if="instancia?.base" flat round dense size="sm" icon="o_content_copy" :aria-label="t('inicio.copiarDireccion')" @click="copiar(instancia.base)">
            <q-tooltip>{{ t('inicio.copiarDireccion') }}</q-tooltip>
          </q-btn>
        </div>
      </template>
      <template #acciones>
        <q-btn outline color="primary" icon="o_hub" :label="t('menu.conectar')" no-caps @click="$emit('ir', 'conectar')" />
        <q-btn unelevated color="primary" icon="o_play_arrow" :label="t('menu.ejecutar')" no-caps @click="$emit('ir', 'ejecutar')" />
      </template>
    </CabeceraPagina>

    <q-banner v-if="error" rounded class="error-caja" role="alert">{{ error }}
      <template #action><q-btn flat no-caps icon="o_refresh" :label="t('comun.reintentar')" @click="cargar" /></template>
    </q-banner>
    <q-banner v-if="instancia?.sugerir_api_propia" rounded class="aviso">
      {{ t('claves.sugerencia', { n: instancia?.app_con_clave_24h ?? 0 }) }}
      <template #action><q-btn flat no-caps color="primary" :label="t('claves.crearApi')" @click="$emit('ir', 'conectar')" /></template>
    </q-banner>
    <q-banner v-if="conProblemas.length" rounded class="aviso">
      {{ conProblemas.length === 1 ? t('inicio.espaciosConProblemaUno', { nombre: conProblemas[0].nombre }) : t('inicio.espaciosConProblema', { n: conProblemas.length }) }}
      <template #action><q-btn flat no-caps color="primary" :label="t('inicio.verEspacios')" @click="$emit('ir', 'espacios')" /></template>
    </q-banner>
    <AvisoConversor v-if="conversor && conversor.estado !== 'preparando'" enlace-ajustes @ajustes="$emit('ir', 'configuracion')" />

    <section class="q-card" data-captura="inicio" data-recorrido="inicio.mapa">
      <div class="seccion row items-end">
        <div class="col">
          <h2 class="titulo-seccion q-ma-none">{{ t('inicio.red') }}</h2>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('inicio.redTexto') }}</div>
        </div>
        <q-btn flat dense no-caps color="primary" :label="t('inicio.detalles')" @click="$emit('ir', 'agentes')" />
      </div>
      <div class="q-px-md q-pb-lg q-pt-sm">
        <MapaRed v-if="adaptadores && instancia" :adaptadores="ordenarAgentes(adaptadores)" :apis="puntos" :base="instancia.base"
          :tranvias="vivos.lista" :resto="vivos.resto" @ir="(p) => $emit('ir', p)" @seguir="seguir" />
        <q-skeleton v-else type="rect" height="200px" />
        <SiguiendoTrabajo v-if="seguido" :tranvia="seguido" class="q-mt-md" @dejar="seguido = undefined" @ir="(p) => $emit('ir', p)" />
        <div v-else-if="vivos.lista.length" class="text-caption texto-suave q-mt-sm">{{ t('vivo.pista') }}</div>
      </div>
    </section>

    <!-- La tarjeta siempre sale: con la guía oculta o completa solo queda la línea para repetir el recorrido. -->
    <section class="q-card" data-recorrido="inicio.primeros-pasos">
      <div class="seccion row items-center no-wrap">
        <div class="col">
          <h2 class="titulo-seccion q-ma-none">{{ t('inicio.primerosPasos') }}</h2>
          <div v-if="guiaVisible && pasosServicio" class="text-caption texto-suave q-mt-xs">{{ t('inicio.guiaProgreso', { hechos: pasos.filter((p) => p.hecho).length, total: pasos.length }) }}</div>
        </div>
        <q-btn v-if="guiaVisible" flat dense no-caps icon="o_visibility_off" :label="t('inicio.ocultarGuia')" class="texto-suave" @click="ocultar" />
      </div>
      <div v-if="guiaVisible && !pasosServicio && !errorPasos" class="text-caption texto-suave q-px-md q-pt-sm">{{ t('inicio.comprobando') }}</div>
      <!-- Si no se pudieron comprobar, la guía no desaparece como si estuviera completa: dice qué pasó y deja reintentar. -->
      <EstadoVacio v-if="guiaVisible && errorPasos" role="alert" :titulo="t('inicio.errorPasos')" :texto="errorPasos" :accion="t('comun.reintentar')" :cargando="comprobandoPasos" @accion="comprobar" />
      <ol v-if="guiaVisible && pasos.length" class="ruta ruta--pasos q-ma-none q-px-md q-pt-sm q-pb-lg">
        <li v-for="p in pasos" :key="p.pagina + p.titulo" class="ruta__parada" :class="{ 'ruta__parada--pasada': p.hecho }">
          <button type="button" class="paso-estacion" @click="$emit('ir', p.pagina)">
            <span class="ruta__punto" aria-hidden="true" />
            <span class="paso-estacion__titulo">{{ p.titulo }}</span>
            <span class="text-caption texto-suave">{{ p.detalle }}</span>
          </button>
        </li>
      </ol>
      <div class="q-px-md q-pb-md" :class="{ 'q-pt-sm': !guiaVisible }">
        <q-btn flat dense no-caps color="primary" icon="o_tour" :label="t('bienvenida.repetir')" @click="emit('recorrido')" />
      </div>
    </section>

    <section>
      <div class="row items-end q-mb-sm">
        <div class="col">
          <h2 class="titulo-seccion q-ma-none">{{ t('inicio.peticiones') }}</h2>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('inicio.peticionesTexto', { periodo: periodoHistorial }) }}</div>
        </div>
      </div>
      <div class="tablero">
        <div class="tablero__cabecera"><span>{{ t('inicio.tablero') }}</span><span>{{ t('inicio.tableroPeriodo', { periodo: periodoHistorial }) }}</span></div>
        <button v-for="c in contadores" :key="c.titulo" type="button" class="tablero__fila" @click="verHistorial(c.filtro)">
          <span class="tablero__punto" :class="'tablero__punto--' + c.tipo" aria-hidden="true" />
          <span class="tablero__nombre">{{ c.titulo }}</span>
          <span class="tablero__detalle">{{ c.detalle }}</span>
          <span class="tablero__cifra">{{ c.valor }}</span>
          <q-icon name="chevron_right" size="20px" />
        </button>
      </div>
    </section>

    <section class="q-card">
      <div class="seccion row items-center">
        <h2 class="titulo-seccion q-ma-none col">{{ t('inicio.recientes') }}</h2>
        <q-btn flat dense no-caps color="primary" :label="t('inicio.verHistorial')" @click="$emit('ir', 'historial')" />
      </div>
      <q-separator />
      <!-- Hasta la primera respuesta no se sabe si hay peticiones: ni lista ni vacío (si falla, lo dice el aviso de arriba). -->
      <div v-if="!recientes" class="q-pa-md" role="status" :aria-label="t('comun.cargando')"><q-skeleton v-for="n in 3" :key="n" type="text" /></div>
      <q-list v-else-if="recientes.length" separator>
        <q-item v-for="trabajo in recientes" :key="trabajo.id" clickable class="q-py-sm" @click="verPeticion(trabajo.id)">
          <q-item-section style="max-width: 120px" class="gt-xs">
            <q-item-label caption class="tabular">{{ haceCuanto(trabajo.creado_en) }}</q-item-label>
          </q-item-section>
          <q-item-section>
            <q-item-label class="ellipsis">{{ textoDe(trabajo) }}</q-item-label>
            <q-item-label caption class="lt-sm">{{ haceCuanto(trabajo.creado_en) }}</q-item-label>
          </q-item-section>
          <q-item-section class="gt-sm" style="max-width: 300px">
            <Linea :agente="trabajo.solicitud.agente" :modelo="trabajo.modelo_efectivo || trabajo.modelo_enviado" class="text-body2" />
          </q-item-section>
          <q-item-section side>
            <span :class="estadoInfo(trabajo.estado).clase">{{ estadoInfo(trabajo.estado).etiqueta }}</span>
          </q-item-section>
        </q-item>
      </q-list>
      <EstadoVacio v-else :titulo="t('inicio.sinTrabajos')" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import Linea from '../componentes/Linea.vue'
import MapaRed from '../componentes/MapaRed.vue'
import SiguiendoTrabajo from '../componentes/SiguiendoTrabajo.vue'
import { useEventosInstancia } from '../composables/useEventosInstancia'
import { aplicar, eventoDe, visibles, type Tranvia } from '../vivo'
import AvisoConversor from '../componentes/AvisoConversor.vue'
import { cargarConversor, conversor } from '../conversor'
import { api, avisar, cargarDiasHistorial, copiar, mensajeError } from '../api/cliente'
import type { Adaptador, EspacioUI, ResumenInstancia, EstadoPunto, Paso, Trabajo } from '../api/tipos'
import { periodoHistorial, guiaOculta, ocultarGuia, sesion, sistema } from '../estado'
import type { Destino } from '../navegacion'
import { asegurarFichas, ordenarAgentes } from '../agentes'
import { estadoInfo, haceCuanto, textoDe } from '../formato'
import { conTexto, guiaVisible as mostrarGuia } from '../pasos'
import { t } from '../i18n'

const emit = defineEmits<{ ir: [destino: Destino]; recorrido: [] }>()

function ocultar() {
  ocultarGuia(true)
  avisar(t('inicio.guiaOculta'))
}

function verPeticion(id: string) {
  emit('ir', { pagina: 'historial', abrir: id })
}

function verHistorial(filtro: string) {
  emit('ir', { pagina: 'historial', filtro })
}
const instancia = ref<ResumenInstancia>()
const recientes = ref<Trabajo[]>()
const adaptadores = ref<Adaptador[]>()
const puntos = ref<EstadoPunto[]>([])

// Las peticiones en curso, en vivo; la seguida guarda su último estado aunque ya haya salido del mapa.
const { tranvias } = useEventosInstancia()
const vivos = computed(() => visibles(tranvias.value))
const seguido = ref<Tranvia>()
function seguir(id: string) {
  const tr = tranvias.value.get(id)
  if (tr) seguido.value = { ...tr }
}
let pidiendoFinal = ''
watch(tranvias, (m) => {
  const tr = seguido.value && m.get(seguido.value.id)
  if (tr) seguido.value = { ...tr }
  else if (seguido.value && seguido.value.terminadoEn === undefined) traerFinal(seguido.value)
}, { deep: true })
// traerFinal pide cómo acabó la seguida cuando salió del mapa sin llegar (el flujo estuvo cortado mientras terminaba).
function traerFinal(previo: Tranvia) {
  if (pidiendoFinal === previo.id) return
  pidiendoFinal = previo.id
  api.trabajo(previo.id).then((trabajo) => {
    if (seguido.value?.id !== previo.id) return
    const m = new Map([[previo.id, previo]])
    aplicar(m, eventoDe(trabajo), Date.now(), new Set())
    seguido.value = m.get(previo.id)
  }).catch(() => {}).finally(() => (pidiendoFinal = '')) // sin servicio: la tarjeta se queda como estaba
}
const error = ref('')

// cuenta son los trabajos de un filtro del Historial; los agrupa el servicio.
const cuenta = (filtro: string) => instancia.value?.contadores[filtro] ?? 0
const contadores = computed(() => [
  { titulo: t('inicio.completados'), detalle: t('inicio.completadosDetalle'), filtro: 'completado', valor: cuenta('completado'), tipo: 'ok' },
  { titulo: t('inicio.enCurso'), detalle: t('inicio.enCursoDetalle'), filtro: 'en_curso', valor: cuenta('en_curso'), tipo: 'curso' },
  { titulo: t('inicio.conError'), detalle: t('inicio.conErrorDetalle'), filtro: 'con_error', valor: cuenta('con_error'), tipo: 'error' },
])

// pasosServicio son los Primeros pasos que calcula el servicio (llegan con la comprobación de los agentes).
const pasosServicio = ref<Paso[]>()
const pasos = computed(() => conTexto(pasosServicio.value ?? [], sistema))
const guiaVisible = computed(() => mostrarGuia(guiaOculta.value, pasosServicio.value))
// errorPasos es por qué no llegaron los pasos; solo cuenta mientras no haya llegado ninguna lista.
const errorPasos = ref('')
const comprobandoPasos = ref(false)

// consultando evita encadenar consultas si el servicio tarda más que el intervalo.
let consultando = false
async function cargar() {
  if (consultando) return
  consultando = true
  try {
    ;[instancia.value, recientes.value] = await Promise.all([api.instancia(), api.trabajos(7)])
    error.value = ''
  } catch (e) {
    error.value = mensajeError(e)
  } finally {
    consultando = false
  }
}

// conProblemas son los espacios que no se pudieron abrir (carpeta movida, local.yaml con claves no aplicadas…).
const conProblemas = ref<EspacioUI[]>([])

let intervalo: number
onMounted(() => {
  if (sesion.value?.local) api.espacios().then((l) => (conProblemas.value = l.filter((e) => e.ultimo_error))).catch(() => {})
  cargarConversor()
  cargarDiasHistorial()
  cargar()
  asegurarFichas()
  comprobar()
  api.puntos().then((p) => (puntos.value = p)).catch(() => {})
  intervalo = window.setInterval(() => !document.hidden && cargar(), 5000)
  comprobacion = window.setInterval(() => !document.hidden && comprobar(), 5 * 60_000)
})
// comprobar pide los Primeros pasos y los agentes con que se calcularon (ejecuta cada CLI: tarda). Sin respuesta, los
// agentes quedan vacíos y, si nunca llegaron pasos, la guía enseña el error con Reintentar.
function comprobar() {
  comprobandoPasos.value = true
  api.pasos().then((r) => {
    adaptadores.value = r.adaptadores
    pasosServicio.value = r.pasos
    errorPasos.value = ''
  }).catch((e) => {
    adaptadores.value ??= []
    if (!pasosServicio.value) errorPasos.value = mensajeError(e)
  }).finally(() => (comprobandoPasos.value = false))
}
let comprobacion = 0
onUnmounted(() => {
  clearInterval(intervalo)
  clearInterval(comprobacion)
})
</script>
