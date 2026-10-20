import type { Tema } from '../tipos'

// archivos sigue un documento desde la app hasta el agente, igual por las tres APIs.
export const archivos: Tema = {
  id: 'archivos',
  icono: 'o_description',
  titulo: 'ayuda.archivos.titulo',
  resumen: 'ayuda.archivos.resumen',
  conDiagrama: true,
  relacionados: ['conectar', 'probar', 'json'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_upload_file', titulo: 'ayuda.archivos.d1', nota: 'ayuda.archivos.d1Nota' },
        { icono: 'o_inventory_2', titulo: 'ayuda.archivos.d2', nota: 'ayuda.archivos.d2Nota' },
        { icono: 'o_transform', titulo: 'ayuda.archivos.d3', nota: 'ayuda.archivos.d3Nota' },
        { icono: 'o_smart_toy', titulo: 'ayuda.archivos.d4', nota: 'ayuda.archivos.d4Nota' },
        { icono: 'o_task_alt', titulo: 'ayuda.archivos.d5', nota: 'ayuda.archivos.d5Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.archivos.paso1', 'ayuda.archivos.paso2', 'ayuda.archivos.paso3'] },
    { tipo: 'ejemplo', contratos: ['nativa', 'openai', 'anthropic'], archivos: ['informe.pdf'] },
    { tipo: 'nota', texto: 'ayuda.archivos.formatos' },
    { tipo: 'nota', texto: 'ayuda.archivos.tamano' },
    { tipo: 'captura', id: 'conversor', alt: 'ayuda.archivos.captura' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'imagenes', pregunta: 'ayuda.archivos.pImagenes', respuesta: 'ayuda.archivos.rImagenes' },
        { id: 'por-tamano', pregunta: 'ayuda.archivos.pTamano', respuesta: 'ayuda.archivos.rTamano' },
        { id: 'falta-conversor', pregunta: 'ayuda.archivos.pConversor', respuesta: 'ayuda.archivos.rConversor' },
        { id: 'sale-algo', pregunta: 'ayuda.archivos.pSale', respuesta: 'ayuda.archivos.rSale' },
      ],
    },
    { tipo: 'errores', codigos: ['conversion_fallida', 'conversor_no_disponible', 'entrada_demasiado_grande'] },
    { tipo: 'ir', pagina: 'ejecutar' },
  ],
}
