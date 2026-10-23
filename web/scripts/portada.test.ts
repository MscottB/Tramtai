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

test('el tranvía pasa por debajo del cartel de Tramtai (se dibuja antes que la cápsula), en los dos mapas', () => {
  for (const [svg, tranvia] of [['mapa-ancho', 'tranvia'], ['mapa-alto', 'tranvia-alto']]) {
    const dibujo = html.match(new RegExp(`<svg class="${svg}"[\\s\\S]*?</svg>`))?.[0] ?? ''
    const tram = dibujo.indexOf(`class="${tranvia}"`)
    const capsula = dibujo.search(/<rect [^>]*stroke="var\(--texto\)"/)
    assert.ok(tram > 0 && capsula > 0, `${svg}: falta el tranvía o la cápsula`)
    assert.equal(dibujo.split(`class="${tranvia}"`).length - 1, 1, `${svg}: un solo tranvía`)
    assert.ok(tram < capsula, `${svg}: el tranvía se dibuja encima del cartel de Tramtai`)
  }
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
  assert.match(tranvia, /new IntersectionObserver[\s\S]*if \(!iniciado\) recorrer\(actual\)[\s\S]*\{ rootMargin: '200px 0px' \}/, 'arranca a 200 px de la pantalla')
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

test('«Por qué Tramtai» es la franja, sin motivos con iconos que repitan Funciones (auditoría 001, 53)', () => {
  assert.doesNotMatch(html, /class="motivos"/)
})

const css = readFileSync(new URL('../../site/estilo.css', import.meta.url), 'utf8')
const COLORES_LINEA = /var\(--(claude|codex|copilot|gemini|antigravity)\)/

test('el color de cada agente solo marca a su agente: mapa, fichas y tarjetas de Agentes (52)', () => {
  assert.doesNotMatch(html, /class="linea"/)
  const sinAgentes = html.replace(/<div class="mapa"[\s\S]*?<\/svg>\s*<\/div>/, '').replace(/<section class="conexiones">[\s\S]*?<\/section>/, '')
    .replace(/<section id="agentes">[\s\S]*?<\/section>/, '')
  assert.doesNotMatch(sinAgentes, COLORES_LINEA)
})

test('Antigravity va como vía muerta en el mapa y el titular cuenta cuatro agentes desde las apps (28)', () => {
  const mapa = html.match(/<div class="mapa"[\s\S]*?<\/svg>\s*<svg class="mapa-alto"[\s\S]*?<\/svg>/)?.[0] ?? ''
  assert.doesNotMatch(mapa, /var\(--antigravity\)/, 'sin línea de color continua')
  assert.equal((mapa.match(/stroke-dasharray/g) ?? []).length, 2, 'punteada en los dos mapas')
  assert.match(mapa, /fuera de tus apps/)
  assert.match(seccion('agentes'), /Cinco agentes; cuatro desde tus apps\./)
  assert.match(seccion('agentes'), /<article class="agente sin-apps">/)
})

test('el hero tiene un titular, dos frases y ninguna pastilla, rejilla ni brillo de fondo (56, 57, 59)', () => {
  const portada = html.match(/<section class="portada">[\s\S]*?<div class="mapa"/)?.[0] ?? ''
  assert.doesNotMatch(portada, /class="chapa"/)
  const entrada = (html.match(/<p class="entrada">\s*<span lang="es">([^<]*)<\/span>/)?.[1] ?? '').split('. ')
  assert.equal(entrada.length, 2, entrada.join(' | '))
  assert.doesNotMatch(css, /\.portada::before|repeating-linear-gradient|radial-gradient/)
})

test('las secciones no abren todas igual: sin etiqueta sobre el h2 y no todas centradas (35, 55)', () => {
  assert.doesNotMatch(html, /<div class="cabeza[^"]*">\s*<span class="etiqueta"/)
  const aperturas = [...html.matchAll(/<div class="cabeza( centrada)?">/g)]
  assert.ok(aperturas.filter((a) => !a[1]).length >= 3, 'al menos tres a la izquierda')
  assert.ok(aperturas.filter((a) => a[1]).length >= 1)
})

test('mayúsculas espaciadas solo en la cabecera del tablero (35) y ámbar solo para cifras (33)', () => {
  const reglas = [...css.matchAll(/([^{}]+)\{([^}]*)\}/g)].filter(([, , cuerpo]) => /text-transform:\s*uppercase/.test(cuerpo)).map(([, s]) => s.trim())
  assert.deepEqual(reglas, ['.tablero-cabecera'])
  assert.doesNotMatch(css + html, /--cifra|#ffcf5c/)
})

test('cada control de la barra, del mapa y de la demo mide 44 px y la barra no tapa las secciones (23, 62)', () => {
  for (const clase of ['.github', '.menu-boton', '.mapa-punto', '.demo-punto'])
    assert.match(css, new RegExp(`\\${clase} \\{[^}]*width: 44px; height: 44px`), clase)
  // .idioma button es ES/EN y tema: 44 de alto el botón, no el grupo con su borde (revisión final web, M13).
  for (const clase of ['.boton', '.copiar', '.mapa-pausa', '.demo-pausa', '.pestanas-lista button', '.idioma button'])
    assert.match(css, new RegExp(`${clase.replace('.', '\\.')} \\{[^}]*min-height: 44px`), clase)
  assert.match(css, /section\[id\] \{ scroll-margin-top: 72px; \}/)
})

test('el foco se ve: sin recortes en ES/EN y en verde claro dentro del código (20)', () => {
  assert.doesNotMatch(css, /\.idioma \{[^}]*overflow: hidden/)
  assert.match(css, /\.codigo :focus-visible \{ outline-color: var\(--tablero-clave\); \}/)
  assert.doesNotMatch(css, /:focus-visible \{[^}]*border-radius/)
})

test('sin JavaScript el menú se ve y el mapa queda quieto; el paso del cursor no levanta nada (60, 61)', () => {
  assert.match(css, /html\.js \.menu-panel \{ display: none;/)
  assert.doesNotMatch(css, /@keyframes/)
  assert.doesNotMatch(css, /translateY/)
})

test('junto a Descargar se avisa del CLI del agente y del aviso de Windows (58)', () => {
  const descargar = seccion('descargar')
  assert.match(descargar, /Antes necesitas el CLI de al menos un agente/)
  assert.match(descargar, /Windows protegió su PC/)
  assert.match(descargar, /Windows protected your PC/)
})

test('los comentarios del código salen en el idioma de la página y Copiar copia solo ese (59)', () => {
  for (const pre of html.match(/<pre class="codigo" id="codigo-mapa-\d"[\s\S]*?<\/pre>/g) ?? [])
    for (const c of pre.match(/<span class="c"[^>]*>/g) ?? []) assert.match(c, /lang="(es|en)"/, pre.slice(0, 60))
  assert.match(js, /\[lang\]:not\(\[lang="' \+ document\.documentElement\.lang \+ '"\]\)/)
})

test('la franja compara sin Tramtai y con Tramtai', () => {
  const franja = html.match(/<div class="franja">([\s\S]*?)<\/div>\s*<\/div>/)?.[1] ?? ''
  for (const t of ['Sin Tramtai', 'Con Tramtai', 'Without Tramtai', 'With Tramtai']) assert.ok(franja.includes(t), `falta «${t}»`)
})

// Revisión final web (I1, hallazgo 26): el titular vale para los tres agentes (Codex lee archivos: sí tiene
// herramientas) y la nota dice cuándo se pasó el canario (docs/equipo/m0-agentes.md).
test('la tarjeta del aislamiento no contradice a Codex y la nota de Agentes fecha el canario', () => {
  assert.doesNotMatch(html, /Agentes sin herramientas|Agents without tools/)
  const nota = html.match(/<p class="nota-agentes[\s\S]*?<\/p>/)?.[0] ?? ''
  assert.match(nota, /<span lang="es">[^<]*el 2026-10-18[^<]*<\/span>/)
  assert.match(nota, /<span lang="en">[^<]*on 2026-10-18[^<]*<\/span>/)
})

// Revisión final web (I3, hallazgo 64): las capturas se sacan de una Tramtai de prueba; la portada lo dice y aclara el
// puerto, porque los ejemplos de código usan el de por defecto.
test('«Así se ve» dice que las capturas tienen datos de ejemplo y aclara su puerto frente al de los ejemplos', () => {
  const nota = asi.match(/<p class="nota-capturas[\s\S]*?<\/p>/)?.[0] ?? ''
  assert.match(nota, /datos de ejemplo/)
  assert.match(nota, /sample data/)
  for (const [, puerto] of html.matchAll(/127\.0\.0\.1:(\d+)/g)) assert.equal(puerto, '17373', 'los ejemplos usan el puerto por defecto')
  assert.match(nota, /17373/)
})

test('las capturas son WebP con tamaño declarado, srcset y carga diferida', () => {
  const imgs = (asi.match(/<img [^>]*src="capturas\/[^>]*>/g) ?? [])
  assert.equal(imgs.length, 10, 'cinco pantallas, en claro y oscuro')
  for (const img of imgs) {
    assert.match(img, /src="capturas\/[a-z-]+-(claro|oscuro)-1440\.webp"/)
    assert.match(img, /width="1440" height="900"/)
    assert.match(img, /srcset="capturas\/[a-z-]+-(claro|oscuro)-640\.webp 640w, capturas\/[a-z-]+-(claro|oscuro)-960\.webp 960w, capturas\/[a-z-]+-(claro|oscuro)-1440\.webp 1440w"/)
    // Las dos columnas de un par (1fr y 1.6fr en 1120 px) dejan la captura en 630 px: el navegador elige la de 640.
    assert.match(img, /sizes="\(max-width: 900px\) 100vw, 630px"/)
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

const demoJs = js.slice(js.indexOf('// Demo del recorrido guiado'))

test('la leyenda cambia un momento después de su aria-live, en el tranvía y en la demo', () => {
  assert.match(js, /function escribirLeyenda\(leyenda, partes\) \{[\s\S]*?setTimeout/, 'el texto llega después del atributo')
  for (const [nombre, parte] of [['tranvía', tranvia], ['demo', demoJs]]) {
    assert.match(parte, /escribirLeyenda\(leyenda, /, nombre)
    assert.doesNotMatch(parte, /leyenda\.innerHTML = ''/, `${nombre}: la leyenda no se cambia en el acto`)
  }
})

test('«reducir movimiento» no pisa una pausa que eligió la persona', () => {
  for (const [nombre, parte] of [['tranvía', tranvia], ['demo', demoJs]]) {
    assert.match(parte, /pausaManual = pausado/, `${nombre}: la pausa de la persona se recuerda`)
    assert.match(parte, /pausado = (quieto|movimiento\.matches) \|\| pausaManual/, `${nombre}: al volver el movimiento sigue en pausa`)
  }
})

test('la demo precarga solo la imagen del paso siguiente que se va a ver', () => {
  assert.match(demoJs, /function precargar\(n\)/)
  assert.match(demoJs, /siguiente\.loading = 'eager'/)
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

test('la portada enseña el recorrido guiado y deja Espacios a la vista', () => {
  const funciones = seccion('funciones')
  assert.match(funciones, /Recorrido y Ayuda/)
  assert.match(funciones, /Tour and Help/)
  assert.match(funciones, /href="#demo"/, 'Funciones lleva a la demo')
  assert.match(asi, /href="#demo"/, '«Así se ve» enlaza la demo')
  assert.match(asi, /Un espacio por proyecto/)
  assert.match(funciones, /Controla<\/span><span lang="en">Control<\/span><\/h3>\s*<article class="tarjeta">\s*<h4><span lang="es">Espacios de trabajo<\/span>/, 'Espacios abre el grupo Controla')
})

test('Funciones son cuatro grupos de tres tarjetas en tres columnas, sin huecos (54)', () => {
  const grupos = seccion('funciones').split('<h3 class="grupo-funciones">').slice(1)
  assert.equal(grupos.length, 4)
  for (const g of grupos) assert.equal((g.match(/<article class="tarjeta">/g) ?? []).length, 3)
  assert.match(css, /\.rejilla \{ display: grid; grid-template-columns: repeat\(3, 1fr\)/)
})
