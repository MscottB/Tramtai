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
      <div v-for="a in adaptadores" :key="a.agente" class="col-12 col-md-6">
        <q-card class="full-height column">
          <q-card-section class="row items-center no-wrap">
            <q-avatar color="primary" text-color="white" icon="o_smart_toy" size="40px" class="q-mr-md" />
            <div class="col">
              <div class="titulo-seccion">{{ nombreAgente(a.agente) }}</div>
              <div class="text-caption texto-suave">{{ a.version_detectada || t('agentes.sinVersion') }}</div>
            </div>
            <q-chip :color="a.problema ? 'warning' : 'positive'" text-color="white" :icon="a.problema ? 'priority_high' : 'check'">
              {{ etiqueta(a) }}
            </q-chip>
          </q-card-section>

          <q-card-section v-if="a.problema" class="q-pt-none">
            <div class="aviso">
              <div>{{ claveQueFalta(a) ? t('agentes.faltaClave', { clave: claveQueFalta(a) }) : a.problema }}</div>
              <template v-if="claveQueFalta(a)">
                <div class="q-mt-sm text-weight-medium">{{ t('agentes.guardaClave') }}</div>
                <div class="bloque-codigo q-mt-xs">tramtai claves guardar {{ claveQueFalta(a) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('agentes.copiarComando')" @click="copiar('tramtai claves guardar ' + claveQueFalta(a))" /></div>
                <div v-if="admiteSuscripcion(a)" class="text-caption q-mt-sm">
                  {{ t('agentes.suscripcionAntes') }} <span class="codigo">adaptadores.{{ a.agente }}.autenticacion: suscripcion</span>
                  {{ t('agentes.suscripcionDespues') }}
                </div>
              </template>
              <div v-else-if="tipo(a) === 'agente.noInstalado'" class="text-caption q-mt-sm">
                {{ t('agentes.instalarAntes') }} <span class="codigo">comandos_agentes.{{ a.agente }}</span>.
              </div>
            </div>
          </q-card-section>

          <q-list dense class="q-pb-sm">
            <q-item>
              <q-item-section class="texto-suave">{{ t('agentes.autenticacion') }}</q-item-section>
              <q-item-section side>{{ a.autenticacion === 'suscripcion' ? t('agentes.suscripcion') : t('agentes.claveApi') }}</q-item-section>
            </q-item>
            <q-item>
              <q-item-section class="texto-suave">{{ t('agentes.enVivo') }}</q-item-section>
              <q-item-section side>{{ a.capacidades.streaming === 'tokens' ? t('agentes.fragmentos') : t('agentes.alTerminar') }}</q-item-section>
            </q-item>
            <q-item>
              <q-item-section class="texto-suave">{{ t('agentes.json') }}</q-item-section>
              <q-item-section side>{{ a.capacidades.salida_estructurada ? t('comun.si') : t('comun.no') }}</q-item-section>
            </q-item>
            <q-item>
              <q-item-section class="texto-suave">{{ t('agentes.informaModelo') }}</q-item-section>
              <q-item-section side>{{ a.capacidades.modelo_efectivo_informado ? t('comun.si') : t('comun.no') }}</q-item-section>
            </q-item>
          </q-list>
          <q-space />
          <q-card-section v-if="a.terminos" class="text-caption texto-suave q-pt-none">{{ a.terminos }}</q-card-section>
        </q-card>
      </div>
    </div>
    <div v-else class="row q-col-gutter-md">
      <div v-for="n in 4" :key="n" class="col-12 col-md-6">
        <q-card class="q-pa-md"><q-skeleton type="text" width="40%" /><q-skeleton v-for="m in 4" :key="m" type="text" /></q-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, avisarError, copiar, nombreAgente, type Adaptador } from '../api'
import { t } from '../i18n'

const adaptadores = ref<Adaptador[]>()
const cargando = ref(false)

// tipo clasifica el problema que informa el servicio (en español) para mostrar la ayuda adecuada.
function tipo(a: Adaptador) {
  if (/no se encontró|envoltorio/.test(a.problema ?? '')) return 'agente.noInstalado' as const
  if (/falta/i.test(a.problema ?? '')) return 'agente.faltaClave' as const
  return 'agente.revisar' as const
}
const etiqueta = (a: Adaptador) => (a.problema ? t(tipo(a)) : t('agente.listo'))
const claveQueFalta = (a: Adaptador) => a.problema?.match(/falta ([A-Z_]+_API_KEY)/)?.[1] ?? ''
const admiteSuscripcion = (a: Adaptador) => a.agente === 'claude' || a.agente === 'codex'

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

onMounted(cargar)
</script>
