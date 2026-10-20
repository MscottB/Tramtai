import type { Tema } from '../tipos'

// recetas explica lo que fija una API y el respaldo entre agentes.
export const recetas: Tema = {
  id: 'recetas',
  icono: 'o_receipt_long',
  titulo: 'ayuda.recetas.titulo',
  resumen: 'ayuda.recetas.resumen',
  conDiagrama: true,
  relacionados: ['probar', 'json', 'conectar'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_receipt_long', titulo: 'ayuda.recetas.d1', nota: 'ayuda.recetas.d1Nota' },
        { icono: 'o_smart_toy', titulo: 'ayuda.recetas.d2', nota: 'ayuda.recetas.d2Nota' },
        { icono: 'o_alt_route', titulo: 'ayuda.recetas.d3', nota: 'ayuda.recetas.d3Nota' },
        { icono: 'o_task_alt', titulo: 'ayuda.recetas.d4', nota: 'ayuda.recetas.d4Nota' },
      ],
      ramas: [{ desde: 1, hacia: 2, etiqueta: 'ayuda.recetas.rama' }],
    },
    { tipo: 'pasos', pasos: ['ayuda.recetas.paso1', 'ayuda.recetas.paso2', 'ayuda.recetas.paso3'] },
    { tipo: 'captura', id: 'editor-receta', alt: 'ayuda.recetas.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'en-espanol', pregunta: 'ayuda.recetas.pEspanol', respuesta: 'ayuda.recetas.rEspanol' },
        { id: 'agente-al-limite', pregunta: 'ayuda.recetas.pLimite', respuesta: 'ayuda.recetas.rLimite' },
        { id: 'tiempo-por-intento', pregunta: 'ayuda.recetas.pTiempo', respuesta: 'ayuda.recetas.rTiempo' },
      ],
    },
    { tipo: 'errores', codigos: ['limite_uso_proveedor'] },
    { tipo: 'ir', pagina: 'ejecutar' },
  ],
}
