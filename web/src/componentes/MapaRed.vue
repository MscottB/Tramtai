<template>
  <!-- En el móvil, la red se lee de arriba abajo: tus apps, Tramtai y las líneas de los agentes en un solo eje. -->
  <!-- role="group" y no "img": dentro hay estaciones y tranvías que se pulsan, y en una imagen no existirían. -->
  <svg v-if="$q.screen.lt.sm" class="mapa-red" :viewBox="`0 0 ${V_ANCHO} ${vAlto}`" role="group" :aria-label="descripcion">
    <line :x1="V_EJE" :y1="vOrigen(0)" :x2="V_EJE" :y2="vAgente(lineas.length - 1)" stroke-width="3" :style="{ stroke: 'var(--borde-fuerte)' }" />
    <g v-for="(o, i) in origenes" :key="'vo' + o.id" class="cursor-pointer" role="link" tabindex="0" @click="abrir(o)" @keyup.enter="abrir(o)">
      <circle :cx="V_EJE" :cy="vOrigen(i)" r="6" stroke-width="3" :style="{ fill: 'var(--superficie)', stroke: o.activo ? 'var(--acento)' : 'var(--borde-fuerte)' }" />
      <text :x="V_EJE + 18" :y="vOrigen(i) + 5" font-size="14" :font-weight="o.principal ? 600 : 500" :class="{ suave: !o.activo }">{{ o.detalle ? o.nombre + ' · ' + o.detalle : o.nombre }}</text>
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
    <text v-if="resto" :x="V_ANCHO - 14" :y="vHub - 6" text-anchor="end" font-size="12" font-weight="700">+{{ resto }}</text>
    <g v-for="v in vivos" :key="'vv' + v.tr.id" class="tranvia-vivo" role="button" tabindex="0" :aria-label="nombreTranvia(v.tr)"
      @click="emit('seguir', v.tr.id)" @keyup.enter="emit('seguir', v.tr.id)" @keydown.space.prevent="emit('seguir', v.tr.id)">
      <rect x="-12" y="-6" width="24" height="12" rx="5" :style="{ offsetPath: `path('${v.vCamino}')`, offsetDistance: v.distancia + '%', fill: v.color }" />
    </g>
  </svg>
  <div v-else style="overflow-x: auto">
    <svg class="mapa-red" :viewBox="`0 0 ${ANCHO} ${alto}`" :style="{ minWidth: '620px' }" role="group" :aria-label="descripcion">
      <!-- Tus apps → Tramtai -->
      <path v-for="o in origenes" :key="'p' + o.id" :d="tramo(X_ORIGEN, o.y, X_HUB, centro + paralela(o.i, origenes.length))" fill="none" stroke-width="3" stroke-linecap="round"
        :style="{ stroke: o.activo ? 'color-mix(in srgb, var(--acento) 55%, transparent)' : 'var(--borde-fuerte)' }" :stroke-dasharray="o.activo ? undefined : PUNTEADO" />
      <!-- Tramtai → agentes: cada línea con su color -->
      <path v-for="a in lineas" :key="'l' + a.agente" :d="tramo(X_HUB + HUB_ANCHO, centro + paralela(a.i, lineas.length), X_AGENTE, a.y)" fill="none" stroke-width="4" stroke-linecap="round"
        :style="{ stroke: a.listo ? a.color : 'var(--borde-fuerte)' }" :stroke-dasharray="a.listo ? undefined : PUNTEADO" />

      <g v-for="o in origenes" :key="'o' + o.id" class="cursor-pointer" role="link" tabindex="0" @click="abrir(o)" @keyup.enter="abrir(o)">
        <circle :cx="X_ORIGEN" :cy="o.y" r="7" stroke-width="3" :style="{ fill: 'var(--superficie)', stroke: o.activo ? 'var(--acento)' : 'var(--borde-fuerte)' }" />
        <text :x="X_ORIGEN - 18" :y="o.detalle ? o.y - 2 : o.y + 5" text-anchor="end" font-size="14" :font-weight="o.principal ? 600 : 500"
          :class="{ suave: !o.activo }">{{ o.nombre }}</text>
        <text v-if="o.detalle" :x="X_ORIGEN - 18" :y="o.y + 15" text-anchor="end" font-size="12" class="suave">{{ o.detalle }}</text>
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

      <!-- Las peticiones en curso: cada tranvía va de su estación a Tramtai y sigue por la línea de su agente. -->
      <text v-if="resto" :x="X_HUB + HUB_ANCHO - 14" :y="centro - 44" text-anchor="end" font-size="12" font-weight="700">+{{ resto }}</text>
      <g v-for="v in vivos" :key="'v' + v.tr.id" class="tranvia-vivo" role="button" tabindex="0" :aria-label="nombreTranvia(v.tr)"
        @click="emit('seguir', v.tr.id)" @keyup.enter="emit('seguir', v.tr.id)" @keydown.space.prevent="emit('seguir', v.tr.id)">
        <rect x="-13" y="-7" width="26" height="14" rx="5" :style="{ offsetPath: `path('${v.camino}')`, offsetDistance: v.distancia + '%', fill: v.color }" />
      </g>
    </svg>
  </div>
