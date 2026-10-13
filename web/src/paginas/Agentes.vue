<template>
  <div class="pagina apilado">
    <div class="row items-end q-col-gutter-md">
      <div class="col-12 col-sm">
        <div class="titulo-pagina">{{ t('menu.agentes') }}</div>
        <div class="texto-suave">{{ t('agentes.subtitulo') }}</div>
      </div>
      <div class="col-12 col-sm-auto">
        <q-btn flat no-caps color="primary" icon="o_refresh" :label="t('agentes.recomprobar')" :loading="cargando" @click="cargar" />
      </div>
    </div>

    <div v-if="adaptadores" class="row q-col-gutter-md">
      <div v-for="a in adaptadores" :key="a.agente" class="col-12 col-lg-6">
        <q-card class="full-height column">
          <q-card-section class="row items-center no-wrap">
            <q-avatar :color="a.problema ? 'grey-5' : 'primary'" text-color="white" icon="o_smart_toy" size="40px" class="q-mr-md" />
            <div class="col titulo-seccion">{{ nombreAgente(a.agente) }}</div>
            <q-chip :color="a.problema ? 'warning' : 'positive'" text-color="white" :icon="a.problema ? 'priority_high' : 'check'">
              {{ a.problema ? t(tipo(a)) : t('agente.listo') }}
            </q-chip>
          </q-card-section>

          <q-card-section class="q-pt-none col">
            <!-- Listo: cómo está conectado y, en Claude y Codex, la otra forma -->
            <template v-if="!a.problema">
              <div class="row items-center q-gutter-x-sm">
                <q-icon :name="suscripcion(a) ? 'o_person' : 'o_key'" color="positive" />
                <span>{{ suscripcion(a) ? t('agentes.listoSuscripcion') : t('agentes.listoClave') }}</span>
              </div>
              <q-btn v-if="puedeCambiar(a)" flat dense no-caps size="sm" color="primary" class="q-mt-sm q-px-none" :loading="cambiando === a.agente"
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
                    </div>
                  </div>
                </div>
              </template>
              <template v-else>
                <div class="text-body2">{{ t('agentes.soloClave') }}</div>
                <div class="bloque-codigo q-mt-sm">{{ comando(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar(comando(a))" /></div>
              </template>
            </template>

            <div v-else-if="tipo(a) === 'agente.noInstalado'" class="aviso text-body2">
              {{ t('agentes.instalarAntes') }} <span class="codigo">comandos_agentes.{{ a.agente }}</span>.
            </div>
            <div v-else class="aviso text-body2">{{ a.problema }}</div>
          </q-card-section>

          <q-expansion-item dense dense-toggle :label="t('agentes.detalles')" header-class="text-caption texto-suave q-px-md">
            <q-list dense class="q-pb-sm">
              <q-item><q-item-section class="texto-suave">{{ t('agentes.version') }}</q-item-section><q-item-section side>{{ a.version_detectada || t('agentes.sinVersion') }}</q-item-section></q-item>
              <q-item><q-item-section class="texto-suave">{{ t('agentes.autenticacion') }}</q-item-section><q-item-section side>{{ suscripcion(a) ? t('agentes.suscripcion') : t('agentes.claveApi') }}</q-item-section></q-item>
              <q-item><q-item-section class="texto-suave">{{ t('agentes.enVivo') }}</q-item-section><q-item-section side>{{ a.capacidades.streaming === 'tokens' ? t('agentes.fragmentos') : t('agentes.alTerminar') }}</q-item-section></q-item>
              <q-item><q-item-section class="texto-suave">{{ t('agentes.json') }}</q-item-section><q-item-section side>{{ a.capacidades.salida_estructurada ? t('comun.si') : t('comun.no') }}</q-item-section></q-item>
              <q-item><q-item-section class="texto-suave">{{ t('agentes.informaModelo') }}</q-item-section><q-item-section side>{{ a.capacidades.modelo_efectivo_informado ? t('comun.si') : t('comun.no') }}</q-item-section></q-item>
              <q-item v-if="a.terminos"><q-item-section class="text-caption texto-suave">{{ a.terminos }}</q-item-section></q-item>
            </q-list>
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
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, copiar, nombreAgente, sesion, type Adaptador } from '../api'
import { t } from '../i18n'

const $q = useQuasar()
const adaptadores = ref<Adaptador[]>()
const cargando = ref(false)
const cambiando = ref('')

// tipo clasifica el problema que informa el servicio (en español) para mostrar la ayuda adecuada.
function tipo(a: Adaptador) {
  if (/no se encontró|envoltorio/.test(a.problema ?? '')) return 'agente.noInstalado' as const
  if (/falta/i.test(a.problema ?? '')) return 'agente.faltaClave' as const
  return 'agente.revisar' as const
}
const suscripcion = (a: Adaptador) => a.autenticacion === 'suscripcion'
// Solo Claude y Codex tienen suscripción, y solo se cambia desde una sesión de este mismo equipo.
const puedeCambiar = (a: Adaptador) => (a.agente === 'claude' || a.agente === 'codex') && !!sesion.value?.local
const cli = (a: Adaptador) => ({ claude: 'Claude Code', codex: 'Codex CLI' })[a.agente] ?? a.agente
const claves: Record<string, string> = { claude: 'ANTHROPIC_API_KEY', codex: 'OPENAI_API_KEY', gemini: 'GEMINI_API_KEY', antigravity: 'GEMINI_API_KEY' }
const comando = (a: Adaptador) => 'tramtai claves guardar ' + (claves[a.agente] ?? a.problema?.match(/[A-Z_]+_API_KEY/)?.[0] ?? '')

async function cargar() {
  cargando.value = true
  try {
    adaptadores.value = (await api.capacidades()).adaptadores
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
}

function confirmarSuscripcion(a: Adaptador) {
  $q.dialog({ title: t('agentes.confirmarTitulo', { agente: nombreAgente(a.agente) }), message: t('agentes.confirmarSuscripcion', { cli: cli(a) }),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('agentes.usarSuscripcion'), unelevated: true, noCaps: true } })
    .onOk(() => cambiar(a, 'suscripcion'))
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

onMounted(cargar)
</script>
