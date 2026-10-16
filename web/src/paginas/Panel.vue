<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.inicio')">
      <template #descripcion>
        <div class="row items-center q-gutter-x-sm">
          <span>{{ t('inicio.direccion') }}</span>
          <span class="direccion">{{ instancia?.base ?? '…' }}</span>
          <q-btn v-if="instancia?.base" flat round dense size="sm" icon="o_content_copy" :aria-label="t('inicio.copiarDireccion')" @click="copiar(instancia.base)">
            <q-tooltip>{{ t('inicio.copiarDireccion') }}</q-tooltip>
          </q-btn>
        </div>
      </template>
      <template #acciones>
        <q-btn outline color="primary" icon="o_hub" :label="t('menu.conectar')" no-caps @click="$emit('ir', 'conectar')" />
        <q-btn unelevated color="primary" icon="o_play_arrow" :label="t('menu.ejecutar')" no-caps @click="$emit('ir', 'ejecutar')" />
      </template>
    </CabeceraPagina>

    <q-banner v-if="error" rounded class="error-caja">{{ error }}</q-banner>

    <section class="q-card">
      <div class="seccion row items-end">
        <div class="col">
          <h2 class="titulo-seccion q-ma-none">{{ t('inicio.red') }}</h2>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('inicio.redTexto') }}</div>
        </div>
        <q-btn flat dense no-caps color="primary" :label="t('inicio.detalles')" @click="$emit('ir', 'agentes')" />
      </div>
      <div class="q-px-md q-pb-lg q-pt-sm">
        <MapaRed v-if="adaptadores && instancia" :adaptadores="adaptadores" :apis="puntos" :base="instancia.base" @ir="(p) => $emit('ir', p)" />
        <q-skeleton v-else type="rect" height="200px" />
      </div>
    </section>

    <section v-if="!guiaOculta && !pasos.every((p) => p.hecho)" class="q-card">
      <div class="seccion row items-center no-wrap">
        <div class="col">
          <h2 class="titulo-seccion q-ma-none">{{ t('inicio.primerosPasos') }}</h2>
          <div class="text-caption texto-suave q-mt-xs">{{ t('inicio.guiaProgreso', { hechos: pasos.filter((p) => p.hecho).length, total: pasos.length }) }}</div>
        </div>
        <q-btn flat dense no-caps icon="o_visibility_off" :label="t('inicio.ocultarGuia')" class="texto-suave" @click="ocultar" />
      </div>
      <ol class="ruta ruta--pasos q-ma-none q-px-md q-pt-sm q-pb-lg">
        <li v-for="p in pasos" :key="p.pagina + p.titulo" class="ruta__parada" :class="{ 'ruta__parada--pasada': p.hecho }">
          <button type="button" class="paso-estacion" @click="$emit('ir', p.pagina)">
            <span class="ruta__punto" aria-hidden="true" />
            <span class="paso-estacion__titulo">{{ p.titulo }}</span>
            <span class="text-caption texto-suave">{{ p.detalle }}</span>
          </button>
        </li>
      </ol>
    </section>

    <section>
      <div class="row items-end q-mb-sm">
        <div class="col">
          <h2 class="titulo-seccion q-ma-none">{{ t('inicio.peticiones') }}</h2>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('inicio.peticionesTexto') }}</div>
        </div>
      </div>
      <div class="tablero">
        <div class="tablero__cabecera"><span>{{ t('inicio.tablero') }}</span><span>{{ t('inicio.tableroPeriodo') }}</span></div>
        <button v-for="c in contadores" :key="c.titulo" type="button" class="tablero__fila" @click="verHistorial(c.filtro)">
          <span class="tablero__punto" :class="'tablero__punto--' + c.tipo" aria-hidden="true" />
          <span class="tablero__nombre">{{ c.titulo }}</span>
          <span class="tablero__detalle">{{ c.detalle }}</span>
          <span class="tablero__cifra">{{ c.valor }}</span>
          <q-icon name="chevron_right" size="20px" />
        </button>
      </div>
    </section>

    <section class="q-card">
      <div class="seccion row items-center">
        <h2 class="titulo-seccion q-ma-none col">{{ t('inicio.recientes') }}</h2>
        <q-btn flat dense no-caps color="primary" :label="t('inicio.verHistorial')" @click="$emit('ir', 'historial')" />
      </div>
      <q-separator />
      <q-list v-if="recientes.length" separator>
        <q-item v-for="trabajo in recientes" :key="trabajo.id" clickable class="q-py-sm" @click="verPeticion(trabajo.id)">
          <q-item-section style="max-width: 120px" class="gt-xs">
            <q-item-label caption class="tabular">{{ haceCuanto(trabajo.creado_en) }}</q-item-label>
          </q-item-section>
          <q-item-section>
            <q-item-label class="ellipsis">{{ textoDe(trabajo) }}</q-item-label>
            <q-item-label caption class="lt-sm">{{ haceCuanto(trabajo.creado_en) }}</q-item-label>
          </q-item-section>
          <q-item-section class="gt-sm" style="max-width: 300px">
            <Linea :agente="trabajo.solicitud.agente" :modelo="trabajo.modelo_efectivo || trabajo.modelo_enviado" class="text-body2" />
          </q-item-section>
          <q-item-section side>
            <span :class="estadoInfo(trabajo.estado).clase">{{ estadoInfo(trabajo.estado).etiqueta }}</span>
          </q-item-section>
        </q-item>
      </q-list>
      <EstadoVacio v-else :titulo="t('inicio.sinTrabajos')" />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import Linea from '../componentes/Linea.vue'