</template>

<script setup lang="ts">
// MapaRed dibuja la red de Tramtai como un mapa de tranvía: tus apps (la clave del espacio y tus APIs) llegan a
// Tramtai, que reparte cada petición por la línea de un agente. Las líneas sin servicio van punteadas.
import { computed } from 'vue'
import type { Adaptador, EstadoPunto } from '../api/tipos'
import { grupoInicial } from '../estado'
import { nombreAgente, colorLinea, agenteListo } from '../agentes'
import { t } from '../i18n'
import { claveEtapa, type Tranvia } from '../vivo'

const props = defineProps<{ adaptadores: Adaptador[]; apis: EstadoPunto[]; base: string; tranvias?: Tranvia[]; resto?: number }>()
const emit = defineEmits<{ ir: [pagina: string]; seguir: [id: string] }>()

const ANCHO = 980
const X_ORIGEN = 230
const X_HUB = 370
const HUB_ANCHO = 170
const X_AGENTE = 770
const PASO = 58
const MAX_APIS = 3

const direccion = computed(() => props.base.replace(/^https?:\/\//, ''))
const filas = computed(() => Math.max(props.adaptadores.length, 1 + Math.min(props.apis.length, MAX_APIS + 1), 3))
const alto = computed(() => filas.value * PASO + 16)
const centro = computed(() => alto.value / 2)
const y = (i: number, n: number) => centro.value + (i - (n - 1) / 2) * PASO

// origenes: la clave del espacio, una estación por grupo y las APIs sin grupo; como mucho MAX_APIS y «y N más».
type Origen = { id: string; nombre: string; activo: boolean; principal: boolean; grupo?: string; detalle?: string }
const origenes = computed(() => {
  const activa = (p: EstadoPunto) => p.error !== 'desactivado'
  const estaciones: Origen[] = []
  const porGrupo = new Map<string, EstadoPunto[]>()
  for (const p of props.apis) if (p.grupo) porGrupo.set(p.grupo, [...(porGrupo.get(p.grupo) ?? []), p])
  for (const [grupo, lista] of [...porGrupo].sort(([a], [b]) => a.localeCompare(b))) {
    const activas = lista.filter(activa).length
    estaciones.push({ id: 'g:' + grupo, nombre: lista.length === 1 ? t('mapa.grupoUno', { grupo }) : t('mapa.grupo', { grupo, n: lista.length }),
      activo: activas > 0, principal: false, grupo, detalle: activas === 1 ? t('mapa.activaUno') : t('mapa.activas', { n: activas }) })
  }
  for (const p of props.apis) if (!p.grupo) estaciones.push({ id: p.nombre, nombre: 'API ' + p.nombre, activo: activa(p), principal: false })
  const lista: Origen[] = [{ id: '-', nombre: t('mapa.principal'), activo: true, principal: true }, ...estaciones.slice(0, MAX_APIS)]
  if (estaciones.length > MAX_APIS) {
    const resto = estaciones.slice(MAX_APIS)
    lista.push({ id: '+', nombre: resto.length === 1 ? t('mapa.masUno') : t('mapa.mas', { n: resto.length }), activo: resto.some((o) => o.activo), principal: false })
  }
  return lista.map((o, i, todos) => ({ ...o, i, y: y(i, todos.length) }))
})
// abrir lleva a Mis APIs; desde un grupo, filtrada por ese grupo.
function abrir(o: Origen) {
  if (o.grupo) grupoInicial.value = o.grupo
  emit('ir', 'conectar')
}

const lineas = computed(() => props.adaptadores.map((a, i) => ({
  agente: a.agente,
  i,
  nombre: nombreAgente(a.agente),
  listo: agenteListo(a),
  estado: a.desconectado ? t('mapa.desconectado') : agenteListo(a) ? t('mapa.enServicio') : t('mapa.sinServicio'),
  color: colorLinea(a.agente),
  y: y(i, props.adaptadores.length),
})))

const enCurso = computed(() => (props.tranvias?.length ?? 0) + (props.resto ?? 0))
const descripcion = computed(() => t('mapa.descripcion', {
  apis: origenes.value.length, listos: lineas.value.filter((l) => l.listo).length, total: lineas.value.length,
}) + (enCurso.value === 1 ? ' ' + t('vivo.enCursoUno') : enCurso.value ? ' ' + t('vivo.enCurso', { n: enCurso.value }) : ''))

// estacionDe es la estación de donde salió una petición: la de su API o su grupo; sin API (la principal, Probar, la
// terminal o la CLI), la clave del espacio.
function estacionDe(tr: Tranvia) {
  const p = tr.api ? props.apis.find((x) => x.nombre === tr.api) : undefined
  const id = !p ? '-' : p.grupo ? 'g:' + p.grupo : p.nombre
  return origenes.value.find((o) => o.id === id) ?? origenes.value.find((o) => o.id === '+') ?? origenes.value[0]
}
// TRAMOS reparte el camino completo (estación → Tramtai → línea) en porcentajes: dónde empieza cada tramo y cuánto
// ocupa. Varios tranvías en la misma línea se separan un 4 % para no taparse.
const TRAMOS = { entrada: [0, 35], centro: [35, 15], linea: [50, 50] } as const
const vivos = computed(() => {
  const porLinea = new Map<string, number>()
  return (props.tranvias ?? []).map((tr) => {
    const k = porLinea.get(tr.agente) ?? 0
    porLinea.set(tr.agente, k + 1)
    const o = estacionDe(tr)
    const a = lineas.value.find((l) => l.agente === tr.agente)
    const entrada = centro.value + paralela(o.i, origenes.value.length)
    const salida = a ? centro.value + paralela(a.i, lineas.value.length) : centro.value
    const linea = a ? tramo(X_HUB + HUB_ANCHO, salida, X_AGENTE, a.y).replace(/^M/, 'L') : ''
    const [base, peso] = TRAMOS[tr.tramo]
    return {
      tr,
      camino: `${tramo(X_ORIGEN, o.y, X_HUB, entrada)} L ${X_HUB + HUB_ANCHO} ${salida} ${linea}`,
      vCamino: `M ${V_EJE} ${vOrigen(o.i)} L ${V_EJE} ${vAgente(a?.i ?? 0)} L ${V_EJE + 34} ${vAgente(a?.i ?? 0)}`,
      distancia: Math.max(0, base + peso * (tr.tramo === 'centro' ? 0.5 : tr.avance) - 4 * k),
      color: tr.estado === 'fallido' || tr.estado === 'interrumpido' ? 'var(--error)' : tr.estado === 'cancelado' ? 'var(--borde-fuerte)' : colorLinea(tr.agente),
    }
  })
})
const nombreTranvia = (tr: Tranvia) => t('vivo.tranvia', { agente: nombreAgente(tr.agente), etapa: t(claveEtapa(tr)) })

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
