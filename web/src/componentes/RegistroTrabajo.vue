<template>
  <q-expansion-item v-model="abierto" dense icon="o_receipt_long" :label="t('registro.titulo')" :caption="resumen"
    header-class="q-px-none text-body2">
    <div v-if="fallo" class="text-caption texto-suave q-py-sm">{{ t('registro.noDisponible') }}</div>
    <template v-else>
      <ol class="registro q-my-sm">
        <li v-for="l in lineas" :key="l.numero" :class="{ 'registro__aviso': l.aviso }">
          <span class="codigo texto-suave">{{ l.hora }}</span>
          <span class="codigo texto-suave registro__desde">{{ l.desde }}</span>
          <span>{{ l.texto }}</span>
        </li>
      </ol>
    </template>
  </q-expansion-item>
</template>

<script setup lang="ts">
// RegistroTrabajo muestra la hora de cada paso de una petición (estados, intentos y avisos) y cuánto duró cada etapa,
// para ver lo que la ruta animada pasa demasiado rápido. Vuelve a pedirlo cuando cambia version.
import { computed, onUnmounted, ref, watch } from 'vue'
import { api, estadoInfo, nombreAgente, nombreEsfuerzo, numero, type EventoRegistro, type Trabajo } from '../api'
import { idioma, t } from '../i18n'

const props = defineProps<{ trabajo: Trabajo; version?: string | number }>()
const emit = defineEmits<{ cargado: [eventos: EventoRegistro[]] }>()

const abierto = ref(false)
const eventos = ref<EventoRegistro[]>([])
const fallo = ref(false)
let pedido = 0
let espera: ReturnType<typeof setTimeout> | undefined

// Al cambiar de trabajo se vacía al momento (no se mezcla con el anterior); los avisos seguidos de un mismo trabajo
// (al abrir uno terminado llegan todos sus eventos de golpe) se agrupan en una sola petición.
watch(() => props.trabajo.id, () => {
  pedido++ // la respuesta del trabajo anterior que aún esté en camino ya no vale
  eventos.value = []
  fallo.value = false
})
watch(() => [props.trabajo.id, props.version], () => {
  clearTimeout(espera)
  espera = setTimeout(cargar, 200)
}, { immediate: true })
onUnmounted(() => clearTimeout(espera))

async function cargar() {
  const actual = ++pedido // una respuesta vieja no pisa la de un trabajo o una versión posterior
  try {
    const lista = await api.registro(props.trabajo.id)
    if (actual !== pedido) return
    eventos.value = lista
    fallo.value = false
    emit('cargado', lista)
  } catch {
    if (actual === pedido) fallo.value = true
  }
}

const ms = (texto: string) => new Date(texto).getTime()
function tiempo(n: number): string {
  if (n < 1000) return `${Math.max(0, Math.round(n))} ms`
  return `${(n / 1000).toLocaleString(idioma.value, { maximumFractionDigits: 1 })} s`
}
function hora(texto: string): string {
  const d = new Date(texto)
  return d.toLocaleTimeString(idioma.value, { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}
function textoIntento(datos: string): string {
  try {
    const i = JSON.parse(datos) as { agente: string; modelo?: string; esfuerzo?: string; del_agente?: boolean; contexto_tokens?: number; correccion?: number; correcciones?: number }
    const partes = [nombreAgente(i.agente) + (i.modelo ? '/' + i.modelo : '')]
    if (i.correccion) partes.unshift(t('registro.correccion', { n: i.correccion, de: i.correcciones ?? i.correccion }))
    if (i.esfuerzo) partes.push(nombreEsfuerzo(i.esfuerzo).toLowerCase() + (i.del_agente ? ` (${t('registro.delAgente')})` : ''))
    if (i.contexto_tokens) partes.push(t('registro.contexto', { n: numero(i.contexto_tokens) }))
    return partes.join(' · ')
  } catch {
    return ''
  }
}

// textoEntrada dice cuánto ocupa lo que recibe el agente y qué documento pesa más.
function textoEntrada(datos: string): string {
  try {
    const m = JSON.parse(datos) as { tokens_estimados: number; documentos?: { nombre: string; tokens_estimados: number }[] }
    const mayor = m.documentos?.[0]
    return t('registro.entrada', { n: numero(m.tokens_estimados) }) + (mayor ? ` (${mayor.nombre} ~${numero(mayor.tokens_estimados)})` : '')
  } catch {
    return ''
  }
}

// textoValidacion resume por qué la respuesta no cumplía el esquema: el primer problema y cuántos más.
function textoValidacion(datos: string): string {
  try {
    const v = JSON.parse(datos) as { problemas?: { texto: string }[] }
    const p = v.problemas ?? []
    if (!p.length) return ''
    return t('registro.validacion', { texto: p[0].texto }) + (p.length > 1 ? ' ' + t('registro.validacionMas', { n: p.length - 1 }) : '')
  } catch {
    return ''
  }
}

// El intento va justo después de su estado ejecutando: se muestran en una sola línea.
const lineas = computed(() => {
  const inicio = ms(props.trabajo.creado_en)
  const lista: { numero: number; hora: string; desde: string; texto: string; aviso: boolean }[] = []
  eventos.value.forEach((e, i) => {
    let texto = ''
    if (e.tipo === 'estado') {
      texto = estadoInfo(e.datos).etiqueta
      const siguiente = eventos.value[i + 1]
      if (siguiente?.tipo === 'intento') texto += ' · ' + textoIntento(siguiente.datos)
    } else if (e.tipo === 'aviso') texto = t('registro.aviso', { texto: e.datos })
    else if (e.tipo === 'entrada') texto = textoEntrada(e.datos)
    else if (e.tipo === 'validacion') texto = textoValidacion(e.datos)
    if (texto) lista.push({ numero: e.numero, hora: hora(e.creado_en), desde: '+' + tiempo(ms(e.creado_en) - inicio), texto, aviso: e.tipo === 'aviso' || e.tipo === 'validacion' })
  })
  return lista
})

// Etapas como las estaciones de la ruta, resumidas bajo el título; cada tramo cuenta desde su estado hasta el siguiente.
const etapaDe: Record<string, 'cola' | 'preparacion' | 'ejecucion' | 'validacion'> = {
  en_cola: 'cola', preparando_entrada: 'preparacion', esperando_agente: 'preparacion', ejecutando: 'ejecucion',
  cancelacion_solicitada: 'ejecucion', validando_salida: 'validacion',
}
const etapas = computed(() => {
  const estados = eventos.value.filter((e) => e.tipo === 'estado')
  const suma: Partial<Record<string, number>> = {}
  estados.slice(0, -1).forEach((e, i) => {
    const etapa = etapaDe[e.datos]
    if (etapa) suma[etapa] = (suma[etapa] ?? 0) + ms(estados[i + 1].creado_en) - ms(e.creado_en)
  })
  return (['cola', 'preparacion', 'ejecucion', 'validacion'] as const).filter((k) => suma[k] !== undefined)
    .map((k) => ({ nombre: t(`ruta.${k}`), duracion: tiempo(suma[k]!) }))
})
const resumen = computed(() => etapas.value.map((e) => `${e.nombre} ${e.duracion}`).join(' · '))
</script>
