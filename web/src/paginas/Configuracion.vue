<template>
  <div class="pagina apilado">
    <div>
      <div class="titulo-pagina">{{ t('menu.configuracion') }}</div>
      <div class="texto-suave">{{ t('config.subtitulo') }}</div>
    </div>

    <div class="row q-col-gutter-md">
      <div v-if="sesion?.autoarranque !== undefined" class="col-12 col-md-6">
        <q-card class="full-height">
          <q-card-section class="row items-start no-wrap">
            <q-icon name="o_rocket_launch" size="28px" color="primary" class="q-mr-md q-mt-xs" />
            <div class="col">
              <div class="titulo-seccion">{{ t('arranque.titulo') }}</div>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('arranque.texto') }}</div>
            </div>
            <q-toggle :model-value="sesion.autoarranque" :disable="cambiandoArranque" :aria-label="t('arranque.titulo')" @update:model-value="cambiarArranque" />
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12" :class="{ 'col-md-6': sesion?.autoarranque !== undefined }">
        <q-card class="full-height">
          <q-card-section class="row items-start no-wrap q-pb-sm">
            <q-icon name="o_delete_sweep" size="28px" color="negative" class="q-mr-md q-mt-xs" />
            <div class="col">
              <div class="titulo-seccion">{{ t('datos.titulo') }}</div>
              <div class="texto-suave text-body2 q-mt-xs">{{ t('datos.texto') }}</div>
            </div>
          </q-card-section>
          <q-list dense>
            <q-item tag="label">
              <q-item-section avatar><q-checkbox v-model="borrarHistorial" :disable="!terminados" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ t('datos.historial') }}</q-item-label>
                <q-item-label caption>{{ t('datos.historialDetalle', { n: terminados }) }}</q-item-label>
              </q-item-section>
            </q-item>
            <q-item tag="label">
              <q-item-section avatar><q-checkbox v-model="borrarPuntos" :disable="!puntos.length" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ t('datos.puntos') }}</q-item-label>
                <q-item-label caption>{{ t('datos.puntosDetalle', { n: puntos.length }) }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
          <q-card-actions class="q-px-md q-pb-md">
            <q-btn outline color="negative" no-caps icon="o_delete" :label="t('datos.boton')" :loading="borrando"
              :disable="!(borrarHistorial && terminados) && !(borrarPuntos && puntos.length)" @click="borrar" />
          </q-card-actions>
        </q-card>
      </div>
    </div>

    <div class="row items-end q-col-gutter-md q-mt-sm">
      <div class="col-12 col-sm">
        <div class="titulo-seccion">{{ t('menu.configuracion') }} · {{ sesion?.nombre || sesion?.instancia }}</div>
      </div>
      <div class="col-12 col-sm-5 col-md-4">
        <q-input v-model="filtro" dense outlined clearable debounce="150" :placeholder="t('config.buscar')">
          <template #prepend><q-icon name="search" /></template>
        </q-input>
      </div>
    </div>

    <q-card>
      <q-card-section class="row q-col-gutter-md text-body2">
        <div v-for="o in origenes" :key="o.id" class="col-12 col-sm-6 col-md">
          <q-chip dense square :color="o.color" text-color="white" class="q-ml-none">{{ o.nombre }}</q-chip>
          <div class="texto-suave text-caption q-mt-xs romper">{{ o.detalle }}</div>
        </div>
      </q-card-section>
    </q-card>

    <q-card v-for="g in grupos" :key="g.titulo">
      <q-card-section class="titulo-seccion q-pb-sm">{{ g.titulo }}</q-card-section>
      <q-list dense separator>
        <q-item v-for="e in g.entradas" :key="e.clave" class="q-py-sm">
          <q-item-section>
            <q-item-label class="codigo">{{ e.clave }}</q-item-label>
          </q-item-section>
          <q-item-section side class="text-right" style="max-width: 55%">
            <q-item-label class="codigo" style="word-break: break-all">{{ valor(e.valor) }}</q-item-label>
          </q-item-section>
          <q-item-section side style="min-width: 90px">
            <q-chip dense square :color="origen(e.origen).color" text-color="white">{{ origen(e.origen).nombre }}</q-chip>
          </q-item-section>
        </q-item>
      </q-list>
    </q-card>
    <div v-if="!cargando && !grupos.length" class="vacio">{{ t('config.sinCoincidencias') }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, sesion, terminales, type EntradaConfiguracion, type EstadoPunto } from '../api'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const $q = useQuasar()
const entradas = ref<EntradaConfiguracion[]>([])
const filtro = ref<string | null>('')
const cargando = ref(true)
const cambiandoArranque = ref(false)
const terminados = ref(0)
const puntos = ref<EstadoPunto[]>([])
const borrarHistorial = ref(true)
const borrarPuntos = ref(false)
const borrando = ref(false)

const origenes = computed(() => [
  { id: 'interno', nombre: t('origen.interno'), color: 'grey-6', detalle: t('origen.internoDetalle') },
  { id: 'global', nombre: t('origen.global'), color: 'primary', detalle: '%APPDATA%\\tramtai\\config.yaml' },
  { id: 'proyecto', nombre: t('origen.proyecto'), color: 'teal-8', detalle: t('origen.proyectoDetalle') },
  { id: 'local', nombre: t('origen.local'), color: 'indigo-6', detalle: t('origen.localDetalle') },
  { id: 'entorno', nombre: t('origen.entorno'), color: 'deep-orange-7', detalle: t('origen.entornoDetalle') },
])
const origen = (id: string) => origenes.value.find((o) => o.id === id) ?? { nombre: id, color: 'grey-6' }

const secciones: Record<string, Clave> = {
  servicio: 'seccion.servicio', predeterminados: 'seccion.predeterminados', modelos: 'seccion.modelos', adaptadores: 'seccion.agentes',
  comandos_agentes: 'seccion.agentes', concurrencia: 'seccion.concurrencia', concurrencia_servicio: 'seccion.concurrencia',
  retencion: 'seccion.retencion', conversion: 'seccion.archivos', compatibilidad: 'seccion.compatibilidad',
}

const grupos = computed(() => {
  const texto = (filtro.value ?? '').toLowerCase().trim()
  const mapa = new Map<string, EntradaConfiguracion[]>()
  for (const e of entradas.value) {
    if (texto && !e.clave.toLowerCase().includes(texto) && !valor(e.valor).toLowerCase().includes(texto)) continue
    const titulo = t(secciones[e.clave.split('.')[0]] ?? 'seccion.otros')
    mapa.set(titulo, [...(mapa.get(titulo) ?? []), e])
  }
  return [...mapa.entries()].map(([titulo, lista]) => ({ titulo, entradas: lista }))
})

function valor(v: unknown): string {
  if (v === true) return t('comun.si').toLowerCase()
  if (v === false) return t('comun.no').toLowerCase()
  return typeof v === 'object' ? JSON.stringify(v) : String(v)
}

async function cambiarArranque(activo: boolean) {
  cambiandoArranque.value = true
  try {
    await api.autoarranque(activo)
    avisar(activo ? t('arranque.activado') : t('arranque.desactivado'))
  } catch (e) {
    avisarError(e)
  } finally {
    cambiandoArranque.value = false
  }
}

async function contar() {
  const [instancia, lista] = await Promise.all([api.instancia(), api.puntos()])
  terminados.value = terminales.reduce((n, e) => n + (instancia.trabajos[e] ?? 0), 0)
  puntos.value = lista
}

function borrar() {
  $q.dialog({ title: t('datos.titulo'), message: t('datos.confirmar', { instancia: sesion.value?.nombre || sesion.value?.instancia || '' }),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('comun.eliminar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      borrando.value = true
      try {
        if (borrarHistorial.value && terminados.value) await api.vaciarHistorial()
        if (borrarPuntos.value) for (const p of puntos.value) await api.eliminarPunto(p.nombre)
        avisar(t('datos.hecho'))
      } catch (e) {
        avisarError(e)
      } finally {
        borrando.value = false
        contar().catch(avisarError)
      }
    })
}

onMounted(async () => {
  contar().catch(avisarError)
  try {
    entradas.value = await api.configuracion()
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
})
</script>
