<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.agentes')" :descripcion="t('agentes.subtitulo')">
      <template #acciones>
        <q-btn flat no-caps color="primary" icon="o_refresh" :label="t('agentes.recomprobar')" :loading="cargando" @click="cargar" />
      </template>
    </CabeceraPagina>

    <div v-if="adaptadores" class="row items-start q-col-gutter-md">
      <div v-for="a in adaptadores" :key="a.agente" class="col-12 col-lg-6">
        <q-card class="column">
          <q-card-section class="row items-center no-wrap">
            <span class="linea__chapa linea__chapa--grande q-mr-md" :class="['linea--' + a.agente, { 'linea__chapa--apagada': a.problema }]" aria-hidden="true">{{ codigoLinea(a.agente) }}</span>
            <div class="col">
              <div class="titulo-seccion">{{ nombreAgente(a.agente) }}</div>
              <div class="text-caption texto-suave">{{ a.version_detectada || t('agentes.sinVersion') }}</div>
            </div>
            <span class="etiqueta" :class="a.problema ? 'etiqueta--aviso' : 'etiqueta--ok'">
              <q-icon :name="a.problema ? 'priority_high' : 'check'" size="14px" />{{ a.problema ? t(tipo(a)) : t('agente.listo') }}
            </span>
          </q-card-section>

          <q-card-section class="q-pt-none col">
            <!-- Listo: cómo está conectado y, en Claude y Codex, la otra forma -->
            <template v-if="!a.problema">
              <div class="metadatos">
                <span><q-icon :name="suscripcion(a) ? 'o_person' : 'o_key'" />{{ suscripcion(a) ? t('agentes.listoSuscripcion') : t('agentes.listoClave') }}</span>
                <span v-if="esfuerzos[a.agente]?.length"><q-icon name="o_psychology" />{{ t('agentes.esfuerzo') }}: {{ esfuerzos[a.agente].map(nombreEsfuerzo).join(' · ') }}</span>
                <span v-if="contextos[a.agente]?.length"><q-icon name="o_memory" />{{ t('agentes.contexto') }}: {{ textoContextos(a.agente) }}</span>
              </div>
              <q-btn unelevated dense no-caps color="primary" icon="o_chat" class="q-mt-md q-mr-sm q-px-md" :label="t('agentes.probar')" @click="probar(a)" />
              <q-btn v-if="puedeCambiar(a)" flat dense no-caps color="primary" class="q-mt-md q-px-sm" :loading="cambiando === a.agente"
                :label="suscripcion(a) ? t('agentes.cambiarAClave') : t('agentes.cambiarASuscripcion')"
                @click="suscripcion(a) ? cambiar(a, 'clave_api') : confirmarSuscripcion(a)" />
            </template>

            <!-- Falta conectarlo: las dos opciones, lado a lado -->
            <template v-else-if="tipo(a) === 'agente.faltaClave'">
              <template v-if="puedeCambiar(a)">
                <div class="text-weight-medium q-mb-sm">{{ t('agentes.elegir') }}</div>
                <div class="row q-col-gutter-sm">
                  <div class="col-12 col-sm-6">
                    <div class="opcion full-height column">
                      <div class="text-weight-medium"><q-icon name="o_person" class="q-mr-xs" />{{ t('agentes.opcionSuscripcion') }}</div>
                      <div class="text-caption texto-suave q-mt-xs col">{{ t('agentes.opcionSuscripcionTexto', { cli: cli(a) }) }}</div>
                      <q-btn unelevated no-caps color="primary" class="q-mt-sm" :label="t('agentes.usarSuscripcion')" :loading="cambiando === a.agente"
                        @click="confirmarSuscripcion(a)" />
                    </div>
                  </div>
                  <div class="col-12 col-sm-6">
                    <div class="opcion full-height">
                      <div class="text-weight-medium"><q-icon name="o_key" class="q-mr-xs" />{{ t('agentes.opcionClave') }}</div>
                      <div class="text-caption texto-suave q-mt-xs">{{ t('agentes.opcionClaveTexto') }}</div>
                      <div class="bloque-codigo q-mt-sm">{{ comando(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(comando(a))" /></div>
                      <div class="text-caption texto-suave q-mt-xs">{{ textoAlmacen(a) }}</div>
                    </div>
                  </div>
                </div>
              </template>
              <template v-else>
                <div class="text-body2">{{ t('agentes.soloClave') }}</div>
                <div class="bloque-codigo q-mt-sm">{{ comando(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(comando(a))" /></div>
                <div class="text-caption texto-suave q-mt-xs">{{ textoAlmacen(a) }}</div>
              </template>
            </template>

            <div v-else-if="tipo(a) === 'agente.noInstalado'" class="aviso text-body2">
              {{ t('agentes.instalarAntes') }} <span class="codigo">comandos_agentes.{{ a.agente }}</span>.
            </div>
            <div v-else class="aviso text-body2">{{ a.problema }}</div>
          </q-card-section>

          <q-expansion-item dense dense-toggle :label="t('agentes.detalles')" header-class="text-caption texto-suave q-px-md">
            <div class="q-px-md q-pb-md">
              <div class="ficha text-caption">
                <div>{{ t('agentes.version') }}</div><div>{{ a.version_detectada || t('agentes.sinVersion') }}</div>
                <div>{{ t('agentes.autenticacion') }}</div><div>{{ suscripcion(a) ? t('agentes.suscripcion') : t('agentes.claveApi') }}</div>
                <div>{{ t('agentes.enVivo') }}</div><div>{{ a.capacidades.streaming === 'tokens' ? t('agentes.fragmentos') : t('agentes.alTerminar') }}</div>
                <div>{{ t('agentes.json') }}</div><div>{{ a.capacidades.salida_estructurada ? t('comun.si') : t('comun.no') }}</div>
                <div>{{ t('agentes.informaModelo') }}</div><div>{{ a.capacidades.modelo_efectivo_informado ? t('comun.si') : t('comun.no') }}</div>
                <div>{{ t('agentes.esfuerzo') }}</div><div>{{ esfuerzos[a.agente]?.length ? esfuerzos[a.agente].map(nombreEsfuerzo).join(' · ') : t('comun.no') }}</div>
                <div>{{ t('agentes.contexto') }}</div><div>{{ contextos[a.agente]?.length ? textoContextos(a.agente) : t('agentes.contextoDesconocido') }}</div>
              </div>
              <div v-if="a.terminos" class="text-caption texto-suave q-mt-sm">{{ a.terminos }}</div>
            </div>
          </q-expansion-item>
        </q-card>
      </div>
    </div>
    <div v-else class="row q-col-gutter-md">
      <div v-for="n in 4" :key="n" class="col-12 col-lg-6">
        <q-card class="q-pa-md"><q-skeleton type="text" width="40%" /><q-skeleton v-for="m in 3" :key="m" type="text" /></q-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, codigoLinea, copiar, modeloInicial, nombreAgente, nombreEsfuerzo, ordenarAgentes, sesion, type Adaptador, numero } from '../api'
import { t } from '../i18n'

const $q = useQuasar()
const emit = defineEmits<{ ir: [pagina: string] }>()

function probar(a: Adaptador) {
  modeloInicial.value = a.agente
  emit('ir', 'ejecutar')
}
const adaptadores = ref<Adaptador[]>()
// esfuerzos: los niveles que admite cada agente, según el catálogo de modelos.
const esfuerzos = ref<Record<string, string[]>>({})
// contextos: el contexto conocido de los modelos de cada agente (configurado o informado), para ver qué cabe.
const contextos = ref<Record<string, { nombre: string; tokens: number; origen: string }[]>>({})
const textoContextos = (agente: string) => (contextos.value[agente] ?? [])
  .map((c) => `${c.nombre} ${numero(c.tokens)}${c.origen === 'configurado' ? ' (' + t('agentes.configurado') + ')' : ''}`).join(' · ')
const cargando = ref(false)
const cambiando = ref('')

// tipo clasifica el problema por su código (el texto llega en el idioma del servicio) para mostrar la ayuda adecuada.
function tipo(a: Adaptador) {
  if (a.problema_codigo === 'no_instalado') return 'agente.noInstalado' as const
  if (a.problema_codigo === 'falta_clave') return 'agente.faltaClave' as const
  return 'agente.revisar' as const
}
const suscripcion = (a: Adaptador) => a.autenticacion === 'suscripcion'
// Solo Claude y Codex tienen suscripción, y solo se cambia desde una sesión de este mismo equipo.
const puedeCambiar = (a: Adaptador) => (a.agente === 'claude' || a.agente === 'codex') && !!sesion.value?.local
const cli = (a: Adaptador) => ({ claude: 'Claude Code', codex: 'Codex CLI' })[a.agente] ?? a.agente
const claves: Record<string, string> = { claude: 'ANTHROPIC_API_KEY', codex: 'OPENAI_API_KEY', gemini: 'GEMINI_API_KEY', antigravity: 'GEMINI_API_KEY' }
const variable = (a: Adaptador) => claves[a.agente] ?? a.problema?.match(/[A-Z_]+_API_KEY/)?.[0] ?? ''
const comando = (a: Adaptador) => 'tramtai claves guardar ' + variable(a)
// textoAlmacen dice dónde queda la clave en este sistema: cifrada en su almacén, o qué hacer si no lo hay.
function textoAlmacen(a: Adaptador) {
  const almacen = sesion.value?.almacen
  if (almacen?.disponible) return t('agentes.almacenDisponible', { almacen: almacen.nombre })
  if (almacen?.nombre) return t('agentes.almacenFalta', { variable: variable(a) })
  return t('agentes.sinAlmacen', { variable: variable(a) })
}

async function cargar() {
  cargando.value = true
  try {
    adaptadores.value = ordenarAgentes((await api.capacidades()).adaptadores)
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
}

// La suscripción solo se activa si el usuario acepta el riesgo con los términos del proveedor (TERMINOS.md, punto 4).
function confirmarSuscripcion(a: Adaptador) {
  $q.dialog({ title: t('agentes.confirmarTitulo', { agente: nombreAgente(a.agente) }),
    message: t('agentes.confirmarSuscripcion', { cli: cli(a) }) + '\n\n' + t('agentes.riesgoSuscripcion'),
    style: 'white-space: pre-line',
    options: { type: 'checkbox', model: [], items: [{ label: t('agentes.aceptoRiesgo'), value: 'acepto' }] },
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('agentes.usarSuscripcion'), unelevated: true, noCaps: true } })
    .onOk((elegidas: string[]) => {
      if (elegidas.includes('acepto')) cambiar(a, 'suscripcion')
      else avisarError(new Error(t('agentes.faltaAceptar')))
    })
}

async function cambiar(a: Adaptador, modo: 'suscripcion' | 'clave_api') {
  cambiando.value = a.agente
  try {
    await api.cambiarAutenticacion(a.agente, modo)
    avisar(t(modo === 'suscripcion' ? 'agentes.suscripcionActivada' : 'agentes.claveActivada', { agente: nombreAgente(a.agente) }))
    await cargar()
  } catch (e) {
    avisarError(e)
  } finally {
    cambiando.value = ''
  }
}

onMounted(() => {
  cargar()
  api.catalogo().then((lista) => {
    for (const m of lista) if (m.agente && !m.modelo && !m.perfil) esfuerzos.value[m.agente] = m.esfuerzos ?? []
    for (const m of lista) {
      if (m.agente && !m.perfil && m.contexto_tokens) {
        const nombre = m.version || m.modelo || t('agentes.predeterminadoCli')
        if (!contextos.value[m.agente]?.some((c) => c.nombre === nombre)) {
          ;(contextos.value[m.agente] ??= []).push({ nombre, tokens: m.contexto_tokens, origen: m.contexto_origen ?? '' })
        }
      }
    }
  }).catch(() => {})
})
</script>
