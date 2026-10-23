import type { Tema } from '../tipos'

// claves compara la clave del espacio con las APIs por aplicación: cuántas hay, qué pueden y qué pasa al regenerarlas.
export const claves: Tema = {
  id: 'claves',
  icono: 'o_key',
  titulo: 'ayuda.claves.titulo',
  resumen: 'ayuda.claves.resumen',
  terminal: 'ayuda.claves.terminal',
  conDiagrama: false,
  relacionados: ['conectar', 'espacios', 'presupuestos'],
  bloques: [
    {
      tipo: 'tabla',
      columnas: ['ayuda.claves.cEspacio', 'ayuda.claves.cApp'],
      filas: [
        { titulo: 'ayuda.claves.fCuantas', celdas: ['ayuda.claves.espacioCuantas', 'ayuda.claves.appCuantas'] },
        { titulo: 'ayuda.claves.fPuede', celdas: ['ayuda.claves.espacioPuede', 'ayuda.claves.appPuede'] },
        { titulo: 'ayuda.claves.fPara', celdas: ['ayuda.claves.espacioPara', 'ayuda.claves.appPara'] },
        { titulo: 'ayuda.claves.fRegenerar', celdas: ['ayuda.claves.espacioRegenerar', 'ayuda.claves.appRegenerar'] },
      ],
    },
    { tipo: 'nota', texto: 'ayuda.claves.nota' },
    {
      tipo: 'preguntas',
      preguntas: [
        { id: 'que-clave', pregunta: 'ayuda.claves.pCual', respuesta: 'ayuda.claves.rCual' },
        { id: 'clave-filtrada-espacio', pregunta: 'ayuda.claves.pFiltrada', respuesta: 'ayuda.claves.rFiltrada' },
        { id: 'sugerencia-api', pregunta: 'ayuda.claves.pSugerencia', respuesta: 'ayuda.claves.rSugerencia' },
      ],
    },
    { tipo: 'ir', pagina: 'conectar' },
  ],
}