import MapaRed from '../componentes/MapaRed.vue'
import { api, avisar, ordenarAgentes, copiar, estadoInfo, filtroHistorial, guiaOculta, ocultarGuia, trabajoAbrir, haceCuanto, mensajeError, sesion, terminales, textoDe, type Adaptador, type EstadoInstancia, type EstadoPunto, type Trabajo, sistema } from '../api'
import { t } from '../i18n'

const emit = defineEmits<{ ir: [pagina: string] }>()

function ocultar() {
  ocultarGuia(true)
  avisar(t('inicio.guiaOculta'))
}

function verPeticion(id: string) {
  trabajoAbrir.value = id
  emit('ir', 'historial')
}

function verHistorial(filtro: string) {
  filtroHistorial.value = filtro
  emit('ir', 'historial')
}
const instancia = ref<EstadoInstancia>()
const recientes = ref<Trabajo[]>([])
const adaptadores = ref<Adaptador[]>()
const puntos = ref<EstadoPunto[]>([])
const error = ref('')

const total = (estados: string[]) => estados.reduce((n, e) => n + (instancia.value?.trabajos[e] ?? 0), 0)
const contadores = computed(() => [
  { titulo: t('inicio.completados'), detalle: t('inicio.completadosDetalle'), filtro: 'ok', valor: total(['completado']), tipo: 'ok' },
  { titulo: t('inicio.enCurso'), detalle: t('inicio.enCursoDetalle'), filtro: 'curso',
    valor: total(Object.keys(instancia.value?.trabajos ?? {}).filter((e) => !terminales.includes(e))), tipo: 'curso' },
  { titulo: t('inicio.conError'), detalle: t('inicio.conErrorDetalle'), filtro: 'error', valor: total(['fallido', 'interrumpido']), tipo: 'error' },
])

const pasos = computed(() => {
  const listos = adaptadores.value?.filter((a) => !a.problema).length ?? 0
  const lista = [
    { titulo: t('inicio.paso1'), detalle: adaptadores.value ? t('inicio.paso1Detalle', { listos, total: adaptadores.value.length }) : t('inicio.comprobando'),
      hecho: listos > 0, pagina: 'agentes' },
    { titulo: t('inicio.paso2'), detalle: t('inicio.paso2Detalle'), hecho: total(['completado']) > 0, pagina: 'ejecutar' },
    { titulo: t('inicio.paso3'), detalle: t('inicio.paso3Detalle'), hecho: puntos.value.length > 0, pagina: 'conectar' },
  ]
  if (sesion.value?.autoarranque !== undefined) {
    lista.push({ titulo: t('inicio.paso4', { sistema }), detalle: t('inicio.paso4Detalle'), hecho: sesion.value.autoarranque, pagina: 'configuracion' })
  }
  return lista
})

// consultando evita encadenar consultas si el servicio tarda más que el intervalo.
let consultando = false
async function cargar() {
  if (consultando) return
  consultando = true
  try {
    ;[instancia.value, recientes.value] = await Promise.all([api.instancia(), api.trabajos(7)])
    error.value = ''
  } catch (e) {
    error.value = mensajeError(e)
  } finally {
    consultando = false
  }
}

let intervalo: number
onMounted(() => {
  cargar()
  api.capacidades().then((c) => (adaptadores.value = ordenarAgentes(c.adaptadores))).catch(() => (adaptadores.value = []))
  api.puntos().then((p) => (puntos.value = p)).catch(() => {})
  intervalo = window.setInterval(() => !document.hidden && cargar(), 5000)
})
onUnmounted(() => clearInterval(intervalo))
</script>
