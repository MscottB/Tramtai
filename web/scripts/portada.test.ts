import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { pasosBienvenida } from '../src/recorrido.ts'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { PORTADA } from './capturas-lista.ts'

const html = readFileSync(new URL('../../site/index.html', import.meta.url), 'utf8')
// seccion respeta las <section> anidadas (las líneas del plano de Funciones lo son): corta en el </section> de la suya.
const seccion = (id: string) => {
  const inicio = html.search(new RegExp(`<section[^>]*id="${id}"`))
  if (inicio < 0) return ''
  const etiquetas = /<section\b|<\/section>/g
  etiquetas.lastIndex = inicio
  let nivel = 0
  for (let m = etiquetas.exec(html); m; m = etiquetas.exec(html)) {
    nivel += m[0] === '</section>' ? -1 : 1
    if (nivel === 0) return html.slice(inicio, m.index + m[0].length)
  }
  return ''
}
// v0.59: «La app» lleva las cinco capturas y, justo después, el recorrido guiado (#demo, dentro de la sección).
const app = seccion('app')
const corte = app.indexOf('<div id="demo"')
const asi = corte < 0 ? app : app.slice(0, corte)
const demo = corte < 0 ? '' : app.slice(corte)
const pasos = pasosBienvenida()

// v0.59 (spec §1): la portada pasa de 14 secciones a diez, en este orden.
test('la portada tiene diez secciones, en su orden', () => {
  const ids = [...html.matchAll(/<section\b[^>]*\bid="([^"]+)"/g)].map((m) => m[1])
  assert.deepEqual(ids, ['portada', 'por-que', 'como', 'app', 'terminal', 'servidor', 'agentes', 'funciones', 'privacidad', 'descargar'])
})

