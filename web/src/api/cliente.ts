// Cliente de la API de Tramtai para la Web UI: usa la cookie de sesión (mismo origen) y envía el token CSRF en
// las peticiones que modifican. Las claves nunca se guardan en el navegador.
import { Notify } from 'quasar'
import { idioma, idiomas, t } from '../i18n'
import type { Clave } from '../idiomas/es'
import { compartida } from '../compartida'
import { ERRORES } from '../ayuda/temas/problemas'
import { actualizacion, dialogoActualizar, diasHistorial, sesion } from '../estado'
import type { Acerca, Capacidades, PrimerosPasos, EspacioUI, AjustesPunto, ConfigPunto, EntradaConfiguracion, EstadoActualizacion, EstadoConversor, ResumenInstancia, EstadoPresupuesto, EstadoPunto, EventoRegistro, LimitesPeticion, Modelo, Novedad, ProgresoActualizacion, Retencion, Sesion, Solicitud, Trabajo, UsoPunto } from './tipos'

// Campos declarados aparte (no en el constructor): node --test ejecuta este archivo quitando solo los tipos.
export class ErrorApi extends Error {
  codigo: string
  estado: number
  // datos es la respuesta entera: algunos errores traen más (las claves de una carpeta que pide confianza).
  datos: Record<string, unknown>
  constructor(codigo: string, mensaje: string, estado: number, datos: Record<string, unknown> = {}) {
    super(mensaje)
    this.codigo = codigo
    this.estado = estado
    this.datos = datos
  }
}

let csrf = ''
// Una sola renovación a la vez, por muchas peticiones que reciban 401 juntas (ver compartida).
const renovarSesion = compartida(() => api.sesion())

// plazo corta las consultas (GET) que se quedan colgadas; las que modifican no lo llevan porque abortarlas no
// deshace la acción en el servicio, y las subidas o la actualización pueden tardar lo que haga falta.
export async function pedir<T>(metodo: string, ruta: string, cuerpo?: unknown, plazo = metodo === 'GET' ? 15000 : 0, reintento = false): Promise<T> {
  const cabeceras: Record<string, string> = {}
  let body: BodyInit | undefined
  if (cuerpo instanceof FormData) {
    body = cuerpo
  } else if (cuerpo !== undefined) {
    body = JSON.stringify(cuerpo)
    cabeceras['Content-Type'] = 'application/json'
  }
  if (metodo !== 'GET') cabeceras['X-CSRF-Token'] = csrf
  const signal = plazo && typeof AbortSignal.timeout === 'function' ? AbortSignal.timeout(plazo) : undefined
  const respuesta = await fetch(ruta, { method: metodo, headers: cabeceras, body, credentials: 'same-origin', signal })
  if (respuesta.status === 204) return undefined as T
  const datos = await respuesta.json().catch(() => ({}))
  if (!respuesta.ok) {
    // La sesión dejó de valer porque el servicio se reinició: se abre otra y se repite la petición una vez.
    if (respuesta.status === 401 && !reintento && ruta !== '/ui/sesion' && sesion.value) {
      try {
        await renovarSesion()
      } catch {
        sesion.value = null
        throw new ErrorApi('sesion_caducada', t('app.sesionCaducada'), 401)
      }
      return pedir<T>(metodo, ruta, cuerpo, plazo, true)
    }
    // CSRF viejo: en otra pestaña se cambió de espacio y la cookie es otra. Se renueva la sesión una vez y no se repite
    // la acción (iría a otro espacio): la página avisa y se recarga.
    if (respuesta.status === 403 && datos?.error?.codigo === 'csrf_invalido' && metodo !== 'GET' && !reintento && sesion.value) {
      await renovarSesion().catch(() => undefined)
      if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('tramtai:espacio-cambiado'))
      throw new ErrorApi('espacio_cambiado', t('app.espacioCambiado'), 403, datos ?? {})
    }
    throw new ErrorApi(datos?.error?.codigo ?? 'error', datos?.error?.mensaje ?? respuesta.statusText, respuesta.status, datos ?? {})
  }
  return datos as T
}

