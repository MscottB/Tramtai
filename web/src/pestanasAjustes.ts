// Ajustes tiene dos pestañas y la que no se ve esconde sus elementos (v-show): un paso del recorrido que señala algo de
// la otra no lo encontraría. pestanaDeAncla dice en qué pestaña está cada ancla; sin pestaña propia (las pestañas
// mismas o las anclas de otras páginas) da undefined. Una prueba exige que toda ancla de Ajustes tenga la suya.
export type PestanaAjustes = 'espacio' | 'tramtai'

const PESTANAS: Record<string, PestanaAjustes> = {
  'configuracion.modelo': 'espacio',
  'configuracion.datos': 'espacio',
}

export const pestanaDeAncla = (ancla: string | undefined): PestanaAjustes | undefined => (ancla ? PESTANAS[ancla] : undefined)
