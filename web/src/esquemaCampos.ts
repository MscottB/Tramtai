// El editor de campos de «Respuesta con forma»: convierte una lista de campos en un JSON Schema y de vuelta. Solo
// representa lo que genera (objetos con additionalProperties: false, hasta 3 niveles); con cualquier otra cosa
// desdeEsquema devuelve null y el esquema se sigue editando en JSON, sin perder nada. Sin dependencias de Vue: se
// prueba con node --test.

export type TipoCampo = 'texto' | 'numero' | 'entero' | 'si_no' | 'fecha' | 'opcion' | 'lista' | 'objeto'
export type Elemento = 'texto' | 'numero' | 'objeto'

export interface Campo {
  nombre: string
  tipo: TipoCampo
  obligatorio: boolean
  descripcion: string
  opciones?: string[] // opcion
  elemento?: Elemento // lista
  campos?: Campo[] // objeto, o lista de objetos
  minimo?: number // numero y entero (minimum); lista (minItems)
  maximo?: number // numero y entero (maximum); lista (maxItems)
}

export const NIVELES = 3

type Esquema = Record<string, unknown>

export function campoVacio(): Campo {
  return { nombre: '', tipo: 'texto', obligatorio: true, descripcion: '' }
}

// aEsquema genera el esquema de un objeto con esos campos.
export function aEsquema(campos: Campo[]): Esquema {
  const properties: Record<string, Esquema> = {}
  for (const c of campos) properties[c.nombre] = propiedad(c)
  const esquema: Esquema = { type: 'object', properties }
  const required = campos.filter((c) => c.obligatorio).map((c) => c.nombre)
  if (required.length) esquema.required = required
  esquema.additionalProperties = false
  return esquema
}

function propiedad(c: Campo): Esquema {
  const p: Esquema = {}
  switch (c.tipo) {
    case 'texto': p.type = 'string'; break
    case 'fecha': p.type = 'string'; p.format = 'date'; break
    case 'numero': p.type = 'number'; break
    case 'entero': p.type = 'integer'; break
    case 'si_no': p.type = 'boolean'; break
    case 'opcion': p.type = 'string'; p.enum = (c.opciones ?? []).filter((o) => o.trim()); break
    case 'objeto': Object.assign(p, aEsquema(c.campos ?? [])); break
    case 'lista':
      p.type = 'array'
      p.items = c.elemento === 'objeto' ? aEsquema(c.campos ?? []) : { type: c.elemento === 'numero' ? 'number' : 'string' }
      break
  }
  if (c.tipo === 'numero' || c.tipo === 'entero' || c.tipo === 'lista') {
    const [min, max] = c.tipo === 'lista' ? ['minItems', 'maxItems'] : ['minimum', 'maximum']
    if (c.minimo !== undefined) p[min] = c.minimo
    if (c.maximo !== undefined) p[max] = c.maximo
  }
  if (c.descripcion) p.description = c.descripcion
  return p
}

// desdeEsquema rehace los campos de un esquema, o null si usa algo que el editor no representa.
export function desdeEsquema(esquema: unknown, nivel = 1): Campo[] | null {
  if (!esObjeto(esquema) || nivel > NIVELES) return null
  if (!soloClaves(esquema, ['type', 'properties', 'required', 'additionalProperties', 'description'])) return null
  if (esquema.type !== 'object' || esquema.additionalProperties !== false || !esObjeto(esquema.properties)) return null
  const propiedades = esquema.properties
  const required = esquema.required ?? []
  if (!Array.isArray(required) || !required.every((r) => typeof r === 'string' && r in propiedades)) return null
  const campos: Campo[] = []
  for (const [nombre, p] of Object.entries(propiedades)) {
    const c = campoDe(nombre, p, required.includes(nombre), nivel)
    if (!c) return null
    campos.push(c)
  }
  return campos
}

