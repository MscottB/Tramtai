// historialVivo pone al día la lista del Historial cuando el flujo del espacio (GET /api/v1/eventos) cuenta un cambio:
// una petición por cambio, no una cada 5 s. Con la pestaña oculta el flujo se cierra (no ocupa un cupo) y al volver
// su foto pone la lista al día. Si el servicio rechaza el flujo (sin cupo), se consulta cada 5 s hasta que vuelva.
// Sin Vue, para poder probarlo: Trabajos.vue le da cómo abrir el flujo, recargar y medir la lista.

// Flujo es lo que se usa de un EventSource.
export interface Flujo {
  addEventListener(tipo: string, f: () => void): void
  onerror: ((ev: never) => unknown) | null
  readyState: number
  close(): void
}

export interface Entorno {
  abrir: () => Flujo
  // recargar pide la primera página en segundo plano; enVuelo cuenta las consultas sin terminar.
  recargar: () => void
  enVuelo: () => number
  // paginasExtra es si hay cargadas páginas de «Ver más»: la recarga solo trae la primera, así que no se recarga.
  paginasExtra: () => boolean
  oculta: () => boolean
  cadaCinco: (f: () => void) => number
  parar: (id: number) => void
}

const CERRADO = 2 // EventSource.CLOSED

export function historialVivo(e: Entorno) {
  let fuente: Flujo | undefined
  let sondeo: number | undefined
  let pendiente = false
  let primeraFoto = true

  // alCambiar recarga, o lo deja pendiente si hay una consulta en vuelo (podría traer la lista de antes del cambio).
  function alCambiar() {
    if (e.paginasExtra()) return
    if (e.enVuelo()) pendiente = true
    else e.recargar()
  }
  // ponerAlDia se llama al terminar cada consulta: hace la recarga que quedó pendiente.
  function ponerAlDia() {
    if (!pendiente || e.enVuelo()) return
    pendiente = false
    alCambiar()
  }
  function seguir() {
    if (fuente || e.oculta()) return
    let enFoto = true // los eventos de la foto son lo que ya se cargó
    const f = (fuente = e.abrir())
    f.addEventListener('trabajo', () => enFoto || alCambiar())
    f.addEventListener('foto', () => {
      enFoto = false
      // El flujo volvió: el sondeo de respaldo sobra.
      if (sondeo !== undefined) e.parar(sondeo)
      sondeo = undefined
      if (!primeraFoto) alCambiar()
      primeraFoto = false
    })
    f.onerror = () => {
      if (f.readyState !== CERRADO) return // se reconecta solo y su foto pone la lista al día
      fuente = undefined
      primeraFoto = false
      sondeo ??= e.cadaCinco(() => !e.oculta() && !e.enVuelo() && !e.paginasExtra() && e.recargar())
    }
  }
  // ocultar cierra el flujo con la pestaña oculta; seguir lo vuelve a abrir.
  function ocultar() {
    fuente?.close()
    fuente = undefined
  }
  function cerrar() {
    ocultar()
    if (sondeo !== undefined) e.parar(sondeo)
    sondeo = undefined
  }
  return { seguir, ponerAlDia, ocultar, cerrar }
}
