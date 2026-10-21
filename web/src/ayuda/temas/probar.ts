import type { Tema } from '../tipos'

// probar es la página donde se prueba un mensaje antes de llevarlo a una app.
export const probar: Tema = {
  id: 'probar',
  icono: 'o_chat',
  titulo: 'ayuda.probar.titulo',
  resumen: 'ayuda.probar.resumen',
  paginaApp: 'ejecutar',
  conDiagrama: true,
  relacionados: ['recetas', 'json', 'archivos'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_edit_note', titulo: 'ayuda.probar.d1', nota: 'ayuda.probar.d1Nota' },
        { icono: 'o_smart_toy', titulo: 'ayuda.probar.d2', nota: 'ayuda.probar.d2Nota' },
        { icono: 'o_task_alt', titulo: 'ayuda.probar.d3', nota: 'ayuda.probar.d3Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.probar.paso1', 'ayuda.probar.paso2', 'ayuda.probar.paso3', 'ayuda.probar.paso4'] },
    {
      tipo: 'recorrido',
      pasos: [
        { ancla: 'ejecutar.receta', titulo: 'ayuda.probar.rec1Titulo', texto: 'ayuda.probar.rec1Texto', bienvenida: true },
        { ancla: 'ejecutar.mensaje', titulo: 'ayuda.probar.rec2Titulo', texto: 'ayuda.probar.rec2Texto', bienvenida: true },
        { ancla: 'ejecutar.codigo', titulo: 'ayuda.probar.rec3Titulo', texto: 'ayuda.probar.rec3Texto', bienvenida: true },
      ],
    },
    { tipo: 'captura', id: 'receta', alt: 'ayuda.probar.capturaModelo' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'como-codigo', pregunta: 'ayuda.probar.pCodigo', respuesta: 'ayuda.probar.rCodigo' },
        { id: 'pruebas-recientes', pregunta: 'ayuda.probar.pRecientes', respuesta: 'ayuda.probar.rRecientes' },
        { id: 'que-esfuerzo', pregunta: 'ayuda.probar.pEsfuerzo', respuesta: 'ayuda.probar.rEsfuerzo' },
      ],
    },
    { tipo: 'ir', pagina: 'ejecutar' },
  ],
}
