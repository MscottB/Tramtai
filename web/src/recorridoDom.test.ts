// La parte del motor de los recorridos que no necesita navegador: qué elemento se señala y qué dice cada paso.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { aDriver, elegirElemento } from './recorridoDom.ts'
import type { PasoMostrado } from './recorrido.ts'

// elemento falso con el tamaño que devolvería getBoundingClientRect().
const elemento = (width: number, height: number) => ({ getBoundingClientRect: () => ({ width, height }) }) as unknown as Element
// raiz falsa: cada ancla devuelve sus elementos, como querySelectorAll('[data-recorrido="…"]').
const raiz = (anclas: Record<string, Element[]>) => ({
  querySelectorAll: (selector: string) => anclas[/data-recorrido="([^"]+)"/.exec(selector)?.[1] ?? ''] ?? [],
}) as unknown as Pick<Document, 'querySelectorAll'>

test('elegirElemento devuelve el primer elemento visible del ancla', () => {
  const visible = elemento(40, 20)
  assert.equal(elegirElemento('app.ayuda', raiz({ 'app.ayuda': [visible] })), visible)
  // una cabecera oculta (0×0) no gana a la visible que viene detrás
  assert.equal(elegirElemento('app.ayuda', raiz({ 'app.ayuda': [elemento(0, 0), visible] })), visible)
})

test('elegirElemento sin ancla, sin elemento o sin tamaño da undefined: el paso sale centrado', () => {
  assert.equal(elegirElemento(undefined, raiz({})), undefined)
  assert.equal(elegirElemento('inicio.mapa', raiz({})), undefined)
  assert.equal(elegirElemento('inicio.mapa', raiz({ 'inicio.mapa': [elemento(0, 0)] })), undefined)
  assert.equal(elegirElemento('inicio.mapa', raiz({ 'inicio.mapa': [elemento(120, 0)] })), undefined)
})

const paso: PasoMostrado = { pagina: 'inicio', ancla: 'inicio.mapa', titulo: 'comun.aceptar', texto: 'comun.cancelar' }

test('aDriver traduce con la t que recibe: el idioma se resuelve al mostrar el paso', () => {
  const es = aDriver(paso, 0, 3, (k) => `es:${k}`).popover
  const en = aDriver(paso, 0, 3, (k) => `en:${k}`).popover
  assert.equal(es?.title, 'es:comun.aceptar')
  assert.equal(es?.description, 'es:comun.cancelar')
  assert.equal(en?.title, 'en:comun.aceptar')
  assert.equal(es?.prevBtnText, 'es:recorrido.anterior')
  assert.equal(es?.closeBtnLabel, 'es:recorrido.cerrar')
})

test('aDriver pone el contador «3 de 14» y «Hecho» solo en el último paso', () => {
  const t = (k: string, p?: Record<string, string | number>) => (k === 'recorrido.progreso' ? `${p?.actual} de ${p?.total}` : k)
  assert.equal(aDriver(paso, 2, 14, t).popover?.progressText, '3 de 14')
  assert.equal(aDriver(paso, 2, 14, t).popover?.nextBtnText, 'recorrido.siguiente')
  assert.equal(aDriver(paso, 13, 14, t).popover?.nextBtnText, 'recorrido.hecho')
})