test('la barra lleva a Cómo funciona, Terminal, Servidor, Agentes, Ayuda y Desarrolladores', () => {
  const nav = html.match(/<nav aria-label="Secciones"[\s\S]*?<\/nav>/)?.[0] ?? ''
  assert.deepEqual([...nav.matchAll(/href="([^"]+)"/g)].map((m) => m[1]), ['#como', '#terminal', '#servidor', '#agentes', 'ayuda/', 'desarrolladores.html'])
  for (const t of ['Cómo funciona', 'How it works', 'Terminal', 'Servidor', 'Server', 'Agentes', 'Agents']) assert.ok(nav.includes(`>${t}<`), t)
})

test('bajo los botones de la portada, tres accesos: Escritorio, Terminal y Servidor', () => {
  const portada = seccion('portada')
  const accesos = portada.match(/<nav class="accesos"[\s\S]*?<\/nav>/)?.[0] ?? ''
  assert.ok(portada.indexOf('class="accesos"') > portada.indexOf('class="acciones"'), 'van después de los botones')
  assert.deepEqual([...accesos.matchAll(/href="([^"]+)"/g)].map((m) => m[1]), ['#app', '#terminal', '#servidor'])
  for (const t of ['Escritorio', 'Desktop', 'Terminal', 'Servidor', 'Server']) assert.ok(accesos.includes(`>${t}<`), t)
  assert.match(css, /\.accesos a \{[^}]*min-height: 44px/)
})

// v0.59 (spec §4): la franja de cuatro cifras del producto y la línea de seis paradas, en lugar de los seis motivos.
const porQue = seccion('por-que')
test('«Por qué Tramtai» tiene cuatro cifras del producto, en Overpass tabular', () => {
  const cifras = porQue.match(/<li class="cifra">[\s\S]*?<\/li>/g) ?? []
  assert.equal(cifras.length, 4)
  assert.deepEqual(cifras.map((c) => c.match(/<span class="cifra-numero">(\d+)<\/span>/)?.[1]), ['4', '3', '0', '1'])
  for (const c of cifras) assert.match(c, /lang="es"[\s\S]*lang="en"/)
  for (const t of ['agentes desde tus apps', 'OpenAI, Anthropic y la propia', 'telemetría y ningún servidor nuestro', 'cambio en tu app: la URL'])
    assert.ok(porQue.includes(t), t)
  const regla = css.match(/\.cifra-numero \{[^}]*\}/)?.[0] ?? ''
  assert.match(regla, /var\(--titulo\)/)
  assert.match(regla, /font-variant-numeric: tabular-nums/)
})

const VENTAJAS = [
  ['Usa lo que ya pagas', 'tu clave de API o, solo para tu uso personal, tu suscripción de Claude, ChatGPT o GitHub Copilot. Sin cuentas nuevas.'],
  ['Sin tocar tu código', 'si tu app usa el SDK de OpenAI o el de Anthropic, cambias la URL. Y a Claude Code, Codex, Continue y opencode los apunta Tramtai por ti.'],
  ['Si un agente falla, sigue otro', 'llega a su límite y la petición continúa con el siguiente de tu lista.'],
  ['Todo queda en tu equipo', 'sin servidor nuestro ni telemetría; tus claves, cifradas en el almacén del sistema.'],
  ['Cada app, con su clave y su límite', 'presupuesto diario o mensual por app y cada petición a la vista.'],
  ['Escritorio, terminal o servidor', 'la misma Tramtai en la app, por SSH o siempre encendida en tu servidor.'],
]
test('la línea de ventajas: seis paradas con el texto de la spec, en los dos idiomas y en el acento', () => {
  const paradas = porQue.match(/<li class="parada-ventaja">[\s\S]*?<\/li>/g) ?? []
  assert.equal(paradas.length, 6)
  paradas.forEach((p, i) => {
    assert.match(p, /lang="es"[\s\S]*lang="en"/)
    assert.ok(p.includes(`<span lang="es">${VENTAJAS[i][0]}</span>`), VENTAJAS[i][0])
    // Bajo su título, la frase empieza en mayúscula.
    const frase = VENTAJAS[i][1][0].toUpperCase() + VENTAJAS[i][1].slice(1)
    assert.ok(p.includes(`<p><span lang="es">${frase}</span>`), frase)
    assert.ok(frase.split(/\s+/).length <= 30, `unas 25 palabras como mucho: ${frase}`)
  })
  // El color de línea es de los agentes: la línea de ventajas va en el acento, en dos columnas desde 900 px.
  const reglas = css.slice(css.indexOf('.ventajas'), css.indexOf('/* Pares'))
  assert.doesNotMatch(reglas, COLORES_LINEA)
  assert.match(reglas, /var\(--acento\)/)
  assert.match(reglas, /@media \(min-width: 900px\) \{[^@]*\.ventajas \{[^}]*grid-template-columns: repeat\(2, minmax\(0, 1fr\)\)/)
})

test('los seis motivos ya no están, ni su CSS', () => {
  assert.doesNotMatch(html, /class="motivos?"/)
  assert.doesNotMatch(css, /\.motivo/)
})

// v0.59 (spec §1, §3): «Cómo funciona» enseña el código con pestañas y el paso 3 cuenta lo de las herramientas.
const como = seccion('como')
test('«Cómo funciona» tiene pestañas OpenAI, Anthropic y curl, con su panel cada una', () => {
  const listas = como.match(/<div class="pestanas-lista" role="tablist"[\s\S]*?<\/div>/g) ?? []
  assert.equal(listas.length, 1)
  const pestanas = listas[0].match(/<button type="button" role="tab"[^>]*>[^<]*<\/button>/g) ?? []
  assert.deepEqual(pestanas.map((p) => p.replace(/<[^>]+>/g, '')), ['OpenAI', 'Anthropic', 'curl'])
  for (const p of pestanas) {
    const panel = p.match(/aria-controls="([^"]+)"/)?.[1] ?? ''
    assert.match(como, new RegExp(`<pre class="codigo" id="${panel}" aria-labelledby="${p.match(/id="([^"]+)"/)?.[1]}"`), panel)
  }
  // Sin JavaScript no hay pestañas que no hagan nada: la lista espera oculta y se ve el primer ejemplo.
  assert.match(listas[0], /role="tablist"[^>]* hidden>/)
  assert.equal((como.match(/<pre class="codigo"[^>]* hidden>/g) ?? []).length, 2)
  assert.match(como, /data-pestanas/)
  assert.match(js, /lista\.hidden = false/)
  assert.match(js, /setAttribute\('role', 'tabpanel'\)/)
})

test('el paso 3 de «Cómo funciona» dice que Tramtai apunta tus herramientas y lleva a su Ayuda', () => {
  const paso = como.match(/<li>[\s\S]*?<\/li>/g)?.[2] ?? ''
  for (const t of ['Claude Code', 'Codex', 'Continue', 'opencode', 'copia de su configuración', 'interruptor', 'Herramientas', 'Tools', 'switch'])
    assert.ok(paso.includes(t), t)
  assert.match(paso, /href="ayuda\/conectar-herramientas\.html"/)
})

// v0.59: las capturas de «La app» van en los dos idiomas, cada una con su pareja (lang y srcset), como la terminal.
test('cada captura de «La app» tiene su pareja en el otro idioma, con lang y srcset', () => {
  const imgs = asi.match(/<img [^>]*>/g) ?? []
  assert.equal(imgs.length, 20, 'cinco pantallas, en claro y oscuro y en los dos idiomas')
  for (const img of imgs) {
    const m = img.match(/class="solo-(claro|oscuro)" lang="(es|en)" src="capturas\/([a-z-]+)-\1-\2-1440\.webp"/)
    assert.ok(m, img.slice(0, 120))
    const [, tema, idioma, nombre] = m
    const base = `capturas/${nombre}-${tema}-${idioma}`
    assert.ok(img.includes(`srcset="${base}-640.webp 640w, ${base}-960.webp 960w, ${base}-1440.webp 1440w"`), `${base}: srcset`)
    assert.doesNotMatch(img, /data-alt-en/, 'el alt va en el idioma de la imagen')
    const otro = idioma === 'es' ? 'en' : 'es'
    assert.ok(imgs.some((i) => i.includes(`class="solo-${tema}" lang="${otro}" src="capturas/${nombre}-${tema}-${otro}-1440.webp"`)), `${base}: falta su pareja`)
  }
})

// Las secciones que se juntaron (spec §1): «Conecta estos agentes» dentro de Agentes, los riesgos con la Privacidad y el
// cierre con Descargar. Un enlace guardado a un ancla de antes llega a su sección nueva.
test('Agentes, Privacidad y Descargar absorben las franjas de antes, y las anclas viejas llevan a su sección', () => {
  assert.match(seccion('agentes'), /Conecta estos agentes[\s\S]*Desde tus apps y SDK/)
  assert.match(seccion('privacidad'), /class="tablero"[\s\S]*<!-- comun:riesgos -->/)
  assert.match(seccion('descargar'), /Gratis para uso personal y no comercial/)
  assert.doesNotMatch(html, /href="#(pagas|capturas|aviso)"/)
  assert.match(js, /pagas: 'por-que', capturas: 'app', aviso: 'privacidad'/)
})
const datos = html.match(/<script type="application\/json" id="ejemplos-mapa">([\s\S]*?)<\/script>/)

test('la portada tiene cuatro ejemplos del recorrido, en los dos idiomas', () => {
  assert.ok(datos, 'falta el JSON de los ejemplos del mapa')
  const ejemplos = JSON.parse(datos[1]) as { origen: number; agentes: number[]; es: string; en: string; codigo: string; titulo_es: string; titulo_en: string }[]
  assert.equal(ejemplos.length, 4)
  for (const e of ejemplos) {
    assert.ok(e.es.trim() && e.en.trim(), `falta el texto de ${e.codigo}`)
    assert.ok(e.titulo_es && e.titulo_en, `falta el título de ${e.codigo}`)
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

test('la cabecera tiene cuatro pestañas con nombre y un panel con el código de la escena', () => {
  const pestanas = html.match(/<button type="button" role="tab" class="mapa-pestana"[\s\S]*?<\/button>/g) ?? []
  assert.equal(pestanas.length, 4)
  const titulos = [['Resume un contrato con Claude', 'Summarize a contract with Claude'], ['Un PDF a JSON con Codex', 'A PDF to JSON with Codex'],
    ['Traduce con GitHub Copilot', 'Translate with GitHub Copilot'], ['Respaldo: de Claude a Codex', 'Fallback: from Claude to Codex']]
  pestanas.forEach((p, i) => { assert.match(p, new RegExp(titulos[i][0])); assert.match(p, new RegExp(titulos[i][1])) })
  assert.match(pestanas[0], /aria-selected="true"/)
  const inicio = html.indexOf('<div class="mapa-panel" id="mapa-panel">')
  assert.ok(inicio >= 0, 'falta el panel de la escena')
  const panel = html.slice(inicio)
  assert.match(panel.slice(0, 4000), /id="codigo-mapa-1"/, 'sin JavaScript se ve el código de la primera escena')
  assert.match(html, /<pre class="codigo[^"]*" id="como-anthropic-panel"/, '«Tres paradas» tiene sus ejemplos')
  assert.match(html, /class="mapa-pausa"/)
})
test('las pestañas se manejan con las flechas y paran la rotación', () => {
  assert.match(tranvia, /ArrowRight/)
  assert.match(tranvia, /aria-selected/)
  assert.match(tranvia, /function elegir\([\s\S]*?pausaManual = true/)
  // Revisión del bloque 1 (M5): el ratón o el foco en el bloque paran el giro.
  assert.match(tranvia, /bloque\.addEventListener\('mouseenter'/)
  assert.match(tranvia, /bloque\.addEventListener\('focusin'/)
  assert.match(tranvia, /if \(quieto \|\| pausado \|\| raton \|\| foco\) return/)
})
// Revisión del bloque 1 (M1, M2): «Seguir» reanuda aunque el ratón o el foco sigan en el bloque, y sin JavaScript no
// queda un panel de pestaña sin pestañas.
test('«Seguir» reanuda el giro y el panel solo es tabpanel con las pestañas', () => {
  assert.match(tranvia, /pausa\.addEventListener\('click'[\s\S]*?raton = foco = false/)
  assert.doesNotMatch(html, /role="tabpanel"/)
  assert.match(tranvia, /panel\.setAttribute\('role', 'tabpanel'\)/)
})
// Revisión del bloque 1 (I1): en el móvil el código que no se ve no ocupa sitio; desde 980 px los cuatro comparten la
// celda (el panel no salta) y la caja oscura se ajusta a su código.
test('el panel de la cabecera no deja caja de código vacía', () => {
  assert.match(css, /\.mapa-codigos > \.codigo \{[^}]*align-self: start/)
  assert.match(css, /\.mapa-codigos > \.codigo\.oculto \{ display: none; \}/)
  assert.match(css, /@media \(min-width: 980px\) \{[^@]*\.mapa-codigos > \.codigo \{ grid-area: 1 \/ 1; \}/)
})

test('el recorrido no se anuncia solo y sus controles hablan en el idioma de la página', () => {
  assert.match(html, /<p class="mapa-leyenda" aria-live="off">/, 'mientras rota solo, la leyenda no se anuncia cada 6,5 s')
  for (const pestana of html.match(/<button type="button" role="tab" class="mapa-pestana"[^>]*>[\s\S]*?<\/button>/g) ?? [])
    assert.doesNotMatch(pestana, /aria-label/, 'el nombre de la pestaña va en spans por idioma, no en los dos a la vez')
  assert.doesNotMatch(html, /class="mapa-pausa" aria-pressed/, 'la pausa cambia su texto: sin aria-pressed')
})

// «Por qué Tramtai» (v0.59) conserva el antes y el después de «Los agentes que ya pagas» (v0.56).
const pagas = porQue
test('«Por qué Tramtai» dibuja el antes y el después, con texto visible', () => {
  const figuras = pagas.match(/<figure class="pagas-diagrama[^"]*">[\s\S]*?<\/figure>/g) ?? []
  assert.equal(figuras.length, 2)
  for (const f of figuras) { assert.match(f, /<svg[^>]*role="img"/); assert.match(f, /<figcaption>[\s\S]*lang="es"[\s\S]*lang="en"/) }
  assert.match(pagas, /Una clave, un SDK y una factura por proveedor/)
  // Revisión final (I2): bajo «al servicio de tus apps», sin ofrecer la suscripción para tus apps (TERMINOS.md §4).
  assert.match(pagas, /Una URL local y los agentes que ya tienes/)
  assert.match(pagas, /One local URL and the agents you already have/)
})
test('la suscripción, solo para tu uso personal, y sin promesas absolutas fuera de la línea (I2, M7)', () => {
  // Revisión final (I2): la suscripción, solo para tu uso personal (TERMINOS.md §4), también en inglés.
  assert.match(pagas, /<span lang="en">Your API key or, only for your personal use, your Claude, ChatGPT or GitHub Copilot subscription\. No new accounts\.<\/span>/)
  assert.doesNotMatch(pagas, /la suscripción o la clave que ya tienes|the subscription or key you already have/)
  // «Siempre encendida» habla del servidor (spec §4); el resto no promete de más.
  const sinLinea = pagas.replace(/<ol class="ventajas">[\s\S]*?<\/ol>/, '')
  assert.doesNotMatch(sinLinea, /\b(nunca|siempre|sin cortes|a nadie|never|always|no one)\b/i)
  // Revisión final (M7): Tramtai sí está en medio del dibujo, pero no hay nube.
  assert.doesNotMatch(pagas, /Sin servidores intermedios|No middle servers/)
})
test('los tres dibujos llaman igual a los tres orígenes (revisión final, M8)', () => {
  assert.doesNotMatch(html, /Tus automatizaciones|Your automations/)
})

const css = readFileSync(new URL('../../site/estilo.css', import.meta.url), 'utf8')
const COLORES_LINEA = /var\(--(claude|codex|copilot|gemini|antigravity)\)/

test('el color de cada agente solo marca a su agente: mapa, diagrama «Con Tramtai», fichas y tarjetas de Agentes (52)', () => {
  assert.doesNotMatch(html, /class="linea"/)
  const sinAgentes = html.replace(/<div class="mapa"[\s\S]*?<\/svg>\s*<\/div>/, '').replace(/<section id="agentes">[\s\S]*?<\/section>/, '').replace(/<figure class="pagas-diagrama con">[\s\S]*?<\/figure>/, '')
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
  const portada = html.match(/<section id="portada" class="portada">[\s\S]*?<div class="mapa"/)?.[0] ?? ''
  assert.ok(portada, 'falta <section id="portada">')
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
  for (const clase of ['.github', '.menu-boton', '.demo-flecha'])
    assert.match(css, new RegExp(`\\${clase} \\{[^}]*width: 44px; height: 44px`), clase)
  // .idioma button es ES/EN y tema: 44 de alto el botón, no el grupo con su borde (revisión final web, M13).
  for (const clase of ['.boton', '.copiar', '.mapa-pausa', '.mapa-pestana', '.demo-pausa', '.pestanas-lista button', '.idioma button'])
    assert.match(css, new RegExp(`${clase.replace('.', '\\.')} \\{[^}]*min-height: 44px`), clase)
  assert.match(css, /section\[id\] \{ scroll-margin-top: 72px; \}/)
  assert.match(css, /#demo \{[^}]*scroll-margin-top: 72px/, 'tampoco tapa el recorrido guiado, que va dentro de «La app»')
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

test('el código del panel de la cabecera salta de línea y «Tres paradas» enseña otro ejemplo', () => {
  assert.match(css, /\.mapa-codigos > \.codigo \{[^}]*white-space: pre-wrap; overflow-wrap: anywhere/, 'sin desplazamiento horizontal en el panel')
  const pre = (id: string) => html.match(new RegExp(`<pre class="codigo" id="${id}"[^>]*>([\\s\\S]*?)</pre>`))?.[1] ?? ''
  assert.match(pre('como-anthropic-panel'), /anthropic/)
  assert.match(pre('como-openai-panel').replace(/<[^>]+>/g, ''), /from openai import OpenAI/)
  assert.match(pre('como-curl-panel'), /^curl /m)
  for (const id of ['como-openai-panel', 'como-anthropic-panel', 'como-curl-panel']) assert.notEqual(pre(id), pre('codigo-mapa-1'), id)
})

test('los diagramas comparan sin Tramtai y con Tramtai', () => {
  const diagramas = pagas.match(/<div class="pagas-diagramas">[\s\S]*<\/figure>/)?.[0] ?? ''
  for (const t of ['Sin Tramtai', 'Con Tramtai', 'Without Tramtai', 'With Tramtai']) assert.ok(diagramas.includes(t), `falta «${t}»`)
})
// Revisión del bloque 1 (M3, M4): los agentes con los nombres del glosario, como en el mapa, y las líneas de las apps a
// 3:1 como mínimo en claro (el acento al 55 % quedaba en 2,5:1).
test('«Con Tramtai» nombra a los agentes como el glosario y sus líneas se ven en claro', () => {
  const con = pagas.match(/<figure class="pagas-diagrama con">[\s\S]*?<\/figure>/)?.[0] ?? ''
  for (const n of ['Claude Code', 'GitHub Copilot', 'Gemini API']) assert.match(con, new RegExp(`>${n}</text>`), n)
  assert.doesNotMatch(html, /var\(--acento\) 55%/)
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
  // Las capturas salen de su propio puerto (PUERTO_CAPTURAS de capturas.mjs), para no tocar el Tramtai de quien las saca.
  const puerto = readFileSync(new URL('./capturas.mjs', import.meta.url), 'utf8').match(/const PUERTO_CAPTURAS = (\d+)/)?.[1]
  assert.ok(puerto && puerto !== '17373' && nota.includes(puerto), `la nota dice el puerto de las capturas (${puerto})`)
})

test('las capturas son WebP con tamaño declarado, srcset y carga diferida', () => {
  const imgs = (asi.match(/<img [^>]*src="capturas\/[^>]*>/g) ?? [])
  assert.equal(imgs.length, 20, 'cinco pantallas, en claro y oscuro y en los dos idiomas')
  for (const img of imgs) {
    assert.match(img, /src="capturas\/[a-z-]+-(claro|oscuro)-(es|en)-1440\.webp"/)
    assert.match(img, /width="1440" height="900"/)
    assert.match(img, /srcset="capturas\/[a-z-]+-(claro|oscuro)-(es|en)-640\.webp 640w, capturas\/[a-z-]+-(claro|oscuro)-(es|en)-960\.webp 960w, capturas\/[a-z-]+-(claro|oscuro)-(es|en)-1440\.webp 1440w"/)
    // Las dos columnas de un par (1fr y 1.6fr en 1120 px) dejan la captura en 630 px: el navegador elige la de 640.
    assert.match(img, /sizes="\(max-width: 900px\) 100vw, 630px"/)
    assert.match(img, /loading="lazy"/, 'ninguna está en la primera pantalla')
  }
})

// v0.59: cada captura de la app y del servidor existe en español e inglés (las del HTML, en la prueba de las parejas). Las
// del servidor (entrar y Inicio con la cabecera «Servidor») van a 1440 y 960 px.
test('cada captura de la app y del servidor tiene su versión es y en en disco', () => {
  const archivos = readdirSync(new URL('../../site/capturas/', import.meta.url))
  const pedidas = [
    ...PORTADA.map((p) => ({ nombre: p.nombre, anchos: [1440, 960, 640] })),
    ...['servidor-entrar', 'servidor-inicio'].map((nombre) => ({ nombre, anchos: [1440, 960] })),
  ]
  for (const { nombre, anchos } of pedidas)
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        for (const ancho of anchos) assert.ok(archivos.includes(`${nombre}-${tema}-${idioma}-${ancho}.webp`), `falta ${nombre}-${tema}-${idioma}-${ancho}.webp`)
})

test('la portada cuenta el mapa en vivo y no habla de la «API principal»', () => {
  const inicio = html.match(/<article class="par">\s*<div>\s*<span class="etiqueta"><span lang="es">Inicio[\s\S]*?<\/article>/)?.[0] ?? ''
  assert.match(inicio, /en vivo/)
  assert.match(inicio, /live/)
  for (const pagina of ['index.html', 'desarrolladores.html'])
    assert.doesNotMatch(readFileSync(new URL(`../../site/${pagina}`, import.meta.url), 'utf8'), /API principal|main API/i, pagina)
})

// La demo del recorrido guiado: una figura por paso de la bienvenida, con la zona que señala el paso y su texto debajo.
const carpeta = new URL('../../site/capturas/', import.meta.url)
const figuras = demo.match(/<figure class="captura demo-paso"[\s\S]*?<\/figure>/g) ?? []

test('la demo es una figura por paso de la bienvenida, cada una con su captura en claro y oscuro y en los dos idiomas', () => {
  assert.ok(demo, 'falta <div id="demo"> en «La app»')
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
  // La primera se ve; el resto espera su turno (sin JavaScript se ven las 14, una debajo de otra, y la lista se oculta).
  assert.doesNotMatch(figuras[0], /^<figure[^>]* hidden/)
  for (const figura of figuras.slice(1)) assert.match(figura, /^<figure[^>]* hidden/)
})

// Revisión del bloque 2 (M5): el último paso sale a pantalla entera y sin el globo; su alt no lo describe.
test('el texto alternativo del último paso describe la página entera, no el globo', () => {
  const ultima = figuras.at(-1) ?? ''
  assert.doesNotMatch(ultima, /cierre del recorrido|end of the tour/)
  assert.match(ultima, /alt="Recorrido guiado, paso 14 de 14: la página Espacios entera"/)
  assert.match(ultima, /alt="Guided tour, step 14 of 14: the whole Workspaces page"/)
})

// Revisión del bloque 2 (I3, M3, M4) y final (I1, M3): las flechas van encima de los bordes de la imagen, que usa todo el
// ancho del escenario (820 px en el escritorio, con la lista a 220); con JavaScript los pasos comparten celda y la pausa
// no sube y baja con cada uno; cada paso enlaza su imagen a tamaño real.
test('la imagen del paso ocupa todo el escenario y los controles no se mueven', () => {
  assert.match(css, /\n\.demo-flecha \{[^}]*grid-area: 1 \/ 1/)
  assert.match(css, /html\.js \.demo \{[^}]*grid-template-columns: 220px minmax\(0, 1fr\)/)
  assert.match(css, /html\.js \.demo-escenario \{[^}]*grid-template-columns: minmax\(0, 1fr\)/)
  for (const f of figuras) for (const s of f.match(/sizes="[^"]+"/g) ?? []) assert.equal(s, 'sizes="(max-width: 899px) calc(100vw - 32px), 820px"')
  figuras.forEach((f, i) => {
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en'])
        assert.match(f, new RegExp(`<a class="solo-${tema}" lang="${idioma}" href="capturas/bienvenida-${i + 1}-${tema}-${idioma}-1440\\.webp">`), `paso ${i + 1}`)
  })
  assert.match(css, /html\.js \.demo-paso \{ grid-area: 1 \/ 1; \}/)
  assert.match(css, /html\.js \.demo-paso\[hidden\] \{[^}]*visibility: hidden/)
})

// v0.56: la lista de pasos al lado, agrupada por página, y flechas a los costados en lugar de los puntos.
test('la lista de pasos va agrupada por página, con los títulos reales del recorrido', () => {
  const botones = [...demo.matchAll(/<button type="button" class="demo-ir" data-paso="(\d+)"><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/button>/g)]
  assert.deepEqual(botones.map((m) => [m[2], m[3]]), pasos.map((p) => [es[p.titulo], en[p.titulo]]))
  assert.deepEqual(botones.map((m) => Number(m[1])), pasos.map((_, i) => i))
  assert.ok((demo.match(/class="demo-grupo"/g) ?? []).length >= 5)
  // Cada grupo se llama como su página en el menú de la app, en los dos idiomas.
  const grupos = [...demo.matchAll(/<li class="demo-grupo"><span class="demo-pagina"><span lang="es">([^<]+)<\/span><span lang="en">([^<]+)<\/span><\/span>/g)].map((m) => [m[1], m[2]])
  const paginas = [...new Set(pasos.map((p) => p.pagina))]
  assert.deepEqual(grupos, paginas.map((p) => [es[`menu.${p}` as keyof typeof es], en[`menu.${p}` as keyof typeof en]]))
  assert.match(demo, /<details class="demo-pasos-movil" open>/)
})
test('cada paso enseña su texto debajo de la imagen, en los dos idiomas', () => {
  const escapar = (s: string) => s.slice(0, 30).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  figuras.forEach((f, i) => {
    assert.match(f, new RegExp(`<figcaption>[\\s\\S]*${escapar(es[pasos[i].texto])}`))
    assert.match(f, new RegExp(`<figcaption>[\\s\\S]*${escapar(en[pasos[i].texto])}`))
  })
})
test('flechas a los costados, con su nombre en cada idioma, y sin puntos', () => {
  for (const [clase, es_, en_] of [['demo-anterior', 'Paso anterior', 'Previous step'], ['demo-siguiente', 'Paso siguiente', 'Next step']])
    assert.match(demo, new RegExp(`<button type="button" class="demo-flecha ${clase}"[\\s\\S]*?solo-lectores">${es_}<[\\s\\S]*?solo-lectores">${en_}<`))
  assert.doesNotMatch(demo, /class="demo-punto"/)
  assert.match(demo, /class="demo-progreso"/)
})

const demoJs = js.slice(js.indexOf('// Demo del recorrido guiado'))

test('las flechas del teclado solo actúan con el foco en la demo', () => {
  assert.match(demoJs, /caja\.addEventListener\('keydown'/)
  assert.doesNotMatch(demoJs, /document\.addEventListener\('keydown'/)
  // Revisión del bloque 2 (M1): Alt+← es el atrás del navegador.
  assert.match(demoJs, /if \(e\.altKey \|\| e\.ctrlKey \|\| e\.metaKey \|\| e\.shiftKey\) return/)
})

// Revisión del bloque 2 (I1, I2, M2): el foco solo se mueve cuando elige la persona, la rotación se para con el foco
// dentro (patrón de carrusel), en el móvil el foco vuelve al desplegable al cerrarlo y en el escritorio la lista no
// se pliega.
test('la demo no mueve el foco sola ni lo pierde, y la lista del escritorio no se pliega', () => {
  const mostrar = demoJs.match(/function mostrar\(n\) \{[\s\S]*?\n  \}/)?.[0] ?? ''
  assert.ok(mostrar, 'falta mostrar()')
  assert.doesNotMatch(mostrar, /\.focus\(\)/, 'el avance solo no mueve el foco')
  assert.match(demoJs, /function elegir\(n\) \{[\s\S]*?\.focus\(\)/)
  assert.match(demoJs, /caja\.addEventListener\('focusin'/)
  assert.match(demoJs, /if \(pausado \|\| foco \|\| /)
  assert.match(demoJs, /detalles\.open = false;[^\n]*\n[^\n]*detalles\.querySelector\('summary'\)\.focus\(\)/)
  assert.match(demoJs, /if \(!movil\.matches\) e\.preventDefault\(\)/)
  assert.match(css, /@media \(min-width: 900px\) \{[^@]*\.demo-pasos-movil > summary \{[^}]*list-style: none/)
})

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
  assert.doesNotMatch(demo, /aria-label=/, 'los nombres van en spans por idioma, no en los dos a la vez')
})

// v0.56: cada captura de la demo es la zona que señala el paso, ampliada y sin el globo del recorrido.
const capturasJs = readFileSync(new URL('./capturas.mjs', import.meta.url), 'utf8')
test('las capturas de la demo recortan la zona del paso y ocultan el globo del recorrido', () => {
  assert.ok(/function zonaDelPaso\(/.test(capturasJs), 'falta zonaDelPaso()')
  assert.ok(capturasJs.includes("paso.ancla ? '.driver-popover'") && capturasJs.includes("visibility = 'hidden'"), 'el globo del recorrido no sale en la captura')
  assert.ok(capturasJs.includes('clip: { ...zona, scale: 1440 / zona.width }'), 'cada captura recorta la zona y la amplía al ancho')
  // Revisión del bloque 2 (I3, M7): la zona mide 1080 px como mucho, así siempre se amplía, y el hueco del velo se mira
  // en las cuatro esquinas de lo señalado.
  // Revisión final (I1, M2): el tope es el ancho de la imagen en la portada (820), así los pasos salen a 1× o más; las
  // listas de filas, cuyo texto habla de la fila entera, abarcan su ancho.
  assert.match(capturasJs, /Math\.min\(maximo, Math\.max\(720, /)
  assert.match(capturasJs, /async function zonaDelPaso\(maximo = 820\)/)
  // El mapa del paso 1 también va entero: sus agentes son la mitad de lo que explica el paso.
  assert.match(capturasJs, /ENTERAS = new Set\(\['agentes\.tarjeta', 'espacios\.lista', 'inicio\.mapa'\]\)/)
  assert.match(capturasJs, /\[b\.left \+ 2, b\.top \+ 2\]/)
  // Un ancla más ancha que la zona se recorta desde su borde izquierdo (con su margen): centrada perdía los nombres.
  assert.ok(capturasJs.includes('b.width + 64 > width ? b.left - 32 : b.left + b.width / 2 - width / 2'), 'el ancla ancha arranca a la izquierda')
})
// El último paso no tiene ancla, pero driver.js deja en el DOM el resaltado del anterior: se decide por el paso.
test('el paso sin ancla sale con la pantalla entera desde arriba, sin velo', () => {
  assert.ok(capturasJs.includes('paso.ancla ? await zonaDelPaso(ENTERAS.has(paso.ancla) ? 1440 : 820) : { x: 0, y: 0, width: 1440, height: 900 }'), 'el último paso recorta una zona')
  assert.ok(capturasJs.includes("'.driver-popover, .driver-overlay'"), 'el último paso sale con el velo')
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
  assert.match(funciones, /Controla<\/span><span lang="en">Control<\/span><\/h3>\s*<ol>\s*<li class="parada">\s*<h4 class="parada-titulo"><span lang="es">Espacios de trabajo<\/span>/, 'Espacios abre la línea Controla')
})

// v0.56: la portada presenta la terminal; en la v0.58, con sus capturas (Inicio y Mis APIs, claro y oscuro, es y en).
test('la novedad «Todo desde la terminal» dice cómo usarla y lleva a su Ayuda, en los dos idiomas', () => {
  const terminal = seccion('terminal')
  assert.ok(terminal, 'falta <section id="terminal">')
  for (const t of ['Todo desde la terminal.', 'Everything from the terminal.', 'Nuevo en la v0.56', 'New in v0.56', 'SSH', '60×20'])
    assert.ok(terminal.includes(t), `falta «${t}»`)
  const codigo = terminal.match(/<pre class="codigo">[\s\S]*?<\/pre>/)?.[0] ?? ''
  assert.match(codigo, /^tramtai servicio iniciar -d$/m)
  assert.match(codigo, /^tramtai terminal<\/pre>$/m)
  for (const c of codigo.match(/<span class="c"[^>]*>/g) ?? []) assert.match(c, /lang="(es|en)"/)
  assert.match(terminal, /href="ayuda\/empezar\.html#s-soloterminal"/)
  const imgs = terminal.match(/<img [^>]*>/g) ?? []
  assert.equal(imgs.length, 8, 'Inicio y Mis APIs, en claro y oscuro y en los dos idiomas')
  for (const p of ['inicio', 'apis'])
    for (const tema of ['claro', 'oscuro'])
      for (const idioma of ['es', 'en']) {
        const base = `capturas/terminal-${p}-${tema}-${idioma}`
        assert.ok(imgs.some((img) => new RegExp(`class="solo-${tema}" lang="${idioma}" src="${base}-1440\\.webp" srcset="${base}-960\\.webp 960w, ${base}-1440\\.webp 1440w" sizes="[^"]+" width="\\d+" height="\\d+" loading="lazy" decoding="async" alt="[^"]+"`).test(img)), `${p}, ${tema}, ${idioma}`)
      }
})

test('la portada presenta el modo servidor, que es otro entorno, y el aviso con los cinco riesgos', () => {
  const servidor = seccion('servidor')
  for (const t of ['Nuevo en la v0.58', 'New in v0.58', 'otro entorno', 'separate environment', 'HTTPS', 'href="servidor.html"'])
    assert.ok(servidor.includes(t), `falta «${t}»`)
  const aviso = seccion('privacidad').match(/<ol class="riesgos">[\s\S]*?<\/ol>/)?.[0] ?? ''
  assert.equal((aviso.match(/<li>/g) ?? []).length, 5, 'los cinco riesgos')
  assert.match(seccion('privacidad'), /<!-- comun:riesgos -->/, 'el aviso sale de riesgos.* (npm run sitio)')
})

test('Funciones tiene la parada de la terminal, que lleva a la novedad', () => {
  const parada = seccion('funciones').match(/<li class="parada">\s*<h4 class="parada-titulo"><span lang="es">La terminal, a la par de la app<\/span>[\s\S]*?<\/li>/)?.[0] ?? ''
  assert.ok(parada, 'falta la parada')
  assert.match(parada, /<span lang="en">The terminal, on par with the app<\/span>/)
  assert.match(parada, /href="#terminal"/)
  assert.match(parada, /SSH/)
})

// v0.56: las funciones son un plano de metro, una línea por grupo con sus tres paradas (sustituye a las cuatro rejillas de
// tarjetas, 54). Sin JavaScript, o en el móvil, cada línea es vertical con el texto de cada parada a la vista.
test('el plano tiene cuatro líneas y doce paradas con su texto en el HTML (sin JavaScript se lee todo)', () => {
  const funciones = seccion('funciones')
  assert.equal((funciones.match(/<section class="metro-linea" data-linea="(conecta|controla|protege|usa)"/g) ?? []).length, 4)
  const paradas = funciones.match(/<li class="parada">[\s\S]*?<\/li>/g) ?? []
  assert.equal(paradas.length, 12)
  for (const p of paradas) { assert.match(p, /<h4 class="parada-titulo">/); assert.match(p, /<p class="parada-texto">[\s\S]*lang="es"[\s\S]*lang="en"/) }
  for (const t of ['Compatible con tus SDK', 'Una API por aplicación', 'Respaldo entre agentes', 'La terminal, a la par de la app', 'Recorrido y Ayuda'])
    assert.match(funciones, new RegExp(t))
  assert.match(funciones, /href="#demo"/)
  for (const linea of funciones.match(/<section class="metro-linea"[\s\S]*?<\/section>/g) ?? [])
    assert.equal((linea.match(/<li class="parada">/g) ?? []).length, 3)
  assert.match(funciones, /<div class="metro-panel" aria-live="polite" hidden><\/div>/, 'el panel solo lo usa el JavaScript')
  assert.doesNotMatch(funciones, /\b(nunca|siempre|sin cortes|a nadie|never|always|no one)\b/i)
})

const metroJs = js.slice(js.indexOf('// Plano de metro de Funciones'))
test('con JavaScript y pantalla ancha el plano es horizontal y cada parada enseña su texto en el panel', () => {
  assert.ok(js.includes('// Plano de metro de Funciones'), 'falta el bloque del plano en sitio.js')
  assert.match(metroJs, /classList\.toggle\('metro--plano'/)
  assert.match(metroJs, /matchMedia\('\(min-width: 900px\)'\)/)
  // Revisión final (M4): una sola parada en el tabulador (tabindex móvil, con flechas, como las pestañas de la
  // cabecera) y la elegida marcada con aria-current, que no se «despulsa».
  assert.match(metroJs, /setAttribute\('aria-current', 'true'\)/)
  assert.doesNotMatch(metroJs, /aria-pressed/)
  assert.match(metroJs, /ArrowRight/)
  assert.match(metroJs, /tabIndex = i === n \? 0 : -1/)
  // Ronda del bloque 2 y final (M6): sin JavaScript, y en vertical, el título es texto; el JS lo convierte en botón solo
  // en el plano y lo devuelve al pasar a vertical.
  assert.doesNotMatch(seccion('funciones'), /parada-boton/)
  assert.match(metroJs, /createElement\('button'\)[\s\S]*?className = 'parada-boton'/)
  assert.match(metroJs, /replaceWith/)
  assert.doesNotMatch(metroJs, /tabIndex = plano/)
  // Revisión final (M5): el título del panel no es un h4 más en el esquema.
  assert.doesNotMatch(metroJs, /'<h4>'/)
  assert.match(metroJs, /metro-panel-titulo/)
  assert.match(css, /\.metro--plano \.parada-texto \{ display: none; \}/)
})
// Revisión final (M1): el titular se queda, y la entradilla aclara que estas cuatro líneas son lo que hace Tramtai.
test('la entradilla de Funciones explica las cuatro líneas del plano', () => {
  const cabeza = seccion('funciones').match(/<div class="cabeza">[\s\S]*?<\/div>/)?.[0] ?? ''
  assert.match(cabeza, /Una red con una línea por agente\./)
  assert.match(cabeza, /cada línea es lo que hace Tramtai/)
  assert.match(cabeza, /each line is what Tramtai does/)
})

test('Homebrew sin la app: el tap lleva la URL del repositorio y después «brew install tramtai» (el tap corto no existe)', () => {
  const tap = 'brew tap mscottb/tramtai https://github.com/MscottB/Tramtai'
  const linux = html.match(/<article class="tarjeta" data-sistema="linux">[\s\S]*?<\/article>/)?.[0] ?? ''
  assert.ok(linux.includes(tap + '\nbrew install tramtai'), 'tarjeta de Linux')
  for (const textos of [es, en]) {
    const r = textos['ayuda.empezar.rTerminal']
    assert.ok(r.includes(tap) && r.includes('brew install tramtai'), r)
  }
  assert.ok(![html, es['ayuda.empezar.rTerminal'], en['ayuda.empezar.rTerminal']].some((x) => x.includes('mscottb/tramtai/tramtai')))
})
