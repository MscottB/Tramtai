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

test('el recorrido no se anuncia solo y sus controles hablan en el idioma de la página', () => {
  assert.match(html, /<p class="mapa-leyenda" aria-live="off">/, 'mientras rota solo, la leyenda no se anuncia cada 6,5 s')
  for (const punto of html.match(/<button type="button" class="mapa-punto"[^>]*>[\s\S]*?<\/button>/g) ?? []) {
    assert.doesNotMatch(punto, /aria-label/, 'el nombre del punto va en spans por idioma, no en los dos a la vez')
    assert.match(punto, /<span lang="es" class="solo-lectores">Ejemplo \d<\/span><span lang="en" class="solo-lectores">Example \d<\/span>/)
  }
  assert.doesNotMatch(html, /class="mapa-pausa" aria-pressed/, 'la pausa cambia su texto: sin aria-pressed')
})
