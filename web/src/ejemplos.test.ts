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
