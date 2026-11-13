<template>
  <!-- Instala, actualiza, desinstala o inicia sesión en los CLI de los agentes, uno tras otro, y enseña cómo va. Sin acción,
       sigue la preparación que ya está en marcha (la lanzó otra pantalla, o antes de recargar). -->
  <!-- column de Quasar parte en líneas (flex-wrap): sin no-wrap, una línea larga del registro (que no se parte) daba su
       ancho a todo el panel y desbordaba la pantalla en un móvil. -->
  <div class="panel-preparacion column no-wrap q-gutter-y-md">
    <div v-if="cuenta" class="text-caption texto-suave"><q-icon name="o_dns" class="q-mr-xs" aria-hidden="true" />{{ t('preparar.cuentaServicio', { cuenta }) }}</div>
    <div v-for="agente in visibles" :key="agente" class="column no-wrap q-gutter-y-sm">
      <!-- El estado, en voz alta para un lector de pantalla cada vez que cambia; recibe el foco cuando el panel sustituye
           al botón que lo lanzó. -->
      <div :ref="(el) => (cajasEstado[agente] = el as HTMLElement | null)" role="status" aria-live="polite" tabindex="-1"
        class="row items-center no-wrap q-gutter-x-sm">
        <q-spinner v-if="girando(agente)" size="18px" color="primary" aria-hidden="true" />
        <q-icon v-else :name="icono(agente)" :color="color(agente)" size="20px" aria-hidden="true" />
        <span class="text-weight-medium">{{ titular(agente) }}</span>
      </div>

      <!-- Iniciar sesión: el enlace (se abre solo al pulsar «Abrir»), el código para copiar y, si el CLI lo pide, el campo para pegar. -->
      <template v-if="vistaEspera(estados[agente])">
        <div v-if="AVISOS_SESION[agente]" class="aviso text-body2">
          {{ t(AVISOS_SESION[agente].texto) }}
          <a class="enlace" :href="AVISOS_SESION[agente].enlace" target="_blank" rel="noopener noreferrer">{{ t('preparar.abrirAjustesSeguridad') }}</a>
        </div>
        <!-- En un móvil, el enlace a todo el ancho y «Abrir» debajo: al lado quedaba una columna estrecha de letras sueltas. -->
        <div v-if="vistaEspera(estados[agente])!.enlace" class="row items-center q-col-gutter-sm">
          <div class="col-12 col-sm"><span class="codigo romper">{{ vistaEspera(estados[agente])!.enlace }}</span></div>
          <div class="col-auto"><q-btn unelevated dense no-caps color="primary" icon-right="o_open_in_new" class="q-px-md" :label="t('preparar.abrir')" :aria-label="t('preparar.abrirEnlace', { agente: nombreAgente(agente) })" :href="vistaEspera(estados[agente])!.enlace" target="_blank" rel="noopener noreferrer" /></div>
        </div>
        <div v-if="vistaEspera(estados[agente])!.codigo">
          <div class="text-caption texto-suave q-mb-xs">{{ t('preparar.codigo') }}</div>
          <div class="bloque-codigo codigo-dispositivo">{{ vistaEspera(estados[agente])!.codigo }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
            :aria-label="t('preparar.copiarCodigo')" @click="copiar(vistaEspera(estados[agente])!.codigo)" /></div>
        </div>
        <!-- Lo pegado es un código de un solo uso: texto (los gestores de contraseñas no lo ofrecen guardar), sin corrector,
             y se vacía al enviarlo o cuando el CLI deja de esperarlo. -->
        <form v-if="vistaEspera(estados[agente])!.pegar" class="row items-start q-gutter-sm" autocomplete="off" @submit.prevent="enviarCodigo(agente)">
          <q-input v-model="pegado" dense outlined type="text" class="col" :label="t('preparar.pegar')" autocomplete="one-time-code" spellcheck="false" autocapitalize="off" />
          <q-btn type="submit" unelevated no-caps color="primary" :label="t('preparar.enviar')" :disable="!pegado.trim()" :loading="enviando" />
        </form>
      </template>

      <!-- Falló: qué hacer y el detalle del servicio. Un listo con motivo (instalado sin iniciar sesión) dice por qué. -->
      <template v-if="estados[agente]?.estado === 'error'">
        <div class="text-body2">{{ explicacion(estados[agente]!) }}</div>
        <div v-if="estados[agente]!.mensaje" class="text-caption texto-suave romper">{{ estados[agente]!.mensaje }}</div>
      </template>
      <div v-else-if="estados[agente]?.estado === 'listo' && estados[agente]!.mensaje" class="aviso text-body2 romper">{{ estados[agente]!.mensaje }}</div>
      <div v-if="fallos[agente]" class="text-body2" role="alert">{{ fallos[agente] }}</div>

      <!-- Inició sesión, pero Tramtai aún lo usa con clave de API y no hay ninguna: «Usar mi suscripción» ahí mismo, con su
           aviso de riesgo; donde no hay botón (el servidor, otro equipo), la orden. -->
      <template v-if="falta(agente)">
        <div class="text-body2">{{ t('preparar.faltaSuscripcionTexto') }}</div>
        <div v-if="puedeElegirSuscripcion(true, sesion, estadoServidor)">
          <q-btn unelevated no-caps color="primary" icon="o_person" :label="t('agentes.usarSuscripcion')" :loading="activando === agente" @click="activar(agente)" />
        </div>
        <template v-else>
          <div class="text-body2">{{ t('preparar.suscripcionConOrden') }}</div>
          <div class="bloque-codigo">{{ ordenUsarSuscripcion(agente, sesion, estadoServidor) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
            :aria-label="t('agentes.copiarComando')" @click="copiar(ordenUsarSuscripcion(agente, sesion, estadoServidor))" /></div>
        </template>
      </template>

      <!-- Las últimas líneas de la salida, tal cual y con los enlaces resaltados (sin enlaces que se abran solos). -->
      <div v-if="estados[agente]?.lineas?.length && (estados[agente]!.estado === 'error' || enMarcha(estados[agente]))"
        class="bloque-codigo sin-ajuste text-caption registro-preparacion" role="log" :aria-label="t('preparar.registro')" tabindex="0">
        <div v-for="(linea, i) in estados[agente]!.lineas" :key="i"><template v-for="(trozo, j) in trozos(linea)" :key="j"><mark v-if="j % 2">{{ trozo }}</mark><template v-else>{{ trozo }}</template></template></div>
      </div>
    </div>
    <!-- Con varios agentes, el que no acabó en listo se puede reintentar o saltar, y se enseña lo que falta. -->
    <div v-if="parada && parada.pendientes.length" class="text-body2">{{ t('preparar.pendientes', { agentes: parada.pendientes.map(nombreAgente).join(', ') }) }}</div>
    <!-- Las acciones son del agente en curso: solo uno se prepara a la vez. -->
    <div v-if="pideSeguir(estados[actual]) || enMarcha(estados[actual]) || (terminado && visibles.length)" class="row items-center q-gutter-sm">
      <q-btn v-if="pideSeguir(estados[actual])" unelevated no-caps color="primary" :label="t('preparar.seguir')" :disable="corriendo" @click="seguir(actual)" />
      <q-btn v-if="parada?.reintentar" :unelevated="!pideSeguir(estados[actual])" :flat="pideSeguir(estados[actual])" no-caps color="primary" icon="o_refresh" :label="t('comun.reintentar')" :disable="corriendo" @click="reintentar" />
      <q-btn v-if="parada?.saltar" flat no-caps color="primary" :label="t('preparar.saltar')" :disable="corriendo" @click="saltar" />
      <q-btn v-if="enMarcha(estados[actual])" flat no-caps color="neutro" icon="o_close" :label="t('comun.cancelar')" :loading="cancelando" @click="cancelar(actual)" />
      <q-btn v-if="terminado && visibles.length" flat no-caps color="primary" :label="t('comun.cerrar')" @click="emit('cerrar')" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { api, avisarError, copiar, mensajeError } from '../api/cliente'
import type { EstadoPreparacion } from '../api/tipos'
import { adaptadoresConocidos, nombreAgente, refrescarAgentes } from '../agentes'
import { sesion } from '../estado'
import { sinDato } from '../formato'
import { esServidor, estadoServidor, ordenUsarSuscripcion, puedeElegirSuscripcion } from '../servidor'
import { usarSuscripcion } from '../suscripcion'
import { AVISOS_SESION, CODIGOS_EXPLICADOS, conSesion, enMarcha, faltaSuscripcion, opcionesParada, ordenSeguir, siguienteTrasSaltar, pideSeguir, prepararAgentes, trozos, vistaEspera, type Accion, type Corrida, type Orden } from '../preparacion'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const props = defineProps<{ agentes: string[]; accion?: Accion; borrarDatos?: boolean; confirmar?: boolean; tanda?: boolean }>()
// tanda (prop): viene de una tanda (la bienvenida), donde un solo agente también se reintenta.
// lanzada: el servicio aceptó la acción del agente (quien muestra el panel ya no debe repetirla si lo vuelve a montar);
// fin: la de un agente terminó (como sea; si no se pudo ni lanzar, con el motivo); listo: acabó la tanda (todos en listo, o los que quedaron saltados), con el estado
// de cada uno (un listo con motivo no está listo para usar, ver listoParaUsar); cerrar: el usuario cierra el panel.
const emit = defineEmits<{ lanzada: [agente: string]; fin: [agente: string, estado: EstadoPreparacion | null, fallo?: string]; listo: [estados: Record<string, EstadoPreparacion | null>]; cerrar: [] }>()

const estados = reactive<Record<string, EstadoPreparacion | null>>({})
const fallos = reactive<Record<string, string>>({})
// acciones: la que se pidió a cada agente, para el titular de un fallo antes de tener su estado.
const acciones = reactive<Record<string, Accion | undefined>>({})
const cajasEstado: Record<string, HTMLElement | null> = {}
const actual = ref('')
const terminado = ref(false)
// paradaEn: el índice del agente donde se paró la tanda (no acabó en listo), o -1.
const paradaEn = ref(-1)
const parada = computed(() => {
  if (!terminado.value || paradaEn.value < 0) return null
  const o = opcionesParada(props.agentes, paradaEn.value, props.tanda)
  return { ...o, reintentar: o.reintentar && !!props.accion }
})
const corriendo = ref(false)
const pegado = ref('')
const enviando = ref(false)
const cancelando = ref(false)
// cuenta: en el servidor se instala y se inicia sesión para la cuenta del servicio, no para quien mira.
const cuenta = ref('')
const visibles = computed(() => props.agentes.filter((a) => a === actual.value || estados[a] || fallos[a]))

let vivo = true
onUnmounted(() => (vivo = false))

const corrida: Corrida = {
  api: { estado: api.preparacion, preparar: api.preparar },
  esperar: () => new Promise((listo) => setTimeout(listo, 1500)),
  vivo: () => vivo,
  empieza: (agente, orden) => {
    actual.value = agente
    acciones[agente] = orden.accion ?? estados[agente]?.accion
    delete fallos[agente]
  },
  lanzada: (agente) => emit('lanzada', agente),
  cambio: (agente, e) => (estados[agente] = e),
  fallo: (agente, e) => (fallos[agente] = mensajeError(e)),
  fin: (agente, e) => {
    if (e?.estado === 'listo' && !e.motivo_codigo && conSesion(agente) && (e.accion === 'instalar' || e.accion === 'iniciar_sesion')) {
      comprobaciones = Promise.all([comprobaciones, comprobarModo(agente)]).then(() => {})
    }
    emit('fin', agente, e, fallos[agente])
  },
}

// Tras iniciar sesión se vuelven a comprobar los agentes: hasta saber si falta usar la suscripción (Ruling 34), el panel
// dice «Comprobando» y la tanda no se da por acabada.
const comprobando = reactive<Record<string, boolean>>({})
let comprobaciones = Promise.resolve()
async function comprobarModo(agente: string) {
  comprobando[agente] = true
  await refrescarAgentes()
  comprobando[agente] = false
}
const adaptador = (agente: string) => adaptadoresConocidos.value?.find((a) => a.agente === agente)
const falta = (agente: string) => !comprobando[agente] && faltaSuscripcion(estados[agente], adaptador(agente))
const activando = ref('')
async function activar(agente: string) {
  activando.value = agente
  try {
    await usarSuscripcion(agente)
  } finally {
    activando.value = ''
  }
  // El botón desaparece al activarla: el foco pasa al estado, que dice cómo quedó, en lugar de perderse.
  await nextTick()
  if (!falta(agente)) cajasEstado[agente]?.focus()
}

// correr recorre los agentes desde uno.
async function correr(desde: number, primera: Orden) {
  terminado.value = false
  paradaEn.value = -1
  corriendo.value = true
  try {
    const resultado = await prepararAgentes(corrida, props.agentes, desde, primera, ordenInicial())
    if (resultado === 'cortado') return
    await comprobaciones
    if (!vivo) return
    terminado.value = true
    if (resultado === 'listo') avisarListo()
    else paradaEn.value = props.agentes.indexOf(actual.value)
  } finally {
    corriendo.value = false
  }
}
const ordenInicial = (): Orden => ({ accion: props.accion, borrar_datos: props.borrarDatos, confirmar: props.confirmar })
const avisarListo = () => emit('listo', Object.fromEntries(props.agentes.map((a) => [a, estados[a] ?? null])))

// reintentar vuelve a pedir el agente donde se paró; saltar sigue con el siguiente (o termina, si era el último).
function reintentar() {
  void correr(paradaEn.value, ordenInicial())
  void enfocarEstado()
}
function saltar() {
  const siguiente = siguienteTrasSaltar(paradaEn.value, props.agentes.length)
  if (siguiente !== 'fin') return void correr(siguiente, ordenInicial())
  paradaEn.value = -1
  avisarListo()
}

// enfocarEstado lleva el foco al estado cuando el panel sustituye al botón que lo lanzó (la acción de la tarjeta o «Seguir»).
async function enfocarEstado() {
  await nextTick()
  cajasEstado[actual.value]?.focus()
}

// seguir repite, con confirmar, la acción que se paró por una versión sin comprobar.
function seguir(agente: string) {
  const e = estados[agente]
  if (!e || corriendo.value) return
  void correr(props.agentes.indexOf(agente), ordenSeguir(e, props.borrarDatos))
  void enfocarEstado()
}

async function cancelar(agente: string) {
  cancelando.value = true
  try {
    await api.cancelarPreparacion(agente)
  } catch (e) {
    avisarError(e)
  } finally {
    cancelando.value = false
  }
}

// enviarCodigo pasa al CLI lo pegado; el campo se vacía siempre: no se queda en la página.
async function enviarCodigo(agente: string) {
  const codigo = pegado.value.trim()
  if (!codigo) return
  enviando.value = true
  try {
    await api.codigoPreparacion(agente, codigo)
  } catch (e) {
    avisarError(e)
  } finally {
    pegado.value = ''
    enviando.value = false
  }
}
watch(() => estados[actual.value]?.estado, (estado) => estado !== 'esperando_usuario' && (pegado.value = ''))

// k da por buena una clave armada con un estado o un código del contrato.
const k = (clave: string) => clave as Clave
const girando = (agente: string) => enMarcha(estados[agente]) || !!comprobando[agente] || (agente === actual.value && corriendo.value && !estados[agente] && !fallos[agente])

function titular(agente: string): string {
  const e = estados[agente]
  const nombre = nombreAgente(agente)
  const accion = e?.accion ?? acciones[agente]
  if (!e) return fallos[agente] && accion ? t(k(`preparar.fallo.${accion}`), { agente: nombre }) : t('preparar.empezando', { agente: nombre })
  switch (e.estado) {
    case 'esperando_usuario':
      return t(e.pide_pegar ? 'preparar.esperaPegar' : 'preparar.esperaCodigo', { agente: nombre })
    case 'listo': {
      if (comprobando[agente]) return t('preparar.estado.comprobando', { agente: nombre })
      if (falta(agente)) return t('preparar.faltaSuscripcion', { agente: nombre })
      const versiones = { agente: nombre, antes: e.version_antes || sinDato(), despues: e.version_despues || sinDato() }
      return e.motivo_codigo && e.accion === 'instalar' ? t('preparar.listoSinSesion', versiones) : t(k(`preparar.listo.${e.accion}`), versiones)
    }
    case 'cancelado':
      return t('preparar.cancelado', { agente: nombre })
    case 'error':
      return e.motivo_codigo === 'version_no_comprobada' ? t('preparar.noComprobadaTitulo', { agente: nombre }) : t(k(`preparar.fallo.${e.accion}`), { agente: nombre })
    default:
      return t(k(`preparar.estado.${e.estado}`), { agente: nombre }) + (e.etapa ? ' · ' + t(k(`preparar.etapa.${e.etapa}`)) : '')
  }
}

// explicacion dice qué pasó y qué hacer, en el idioma de la Web UI (el detalle del servicio va debajo).
function explicacion(e: EstadoPreparacion): string {
  if (e.motivo_codigo === 'version_no_comprobada') return t(k(`preparar.noComprobada.${e.accion}`), { agente: nombreAgente(e.agente) })
  return e.motivo_codigo && CODIGOS_EXPLICADOS.includes(e.motivo_codigo) ? t(k(`preparar.error.${e.motivo_codigo}`)) : ''
}

function icono(agente: string) {
  const e = estados[agente]
  if (e?.estado === 'listo') return e.motivo_codigo || falta(agente) ? 'o_warning' : 'o_check_circle'
  return e?.estado === 'cancelado' ? 'o_block' : pideSeguir(e) ? 'o_warning' : 'o_error'
}
function color(agente: string) {
  const e = estados[agente]
  if (e?.estado === 'listo') return e.motivo_codigo || falta(agente) ? 'warning' : 'positive'
  return e?.estado === 'cancelado' ? 'neutro' : pideSeguir(e) ? 'warning' : 'negative'
}

onMounted(() => {
  if (esServidor(sesion.value)) api.acerca().then((a) => (cuenta.value = a.cuenta ?? '')).catch(() => {})
  void correr(0, ordenInicial())
  if (props.accion) void enfocarEstado()
})
</script>

<style scoped>
/* El código de dispositivo se copia a mano en otra pestaña: grande y espaciado para leerlo sin errores. */
.codigo-dispositivo {
  font-size: 1.35rem;
  letter-spacing: 0.08em;
}
.registro-preparacion {
  max-height: 14rem;
  overflow-y: auto;
  padding-right: 14px;
}
.registro-preparacion mark {
  background: var(--aviso-suave);
  color: inherit;
}

</style>
