<template>
  <div class="pagina apilado">
    <div class="row items-end q-col-gutter-md">
      <div class="col-12 col-sm">
        <div class="titulo-pagina">Inicio</div>
        <div class="texto-suave">Tus agentes de IA como una API en este equipo: {{ instancia?.base ?? '…' }}</div>
      </div>
      <div class="col-12 col-sm-auto">
        <q-btn unelevated color="primary" icon="o_play_arrow" label="Ejecutar" no-caps @click="$emit('ir', 'ejecutar')" />
      </div>
    </div>

    <q-banner v-if="error" rounded class="error-caja">{{ error }}</q-banner>

    <div class="row q-col-gutter-md">
      <div v-for="c in contadores" :key="c.titulo" class="col-12 col-sm-4">
        <q-card class="q-pa-md full-height">
          <div class="row items-center no-wrap">
            <q-avatar :color="c.color" text-color="white" :icon="c.icono" size="40px" class="q-mr-md" />
            <div>
              <div class="text-h5 text-weight-bold">{{ c.valor }}</div>
              <div class="texto-suave text-body2">{{ c.titulo }}</div>
            </div>
          </div>
        </q-card>
      </div>
    </div>

    <div class="row q-col-gutter-md">
      <div class="col-12 col-md-7">
        <q-card class="full-height">
          <q-card-section class="titulo-seccion">Primeros pasos</q-card-section>
          <q-list>
            <q-item v-for="(p, i) in pasos" :key="p.titulo" class="q-py-md">
              <q-item-section avatar>
                <q-avatar size="32px" :color="p.hecho ? 'positive' : 'grey-3'" :text-color="p.hecho ? 'white' : 'grey-8'"
                  :icon="p.hecho ? 'check' : undefined">{{ p.hecho ? '' : i + 1 }}</q-avatar>
              </q-item-section>
              <q-item-section>
                <q-item-label class="text-weight-medium">{{ p.titulo }}</q-item-label>
                <q-item-label caption>{{ p.detalle }}</q-item-label>
              </q-item-section>
              <q-item-section side>
                <q-btn outline color="primary" no-caps :label="p.accion" @click="$emit('ir', p.pagina)" />
              </q-item-section>
            </q-item>
          </q-list>
        </q-card>
      </div>
      <div class="col-12 col-md-5">
        <q-card class="full-height">
          <q-card-section class="row items-center">
            <div class="titulo-seccion col">Agentes</div>
            <q-btn flat dense no-caps color="primary" label="Detalles" @click="$emit('ir', 'agentes')" />
          </q-card-section>
          <q-list v-if="adaptadores">
            <q-item v-for="a in adaptadores" :key="a.agente">
              <q-item-section avatar><q-icon name="o_smart_toy" color="grey-7" /></q-item-section>
              <q-item-section>{{ nombreAgente(a.agente) }}</q-item-section>
              <q-item-section side>
                <q-chip dense :color="listo(a) ? 'positive' : 'warning'" text-color="white" :icon="listo(a) ? 'check' : 'priority_high'">
                  {{ estadoAgente(a) }}
                </q-chip>
              </q-item-section>
            </q-item>
          </q-list>
          <div v-else class="q-pa-md"><q-skeleton v-for="n in 4" :key="n" type="text" class="q-mb-sm" /></div>
        </q-card>
      </div>
    </div>

    <q-card>
      <q-card-section class="row items-center">
        <div class="titulo-seccion col">Trabajos recientes</div>
        <q-btn flat dense no-caps color="primary" label="Ver historial" @click="$emit('ir', 'historial')" />
      </q-card-section>
      <q-list v-if="recientes.length" separator>
        <q-item v-for="t in recientes" :key="t.id" clickable @click="$emit('ir', 'historial')">
          <q-item-section avatar>
            <q-icon :name="estadoInfo(t.estado).icono" :color="estadoInfo(t.estado).color" />
          </q-item-section>
          <q-item-section>
            <q-item-label class="ellipsis">{{ textoDe(t) }}</q-item-label>
            <q-item-label caption>{{ estadoInfo(t.estado).etiqueta }} · {{ modeloDe(t) }}</q-item-label>
          </q-item-section>
          <q-item-section side class="text-caption">{{ haceCuanto(t.creado_en) }}</q-item-section>
        </q-item>
      </q-list>
      <div v-else class="vacio">
        <q-icon name="o_inbox" size="40px" />
        <div class="q-mt-sm">Todavía no hay trabajos. Prueba un agente desde <b>Ejecutar</b>.</div>
      </div>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api, estadoInfo, haceCuanto, mensajeError, modeloDe, nombreAgente, terminales, textoDe, type Adaptador, type EstadoInstancia, type Trabajo } from '../api'

defineEmits<{ ir: [pagina: string] }>()
const instancia = ref<EstadoInstancia>()
const recientes = ref<Trabajo[]>([])
const adaptadores = ref<Adaptador[]>()
const puntos = ref(0)
const error = ref('')

const total = (estados: string[]) => estados.reduce((n, e) => n + (instancia.value?.trabajos[e] ?? 0), 0)
const contadores = computed(() => [
  { titulo: 'Completados', valor: total(['completado']), icono: 'o_check_circle', color: 'positive' },
  { titulo: 'En curso', valor: total(Object.keys(instancia.value?.trabajos ?? {}).filter((e) => !terminales.includes(e))), icono: 'o_autorenew', color: 'primary' },
  { titulo: 'Con error', valor: total(['fallido', 'interrumpido']), icono: 'o_error', color: 'negative' },
])

const listo = (a: Adaptador) => !a.problema
function estadoAgente(a: Adaptador) {
  if (!a.problema) return 'Listo'
  if (/no se encontró|envoltorio/.test(a.problema)) return 'No instalado'
  if (/falta/i.test(a.problema)) return 'Falta la clave'
  return 'Revisar'
}

const pasos = computed(() => {
  const listos = adaptadores.value?.filter(listo).length ?? 0
  return [
    { titulo: 'Deja listo al menos un agente', detalle: adaptadores.value ? `${listos} de ${adaptadores.value.length} agentes listos` : 'Comprobando…',
      hecho: listos > 0, accion: 'Agentes', pagina: 'agentes' },
    { titulo: 'Prueba un agente', detalle: 'Escribe un mensaje y mira la respuesta en vivo', hecho: total(['completado']) > 0, accion: 'Ejecutar', pagina: 'ejecutar' },
    { titulo: 'Conecta tu aplicación', detalle: 'Cualquier app compatible con OpenAI o Anthropic, con la URL y la clave de aquí',
      hecho: puntos.value > 0, accion: 'Conectar', pagina: 'conectar' },
  ]
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
