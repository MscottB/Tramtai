// node --test web/src/*.test.ts (Node ejecuta TypeScript sin compilar).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { aEsquema, desdeEsquema, ejemploDe, problemaDeCampos, type Campo } from './esquemaCampos.ts'
import { plantillas } from './plantillas.ts'

const campos: Campo[] = [
  { nombre: 'nombre', tipo: 'texto', obligatorio: true, descripcion: 'Quién' },
  { nombre: 'total', tipo: 'numero', obligatorio: false, descripcion: '', minimo: 0 },
  { nombre: 'unidades', tipo: 'entero', obligatorio: true, descripcion: '', minimo: 1, maximo: 10 },
  { nombre: 'pagado', tipo: 'si_no', obligatorio: true, descripcion: '' },
  { nombre: 'fecha', tipo: 'fecha', obligatorio: true, descripcion: '' },
  { nombre: 'estado', tipo: 'opcion', obligatorio: true, descripcion: '', opciones: ['abierto', 'cerrado'] },
  { nombre: 'etiquetas', tipo: 'lista', elemento: 'texto', obligatorio: false, descripcion: '', minimo: 1, maximo: 5 },
  { nombre: 'notas', tipo: 'lista', elemento: 'numero', obligatorio: true, descripcion: '' },
  { nombre: 'cliente', tipo: 'objeto', obligatorio: true, descripcion: 'Datos', campos: [
    { nombre: 'direccion', tipo: 'objeto', obligatorio: false, descripcion: '', campos: [{ nombre: 'calle', tipo: 'texto', obligatorio: true, descripcion: '' }] },
  ] },
  { nombre: 'lineas', tipo: 'lista', elemento: 'objeto', obligatorio: true, descripcion: '', campos: [{ nombre: 'precio', tipo: 'numero', obligatorio: true, descripcion: '' }] },
]

test('de los campos al esquema y de vuelta, idéntico', () => {
  const esquema = aEsquema(campos)
  assert.equal(esquema.additionalProperties, false)
  assert.deepEqual(esquema.required, ['nombre', 'unidades', 'pagado', 'fecha', 'estado', 'notas', 'cliente', 'lineas'])
  assert.deepEqual((esquema.properties as Record<string, unknown>).fecha, { type: 'string', format: 'date' })
  assert.deepEqual(desdeEsquema(JSON.parse(JSON.stringify(esquema))), campos)
})

test('un esquema avanzado se queda en JSON', () => {
  const avanzados: unknown[] = [
    { type: 'object', properties: { a: { type: 'string' } } }, // sin additionalProperties: false
    { type: 'object', additionalProperties: false, properties: { a: { type: 'string', pattern: '^x' } } },
    { type: 'object', additionalProperties: false, properties: { a: { oneOf: [{ type: 'string' }, { type: 'number' }] } } },
    { type: 'object', additionalProperties: false, properties: { a: { $ref: '#/$defs/x' } } },
    { type: 'object', additionalProperties: false, required: ['b'], properties: { a: { type: 'string' } } },
    { type: 'array', items: { type: 'string' } },
    { $schema: 'https://json-schema.org/draft/2020-12/schema', type: 'object', additionalProperties: false, properties: {} },
  ]
  for (const e of avanzados) assert.equal(desdeEsquema(e), null, JSON.stringify(e))
  // Cuatro niveles de objetos: el editor llega a tres.
  const nivel = (n: number): unknown => (n === 0 ? { type: 'string' } : { type: 'object', additionalProperties: false, properties: { x: nivel(n - 1) } })
  assert.notEqual(desdeEsquema(nivel(3)), null)
  assert.equal(desdeEsquema(nivel(4)), null)
})

test('cada plantilla genera un esquema de objeto que el editor representa', () => {
  assert.equal(plantillas.length, 7)
  for (const p of plantillas) {
    for (const idioma of ['es', 'en'] as const) {
      const c = p.campos(idioma)
      assert.equal(problemaDeCampos(c), null, p.id)
      const esquema = aEsquema(c)
      assert.equal(esquema.type, 'object')
      assert.ok(Array.isArray(esquema.required) && esquema.required.length > 0, p.id)
      assert.deepEqual(desdeEsquema(esquema), c, p.id)
      assert.ok(p.instrucciones(idioma).length > 0)
      assert.deepEqual(cumple(esquema, ejemploDe(c)), [], `${p.id}: el ejemplo cumple el esquema`)
    }
  }
  assert.deepEqual(Object.keys(aEsquema(plantillas[0].campos('en')).properties as object), ['name', 'date', 'total'])
})

test('el ejemplo cumple el esquema', () => {
  assert.deepEqual(cumple(aEsquema(campos), ejemploDe(campos)), [])
})

test('problemas de los campos', () => {
  assert.deepEqual(problemaDeCampos([{ nombre: '', tipo: 'texto', obligatorio: true, descripcion: '' }]), { campo: '', problema: 'sin_nombre' })
  const a: Campo = { nombre: 'a', tipo: 'texto', obligatorio: true, descripcion: '' }
  assert.deepEqual(problemaDeCampos([a, a]), { campo: 'a', problema: 'repetido' })
  assert.deepEqual(problemaDeCampos([{ ...a, tipo: 'opcion', opciones: [' '] }]), { campo: 'a', problema: 'sin_opciones' })
  assert.deepEqual(problemaDeCampos([{ ...a, tipo: 'objeto', campos: [] }]), { campo: 'a', problema: 'sin_campos' })
})

// cumple es una comprobación sencilla de lo que genera aEsquema: tipos, enum, obligatorios, sobrantes y tamaño.
function cumple(e: Record<string, unknown>, v: unknown, ruta = ''): string[] {
  const tipos: Record<string, (x: unknown) => boolean> = {
    string: (x) => typeof x === 'string', number: (x) => typeof x === 'number', integer: Number.isInteger,
    boolean: (x) => typeof x === 'boolean', array: Array.isArray, object: (x) => !!x && typeof x === 'object' && !Array.isArray(x),
  }
  if (!tipos[e.type as string](v)) return [`${ruta}: tipo`]
  const fallos: string[] = []
  if (e.enum && !(e.enum as unknown[]).includes(v)) fallos.push(`${ruta}: enum`)
  if (e.format === 'date' && !/^\d{4}-\d{2}-\d{2}$/.test(v as string)) fallos.push(`${ruta}: fecha`)
  if (typeof e.minimum === 'number' && (v as number) < e.minimum) fallos.push(`${ruta}: mínimo`)
  if (typeof e.maximum === 'number' && (v as number) > e.maximum) fallos.push(`${ruta}: máximo`)
  if (e.type === 'array') {
    const lista = v as unknown[]
    if (typeof e.minItems === 'number' && lista.length < e.minItems) fallos.push(`${ruta}: minItems`)
    if (typeof e.maxItems === 'number' && lista.length > e.maxItems) fallos.push(`${ruta}: maxItems`)
    lista.forEach((x, i) => fallos.push(...cumple(e.items as Record<string, unknown>, x, `${ruta}[${i}]`)))
  }
  if (e.type === 'object') {
    const o = v as Record<string, unknown>
    const props = e.properties as Record<string, Record<string, unknown>>
    for (const r of (e.required as string[] | undefined) ?? []) if (!(r in o)) fallos.push(`${ruta}.${r}: falta`)
    for (const [k, x] of Object.entries(o)) {
      if (!props[k]) fallos.push(`${ruta}.${k}: sobra`)
      else fallos.push(...cumple(props[k], x, `${ruta}.${k}`))
    }
  }
  return fallos
}
