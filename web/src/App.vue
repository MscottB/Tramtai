<template>
  <div v-if="cargando" class="fullscreen flex flex-center">
    <q-spinner size="44px" color="primary" />
  </div>
  <EntrarPagina v-else-if="!sesion" :aviso="avisoEntrada" @entrar="sesion = $event" />
  <q-layout v-else view="hHh Lpr fFf">
    <q-header class="cabecera-app">
      <q-toolbar class="q-px-md" style="min-height: 60px">
        <q-btn flat dense round icon="menu" aria-label="Abrir el menú" class="lt-md q-mr-sm" @click="menu = !menu" />
        <img src="/logo.png" alt="" width="30" height="30" style="border-radius: 8px" />
        <div class="q-ml-sm text-weight-bold text-subtitle1">Tramtai</div>
        <q-badge outline color="grey-7" class="q-ml-sm gt-xs">{{ sesion.version }}</q-badge>
        <q-space />
        <q-btn-dropdown v-if="sesion.local && sesion.instancias?.length" flat no-caps :icon="iconoInstancia(sesion.ambito)"
          :label="sesion.nombre || 'Elegir instancia'" aria-label="Cambiar de instancia">
          <q-list style="min-width: 280px">
            <q-item-label header>Instancias activas</q-item-label>
            <q-item v-for="i in sesion.instancias" :key="i.id" v-close-popup clickable :active="i.id === sesion.instancia" @click="cambiar(i.id)">
              <q-item-section avatar><q-icon :name="iconoInstancia(i.ambito)" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ i.nombre }}</q-item-label>
                <q-item-label caption class="ellipsis">{{ i.raiz || 'Sin proyecto: texto y archivos' }}</q-item-label>
              </q-item-section>
              <q-item-section v-if="i.id === sesion.instancia" side><q-icon name="check" color="primary" /></q-item-section>
            </q-item>
            <q-separator />
            <q-item>
              <q-item-section class="text-caption texto-suave">
                Para trabajar con un proyecto, ejecuta <span class="codigo">tramtai proyecto iniciar</span> en su carpeta.
              </q-item-section>
            </q-item>
          </q-list>
        </q-btn-dropdown>
        <q-chip v-else-if="sesion.instancia" outline :icon="iconoInstancia(sesion.ambito)">{{ sesion.nombre || sesion.instancia }}</q-chip>
        <q-btn v-if="!sesion.local" flat round dense icon="logout" aria-label="Salir" class="q-ml-xs" @click="salir">
          <q-tooltip>Salir</q-tooltip>
        </q-btn>
      </q-toolbar>
    </q-header>

    <q-drawer v-model="menu" show-if-above bordered :width="232" :breakpoint="1023" class="cajon-app">
      <q-list class="q-pt-md">
        <q-item v-for="p in paginas" :key="p.id" v-ripple clickable :active="pagina === p.id" active-class="text-primary" @click="ir(p.id)">
          <q-item-section avatar><q-icon :name="p.icono" /></q-item-section>
          <q-item-section>{{ p.titulo }}</q-item-section>
        </q-item>
      </q-list>
    </q-drawer>

    <q-page-container>
      <q-page class="q-pa-md q-pa-lg-lg">
        <div v-if="!sesion.instancia" class="pagina">
          <q-card class="q-pa-lg text-center" style="max-width: 560px; margin: 48px auto">
            <q-icon name="power_settings_new" size="48px" color="primary" />
            <div class="titulo-seccion q-mt-md">Ninguna instancia activa</div>
            <p class="texto-suave q-mt-sm">
              Una instancia es el espacio donde se ejecutan y se guardan tus trabajos. Activa la instancia global para empezar.
            </p>
            <q-btn unelevated color="primary" icon="play_arrow" label="Activar la instancia global" :loading="activando" @click="activarGlobal" />
            <div class="text-caption texto-suave q-mt-md">
              Para un proyecto concreto, ejecuta <span class="codigo">tramtai proyecto iniciar</span> en su carpeta.
            </div>
          </q-card>
        </div>
        <component :is="actual.componente" v-else :key="sesion.instancia" @ir="ir" />
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { computed, markRaw, onMounted, ref } from 'vue'
import { api, avisarError, type Sesion } from './api'
import EntrarPagina from './paginas/Entrar.vue'
import InicioPagina from './paginas/Panel.vue'
import EjecutarPagina from './paginas/Ejecutar.vue'
import HistorialPagina from './paginas/Trabajos.vue'
import ConectarPagina from './paginas/Accesos.vue'
import ConfiguracionPagina from './paginas/Configuracion.vue'
import AgentesPagina from './paginas/Agentes.vue'

const paginas = [
  { id: 'inicio', titulo: 'Inicio', icono: 'o_home', componente: markRaw(InicioPagina) },
  { id: 'ejecutar', titulo: 'Ejecutar', icono: 'o_play_circle', componente: markRaw(EjecutarPagina) },
  { id: 'historial', titulo: 'Historial', icono: 'o_history', componente: markRaw(HistorialPagina) },
  { id: 'conectar', titulo: 'Conectar apps', icono: 'o_link', componente: markRaw(ConectarPagina) },
  { id: 'agentes', titulo: 'Agentes', icono: 'o_smart_toy', componente: markRaw(AgentesPagina) },
  { id: 'configuracion', titulo: 'Configuración', icono: 'o_tune', componente: markRaw(ConfiguracionPagina) },
]
// Direcciones de versiones anteriores de la Web UI.
const antiguas: Record<string, string> = { panel: 'inicio', trabajos: 'historial', accesos: 'conectar' }

function desdeHash(): string {
  const id = location.hash.slice(1)
  return antiguas[id] ?? (paginas.some((p) => p.id === id) ? id : 'inicio')
}

const pagina = ref(desdeHash())
const actual = computed(() => paginas.find((p) => p.id === pagina.value) ?? paginas[0])
const menu = ref(false)
const cargando = ref(true)
const activando = ref(false)
const sesion = ref<Sesion | null>(null)
const avisoEntrada = ref(
  new URLSearchParams(location.search).get('error') === 'codigo'
    ? 'El enlace de acceso caducó o ya se usó. Vuelve a abrirlo con «tramtai abrir».'
    : '',
)

function ir(id: string) {
  pagina.value = id
  history.replaceState(null, '', '#' + id)
  window.scrollTo({ top: 0 })
}

function iconoInstancia(ambito?: string) {
  return ambito === 'proyecto' ? 'o_folder' : 'o_public'
}

onMounted(async () => {
  if (location.search) history.replaceState(null, '', location.pathname + location.hash)
  window.addEventListener('hashchange', () => (pagina.value = desdeHash()))
  try {
    sesion.value = await api.sesion()
  } catch {
    sesion.value = null
  } finally {
    cargando.value = false
  }
})

async function cambiar(instancia: string) {
  if (instancia === sesion.value?.instancia) return
  try {
    sesion.value = await api.cambiarInstancia(instancia)
  } catch (e) {
    avisarError(e)
  }
}

async function activarGlobal() {
  activando.value = true
  try {
    sesion.value = await api.activarGlobal()
  } catch (e) {
    avisarError(e)
  } finally {
    activando.value = false
  }
}

async function salir() {
  await api.salir()
  sesion.value = null
}
</script>
