<template>
  <div v-if="cargando" class="fullscreen flex flex-center">
    <q-spinner size="44px" color="primary" />
  </div>
  <EntrarPagina v-else-if="!sesion" :aviso="avisoEntrada" />
  <q-layout v-else view="hHh Lpr fFf">
    <q-header class="cabecera-app">
      <q-toolbar class="q-px-md" style="min-height: 60px">
        <q-btn flat dense round icon="menu" :aria-label="t('app.abrirMenu')" class="lt-md q-mr-sm" @click="menu = !menu" />
        <q-btn v-if="pila.length" flat round dense icon="arrow_back" :aria-label="t('app.volver')" class="q-mr-xs" @click="volver">
          <q-tooltip>{{ t('app.volverA', { pagina: tituloDe(pila[pila.length - 1]) }) }}</q-tooltip>
        </q-btn>
        <img src="/logo.png" alt="" width="28" height="28" style="border-radius: 8px" />
        <div class="q-ml-sm marca">Tramtai</div>
        <span class="version q-ml-sm gt-xs">{{ sesion.version }}</span>
        <q-space />
        <q-btn-dropdown v-if="sesion.local && sesion.instancias?.length" flat no-caps :icon="iconoInstancia(sesion.ambito)"
          :label="sesion.nombre || t('app.elegirInstancia')" :aria-label="t('app.cambiarInstancia')">
          <q-list style="min-width: 300px; max-width: 360px">
            <q-item-label header class="q-pb-xs">{{ t('app.instanciasActivas') }}</q-item-label>
            <div class="q-px-md q-pb-sm text-caption texto-suave">{{ t('app.espacioAyuda') }}</div>
            <q-item v-for="i in sesion.instancias" :key="i.id" v-close-popup clickable :active="i.id === sesion.instancia" @click="cambiar(i.id)">
              <q-item-section avatar><q-icon :name="iconoInstancia(i.ambito)" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ i.nombre }}</q-item-label>
                <q-item-label caption class="ellipsis">{{ i.raiz || t('app.sinProyecto') }}</q-item-label>
              </q-item-section>
              <q-item-section v-if="i.id === sesion.instancia" side><q-icon name="check" color="primary" /></q-item-section>
            </q-item>
            <q-separator />
            <q-item>
              <q-item-section class="text-caption texto-suave">
                <span>{{ t('app.proyectoAntes') }} <span class="codigo">tramtai proyecto iniciar</span> {{ t('app.proyectoDespues') }}</span>
              </q-item-section>
            </q-item>
          </q-list>
        </q-btn-dropdown>
        <q-chip v-else-if="sesion.instancia" outline :icon="iconoInstancia(sesion.ambito)">{{ sesion.nombre || sesion.instancia }}</q-chip>
        <q-btn flat dense no-caps class="pastilla q-ml-sm" :aria-label="t('app.servicio')">
          <span class="punto" :class="conectado ? 'punto--ok' : 'punto--error'" />
          <span class="gt-xs q-ml-sm">{{ conectado ? t('app.enMarcha') : t('app.sinConexion') }}</span>
          <q-icon name="arrow_drop_down" size="20px" class="texto-suave" />
          <q-menu anchor="bottom right" self="top right">
            <q-list style="min-width: 280px; max-width: 340px">
              <q-item>
                <q-item-section avatar><q-icon :name="conectado ? 'o_check_circle' : 'o_cloud_off'" :color="conectado ? 'positive' : 'negative'" /></q-item-section>
                <q-item-section>
                  <q-item-label class="text-weight-medium">{{ conectado ? t('app.enMarcha') : t('app.sinConexion') }}</q-item-label>
                  <q-item-label caption>{{ conectado ? t('app.servicioDetalle', { version: sesion.version, url: origen }) : t('app.comoIniciar') }}</q-item-label>
                </q-item-section>
              </q-item>
              <q-separator />
              <q-item v-if="conectado && sesion.local && sesion.servicio?.reiniciable" v-close-popup clickable @click="reiniciarYSeguir().catch(avisarError)">
                <q-item-section avatar><q-icon name="o_restart_alt" /></q-item-section>
                <q-item-section>{{ t('app.reiniciar') }}</q-item-section>
              </q-item>
              <q-item v-if="conectado && sesion.local" v-close-popup clickable @click="detener">
                <q-item-section avatar><q-icon name="o_power_settings_new" color="negative" /></q-item-section>
                <q-item-section class="text-negative">{{ t('app.detener') }}</q-item-section>
              </q-item>
              <q-item v-else-if="!conectado" v-close-popup clickable @click="comprobar">
                <q-item-section avatar><q-icon name="o_refresh" /></q-item-section>
                <q-item-section>{{ t('app.reintentar') }}</q-item-section>
              </q-item>
            </q-list>
          </q-menu>
        </q-btn>
        <q-btn flat round dense :icon="iconoTema" :aria-label="t('app.tema')" class="q-ml-sm">
          <q-tooltip>{{ t('app.tema') }}</q-tooltip>
          <q-menu anchor="bottom right" self="top right">
            <q-list style="min-width: 200px">
              <q-item v-for="op in temas" :key="op.valor" v-close-popup clickable :active="op.valor === tema" @click="cambiarTema(op.valor)">
                <q-item-section avatar><q-icon :name="op.icono" /></q-item-section>
                <q-item-section>{{ t(op.etiqueta) }}</q-item-section>
                <q-item-section v-if="op.valor === tema" side><q-icon name="check" color="primary" /></q-item-section>
              </q-item>
            </q-list>
          </q-menu>
        </q-btn>
        <q-btn flat round dense icon="o_translate" :aria-label="t('app.idioma')" class="q-ml-xs">
          <q-tooltip>{{ t('app.idioma') }}</q-tooltip>
          <q-menu anchor="bottom right" self="top right">
            <q-list style="min-width: 160px">
              <q-item v-for="(datos, codigo) in idiomas" :key="codigo" v-close-popup clickable :active="codigo === idioma" @click="cambiarIdioma(codigo)">
                <q-item-section>{{ datos.nombre }}</q-item-section>
                <q-item-section v-if="codigo === idioma" side><q-icon name="check" color="primary" /></q-item-section>
              </q-item>
            </q-list>
          </q-menu>
        </q-btn>
        <q-btn v-if="!sesion.local" flat round dense icon="logout" :aria-label="t('app.salir')" class="q-ml-xs" @click="api.salir()">
          <q-tooltip>{{ t('app.salir') }}</q-tooltip>
        </q-btn>
      </q-toolbar>
    </q-header>

    <q-drawer v-model="menu" show-if-above bordered :width="256" :breakpoint="1023" class="cajon-app">
      <div class="column full-height no-wrap">
        <q-list class="q-pt-md q-px-sm">
          <q-item v-for="p in principales" :key="p.id" v-ripple clickable :active="pagina === p.id" active-class="item-activo" class="item-menu" @click="ir(p.id)">
            <q-item-section avatar><q-icon :name="p.icono" /></q-item-section>
            <q-item-section>
              <q-item-label class="text-weight-medium">{{ t(p.titulo) }}</q-item-label>
              <q-item-label caption>{{ t(p.detalle) }}</q-item-label>
            </q-item-section>
          </q-item>
        </q-list>
        <q-space />
        <q-list class="q-pb-md q-px-sm">
          <q-item v-ripple clickable :active="pagina === ayuda.id" active-class="item-activo" class="item-menu" @click="ir(ayuda.id)">
            <q-item-section avatar><q-icon :name="ayuda.icono" /></q-item-section>
            <q-item-section>
              <q-item-label class="text-weight-medium">{{ t(ayuda.titulo) }}</q-item-label>
              <q-item-label caption>{{ t(ayuda.detalle) }}</q-item-label>
            </q-item-section>
          </q-item>
        </q-list>
      </div>
    </q-drawer>

    <q-page-container>
      <q-page class="q-pa-md q-pa-lg-lg">
        <q-banner v-if="!conectado" rounded class="error-caja q-mb-md pagina">
          <template #avatar><q-icon name="o_cloud_off" /></template>
          <div class="text-weight-medium">{{ t('app.sinConexionTexto') }}</div>
          <div class="text-body2 q-mt-xs">{{ t('app.comoIniciar') }}</div>
          <template #action><q-btn flat no-caps icon="o_refresh" :label="t('app.reintentar')" @click="comprobar" /></template>
        </q-banner>
        <div v-if="!sesion.instancia" class="pagina">
          <q-card class="q-pa-lg text-center" style="max-width: 560px; margin: 48px auto">
            <q-icon name="power_settings_new" size="48px" color="primary" />
            <div class="titulo-seccion q-mt-md">{{ t('app.sinInstancia') }}</div>
            <p class="texto-suave q-mt-sm">{{ t('app.sinInstanciaTexto') }}</p>
            <q-btn unelevated color="primary" icon="play_arrow" :label="t('app.activarGlobal')" no-caps :loading="activando" @click="activarGlobal" />
            <div class="text-caption texto-suave q-mt-md">
              {{ t('app.proyectoAntes') }} <span class="codigo">tramtai proyecto iniciar</span> {{ t('app.proyectoDespues') }}
            </div>
          </q-card>
        </div>
        <component :is="actual.componente" v-else :key="sesion.instancia" @ir="ir" />
      </q-page>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { computed, markRaw, onMounted, onUnmounted, ref, watchEffect } from 'vue'
