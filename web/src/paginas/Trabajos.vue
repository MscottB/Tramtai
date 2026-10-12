<template>
  <q-table title="Trabajos" :rows="trabajos" :columns="columnas" row-key="id" :loading="cargando" flat bordered
    :pagination="{ rowsPerPage: 20 }" @row-click="(_e: Event, fila: Trabajo) => abrir(fila)">
    <template #top-right>
      <q-btn flat round icon="refresh" @click="cargar"><q-tooltip>Actualizar</q-tooltip></q-btn>
    </template>
    <template #body-cell-estado="props">
      <q-td :props="props"><q-badge :color="colorEstado(props.value)">{{ props.value }}</q-badge></q-td>
    </template>
  </q-table>

  <q-dialog v-model="dialogo">
    <q-card v-if="detalle" style="width: 900px; max-width: 95vw">
      <q-card-section class="row items-center q-gutter-sm">
        <q-badge :color="colorEstado(detalle.estado)" class="text-body2">{{ detalle.estado }}</q-badge>
        <span class="codigo">{{ detalle.id }}</span>
        <q-space />
        <q-btn flat round icon="close" v-close-popup />
      </q-card-section>
      <q-list dense>
        <q-item><q-item-section>Agente y modelo</q-item-section><q-item-section side>{{ modeloDe(detalle) }}</q-item-section></q-item>
        <q-item v-if="detalle.punto_acceso"><q-item-section>Punto de acceso</q-item-section><q-item-section side>{{ detalle.punto_acceso }}</q-item-section></q-item>
        <q-item><q-item-section>Uso</q-item-section><q-item-section side>{{ detalle.uso.tokens_entrada }} / {{ detalle.uso.tokens_salida }} tokens{{ detalle.uso.costo_usd ? ` · US$ ${detalle.uso.costo_usd.toFixed(4)}` : '' }}</q-item-section></q-item>
        <q-item><q-item-section>Creado</q-item-section><q-item-section side>{{ fecha(detalle.creado_en) }}</q-item-section></q-item>
        <q-item v-if="detalle.codigo_error"><q-item-section class="text-negative">{{ detalle.codigo_error }}: {{ detalle.mensaje_error }}</q-item-section></q-item>
      </q-list>
      <q-separator />
      <q-card-section>
        <div class="text-caption text-grey-7 q-mb-xs">Mensaje</div>
        <pre class="salida">{{ detalle.solicitud.mensajes?.at(-1)?.contenido }}</pre>
      </q-card-section>
      <q-card-section v-if="detalle.salida">
        <div class="text-caption text-grey-7 q-mb-xs">Salida</div>
        <pre class="salida">{{ detalle.salida }}</pre>
      </q-card-section>
      <q-card-actions align="right">
        <q-btn v-if="!terminales.includes(detalle.estado)" flat color="primary" icon="visibility" label="Seguir en vivo" @click="seguirEnVivo" />
        <q-btn v-if="!terminales.includes(detalle.estado)" flat color="negative" icon="stop" label="Cancelar" @click="accion('cancelar')" />
        <q-btn v-else flat color="primary" icon="replay" label="Reintentar" @click="accion('reintentar')" />
        <q-btn v-if="terminales.includes(detalle.estado)" flat color="negative" icon="delete" label="Eliminar" @click="eliminar" />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useQuasar, type QTableColumn } from 'quasar'
import { api, colorEstado, fecha, mensajeError, modeloDe, terminales, trabajoSeguido, type Trabajo } from '../api'

const emit = defineEmits<{ ir: [pagina: string] }>()
const $q = useQuasar()
const trabajos = ref<Trabajo[]>([])
const cargando = ref(false)
const dialogo = ref(false)
const detalle = ref<Trabajo>()
const columnas: QTableColumn<Trabajo>[] = [
  { name: 'estado', label: 'Estado', field: 'estado', align: 'left' },
  { name: 'mensaje', label: 'Mensaje', field: (t) => t.solicitud.mensajes?.at(-1)?.contenido ?? '', align: 'left',
    format: (v: string) => (v.length > 60 ? v.slice(0, 60) + '…' : v) },
  { name: 'modelo', label: 'Agente y modelo', field: (t) => modeloDe(t), align: 'left' },
  { name: 'punto', label: 'Punto', field: (t) => t.punto_acceso ?? '', align: 'left' },
  { name: 'tokens', label: 'Tokens', field: (t) => t.uso.tokens_entrada + t.uso.tokens_salida, align: 'right' },
  { name: 'creado', label: 'Creado', field: 'creado_en', format: (v: string) => fecha(v), align: 'left' },
]

async function cargar() {
  cargando.value = true
  try {
    trabajos.value = await api.trabajos(100)
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  } finally {
    cargando.value = false
  }
}

async function abrir(t: Trabajo) {
  detalle.value = await api.trabajo(t.id)
  dialogo.value = true
}

async function accion(nombre: 'cancelar' | 'reintentar') {
  if (!detalle.value) return
  try {
    const t = await api.accion(detalle.value.id, nombre)
    $q.notify({ type: 'positive', message: nombre === 'reintentar' ? `Nuevo intento ${t.id}` : 'Cancelación solicitada' })
    detalle.value = nombre === 'reintentar' ? t : await api.trabajo(detalle.value.id)
    cargar()
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  }
}

function eliminar() {
  const id = detalle.value?.id
  if (!id) return
  $q.dialog({ title: 'Eliminar trabajo', message: `¿Eliminar ${id}?`, cancel: true }).onOk(async () => {
    try {
      await api.eliminar(id)
      dialogo.value = false
      cargar()
    } catch (e) {
      $q.notify({ type: 'negative', message: mensajeError(e) })
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
