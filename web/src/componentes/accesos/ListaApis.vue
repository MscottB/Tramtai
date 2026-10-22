<template>
  <q-card-section>
    <h2 class="titulo-seccion">{{ t('puntos.titulo') }}</h2>
    <div class="texto-suave text-body2">{{ t('puntos.texto') }}</div>
  </q-card-section>
  <q-card-section v-if="puntos.length > 1 || hayFiltro" class="row q-col-gutter-sm q-pt-none">
    <div class="col-12 col-md">
      <q-input v-model="busqueda" dense outlined clearable debounce="150" :placeholder="t('puntos.buscarApi')" :aria-label="t('puntos.buscarApi')">
        <template #prepend><q-icon name="search" /></template>
      </q-input>
    </div>
    <div v-if="hayGrupos" class="col-6 col-sm-3 col-md-2">
      <q-select v-model="filtroGrupo" :options="opcionesGrupo" emit-value map-options dense outlined :label="t('puntos.filtroGrupo')" />
    </div>
    <div class="col-6 col-sm-3 col-md-2">
      <q-select v-model="filtroEstado" :options="opcionesEstado" emit-value map-options dense outlined :label="t('puntos.filtroEstado')" />
    </div>
    <div v-if="opcionesAgente.length > 2" class="col-6 col-sm-3 col-md-2">
      <q-select v-model="filtroAgente" :options="opcionesAgente" emit-value map-options dense outlined :label="t('puntos.filtroAgente')" />
    </div>
    <div class="col-6 col-sm-3 col-md-2">
      <q-select v-model="orden" :options="opcionesOrden" emit-value map-options dense outlined :label="t('puntos.orden')" />
    </div>
    <div class="col-12">
      <q-btn flat dense no-caps color="primary" :icon="masFiltros ? 'expand_less' : 'o_filter_list'" :aria-expanded="masFiltros"
        :label="filtrosExtra ? t('puntos.masFiltrosN', { n: filtrosExtra }) : t('puntos.masFiltros')" @click="masFiltros = !masFiltros" />
      <q-btn v-if="hayFiltro" flat dense no-caps color="neutro" icon="o_filter_alt_off" :label="t('puntos.limpiarFiltros')" class="q-ml-sm" @click="limpiarFiltros" />
    </div>
    <template v-if="masFiltros">
      <div class="col-12 col-sm-4">
        <q-select v-model="filtroContrato" :options="opcionesContrato" emit-value map-options dense outlined :label="t('puntos.filtroContrato')" />
      </div>
      <div class="col-12 col-sm-5">
        <q-select v-model="filtroCaracteristicas" :options="opcionesCaracteristicas" emit-value map-options multiple use-chips dense outlined
          :label="t('puntos.filtroCaracteristicas')" :hint="t('puntos.filtroCaracteristicasHint')" />
      </div>
      <div class="col-12 col-sm-3">
        <q-select v-model="filtroUso" :options="opcionesUso" emit-value map-options dense outlined :label="t('puntos.filtroUso')" />
      </div>
    </template>
  </q-card-section>
  <q-separator />
  <template v-if="puntos.length">
    <template v-for="f in filasPagina">
      <div v-if="f.tipo === 'grupo'" :key="'g:' + f.g.nombre" class="row items-center q-px-md q-pt-md q-pb-xs cabecera-grupo">
        <q-btn flat dense round size="sm" :icon="plegados.has(f.g.nombre) ? 'chevron_right' : 'expand_more'" class="q-mr-xs"
          :aria-expanded="!plegados.has(f.g.nombre)" :aria-label="t(plegados.has(f.g.nombre) ? 'puntos.desplegar' : 'puntos.plegar', { grupo: f.g.nombre || t('puntos.sinGrupo') })"
          @click="plegar(f.g.nombre)" />
        <q-icon :name="f.g.nombre ? 'o_folder' : 'o_folder_off'" class="q-mr-sm texto-suave" />
        <div class="text-weight-medium cursor-pointer" @click="plegar(f.g.nombre)">{{ f.g.nombre || t('puntos.sinGrupo') }}<span v-if="f.sigue" class="texto-suave text-caption q-ml-xs">{{ t('puntos.sigue') }}</span></div>
        <div class="text-caption texto-suave q-ml-sm">{{ t('puntos.activos', { activos: f.g.activos, total: f.g.total }) }}</div>
        <q-space />
        <q-btn flat dense no-caps size="sm" color="primary" icon="o_play_arrow" :label="$q.screen.gt.xs ? t('puntos.activarTodos') : undefined"
          :aria-label="t('puntos.activarTodos')" :disable="f.g.activos === f.g.total" :loading="ocupado === f.g.nombre + ':1'" @click="emit('activarGrupo', f.g.nombre, true)">
          <q-tooltip v-if="$q.screen.xs">{{ t('puntos.activarTodos') }}</q-tooltip>
        </q-btn>
        <q-btn flat dense no-caps size="sm" color="neutro" icon="o_stop" :label="$q.screen.gt.xs ? t('puntos.desactivarTodos') : undefined"
          :aria-label="t('puntos.desactivarTodos')" :disable="f.g.activos === 0" :loading="ocupado === f.g.nombre + ':0'" @click="emit('activarGrupo', f.g.nombre, false)">
          <q-tooltip v-if="$q.screen.xs">{{ t('puntos.desactivarTodos') }}</q-tooltip>
        </q-btn>
      </div>
    <div v-else :key="'a:' + f.p.nombre" class="fila-api cursor-pointer" @click="emit('abrir', f.p)">
      <span class="linea__chapa linea__chapa--grande" :class="['linea--' + (f.p.agente ?? 'otro'), { 'linea__chapa--apagada': !activo(f.p) }]" aria-hidden="true">{{ f.p.agente ? codigoLinea(f.p.agente) : 'API' }}</span>
      <div class="fila-api__info">
        <div class="row items-center q-gutter-x-sm">
          <span class="text-weight-medium">{{ f.p.nombre }}</span>
          <span class="etiqueta" :class="!activo(f.p) ? 'etiqueta--neutro' : f.p.error ? 'etiqueta--error' : 'etiqueta--ok'">{{ activo(f.p) ? t('puntos.activo') : t('puntos.desactivado') }}</span>
          <span v-if="f.p.esquema" class="etiqueta etiqueta--info">{{ t('puntos.json') }}</span>
          <span v-if="f.p.esquema && f.p.reintentos_salida" class="etiqueta etiqueta--neutro">{{ t('puntos.correcciones', { n: f.p.reintentos_salida }) }}</span>
          <span v-if="f.p.respaldo?.length" class="etiqueta etiqueta--neutro">{{ t('puntos.respaldoDe', { agentes: f.p.respaldo.map(etiquetaDe).join(', ') }) }}</span>
        </div>
        <div class="row items-center no-wrap codigo texto-suave">
          <span class="ellipsis">{{ f.p.url || t('puntos.sinDireccion') }}</span>
          <q-btn v-if="f.p.url" flat dense round size="sm" icon="content_copy" :aria-label="t('puntos.copiarUrl')" @click.stop="copiar(f.p.url)"><q-tooltip>{{ t('puntos.copiarUrl') }}</q-tooltip></q-btn>
        </div>
        <div class="text-caption texto-suave">
          <template v-if="idModelo(f.p)">{{ etiquetaDe(idModelo(f.p)!) }} · </template>{{ f.p.contratos.map(nombreContrato).join(' · ') }}
          <template v-if="f.p.esfuerzo"> · {{ t('esfuerzo.titulo') }} {{ nombreEsfuerzo(f.p.esfuerzo).toLowerCase() }}</template>
        </div>
        <div v-if="f.p.instrucciones" class="text-caption texto-suave row no-wrap items-start">
          <q-icon name="o_notes" class="q-mr-xs q-mt-xs" /><span class="recortar-2">{{ f.p.instrucciones }}</span>
        </div>
        <div v-if="f.p.error && f.p.error !== 'desactivado'" class="text-caption text-negative">{{ f.p.error }}</div>
        <div v-if="soloDesconectados(f.p)" class="text-caption text-warning">{{ t('puntos.soloDesconectados') }}</div>
      </div>
      <div class="fila-api__uso text-caption">
        <a v-if="uso[f.p.nombre]" href="#historial" class="enlace" @click.prevent.stop="emit('verPeticiones', f.p.nombre)">{{ textoUso(f.p.nombre) }}</a>
        <span v-else class="texto-suave">{{ t('puntos.sinUso') }}</span>
        <BarrasPresupuesto v-if="presupuestos[f.p.nombre]" :estado="presupuestos[f.p.nombre]" sin-boton />
      </div>
      <div class="fila-api__acciones row items-center no-wrap q-gutter-x-xs">
        <q-btn unelevated dense no-caps color="primary" icon="o_cable" class="q-px-sm" :label="t('puntos.conectar')" @click.stop="emit('abrir', f.p)" />
      </div>
    </div>
    </template>
    <div v-if="totalPaginas > 1" class="row items-center justify-between q-px-md q-py-sm q-gutter-sm">
      <span class="text-caption texto-suave">{{ t('puntos.mostrando', { desde: (pagina - 1) * porPagina + 1, hasta: Math.min(pagina * porPagina, apisVisibles), total: apisVisibles }) }}</span>
      <q-pagination v-model="pagina" :max="totalPaginas" :max-pages="6" direction-links boundary-links dense color="primary" />
    </div>
  </template>
  <div v-if="puntos.length && !grupos.length" class="q-pa-md texto-suave">{{ t('puntos.sinResultados') }}</div>
  <EstadoVacio v-if="!puntos.length" :titulo="t('puntos.vacio')" :accion="t('puntos.nuevo')" @accion="emit('nuevo')" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useQuasar } from 'quasar'
