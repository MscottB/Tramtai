import { test } from 'node:test'
import assert from 'node:assert/strict'
import { necesitaConversor, seguirPreparacion } from './conversor.ts'
import type { EstadoConversor } from './api/tipos.ts'

test('necesitan el conversor HTML, PDF, Office y ZIP; el texto no', () => {
  for (const n of ['a.PDF', 'b.docx', 'c.xlsx', 'd.pptx', 'e.html', 'f.htm', 'g.zip']) assert.equal(necesitaConversor(n), true, n)
  for (const n of ['a.md', 'b.txt', 'c.json', 'd.csv', 'sin-extension']) assert.equal(necesitaConversor(n), false, n)
})

test('seguir una preparación consulta hasta que deja de preparar', async () => {
  const estados: EstadoConversor[] = [
    { estado: 'preparando', etapa: 'python', progreso: 10, preparar_al_iniciar: true },
    { estado: 'preparando', etapa: 'paquetes', progreso: 60, preparar_al_iniciar: true },
    { estado: 'listo', origen: 'preparado', version: '0.1.8', preparar_al_iniciar: true },
  ]
  const vistos: string[] = []
  let esperas = 0
  const final = await seguirPreparacion(async () => estados.shift()!, (e) => vistos.push(e.estado), async () => { esperas++ })
  assert.equal(final.estado, 'listo')
  assert.deepEqual(vistos, ['preparando', 'preparando', 'listo'])
  assert.equal(esperas, 2)
})
