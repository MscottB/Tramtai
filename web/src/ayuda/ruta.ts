// ruta traduce la dirección de la app (#pagina o #ayuda/<tema>) y la arma; App.vue navega con ella.
import { temaPorId } from './indice'

const PAGINAS = ['inicio', 'agentes', 'ejecutar', 'conectar', 'historial', 'configuracion', 'ayuda']
// ANTIGUAS son direcciones de versiones anteriores de la Web UI.
const ANTIGUAS: Record<string, string> = { panel: 'inicio', trabajos: 'historial', accesos: 'conectar' }

export function leerHash(hash: string): { pagina: string; tema?: string } {
  const ruta = hash.replace(/^#/, '')
  const corte = ruta.indexOf('/')
  const id = corte < 0 ? ruta : ruta.slice(0, corte)
  const tema = corte < 0 ? '' : ruta.slice(corte + 1)
  const pagina = ANTIGUAS[id] ?? (PAGINAS.includes(id) ? id : 'inicio')
  return pagina === 'ayuda' && tema && temaPorId(tema) ? { pagina, tema } : { pagina }
}

export function hashDe(pagina: string, tema?: string): string {
  return '#' + pagina + (tema ? '/' + tema : '')
}
