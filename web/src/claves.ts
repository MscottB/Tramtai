// claves decide cuándo sugerir a las apps que usan la clave del espacio que tengan su propia API. Sin Vue, para poder
// probarlo.

// sugerirApiPropia: con más de 20 peticiones de apps con la clave del espacio en 24 h conviene darles su propia API
// (para limitarlas, medirlas y revocarlas por separado). No obliga a nada.
export function sugerirApiPropia(n?: number): boolean {
  return (n ?? 0) > 20
}
