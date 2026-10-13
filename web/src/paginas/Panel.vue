<template>
  <div class="pagina apilado">
    <div class="row items-center q-col-gutter-md">
      <div class="col-12 col-md">
        <div class="titulo-pagina">{{ t('menu.inicio') }}</div>
        <div class="row items-center q-gutter-x-sm q-mt-xs texto-suave">
          <span>{{ t('inicio.direccion') }}</span>
          <span class="codigo direccion">{{ instancia?.base ?? '…' }}</span>
          <q-btn v-if="instancia?.base" flat round dense size="sm" icon="o_content_copy" :aria-label="t('inicio.copiarDireccion')" @click="copiar(instancia.base)">
            <q-tooltip>{{ t('inicio.copiarDireccion') }}</q-tooltip>
          </q-btn>
        </div>
      </div>
      <div class="col-12 col-md-auto row q-gutter-sm">
        <q-btn outline color="primary" icon="o_hub" :label="t('menu.conectar')" no-caps @click="$emit('ir', 'conectar')" />
        <q-btn unelevated color="primary" icon="o_play_arrow" :label="t('menu.ejecutar')" no-caps @click="$emit('ir', 'ejecutar')" />
      </div>
    </div>

    <q-banner v-if="error" rounded class="error-caja">{{ error }}</q-banner>

    <q-card v-if="!guiaOculta && !pasos.every((p) => p.hecho)">
      <q-card-section class="row items-center no-wrap q-pb-sm">
        <div class="icono-suave icono-suave--primary q-mr-md"><q-icon name="o_flag" size="22px" /></div>
        <div class="col">
          <div class="titulo-seccion">{{ t('inicio.primerosPasos') }}</div>
          <div class="text-caption texto-suave">{{ t('inicio.guiaProgreso', { hechos: pasos.filter((p) => p.hecho).length, total: pasos.length }) }}</div>
        </div>
        <q-btn flat dense no-caps icon="o_visibility_off" :label="t('inicio.ocultarGuia')" class="texto-suave" @click="ocultar" />
      </q-card-section>
      <div class="row q-col-gutter-sm q-px-md q-pb-md">
        <div v-for="p in pasos" :key="p.pagina + p.titulo" class="col-12 col-sm-6" :class="pasos.length > 3 ? 'col-lg-3' : 'col-lg-4'">
          <div class="paso" :class="{ 'paso--hecho': p.hecho }" role="button" tabindex="0" @click="$emit('ir', p.pagina)" @keyup.enter="$emit('ir', p.pagina)">
            <div class="row items-center no-wrap q-mb-xs">
              <q-icon :name="p.hecho ? 'check_circle' : 'radio_button_unchecked'" :color="p.hecho ? 'positive' : 'grey-6'" size="20px" class="q-mr-sm" />
              <span class="text-weight-medium">{{ p.titulo }}</span>
            </div>
            <div class="text-caption texto-suave">{{ p.detalle }}</div>
          </div>
        </div>
      </div>
    </q-card>

    <div>
      <div class="titulo-seccion">{{ t('inicio.peticiones') }}</div>
      <div class="texto-suave text-body2 q-mb-sm">{{ t('inicio.peticionesTexto') }}</div>
      <div class="row q-col-gutter-md">
        <div v-for="c in contadores" :key="c.titulo" class="col-12 col-sm-4">
          <q-card v-ripple class="q-pa-md full-height cursor-pointer tarjeta-clicable" @click="verHistorial(c.filtro)">
            <div class="row items-center no-wrap">
              <div class="icono-suave q-mr-md" :class="'icono-suave--' + c.color"><q-icon :name="c.icono" size="22px" /></div>
              <div class="col">
                <div class="text-weight-medium">{{ c.titulo }}</div>
                <div class="texto-suave text-caption">{{ c.detalle }}</div>
              </div>
              <div class="cifra q-ml-sm">{{ c.valor }}</div>
            </div>
          </q-card>
        </div>
      </div>
    </div>

    <div class="row q-col-gutter-md">
      <div class="col-12 col-md-7">
        <q-card class="full-height">
          <q-card-section class="row items-center">
            <div class="titulo-seccion col">{{ t('inicio.recientes') }}</div>
            <q-btn flat dense no-caps color="primary" :label="t('inicio.verHistorial')" @click="$emit('ir', 'historial')" />
          </q-card-section>
          <q-list v-if="recientes.length">
            <q-item v-for="trabajo in recientes" :key="trabajo.id" clickable @click="verPeticion(trabajo.id)">
              <q-item-section avatar>
                <q-icon :name="estadoInfo(trabajo.estado).icono" :color="estadoInfo(trabajo.estado).color" />
              </q-item-section>
              <q-item-section>
                <q-item-label class="ellipsis">{{ textoDe(trabajo) }}</q-item-label>
                <q-item-label caption>{{ estadoInfo(trabajo.estado).etiqueta }} · {{ modeloDe(trabajo) }}</q-item-label>
              </q-item-section>
              <q-item-section side class="text-caption">{{ haceCuanto(trabajo.creado_en) }}</q-item-section>
            </q-item>
          </q-list>
          <div v-else class="vacio">
            <q-icon name="o_inbox" size="40px" />
            <div class="q-mt-sm">{{ t('inicio.sinTrabajos') }}</div>
          </div>
        </q-card>
      </div>
      <div class="col-12 col-md-5">
        <q-card class="full-height">
          <q-card-section class="row items-center">
            <div class="titulo-seccion col">{{ t('menu.agentes') }}</div>
            <q-btn flat dense no-caps color="primary" :label="t('inicio.detalles')" @click="$emit('ir', 'agentes')" />
          </q-card-section>
          <q-list v-if="adaptadores">
            <q-item v-for="a in adaptadores" :key="a.agente">
              <q-item-section avatar><q-icon name="o_smart_toy" class="texto-suave" /></q-item-section>
              <q-item-section>{{ nombreAgente(a.agente) }}</q-item-section>
              <q-item-section side>
                <span class="etiqueta" :class="a.problema ? 'etiqueta--aviso' : 'etiqueta--ok'">
                  <q-icon :name="a.problema ? 'priority_high' : 'check'" size="14px" />{{ estadoAgente(a) }}
                </span>
              </q-item-section>
            </q-item>
          </q-list>
          <div v-else class="q-pa-md"><q-skeleton v-for="n in 4" :key="n" type="text" class="q-mb-sm" /></div>
        </q-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api, avisar, copiar, estadoInfo, filtroHistorial, guiaOculta, ocultarGuia, trabajoAbrir, haceCuanto, mensajeError, modeloDe, nombreAgente, sesion, terminales, textoDe, type Adaptador, type EstadoInstancia, type Trabajo } from '../api'
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
const puntos = ref(0)
const error = ref('')

