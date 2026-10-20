import { test } from 'node:test'
import assert from 'node:assert/strict'
import { abrirAyuda, cerrarAyuda, panel } from './panel.ts'

test('abrir y cerrar el panel con un tema', () => {
  abrirAyuda('archivos')
  assert.deepEqual({ ...panel }, { abierto: true, tema: 'archivos', ancla: '' })
  cerrarAyuda()
  assert.equal(panel.abierto, false)
})

test('un tema que no existe no abre nada', () => {
  abrirAyuda('noexiste')
  assert.equal(panel.abierto, false)
})

test('el ancla se guarda y otro tema con el panel abierto lo cambia', () => {
  abrirAyuda('problemas', 'e-cola_llena')
  assert.equal(panel.ancla, 'e-cola_llena')
  abrirAyuda('json')
  assert.deepEqual({ ...panel }, { abierto: true, tema: 'json', ancla: '' })
  cerrarAyuda()
})
