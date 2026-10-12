<template>
  <q-table title="Configuración efectiva" :rows="entradas" :columns="columnas" row-key="clave" :filter="filtro" :loading="cargando"
    flat bordered dense :pagination="{ rowsPerPage: 0 }" hide-pagination>
    <template #top-right>
      <q-input v-model="filtro" dense outlined debounce="200" placeholder="Filtrar">
        <template #append><q-icon name="search" /></template>
      </q-input>
    </template>
    <template #body-cell-origen="props">
      <q-td :props="props"><q-badge outline color="primary">{{ props.value }}</q-badge></q-td>
    </template>
  </q-table>
  <div class="text-caption text-grey-7 q-mt-sm">
    Precedencia de menor a mayor: interno, global, proyecto, local y entorno. Se edita en los archivos de configuración
    (<span class="codigo">tramtai configuracion explicar &lt;clave&gt;</span>).
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useQuasar, type QTableColumn } from 'quasar'
import { api, mensajeError, type EntradaConfiguracion } from '../api'

const $q = useQuasar()
const entradas = ref<EntradaConfiguracion[]>([])
const filtro = ref('')
const cargando = ref(false)
const columnas: QTableColumn<EntradaConfiguracion>[] = [
  { name: 'clave', label: 'Clave', field: 'clave', align: 'left', classes: 'codigo' },
  { name: 'valor', label: 'Valor', field: (e) => (typeof e.valor === 'object' ? JSON.stringify(e.valor) : String(e.valor)), align: 'left' },
  { name: 'origen', label: 'Origen', field: 'origen', align: 'left' },
]

onMounted(async () => {
  cargando.value = true
  try {
    entradas.value = await api.configuracion()
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  } finally {
    cargando.value = false
  }
})
</script>
