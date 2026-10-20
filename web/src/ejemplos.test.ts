import { test } from 'node:test'
import assert from 'node:assert/strict'
import { ejemplos, type Prueba } from './ejemplos.ts'

const prueba: Prueba = { archivos: ['informe.pdf'], solicitud: { mensajes: [{ rol: 'usuario', contenido: 'Resume el informe' }] } }
const todos = ejemplos({ url: 'http://127.0.0.1:17373', modelo: 'claude/sonnet', contratos: ['nativa', 'openai', 'anthropic'], prueba })
const codigo = (id: string) => todos.find((e) => e.id === id)!

test('OpenAI manda el archivo dentro del mensaje, en base64', () => {
  const python = codigo('python')
  assert.equal(python.nota, undefined)
  assert.match(python.codigo('k'), /base64/)
  assert.match(python.codigo('k'), /"type": "file"/)
  assert.match(python.codigo('k'), /archivo\("informe\.pdf", "application\/pdf"\)/)
  const curl = codigo('curl')
  assert.equal(curl.nota, undefined)
  assert.match(curl.codigo('k'), /base64 < 'informe\.pdf'/)
  assert.match(curl.codigo('k'), /--data-binary @-/)
})

test('Anthropic manda el archivo como bloque document', () => {
  const anthropic = codigo('anthropic')
  assert.equal(anthropic.nota, undefined)
  assert.match(anthropic.codigo('k'), /"type": "document"/)
  assert.match(anthropic.codigo('k'), /documento\("informe\.pdf", "application\/pdf"\)/)
})

test('la API propia sube el archivo a /cargas', () => {
  const curl = codigo('curl-nativa').codigo('k')
  assert.match(curl, /\/api\/v1\/cargas/)
  assert.match(curl, /informe\.pdf/)
})

test('Go por la API propia sube, crea, sigue y lee el resultado', () => {
  const go = codigo('go-nativa').codigo('k')
  for (const parte of ['net/http', '/api/v1"', '"/cargas"', '"/trabajos"', 'Authorization', '/eventos', '/salida', 'informe.pdf']) {
    assert.ok(go.includes(parte), parte)
  }
  // Sin archivos no hay paso de subida.
  const sinArchivos = ejemplos({ url: 'http://x', modelo: 'claude/sonnet', contratos: ['nativa'], prueba: { archivos: [], solicitud: { mensajes: [{ rol: 'usuario', contenido: 'Hola' }] } } })
    .find((e) => e.id === 'go-nativa')!.codigo('k')
  assert.ok(!sinArchivos.includes('/cargas'))
  assert.ok(!sinArchivos.includes('"bytes"'), 'sin archivos no importa bytes (no compilaría)')
})

test('el ejemplo de herramientas hace el ciclo completo por OpenAI', () => {
  const lista = ejemplos({ url: 'http://127.0.0.1:17373', modelo: 'claude/haiku', contratos: ['openai'] })
  const herramientas = lista.find((e) => e.id === 'python-herramientas')!.codigo('k')
  for (const parte of ['tools=', 'tool_calls', '"role": "tool"', 'http://127.0.0.1:17373/openai/v1', 'api_key="k"', 'model="claude/haiku"']) {
    assert.ok(herramientas.includes(parte), parte)
  }
  // Solo con OpenAI: una API que solo habla Anthropic no lo ofrece.
  assert.equal(ejemplos({ url: 'http://x', modelo: 'm', contratos: ['anthropic'] }).find((e) => e.id === 'python-herramientas'), undefined)
})

test('Go por la API propia no se rompe con comillas invertidas ni %', () => {
  const rara: Prueba = { archivos: ['a.pdf'], solicitud: { mensajes: [{ rol: 'usuario', contenido: 'usa `x` al 100%' }] } }
  const go = ejemplos({ url: 'http://x', modelo: 'm', contratos: ['nativa'], prueba: rara }).find((e) => e.id === 'go-nativa')!.codigo('k')
  assert.ok(!go.includes('Sprintf(solicitud'), 'no pasa el mensaje por un formato')
  assert.ok(!/solicitud := `/.test(go), 'no usa un literal con comillas invertidas')
  assert.ok(go.includes('usa `x` al 100%'), 'el mensaje llega intacto')
})