import { useQuasar } from 'quasar'
import { api, avisarError, reiniciarYSeguir, sesion } from './api'
import { cambiarIdioma, idioma, idiomas, t } from './i18n'
import { cambiarTema, tema, type Tema } from './tema'
import EntrarPagina from './paginas/Entrar.vue'
import InicioPagina from './paginas/Panel.vue'
import EjecutarPagina from './paginas/Ejecutar.vue'
import HistorialPagina from './paginas/Trabajos.vue'
import ConectarPagina from './paginas/Accesos.vue'
import ConfiguracionPagina from './paginas/Configuracion.vue'
import AgentesPagina from './paginas/Agentes.vue'
import AyudaPagina from './paginas/Ayuda.vue'

// Orden del menú según el recorrido: conectar un agente, probarlo, conectar las apps y revisar lo ejecutado.
const $q = useQuasar()

const principales = [
  { id: 'inicio', titulo: 'menu.inicio', detalle: 'menuDetalle.inicio', icono: 'o_home', componente: markRaw(InicioPagina) },
  { id: 'agentes', titulo: 'menu.agentes', detalle: 'menuDetalle.agentes', icono: 'o_smart_toy', componente: markRaw(AgentesPagina) },
  { id: 'ejecutar', titulo: 'menu.ejecutar', detalle: 'menuDetalle.ejecutar', icono: 'o_chat', componente: markRaw(EjecutarPagina) },
  { id: 'conectar', titulo: 'menu.conectar', detalle: 'menuDetalle.conectar', icono: 'o_hub', componente: markRaw(ConectarPagina) },
  { id: 'historial', titulo: 'menu.historial', detalle: 'menuDetalle.historial', icono: 'o_history', componente: markRaw(HistorialPagina) },
  { id: 'configuracion', titulo: 'menu.configuracion', detalle: 'menuDetalle.configuracion', icono: 'o_settings', componente: markRaw(ConfiguracionPagina) },
] as const
const ayuda = { id: 'ayuda', titulo: 'menu.ayuda', detalle: 'menuDetalle.ayuda', icono: 'o_help_outline', componente: markRaw(AyudaPagina) } as const
const paginas = [...principales, ayuda]
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
const conectado = ref(true)
const origen = location.origin
let sondeo: number
// comprobar consulta /healthz (sin clave): si el servicio se detiene, la cabecera lo muestra; cuando vuelve, la
// página se recarga para abrir una sesión nueva (las sesiones no sobreviven a un reinicio del servicio).
async function comprobar() {
  const antes = conectado.value
  conectado.value = await fetch('/healthz', { cache: 'no-store' }).then((r) => r.ok).catch(() => false)
  if (!antes && conectado.value) location.reload()
}