import BarrasPresupuesto from '../BarrasPresupuesto.vue'
import EstadoVacio from '../EstadoVacio.vue'
import { copiar } from '../../api/cliente'
import type { EstadoPresupuesto, EstadoPunto, UsoPunto } from '../../api/tipos'
import { periodoHistorial } from '../../estado'
import { codigoLinea, etiquetaModelo, nombreAgente, ordenarAgentes, soloDesconectados, type OpcionModelo } from '../../agentes'
import { dividirRespaldo, idModeloDe as idModelo } from '../../recetas'
import { haceCuanto, nombreEsfuerzo } from '../../formato'
import { t } from '../../i18n'
import { activo, contratos, guardar, leer, nombreContrato } from './comun'

const props = defineProps<{ puntos: EstadoPunto[]; uso: Record<string, UsoPunto>; presupuestos: Record<string, EstadoPresupuesto>; opciones: OpcionModelo[];
  ocupado: string }>()
const emit = defineEmits<{ abrir: [p: EstadoPunto]; nuevo: []; activarGrupo: [grupo: string, valor: boolean]; verPeticiones: [nombre: string];
  grupos: [nombres: string[]] }>()
const filtroGrupo = defineModel<string | null>('filtroGrupo', { required: true }) // null: todos; '': sin grupo
const $q = useQuasar()

