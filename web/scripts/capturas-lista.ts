// CAPTURAS son las zonas de la app que muestra la Ayuda: cada una es el elemento [data-captura=<id>] de esa página.
// PORTADA son las pantallas completas de la página del producto (site/capturas/<nombre>-<claro|oscuro>.png).
export const CAPTURAS: { id: string; pagina: string }[] = [
  { id: 'inicio', pagina: 'inicio' },
  { id: 'agente', pagina: 'agentes' },
  { id: 'receta', pagina: 'ejecutar' },
  { id: 'mis-apis', pagina: 'conectar' },
  { id: 'presupuesto', pagina: 'conectar' },
  { id: 'historial', pagina: 'historial' },
  { id: 'conversor', pagina: 'configuracion' },
  { id: 'actualizaciones', pagina: 'configuracion' },
]

export const PORTADA: { nombre: string; pagina: string }[] = [
  { nombre: 'inicio', pagina: 'inicio' },
  { nombre: 'agentes', pagina: 'agentes' },
  { nombre: 'mis-apis', pagina: 'conectar' },
  { nombre: 'probar', pagina: 'ejecutar' },
]
