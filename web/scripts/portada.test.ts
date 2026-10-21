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

const motivos = html.match(/<ul class="motivos">([\s\S]*?)<\/ul>/)?.[1] ?? ''

test('«Por qué Tramtai» son seis motivos concretos, cada uno con su icono', () => {
  const titulos = [...motivos.matchAll(/<h3><span lang="es">([^<]+)<\/span><span lang="en">[^<]+<\/span><\/h3>/g)].map((m) => m[1])
  assert.deepEqual(titulos, ['Lo que ya pagas', 'Local', 'Compatible', 'Sin cortes', 'Por proyecto', 'Bajo control'])
  assert.equal((motivos.match(/<svg /g) ?? []).length, 6)
})

test('la franja compara sin Tramtai y con Tramtai', () => {
  const franja = html.match(/<div class="franja">([\s\S]*?)<\/div>\s*<\/div>/)?.[1] ?? ''
  for (const t of ['Sin Tramtai', 'Con Tramtai', 'Without Tramtai', 'With Tramtai']) assert.ok(franja.includes(t), `falta «${t}»`)
})

test('las capturas son WebP con tamaño declarado, srcset y carga diferida', () => {
  const imgs = html.match(/<img [^>]*src="capturas\/[^>]*>/g) ?? []
  assert.equal(imgs.length, 10, 'cinco pantallas, en claro y oscuro')
  for (const img of imgs) {
    assert.match(img, /src="capturas\/[a-z-]+-(claro|oscuro)-1440\.webp"/)
    assert.match(img, /width="1440" height="900"/)
    assert.match(img, /srcset="capturas\/[a-z-]+-(claro|oscuro)-960\.webp 960w, capturas\/[a-z-]+-(claro|oscuro)-1440\.webp 1440w"/)
    assert.match(img, /sizes="/)
    assert.match(img, /loading="lazy"/, 'ninguna está en la primera pantalla')
  }
})

test('la portada cuenta el mapa en vivo y no habla de la «API principal»', () => {
  const inicio = html.match(/<article class="par">\s*<div>\s*<span class="etiqueta"><span lang="es">Inicio[\s\S]*?<\/article>/)?.[0] ?? ''
  assert.match(inicio, /en vivo/)
  assert.match(inicio, /live/)
  for (const pagina of ['index.html', 'desarrolladores.html'])
    assert.doesNotMatch(readFileSync(new URL(`../../site/${pagina}`, import.meta.url), 'utf8'), /API principal|main API/i, pagina)
})