// accionEspacio activa, detiene, confía, reasocia u olvida un espacio y devuelve la lista actualizada.
function accionEspacio(accion: string, cuerpo: Record<string, unknown>) {
  return pedir<{ espacios: EspacioUI[] }>('POST', `/ui/espacios/${accion}`, cuerpo).then((r) => r.espacios)
}

function recordar(s: Sesion): Sesion {
  csrf = s.csrf
  sesion.value = s
  return s
}

export const api = {
  sesion: () => pedir<Sesion>('GET', '/ui/sesion').then(recordar),
  cambiarInstancia: (instancia: string) => pedir<Sesion>('POST', '/ui/sesion', { instancia }).then(recordar),
  activarGlobal: () => pedir<Sesion>('POST', '/ui/instancias').then(recordar),
  // abrirProyecto activa el espacio de una carpeta; si aún no es un proyecto, el servicio crea su .tramtai/ (y queda
  // confiado). Si ya lo era, responde confianza_requerida (409) y se repite con confiar; dentro de otro proyecto,
  // dentro_de_proyecto y se repite con crear_aqui.
  abrirProyecto: (raiz: string, opciones: { confiar?: boolean; crear_aqui?: boolean } = {}) =>
    pedir<Sesion>('POST', '/ui/proyectos', { raiz, ...opciones }).then(recordar),
  // conectarAgente conecta o desconecta un agente en todos los espacios.
  conectarAgente: (agente: string, conectado: boolean) => pedir<void>('POST', '/ui/agentes/conexion', { agente, conectado }),
  espacios: () => pedir<{ espacios: EspacioUI[] }>('GET', '/ui/espacios').then((r) => r.espacios),
  activarEspacio: (id: string) => accionEspacio('activar', { id }),
  detenerEspacio: (id: string) => accionEspacio('detener', { id }),
  confiarEspacio: (id: string, confiar: boolean) => accionEspacio('confiar', { id, confiar }),
  ubicarEspacio: (id: string, ruta: string) => accionEspacio('ubicar', { id, ruta }),
  olvidarEspacio: (id: string, nombre: string) => accionEspacio('olvidar', { id, nombre }),
  autoarranque: (activo: boolean) => pedir<Sesion>('POST', '/ui/autoarranque', { activo }).then(recordar),
  actualizacion: () => pedir<EstadoActualizacion>('GET', '/ui/actualizacion'),
  acerca: () => pedir<Acerca>('GET', '/ui/acerca'),
  limites: () => pedir<LimitesPeticion>('GET', '/api/v1/limites'),
  // El documento OpenAPI se reescribe tal cual (servers, operaciones) antes de guardarlo.
  openapi: () => pedir<Record<string, any>>('GET', '/api/v1/openapi.json'),
  buscarActualizaciones: (buscar: boolean) => pedir<EstadoActualizacion>('POST', '/ui/actualizacion', { buscar }),
  buscarAhora: () => pedir<EstadoActualizacion>('GET', '/ui/actualizacion?ahora=1'),
  actualizar: () => pedir<{ version: string }>('POST', '/ui/actualizar'),
  progresoActualizacion: () => pedir<ProgresoActualizacion>('GET', '/ui/actualizacion/progreso'),
  // En la cuenta atrás antes de instalar: instalar ya («Reiniciar ahora») o cancelar.
  instalarYa: () => pedir<void>('POST', '/ui/actualizar/instalar'),
  cancelarActualizacion: () => pedir<void>('POST', '/ui/actualizar/cancelar'),
  // novedades da lo que trae cada versión después de desde y hasta hasta (sin hasta, la instalada).
  novedades: (desde: string, hasta = '') =>
    pedir<{ versiones: Novedad[] }>('GET', '/ui/actualizacion/novedades?' + new URLSearchParams({ desde, ...(hasta ? { hasta } : {}) })).then((r) => r.versiones),
  cambiarAutenticacion: (agente: string, modo: 'suscripcion' | 'clave_api') => pedir<void>('POST', '/ui/autenticacion', { agente, modo }),
  salir: () => pedir<void>('DELETE', '/ui/sesion').then(() => (sesion.value = null)),
  detenerServicio: () => pedir<unknown>('POST', '/ui/servicio/detener'),
  fijarPuerto: (puerto: number, automatico: boolean) => pedir<Sesion>('POST', '/ui/servicio/puerto', { puerto, automatico }).then(recordar),
  reiniciarServicio: () => pedir<{ url: string }>('POST', '/ui/servicio/reiniciar'),
  instancia: () => pedir<ResumenInstancia>('GET', '/api/v1/instancia'),
  claveInstancia: () => pedir<{ token: string; archivo: string }>('GET', '/api/v1/instancia/token'),
  // regenerarClaveInstancia cambia la clave que usan las aplicaciones; la anterior deja de valer al momento.
  regenerarClaveInstancia: () => pedir<{ token: string; archivo: string }>('POST', '/api/v1/instancia/token'),
  // heredar quita un valor propio de un proyecto (de su local.yaml): vuelve a valer el de Global.
  heredar: (clave: string) => pedir<void>('POST', '/api/v1/instancia/heredar', { clave }),
  configuracion: () =>
    pedir<{ configuracion: EntradaConfiguracion[] }>('GET', '/api/v1/instancia/configuracion').then((r) => r.configuracion),
  catalogo: () => pedir<{ modelos: Modelo[] }>('GET', '/api/v1/modelos').then((r) => r.modelos),
  // fijarPredeterminado guarda el modelo predeterminado y, si se indica, su esfuerzo ('' = el del agente); con modelo
  // vacío solo cambia el esfuerzo (en un proyecto no fija el modelo heredado).
  fijarPredeterminado: (modelo: string, esfuerzo?: string) => pedir<void>('POST', '/api/v1/instancia/predeterminado', { modelo, esfuerzo }),
  // capacidades comprueba cada agente con hasta 10 s por agente: lleva un plazo mayor.
  capacidades: () => pedir<Capacidades>('GET', '/api/v1/capacidades', undefined, 60000),
  // pasos son los Primeros pasos de Inicio, ya calculados, con los agentes que comprobó (tarda como capacidades).
  pasos: () => pedir<PrimerosPasos>('GET', '/api/v1/instancia/pasos', undefined, 60000),
  // trabajos lista los más recientes; filtro acota por API, grupo, origen, estado (en_curso, completado o con_error), texto
  // o fecha (antes, para la página siguiente). Todo lo filtra el servicio.
  trabajos: (limite = 50, filtro: { punto?: string; grupo?: string; origen?: string; estado?: string; texto?: string; antes?: string; antes_id?: string } = {}) => {
    const q = new URLSearchParams({ limite: String(limite) })
    for (const [clave, valor] of Object.entries(filtro)) if (valor) q.set(clave, valor)
    return pedir<{ trabajos: Trabajo[] }>('GET', '/api/v1/trabajos?' + q).then((r) => r.trabajos)
  },
  retencion: () => pedir<Retencion>('GET', '/api/v1/instancia/retencion'),
  fijarRetencion: (r: { horas_resultado: number; horas_metadatos: number; max_trabajos: number }) => pedir<void>('PUT', '/api/v1/instancia/retencion', r),
  trabajo: (id: string) => pedir<Trabajo>('GET', `/api/v1/trabajos/${encodeURIComponent(id)}`),
  registro: (id: string) => pedir<{ eventos: EventoRegistro[] }>('GET', `/api/v1/trabajos/${encodeURIComponent(id)}/registro`).then((r) => r.eventos),
  crear: (s: Solicitud) => pedir<Trabajo>('POST', '/api/v1/trabajos', s),
  accion: (id: string, accion: 'cancelar' | 'reintentar') =>
    pedir<Trabajo>('POST', `/api/v1/trabajos/${encodeURIComponent(id)}/${accion}`),
  // corregir pide al mismo agente que corrija la respuesta que no cumplía el esquema, como un intento nuevo.
  corregir: (id: string) => pedir<Trabajo>('POST', `/api/v1/trabajos/${encodeURIComponent(id)}/reintentar`, { corregir: true }),
  // validarEsquema compila el texto del esquema en el servicio, sin ejecutar nada; linea y columna si no es JSON.
  validarEsquema: (texto: string) =>
    pedir<{ valido: boolean; error?: string; linea?: number; columna?: number }>('POST', '/api/v1/esquemas/validar', { esquema: texto }),
  eliminar: (id: string) => pedir<void>('DELETE', `/api/v1/trabajos/${encodeURIComponent(id)}`),
  vaciarHistorial: () => pedir<{ eliminados: number }>('DELETE', '/api/v1/trabajos').then((r) => r.eliminados),
  conversor: () => pedir<EstadoConversor>('GET', '/api/v1/conversor'),
  prepararConversor: () => pedir<EstadoConversor>('POST', '/api/v1/conversor/preparar'),
  ajustesConversor: (prepararAlIniciar: boolean) => pedir<void>('PUT', '/api/v1/conversor/ajustes', { preparar_al_iniciar: prepararAlIniciar }),
  subir: (archivo: File) => {
    const datos = new FormData()
    datos.append('archivo', archivo)
    return pedir<{ identificador_carga: string }>('POST', '/api/v1/cargas', datos)
  },
  puntos: () => pedir<{ puntos_acceso: EstadoPunto[] }>('GET', '/api/v1/puntos').then((r) => r.puntos_acceso),
  puntosConUso: () => pedir<{ puntos_acceso: EstadoPunto[]; uso?: Record<string, UsoPunto>; presupuestos?: Record<string, EstadoPresupuesto> }>('GET', '/api/v1/puntos'),
  presupuestoInstancia: () => pedir<{ presupuesto?: EstadoPresupuesto }>('GET', '/api/v1/instancia/presupuesto').then((r) => r.presupuesto),
  // reiniciarPresupuesto pone a cero el consumo del día y del mes de una API, o del espacio sin nombre.
  reiniciarPresupuesto: (nombre?: string) =>
    pedir<void>('POST', nombre ? `/api/v1/puntos/${encodeURIComponent(nombre)}/presupuesto/reiniciar` : '/api/v1/instancia/presupuesto/reiniciar'),
  crearPunto: (p: ConfigPunto) => pedir<{ punto_acceso?: EstadoPunto; token: string }>('POST', '/api/v1/puntos', p),
  activarPunto: (nombre: string, activo: boolean) => pedir<unknown>('PATCH', `/api/v1/puntos/${encodeURIComponent(nombre)}`, { activo }),
  // ajustesPunto cambia solo lo que se envía; modelo es un id del catálogo y '' vuelve al del espacio.
  ajustesPunto: (nombre: string, ajustes: AjustesPunto) => pedir<unknown>('PATCH', `/api/v1/puntos/${encodeURIComponent(nombre)}`, ajustes),
  eliminarPunto: (nombre: string) => pedir<void>('DELETE', `/api/v1/puntos/${encodeURIComponent(nombre)}`),
  // regenerarClave crea una clave nueva para la API: solo se guarda su huella, así que es la única vez que se ve.
  regenerarClave: (nombre: string) => pedir<{ token: string }>('POST', `/api/v1/puntos/${encodeURIComponent(nombre)}/token`).then((r) => r.token),
}

