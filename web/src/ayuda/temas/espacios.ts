import type { Tema } from '../tipos'

// espacios explica la global y los proyectos: qué guarda cada uno, qué heredan y cómo se gestionan.
export const espacios: Tema = {
  id: 'espacios',
  icono: 'o_workspaces',
  titulo: 'ayuda.espacios.titulo',
  resumen: 'ayuda.espacios.resumen',
  paginaApp: 'espacios',
  conDiagrama: false,
  relacionados: ['conceptos', 'ajustes'],
  bloques: [{ tipo: 'ir', pagina: 'espacios' }],
}
