import { test } from 'node:test'
import assert from 'node:assert/strict'
import { aceptar, listaFormatos, necesitaConversor, seguirPreparacion } from './conversor.ts'
import type { EstadoConversor } from './api/tipos.ts'

test('necesitan el conversor HTML, PDF y Office; el texto y el ZIP no', () => {
  for (const n of ['a.PDF', 'b.docx', 'c.xlsx', 'd.pptx', 'e.html', 'f.htm']) assert.equal(necesitaConversor(n), true, n)
  for (const n of ['a.md', 'b.txt', 'c.json', 'd.csv', 'g.zip', 'sin-extension']) assert.equal(necesitaConversor(n), false, n)
})

test('el selector y la lista de formatos salen de los límites del servicio', () => {
  const limites = { extensiones_texto: ['.md', '.txt'], extensiones_conversor: ['.docx', '.pdf'] }
  assert.equal(aceptar(limites), '.docx,.pdf,.md,.txt')
  assert.equal(listaFormatos(limites), 'DOCX, PDF, MD, TXT')
  // Sin límites (un servicio anterior) no se filtra nada: decide el servicio.
  assert.equal(aceptar(undefined), undefined)
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

test('se sigue consultando mientras se prepara uno nuevo con el anterior listo', async () => {
  const estados: EstadoConversor[] = [
    { estado: 'listo', origen: 'preparado', etapa: 'paquetes', progreso: 50, preparar_al_iniciar: true },
    { estado: 'listo', origen: 'preparado', version: '0.1.8', preparar_al_iniciar: true },
  ]
  let consultas = 0
  const final = await seguirPreparacion(async () => (consultas++, estados.shift()!), () => {}, async () => {})
  assert.equal(consultas, 2)
  assert.equal(final.etapa, undefined)
})