export function cargarDiasHistorial() {
  api.retencion().then((r) => (diasHistorial.value = Math.round((r.horas_metadatos / 24) * 10) / 10)).catch(() => {})
}

// tituloError es el texto de un código de error del servicio en el idioma de la Web UI (o el código si no lo conoce).
export function tituloError(codigo?: string): string {
  const clave = ('error.' + codigo) as Clave
  return codigo && clave in idiomas[idioma.value].textos ? t(clave) : (codigo ?? '')
}

// mensajeError usa el mensaje del servicio, que está en español y trae el detalle; en otro idioma, el texto del código.
export function mensajeError(e: unknown): string {
  if (e instanceof ErrorApi && idioma.value !== 'es' && tituloError(e.codigo) !== e.codigo) return tituloError(e.codigo)
  return e instanceof Error ? e.message : String(e)
}

// pasoError es qué hacer ante un código de error: la solución que da la Ayuda (la de «¿Qué hago?»), o nada.
export function pasoError(codigo?: string): string {
  const e = codigo ? ERRORES[codigo] : undefined
  return e ? t(e.solucion) : ''
}

// textosAviso son el mensaje y la línea de debajo del aviso de un error. Sin Vue ni Quasar, para poder probarlo.
export function textosAviso(e: unknown): { message: string; caption?: string } {
  const mensaje = mensajeError(e)
  // En otro idioma, el detalle original va debajo para no perderlo; después, qué hacer.
  const detalle = e instanceof Error && e.message !== mensaje ? e.message : ''
  const codigo = e instanceof ErrorApi ? e.codigo : undefined
  // Sin solución en la Ayuda, el qué hacer está en el texto del código; en inglés ya es el mensaje y no se repite.
  const titulo = tituloError(codigo)
  const paso = pasoError(codigo) || (titulo !== (codigo ?? '') && titulo !== mensaje ? titulo : '')
  return { message: mensaje, caption: [detalle, paso].filter(Boolean).join(' ') || undefined }
}