const busqueda = ref<string | null>('')
const orden = ref<'nombre' | 'uso' | 'ultima'>(leer('tramtai.ordenApis', 'nombre'))
watch(orden, (v) => guardar('tramtai.ordenApis', v))
const plegados = ref(new Set<string>(leer<string[]>('tramtai.gruposPlegados', [])))
function plegar(grupo: string) {
  const nuevos = new Set(plegados.value)
  if (!nuevos.delete(grupo)) nuevos.add(grupo)
  plegados.value = nuevos
  guardar('tramtai.gruposPlegados', [...nuevos])
}
const opcionesOrden = computed(() => [
  { label: t('puntos.ordenNombre'), value: 'nombre' }, { label: t('puntos.ordenUso'), value: 'uso' }, { label: t('puntos.ordenUltima'), value: 'ultima' },
])
const comparar = (a: EstadoPunto, b: EstadoPunto) => {
  if (orden.value === 'uso') return (props.uso[b.nombre]?.peticiones ?? 0) - (props.uso[a.nombre]?.peticiones ?? 0) || a.nombre.localeCompare(b.nombre)
  if (orden.value === 'ultima') return (props.uso[b.nombre]?.ultima ?? '').localeCompare(props.uso[a.nombre]?.ultima ?? '') || a.nombre.localeCompare(b.nombre)
  return a.nombre.localeCompare(b.nombre)
}
// grupos ordena los puntos por grupo (los que no tienen, al final), aplica el buscador, el grupo elegido y el orden;
// activos y total cuentan el grupo entero, porque activar o detener todas actúa sobre todo el grupo.
// Filtros: el buscador (nombre, grupo, modelo e instrucciones), el grupo, el estado y el agente, y en «Más filtros» lo
// que acepta (contrato), lo que tiene configurado (características, todas a la vez) y si tiene peticiones; pasa dice si
// una API los cumple todos.
const filtroEstado = ref<'todas' | 'activas' | 'detenidas' | 'error'>('todas')
const filtroAgente = ref<string | null>(null)
const filtroContrato = ref<string | null>(null)
const filtroCaracteristicas = ref<string[]>([])
const filtroUso = ref<'todas' | 'con' | 'sin'>('todas')
const masFiltros = ref(false)
const filtrosExtra = computed(() => Number(filtroContrato.value !== null) + filtroCaracteristicas.value.length + Number(filtroUso.value !== 'todas'))
const hayFiltro = computed(() => !!busqueda.value || filtroGrupo.value !== null || filtroEstado.value !== 'todas' || filtroAgente.value !== null ||
  filtrosExtra.value > 0)
