<template>
  <!-- En el móvil, la red se lee de arriba abajo: tus apps, Tramtai y las líneas de los agentes en un solo eje. -->
  <svg v-if="$q.screen.lt.sm" class="mapa-red" :viewBox="`0 0 ${V_ANCHO} ${vAlto}`" role="img" :aria-label="descripcion">
    <line :x1="V_EJE" :y1="vOrigen(0)" :x2="V_EJE" :y2="vAgente(lineas.length - 1)" stroke-width="3" :style="{ stroke: 'var(--borde-fuerte)' }" />
    <g v-for="(o, i) in origenes" :key="'vo' + o.id" class="cursor-pointer" role="link" tabindex="0" @click="emit('ir', 'conectar')" @keyup.enter="emit('ir', 'conectar')">
      <circle :cx="V_EJE" :cy="vOrigen(i)" r="6" stroke-width="3" :style="{ fill: 'var(--superficie)', stroke: o.activo ? 'var(--acento)' : 'var(--borde-fuerte)' }" />
      <text :x="V_EJE + 18" :y="vOrigen(i) + 5" font-size="14" :font-weight="o.principal ? 600 : 500" :class="{ suave: !o.activo }">{{ o.nombre }}</text>
    </g>
    <g class="cursor-pointer" role="link" tabindex="0" @click="emit('ir', 'ejecutar')" @keyup.enter="emit('ir', 'ejecutar')">
      <rect x="4" :y="vHub" :width="V_ANCHO - 8" height="58" rx="29" stroke-width="4" :style="{ fill: 'var(--superficie)', stroke: 'var(--texto)' }" />
      <text :x="V_ANCHO / 2" :y="vHub + 26" text-anchor="middle" font-size="16" font-weight="800">Tramtai</text>
      <text :x="V_ANCHO / 2" :y="vHub + 44" text-anchor="middle" font-size="12" class="suave">{{ direccion }}</text>
    </g>
    <g v-for="(a, i) in lineas" :key="'va' + a.agente" class="cursor-pointer" role="link" tabindex="0" @click="emit('ir', 'agentes')" @keyup.enter="emit('ir', 'agentes')">
      <line :x1="V_EJE" :y1="vAgente(i)" :x2="V_EJE + 26" :y2="vAgente(i)" stroke-width="4" stroke-linecap="round"
        :style="{ stroke: a.listo ? a.color : 'var(--borde-fuerte)' }" :stroke-dasharray="a.listo ? undefined : '4 5'" />
      <circle :cx="V_EJE + 34" :cy="vAgente(i)" r="7" stroke-width="3" :style="{ fill: a.listo ? a.color : 'var(--superficie)', stroke: a.listo ? a.color : 'var(--borde-fuerte)' }" />
      <text :x="V_EJE + 52" :y="vAgente(i) + 5" font-size="15" font-weight="650">{{ a.nombre }}</text>
      <text :x="V_ANCHO - 6" :y="vAgente(i) + 5" text-anchor="end" font-size="12" :style="{ fill: a.listo ? 'var(--ok)' : 'var(--aviso)' }">{{ a.estado }}</text>
    </g>
  </svg>
  <div v-else style="overflow-x: auto">
    <svg class="mapa-red" :viewBox="`0 0 ${ANCHO} ${alto}`" :style="{ minWidth: '620px' }" role="img" :aria-label="descripcion">
      <!-- Tus apps → Tramtai -->
      <path v-for="o in origenes" :key="'p' + o.id" :d="tramo(X_ORIGEN, o.y, X_HUB, centro + paralela(o.i, origenes.length))" fill="none" stroke-width="3" stroke-linecap="round"
        :style="{ stroke: o.activo ? 'color-mix(in srgb, var(--acento) 55%, transparent)' : 'var(--borde-fuerte)' }" :stroke-dasharray="o.activo ? undefined : PUNTEADO" />
      <!-- Tramtai → agentes: cada línea con su color -->
      <path v-for="a in lineas" :key="'l' + a.agente" :d="tramo(X_HUB + HUB_ANCHO, centro + paralela(a.i, lineas.length), X_AGENTE, a.y)" fill="none" stroke-width="4" stroke-linecap="round"
        :style="{ stroke: a.listo ? a.color : 'var(--borde-fuerte)' }" :stroke-dasharray="a.listo ? undefined : PUNTEADO" />

      <g v-for="o in origenes" :key="'o' + o.id" class="cursor-pointer" role="link" tabindex="0" @click="emit('ir', 'conectar')" @keyup.enter="emit('ir', 'conectar')">
        <circle :cx="X_ORIGEN" :cy="o.y" r="7" stroke-width="3" :style="{ fill: 'var(--superficie)', stroke: o.activo ? 'var(--acento)' : 'var(--borde-fuerte)' }" />
        <text :x="X_ORIGEN - 18" :y="o.y + 5" text-anchor="end" font-size="14" :font-weight="o.principal ? 600 : 500"
          :class="{ suave: !o.activo }">{{ o.nombre }}</text>
      </g>

      <!-- Tramtai es el intercambiador: la cápsula blanca de borde grueso de los transbordos. -->
      <g class="cursor-pointer" role="link" tabindex="0" @click="emit('ir', 'ejecutar')" @keyup.enter="emit('ir', 'ejecutar')">
        <rect :x="X_HUB" :y="centro - 36" :width="HUB_ANCHO" height="72" rx="36" stroke-width="5" :style="{ fill: 'var(--superficie)', stroke: 'var(--texto)' }" />
        <text :x="X_HUB + HUB_ANCHO / 2" :y="centro - 3" text-anchor="middle" font-size="18" font-weight="800">Tramtai</text>
        <text :x="X_HUB + HUB_ANCHO / 2" :y="centro + 17" text-anchor="middle" font-size="12" class="suave">{{ direccion }}</text>
      </g>

      <g v-for="a in lineas" :key="'a' + a.agente" class="cursor-pointer" role="link" tabindex="0" @click="emit('ir', 'agentes')" @keyup.enter="emit('ir', 'agentes')">
        <circle :cx="X_AGENTE" :cy="a.y" r="9" stroke-width="3.5" :style="{ fill: a.listo ? a.color : 'var(--superficie)', stroke: a.listo ? a.color : 'var(--borde-fuerte)' }" />
        <text :x="X_AGENTE + 20" :y="a.y - 2" font-size="15" font-weight="650">{{ a.nombre }}</text>
        <text :x="X_AGENTE + 20" :y="a.y + 16" font-size="12" :style="{ fill: a.listo ? 'var(--ok)' : 'var(--aviso)' }">{{ a.estado }}</text>
      </g>
    </svg>
  </div>
