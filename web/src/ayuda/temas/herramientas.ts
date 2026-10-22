import type { Tema } from '../tipos'

// herramientas explica la ida y vuelta de una llamada a una función de la app.
export const herramientas: Tema = {
  id: 'herramientas',
  icono: 'o_handyman',
  titulo: 'ayuda.herramientas.titulo',
  resumen: 'ayuda.herramientas.resumen',
  conDiagrama: true,
  relacionados: ['conectar', 'json', 'archivos'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_apps', titulo: 'ayuda.herramientas.d1', nota: 'ayuda.herramientas.d1Nota' },
        { icono: 'o_route', titulo: 'ayuda.herramientas.d2', nota: 'ayuda.herramientas.d2Nota' },
        { icono: 'o_play_circle', titulo: 'ayuda.herramientas.d3', nota: 'ayuda.herramientas.d3Nota' },
        { icono: 'o_task_alt', titulo: 'ayuda.herramientas.d4', nota: 'ayuda.herramientas.d4Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.herramientas.paso1', 'ayuda.herramientas.paso2', 'ayuda.herramientas.paso3'] },
    { tipo: 'nota', texto: 'ayuda.herramientas.eleccion' },
    { tipo: 'nota', texto: 'ayuda.herramientas.completa' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'ejemplo-completo', pregunta: 'ayuda.herramientas.pGuia', respuesta: 'ayuda.herramientas.rGuia' },
        { id: 'ejecuta-algo', pregunta: 'ayuda.herramientas.pSeguridad', respuesta: 'ayuda.herramientas.rSeguridad' },
      ],
    },
    { tipo: 'ir', pagina: 'conectar' },
  ],
}
