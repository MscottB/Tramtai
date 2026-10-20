<template>
  <div class="pagina apilado">
    <CabeceraPagina tema="historial" :titulo="t('menu.historial')" :descripcion="t('historial.subtitulo')">
      <template #acciones>
        <q-btn flat no-caps color="negative" icon="o_delete_sweep" :label="t('historial.vaciar')" :disable="!hayTerminados" @click="vaciar" />
        <q-btn flat no-caps color="primary" icon="o_refresh" :label="t('comun.actualizar')" :loading="cargando" @click="cargar()" />
      </template>
    </CabeceraPagina>

    <q-card data-captura="historial">
      <q-card-section class="row items-center q-col-gutter-sm">
        <div class="col-12 col-md">
          <q-btn-toggle v-model="filtro" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
            :options="[{ label: t('historial.todos'), value: 'todos' }, { label: t('inicio.enCurso'), value: 'curso' }, { label: t('inicio.completados'), value: 'ok' }, { label: t('inicio.conError'), value: 'error' }]" />
        </div>
        <div class="col-12 col-sm-6 col-md-3">
          <q-select v-model="origen" :options="origenes" emit-value map-options dense outlined :label="t('historial.origen')" />
        </div>
        <div class="col-12 col-sm-6 col-md-3">
          <q-input v-model="busqueda" dense outlined clearable :placeholder="t('historial.buscar')" :aria-label="t('historial.buscar')" debounce="150">
            <template #prepend><q-icon name="search" /></template>
          </q-input>
        </div>
      </q-card-section>
      <q-separator />
      <q-table :rows="filas" :columns="columnas" row-key="id" flat :loading="cargando && !trabajos.length"
        :pagination="{ rowsPerPage: 25 }" :rows-per-page-options="[25, 50, 100]" :no-data-label="t('historial.sinFiltro')"
        table-class="fila-clicable" @row-click="(_e: Event, fila: Trabajo) => abrir(fila)">
        <template #no-data>
          <EstadoVacio v-if="!cargando" class="full-width" :titulo="hayFiltro ? t('historial.sinFiltro') : t('historial.vacio')"
            :accion="hayFiltro ? t('historial.quitarFiltros') : t('historial.irProbar')" @accion="hayFiltro ? quitarFiltros() : emit('ir', 'ejecutar')" />
        </template>
        <template #body-cell-estado="props">
          <q-td :props="props">
            <span :class="estadoInfo(props.value).clase"><q-icon :name="estadoInfo(props.value).icono" size="14px" />{{ estadoInfo(props.value).etiqueta }}</span>
          </q-td>
        </template>
        <template #body-cell-modelo="props">
          <q-td :props="props" style="max-width: 280px">
            <Linea :agente="props.row.solicitud.agente" :modelo="props.row.modelo_efectivo || props.row.modelo_enviado" />
          </q-td>
        </template>
        <template #body-cell-solicitud="props">
          <q-td :props="props" style="max-width: 420px; white-space: normal">
            <!-- El clic abre por @row-click; el teclado llega aquí para que cada fila se pueda abrir sin ratón. -->
            <div class="recortar-2" role="button" tabindex="0" @keydown.enter.prevent="abrir(props.row)" @keydown.space.prevent="abrir(props.row)">{{ props.value }}</div>
          </q-td>
        </template>
        <template #body-cell-cuando="props">
          <q-td :props="props">{{ haceCuanto(props.value) }}<q-tooltip>{{ fecha(props.value) }}</q-tooltip></q-td>
        </template>
      </q-table>
      <div v-if="trabajos.length" class="row items-center q-px-md q-pb-md q-gutter-x-md">
        <span class="text-caption texto-suave">{{ t('historial.mostrando', { n: trabajos.length }) }}</span>
        <q-btn v-if="hayMas" flat dense no-caps color="primary" icon="o_expand_more" :label="t('historial.verMas')" :loading="cargandoMas" @click="verMas" />
      </div>
    </q-card>

    <!-- allow-focus-outside: «¿Qué hago?» abre el panel de la Ayuda encima y el foco tiene que poder ir a él. -->
    <q-dialog v-model="dialogo" position="right" full-height allow-focus-outside>
      <q-card v-if="detalle" class="column no-wrap hoja-detalle" style="width: 600px; max-width: 100vw">
        <q-card-section class="row items-center no-wrap q-gutter-x-sm">
          <span :class="estadoInfo(detalle.estado).clase"><q-icon :name="estadoInfo(detalle.estado).icono" size="14px" />{{ estadoInfo(detalle.estado).etiqueta }}</span>
          <div class="texto-suave text-caption">{{ fecha(detalle.creado_en) }}</div>
          <q-space />
          <q-btn v-close-popup flat round icon="close" :aria-label="t('comun.cerrar')" />
        </q-card-section>
        <q-separator />
        <q-card-section class="q-py-md">
          <Ruta :estado="detalle.estado" />
        </q-card-section>
        <q-separator />
        <q-card-section class="col scroll q-gutter-y-lg">
          <q-banner v-if="detalle.codigo_error" rounded dense class="error-caja">
            <template #avatar><q-icon name="o_error" /></template>
            <b>{{ tituloError(detalle.codigo_error) }}</b><span v-if="detalle.mensaje_error">: {{ detalle.mensaje_error }}</span>
            <template #action><BotonAyuda tema="problemas" :ancla="'e-' + detalle.codigo_error" que-hago /></template>
          </q-banner>
          <div>
            <div class="row items-center q-mb-xs">
              <div class="text-weight-medium col">{{ t('historial.solicitud') }}</div>
              <q-btn v-if="textoDe(detalle)" flat dense round size="sm" icon="o_content_copy" :aria-label="t('historial.copiarSolicitud')" @click="copiar(textoDe(detalle))" />
            </div>
            <div class="bloque-texto">{{ textoDe(detalle) || '—' }}</div>
          </div>
          <div>
            <div class="row items-center q-mb-xs">
              <div class="text-weight-medium col">{{ t('ejecutar.respuesta') }}</div>
              <q-btn v-if="detalle.salida" flat dense round size="sm" icon="o_content_copy" :aria-label="t('ejecutar.copiarRespuesta')" @click="copiar(detalle.salida)" />
            </div>
            <div v-if="detalle.disponibilidad === 'expirada'" class="texto-suave">{{ t('historial.expirada') }}</div>
            <SalidaTrabajo v-else-if="detalle.salida" :texto="detalle.salida" caja />
            <div v-else class="texto-suave">{{ terminales.includes(detalle.estado) ? t('historial.sinRespuesta') : t('historial.todaviaEnCurso') }}</div>
          </div>
          <div class="ficha text-body2">
            <div>{{ t('historial.agenteModelo') }}</div><div>{{ modeloDe(detalle) }}</div>
            <div>{{ t('historial.esfuerzo') }}</div><div>{{ esfuerzoUsado(detalle, eventosDetalle) || '—' }}</div>
            <div>{{ t('historial.origen') }}</div><div>{{ textoOrigen(detalle) }}</div>
            <div>{{ t('historial.tokens') }}</div><div>{{ detalle.uso.tokens_entrada }} → {{ detalle.uso.tokens_salida }}</div>
            <template v-if="detalle.uso.costo_usd"><div>{{ t('historial.coste') }}</div><div>US$ {{ detalle.uso.costo_usd.toFixed(4) }}</div></template>
            <template v-if="detalle.terminado_en"><div>{{ t('historial.duracion') }}</div><div>{{ duracion(detalle.creado_en, detalle.terminado_en) }}</div></template>
            <div>{{ t('historial.identificador') }}</div>
            <div><q-btn flat dense no-caps class="codigo q-pa-none" :label="detalle.id" :aria-label="t('comun.copiar') + ' ' + detalle.id" @click="copiar(detalle.id)"><q-tooltip>{{ t('comun.copiar') }}</q-tooltip></q-btn></div>
          </div>
          <RegistroTrabajo :trabajo="detalle" :version="detalle.estado" @cargado="(e: EventoRegistro[]) => (eventosDetalle = e)" />
        </q-card-section>
        <q-separator />
        <q-card-actions class="q-pa-md">
          <q-btn v-if="terminales.includes(detalle.estado)" flat color="negative" icon="o_delete" :label="t('comun.eliminar')" no-caps @click="eliminar" />
          <q-space />
          <template v-if="!terminales.includes(detalle.estado)">
            <q-btn flat color="negative" icon="o_stop_circle" :label="t('comun.cancelar')" no-caps @click="accion('cancelar')" />
            <q-btn unelevated color="primary" icon="o_visibility" :label="t('historial.verEnVivo')" no-caps @click="seguirEnVivo" />
          </template>
          <q-btn v-else unelevated color="primary" icon="o_replay" :label="t('historial.reintentar')" no-caps @click="accion('reintentar')" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import BotonAyuda from '../componentes/ayuda/BotonAyuda.vue'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import { useQuasar, type QTableColumn } from 'quasar'
