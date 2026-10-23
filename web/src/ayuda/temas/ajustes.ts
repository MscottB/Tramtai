import type { Tema } from '../tipos'

// ajustes reúne el arranque, los predeterminados, los datos y las actualizaciones.
export const ajustes: Tema = {
  id: 'ajustes',
  icono: 'o_settings',
  titulo: 'ayuda.ajustes.titulo',
  resumen: 'ayuda.ajustes.resumen',
  terminal: 'ayuda.ajustes.terminal',
  paginaApp: 'configuracion',
  conDiagrama: false,
  relacionados: ['empezar', 'historial', 'archivos'],
  bloques: [
    { tipo: 'pasos', pasos: ['ayuda.ajustes.paso1', 'ayuda.ajustes.paso2', 'ayuda.ajustes.paso3'] },
    {
      tipo: 'recorrido',
      pasos: [
        { ancla: 'configuracion.pestanas', titulo: 'ayuda.ajustes.rec1Titulo', texto: 'ayuda.ajustes.rec1Texto' },
        { ancla: 'configuracion.modelo', titulo: 'ayuda.ajustes.rec2Titulo', texto: 'ayuda.ajustes.rec2Texto' },
        { ancla: 'configuracion.datos', titulo: 'ayuda.ajustes.rec3Titulo', texto: 'ayuda.ajustes.rec3Texto' },
      ],
    },
    { tipo: 'captura', id: 'actualizaciones', alt: 'ayuda.ajustes.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'al-actualizar', pregunta: 'ayuda.ajustes.pActualizar', respuesta: 'ayuda.ajustes.rActualizar' },
        { id: 'actualizar-otros', pregunta: 'ayuda.ajustes.pOtros', respuesta: 'ayuda.ajustes.rOtros' },
        { id: 'buscar-version', pregunta: 'ayuda.ajustes.pBuscar', respuesta: 'ayuda.ajustes.rBuscar' },
        { id: 'que-version', pregunta: 'ayuda.ajustes.pVersion', respuesta: 'ayuda.ajustes.rVersion' },
        { id: 'datos-ruta', pregunta: 'ayuda.ajustes.pDatos', respuesta: 'ayuda.ajustes.rDatos' },
        { id: 'cuanto-se-guarda', pregunta: 'ayuda.ajustes.pRetencion', respuesta: 'ayuda.ajustes.rRetencion' },
        { id: 'dos-pestanas', pregunta: 'ayuda.ajustes.pPestanas', respuesta: 'ayuda.ajustes.rPestanas', tema: 'espacios' },
      ],
    },
    { tipo: 'ir', pagina: 'configuracion' },
  ],
}