function detener() {
  $q.dialog({ title: t('app.detenerTitulo'), message: t('app.detenerTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('app.detenerBoton'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.detenerServicio()
        setTimeout(comprobar, 1500)
      } catch (e) {
        avisarError(e)
      }
    })
}
// En la app de escritorio (WebView2) la bandeja abre aquí sus avisos, con el estilo de la app, y la respuesta le
// vuelve con postMessage.
const escritorio = (window as { chrome?: { webview?: { postMessage(mensaje: string): void } } }).chrome?.webview
function salirEscritorio() {
  $q.dialog({ title: t('escritorio.salirTitulo'), message: t('escritorio.salirTexto'),
    options: { type: 'checkbox', model: [], items: [{ label: t('escritorio.noMostrar'), value: 'no' }] },
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('escritorio.salirBoton'), unelevated: true, noCaps: true } })
    .onOk((marcas: string[]) => escritorio?.postMessage('tramtai:salir:' + (marcas.includes('no') ? '1' : '0')))
}
if (escritorio) {
  Object.assign(window, { tramtaiEscritorio: { detener, salir: salirEscritorio } })
  // La ventana retiene lo que la bandeja le pide ejecutar hasta recibir este aviso de Wails, que solo envía su
  // propio runtime (esta página no lo carga): sin él, los avisos de Salir y Detener no se abrirían.
  escritorio.postMessage('wails:runtime:ready')
}