import Linea from '../componentes/Linea.vue'
import Ruta from '../componentes/Ruta.vue'
import RegistroTrabajo from '../componentes/RegistroTrabajo.vue'
import SalidaTrabajo from '../componentes/SalidaTrabajo.vue'
import { api, avisar, avisarError, copiar, tituloError } from '../api/cliente'
import type { EstadoPunto, EventoRegistro, Trabajo } from '../api/tipos'
import { filtroHistorial, origenHistorial, trabajoAbrir, trabajoSeguido } from '../estado'
import { modeloDe } from '../agentes'
import { duracion, estadoInfo, fecha, haceCuanto, esfuerzoUsado, terminales, textoDe } from '../formato'
import { t } from '../i18n'

const emit = defineEmits<{ ir: [pagina: string] }>()
const $q = useQuasar()
const trabajos = ref<Trabajo[]>([])
const hayTerminados = computed(() => trabajos.value.some((x) => terminales.includes(x.estado)))
const cargando = ref(false)
const dialogo = ref(false)
const detalle = ref<Trabajo>()
const eventosDetalle = ref<EventoRegistro[]>([])
const filtro = ref(filtroHistorial.value || 'todos')
filtroHistorial.value = ''
const busqueda = ref<string | null>('')
// origen: '' todos; «o:probar» un origen, «g:web» un grupo o «p:tienda» una API (se filtra en el servicio).
// Llega una API por su nombre (Mis APIs) o un filtro ya armado («o:probar» desde Probar).
const origen = ref(origenHistorial.value.includes(':') ? origenHistorial.value : origenHistorial.value ? 'p:' + origenHistorial.value : '')
const hayFiltro = computed(() => filtro.value !== 'todos' || !!busqueda.value || !!origen.value)
function quitarFiltros() {
  filtro.value = 'todos'
  busqueda.value = ''
  origen.value = ''
}
origenHistorial.value = ''
const apis = ref<EstadoPunto[]>([])
const nombresOrigen: Record<string, 'historial.origenProbar' | 'historial.origenTerminal' | 'historial.origenCli' | 'historial.origenApp'> = {
  probar: 'historial.origenProbar', terminal: 'historial.origenTerminal', cli: 'historial.origenCli', app: 'historial.origenApp',
}
const origenes = computed(() => [
  { label: t('historial.todosOrigenes'), value: '' },
  ...Object.entries(nombresOrigen).map(([o, clave]) => ({ label: t(clave), value: 'o:' + o })),
  ...[...new Set(apis.value.map((p) => p.grupo).filter((g): g is string => !!g))].sort().map((g) => ({ label: t('historial.grupo', { nombre: g }), value: 'g:' + g })),
  ...apis.value.map((p) => ({ label: t('historial.api', { nombre: p.nombre }), value: 'p:' + p.nombre })),
])
// textoOrigen dice de dónde llegó: la API (con su grupo actual), el origen guardado o, en los antiguos, la principal.
function textoOrigen(x: Trabajo) {
  if (x.punto_acceso) {
    const grupo = apis.value.find((p) => p.nombre === x.punto_acceso)?.grupo
    return t('historial.api', { nombre: x.punto_acceso }) + (grupo ? ' · ' + grupo : '')
  }
  return x.origen && nombresOrigen[x.origen] ? t(nombresOrigen[x.origen]) : t('historial.origenPrincipal')
}
function filtroDe(valor: string) {
  const [tipo, nombre] = [valor.slice(0, 2), valor.slice(2)]
  return tipo === 'o:' ? { origen: nombre } : tipo === 'g:' ? { grupo: nombre } : tipo === 'p:' ? { punto: nombre } : {}
}
watch(origen, () => cargar())