export function avisarError(e: unknown) {
  Notify.create({ type: 'negative', ...textosAviso(e), position: 'top' })
}

// duracionAviso da tiempo a leer un aviso: unos 60 ms por letra, entre 4 y 10 s.
export const duracionAviso = (texto: string) => Math.min(10000, Math.max(4000, texto.length * 60))

export function avisar(mensaje: string) {
  Notify.create({ type: 'positive', message: mensaje, position: 'top', timeout: duracionAviso(mensaje) })
}

export async function copiar(texto: string, que = t('comun.copiado')) {
  try {
    await navigator.clipboard.writeText(texto)
    Notify.create({ message: que, icon: 'content_copy', timeout: duracionAviso(que), position: 'top' })
  } catch {
    Notify.create({ type: 'warning', message: t('comun.noCopiado'), position: 'top' })
  }
}

const esperar = (ms: number) => new Promise((listo) => setTimeout(listo, ms))
// responde indica si hay un servicio en url; con no-cors basta para otro puerto, que es otro origen.
const responde = (url: string) => fetch(url + '/healthz', { mode: 'no-cors', cache: 'no-store' }).then(() => true).catch(() => false)

// reiniciarYSeguir reinicia el servicio y abre la página en su nueva dirección cuando responde (el puerto puede
// cambiar). Primero espera a que el actual termine, porque mientras drena los trabajos sigue respondiendo.
export async function reiniciarYSeguir() {
  const { url } = await api.reiniciarServicio()
  const aviso = Notify.create({ type: 'ongoing', message: t('app.reiniciando'), position: 'top' })
  for (let i = 0; i < 120 && (await responde(location.origin)); i++) await esperar(500)
  for (let i = 0; i < 120; i++) {
    if (await responde(url)) {
      location.href = url + '/' + location.hash
      return
    }
    await esperar(500)
  }
  aviso()
  Notify.create({ type: 'negative', message: t('app.reinicioFallido', { url }), position: 'top', timeout: 0, actions: [{ label: t('comun.cerrar'), color: 'white' }] })
}

// buscarActualizacionAhora consulta GitHub en el momento (Ajustes y la bandeja): si hay versión nueva abre su diálogo
// y, si no, avisa de que está al día o de que no se pudo consultar.
export async function buscarActualizacionAhora() {
  try {
    const estado = await api.buscarAhora()
    actualizacion.value = estado
    if (estado.disponible) dialogoActualizar.value = true
    else if (estado.fallo) Notify.create({ type: 'warning', message: t('actualizacion.falloConsulta'), position: 'top' })
    else avisar(t('actualizacion.alDia', { actual: estado.actual }))
  } catch (e) {
    avisarError(e)
  }
}
