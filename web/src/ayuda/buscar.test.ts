import { test } from 'node:test'
import assert from 'node:assert/strict'
import es from '../idiomas/es.ts'
import en from '../idiomas/en.ts'
import { temas } from './indice.ts'
import { buscar, normalizar } from './buscar.ts'

const te = (c: string) => (es as Record<string, string>)['error.' + c] ?? c

test('normalizar quita tildes, mayúsculas y espacios de más', () => {
  assert.equal(normalizar('  Conversión   ÉXITO '), 'conversion exito')
})

test('sin tildes ni mayúsculas', () => {
  assert.ok(buscar('CONVERSION', temas, es, te).some((r) => r.tema === 'archivos'))
})

test('el título pesa más', () => {
  const r = buscar('archivos', temas, es, te)[0]
  assert.equal(r.tema, 'archivos')
  assert.equal(r.ancla, '')
})

test('por código de error lleva a su ancla', () => {
  assert.ok(buscar('cola_llena', temas, es, te).some((r) => r.tema === 'problemas' && r.ancla === 'e-cola_llena'))
})

test('por pregunta lleva a su ancla y dice dónde', () => {
  const r = buscar('imágenes', temas, es, te).find((x) => x.ancla === 'imagenes')
  assert.ok(r)
  assert.equal(r.donde, '¿Puedo enviar imágenes?')
})

test('busca por palabras, en cualquier orden, y el guion bajo cuenta como espacio', () => {
  assert.ok(buscar('cola llena', temas, es, te).some((r) => r.ancla === 'e-cola_llena'))
  assert.ok(buscar('llena cola', temas, es, te).some((r) => r.ancla === 'e-cola_llena'))
  assert.ok(buscar('clave   filtra', temas, es, te).some((r) => r.ancla === 'clave-filtrada'))
})

test('encuentra con otra forma de la palabra y dentro de las tablas', () => {
  assert.ok(buscar('clave filtrada', temas, es, te).some((r) => r.ancla === 'clave-filtrada'))
  assert.ok(buscar((es as Record<string, string>)['guia.fLimites'], temas, es, te).some((r) => r.tema === 'conectar'))
})

test('vacía o solo espacios no devuelve nada', () => {
  assert.deepEqual(buscar('   ', temas, es, te), [])
  assert.deepEqual(buscar('', temas, es, te), [])
})

test('caracteres de expresiones regulares no rompen', () => {
  assert.doesNotThrow(() => buscar('(*?[', temas, es, te))
})

test('en inglés también', () => {
  assert.ok(buscar('files', temas, en, te).some((r) => r.tema === 'archivos'))
})

test('como mucho 30 resultados, sin repetir tema y ancla', () => {
  const r = buscar('a', temas, es, te)
  assert.ok(r.length <= 30)
  assert.equal(new Set(r.map((x) => x.tema + '#' + x.ancla)).size, r.length)
})
