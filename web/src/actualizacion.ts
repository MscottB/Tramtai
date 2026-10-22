// abandonarSinFase decide si se deja de seguir una actualización cuyo progreso llega sin fase. Si ya se vio una fase,
// terminó o falló en otra pestaña. Si no, el servicio aún consulta la versión: se espera 30 s (75 consultas de 400 ms)
// por si esa consulta falla.
export function abandonarSinFase(vistaFase: boolean, consultasSinFase: number): boolean {
  return vistaFase || consultasSinFase >= 75
}
