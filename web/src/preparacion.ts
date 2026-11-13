// preparacion: instalar, actualizar, desinstalar e iniciar sesión en los CLI de los agentes. Qué acción ofrece cada
// tarjeta (según su estado y GET /api/v1/agentes/versiones) y el bucle que lanza y sigue las preparaciones, sin Vue para
// poder probarlo.
import type { Adaptador, EstadoPreparacion, PedidoPreparacion, VersionesAgente } from './api/tipos'
import { enlaceSeguro } from './enlaces'
import { fichaDe, nombreAgente } from './agentes'
import { t } from './i18n'
import type { Clave } from './idiomas/es'

export type Accion = PedidoPreparacion['accion']

// enMarcha: la preparación aún no terminó (listo, error y cancelado son finales).
export const enMarcha = (e?: EstadoPreparacion | null) => !!e && !['listo', 'error', 'cancelado'].includes(e.estado)

// listoParaUsar: terminó bien y sin nada pendiente. Un listo con motivo (instalado, pero sin iniciar sesión porque la
// versión no está comprobada) no cuenta.
export const listoParaUsar = (e?: EstadoPreparacion | null) => e?.estado === 'listo' && !e.motivo_codigo

// seguirPreparacion consulta el estado hasta que termina, como la del conversor; esperar separa las consultas.
export async function seguirPreparacion(obtener: () => Promise<EstadoPreparacion | null>, alCambiar: (e: EstadoPreparacion | null) => void,
  esperar: () => Promise<void> = () => new Promise((listo) => setTimeout(listo, 1500))): Promise<EstadoPreparacion | null> {
  let e = await obtener()
  alCambiar(e)
  while (enMarcha(e)) {
    await esperar()
    e = await obtener()
    alCambiar(e)
  }
  return e
}

// conReintentos repite una consulta que falla (la red, un reinicio del servicio) hasta veces en total, con esperar entre
// medias; si sigue fallando, da el último error.
export async function conReintentos<T>(consultar: () => Promise<T>, esperar: () => Promise<void>, veces = 3): Promise<T> {
  for (let i = 1; ; i++) {
    try {
      return await consultar()
    } catch (e) {
      if (i >= veces) throw e
      await esperar()
    }
  }
}

// Orden es lo que se pide a un agente: sin accion, solo se sigue la que haya.
export interface Orden {
  accion?: Accion
  borrar_datos?: boolean
  confirmar?: boolean
}

// Corrida es lo que necesita el bucle: las rutas, cómo esperar entre consultas, si la pantalla sigue ahí y a quién avisar.
export interface Corrida {
  api: {
    estado: (agente: string) => Promise<EstadoPreparacion | null>
    preparar: (agente: string, pedido: PedidoPreparacion) => Promise<EstadoPreparacion>
  }
  esperar: () => Promise<void>
  vivo: () => boolean
  // empieza: el agente pasa a ser el de esta orden; lanzada: el servicio la aceptó (202, o 409 con la que ya estaba en
  // marcha), así que repetirla ya no toca; cambio: un estado nuevo; fallo: no se pudo pedir ni consultar; fin: terminó.
  empieza?: (agente: string, orden: Orden) => void
  lanzada?: (agente: string) => void
  cambio: (agente: string, e: EstadoPreparacion | null) => void
  fallo: (agente: string, error: unknown) => void
  fin?: (agente: string, e: EstadoPreparacion | null) => void
}

class Cortada extends Error {}

// correrAgente lanza la orden (o, sin acción, mira la que haya) y la sigue hasta que termina. Con otra en marcha para el
// mismo agente (la app y la Web UI a la vez) sigue esa. Devuelve el estado final, o null si falló o se cortó.
export async function correrAgente(c: Corrida, agente: string, orden: Orden): Promise<EstadoPreparacion | null> {
  c.empieza?.(agente, orden)
  const esperar = async () => {
    await c.esperar()
    if (!c.vivo()) throw new Cortada()
  }
  let primera: EstadoPreparacion | null | undefined
  try {
    if (orden.accion) {
      try {
        primera = await c.api.preparar(agente, { accion: orden.accion, borrar_datos: !!orden.borrar_datos, confirmar: !!orden.confirmar })
      } catch (e) {
        const enCurso = (e as { codigo?: string; datos?: { preparacion?: EstadoPreparacion } })
        if (enCurso?.codigo !== 'ya_en_curso' || !enCurso.datos?.preparacion) throw e
        primera = enCurso.datos.preparacion
      }
      c.lanzada?.(agente)
    }
    return await seguirPreparacion(async () => {
      if (primera === undefined) return conReintentos(() => c.api.estado(agente), esperar)
      const e = primera
      primera = undefined
      return e
    }, (e) => c.vivo() && c.cambio(agente, e), esperar)
  } catch (e) {
    if (c.vivo() && !(e instanceof Cortada)) c.fallo(agente, e)
    return null
  }
}

// prepararAgentes recorre los agentes desde uno: el primero con su orden y los demás con resto. Se para en el primero que
// no termina en listo («Seguir» vuelve a empezar desde él). 'listo' si llegaron todos; 'cortado' si la pantalla se fue.
export async function prepararAgentes(c: Corrida, agentes: string[], desde: number, primera: Orden, resto: Orden): Promise<'listo' | 'parado' | 'cortado'> {
  for (let i = desde; i < agentes.length; i++) {
    const e = await correrAgente(c, agentes[i], i === desde ? primera : resto)
    if (!c.vivo()) return 'cortado'
    c.fin?.(agentes[i], e)
    if (e?.estado !== 'listo') return 'parado'
  }
  return 'listo'
}

