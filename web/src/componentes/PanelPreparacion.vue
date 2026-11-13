<template>
  <!-- Instala, actualiza, desinstala o inicia sesión en los CLI de los agentes, uno tras otro, y enseña cómo va. Sin acción,
       sigue la preparación que ya está en marcha (la lanzó otra pantalla, o antes de recargar). -->
  <div class="panel-preparacion column q-gutter-y-md">
    <div v-if="cuenta" class="text-caption texto-suave"><q-icon name="o_dns" class="q-mr-xs" aria-hidden="true" />{{ t('preparar.cuentaServicio', { cuenta }) }}</div>
    <div v-for="agente in visibles" :key="agente" class="column q-gutter-y-sm">
      <!-- El estado, en voz alta para un lector de pantalla cada vez que cambia. -->
      <div role="status" aria-live="polite" class="row items-center no-wrap q-gutter-x-sm">
        <q-spinner v-if="enMarcha(estados[agente]) || (agente === actual && !estados[agente] && !fallos[agente])" size="18px" color="primary" aria-hidden="true" />
        <q-icon v-else :name="icono(agente)" :color="color(agente)" size="20px" aria-hidden="true" />
        <span class="text-weight-medium">{{ titular(agente) }}</span>
      </div>

      <!-- Iniciar sesión: el enlace (se abre solo al pulsar «Abrir»), el código para copiar y, si el CLI lo pide, el campo para pegar. -->
      <template v-if="vistaEspera(estados[agente])">
        <div v-if="AVISOS_SESION[agente]" class="aviso text-body2">
          {{ t(AVISOS_SESION[agente].texto) }}
          <a class="enlace" :href="AVISOS_SESION[agente].enlace" target="_blank" rel="noopener noreferrer">{{ t('preparar.abrirAjustesSeguridad') }}</a>
        </div>
        <div v-if="vistaEspera(estados[agente])!.enlace" class="row items-center no-wrap q-gutter-x-sm">
          <span class="codigo romper col">{{ vistaEspera(estados[agente])!.enlace }}</span>
          <q-btn unelevated dense no-caps color="primary" icon-right="o_open_in_new" class="q-px-md" :label="t('preparar.abrir')"
            :aria-label="t('preparar.abrirEnlace', { agente: nombreAgente(agente) })"
            :href="vistaEspera(estados[agente])!.enlace" target="_blank" rel="noopener noreferrer" />
        </div>
        <div v-if="vistaEspera(estados[agente])!.codigo">
          <div class="text-caption texto-suave q-mb-xs">{{ t('preparar.codigo') }}</div>
          <div class="bloque-codigo codigo-dispositivo">{{ vistaEspera(estados[agente])!.codigo }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
            :aria-label="t('preparar.copiarCodigo')" @click="copiar(vistaEspera(estados[agente])!.codigo)" /></div>
        </div>
        <form v-if="vistaEspera(estados[agente])!.pegar" class="row items-start q-gutter-sm" @submit.prevent="enviarCodigo(agente)">
          <q-input v-model="pegado" dense outlined type="password" autocomplete="off" class="col" :label="t('preparar.pegar')"
            :input-attrs="{ autocomplete: 'off', spellcheck: 'false' }" />
          <q-btn type="submit" unelevated no-caps color="primary" :label="t('preparar.enviar')" :disable="!pegado.trim()" :loading="enviando" />
        </form>
      </template>

      <!-- Falló: qué hacer, el detalle del servicio y, si la versión no estaba comprobada, «Seguir» (nunca al iniciar sesión). -->
      <template v-if="estados[agente]?.estado === 'error'">
        <div class="text-body2">{{ explicacion(estados[agente]!) }}</div>
        <div v-if="estados[agente]!.mensaje" class="text-caption texto-suave romper">{{ estados[agente]!.mensaje }}</div>
      </template>
      <div v-if="fallos[agente]" class="text-body2" role="alert">{{ fallos[agente] }}</div>

      <!-- Las últimas líneas de la salida, tal cual y con los enlaces resaltados (sin enlaces que se abran solos). -->
      <div v-if="estados[agente]?.lineas?.length && (estados[agente]!.estado === 'error' || enMarcha(estados[agente]))"
        class="bloque-codigo sin-ajuste text-caption registro-preparacion" :aria-label="t('preparar.registro')" tabindex="0">
        <div v-for="(linea, i) in ultimas(estados[agente]!)" :key="i"><template v-for="(trozo, j) in trozos(linea)" :key="j"><mark v-if="j % 2">{{ trozo }}</mark><template v-else>{{ trozo }}</template></template></div>
      </div>
    </div>
    <!-- Las acciones son del agente en curso: solo uno se prepara a la vez. -->
    <div v-if="pideSeguir(estados[actual]) || enMarcha(estados[actual]) || (terminado && visibles.length)" class="row items-center q-gutter-sm">
      <q-btn v-if="pideSeguir(estados[actual])" unelevated no-caps color="primary" :label="t('preparar.seguir')" @click="seguir(actual)" />
      <q-btn v-if="enMarcha(estados[actual])" flat no-caps color="neutro" icon="o_close" :label="t('comun.cancelar')" :loading="cancelando" @click="cancelar(actual)" />
      <q-btn v-if="terminado && visibles.length" flat no-caps color="primary" :label="t('comun.cerrar')" @click="emit('cerrar')" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { api, avisarError, copiar, ErrorApi, mensajeError } from '../api/cliente'
import type { EstadoPreparacion } from '../api/tipos'
import { nombreAgente } from '../agentes'
import { sesion } from '../estado'
import { esServidor } from '../servidor'
import { AVISOS_SESION, enMarcha, pideSeguir, seguirPreparacion, trozos, vistaEspera, type Accion } from '../preparacion'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const props = defineProps<{ agentes: string[]; accion?: Accion; borrarDatos?: boolean; confirmar?: boolean }>()
// fin: la preparación de un agente terminó (como sea); listo: todas terminaron bien; cerrar: el usuario cierra el panel.
const emit = defineEmits<{ fin: [agente: string, estado: EstadoPreparacion | null]; listo: []; cerrar: [] }>()

const estados = reactive<Record<string, EstadoPreparacion | null>>({})
const fallos = reactive<Record<string, string>>({})
const actual = ref('')
const terminado = ref(false)
const pegado = ref('')
const enviando = ref(false)
const cancelando = ref(false)
// cuenta: en el servidor se instala y se inicia sesión para la cuenta del servicio, no para quien mira.
const cuenta = ref('')
const visibles = computed(() => props.agentes.filter((a) => a === actual.value || estados[a] || fallos[a]))

let vivo = true
onUnmounted(() => (vivo = false))
// esperar separa las consultas y corta el sondeo si el panel ya no está.
const esperar = () => new Promise<void>((listo, fallo) => setTimeout(() => (vivo ? listo() : fallo(new Error('cerrado'))), 1500))

// correr lanza la acción (o, sin ella, mira la que haya) y la sigue hasta que termina. Con otra en marcha para el mismo
// agente (la app y la Web UI a la vez), sigue esa.
async function correr(agente: string, accion: Accion | undefined, confirmar: boolean): Promise<EstadoPreparacion | null> {
  actual.value = agente
  delete fallos[agente]
  let primera: Promise<EstadoPreparacion | null> | undefined = accion
    ? api.preparar(agente, { accion, borrar_datos: !!props.borrarDatos, confirmar }).catch((e) => {
      if (e instanceof ErrorApi && e.codigo === 'ya_en_curso' && e.datos.preparacion) return e.datos.preparacion as EstadoPreparacion
      throw e
    })
    : api.preparacion(agente)
  try {
    return await seguirPreparacion(() => {
      const p = primera ?? api.preparacion(agente)
      primera = undefined
      return p
    }, (e) => (estados[agente] = e), esperar)
  } catch (e) {
    if (vivo) fallos[agente] = mensajeError(e)
    return null
  }
}

// empezar recorre los agentes desde uno (el primero, con su acción y confirmar); se para en el primero que no termina
// bien.
async function empezar(desde: number, accion: Accion | undefined, confirmar: boolean) {
  terminado.value = false
  for (let i = desde; i < props.agentes.length; i++) {
    const agente = props.agentes[i]
    const e = i === desde ? await correr(agente, accion, confirmar) : await correr(agente, props.accion, !!props.confirmar)
    if (!vivo) return
    emit('fin', agente, e)
    if (e?.estado !== 'listo') {
      terminado.value = !enMarcha(e)
      return
    }
  }
  terminado.value = true
  emit('listo')
}

// seguir repite la acción que falló con la versión sin comprobar, después del aviso (la de su estado: la página puede
// haber pasado el panel a solo mirar).
const seguir = (agente: string) => empezar(props.agentes.indexOf(agente), estados[agente]?.accion, true)

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

// enviarCodigo pasa al CLI lo pegado y vacía el campo: no se queda en la página.
async function enviarCodigo(agente: string) {
  const codigo = pegado.value.trim()
  if (!codigo) return
  enviando.value = true
  try {
    await api.codigoPreparacion(agente, codigo)
    pegado.value = ''
  } catch (e) {
    avisarError(e)
  } finally {
    enviando.value = false
  }
}

const ultimas = (e: EstadoPreparacion) => e.lineas ?? []
// k da por buena una clave armada con un estado o un código del contrato.
const k = (clave: string) => clave as Clave

function titular(agente: string): string {
  const e = estados[agente]
  const nombre = nombreAgente(agente)
  if (!e) return fallos[agente] ? t(k(`preparar.fallo.${props.accion ?? 'instalar'}`), { agente: nombre }) : t('preparar.empezando', { agente: nombre })
  switch (e.estado) {
    case 'esperando_usuario':
      return t(e.pide_pegar ? 'preparar.esperaPegar' : 'preparar.esperaCodigo', { agente: nombre })
    case 'listo':
      return t(k(`preparar.listo.${e.accion}`), { agente: nombre, antes: e.version_antes ?? '', despues: e.version_despues ?? '' })
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
  const clave = `preparar.error.${e.motivo_codigo}` as Clave
  return e.motivo_codigo && clave in textosConocidos ? t(clave) : ''
}
// textosConocidos: los códigos con explicación (uno nuevo del servicio se queda con su detalle).
const textosConocidos = Object.fromEntries(['instalador_fallo', 'no_encontrado_tras_instalar', 'plazo_agotado', 'salida_no_reconocida',
  'inicio_sesion_fallo', 'sin_red', 'no_disponible_en_servidor', 'interrumpida'].map((c) => [`preparar.error.${c}`, true]))

const icono = (agente: string) => ({ listo: 'o_check_circle', cancelado: 'o_block' } as Record<string, string>)[estados[agente]?.estado ?? ''] ?? 'o_error'
const color = (agente: string) => ({ listo: 'positive', cancelado: 'grey-7' } as Record<string, string>)[estados[agente]?.estado ?? ''] ?? (pideSeguir(estados[agente]) ? 'warning' : 'negative')

onMounted(() => {
  if (esServidor(sesion.value)) api.acerca().then((a) => (cuenta.value = a.cuenta ?? '')).catch(() => {})
  void empezar(0, props.accion, !!props.confirmar)
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
