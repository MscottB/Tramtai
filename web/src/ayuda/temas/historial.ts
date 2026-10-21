import type { Tema } from '../tipos'

// historial explica qué queda de cada petición y cómo revisarla.
export const historial: Tema = {
  id: 'historial',
  icono: 'o_history',
  titulo: 'ayuda.historial.titulo',
  resumen: 'ayuda.historial.resumen',
  paginaApp: 'historial',
  conDiagrama: false,
  relacionados: ['problemas', 'probar', 'ajustes'],
  bloques: [
    { tipo: 'pasos', pasos: ['ayuda.historial.paso1', 'ayuda.historial.paso2', 'ayuda.historial.paso3'] },
    {
      tipo: 'recorrido',
      pasos: [
        { ancla: 'historial.filtros', titulo: 'ayuda.historial.rec1Titulo', texto: 'ayuda.historial.rec1Texto' },
        { ancla: 'historial.lista', titulo: 'ayuda.historial.rec2Titulo', texto: 'ayuda.historial.rec2Texto' },
        { ancla: 'historial.detalle', titulo: 'ayuda.historial.rec3Titulo', texto: 'ayuda.historial.rec3Texto' },
      ],
    },
    { tipo: 'captura', id: 'historial', alt: 'ayuda.historial.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'estados', pregunta: 'ayuda.historial.pEstados', respuesta: 'ayuda.historial.rEstados' },
        { id: 'seguir-en-vivo', pregunta: 'ayuda.historial.pSeguir', respuesta: 'ayuda.historial.rSeguir' },
        { id: 'tiempos', pregunta: 'ayuda.historial.pTiempos', respuesta: 'ayuda.historial.rTiempos' },
        { id: 'en-vivo', pregunta: 'ayuda.historial.pVivo', respuesta: 'ayuda.historial.rVivo' },
        { id: 'primer-plano', pregunta: 'ayuda.historial.pPrimerPlano', respuesta: 'ayuda.historial.rPrimerPlano' },
        { id: 'borrar-datos', pregunta: 'ayuda.historial.pBorrar', respuesta: 'ayuda.historial.rBorrar' },
      ],
    },
    { tipo: 'ir', pagina: 'historial' },
  ],
}