// ordenSeguir repite, con confirmar, la acción que se paró por una versión sin comprobar: la de su estado (quien muestra
// el panel puede haberlo pasado a solo mirar).
export const ordenSeguir = (e: EstadoPreparacion, borrarDatos = false): Orden => ({ accion: e.accion, borrar_datos: borrarDatos, confirmar: true })

// masNueva compara dos versiones por sus números («2.10.0» es más nueva que «2.9.9»).
export function masNueva(a: string, b: string): boolean {
  const [x, y] = [a, b].map((v) => v.split('.').map((n) => parseInt(n, 10) || 0))
  for (let i = 0; i < Math.max(x.length, y.length); i++) if ((x[i] ?? 0) !== (y[i] ?? 0)) return (x[i] ?? 0) > (y[i] ?? 0)
  return false
}

// versionNueva es la última publicada si es más nueva que la instalada ('' si no la hay o no se sabe).
export const versionNueva = (v?: VersionesAgente) => (v?.instalada && v.ultima && masNueva(v.ultima, v.instalada) ? v.ultima : '')

// conSesion: el agente inicia sesión con su CLI (su ficha admite la suscripción); si no, va con clave (Antigravity).
export const conSesion = (agente: string) => !!fichaDe(agente)?.autenticaciones.includes('suscripcion')

export interface Principal {
  accion?: Accion
  etiqueta?: Clave
  // aviso: la versión que usaría no está comprobada.
  aviso?: string
}

// avisoNoComprobada es el aviso de la tarjeta para una acción con la versión sin comprobar (para iniciar sesión, el
// motivo del servicio, que trae la orden para la consola).
export function avisoNoComprobada(accion: Accion, agente: string, v?: VersionesAgente): string {
  if (accion === 'iniciar_sesion' && v?.motivo_codigo === 'version_no_comprobada' && v.motivo) return v.motivo
  if (accion === 'actualizar') return t('preparar.noComprobada.actualizar')
  const titulo = t('preparar.noComprobadaTitulo', { agente: nombreAgente(agente) }) + '.'
  return accion === 'instalar' && conSesion(agente) ? `${titulo} ${t('preparar.noComprobada.instalar')}` : titulo
}

// accionPrincipal es la única acción de preparar que ofrece la tarjeta: no instalado → Instalar; sin sesión → Iniciar
// sesión; la sesión no responde o el CLI pasó a una clave → Volver a iniciar sesión; listo con una versión nueva →
// Actualizar. Si la receta no tiene esa acción, nada; si la versión no está comprobada, el aviso, y sin el botón cuando
// el servicio no la habilita (iniciar sesión).
export function accionPrincipal(a: Adaptador, v?: VersionesAgente): Principal {
  const accion: Accion | undefined = a.estado === 'no_instalado' ? 'instalar'
    : ['sin_sesion', 'sesion_no_responde', 'usa_clave'].includes(a.estado) ? 'iniciar_sesion'
      : a.estado === 'listo' && versionNueva(v) ? 'actualizar' : undefined
  if (!v || !accion || !(accion in v.comprobadas)) return {}
  const aviso = v.comprobadas[accion] ? undefined : avisoNoComprobada(accion, a.agente, v)
  if (!v.acciones.includes(accion)) return { aviso }
  return { accion, etiqueta: etiquetaAccion(accion, a.estado), aviso }
}

export const etiquetaAccion = (accion: Accion, estado?: Adaptador['estado']): Clave =>
  accion === 'iniciar_sesion' ? (estado === 'sin_sesion' ? 'preparar.iniciarSesion' : 'preparar.volverAIniciar') : `preparar.${accion}`

// pideSeguir: la versión no estaba comprobada y el usuario puede seguir igualmente (confirmar), salvo al iniciar sesión,
// que no se salta nunca.
export const pideSeguir = (e?: EstadoPreparacion | null) =>
  e?.estado === 'error' && e.motivo_codigo === 'version_no_comprobada' && e.accion !== 'iniciar_sesion'

// confirmarDesinstalar: el diálogo de desinstalar ya pide confirmación; con la versión sin comprobar la manda (Ruling 22).
export const confirmarDesinstalar = (v?: VersionesAgente) => v?.comprobadas.desinstalar === false

// vistaEspera es lo que el usuario tiene que hacer para iniciar sesión: abrir el enlace (solo https), el código de
// dispositivo para copiar y, si el CLI lo pide, pegar el que le den.
export function vistaEspera(e?: EstadoPreparacion | null) {
  if (e?.estado !== 'esperando_usuario') return undefined
  return { enlace: enlaceSeguro(e.enlace), codigo: e.codigo ?? '', pegar: !!e.pide_pegar }
}

// trozos parte una línea de la salida en texto y enlaces (los impares), para resaltar los enlaces.
export const trozos = (linea: string) => linea.split(/(https?:\/\/\S+)/)

// AVISOS_SESION: lo que la cuenta del proveedor exige antes de iniciar sesión, que no es un ajuste del CLI. Codex entra
// por código de dispositivo, y ChatGPT pide activarlo una vez en sus ajustes de seguridad (m0.md, Ruling 8).
export const AVISOS_SESION: Record<string, { texto: Clave; enlace: string }> = {
  codex: { texto: 'preparar.avisoCodex', enlace: 'https://chatgpt.com/#settings/Security' },
}
