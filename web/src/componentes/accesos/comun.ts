// Lo que comparten la página Mis APIs y sus componentes: los contratos, si una API está activa y las preferencias
// que se recuerdan en este navegador.
import { t } from '../../i18n'
import type { EstadoPunto } from '../../api/tipos'

export const contratos = ['openai', 'anthropic', 'nativa']
export const nombreContrato = (c: string) => ({ openai: 'OpenAI', anthropic: 'Anthropic', nativa: t('contrato.nativa') })[c] ?? c
export const activo = (p: EstadoPunto) => p.error !== 'desactivado'

// leer y guardar recuerdan en este navegador los grupos plegados y el orden; sin almacenamiento, lo predeterminado.
export function leer<T>(clave: string, porDefecto: T): T {
  try {
    const v = localStorage.getItem(clave)
    return v === null ? porDefecto : (JSON.parse(v) as T)
  } catch {
    return porDefecto
  }
}
export function guardar(clave: string, valor: unknown) {
  try {
    localStorage.setItem(clave, JSON.stringify(valor))
  } catch {
    // sin almacenamiento solo no se recuerda
  }
}