const temas: { valor: Tema; etiqueta: 'app.temaSistema' | 'app.temaClaro' | 'app.temaOscuro'; icono: string }[] = [
  { valor: 'sistema', etiqueta: 'app.temaSistema', icono: 'o_brightness_auto' },
  { valor: 'claro', etiqueta: 'app.temaClaro', icono: 'o_light_mode' },
  { valor: 'oscuro', etiqueta: 'app.temaOscuro', icono: 'o_dark_mode' },
]
const iconoTema = computed(() => temas.find((op) => op.valor === tema.value)?.icono ?? 'o_brightness_auto')
onUnmounted(() => clearInterval(sondeo))
const avisoEntrada = ref(new URLSearchParams(location.search).get('error') === 'codigo' ? t('app.codigoCaducado') : '')

// pila son las páginas anteriores, para el botón Volver; también funcionan el atrás del navegador y Alt+←.
const pila = ref<string[]>([])

function ir(id: string) {
  if (id === pagina.value) return
  pila.value.push(pagina.value)
  pagina.value = id
  history.pushState(null, '', '#' + id)
  window.scrollTo({ top: 0 })
}

function volver() {
  history.back()
}

function tituloDe(id?: string) {
  const p = paginas.find((x) => x.id === id)
  return p ? t(p.titulo) : ''
}

watchEffect(() => (document.title = `${tituloDe(pagina.value)} · Tramtai`))

function iconoInstancia(ambito?: string) {
  return ambito === 'proyecto' ? 'o_folder' : 'o_public'
}

onMounted(async () => {
  if (location.search) history.replaceState(null, '', location.pathname + location.hash)
  // Atrás o adelante en el navegador: la pila sigue el mismo recorrido.
  window.addEventListener('popstate', () => {
    const nueva = desdeHash()
    if (pila.value[pila.value.length - 1] === nueva) pila.value.pop()
    else pila.value.push(pagina.value)
    pagina.value = nueva
  })
  sondeo = window.setInterval(comprobar, 10000)
  try {
    await api.sesion()
  } catch {
    sesion.value = null
  } finally {
    cargando.value = false
  }
})

async function cambiar(instancia: string) {
  if (instancia === sesion.value?.instancia) return
  try {
    await api.cambiarInstancia(instancia)
  } catch (e) {
    avisarError(e)
  }
}

async function activarGlobal() {
  activando.value = true
  try {
    await api.activarGlobal()
  } catch (e) {
    avisarError(e)
  } finally {
    activando.value = false
  }
}
</script>
