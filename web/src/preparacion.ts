// preparacion: instalar, actualizar, desinstalar e iniciar sesión en los CLI de los agentes. Qué acción ofrece cada
// tarjeta (según su estado y GET /api/v1/agentes/versiones) y el sondeo de una preparación mientras está en marcha.
import type { Adaptador, EstadoPreparacion, PedidoPreparacion, VersionesAgente } from './api/tipos'
import { enlaceSeguro } from './enlaces'
import type { Clave } from './idiomas/es'

export type Accion = PedidoPreparacion['accion']

// enMarcha: la preparación aún no terminó (listo, error y cancelado son finales).
export const enMarcha = (e?: EstadoPreparacion | null) => !!e && !['listo', 'error', 'cancelado'].includes(e.estado)

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

// masNueva compara dos versiones por sus números («2.10.0» es más nueva que «2.9.9»).
export function masNueva(a: string, b: string): boolean {
  const [x, y] = [a, b].map((v) => v.split('.').map((n) => parseInt(n, 10) || 0))
  for (let i = 0; i < Math.max(x.length, y.length); i++) if ((x[i] ?? 0) !== (y[i] ?? 0)) return (x[i] ?? 0) > (y[i] ?? 0)
  return false
}

// versionNueva es la última publicada si es más nueva que la instalada ('' si no la hay o no se sabe).
export const versionNueva = (v?: VersionesAgente) => (v?.instalada && v.ultima && masNueva(v.ultima, v.instalada) ? v.ultima : '')

export interface Principal {
  accion?: Accion
  etiqueta?: Clave
  // aviso: la versión que usaría no está comprobada (el texto del servicio).
  aviso?: string
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
  const aviso = v.comprobadas[accion] ? undefined : v.motivo
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
