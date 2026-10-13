<template>
  <div class="pagina apilado">
    <div class="row items-end q-col-gutter-md">
      <div class="col-12 col-sm">
        <div class="titulo-pagina">Historial</div>
        <div class="texto-suave">Los trabajos de esta instancia, lanzados desde aquí, la terminal o cualquier aplicación conectada.</div>
      </div>
      <div class="col-12 col-sm-auto">
        <q-btn flat no-caps color="primary" icon="o_refresh" label="Actualizar" :loading="cargando" @click="cargar" />
      </div>
    </div>

    <q-card>
      <q-card-section class="row items-center q-col-gutter-sm">
        <div class="col-12 col-md">
          <q-btn-toggle v-model="filtro" no-caps flat toggle-color="primary"
            :options="[{ label: 'Todos', value: 'todos' }, { label: 'En curso', value: 'curso' }, { label: 'Completados', value: 'ok' }, { label: 'Con error', value: 'error' }]" />
        </div>
        <div class="col-12 col-md-4">
          <q-input v-model="busqueda" dense outlined clearable placeholder="Buscar en las solicitudes" debounce="150">
            <template #prepend><q-icon name="search" /></template>
          </q-input>
        </div>
      </q-card-section>
      <q-separator />
      <q-table :rows="filas" :columns="columnas" row-key="id" flat :loading="cargando && !trabajos.length"
        :pagination="{ rowsPerPage: 25 }" :rows-per-page-options="[25, 50, 100]" no-data-label="No hay trabajos con este filtro"
        table-class="fila-clicable" @row-click="(_e: Event, fila: Trabajo) => abrir(fila)">
        <template #body-cell-estado="props">
          <q-td :props="props">
            <q-chip dense square :color="estadoInfo(props.value).color" text-color="white" :icon="estadoInfo(props.value).icono" class="q-ma-none">
              {{ estadoInfo(props.value).etiqueta }}
            </q-chip>
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
      <q-card v-if="detalle" class="column no-wrap" style="width: 600px; max-width: 100vw; border-radius: 0">
        <q-card-section class="row items-center no-wrap q-gutter-x-sm">
          <q-chip square :color="estadoInfo(detalle.estado).color" text-color="white" :icon="estadoInfo(detalle.estado).icono">
            {{ estadoInfo(detalle.estado).etiqueta }}
          </q-chip>
          <div class="texto-suave text-caption">{{ fecha(detalle.creado_en) }}</div>
          <q-space />
          <q-btn flat round icon="close" aria-label="Cerrar" v-close-popup />
        </q-card-section>
        <q-separator />
        <q-card-section class="col scroll q-gutter-y-lg">
          <q-banner v-if="detalle.codigo_error" rounded dense class="error-caja">
            <template #avatar><q-icon name="o_error" /></template>
            <b>{{ detalle.codigo_error }}</b>: {{ detalle.mensaje_error }}
          </q-banner>
          <div>
            <div class="row items-center q-mb-xs">
              <div class="text-weight-medium col">Solicitud</div>
              <q-btn v-if="textoDe(detalle)" flat dense round size="sm" icon="o_content_copy" aria-label="Copiar la solicitud" @click="copiar(textoDe(detalle))" />
            </div>
            <div class="bloque-texto">{{ textoDe(detalle) || '—' }}</div>
          </div>
          <div>
            <div class="row items-center q-mb-xs">
              <div class="text-weight-medium col">Respuesta</div>
              <q-btn v-if="detalle.salida" flat dense round size="sm" icon="o_content_copy" aria-label="Copiar la respuesta" @click="copiar(detalle.salida)" />
            </div>
            <div v-if="detalle.disponibilidad === 'expirada'" class="texto-suave">La respuesta ya no se conserva: los resultados se guardan 24 horas.</div>
            <div v-else-if="detalle.salida" class="bloque-texto" :class="{ json: esJSON(detalle.salida) }">{{ detalle.salida }}</div>
            <div v-else class="texto-suave">{{ terminales.includes(detalle.estado) ? 'Sin respuesta.' : 'Todavía en curso…' }}</div>
          </div>
          <q-list dense bordered separator class="rounded-borders">
            <q-item><q-item-section class="texto-suave">Agente y modelo</q-item-section><q-item-section side>{{ modeloDe(detalle) }}</q-item-section></q-item>
            <q-item v-if="detalle.punto_acceso"><q-item-section class="texto-suave">Punto de acceso</q-item-section><q-item-section side>{{ detalle.punto_acceso }}</q-item-section></q-item>
            <q-item><q-item-section class="texto-suave">Tokens</q-item-section><q-item-section side>{{ detalle.uso.tokens_entrada }} → {{ detalle.uso.tokens_salida }}</q-item-section></q-item>
            <q-item v-if="detalle.uso.costo_usd"><q-item-section class="texto-suave">Coste</q-item-section><q-item-section side>US$ {{ detalle.uso.costo_usd.toFixed(4) }}</q-item-section></q-item>
            <q-item v-if="detalle.terminado_en"><q-item-section class="texto-suave">Duración</q-item-section><q-item-section side>{{ duracion(detalle.creado_en, detalle.terminado_en) }}</q-item-section></q-item>
            <q-item>
              <q-item-section class="texto-suave">Identificador</q-item-section>
              <q-item-section side class="codigo cursor-pointer" @click="copiar(detalle.id)">{{ detalle.id }}</q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
        <q-separator />
        <q-card-actions class="q-pa-md">
          <q-btn v-if="terminales.includes(detalle.estado)" flat color="negative" icon="o_delete" label="Eliminar" no-caps @click="eliminar" />
          <q-space />
          <template v-if="!terminales.includes(detalle.estado)">
            <q-btn flat color="negative" icon="o_stop_circle" label="Cancelar" no-caps @click="accion('cancelar')" />
            <q-btn unelevated color="primary" icon="o_visibility" label="Ver en vivo" no-caps @click="seguirEnVivo" />
          </template>
          <q-btn v-else unelevated color="primary" icon="o_replay" label="Reintentar" no-caps @click="accion('reintentar')" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useQuasar, type QTableColumn } from 'quasar'
