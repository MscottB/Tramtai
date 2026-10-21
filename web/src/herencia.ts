// herencia dice, para Ajustes, si un valor de un proyecto es suyo o viene de Global, y cuáles se pueden devolver al de
// Global desde la app. Sin Vue, para poder probarlo.
import type { EntradaConfiguracion } from './api/tipos'

export type Procedencia = 'propio' | 'global'

// procedencia de clave en un proyecto: propio si lo define su config.yaml o su local.yaml; de Global si viene de la
// configuración global o del valor de fábrica. En la global (o del entorno, o sin el valor) no se marca.
export function procedencia(entradas: EntradaConfiguracion[], clave: string, ambito: 'global' | 'proyecto'): Procedencia | undefined {
  if (ambito !== 'proyecto') return undefined
  const origen = entradas.find((e) => e.clave === clave)?.origen
  if (origen === 'proyecto' || origen === 'local') return 'propio'
  if (origen === 'global' || origen === 'interno') return 'global'
  return undefined
}

// heredable: las claves que la app deja tener propias en un proyecto (POST /api/v1/instancia/heredar).
export function heredable(clave: string): boolean {
  return ['predeterminados.', 'retencion.', 'presupuesto.'].some((p) => clave.startsWith(p))
}
