<template>
  <div class="pagina apilado">
    <div class="row items-end q-col-gutter-md">
      <div class="col-12 col-sm">
        <div class="titulo-pagina">Configuración</div>
        <div class="texto-suave">Los valores que usa esta instancia y de dónde sale cada uno. Se cambian en los archivos de configuración.</div>
      </div>
      <div class="col-12 col-sm-4">
        <q-input v-model="filtro" dense outlined clearable debounce="150" placeholder="Buscar una clave o un valor">
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
    <div v-if="!cargando && !grupos.length" class="vacio">Ninguna clave coincide con la búsqueda.</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { api, avisarError, type EntradaConfiguracion } from '../api'

const entradas = ref<EntradaConfiguracion[]>([])
const filtro = ref<string | null>('')
const cargando = ref(true)

const origenes = [
  { id: 'interno', nombre: 'Interno', color: 'grey-6', detalle: 'Valor predeterminado de Tramtai' },
  { id: 'global', nombre: 'Global', color: 'primary', detalle: '%APPDATA%\\tramtai\\config.yaml' },
  { id: 'proyecto', nombre: 'Proyecto', color: 'teal-8', detalle: '.tramtai/config.yaml (se versiona)' },
  { id: 'local', nombre: 'Local', color: 'indigo-6', detalle: '.tramtai/local.yaml (solo este equipo)' },
  { id: 'entorno', nombre: 'Entorno', color: 'deep-orange-7', detalle: 'Variables TRAMTAI_SECCION__CLAVE' },
]
const origen = (id: string) => origenes.find((o) => o.id === id) ?? { nombre: id, color: 'grey-6' }

const titulos: Record<string, string> = {
  servicio: 'Servicio', predeterminados: 'Valores predeterminados', modelos: 'Modelos', adaptadores: 'Agentes', comandos_agentes: 'Agentes',
  concurrencia: 'Concurrencia', concurrencia_servicio: 'Concurrencia', retencion: 'Retención', conversion: 'Archivos', compatibilidad: 'Compatibilidad',
}

const grupos = computed(() => {
  const texto = (filtro.value ?? '').toLowerCase().trim()
  const mapa = new Map<string, EntradaConfiguracion[]>()
  for (const e of entradas.value) {
    if (texto && !e.clave.toLowerCase().includes(texto) && !valor(e.valor).toLowerCase().includes(texto)) continue
    const titulo = titulos[e.clave.split('.')[0]] ?? 'Otros'
    mapa.set(titulo, [...(mapa.get(titulo) ?? []), e])
  }
  return [...mapa.entries()].map(([titulo, lista]) => ({ titulo, entradas: lista }))
})

function valor(v: unknown): string {
  if (v === true) return 'sí'
  if (v === false) return 'no'
  return typeof v === 'object' ? JSON.stringify(v) : String(v)
}

onMounted(async () => {
  try {
    entradas.value = await api.configuracion()
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
})
</script>