function campoDe(nombre: string, p: unknown, obligatorio: boolean, nivel: number): Campo | null {
  if (!esObjeto(p) || (p.description !== undefined && typeof p.description !== 'string')) return null
  const c: Campo = { nombre, tipo: 'texto', obligatorio, descripcion: (p.description as string | undefined) ?? '' }
  // rango copia el mínimo y el máximo; false si alguno no es un número.
  const rango = (min: string, max: string) => {
    for (const [clave, destino] of [[min, 'minimo'], [max, 'maximo']] as const) {
      if (p[clave] === undefined) continue
      if (typeof p[clave] !== 'number') return false
      c[destino] = p[clave] as number
    }
    return true
  }
  switch (p.type) {
    case 'string':
      if (p.format === 'date' && soloClaves(p, ['type', 'format', 'description'])) return { ...c, tipo: 'fecha' }
      if (Array.isArray(p.enum) && p.enum.every((v) => typeof v === 'string') && soloClaves(p, ['type', 'enum', 'description'])) {
        return { ...c, tipo: 'opcion', opciones: [...(p.enum as string[])] }
      }
      return soloClaves(p, ['type', 'description']) ? c : null
    case 'number':
    case 'integer':
      c.tipo = p.type === 'number' ? 'numero' : 'entero'
      return soloClaves(p, ['type', 'minimum', 'maximum', 'description']) && rango('minimum', 'maximum') ? c : null
    case 'boolean':
      return soloClaves(p, ['type', 'description']) ? { ...c, tipo: 'si_no' } : null
    case 'object': {
      const campos = desdeEsquema(p, nivel + 1)
      return campos ? { ...c, tipo: 'objeto', campos } : null
    }
    case 'array': {
      if (!soloClaves(p, ['type', 'items', 'minItems', 'maxItems', 'description']) || !rango('minItems', 'maxItems') || !esObjeto(p.items)) return null
      c.tipo = 'lista'
      const items = p.items
      if (items.type === 'object') {
        const campos = desdeEsquema(items, nivel + 1)
        return campos ? { ...c, elemento: 'objeto', campos } : null
      }
      if ((items.type === 'string' || items.type === 'number') && soloClaves(items, ['type'])) {
        return { ...c, elemento: items.type === 'string' ? 'texto' : 'numero' }
      }
      return null
    }
  }
  return null
}

// ejemploDe es una respuesta de muestra con esa forma, para la vista previa.
export function ejemploDe(campos: Campo[]): Record<string, unknown> {
  const ejemplo: Record<string, unknown> = {}
  for (const c of campos) ejemplo[c.nombre] = valorDe(c)
  return ejemplo
}

function valorDe(c: Campo): unknown {
  switch (c.tipo) {
    case 'texto': return '…'
    case 'fecha': return '2026-01-31'
    case 'numero': return c.minimo ?? 0
    case 'entero': return Math.ceil(c.minimo ?? 0)
    case 'si_no': return true
    case 'opcion': return c.opciones?.find((o) => o.trim()) ?? ''
    case 'objeto': return ejemploDe(c.campos ?? [])
    case 'lista': {
      const elemento = () => (c.elemento === 'objeto' ? ejemploDe(c.campos ?? []) : c.elemento === 'numero' ? 0 : '…')
      return Array.from({ length: Math.max(c.minimo ?? 1, 1) }, elemento)
    }
  }
}

export type ProblemaCampo = { campo: string; problema: 'sin_nombre' | 'repetido' | 'sin_opciones' | 'sin_campos' }

// problemaDeCampos es el primer campo que impide generar un esquema útil, o null si no hay ninguno.
export function problemaDeCampos(campos: Campo[], ruta = ''): ProblemaCampo | null {
  const vistos = new Set<string>()
  for (const c of campos) {
    const aqui = ruta ? `${ruta}.${c.nombre}` : c.nombre
    if (!c.nombre.trim()) return { campo: ruta, problema: 'sin_nombre' }
    if (vistos.has(c.nombre)) return { campo: aqui, problema: 'repetido' }
    vistos.add(c.nombre)
    if (c.tipo === 'opcion' && !(c.opciones ?? []).some((o) => o.trim())) return { campo: aqui, problema: 'sin_opciones' }
    const sub = c.tipo === 'objeto' || (c.tipo === 'lista' && c.elemento === 'objeto')
    if (sub && !(c.campos ?? []).length) return { campo: aqui, problema: 'sin_campos' }
    if (sub) {
      const p = problemaDeCampos(c.campos ?? [], aqui)
      if (p) return p
    }
  }
  return null
}

function esObjeto(v: unknown): v is Esquema {
  return !!v && typeof v === 'object' && !Array.isArray(v)
}

function soloClaves(o: Esquema, permitidas: string[]): boolean {
  return Object.keys(o).every((k) => permitidas.includes(k))
}
