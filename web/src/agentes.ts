// Agentes y modelos: las fichas del servicio, su color y orden, la disponibilidad y las opciones del selector de modelo.
import { computed, ref } from 'vue'
import { t } from './i18n'
import type { Clave } from './idiomas/es'
import { api, pedir } from './api/cliente'
import { numero, sinDato } from './formato'
import type { Adaptador, ConfigPunto, Modelo, Trabajo } from './api/tipos'

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
  // desconectado: el usuario lo desconectó en Agentes (no se usa ni como respaldo).
  desconectado?: boolean
}

// fichas son las de los agentes del servicio, en su orden; se cargan al abrir la sesión.
export const fichas = ref<FichaAgente[]>([])
export const fichaDe = (agente?: string) => fichas.value.find((f) => f.agente === agente)

// agenteListo: se puede usar ya (un aviso, como el AGENTS.md de Codex, no lo impide); uno desconectado no.
export const agenteListo = (a: Adaptador) => !a.desconectado && (!a.problema || a.problema_codigo === 'aviso')

// EstadoAgente es lo que enseña la tarjeta de un agente en Agentes.
export type EstadoAgente = 'listo' | 'desconectado' | 'noInstalado' | 'faltaClave' | 'sinSesion' | 'usaClave' | 'sesionNoResponde' | 'revisar'

// estadoAgente clasifica un agente por su conexión y el código de su problema (el texto llega en el idioma del
// servicio). Con suscripción, «falta la clave» es que su CLI no tiene sesión.
export function estadoAgente(a: Adaptador): EstadoAgente {
  if (a.desconectado) return 'desconectado'
  if (!a.problema || a.problema_codigo === 'aviso') return 'listo'
  switch (a.problema_codigo) {
    case 'no_instalado':
      return 'noInstalado'
    case 'falta_clave':
      return a.autenticacion === 'suscripcion' ? 'sinSesion' : 'faltaClave'
    case 'usa_clave_api':
      return 'usaClave'
    case 'sesion_no_responde':
      return 'sesionNoResponde'
  }
  return 'revisar'
}

// ofrecerDejarSuscripcion: «Dejar de usar la suscripción» se ofrece en cualquier estado si el agente está en
// suscripción y su ficha admite las dos formas.
export const ofrecerDejarSuscripcion = (a: Adaptador, admiteLasDos: boolean) => admiteLasDos && a.autenticacion === 'suscripcion'

// soloDesconectados: la API solo puede usar agentes desconectados (su agente, sus modelos permitidos y su respaldo);
// una API sin agente propio usa el predeterminado del espacio y no cuenta.
export function soloDesconectados(p: ConfigPunto): boolean {
  // Un perfil («perfil/rapido») no es un agente: lo resuelve el servicio y no cuenta.
  const agentes = new Set([p.agente, ...(p.modelos_permitidos ?? []), ...(p.respaldo ?? [])]
    .filter((x): x is string => !!x && !x.startsWith('perfil/')).map((x) => x.split(/[/+]/)[0]))
  return agentes.size > 0 && [...agentes].every((a) => fichaDe(a)?.desconectado)
}

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

// luminancia relativa de un color #rrggbb, con la fórmula de WCAG 2.x.
function luminancia(color: string) {
  const [r, g, b] = [1, 3, 5].map((i) => parseInt(color.slice(i, i + 2), 16) / 255).map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

// contraste entre dos colores #rrggbb (de 1 a 21).
export function contraste(a: string, b: string) {
  const [claro, oscuro] = [luminancia(a), luminancia(b)].sort((x, y) => y - x)
  return (claro + 0.05) / (oscuro + 0.05)
}

// mezclar acerca el color a otro en la proporción peso (0 a 1).
function mezclar(color: string, hacia: string, peso: number) {
  return '#' + [1, 3, 5].map((i) => {
    const [a, b] = [color, hacia].map((c) => parseInt(c.slice(i, i + 2), 16))
    return Math.round(a + (b - a) * peso).toString(16).padStart(2, '0')
  }).join('')
}

const BLANCO = '#ffffff'
const TINTA = '#0a1116'

// chapaDe da el fondo y el texto de la chapa de una línea: el texto que más contrasta con el color de la ficha y, si
// aun así no llega a 4,5:1 (el naranja de Claude con blanco), el color oscurecido o aclarado lo justo.
export function chapaDe(color: string) {
  const texto = contraste(color, BLANCO) >= contraste(color, TINTA) ? BLANCO : TINTA
  let fondo = color
  for (let paso = 1; contraste(fondo, texto) < 4.5 && paso <= 20; paso++) fondo = mezclar(color, texto === BLANCO ? '#000000' : BLANCO, paso * 0.05)
  return { fondo, texto }
}

// pintarLineas da a cada agente su color (--linea-<id> en cada tema y la clase .linea--<id>) y el de su chapa, para que
// uno nuevo tenga el suyo sin tocar los estilos. Solo se aceptan ids y colores con forma conocida: el texto va a una
// hoja de estilos.
function pintarLineas(lista: FichaAgente[]) {
  const tema = (id: string, c: string) => {
    if (!/^#[0-9a-f]{6}$/i.test(c)) return `--linea-${id}:var(--linea-otro)`
    const chapa = chapaDe(c)
    return `--linea-${id}:${c};--chapa-${id}:${chapa.fondo};--chapa-texto-${id}:${chapa.texto}`
  }
  const reglas = lista.filter((f) => /^[a-z0-9_-]+$/.test(f.agente)).map((f) => `:root{${tema(f.agente, f.color.claro)}}` +
    `body.body--dark{${tema(f.agente, f.color.oscuro)}}` +
    `.linea--${f.agente}{--c:var(--linea-${f.agente});--c-chapa:var(--chapa-${f.agente},var(--c));--c-chapa-texto:var(--chapa-texto-${f.agente})}`)
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
  return agente ? (fichaDe(agente)?.nombre ?? agente) : sinDato()
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
  problemasAgentes.value = Object.fromEntries(adaptadores.filter((a) => !agenteListo(a)).map((a) => [a.agente,
    a.desconectado ? t('modelo.desconectado') : a.problema_codigo === 'no_instalado' ? t('modelo.noInstalado')
      : a.problema_codigo === 'falta_clave' ? t('modelo.faltaClave') : t('modelo.conProblema', { problema: a.problema ?? '' })]))
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
  const agente = trabajo.solicitud.agente
  return [agente && nombreAgente(agente), trabajo.modelo_efectivo || trabajo.modelo_enviado].filter(Boolean).join(' · ') || sinDato()
}
