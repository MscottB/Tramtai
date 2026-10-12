<template>
  <q-table title="Puntos de acceso" :rows="puntos" :columns="columnas" row-key="nombre" :loading="cargando" flat bordered hide-pagination
    :pagination="{ rowsPerPage: 0 }" no-data-label="Sin puntos de acceso: crea uno para cada aplicación que se conecte">
    <template #top-right>
      <q-btn color="primary" icon="add" label="Nuevo" @click="abrirNuevo" />
    </template>
    <template #body-cell-url="props">
      <q-td :props="props" class="codigo">
        {{ props.value || '—' }}
        <q-btn v-if="props.value" flat dense round size="sm" icon="content_copy" @click="copiar(props.value)" />
      </q-td>
    </template>
    <template #body-cell-estado="props">
      <q-td :props="props">
        <q-toggle :model-value="props.row.error !== 'desactivado'" @update:model-value="(v: boolean) => activar(props.row.nombre, v)" />
        <span v-if="props.row.error && props.row.error !== 'desactivado'" class="text-negative text-caption">{{ props.row.error }}</span>
      </q-td>
    </template>
    <template #body-cell-acciones="props">
      <q-td :props="props">
        <q-btn flat dense round icon="key" @click="mostrarToken(props.row.nombre)"><q-tooltip>Token</q-tooltip></q-btn>
        <q-btn flat dense round icon="delete" color="negative" @click="eliminar(props.row.nombre)"><q-tooltip>Eliminar</q-tooltip></q-btn>
      </q-td>
    </template>
  </q-table>

  <q-dialog v-model="nuevo">
    <q-card style="width: 640px; max-width: 95vw">
      <q-card-section class="text-h6">Nuevo punto de acceso</q-card-section>
      <q-card-section class="q-gutter-md">
        <q-input v-model="formulario.nombre" label="Nombre" outlined hint="minúsculas, números, _ y -" />
        <q-input v-model.number="formulario.puerto" type="number" label="Puerto propio (opcional)" outlined
          hint="Vacío: /p/<nombre> en el puerto del servicio" />
        <div>
          <div class="text-caption text-grey-7">Contratos</div>
          <q-option-group v-model="formulario.contratos" type="checkbox" inline
            :options="[{ label: 'Nativa', value: 'nativa' }, { label: 'OpenAI', value: 'openai' }, { label: 'Anthropic', value: 'anthropic' }]" />
        </div>
        <q-select v-model="formulario.modelo" :options="modelos" label="Modelo predeterminado" outlined clearable />
        <q-select v-model="formulario.permitidos" :options="modelos" label="Modelos permitidos (vacío: todos)" outlined multiple use-chips />
        <q-input v-model="formulario.alias" type="textarea" autogrow outlined label="Alias de modelos"
          hint="Uno por línea: gpt-4o-mini=claude/haiku" class="codigo" />
        <q-toggle v-model="formulario.forzar" label="Forzar el modelo del punto (ignora el de la solicitud)" />
      </q-card-section>
      <q-card-actions align="right">
        <q-btn flat label="Cancelar" v-close-popup />
        <q-btn color="primary" label="Crear" :loading="guardando" :disable="!formulario.nombre" @click="crear" />
      </q-card-actions>
    </q-card>
  </q-dialog>

  <q-dialog v-model="dialogoToken">
    <q-card style="width: 560px; max-width: 95vw">
      <q-card-section class="text-h6">Token de {{ tokenNombre }}</q-card-section>
      <q-card-section>
        <q-input :model-value="token" readonly outlined class="codigo">
          <template #append><q-btn flat round icon="content_copy" @click="copiar(token)" /></template>
        </q-input>
        <div class="text-caption text-grey-7 q-mt-sm">Úsalo como API key en la aplicación que se conecta a este punto.</div>
      </q-card-section>
      <q-card-actions align="right"><q-btn flat label="Cerrar" v-close-popup /></q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useQuasar, type QTableColumn } from 'quasar'