const columnas = computed<QTableColumn<Trabajo>[]>(() => [
  { name: 'estado', label: t('historial.estado'), field: 'estado', align: 'left' },
  { name: 'solicitud', label: t('historial.solicitud'), field: (x) => textoDe(x), align: 'left' },
  { name: 'modelo', label: t('historial.agenteModelo'), field: (x) => modeloDe(x), align: 'left', classes: 'gt-sm', headerClasses: 'gt-sm' },
  { name: 'origen', label: t('historial.origen'), field: (x) => textoOrigen(x), align: 'left', classes: 'gt-sm', headerClasses: 'gt-sm' },
  { name: 'tokens', label: t('historial.tokens'), field: (x) => x.uso.tokens_entrada + x.uso.tokens_salida, align: 'right', classes: 'gt-xs', headerClasses: 'gt-xs' },
  { name: 'cuando', label: t('historial.cuando'), field: 'creado_en', align: 'right' },
])

const filas = computed(() => {
  const texto = (busqueda.value ?? '').toLowerCase().trim()
  return trabajos.value.filter((x) => {
    const terminado = terminales.includes(x.estado)
    const pasa = { todos: true, curso: !terminado, ok: x.estado === 'completado', error: ['fallido', 'interrumpido'].includes(x.estado) }[filtro.value]
    return pasa && (!texto || textoDe(x).toLowerCase().includes(texto) || x.id.includes(texto))
  })
})