import { api, avisarError, copiar, duracion, esJSON, estadoInfo, fecha, haceCuanto, modeloDe, terminales, textoDe, trabajoSeguido, type Trabajo } from '../api'

const emit = defineEmits<{ ir: [pagina: string] }>()
const $q = useQuasar()
const trabajos = ref<Trabajo[]>([])
const cargando = ref(false)
const dialogo = ref(false)
const detalle = ref<Trabajo>()
const filtro = ref('todos')
const busqueda = ref<string | null>('')

const columnas: QTableColumn<Trabajo>[] = [
  { name: 'estado', label: 'Estado', field: 'estado', align: 'left' },
  { name: 'solicitud', label: 'Solicitud', field: (t) => textoDe(t), align: 'left' },
  { name: 'modelo', label: 'Agente y modelo', field: (t) => modeloDe(t), align: 'left', classes: 'gt-sm', headerClasses: 'gt-sm' },
  { name: 'tokens', label: 'Tokens', field: (t) => t.uso.tokens_entrada + t.uso.tokens_salida, align: 'right', classes: 'gt-xs', headerClasses: 'gt-xs' },
  { name: 'cuando', label: 'Cuándo', field: 'creado_en', align: 'right' },
]

const filas = computed(() => {
  const texto = (busqueda.value ?? '').toLowerCase().trim()
  return trabajos.value.filter((t) => {
    const terminado = terminales.includes(t.estado)
    const pasa = { todos: true, curso: !terminado, ok: t.estado === 'completado', error: ['fallido', 'interrumpido'].includes(t.estado) }[filtro.value]
    return pasa && (!texto || textoDe(t).toLowerCase().includes(texto) || t.id.includes(texto))
  })
})

async function cargar() {
  cargando.value = true
  try {
    trabajos.value = await api.trabajos(200)
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
}

async function abrir(t: Trabajo) {
  try {
    detalle.value = await api.trabajo(t.id)
    dialogo.value = true
  } catch (e) {
    avisarError(e)
  }
}

async function accion(nombre: 'cancelar' | 'reintentar') {
  if (!detalle.value) return
  try {
    const t = await api.accion(detalle.value.id, nombre)
    if (nombre === 'reintentar') {
      trabajoSeguido.value = t.id
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

function eliminar() {
  const id = detalle.value?.id
  if (!id) return
  $q.dialog({ title: 'Eliminar trabajo', message: 'Se borran la solicitud, la respuesta y su historial. No se puede deshacer.',
    cancel: { flat: true, label: 'Cancelar', noCaps: true }, ok: { color: 'negative', label: 'Eliminar', unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.eliminar(id)
        dialogo.value = false
        cargar()
      } catch (e) {
        avisarError(e)
      }
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
  intervalo = window.setInterval(() => !dialogo.value && cargar(), 5000)
})
onUnmounted(() => clearInterval(intervalo))
</script>
