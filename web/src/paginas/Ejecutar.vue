<template>
  <div class="q-gutter-md">
    <q-card>
      <q-card-section class="q-gutter-md">
        <q-select v-model="modelo" :options="opcionesModelo" label="Modelo" emit-value map-options outlined />
        <q-file v-model="archivos" multiple use-chips clearable outlined label="Archivos (PDF, DOCX, PPTX, XLSX, HTML, texto o ZIP)">
          <template #prepend><q-icon name="attach_file" /></template>
        </q-file>
        <q-input v-model="mensaje" type="textarea" autogrow outlined label="Mensaje" hint="Ctrl+Enter para ejecutar" @keydown.ctrl.enter="ejecutar" />
        <q-expansion-item dense icon="data_object" label="Salida estructurada (JSON Schema)">
          <q-input v-model="esquema" type="textarea" autogrow outlined class="q-mt-sm codigo"
            placeholder='{"type":"object","properties":{"resumen":{"type":"string"}},"required":["resumen"]}' />
        </q-expansion-item>
        <div class="row q-gutter-sm">
          <q-btn color="primary" icon="play_arrow" label="Ejecutar" :loading="enviando" :disable="activo || !mensaje.trim()" @click="ejecutar" />
          <q-btn v-if="activo" color="negative" outline icon="stop" label="Cancelar" @click="cancelar" />
        </div>
      </q-card-section>
    </q-card>
    <q-card v-if="trabajo">
      <q-card-section class="row items-center q-gutter-sm">
        <q-badge :color="colorEstado(estado)" class="text-body2">{{ estado }}</q-badge>
        <span class="codigo text-caption">{{ trabajo.id }}</span>
        <q-spinner-dots v-if="activo" color="primary" />
      </q-card-section>
      <q-separator />
      <q-card-section>
        <pre class="salida">{{ salida || (activo ? '…' : '') }}</pre>
      </q-card-section>
      <q-card-section v-if="resumen" class="text-caption text-grey-7">{{ resumen }}</q-card-section>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, aplicarModelo, colorEstado, mensajeError, terminales, trabajoSeguido, type Solicitud, type Trabajo } from '../api'

const $q = useQuasar()
const opcionesModelo = ref<{ label: string; value: string }[]>([{ label: 'Predeterminado de la instancia', value: '' }])
const modelo = ref('')
const archivos = ref<File[] | null>(null)
const mensaje = ref('')
const esquema = ref('')
const enviando = ref(false)
const trabajo = ref<Trabajo>()
const estado = ref('')
const salida = ref('')
const resumen = ref('')
const activo = ref(false)
let fuente: EventSource | undefined

onMounted(async () => {
  try {
    const ids = await api.modelos()
    opcionesModelo.value.push(...ids.map((id) => ({ label: id, value: id })))
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  }
  if (trabajoSeguido.value) {
    const t = await api.trabajo(trabajoSeguido.value)
    trabajoSeguido.value = ''
    seguir(t)
  }
})
onUnmounted(() => fuente?.close()) // el trabajo sigue en el servicio aunque se cierre la página

async function ejecutar() {
  if (activo.value || !mensaje.value.trim()) return
  let esquemaJSON: unknown
  if (esquema.value.trim()) {
    try {
      esquemaJSON = JSON.parse(esquema.value)
    } catch {
      $q.notify({ type: 'negative', message: 'El esquema no es JSON válido' })
      return
    }
  }
  enviando.value = true
  try {
    const cargas: string[] = []
    for (const archivo of archivos.value ?? []) cargas.push((await api.subir(archivo)).identificador_carga)
    let s: Solicitud = { mensajes: [{ rol: 'usuario', contenido: mensaje.value }], cargas: cargas.length ? cargas : undefined }
    if (esquemaJSON) s.salida = { formato: 'json', esquema: esquemaJSON }
    s = aplicarModelo(s, modelo.value)
    seguir(await api.crear(s))
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  } finally {
    enviando.value = false
  }
}

// seguir muestra los eventos en vivo; EventSource se reconecta solo y reanuda con Last-Event-ID.
function seguir(t: Trabajo) {
  fuente?.close()
  trabajo.value = t
  estado.value = t.estado
  salida.value = ''
  resumen.value = ''
  activo.value = !terminales.includes(t.estado)
  fuente = new EventSource(`/api/v1/trabajos/${encodeURIComponent(t.id)}/eventos`)
  fuente.addEventListener('texto', (e) => (salida.value += (e as MessageEvent).data))
  fuente.addEventListener('estado', (e) => (estado.value = (e as MessageEvent).data))
  fuente.addEventListener('fin', () => {
    fuente?.close()
    terminar(t.id)
  })
}

async function terminar(id: string) {
  const final = await api.trabajo(id)
  activo.value = false
  estado.value = final.estado
  if (final.estado === 'completado' && final.salida !== salida.value) {
    salida.value = salida.value ? salida.value + '\n\n— salida final —\n' + final.salida : (final.salida ?? '')
  }
  const partes = [`modelo ${final.modelo_efectivo || 'no verificable'}`, `tokens ${final.uso.tokens_entrada}/${final.uso.tokens_salida}`]
  if (final.uso.costo_usd) partes.push(`US$ ${final.uso.costo_usd.toFixed(4)}`)
  if (final.codigo_error) partes.push(`${final.codigo_error}: ${final.mensaje_error}`)
  resumen.value = partes.join(' · ')
}

async function cancelar() {
  if (!trabajo.value) return
  try {
    await api.accion(trabajo.value.id, 'cancelar')
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  }
}
</script>
