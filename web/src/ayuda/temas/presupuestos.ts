import type { Tema } from '../tipos'

// presupuestos explica los límites de consumo de una API o del espacio.
export const presupuestos: Tema = {
  id: 'presupuestos',
  icono: 'o_savings',
  titulo: 'ayuda.presupuestos.titulo',
  resumen: 'ayuda.presupuestos.resumen',
  conDiagrama: true,
  relacionados: ['conectar', 'historial', 'conceptos'],
  bloques: [
    {
      tipo: 'diagrama',
      paradas: [
        { icono: 'o_query_stats', titulo: 'ayuda.presupuestos.d1', nota: 'ayuda.presupuestos.d1Nota' },
        { icono: 'o_warning', titulo: 'ayuda.presupuestos.d2', nota: 'ayuda.presupuestos.d2Nota' },
        { icono: 'o_block', titulo: 'ayuda.presupuestos.d3', nota: 'ayuda.presupuestos.d3Nota' },
      ],
    },
    { tipo: 'pasos', pasos: ['ayuda.presupuestos.paso1', 'ayuda.presupuestos.paso2', 'ayuda.presupuestos.paso3'] },
    { tipo: 'captura', id: 'presupuesto', alt: 'ayuda.presupuestos.captura' },
    { tipo: 'nota', texto: 'ayuda.presupuestos.nota' },
    { tipo: 'errores', codigos: ['presupuesto_agotado'] },
    { tipo: 'ir', pagina: 'conectar' },
  ],
}