// pedidoLista y pedidoDetalle numeran las consultas: solo se aplica la respuesta de la última, para que una
// lenta (otro origen, otra fila) no pise a la más reciente. enVuelo evita encadenar consultas en segundo plano.
let pedidoLista = 0
let pedidoDetalle = 0
let enVuelo = 0

// cargar actualiza la lista; en segundo plano (el sondeo) no muestra el indicador ni avisa de errores.
async function cargar(segundoPlano = false) {
  const n = ++pedidoLista
  if (!segundoPlano) cargando.value = true
  enVuelo++
  try {
    const lista = await api.trabajos(PAGINA, filtroDe(origen.value))
    if (n === pedidoLista) {
      trabajos.value = lista
      hayMas.value = lista.length === PAGINA
    }
  } catch (e) {
    if (n === pedidoLista && !segundoPlano) avisarError(e)
  } finally {
    enVuelo--
    if (n === pedidoLista) cargando.value = false
  }
}

// Ver más pide la página siguiente: los creados antes del último que ya se ve.
const PAGINA = 200
const hayMas = ref(false)
const cargandoMas = ref(false)
async function verMas() {
  const ultimo = trabajos.value[trabajos.value.length - 1]
  if (!ultimo) return
  const n = pedidoLista // si el filtro cambia mientras carga, esta página ya no es de la lista que se ve
  cargandoMas.value = true
  enVuelo++
  try {
    const mas = await api.trabajos(PAGINA, { ...filtroDe(origen.value), antes: ultimo.creado_en, antes_id: ultimo.id })
    if (n !== pedidoLista) return
    trabajos.value = [...trabajos.value, ...mas]
    hayMas.value = mas.length === PAGINA
  } catch (e) {
    if (n === pedidoLista) avisarError(e)
  } finally {
    enVuelo--
    cargandoMas.value = false
  }
}

async function abrir(elegido: Trabajo) {
  eventosDetalle.value = []
  const n = ++pedidoDetalle
  try {
    const trabajo = await api.trabajo(elegido.id)
    if (n !== pedidoDetalle) return
    detalle.value = trabajo
    dialogo.value = true
  } catch (e) {
    if (n === pedidoDetalle) avisarError(e)
  }
}

async function accion(nombre: 'cancelar' | 'reintentar') {
  if (!detalle.value) return
  try {
    const nuevo = await api.accion(detalle.value.id, nombre)
    if (nombre === 'reintentar') {
      trabajoSeguido.value = nuevo.id
      dialogo.value = false
      emit('ir', 'ejecutar')
      return
    }
    detalle.value = await api.trabajo(detalle.value.id)
    cargar()
  } catch (e) {
    avisarError(e)
  }
}

function confirmar(titulo: string, mensaje: string, hacer: () => Promise<void>) {
  $q.dialog({ title: titulo, message: mensaje,
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('comun.eliminar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await hacer()
      } catch (e) {
        avisarError(e)
      }
    })
}

function eliminar() {
  const id = detalle.value?.id
  if (!id) return
  confirmar(t('historial.eliminarTitulo'), t('historial.eliminarTexto'), async () => {
    await api.eliminar(id)
    dialogo.value = false
    cargar()
  })
}

function vaciar() {
  confirmar(t('historial.vaciarTitulo'), t('historial.vaciarTexto'), async () => {
    avisar(t('historial.vaciado', { n: await api.vaciarHistorial() }))
    cargar()
  })
}

function seguirEnVivo() {
  if (!detalle.value) return
  trabajoSeguido.value = detalle.value.id
  dialogo.value = false
  emit('ir', 'ejecutar')
}

let intervalo: number
onMounted(() => {
  cargar()
  api.puntos().then((l) => (apis.value = l)).catch(() => {})
  if (trabajoAbrir.value) {
    abrir({ id: trabajoAbrir.value } as Trabajo)
    trabajoAbrir.value = ''
  }
  // Con páginas extra cargadas (Ver más), el sondeo no recarga: la recarga solo trae la primera página.
  intervalo = window.setInterval(() => !dialogo.value && !document.hidden && !enVuelo && trabajos.value.length <= PAGINA && cargar(true), 5000)
})
onUnmounted(() => clearInterval(intervalo))
</script>
