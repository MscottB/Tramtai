<template>
  <div v-if="cargando" class="fullscreen flex flex-center">
    <q-spinner size="48px" color="primary" />
  </div>
  <EntrarPagina v-else-if="!sesion" :aviso="avisoEntrada" @entrar="sesion = $event" />
  <q-layout v-else view="hHh lpR fFf">
    <q-header elevated class="bg-primary text-white">
      <q-toolbar>
        <q-btn flat dense round icon="menu" aria-label="Menú" @click="menu = !menu" />
        <q-toolbar-title>
          Tramtai <span class="text-caption q-ml-sm">{{ sesion.version }}</span>
        </q-toolbar-title>
        <q-chip dense color="white" text-color="primary" icon="dns">
          {{ sesion.instancia }}
          <q-tooltip>{{ sesion.ambito === 'proyecto' ? sesion.raiz : 'Instancia global' }}</q-tooltip>
        </q-chip>
        <q-btn flat dense round icon="logout" aria-label="Salir" @click="salir">
          <q-tooltip>Salir</q-tooltip>
        </q-btn>
      </q-toolbar>
    </q-header>
    <q-drawer v-model="menu" show-if-above bordered :width="230">
      <q-list padding>
        <q-item v-for="p in paginas" :key="p.id" v-ripple clickable :active="pagina === p.id" @click="pagina = p.id">
          <q-item-section avatar><q-icon :name="p.icono" /></q-item-section>
          <q-item-section>{{ p.titulo }}</q-item-section>
        </q-item>
      </q-list>
    </q-drawer>
    <q-page-container>
      <q-page padding>
        <component :is="actual.componente" @ir="pagina = $event" />
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { computed, markRaw, onMounted, ref, watch } from 'vue'
import { api, type Sesion } from './api'
import EntrarPagina from './paginas/Entrar.vue'
import PanelPagina from './paginas/Panel.vue'
import EjecutarPagina from './paginas/Ejecutar.vue'
import TrabajosPagina from './paginas/Trabajos.vue'
import AccesosPagina from './paginas/Accesos.vue'
import ConfiguracionPagina from './paginas/Configuracion.vue'
import AgentesPagina from './paginas/Agentes.vue'

const paginas = [
  { id: 'panel', titulo: 'Panel', icono: 'dashboard', componente: markRaw(PanelPagina) },
  { id: 'ejecutar', titulo: 'Ejecutar', icono: 'play_circle', componente: markRaw(EjecutarPagina) },
  { id: 'trabajos', titulo: 'Trabajos', icono: 'list_alt', componente: markRaw(TrabajosPagina) },
  { id: 'accesos', titulo: 'Puntos de acceso', icono: 'hub', componente: markRaw(AccesosPagina) },
  { id: 'configuracion', titulo: 'Configuración', icono: 'tune', componente: markRaw(ConfiguracionPagina) },
  { id: 'agentes', titulo: 'Agentes', icono: 'smart_toy', componente: markRaw(AgentesPagina) },
]

const pagina = ref(location.hash.slice(1) || 'panel')
const actual = computed(() => paginas.find((p) => p.id === pagina.value) ?? paginas[0])
watch(pagina, (p) => history.replaceState(null, '', '#' + p))

const menu = ref(false)
const cargando = ref(true)
const sesion = ref<Sesion | null>(null)
const avisoEntrada = ref(
  new URLSearchParams(location.search).get('error') === 'codigo'
    ? 'El enlace de acceso caducó o ya se usó. Vuelve a abrirlo con "tramtai global abrir" o entra con el token.'
    : '',
)

onMounted(async () => {
  if (location.search) history.replaceState(null, '', location.pathname + location.hash)
  try {
    sesion.value = await api.sesion()
  } catch {
    sesion.value = null
  } finally {
    cargando.value = false
  }
})

async function salir() {
  await api.salir()
  sesion.value = null
}
</script>
