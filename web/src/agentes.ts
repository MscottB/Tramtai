// Agentes y modelos: las fichas del servicio, su color y orden, la disponibilidad y las opciones del selector de modelo.
import { computed, ref } from 'vue'
import { t } from './i18n'
import type { Clave } from './idiomas/es'
import { api, pedir } from './api/cliente'
import { numero } from './formato'
import type { Adaptador, Modelo, Trabajo } from './api/tipos'

const perfiles = ['rapido', 'equilibrado', 'profundo']

// FichaAgente es lo que el servicio sabe de cada agente (GET /api/v1/agentes): nombre, color, qué admite y dónde se
// instala. La Web UI no nombra agentes: todo lo que depende de uno sale de su ficha.
export interface FichaAgente {
  agente: string
  nombre: string
  programa: string
  codigo: string
  color: { claro: string; oscuro: string }
  orden: number
  streaming: string
  salida_estructurada: boolean
  modelo_efectivo_informado: boolean
  esfuerzos: string[]
  autenticaciones: string[]
  variables_clave: string[]
  permitido_apps: boolean
  url_instalacion: string
}

// fichas son las de los agentes del servicio, en su orden; se cargan al abrir la sesión.
export const fichas = ref<FichaAgente[]>([])
export const fichaDe = (agente?: string) => fichas.value.find((f) => f.agente === agente)

// agenteListo: se puede usar ya (un aviso, como el AGENTS.md de Codex, no lo impide).
export const agenteListo = (a: Adaptador) => !a.problema || a.problema_codigo === 'aviso'

// asegurarFichas las vuelve a pedir si faltan (si falló la petición al abrir la sesión).
export function asegurarFichas() {
  if (!fichas.value.length) cargarFichas()
}

export async function cargarFichas() {
  try {
    fichas.value = (await pedir<{ agentes: FichaAgente[] }>('GET', '/api/v1/agentes')).agentes
    pintarLineas(fichas.value)
  } catch {
    // sin fichas se ven los ids y el color neutro; se vuelven a pedir en la próxima sesión
  }
}