const total = (estados: string[]) => estados.reduce((n, e) => n + (instancia.value?.trabajos[e] ?? 0), 0)
const contadores = computed(() => [
  { titulo: t('inicio.completados'), detalle: t('inicio.completadosDetalle'), filtro: 'ok', valor: total(['completado']), icono: 'o_check_circle', color: 'positive' },
  { titulo: t('inicio.enCurso'), detalle: t('inicio.enCursoDetalle'), filtro: 'curso',
    valor: total(Object.keys(instancia.value?.trabajos ?? {}).filter((e) => !terminales.includes(e))), icono: 'o_autorenew', color: 'primary' },
  { titulo: t('inicio.conError'), detalle: t('inicio.conErrorDetalle'), filtro: 'error', valor: total(['fallido', 'interrumpido']), icono: 'o_error', color: 'negative' },
])

// estadoAgente resume el problema que informa el servicio (en español) en una etiqueta traducida.
function estadoAgente(a: Adaptador) {
  if (!a.problema) return t('agente.listo')
  if (/no se encontró|envoltorio/.test(a.problema)) return t('agente.noInstalado')
  if (/falta/i.test(a.problema)) return t('agente.faltaClave')
  return t('agente.revisar')
}

const pasos = computed(() => {
  const listos = adaptadores.value?.filter((a) => !a.problema).length ?? 0
  const lista = [
    { titulo: t('inicio.paso1'), detalle: adaptadores.value ? t('inicio.paso1Detalle', { listos, total: adaptadores.value.length }) : t('inicio.comprobando'),
      hecho: listos > 0, accion: t('menu.agentes'), pagina: 'agentes' },
    { titulo: t('inicio.paso2'), detalle: t('inicio.paso2Detalle'), hecho: total(['completado']) > 0, accion: t('menu.ejecutar'), pagina: 'ejecutar' },
    { titulo: t('inicio.paso3'), detalle: t('inicio.paso3Detalle'), hecho: puntos.value > 0, accion: t('inicio.paso3Accion'), pagina: 'conectar' },
  ]
  if (sesion.value?.autoarranque !== undefined) {
    lista.push({ titulo: t('inicio.paso4'), detalle: t('inicio.paso4Detalle'), hecho: sesion.value.autoarranque,
      accion: t('inicio.paso4Accion'), pagina: 'configuracion' })
  }
  return lista
})

async function cargar() {
  try {
    ;[instancia.value, recientes.value] = await Promise.all([api.instancia(), api.trabajos(6)])
    error.value = ''
  } catch (e) {
    error.value = mensajeError(e)
  }
}

let intervalo: number
onMounted(() => {
  cargar()
  api.capacidades().then((c) => (adaptadores.value = c.adaptadores)).catch(() => (adaptadores.value = []))
  api.puntos().then((p) => (puntos.value = p.length)).catch(() => {})
  intervalo = window.setInterval(cargar, 5000)
})
onUnmounted(() => clearInterval(intervalo))
</script>