</template>

<script setup lang="ts">
// MapaRed dibuja la red de Tramtai como un mapa de tranvía: tus apps (la API principal y tus APIs) llegan a
// Tramtai, que reparte cada petición por la línea de un agente. Las líneas sin servicio van punteadas.
import { computed } from 'vue'
import { nombreAgente, type Adaptador, type EstadoPunto } from '../api'
import { t } from '../i18n'

const props = defineProps<{ adaptadores: Adaptador[]; apis: EstadoPunto[]; base: string }>()
const emit = defineEmits<{ ir: [pagina: string] }>()

const ANCHO = 980
const X_ORIGEN = 230
const X_HUB = 370
const HUB_ANCHO = 170
const X_AGENTE = 770
const PASO = 58
const MAX_APIS = 3

const colores: Record<string, string> = { claude: 'var(--linea-claude)', codex: 'var(--linea-codex)', gemini: 'var(--linea-gemini)', antigravity: 'var(--linea-antigravity)' }
const direccion = computed(() => props.base.replace(/^https?:\/\//, ''))
const filas = computed(() => Math.max(props.adaptadores.length, 1 + Math.min(props.apis.length, MAX_APIS + 1), 3))
const alto = computed(() => filas.value * PASO + 16)
const centro = computed(() => alto.value / 2)
const y = (i: number, n: number) => centro.value + (i - (n - 1) / 2) * PASO

const origenes = computed(() => {
  const activas = props.apis.filter((p) => p.error !== 'desactivado')
  const lista = [{ id: '-', nombre: t('mapa.principal'), activo: true, principal: true }]
  for (const p of props.apis.slice(0, MAX_APIS)) lista.push({ id: p.nombre, nombre: 'API ' + p.nombre, activo: p.error !== 'desactivado', principal: false })
  if (props.apis.length > MAX_APIS) lista.push({ id: '+', nombre: t('mapa.mas', { n: props.apis.length - MAX_APIS }), activo: activas.length > MAX_APIS, principal: false })
  return lista.map((o, i, todos) => ({ ...o, i, y: y(i, todos.length) }))
})

const lineas = computed(() => props.adaptadores.map((a, i) => ({
  agente: a.agente,
  i,
  nombre: nombreAgente(a.agente),
  listo: !a.problema,
  estado: a.problema ? t('mapa.sinServicio') : t('mapa.enServicio'),
  color: colores[a.agente] ?? 'var(--linea-otro)',
  y: y(i, props.adaptadores.length),
})))

const descripcion = computed(() => t('mapa.descripcion', {
  apis: origenes.value.length, listos: lineas.value.filter((l) => l.listo).length, total: lineas.value.length,
}))

// Disposición vertical (móvil): orígenes arriba, Tramtai en medio y los agentes debajo, sobre un mismo eje.
const V_ANCHO = 320
const V_EJE = 18
const V_PASO = 38
const vOrigen = (i: number) => 14 + i * V_PASO
const vHub = computed(() => vOrigen(origenes.value.length - 1) + 30)
const vAgente = (i: number) => vHub.value + 58 + 32 + i * V_PASO
const vAlto = computed(() => vAgente(Math.max(lineas.value.length - 1, 0)) + 20)

// paralela separa cada línea en el borde de Tramtai, para que salgan juntas pero sin tocarse.
const paralela = (i: number, n: number) => (i - (n - 1) / 2) * 10

// Las líneas sin servicio van punteadas: guiones de longitud 0 con extremo redondo, que se ven como puntos limpios
// también en las curvas (un guion largo se partía en trozos sueltos en cada quiebre).
const PUNTEADO = '0 9'

// tramo une dos estaciones como en un mapa de líneas: recto, una curva suave y recto otra vez.
function tramo(x1: number, y1: number, x2: number, y2: number) {
  const medio = (x1 + x2) / 2
  const curva = Math.min((x2 - x1) / 2 - 12, Math.max(36, Math.abs(y2 - y1)))
  return `M ${x1} ${y1} L ${medio - curva} ${y1} C ${medio} ${y1} ${medio} ${y2} ${medio + curva} ${y2} L ${x2} ${y2}`
}
</script>
