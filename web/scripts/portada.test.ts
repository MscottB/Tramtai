import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { pasosBienvenida } from '../src/recorrido.ts'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'

const html = readFileSync(new URL('../../site/index.html', import.meta.url), 'utf8')
const seccion = (id: string) => html.match(new RegExp(`<section[^>]*id="${id}"[^>]*>[\\s\\S]*?</section>`))?.[0] ?? ''
const asi = seccion('capturas')
const demo = seccion('demo')
const pasos = pasosBienvenida()
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

// Lighthouse (rendimiento móvil de la portada): medir el tranvía metiendo un path en el svg forzaba un reflow de toda la
// página al cargar. Ahora se mide fuera de la página y todo el trabajo espera a que el mapa se acerque a la pantalla.
const js = readFileSync(new URL('../../site/sitio.js', import.meta.url), 'utf8')
const tranvia = js.slice(js.indexOf('// Recorrido de la portada'), js.indexOf('// Demo del recorrido guiado'))

test('el tranvía mide los caminos fuera de la página y no arranca hasta que el mapa está cerca', () => {
  assert.ok(tranvia.length > 500, 'no se encuentra el recorrido del tranvía en sitio.js')
  assert.match(tranvia, /try \{ l = p\.getTotalLength\(\); \} catch/, 'primero mide el path suelto, sin tocar el svg')
  assert.match(tranvia, /medidas\[d\]/, 'cada camino se mide una sola vez')
  assert.doesNotMatch(tranvia, /^\s*recorrer\(0\);\s*$/m, 'recorrer(0) no se llama al cargar')
  assert.match(tranvia, /new IntersectionObserver[\s\S]*if \(!iniciado\) recorrer\(actual\)/, 'arranca al acercarse')
  assert.match(tranvia, /else if \(iniciado\) recorrer\(actual\)/, 'volver a la pestaña no arranca un tranvía que aún no se ve')
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
  const imgs = (asi.match(/<img [^>]*src="capturas\/[^>]*>/g) ?? [])
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

// La demo del recorrido guiado: una figura por paso de la bienvenida, con la app a pantalla completa y el recorrido encima.
const carpeta = new URL('../../site/capturas/', import.meta.url)
const figuras = demo.match(/<figure class="captura demo-paso"[\s\S]*?<\/figure>/g) ?? []

test('la demo es una figura por paso de la bienvenida, cada una con su captura en claro y oscuro y en los dos idiomas', () => {
  assert.ok(demo, 'falta <section id="demo">')
  assert.equal(figuras.length, pasos.length, 'una figura por paso')
  figuras.forEach((figura, i) => {
    const n = i + 1
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en']) {
        const base = `capturas/bienvenida-${n}-${tema}-${idioma}`
        // Cada imagen lleva su idioma (lang) y su alt en ese idioma: el CSS oculta la del otro, como en la Ayuda.
        assert.match(figura, new RegExp(`<img class="solo-${tema}" lang="${idioma}" src="${base}-1440\\.webp" srcset="${base}-960\\.webp 960w, ${base}-1440\\.webp 1440w" sizes="[^"]+" width="1440" height="900" loading="lazy" decoding="async" alt="[^"]+">`), `paso ${n}, ${tema}, ${idioma}`)
      }
  })
  // La primera se ve; el resto espera su turno (sin JavaScript queda la primera y la lista de pasos).
  assert.doesNotMatch(figuras[0], /^<figure[^>]* hidden/)
  for (const figura of figuras.slice(1)) assert.match(figura, /^<figure[^>]* hidden/)
})

test('la lista de pasos de la demo usa los títulos reales del recorrido, en los dos idiomas', () => {
  const lista = demo.match(/<ol class="demo-lista">([\s\S]*?)<\/ol>/)?.[1] ?? ''
  const titulos = [...lista.matchAll(/<li><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/li>/g)].map((m) => [m[1], m[2]])
  assert.deepEqual(titulos, pasos.map((p) => [es[p.titulo], en[p.titulo]]))
})

test('la demo se puede pausar y elegir, sin anunciarse sola y con los controles en el idioma de la página', () => {
  assert.match(demo, /<button type="button" class="demo-pausa">/)
  assert.doesNotMatch(demo, /class="demo-pausa"[^>]* aria-pressed/, 'la pausa cambia su texto: sin aria-pressed')
  assert.match(demo, /<p class="demo-leyenda" aria-live="off">/, 'mientras avanza sola, la leyenda no se anuncia cada 6 s')
  const puntos = demo.match(/<button type="button" class="demo-punto"[^>]*>[\s\S]*?<\/button>/g) ?? []
  assert.equal(puntos.length, pasos.length)
  puntos.forEach((punto, i) => {
    assert.doesNotMatch(punto, /aria-label/, 'el nombre del punto va en spans por idioma, no en los dos a la vez')
    assert.match(punto, new RegExp(`<span lang="es" class="solo-lectores">Paso ${i + 1}</span><span lang="en" class="solo-lectores">Step ${i + 1}</span>`))
  })
})

test('las capturas de la demo existen y pesan poco: 150 KB cada una de 1440 y 6 MB en total', () => {
  const archivos = readdirSync(carpeta).filter((a) => a.startsWith('bienvenida-'))
  let total = 0
  for (let n = 1; n <= pasos.length; n++)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const ancho of [960, 1440]) {
          const nombre = `bienvenida-${n}-${tema}-${idioma}-${ancho}.webp`
          assert.ok(archivos.includes(nombre), nombre)
          const peso = statSync(new URL(nombre, carpeta)).size
          total += peso
          if (ancho === 1440) assert.ok(peso <= 150 * 1024, `${nombre} pesa ${peso} bytes`)
        }
  assert.equal(archivos.length, pasos.length * 8, 'sobran capturas de la demo')
  assert.ok(total <= 6 * 1024 * 1024, `la demo pesa ${total} bytes`)
})

test('la portada estrena el recorrido guiado y deja Espacios a la vista', () => {
  const funciones = seccion('funciones')
  assert.match(funciones, /Aprende la app en 2 minutos/)
  assert.match(funciones, /Learn the app in 2 minutes/)
  assert.match(funciones, /href="#demo"/, 'Funciones lleva a la demo')
  assert.match(asi, /href="#demo"/, '«Así se ve» enlaza la demo')
  const entrada = html.match(/<p class="entrada">([\s\S]*?)<\/p>/)?.[1] ?? ''
  assert.match(entrada, /espacio/i)
  assert.match(entrada, /workspace/i)
  assert.match(funciones, /Controla<\/span><span lang="en">Control<\/span><\/h3>\s*<article class="tarjeta">\s*<div class="linea"[^>]*><\/div>\s*<h4><span lang="es">Espacios de trabajo<\/span>/, 'Espacios abre el grupo Controla')
})
