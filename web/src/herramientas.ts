// Herramientas de terceros (Claude Code, Codex, Continue, opencode, Kilo) apuntadas a Tramtai: lo que decide la pestaña de
// Mis APIs (componentes/accesos/Herramientas.vue) y su acceso al servicio. Los textos de los estados son fijos en inglés
// (los mismos de la CLI y la terminal).
import { ref } from 'vue'
import { api, avisarError, ErrorApi, mensajeError } from './api/cliente'
import type { EstadoHerramienta } from './api/tipos'
import { fichaDe, fichas, type OpcionModelo } from './agentes'
import es, { type Clave } from './idiomas/es'
import { fecha } from './formato'
import { t } from './i18n'

// TEMA_AYUDA es el tema de la Ayuda que explica estas herramientas. Hoy es el de conectar una app: el tema propio lo trae la
// Tarea 10 con otro id (el id «herramientas» ya es el de las herramientas de un agente), y entonces se cambia aquí.
export const TEMA_AYUDA = 'conectar'

// textoEstado dice el estado con una palabra; claseEstado, el color de su etiqueta (nunca el único aviso).
export function textoEstado(estado: string): string {
  const clave = `herramientas.estado.${estado}`
  return clave in es ? t(clave as Clave) : estado
}
export const claseEstado = (estado: string) => ({ activa: 'ok', desactualizada: 'aviso', detectada: 'info' })[estado] ?? 'neutro'

// estaPuesta: Tramtai escribió en su archivo (también si ya no funciona); es lo que muestra el interruptor.
export const estaPuesta = (h: EstadoHerramienta) => h.estado === 'activa' || h.estado === 'desactualizada'
// puedeCambiar: desde un proyecto solo Claude Code tiene archivo propio (solo_global), y sin detectar no hay archivo que tocar.
export const puedeCambiar = (h: EstadoHerramienta) => h.estado !== 'solo_global' && h.estado !== 'no_detectada'

// puedeActivarTodas: hay alguna fila que «Activar todas» activaría o reactivaría (no si todas están ya activas).
export const puedeActivarTodas = (lista: EstadoHerramienta[]) => lista.some((h) => ['detectada', 'desactivada', 'desactualizada'].includes(h.estado))

// textoMotivo es por qué una herramienta no va, en el idioma de la interfaz: el servicio da un código con sus datos y, si
// esta Web UI no tiene texto para ese código (o es un servicio anterior), su propio texto.
export function textoMotivo(h: EstadoHerramienta): string {
  const clave = `herramientas.motivo.${h.motivo_codigo}`
  return h.motivo_codigo && clave in es ? t(clave as Clave, h.motivo_datos ?? {}) : (h.motivo ?? '')
}

// textoRestaurar avisa de lo que se pierde al restaurar y de qué fecha es la copia (sin fecha, sin decirla).
export const textoRestaurar = (h: EstadoHerramienta) =>
  h.fecha_copia ? t('herramientas.restaurarTexto', { fecha: fecha(h.fecha_copia) }) : t('herramientas.restaurarTextoSinFecha')

export type AccionHerramienta = 'modelo' | 'olvidar' | 'restaurar'
// accionesDe son las del menú de la fila; restaurar, la destructiva, va la última y solo si hay copia a la que volver.
export function accionesDe(h: EstadoHerramienta): AccionHerramienta[] {
  const acciones: AccionHerramienta[] = []
  if (estaPuesta(h)) acciones.push('modelo')
  if (estaPuesta(h) || h.estado === 'desactivada') acciones.push('olvidar')
  if (h.hay_copia) acciones.push('restaurar')
  return acciones
}

// opcionesPermitidas deja los modelos de los agentes que una API admite (los que aún no pasaron su prueba de seguridad
// solo sirven desde Probar y la terminal; el servicio los rechazaría); los perfiles se quedan.
export const opcionesPermitidas = (opciones: OpcionModelo[]) => opciones.filter((o) => o.grupo === 'perfil' || fichaDe(o.grupo)?.permitido_apps !== false)