// pintarLineas da a cada agente su color (--linea-<id> en cada tema y la clase .linea--<id>), para que uno nuevo tenga
// el suyo sin tocar los estilos. Solo se aceptan ids y colores con forma conocida: el texto va a una hoja de estilos.
function pintarLineas(lista: FichaAgente[]) {
  const color = (c: string) => (/^#[0-9a-f]{6}$/i.test(c) ? c : 'var(--linea-otro)')
  const reglas = lista.filter((f) => /^[a-z0-9_-]+$/.test(f.agente)).map((f) => `:root{--linea-${f.agente}:${color(f.color.claro)}}` +
    `body.body--dark{--linea-${f.agente}:${color(f.color.oscuro)}}.linea--${f.agente}{--c:var(--linea-${f.agente})}`)
  let estilo = document.getElementById('tramtai-lineas')
  if (!estilo) {
    estilo = document.createElement('style')
    estilo.id = 'tramtai-lineas'
    document.head.appendChild(estilo)
  }
  estilo.textContent = reglas.join('\n')
}

// colorLinea es el color de la línea de un agente para usarlo en estilos en línea (el mapa de Inicio).
export const colorLinea = (agente: string) => (/^[a-z0-9_-]+$/.test(agente) ? `var(--linea-${agente}, var(--linea-otro))` : 'var(--linea-otro)')

// codigoLinea es el distintivo de la línea de un agente, como el número de una línea de tranvía.
export function codigoLinea(agente?: string): string {
  return fichaDe(agente)?.codigo || (agente ?? 'API').slice(0, 2).toUpperCase()
}

// ordenAgentes es el orden de los agentes en toda la Web UI: el de sus fichas.
export const ordenAgentes = computed(() => fichas.value.map((f) => f.agente))

// ordenarAgentes pone los agentes en el orden de sus fichas; los desconocidos, al final y por nombre.
export function ordenarAgentes<T extends { agente: string }>(lista: T[]): T[] {
  const orden = ordenAgentes.value
  const posicion = (a: string) => orden.indexOf(a) + 1 || orden.length + 1
  return [...lista].sort((a, b) => posicion(a.agente) - posicion(b.agente) || a.agente.localeCompare(b.agente))
}

export function nombreAgente(agente?: string): string {
  return agente ? (fichaDe(agente)?.nombre ?? agente) : '—'
}

// etiquetaModelo da el nombre legible de agente o agente/modelo: "Claude · sonnet".
export function etiquetaModelo(id: string): string {
  const [agente, ...resto] = id.split('/')
  return resto.length ? `${nombreAgente(agente)} · ${resto.join('/')}` : nombreAgente(agente)
}

export interface OpcionModelo {
  // label es el nombre completo («Claude · sonnet»); corto, el que se ve dentro del grupo de su agente («sonnet»).
  label: string
  corto: string
  caption: string
  // version es el modelo exacto que hay detrás, si se conoce (claude-sonnet-5-5).
  version: string
  // grupo es el agente, o «perfil» para los perfiles.
  grupo: string
  value: string
  predeterminado: boolean
  esfuerzos: string[]
  // exacta: una versión concreta vista en el historial; va al final de su grupo.
  exacta?: boolean
  // cabecera: el título de un grupo en la lista, que no se puede elegir.
  cabecera?: boolean
  disable?: boolean
}

// opcionModelo convierte una entrada del catálogo en una opción legible, con la versión exacta y el contexto si se
// conocen.
export function opcionModelo(m: Modelo): OpcionModelo {
  const grupo = m.perfil ? 'perfil' : (m.agente ?? m.id.split('/')[0])
  const exacta = !m.perfil && m.origen === 'visto'
  let label = etiquetaModelo(m.id)
  let corto = m.modelo ?? t('modelo.delCli')
  let version = exacta ? (m.modelo ?? '') : (m.version ?? '')
  let caption = exacta ? t('modelo.versionConcreta') : m.version || t('modelo.sinVersion')
  if (m.perfil) {
    label = corto = t('modelo.perfil', { nombre: perfiles.includes(m.perfil) ? t(`perfil.${m.perfil}` as Clave) : m.perfil })
    caption = [m.resuelto ? etiquetaModelo(m.resuelto) : t('modelo.perfilDetalle'), m.version].filter(Boolean).join(' · ')
    version = m.version ?? ''
  }
  if (m.contexto_tokens) caption += ' · ' + t('modelo.contexto', { n: numero(m.contexto_tokens) })
  return { label, corto, caption, version, grupo, value: m.id, predeterminado: !!m.predeterminado, esfuerzos: m.esfuerzos ?? [], exacta }
}

// opcionEscrita es un agente/modelo escrito a mano, que no está en el catálogo.
export function opcionEscrita(valor: string): OpcionModelo {
  const [agente, ...resto] = valor.split('/')
  return { label: etiquetaModelo(valor), corto: resto.join('/') || t('modelo.delCli'), caption: t('modelo.escrito'), version: '', grupo: agente,
    value: valor, predeterminado: false, esfuerzos: [] }
}

// problemasAgentes dice por qué no se puede usar cada agente (no instalado, sin sesión o clave…); comprobar los CLI
// tarda, así que se pide una vez por sesión de la página y se renueva cuando Agentes vuelve a comprobarlos.
export const problemasAgentes = ref<Record<string, string>>({})
let disponibilidad: Promise<void> | undefined
export function fijarDisponibilidad(adaptadores: Adaptador[]) {
  problemasAgentes.value = Object.fromEntries(adaptadores.filter((a) => a.problema && a.problema_codigo !== 'aviso').map((a) => [a.agente,
    a.problema_codigo === 'no_instalado' ? t('modelo.noInstalado') : a.problema_codigo === 'falta_clave' ? t('modelo.faltaClave') : t('modelo.conProblema', { problema: a.problema ?? '' })]))
}
export function cargarDisponibilidad() {
  disponibilidad ??= api.capacidades().then((r) => fijarDisponibilidad(r.adaptadores)).catch(() => {
    disponibilidad = undefined // sin respuesta no se marca nada: se vuelve a pedir la próxima vez
  })
  return disponibilidad
}

// esfuerzoEspacio es el esfuerzo que el espacio usa con su modelo predeterminado ('' si usa el de su agente).
export const esfuerzoEspacio = (catalogo: Modelo[]) => catalogo.find((m) => m.predeterminado)?.esfuerzo_espacio ?? ''

// agruparOpciones ordena las opciones por grupo (los perfiles, los agentes en el orden de la red y al final los que no
// se pueden usar) con un título por grupo; dentro, primero el modelo de su CLI, luego los configurados y al final las
// versiones vistas.
export function agruparOpciones(lista: OpcionModelo[]): OpcionModelo[] {
  const apagado = (g: string) => lista.filter((o) => o.grupo === g).every((o) => o.disable)
  const orden = ordenAgentes.value
  const lugar = (g: string) => (apagado(g) ? 100 : 0) + (g === 'perfil' ? -1 : orden.includes(g) ? orden.indexOf(g) : orden.length)
  const rango = (o: OpcionModelo) => (o.grupo === 'perfil' ? 1 : !o.value.includes('/') ? 0 : o.exacta ? 2 : 1)
  const grupos = [...new Set(lista.map((o) => o.grupo))].sort((a, b) => lugar(a) - lugar(b) || a.localeCompare(b))
  return grupos.flatMap((g) => [
    { label: (g === 'perfil' ? t('conectar.perfiles') : nombreAgente(g)) + (apagado(g) ? ' · ' + t('modelo.noDisponible') : ''), corto: '', caption: '', version: '', grupo: g, value: '#' + g,
      predeterminado: false, esfuerzos: [], cabecera: true, disable: true },
    ...lista.filter((o) => o.grupo === g).sort((a, b) => rango(a) - rango(b) || a.corto.localeCompare(b.corto)),
  ])
}

// esfuerzosDe son los niveles de esfuerzo del modelo elegido ('' es el predeterminado del espacio); uno escrito a
// mano que no está en el catálogo tiene los de su agente.
export function esfuerzosDe(catalogo: Modelo[], seleccion: string | null): string[] {
  return entradaCatalogo(catalogo, seleccion)?.esfuerzos ?? []
}

// esfuerzoPredeterminadoDe es el nivel que usará el agente con el modelo elegido si no se indica otro ('' si no se sabe).
export function esfuerzoPredeterminadoDe(catalogo: Modelo[], seleccion: string | null): string {
  // Solo la entrada exacta: el predeterminado depende del modelo, y la del agente lleva el de otro (el de su CLI).
  const m = seleccion ? catalogo.find((x) => x.id === seleccion) : catalogo.find((x) => x.predeterminado)
  return m?.esfuerzo_predeterminado ?? ''
}

// entradaCatalogo es la entrada del catálogo elegida (o la predeterminada); un id escrito a mano cae en su agente.
function entradaCatalogo(catalogo: Modelo[], seleccion: string | null): Modelo | undefined {
  return seleccion
    ? (catalogo.find((x) => x.id === seleccion) ?? catalogo.find((x) => x.id === seleccion.split('/')[0]))
    : catalogo.find((x) => x.predeterminado)
}

export function modeloDe(trabajo: Trabajo): string {
  const modelo = trabajo.modelo_efectivo || trabajo.modelo_enviado
  return [nombreAgente(trabajo.solicitud.agente), modelo].filter((x) => x && x !== '—').join(' · ') || '—'
}