function limpiarFiltros() {
  busqueda.value = ''
  filtroGrupo.value = null
  filtroEstado.value = 'todas'
  filtroAgente.value = null
  filtroContrato.value = null
  filtroCaracteristicas.value = []
  filtroUso.value = 'todas'
}
const caracteristicas: Record<string, (p: EstadoPunto) => boolean> = {
  json: (p) => !!p.esquema,
  respaldo: (p) => !!p.respaldo?.length,
  presupuesto: (p) => !!props.presupuestos[p.nombre],
  instrucciones: (p) => !!p.instrucciones?.trim(),
  forzado: (p) => !!p.forzar_modelo,
  restringida: (p) => !!p.modelos_permitidos?.length,
}
function pasa(p: EstadoPunto) {
  const texto = (busqueda.value ?? '').trim().toLowerCase()
  const estado = !activo(p) ? 'detenidas' : p.error ? 'error' : 'activas'
  const buscable = [p.nombre, p.grupo, idModelo(p), p.instrucciones].filter(Boolean).join(' ').toLowerCase()
  return (!texto || buscable.includes(texto)) && (filtroEstado.value === 'todas' || filtroEstado.value === estado) &&
    (filtroAgente.value === null || (p.agente ?? '') === filtroAgente.value) &&
    (filtroContrato.value === null || p.contratos.includes(filtroContrato.value)) &&
    filtroCaracteristicas.value.every((c) => caracteristicas[c]?.(p)) &&
    (filtroUso.value === 'todas' || (filtroUso.value === 'con') === !!props.uso[p.nombre]?.peticiones)
}
const opcionesContrato = computed(() => [{ label: t('puntos.contratosTodos'), value: null }, ...contratos.map((value) => ({ label: nombreContrato(value), value }))])
const opcionesCaracteristicas = computed(() => [
  { label: t('puntos.carJson'), value: 'json' }, { label: t('puntos.carRespaldo'), value: 'respaldo' },
  { label: t('puntos.carPresupuesto'), value: 'presupuesto' }, { label: t('puntos.carInstrucciones'), value: 'instrucciones' },
  { label: t('puntos.carForzado'), value: 'forzado' }, { label: t('puntos.carRestringida'), value: 'restringida' },
])
const opcionesUso = computed(() => [
  { label: t('puntos.usoTodas'), value: 'todas' }, { label: t('puntos.usoCon'), value: 'con' }, { label: t('puntos.usoSin'), value: 'sin' },
])
const opcionesEstado = computed(() => [
  { label: t('puntos.estadoTodas'), value: 'todas' }, { label: t('puntos.estadoActivas'), value: 'activas' },
  { label: t('puntos.estadoDetenidas'), value: 'detenidas' }, { label: t('puntos.estadoError'), value: 'error' },
])
// opcionesAgente son los agentes que usan las APIs (sin agente propio, el predeterminado del espacio).
const opcionesAgente = computed(() => [
  { label: t('puntos.agentesTodos'), value: null },
  ...ordenarAgentes([...new Set(props.puntos.map((p) => p.agente ?? ''))].map((agente) => ({ agente })))
    .map(({ agente }) => ({ label: agente ? nombreAgente(agente) : t('puntos.agentePredeterminado'), value: agente })),
])
const grupos = computed(() => {
  const mapa = new Map<string, EstadoPunto[]>()
  for (const p of props.puntos) mapa.set(p.grupo ?? '', [...(mapa.get(p.grupo ?? '') ?? []), p])
  return [...mapa.entries()]
    .sort(([a], [b]) => (!a ? 1 : !b ? -1 : a.localeCompare(b)))
    .filter(([nombre]) => filtroGrupo.value === null || filtroGrupo.value === nombre)
    .map(([nombre, lista]) => ({ nombre, total: lista.length, activos: lista.filter(activo).length, puntos: lista.filter(pasa).sort(comparar) }))
    .filter((g) => g.puntos.length)
})