// modeloInicial es el primer agente conectado y permitido en APIs, en el orden de las fichas, con el modelo de su CLI; sin
// fichas que coincidan, la primera opción que no sea un perfil.
export function modeloInicial(opciones: OpcionModelo[]): string {
  for (const f of fichas.value) {
    if (f.desconectado || !f.permitido_apps) continue
    const o = opciones.find((x) => x.value === f.agente) ?? opciones.find((x) => x.grupo === f.agente)
    if (o) return o.value
  }
  return opciones.find((o) => o.grupo !== 'perfil')?.value ?? ''
}

// crearHerramientas es el estado de la pestaña y sus acciones. Cada acción vuelve a pedir la lista al terminar, también si
// falló: una activación de varias puede haber dejado algunas hechas.
// alCambiar se llama tras cada cambio que sale bien (Mis APIs recarga su lista: la API de la herramienta cambia con ella).
export function crearHerramientas(alCambiar?: () => void) {
  const lista = ref<EstadoHerramienta[]>([])
  const cargando = ref(true)
  // error: la carga falló y no hay lista que enseñar; noDisponible, el servicio está en modo servidor (409).
  const error = ref('')
  const noDisponible = ref(false)
  // ocupado es lo que se está cambiando: el id de la herramienta, 'todas:1' o 'todas:0'.
  const ocupado = ref('')

  function falla(e: unknown) {
    if (e instanceof ErrorApi && e.codigo === 'no_disponible_en_servidor') {
      noDisponible.value = true
      lista.value = []
    } else if (lista.value.length) avisarError(e)
    else error.value = mensajeError(e)
  }

  async function cargar() {
    cargando.value = true
    try {
      lista.value = await api.herramientas()
      error.value = ''
      noDisponible.value = false
    } catch (e) {
      falla(e)
    } finally {
      cargando.value = false
    }
  }

  // hacer devuelve lo que dio la acción, o undefined si falló (el fallo ya se avisó).
  async function hacer<T>(clave: string, accion: () => Promise<T>): Promise<T | undefined> {
    ocupado.value = clave
    try {
      const r = await accion()
      alCambiar?.()
      return r
    } catch (e) {
      falla(e)
    } finally {
      ocupado.value = ''
      await cargar()
    }
  }
  const salio = async (clave: string, accion: () => Promise<unknown>) => (await hacer(clave, async () => (await accion(), true))) === true
  const activar = (ids: string[], modelo: string) => salio(ids.join(), () => api.activarHerramientas({ ids, todas: false, modelo }))
  const activarTodas = (modelo: string) => salio('todas:1', () => api.activarHerramientas({ ids: [], todas: true, modelo }))

  // El diálogo de activar: a quién (objetivo: las ids o todas; cambiando, ya activa y solo cambia de modelo) y con qué modelo.
  const dialogoModelo = ref(false)
  const modelo = ref('')
  const objetivo = ref({ ids: [] as string[], todas: false, cambiando: false, nombre: '' })
  // pedirModelo abre el diálogo para esas herramientas (sin ids, todas) con su modelo actual o el primer agente conectado.
  function pedirModelo(opciones: OpcionModelo[], ids?: string[], actual = '') {
    const primera = ids && lista.value.find((x) => x.id === ids[0])
    objetivo.value = { ids: ids ?? [], todas: !ids, cambiando: !!primera && estaPuesta(primera), nombre: primera ? primera.nombre : (ids?.[0] ?? '') }
    modelo.value = actual || modeloInicial(opciones)
    dialogoModelo.value = true
  }
  async function confirmarModelo() {
    dialogoModelo.value = false
    const { ids, todas } = objetivo.value
    return todas ? activarTodas(modelo.value) : activar(ids, modelo.value)
  }

  return {
    lista, cargando, error, noDisponible, ocupado, cargar, activar, activarTodas,
    dialogoModelo, modelo, objetivo, pedirModelo, confirmarModelo,
    desactivar: (ids: string[], todas: boolean) => salio(todas ? 'todas:0' : ids.join(), () => api.desactivarHerramientas({ ids, todas })),
    olvidar: (id: string) => salio(id, () => api.olvidarHerramienta(id)),
    restaurar: (id: string) => hacer(id, () => api.restaurarHerramienta(id)),
  }
}
