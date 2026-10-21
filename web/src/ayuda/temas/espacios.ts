import type { Tema } from '../tipos'

// espacios explica la global y los proyectos: qué guarda cada uno, qué heredan, la confianza y cómo se gestionan.
export const espacios: Tema = {
  id: 'espacios',
  icono: 'o_workspaces',
  titulo: 'ayuda.espacios.titulo',
  resumen: 'ayuda.espacios.resumen',
  paginaApp: 'espacios',
  conDiagrama: true,
  relacionados: ['claves', 'ajustes', 'conceptos'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_public', titulo: 'ayuda.espacios.d1', nota: 'ayuda.espacios.d1Nota' },
        { icono: 'o_tune', titulo: 'ayuda.espacios.d2', nota: 'ayuda.espacios.d2Nota' },
        { icono: 'o_folder', titulo: 'ayuda.espacios.d3', nota: 'ayuda.espacios.d3Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.espacios.paso1', 'ayuda.espacios.paso2', 'ayuda.espacios.paso3'] },
    {
      tipo: 'recorrido',
      pasos: [
        { ancla: 'espacios.lista', titulo: 'ayuda.espacios.rec1Titulo', texto: 'ayuda.espacios.rec1Texto', bienvenida: true },
        { ancla: 'espacios.abrir', titulo: 'ayuda.espacios.rec2Titulo', texto: 'ayuda.espacios.rec2Texto', bienvenida: true },
        { ancla: 'espacios.actualizar', titulo: 'ayuda.espacios.rec3Titulo', texto: 'ayuda.espacios.rec3Texto' },
      ],
    },
    { tipo: 'captura', id: 'espacios', alt: 'ayuda.espacios.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'confiar', pregunta: 'ayuda.espacios.pConfiar', respuesta: 'ayuda.espacios.rConfiar' },
        { id: 'mover', pregunta: 'ayuda.espacios.pMover', respuesta: 'ayuda.espacios.rMover' },
        { id: 'copiar-proyecto', pregunta: 'ayuda.espacios.pCopiar', respuesta: 'ayuda.espacios.rCopiar' },
        { id: 'olvidar', pregunta: 'ayuda.espacios.pOlvidar', respuesta: 'ayuda.espacios.rOlvidar' },
        { id: 'no-encontrado', pregunta: 'ayuda.espacios.pNoEncontrado', respuesta: 'ayuda.espacios.rNoEncontrado' },
        { id: 'heredar', pregunta: 'ayuda.espacios.pHeredar', respuesta: 'ayuda.espacios.rHeredar' },
        { id: 'espacios-terminal', pregunta: 'ayuda.espacios.pTerminal', respuesta: 'ayuda.espacios.rTerminal' },
      ],
    },
    { tipo: 'errores', codigos: ['confianza_requerida', 'espacio_detenido'] },
    { tipo: 'ir', pagina: 'espacios' },
  ],
}
