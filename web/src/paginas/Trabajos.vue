<template>
  <div class="pagina apilado">
    <div class="row items-end q-col-gutter-md">
      <div class="col-12 col-sm">
        <div class="titulo-pagina">{{ t('menu.historial') }}</div>
        <div class="texto-suave">{{ t('historial.subtitulo') }}</div>
      </div>
      <div class="col-12 col-sm-auto row no-wrap q-gutter-x-sm">
        <q-btn flat no-caps color="negative" icon="o_delete_sweep" :label="t('historial.vaciar')" :disable="!trabajos.some((x) => terminales.includes(x.estado))" @click="vaciar" />
        <q-btn flat no-caps color="primary" icon="o_refresh" :label="t('comun.actualizar')" :loading="cargando" @click="cargar" />
      </div>
    </div>

    <q-card>
      <q-card-section class="row items-center q-col-gutter-sm">
        <div class="col-12 col-md">
          <q-btn-toggle v-model="filtro" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
            :options="[{ label: t('historial.todos'), value: 'todos' }, { label: t('inicio.enCurso'), value: 'curso' }, { label: t('inicio.completados'), value: 'ok' }, { label: t('inicio.conError'), value: 'error' }]" />
        </div>
        <div class="col-12 col-sm-6 col-md-3">
          <q-select v-model="origen" :options="origenes" emit-value map-options dense outlined :label="t('historial.origen')" />
        </div>
        <div class="col-12 col-sm-6 col-md-3">
          <q-input v-model="busqueda" dense outlined clearable :placeholder="t('historial.buscar')" debounce="150">
            <template #prepend><q-icon name="search" /></template>
          </q-input>
        </div>
      </q-card-section>
      <q-separator />
      <q-table :rows="filas" :columns="columnas" row-key="id" flat :loading="cargando && !trabajos.length"
        :pagination="{ rowsPerPage: 25 }" :rows-per-page-options="[25, 50, 100]" :no-data-label="t('historial.sinFiltro')"
        table-class="fila-clicable" @row-click="(_e: Event, fila: Trabajo) => abrir(fila)">
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
            <div class="recortar-2">{{ props.value }}</div>
          </q-td>
        </template>
        <template #body-cell-cuando="props">
          <q-td :props="props">{{ haceCuanto(props.value) }}<q-tooltip>{{ fecha(props.value) }}</q-tooltip></q-td>
        </template>
      </q-table>
    </q-card>

    <q-dialog v-model="dialogo" position="right" full-height>
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
            <div v-else-if="detalle.salida" class="bloque-texto" :class="{ json: esJSON(detalle.salida) }">{{ detalle.salida }}</div>
            <div v-else class="texto-suave">{{ terminales.includes(detalle.estado) ? t('historial.sinRespuesta') : t('historial.todaviaEnCurso') }}</div>
          </div>
          <div class="ficha text-body2">
            <div>{{ t('historial.agenteModelo') }}</div><div>{{ modeloDe(detalle) }}</div>
            <div>{{ t('historial.esfuerzo') }}</div><div>{{ nombreEsfuerzo(detalle.solicitud.esfuerzo) }}</div>
            <template v-if="detalle.punto_acceso"><div>{{ t('historial.punto') }}</div><div>{{ detalle.punto_acceso }}</div></template>
            <div>{{ t('historial.tokens') }}</div><div>{{ detalle.uso.tokens_entrada }} → {{ detalle.uso.tokens_salida }}</div>
            <template v-if="detalle.uso.costo_usd"><div>{{ t('historial.coste') }}</div><div>US$ {{ detalle.uso.costo_usd.toFixed(4) }}</div></template>
            <template v-if="detalle.terminado_en"><div>{{ t('historial.duracion') }}</div><div>{{ duracion(detalle.creado_en, detalle.terminado_en) }}</div></template>
            <div>{{ t('historial.identificador') }}</div>
            <div class="codigo cursor-pointer" @click="copiar(detalle.id)">{{ detalle.id }}<q-tooltip>{{ t('comun.copiar') }}</q-tooltip></div>
          </div>
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
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useQuasar, type QTableColumn } from 'quasar'
import Linea from '../componentes/Linea.vue'
import Ruta from '../componentes/Ruta.vue'
import { api, avisar, avisarError, copiar, duracion, esJSON, estadoInfo, fecha, filtroHistorial, haceCuanto, origenHistorial, trabajoAbrir, modeloDe, nombreEsfuerzo, terminales, textoDe, tituloError, trabajoSeguido, type Trabajo } from '../api'
import { t } from '../i18n'