// Páginas de 20 APIs. Las filas son las cabeceras de grupo y las APIs de los grupos desplegados; una página que empieza
// en medio de un grupo repite su cabecera, y los grupos plegados del final se ven en la última página.
type Grupo = (typeof grupos.value)[number]
type Fila = { tipo: 'grupo'; g: Grupo; sigue: boolean } | { tipo: 'api'; p: EstadoPunto }
const porPagina = 20
const pagina = ref(1)
const filas = computed<Fila[]>(() => grupos.value.flatMap((g): Fila[] => [
  ...(hayGrupos.value ? [{ tipo: 'grupo' as const, g, sigue: false }] : []),
  ...(!hayGrupos.value || !plegados.value.has(g.nombre) ? g.puntos.map((p) => ({ tipo: 'api' as const, p })) : []),
]))
const apisVisibles = computed(() => filas.value.filter((f) => f.tipo === 'api').length)
const totalPaginas = computed(() => Math.max(1, Math.ceil(apisVisibles.value / porPagina)))
const filasPagina = computed(() => {
  const desde = (pagina.value - 1) * porPagina
  const hasta = desde + porPagina
  const ultima = pagina.value === totalPaginas.value
  const lista: Fila[] = []
  let n = 0
  let grupo: Grupo | undefined
  let grupoPuesto = false
  for (const f of filas.value) {
    if (f.tipo === 'grupo') {
      grupo = f.g
      grupoPuesto = n >= desde && (n < hasta || ultima)
      if (grupoPuesto) lista.push(f)
      continue
    }
    if (n >= desde && n < hasta) {
      if (grupo && !grupoPuesto) {
        lista.push({ tipo: 'grupo', g: grupo, sigue: true })
        grupoPuesto = true
      }
      lista.push(f)
    }
    n++
  }
  return lista
})
watch([busqueda, filtroGrupo, filtroEstado, filtroAgente, filtroContrato, filtroCaracteristicas, filtroUso, orden], () => (pagina.value = 1))
watch(totalPaginas, (n) => pagina.value > n && (pagina.value = n))
const hayGrupos = computed(() => props.puntos.some((p) => p.grupo))
const opcionesGrupo = computed(() => [
  { label: t('puntos.todosGrupos'), value: null },
  ...[...new Set(props.puntos.map((p) => p.grupo).filter((g): g is string => !!g))].sort().map((g) => ({ label: g, value: g })),
  { label: t('puntos.sinGrupo'), value: '' },
])
// etiquetaDe da el nombre legible de un modelo o de una entrada de respaldo con su esfuerzo propio («Codex · alto»).
const etiquetaDe = (id: string) => {
  const { base, esfuerzo } = dividirRespaldo(id)
  const nombre = props.opciones.find((o) => o.value === base)?.label ?? etiquetaModelo(base)
  return esfuerzo ? `${nombre} · ${nombreEsfuerzo(esfuerzo).toLowerCase()}` : nombre
}
function textoUso(nombre: string) {
  const u = props.uso[nombre]
  return [u.peticiones === 1 ? t('puntos.usoUno', { periodo: periodoHistorial.value }) : t('puntos.uso', { n: u.peticiones, periodo: periodoHistorial.value }), u.errores ? t('puntos.usoErrores', { n: u.errores }) : '',
    u.ultima ? t('puntos.usoUltima', { cuando: haceCuanto(u.ultima) }) : ''].filter(Boolean).join(' · ')
}

// Los grupos que se ven ahora son las sugerencias del campo Grupo al crear una API.
watch(() => grupos.value.map((g) => g.nombre).filter(Boolean), (nombres) => emit('grupos', nombres), { immediate: true })
</script>