import { api, aplicarModelo, mensajeError, type ConfigPunto, type EstadoPunto } from '../api'

const $q = useQuasar()
const puntos = ref<EstadoPunto[]>([])
const modelos = ref<string[]>([])
const cargando = ref(false)
const nuevo = ref(false)
const guardando = ref(false)
const dialogoToken = ref(false)
const token = ref('')
const tokenNombre = ref('')
const columnas: QTableColumn<EstadoPunto>[] = [
  { name: 'nombre', label: 'Nombre', field: 'nombre', align: 'left' },
  { name: 'url', label: 'URL', field: (p) => p.url ?? '', align: 'left' },
  { name: 'contratos', label: 'Contratos', field: (p) => p.contratos.join(', '), align: 'left' },
  { name: 'modelo', label: 'Modelo', field: (p) => [p.agente, p.modelo].filter(Boolean).join('/') || '—', align: 'left' },
  { name: 'estado', label: 'Activo', field: 'activo', align: 'left' },
  { name: 'acciones', label: '', field: 'nombre', align: 'right' },
]
const vacio = () => ({ nombre: '', puerto: undefined as number | undefined, contratos: ['nativa', 'openai', 'anthropic'],
  modelo: null as string | null, permitidos: [] as string[], alias: '', forzar: false })
const formulario = reactive(vacio())

async function cargar() {
  cargando.value = true
  try {
    puntos.value = await api.puntos()
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  } finally {
    cargando.value = false
  }
}

async function abrirNuevo() {
  Object.assign(formulario, vacio())
  nuevo.value = true
  if (!modelos.value.length) modelos.value = await api.modelos().catch(() => [])
}

async function crear() {
  const alias: Record<string, string> = {}
  for (const linea of formulario.alias.split('\n').map((l) => l.trim()).filter(Boolean)) {
    const [origen, destino] = linea.split('=').map((p) => p.trim())
    if (!origen || !destino) {
      $q.notify({ type: 'negative', message: `Alias no válido: ${linea}` })
      return
    }
    alias[origen] = destino
  }
  const elegido = aplicarModelo({ mensajes: [] }, formulario.modelo ?? '')
  const punto: ConfigPunto = {
    nombre: formulario.nombre.trim(),
    puerto: formulario.puerto || undefined,
    contratos: formulario.contratos,
    agente: elegido.agente,
    modelo: elegido.modelo,
    perfil_modelo: elegido.perfil_modelo,
    modelos_permitidos: formulario.permitidos.length ? formulario.permitidos : undefined,
    alias_modelos: Object.keys(alias).length ? alias : undefined,
    forzar_modelo: formulario.forzar || undefined,
  }
  guardando.value = true
  try {
    const r = await api.crearPunto(punto)
    nuevo.value = false
    await cargar()
    tokenNombre.value = punto.nombre
    token.value = r.token
    dialogoToken.value = true
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  } finally {
    guardando.value = false
  }
}

async function activar(nombre: string, activo: boolean) {
  try {
    await api.activarPunto(nombre, activo)
    await cargar()
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  }
}

function eliminar(nombre: string) {
  $q.dialog({ title: 'Eliminar punto de acceso', message: `¿Eliminar ${nombre}? Las aplicaciones que lo usan dejarán de funcionar.`, cancel: true })
    .onOk(async () => {
      try {
        await api.eliminarPunto(nombre)
        await cargar()
      } catch (e) {
        $q.notify({ type: 'negative', message: mensajeError(e) })
      }
    })
}

async function mostrarToken(nombre: string) {
  try {
    token.value = await api.tokenPunto(nombre)
    tokenNombre.value = nombre
    dialogoToken.value = true
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  }
}

async function copiar(texto: string) {
  await navigator.clipboard.writeText(texto)
  $q.notify({ message: 'Copiado', timeout: 1000 })
}

onMounted(cargar)
</script>
