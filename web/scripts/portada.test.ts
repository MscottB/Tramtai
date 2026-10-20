import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const html = readFileSync(new URL('../../site/index.html', import.meta.url), 'utf8')
const datos = html.match(/<script type="application\/json" id="ejemplos-mapa">([\s\S]*?)<\/script>/)

test('la portada tiene cuatro ejemplos del recorrido, en los dos idiomas', () => {
  assert.ok(datos, 'falta el JSON de los ejemplos del mapa')
  const ejemplos = JSON.parse(datos[1]) as { origen: number; agentes: number[]; es: string; en: string; codigo: string }[]
  assert.equal(ejemplos.length, 4)
  for (const e of ejemplos) {
    assert.ok(e.es.trim() && e.en.trim(), `falta el texto de ${e.codigo}`)
    assert.ok([0, 1, 2].includes(e.origen), `origen ${e.origen}`)
    assert.ok(e.agentes.length >= 1 && e.agentes.every((a) => a >= 0 && a <= 4), `agentes ${e.agentes}`)
    assert.match(html, new RegExp(`<pre class="codigo[^"]*" id="${e.codigo}"`), `falta el bloque ${e.codigo}`)
  }
  assert.ok(ejemplos.some((e) => e.agentes.length === 2), 'falta el ejemplo de respaldo')
})

test('el recorrido se puede pausar y elegir', () => {
  assert.match(html, /class="mapa-pausa"/)
  assert.equal((html.match(/class="mapa-punto"/g) ?? []).length, 4)
})