const emit = defineEmits<{ ir: [pagina: string] }>()
const $q = useQuasar()
const trabajos = ref<Trabajo[]>([])
const cargando = ref(false)
const dialogo = ref(false)
const detalle = ref<Trabajo>()
const filtro = ref(filtroHistorial.value || 'todos')
filtroHistorial.value = ''
const busqueda = ref<string | null>('')
// origen: '' todas, '-' la API principal, o el nombre de una API (se filtra en el servicio).
const origen = ref(origenHistorial.value)
origenHistorial.value = ''
const nombresApis = ref<string[]>([])
const origenes = computed(() => [
  { label: t('historial.todosOrigenes'), value: '' },
  { label: t('historial.origenPrincipal'), value: '-' },
  ...nombresApis.value.map((n) => ({ label: 'API ' + n, value: n })),
])
watch(origen, () => cargar())

const columnas = computed<QTableColumn<Trabajo>[]>(() => [
  { name: 'estado', label: t('historial.estado'), field: 'estado', align: 'left' },
  { name: 'solicitud', label: t('historial.solicitud'), field: (x) => textoDe(x), align: 'left' },
  { name: 'modelo', label: t('historial.agenteModelo'), field: (x) => modeloDe(x), align: 'left', classes: 'gt-sm', headerClasses: 'gt-sm' },
  { name: 'origen', label: t('historial.origen'), field: (x) => (x.punto_acceso ? 'API ' + x.punto_acceso : t('historial.origenPrincipal')), align: 'left', classes: 'gt-sm', headerClasses: 'gt-sm' },
  { name: 'tokens', label: t('historial.tokens'), field: (x) => x.uso.tokens_entrada + x.uso.tokens_salida, align: 'right', classes: 'gt-xs', headerClasses: 'gt-xs' },
  { name: 'cuando', label: t('historial.cuando'), field: 'creado_en', align: 'right' },
])

const filas = computed(() => {
  const texto = (busqueda.value ?? '').toLowerCase().trim()
  return trabajos.value.filter((x) => {
    const terminado = terminales.includes(x.estado)
    const pasa = { todos: true, curso: !terminado, ok: x.estado === 'completado', error: ['fallido', 'interrumpido'].includes(x.estado) }[filtro.value]
    return pasa && (origen.value !== '-' || !x.punto_acceso) && (!texto || textoDe(x).toLowerCase().includes(texto) || x.id.includes(texto))
  })
})

async function cargar() {
  cargando.value = true
  try {
    trabajos.value = await api.trabajos(200, origen.value === '-' ? '' : origen.value)
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
}

async function abrir(elegido: Trabajo) {
  try {
    detalle.value = await api.trabajo(elegido.id)
    dialogo.value = true
  } catch (e) {
    avisarError(e)
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
  api.puntos().then((l) => (nombresApis.value = l.map((x) => x.nombre))).catch(() => {})
  if (trabajoAbrir.value) {
    abrir({ id: trabajoAbrir.value } as Trabajo)
    trabajoAbrir.value = ''
  }
  intervalo = window.setInterval(() => !dialogo.value && cargar(), 5000)
})
onUnmounted(() => clearInterval(intervalo))
</script>
