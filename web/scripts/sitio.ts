// sitio arma la Ayuda del sitio (site/ayuda/, la página de paso site/ayuda.html, 404.html y sitemap.xml) desde los
// mismos temas que la app, y es la única fuente de la cabeza, la barra y el pie de todas las páginas: rellenar los pone
// al día en las páginas escritas a mano (PAGINAS_FIJAS). Son funciones puras: sitio-ayuda.mjs les da los textos, los
// ejemplos y las capturas y escribe los archivos.
import type { Bloque, Tema } from '../src/ayuda/tipos'
import { bloquesVisibles, capturaDeAncla, partesOpciones } from '../src/ayuda/vista'
import { campos, normalizar } from '../src/ayuda/buscar'
import { ERRORES } from '../src/ayuda/temas/problemas'

type Textos = Record<string, string>
export interface Fuente {
  temas: Tema[]
  es: Textos
  en: Textos
  // ejemplos da el código de un bloque ejemplo en un idioma (el mensaje de ejemplo cambia con el idioma).
  ejemplos: (b: Extract<Bloque, { tipo: 'ejemplo' }>, idioma: 'es' | 'en') => { id: string; titulo: string; codigo: string }[]
  // capturas son los archivos de web/src/ayuda/capturas que existen, con su tamaño en píxeles.
  capturas: Map<string, { ancho: number; alto: number }>
  // paginas es el HTML de servidor.html y desarrolladores.html (de site/): el buscador indexa sus apartados.
  paginas?: Record<string, string>
}

// URL_SITIO es la dirección pública del sitio (GitHub Pages): canonical, Open Graph, sitemap y 404 la necesitan absoluta.
export const URL_SITIO = 'https://mscottb.github.io/Tramtai/'
const REPO = 'https://github.com/MscottB/Tramtai'

// Lugar dice dónde está una página: rel lleva de ella a la raíz del sitio y ruta es su dirección desde la raíz (null en
// la 404, que no tiene una propia).
export type Actual = 'inicio' | 'producto' | 'agentes' | 'servidor' | 'desarrolladores' | 'ayuda' | 'descargar' | ''
interface Lugar { rel: string; ruta: string | null; actual: Actual }

// PAGINAS_FIJAS son las páginas escritas a mano; sitio-ayuda.mjs les pone al día las zonas comunes con rellenar.
export const PAGINAS_FIJAS: (Lugar & { archivo: string })[] = [
  { archivo: 'index.html', rel: '', ruta: '', actual: 'inicio' },
  { archivo: 'producto.html', rel: '', ruta: 'producto.html', actual: 'producto' },
  { archivo: 'agentes.html', rel: '', ruta: 'agentes.html', actual: 'agentes' },
  { archivo: 'servidor.html', rel: '', ruta: 'servidor.html', actual: 'servidor' },
  { archivo: 'desarrolladores.html', rel: '', ruta: 'desarrolladores.html', actual: 'desarrolladores' },
  { archivo: 'descargar.html', rel: '', ruta: 'descargar.html', actual: 'descargar' },
]

// riesgos son los cinco riesgos de los términos (spec §10), los de riesgos.* de los catálogos: el mismo texto que
// terminos.Riesgos en Go. El sitio, el README y la guía del servidor los toman de aquí al generar, nunca copiados a mano.
// Sin el modo servidor (lo que lee cualquiera), el 3 y el 4 llevan su variante «Si usas el modo servidor…».
export function riesgos(textos: Textos, servidor: boolean): string[] {
  return [1, 2, 3, 4, 5].map((n) => textos[!servidor && (n === 3 || n === 4) ? `riesgos.${n}.sinServidor` : `riesgos.${n}`])
}

// listaRiesgos es el aviso de una página del sitio, en los dos idiomas.
function listaRiesgos(es: Textos, en: Textos, servidor: boolean): string {
  const [rEs, rEn] = [riesgos(es, servidor), riesgos(en, servidor)]
  return `<ol class="riesgos">${rEs.map((r, i) => `\n  <li>${bilingue(r, rEn[i])}</li>`).join('')}\n</ol>`
}

// rellenarMarkdown pone al día las zonas del aviso de un Markdown (<!-- riesgos:es --> … <!-- /riesgos:es -->, y
// riesgos-servidor con los del servidor): el README y docs/servidor.md, que npm run sitio también escribe.
export function rellenarMarkdown(md: string, es: Textos, en: Textos): string {
  return md.replace(/<!-- (riesgos(?:-servidor)?):(es|en) -->[\s\S]*?<!-- \/\1:\2 -->/g, (_, zona: string, idioma: string) => {
    const lista = riesgos(idioma === 'es' ? es : en, zona === 'riesgos-servidor').map((r, i) => `${i + 1}. ${r}`).join('\n')
    return `<!-- ${zona}:${idioma} -->\n${lista}\n<!-- /${zona}:${idioma} -->`
  })
}

// PRIMERO aplica el tema y el idioma guardados antes de pintar (sitio.js hace el resto con defer). La clase js deja al
// CSS esconder el menú tras su botón solo si hay JavaScript que lo abra.
const PRIMERO = `<script>document.documentElement.classList.add('js'); try { var t = localStorage.getItem('tramtai.tema'); if (t === 'claro' || t === 'oscuro') document.documentElement.dataset.tema = t; var i = localStorage.getItem('tramtai.idioma') } catch (e) {}
document.documentElement.lang = i === 'es' || i === 'en' ? i : (navigator.language || 'es').slice(0, 2) === 'es' ? 'es' : 'en'</script>`

// cabeza es lo común del <head>: metadatos para buscadores y para compartir, iconos, estilos y scripts.
export function cabeza(l: Lugar, titulo: string, descripcion: string): string {
  const url = l.ruta === null ? '' : URL_SITIO + l.ruta
  return `${url ? `<link rel="canonical" href="${url}">\n<meta property="og:url" content="${url}">` : '<meta name="robots" content="noindex">'}
<meta property="og:type" content="website">
<meta property="og:site_name" content="Tramtai">
<meta property="og:title" content="${escapar(titulo)}">
<meta property="og:description" content="${escapar(descripcion)}">
<meta property="og:image" content="${URL_SITIO}og.png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:locale" content="es_ES">
<meta property="og:locale:alternate" content="en_US">
<meta name="twitter:card" content="summary_large_image">
<meta name="theme-color" content="#0b6e64" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#0b0d10" media="(prefers-color-scheme: dark)">
<link rel="icon" href="${l.rel}favicon.svg" type="image/svg+xml">
<link rel="icon" href="${l.rel}favicon-32.png" sizes="32x32" type="image/png">
<link rel="apple-touch-icon" href="${l.rel}apple-touch-icon.png">
<link rel="manifest" href="${l.rel}manifest.webmanifest">
<link rel="preload" href="${l.rel}inter.woff2" as="font" type="font/woff2" crossorigin>
<link rel="stylesheet" href="${l.rel}estilo.css">
<script src="${l.rel}sitio.js" defer></script>
${PRIMERO}`
}

const GITHUB = 'M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.9.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z'
const ICONO_TEMA = {
  sistema: '<circle cx="12" cy="12" r="8"/><path d="M12 4a8 8 0 0 1 0 16z" fill="currentColor"/>',
  claro: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
  oscuro: '<path d="M20 14.5A8 8 0 0 1 9.5 4a8 8 0 1 0 10.5 10.5z"/>',
}
const svg = (d: string) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${d}</svg>`
const soloLectores = (es: string, en: string) => `<span lang="es" class="solo-lectores">${es}</span><span lang="en" class="solo-lectores">${en}</span>`

// enlacePagina arma el enlace a una página del sitio con su nombre en los dos idiomas y marca la página actual.
const enlacePagina = (rel: string, actual: Actual) => (a: Actual, ruta: string, es: string, en: string, clase = ''): string =>
  `<a href="${ruta ? rel + ruta : rel || './'}"${clase ? ` class="${clase}"` : ''}${a === actual ? ' aria-current="page"' : ''}>${bilingue(es, en)}</a>`

// cabecera es el enlace para saltar al contenido y la barra: las siete páginas, el buscador (oculto hasta que sitio.js lo
// encienda), idioma, tema, GitHub y Descargar. En pantallas medianas y pequeñas las páginas, el buscador, el idioma y el
// tema van juntos en el panel del menú (#menu), así cada control cabe con 44 px.
export function cabecera(rel: string, actual: Actual): string {
  const enlace = enlacePagina(rel, actual)
  const tema = (t: keyof typeof ICONO_TEMA, es: string, en: string) =>
    `<button type="button" data-tema="${t}" aria-pressed="${t === 'sistema'}">${svg(ICONO_TEMA[t])}${soloLectores(es, en)}</button>`
  return `<a class="saltar" href="#contenido">${bilingue('Saltar al contenido', 'Skip to content')}</a>
<header class="barra">
  <div class="envoltura">
    <a class="marca" href="${rel || './'}"><img src="${rel}favicon.svg" width="30" height="30" alt=""><span class="marca-texto">Tramtai</span></a>
    <button type="button" class="menu-boton" aria-expanded="false" aria-controls="menu" hidden>${svg('<path d="M4 7h16M4 12h16M4 17h16"/>')}${soloLectores('Menú', 'Menu')}</button>
    <div id="menu" class="menu-panel">
      <nav aria-label="Secciones" data-aria-en="Sections">
        ${enlace('producto', 'producto.html', 'Producto', 'Product')}
        ${enlace('agentes', 'agentes.html', 'Agentes', 'Agents')}
        ${enlace('servidor', 'servidor.html', 'Servidor', 'Server')}
        ${enlace('desarrolladores', 'desarrolladores.html', 'Desarrolladores', 'Developers')}
        ${enlace('ayuda', 'ayuda/', 'Ayuda', 'Help')}
        ${enlace('descargar', 'descargar.html', 'Descargar', 'Download', 'solo-menu')}
      </nav>
      <button type="button" class="buscar" data-buscar aria-label="Buscar" data-aria-en="Search" hidden>${svg('<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>')}${soloLectores('Buscar', 'Search')}</button>
      <div class="preferencias">
        <div class="idioma" role="group" aria-label="Idioma" data-aria-en="Language">
          <button type="button" data-idioma="es" aria-pressed="true">ES</button>
          <button type="button" data-idioma="en" aria-pressed="false">EN</button>
        </div>
        <div class="idioma tema" role="group" aria-label="Tema" data-aria-en="Theme">
          ${tema('sistema', 'Tema del sistema', 'System theme')}
          ${tema('claro', 'Tema claro', 'Light theme')}
          ${tema('oscuro', 'Tema oscuro', 'Dark theme')}
        </div>
      </div>
    </div>
    <a class="github" href="${REPO}" aria-label="GitHub" title="GitHub"><svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="${GITHUB}"/></svg></a>
    <a class="boton principal" href="${rel || './'}descargar.html"${actual === 'descargar' ? ' aria-current="page"' : ''}>${svg('<path d="M12 4v11m0 0-4.5-4.5M12 15l4.5-4.5M5 20h14"/>')}<span class="boton-texto">${bilingue('Descargar', 'Download')}</span></a>
  </div>
</header>`
}

// pie lleva a todo lo que no está en la barra: empezar, el código, las versiones, la licencia y los términos.
export function pie(rel: string, actual: Actual = ''): string {
  const enlace = enlacePagina(rel, actual)
  return `<footer>
  <div class="envoltura">
    <nav class="pie-enlaces" aria-label="Pie de página" data-aria-en="Footer">
      ${enlace('inicio', '', 'Inicio', 'Home')}
      ${enlace('producto', 'producto.html', 'Producto', 'Product')}
      ${enlace('agentes', 'agentes.html', 'Agentes', 'Agents')}
      ${enlace('servidor', 'servidor.html', 'Servidor', 'Server')}
      ${enlace('desarrolladores', 'desarrolladores.html', 'Desarrolladores', 'Developers')}
      ${enlace('ayuda', 'ayuda/', 'Ayuda', 'Help')}
      ${enlace('descargar', 'descargar.html', 'Descargar', 'Download')}
      <a href="${rel}ayuda/empezar.html">${bilingue('Primeros pasos', 'Getting started')}</a>
      <a href="${REPO}">GitHub</a>
      <a href="${REPO}/releases">${bilingue('Versiones', 'Releases')}</a>
      <a href="${REPO}/blob/main/LICENSE.md">${bilingue('Licencia', 'License')}</a>
      <a href="${REPO}/blob/main/TERMINOS.md">${bilingue('Términos de uso', 'Terms of use')}</a>
    </nav>
    <p>© 2026 Michael Scott B. · ${bilingue('Claude, Codex, GitHub Copilot, Gemini y Antigravity son marcas de sus respectivos dueños. Tramtai no está afiliado a ellos.',
      'Claude, Codex, GitHub Copilot, Gemini and Antigravity are trademarks of their respective owners. Tramtai is not affiliated with them.')}</p>
  </div>
</footer>`
}

function desescapar(s: string): string {
  return s.replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&')
}

// Terminal son las pantallas de la terminal de la portada (site/capturas/terminal.json, de capturas --terminal): el HTML
// de cada página en cada idioma, con clases t-<token> que la hoja pinta según el tema.
export type Terminal = Record<'es' | 'en', Record<'inicio' | 'apis' | 'historial', string>>
// PANTALLAS son sus pestañas, con la tecla que lleva a cada página en la TUI y lo que oye un lector de pantalla (cada
// pantalla es role="img": su texto sigue seleccionable, pero 28 filas de columnas no se leen bien en voz alta).
const PANTALLAS = [
  { nombre: 'inicio', tecla: '1', es: 'Inicio', en: 'Home',
    alt: { es: 'La terminal en Inicio: el estado del servicio, la red, las salidas de los últimos 7 días y los primeros pasos', en: 'The terminal on Home: the service status, the network, the last 7 days of departures and getting started' } },
  { nombre: 'apis', tecla: '4', es: 'Mis APIs', en: 'My APIs',
    alt: { es: 'La terminal en Mis APIs: las APIs por aplicación, en sus grupos, con su estado y su consumo', en: 'The terminal on My APIs: the per-app APIs, in their groups, with their status and usage' } },
  { nombre: 'historial', tecla: '5', es: 'Historial', en: 'History',
    alt: { es: 'La terminal en Historial: las peticiones recientes, con su estado, y los filtros', en: 'The terminal on History: the recent requests, with their status, and the filters' } },
] as const

// ventanaTerminal es la ventana de la portada (comun:terminal): semáforo, pestañas, rótulo y un panel por página con sus
// dos idiomas. Sin JavaScript las pestañas esperan ocultas, Inicio se ve y las otras quedan en <details>; sitio.js las
// saca de ahí al encender las pestañas.
function ventanaTerminal(t: Terminal): string {
  const pestanas = PANTALLAS.map((p, i) => `<button type="button" role="tab" id="terminal-${p.nombre}" aria-controls="terminal-${p.nombre}-panel" aria-selected="${i === 0}" tabindex="${i === 0 ? 0 : -1}" data-tecla="${p.tecla}">${p.tecla} ${bilingue(p.es, p.en)}</button>`)
  const paneles = PANTALLAS.map((p, i) => {
    const panel = `<div class="ventana-pantalla" id="terminal-${p.nombre}-panel" aria-labelledby="terminal-${p.nombre}" tabindex="0">
<pre class="pantalla" lang="es" role="img" aria-label="${escapar(p.alt.es)}">${t.es[p.nombre]}</pre>
<pre class="pantalla" lang="en" role="img" aria-label="${escapar(p.alt.en)}">${t.en[p.nombre]}</pre>
</div>`
    return i === 0 ? panel : `<details class="ventana-mas">\n<summary>${p.tecla} ${bilingue(p.es, p.en)}</summary>\n${panel}\n</details>`
  })
  return `<div class="ventana-terminal">
<div class="ventana-barra">
<span class="ventana-semaforo" aria-hidden="true"><span></span><span></span><span></span></span>
<div class="pestanas-lista" role="tablist" aria-label="Páginas de la terminal" data-aria-en="Terminal pages" hidden>${pestanas.join('')}</div>
<span class="ventana-rotulo">tramtai terminal · ssh</span>
</div>
${paneles.join('\n')}
</div>`
}

// recorteTerminal es FIG 17 de Producto (comun:terminal-recorte): la pantalla de Inicio de la terminal, la misma de la
// ventana de Inicio, que la hoja recorta a su esquina de arriba a la izquierda (el menú con sus páginas).
function recorteTerminal(t: Terminal): string {
  const alt = { es: 'La terminal en Inicio: el menú con las páginas de la app y el estado del servicio', en: 'The terminal on Home: the menu with the app’s pages and the service status' }
  return (['es', 'en'] as const).map((i) => `<pre class="pantalla" lang="${i}" role="img" aria-label="${escapar(alt[i])}">${t[i].inicio}</pre>`).join('\n')
}

// CAMINO es el dibujo de tu app → Tramtai → los agentes (comun:camino): FIG 2 de Inicio y FIG 1 de Desarrolladores. Cada
// página pone su <figure class="tarjeta fig-camino vivo"> y su rótulo; las líneas se dibujan al aparecer y el flujo
// corre por ellas.
export const CAMINO = `<svg viewBox="0 0 352 196" role="img" aria-label="Tu app habla con Tramtai, y Tramtai con Claude Code, Codex, GitHub Copilot y la Gemini API" data-aria-en="Your app talks to Tramtai, and Tramtai to Claude Code, Codex, GitHub Copilot and the Gemini API">
  <g fill="none" stroke="var(--acento)" stroke-width="1.3" stroke-linejoin="round">
    <path class="dibuja" pathLength="1" d="M8 140 L44 122 L80 140 L44 158 Z M8 140 V150 L44 168 L80 150 V140 M44 158 V168"/>
    <path class="dibuja" pathLength="1" style="--retraso: .4s" d="M102 88 L148 65 L194 88 L148 111 Z M102 88 V100 L148 123 L194 100 V88 M148 111 V123"/>
  </g>
  <g fill="none" stroke="var(--acento)" stroke-width="1.3" stroke-linecap="round">
    <path class="base" d="M62 131 L125 99.5 M194 88 L232 38 M194 88 L232 70 M194 88 L232 102 M194 88 L232 134"/>
    <path class="flujo" stroke-dasharray="4 6" d="M62 131 L125 99.5"/>
    <path class="flujo" stroke-dasharray="4 6" d="M194 88 L232 38 M194 88 L232 70 M194 88 L232 102 M194 88 L232 134"/>
  </g>
  <circle class="pulso" cx="236" cy="38" r="5" fill="var(--claude)"/>
  <circle class="pulso" cx="236" cy="70" r="5" fill="var(--codex)" style="--retraso: .6s"/>
  <circle class="pulso" cx="236" cy="102" r="5" fill="var(--copilot)" style="--retraso: 1.2s"/>
  <circle class="pulso" cx="236" cy="134" r="5" fill="var(--gemini)" style="--retraso: 1.8s"/>
  <g font-size="13">
    <text x="44" y="190" text-anchor="middle" lang="es">tu app</text><text x="44" y="190" text-anchor="middle" lang="en">your app</text>
    <text class="nombre" x="148" y="92.5" text-anchor="middle">Tramtai</text>
    <text x="246" y="42.5">Claude Code</text>
    <text x="246" y="74.5">Codex</text>
    <text x="246" y="106.5">GitHub Copilot</text>
    <text x="246" y="138.5">Gemini API</text>
  </g>
</svg>`

// Apartado es una entrada de «En esta página»: el id de su sección y su título en cada idioma, en texto.
export interface Apartado { id: string; es: string; en: string }

// BOTON_INDICE abre el índice de la documentación en el móvil: sitio.js lo enciende; sin JavaScript se queda oculto y el
// índice se ve abierto encima del contenido.
const BOTON_INDICE = `<button type="button" class="doc-indice-boton" aria-expanded="false" aria-controls="doc-indice-lista" hidden>${svg('<path d="M4 7h16M4 12h10M4 17h13"/>')}${bilingue('Índice', 'Contents')}</button>`

// enEstaPagina es la columna de la derecha de la documentación: un enlace por apartado; sitio.js marca al bajar el que se
// está leyendo.
function enEstaPagina(apartados: Apartado[]): string {
  if (!apartados.length) return '<aside class="doc-pagina"></aside>'
  return `<aside class="doc-pagina">
<nav aria-label="En esta página" data-aria-en="On this page">
<p class="fig doc-grupo" aria-hidden="true">${bilingue('En esta página', 'On this page')}</p>
<ul>${apartados.map((a) => `\n  <li><a href="#${a.id}">${bilingue(a.es, a.en)}</a></li>`).join('')}
</ul>
</nav>
</aside>`
}

// columnasDoc es la plantilla de documentación (spec §6) de la Ayuda: el índice a la izquierda (indice, sus grupos y
// enlaces), el contenido en medio y «En esta página» a la derecha. servidor.html y desarrolladores.html la escriben a
// mano con el mismo marcado y rellenar les pone el botón y «En esta página».
export function columnasDoc(indice: string, contenido: string, apartados: Apartado[]): string {
  return `<div class="doc">
<nav class="doc-indice" aria-label="Índice" data-aria-en="Contents">
${BOTON_INDICE}
<div class="doc-indice-lista" id="doc-indice-lista">
${indice}
</div>
</nav>
<article id="contenido" tabindex="-1" class="prosa">
${contenido}
</article>
${enEstaPagina(apartados)}
</div>`
}

// apartadosDe da los apartados del contenido de una página escrita a mano (de <article id="contenido"> a su «En esta
// página»): cada <section id> con su <h2>, en los dos idiomas.
function apartadosDe(html: string): Apartado[] {
  const desde = html.indexOf('<article id="contenido"')
  const hasta = html.indexOf('<!-- comun:en-esta-pagina -->')
  if (desde < 0 || hasta < desde) return []
  const contenido = html.slice(desde, hasta)
  const en = apartados(contenido, 'en')
  return apartados(contenido, 'es').map((a) => ({ id: a.id, es: a.titulo, en: en.find((b) => b.id === a.id)?.titulo ?? a.titulo }))
}

// rellenar pone al día las zonas comunes (<!-- comun:x --> … <!-- /comun:x -->) de una página escrita a mano, con su
// título y su descripción. El aviso de los riesgos (comun:riesgos y comun:riesgos-servidor), la terminal
// (comun:terminal y su recorte, comun:terminal-recorte), el camino (comun:camino) y las piezas de la documentación (comun:indice-boton y
// comun:en-esta-pagina) solo van donde la página
// los pide, con los textos de los catálogos, las pantallas de terminal.json y los apartados de la propia página.
export function rellenar(html: string, l: Lugar, textos: { es: Textos; en: Textos; terminal?: Terminal }): string {
  const titulo = desescapar(html.match(/<title[^>]*>([^<]*)<\/title>/)?.[1] ?? 'Tramtai')
  const descripcion = desescapar(html.match(/<meta name="description" content="([^"]*)"[^>]*>/)?.[1] ?? '')
  const zonas: Record<string, string> = { cabeza: cabeza(l, titulo, descripcion), cabecera: cabecera(l.rel, l.actual), pie: pie(l.rel, l.actual) }
  const opcionales: Record<string, string> = {
    anclas: saltoPortada(),
    riesgos: listaRiesgos(textos.es, textos.en, false), 'riesgos-servidor': listaRiesgos(textos.es, textos.en, true),
    ...(textos.terminal ? { terminal: ventanaTerminal(textos.terminal), 'terminal-recorte': recorteTerminal(textos.terminal) } : {}),
    'indice-boton': BOTON_INDICE, 'en-esta-pagina': enEstaPagina(apartadosDe(html)), camino: CAMINO,
  }
  for (const [zona, contenido] of [...Object.entries(zonas), ...Object.entries(opcionales)]) {
    const re = new RegExp(`<!-- comun:${zona} -->[\\s\\S]*?<!-- /comun:${zona} -->`)
    if (!re.test(html)) {
      if (zona in opcionales) continue
      throw new Error(`falta la zona comun:${zona}`)
    }
    html = html.replace(re, () => `<!-- comun:${zona} -->\n${contenido}\n<!-- /comun:${zona} -->`)
  }
  return html
}

// ANCLAS_VIEJAS lleva cada sección de la ayuda de una sola página (ayuda anterior) a su tema.
export const ANCLAS_VIEJAS: Record<string, string> = {
  instalar: 'empezar.html', agentes: 'agentes.html', terminos: 'agentes.html#terminos', aplicaciones: 'conectar.html',
  herramientas: 'herramientas.html', apis: 'conectar.html', recetas: 'recetas.html', archivos: 'archivos.html',
  actualizaciones: 'ajustes.html', preguntas: 'preguntas.html', mas: 'index.html',
}

// destinoAncla es también el script de la página de paso (se incrusta tal cual con ANCLAS_VIEJAS).
export function destinoAncla(hash: string): string {
  const ancla = hash.replace(/^#/, '')
  return 'ayuda/' + (Object.hasOwn(ANCLAS_VIEJAS, ancla) ? ANCLAS_VIEJAS[ancla] : '')
}

// ANCLAS_PORTADA lleva cada anclaje de la portada de hoy a su sitio nuevo (spec §7); Inicio la incrusta como salto. Las
// tres últimas son de la portada de antes de la v0.59 (#pagas, #capturas y #aviso).
export const ANCLAS_PORTADA: Record<string, string> = {
  descargar: 'descargar.html', como: 'producto.html#como', funciones: 'producto.html#funciones', 'por-que': 'producto.html#por-que',
  privacidad: 'producto.html#privacidad', agentes: 'agentes.html', app: '#formas', terminal: '#formas', servidor: 'servidor.html',
  pagas: 'producto.html#por-que', capturas: '#formas', aviso: 'producto.html#privacidad',
}

// destinoPortada es el destino de un anclaje viejo, o null si no está en la tabla (la portada nueva lo conserva o no existe).
export function destinoPortada(hash: string): string | null {
  const ancla = hash.replace(/^#/, '')
  return Object.hasOwn(ANCLAS_PORTADA, ancla) ? ANCLAS_PORTADA[ancla] : null
}

// saltoPortada es el salto de Inicio (comun:anclas, en su <head>): un anclaje viejo va a su sitio nuevo antes de pintar,
// y también si cambia con la página abierta. Sin JavaScript, cada anclaje viejo es el id de la tarjeta de «Sigue por
// aquí» (o de la ventana) que lleva a ese sitio.
// Es JavaScript escrito tal cual (no el texto de destinoPortada, que cambia con cómo se compile): las pruebas lo ejecutan.
function saltoPortada(): string {
  return `<script>(function () {
  var anclas = ${JSON.stringify(ANCLAS_PORTADA)};
  function saltar() {
    var ancla = location.hash.slice(1);
    if (Object.prototype.hasOwnProperty.call(anclas, ancla)) location.replace(anclas[ancla]);
  }
  saltar();
  addEventListener('hashchange', saltar);
})();</script>`
}

export function escapar(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}

function bilingue(es: string, en: string): string {
  return `<span lang="es">${conOpciones(es)}</span><span lang="en">${conOpciones(en)}</span>`
}

// conOpciones escapa un texto y envuelve sus opciones de la CLI en .opcion-cli, que no se parte tras el guion.
function conOpciones(s: string): string {
  return partesOpciones(s).map((p) => (p.opcion ? `<span class="opcion-cli">${escapar(p.texto)}</span>` : escapar(p.texto))).join('')
}

const GUIA = new Set(['conectar', 'herramientas', 'archivos'])

// AVISO_DESCARGA es lo que hay que saber antes de descargar; la portada repite la primera frase bajo Descargar.
export const AVISO_DESCARGA = {
  es: 'Necesitas al menos un agente: Tramtai instala el CLI de Claude Code, Codex, GitHub Copilot o Antigravity y te ayuda a iniciar sesión; para Gemini basta una clave de la Gemini API. En Windows puede salir «Windows protegió su PC», porque el instalador aún no está firmado.',
  en: 'You need at least one agent: Tramtai installs the CLI of Claude Code, Codex, GitHub Copilot or Antigravity and helps you sign in; for Gemini, a Gemini API key is enough. On Windows you may see “Windows protected your PC”, because the installer is not signed yet.',
}

// pagina es el armazón de cada página generada; main es su <main> entero (el de la Ayuda, mainAyuda).
function pagina(l: Lugar, titulo: [string, string], descripcion: string, main: string): string {
  return `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title data-en="${escapar(titulo[1])} · Tramtai">${escapar(titulo[0])} · Tramtai</title>
<meta name="description" content="${escapar(descripcion)}">
${cabeza(l, `${titulo[0]} · Tramtai`, descripcion)}
</head>
<body>
${cabecera(l.rel, l.actual)}

${main}

${pie(l.rel, l.actual)}
</body>
</html>
`
}

// mainAyuda es el <main> de la Ayuda, con la plantilla de documentación dentro. tema-ayuda le dice a sitio.js que la
// Ayuda recuerda su lenguaje de ejemplo aparte.
const mainAyuda = (doc: string) => `<main class="envoltura doc-envoltura tema-ayuda">\n${doc}\n</main>`

// GRUPOS_AYUDA ordenan los temas en el índice de la Ayuda y en su inicio; cada tema va en uno, en el orden de temas.
export const GRUPOS_AYUDA: (Apartado & { temas: string[] })[] = [
  { id: 'g-empezar', es: 'Empezar', en: 'Get started', temas: ['empezar', 'agentes', 'preparar', 'probar'] },
  { id: 'g-apps', es: 'Desde tus apps', en: 'From your apps', temas: ['conectar', 'conectar-herramientas', 'archivos', 'json', 'recetas', 'herramientas'] },
  { id: 'g-dia', es: 'Día a día', en: 'Day to day', temas: ['presupuestos', 'historial', 'espacios', 'claves', 'ajustes'] },
  { id: 'g-servidor', es: 'Servidor', en: 'Server', temas: ['servidor', 'entornos'] },
  { id: 'g-dudas', es: 'Dudas y errores', en: 'Questions and errors', temas: ['problemas', 'conceptos', 'preguntas'] },
]
const LUPA = svg('<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>')

// indiceAyuda es la columna de la izquierda de la Ayuda: el buscador, el inicio, los temas por grupos (el actual, con
// aria-current) y las dos guías.
function indiceAyuda(f: Fuente, actual: string): string {
  const titulo = (id: string) => {
    const t = f.temas.find((x) => x.id === id)
    if (!t) throw new Error(`GRUPOS_AYUDA cita un tema que no existe: ${id}`)
    return bilingue(f.es[t.titulo] ?? id, f.en[t.titulo] ?? id)
  }
  const enlace = (href: string, texto: string, marcado = false) => `\n  <li><a href="${href}"${marcado ? ' aria-current="page"' : ''}>${texto}</a></li>`
  return `<button type="button" class="buscar-campo" data-buscar hidden>${LUPA}<span>${bilingue('Buscar', 'Search')}</span><kbd>Ctrl K</kbd></button>
<ul>${enlace('index.html', bilingue('Inicio de la Ayuda', 'Help home'), actual === '')}
</ul>
${GRUPOS_AYUDA.map((g) => `<p class="fig doc-grupo" id="i-${g.id}">${bilingue(g.es, g.en)}</p>\n<ul aria-labelledby="i-${g.id}">${g.temas.map((id) => enlace(`${id}.html`, titulo(id), id === actual)).join('')}\n</ul>`).join('\n')}
<p class="fig doc-grupo" id="i-mas">${bilingue('Más documentación', 'More documentation')}</p>
<ul aria-labelledby="i-mas">${enlace('../servidor.html', bilingue('Guía del servidor', 'Server guide'))}${enlace('../desarrolladores.html', bilingue('Guía del desarrollador', 'Developer guide'))}
</ul>`
}

function tarjetas(f: Fuente, lista: Tema[]): string {
  const tx = (k: string) => bilingue(f.es[k] ?? k, f.en[k] ?? k)
  return `<div class="tarjetas-temas">${lista.map((t) => `
  <a class="tarjeta-tema" href="${t.id}.html"><strong>${tx(t.titulo)}</strong><span>${tx(t.resumen)}</span></a>`).join('')}
</div>`
}

// imagenes son las cuatro variantes de una captura (claro/oscuro × es/en) que existen; el CSS deja ver la del tema y el
// idioma activos. alt da el texto alternativo en cada idioma; con aSuTamano la imagen no pasa de su ancho (los recortes
// pequeños de un paso no se estiran hasta llenar la columna, y su alto se reserva por el tamaño declarado).
function imagenes(f: Fuente, id: string, alt: (i: 'es' | 'en') => string, aSuTamano = false): string {
  return (['claro', 'oscuro'] as const).flatMap((c) => (['es', 'en'] as const).map((i) => {
    const archivo = `${id}.${c}.${i}.webp`
    const texto = alt(i)
    const m = f.capturas.get(archivo)
    return m ? `<img class="solo-${c}" lang="${i}" src="capturas/${archivo}" width="${m.ancho}" height="${m.alto}" loading="lazy" alt="${escapar(texto)}"${aSuTamano ? ` style="max-width: ${m.ancho}px"` : ''}>` : ''
  })).join('')
}

function bloque(f: Fuente, tema: Tema, b: Bloque, n: number): string {
  const tx = (k: string) => bilingue(f.es[k] ?? k, f.en[k] ?? k)
  const de = (i: 'es' | 'en', k: string) => (i === 'es' ? f.es : f.en)[k] ?? k
  switch (b.tipo) {
    case 'diagrama':
      return `<ol class="diagrama" style="--paradas: ${b.paradas.length}">${b.paradas.map((p, i) => `
  <li><strong>${i + 1}. ${tx(p.titulo)}</strong>${p.nota ? `<span>${tx(p.nota)}</span>` : ''}</li>`).join('')}
</ol>${(b.ramas ?? []).map((r) => `\n<p class="diagrama-rama">↳ ${r.desde + 1} → ${r.hacia + 1}: ${tx(r.etiqueta)}</p>`).join('')}`
    case 'pasos':
      return `<ol class="pasos">${b.pasos.map((p) => `\n  <li>${tx(p)}</li>`).join('')}\n</ol>`
    case 'captura': {
      const imgs = imagenes(f, b.id, (i) => de(i, b.alt))
      return imgs ? `<figure class="captura">${imgs}</figure>` : ''
    }
    case 'recorrido':
      // Un paso sin captura (la demo no muestra su ancla, ver SIN_CAPTURA) sale solo con su texto. El texto alternativo
      // dice que es una captura del paso, sin repetir a secas el título que va justo encima.
      return `<ol class="pasos recorrido-pasos">${b.pasos.map((p) => {
        const imgs = imagenes(f, capturaDeAncla(p.ancla), (i) => `${i === 'es' ? 'Captura de pantalla' : 'Screenshot'}: ${de(i, p.titulo)}`, true)
        return `\n  <li><div><strong>${tx(p.titulo)}</strong> ${tx(p.texto)}${imgs ? `<figure class="captura captura-paso">${imgs}</figure>` : ''}</div></li>`
      }).join('')}\n</ol>`
    case 'ejemplo': {
      const porIdioma = { es: f.ejemplos(b, 'es'), en: f.ejemplos(b, 'en') }
      const id = (e: string) => `e${n}-${e}`
      const lista = porIdioma.es.map((e, i) => `<button type="button" role="tab" id="${id(e.id)}" aria-controls="${id(e.id)}-panel" aria-selected="${i === 0}" tabindex="${i === 0 ? 0 : -1}" data-lenguaje="${e.id}">${escapar(e.titulo)}</button>`).join('')
      const paneles = porIdioma.es.map((e, i) => `
  <div role="tabpanel" id="${id(e.id)}-panel" aria-labelledby="${id(e.id)}">
    <pre class="codigo" lang="es"><code>${escapar(e.codigo)}</code></pre>
    <pre class="codigo" lang="en"><code>${escapar(porIdioma.en[i]?.codigo ?? e.codigo)}</code></pre>
  </div>`).join('')
      return `<div class="pestanas" data-pestanas>\n  <div class="pestanas-lista" role="tablist" aria-label="Lenguaje" data-aria-en="Language">${lista}</div>${paneles}\n</div>`
    }
    case 'tabla':
      return `<div class="tabla"><table>
  <thead><tr><th></th>${b.columnas.map((c) => `<th>${tx(c)}</th>`).join('')}</tr></thead>
  <tbody>${b.filas.map((fila) => `\n    <tr><th scope="row">${tx(fila.titulo)}</th>${fila.celdas.map((c) => `<td>${tx(c)}</td>`).join('')}</tr>`).join('')}
  </tbody>
</table></div>`
    case 'preguntas':
      return b.preguntas.map((p) => {
        const destino = p.tema ? f.temas.find((t) => t.id === p.tema) : undefined
        return `<details id="${p.id}"><summary>${tx(p.pregunta)}</summary><p>${tx(p.respuesta)}${destino ? ` <a href="${destino.id}.html">${tx(destino.titulo)} →</a>` : ''}</p></details>`
      }).join('\n')
    case 'errores':
      return `<dl class="errores">${b.codigos.map((c) => {
        const e = ERRORES[c]
        const titulo = e?.titulo ?? 'error.' + c
        return `\n  <div id="e-${c}"><dt>${tx(titulo)}${c === 'sin_conexion' ? '' : ` <code>${escapar(c)}</code>`}</dt><dd>${e ? tx(e.solucion) : ''}</dd></div>`
      }).join('')}\n</dl>`
    case 'nota':
      return `<p class="aviso">${tx(b.texto)}</p>`
    case 'descargas':
      // Lo que frena el primer arranque va junto al botón, no solo en las preguntas de más abajo.
      return `<p><a class="boton principal" href="../descargar.html">${bilingue('Descargar Tramtai', 'Download Tramtai')}</a></p>
<p class="aviso">${bilingue(AVISO_DESCARGA.es, AVISO_DESCARGA.en)} <a href="#windows">${bilingue('Qué hacer con el aviso de Windows', 'What to do about the Windows warning')}</a></p>`
    default:
      return ''
  }
}

// SECCIONES son los bloques que abren una sección con su h2 (y su entrada en «En esta página»); los demás (capturas,
// notas) van en la sección del bloque anterior.
const SECCIONES: Partial<Record<Bloque['tipo'], [string, string]>> = {
  diagrama: ['Cómo funciona', 'How it works'], recorrido: ['Qué verás en pantalla', 'What you will see on screen'], pasos: ['Paso a paso', 'Step by step'], ejemplo: ['Ejemplo', 'Example'],
  tabla: ['De un vistazo', 'At a glance'], preguntas: ['Preguntas', 'Questions'], errores: ['Si algo falla', 'If something fails'],
}

function paginaTema(f: Fuente, tema: Tema): string {
  const tx = (k: string) => bilingue(f.es[k] ?? k, f.en[k] ?? k)
  const relacionados = tema.relacionados.map((r) => f.temas.find((t) => t.id === r)).filter((t): t is Tema => !!t)
  const secciones: { id: string; titulo: [string, string]; partes: string[] }[] = [{ id: '', titulo: ['', ''], partes: [] }]
  bloquesVisibles(tema, 'sitio').forEach((b, i) => {
    // Unos pasos con titulo abren su propia sección, con ese título y el id de su clave (s-soloTerminal → s-soloterminal).
    const propio = b.tipo === 'pasos' ? b.titulo : undefined
    const titulo: [string, string] | undefined = propio ? [f.es[propio] ?? propio, f.en[propio] ?? propio] : SECCIONES[b.tipo]
    const id = propio ? 's-' + propio.split('.').pop()!.toLowerCase() : 's-' + b.tipo
    if (titulo && !secciones.some((s) => s.id === id)) secciones.push({ id, titulo, partes: [] })
    const html = bloque(f, tema, b, i)
    if (html) secciones[secciones.length - 1].partes.push(html)
  })
  if (GUIA.has(tema.id)) secciones[secciones.length - 1].partes.push(`<p class="aviso"><a href="../desarrolladores.html">${bilingue('Guía del desarrollador', 'Developer guide')}</a>: ${bilingue('el ciclo completo en Python, JavaScript, Go y curl.', 'the full cycle in Python, JavaScript, Go and curl.')}</p>`)
  // Al final del tema, fuera de la última sección (no es una pregunta más), como en la Ayuda de la Web UI.
  secciones.push({ id: '', titulo: ['', ''], partes: [`<p class="aviso">${tx(tema.terminal)}</p>`] })
  if (relacionados.length) secciones.push({ id: 's-relacionados', titulo: ['Temas relacionados', 'Related topics'], partes: [tarjetas(f, relacionados)] })
  const conTitulo = secciones.filter((s) => s.id && s.partes.length)
  const cuerpo = secciones.filter((s) => s.partes.length).map((s) => (s.id
    ? `<section id="${s.id}"><h2>${bilingue(...s.titulo)}</h2>\n${s.partes.join('\n\n')}\n</section>`
    : s.partes.join('\n\n'))).join('\n\n')
  const grupo = GRUPOS_AYUDA.find((g) => g.temas.includes(tema.id))
  const contenido = `<header class="doc-cabeza">
  <p class="fig doc-ruta">${bilingue('Ayuda', 'Help')}${grupo ? ` · ${bilingue(grupo.es, grupo.en)}` : ''}</p>
  <h1 class="titular">${tx(tema.titulo)}</h1>
  <p class="entradilla">${tx(tema.resumen)}</p>
</header>

${cuerpo}`
  return pagina({ rel: '../', ruta: `ayuda/${tema.id}.html`, actual: 'ayuda' }, [f.es[tema.titulo] ?? tema.id, f.en[tema.titulo] ?? tema.id],
    f.es[tema.resumen] ?? '', mainAyuda(columnasDoc(indiceAyuda(f, tema.id), contenido, conTitulo.map((s) => ({ id: s.id, es: s.titulo[0], en: s.titulo[1] })))))
}

function paginaInicio(f: Fuente): string {
  const grupos = GRUPOS_AYUDA.map((g) => `<section id="${g.id}"><h2>${bilingue(g.es, g.en)}</h2>
${tarjetas(f, g.temas.map((id) => f.temas.find((t) => t.id === id)!))}
</section>`).join('\n\n')
  const mas: [string, string] = ['Más documentación', 'More documentation']
  const contenido = `<header class="doc-cabeza">
  <p class="fig doc-ruta">${bilingue('Ayuda', 'Help')}</p>
  <h1 class="titular"><span lang="es">La Ayuda de Tramtai, <em>tema a tema.</em></span><span lang="en">Tramtai’s Help, <em>topic by topic.</em></span></h1>
  <p class="entradilla">${bilingue('Qué hace Tramtai, cómo empezar, cada función y qué hacer si algo falla.', 'What Tramtai does, how to start, every feature and what to do if something fails.')}</p>
</header>
<button type="button" class="buscar-campo" data-buscar hidden>${LUPA}<span>${bilingue('Buscar en el sitio', 'Search the site')}</span><kbd>Ctrl K</kbd></button>

${grupos}

<section id="mas"><h2>${bilingue(...mas)}</h2>
<ul class="lista-enlaces">
  <li><a href="../desarrolladores.html">${bilingue('Guía del desarrollador', 'Developer guide')}</a></li>
  <li><a href="../servidor.html">${bilingue('Guía del servidor', 'Server guide')}</a></li>
  <li><a href="https://github.com/MscottB/Tramtai/blob/main/docs/inicio.md">${bilingue('Documentación completa en GitHub', 'Full documentation on GitHub')}</a></li>
</ul>
</section>`
  const enPagina = [...GRUPOS_AYUDA, { id: 'mas', es: mas[0], en: mas[1] }]
  return pagina({ rel: '../', ruta: 'ayuda/', actual: 'ayuda' }, ['Ayuda', 'Help'],
    'Ayuda de Tramtai: instalar, conectar agentes y apps, archivos, JSON, recetas, herramientas y errores.', mainAyuda(columnasDoc(indiceAyuda(f, ''), contenido, enPagina)))
}

// pagina404 la sirve GitHub Pages en cualquier ruta que no existe: por eso todo va con la dirección absoluta.
function pagina404(): string {
  const main = `<main id="contenido" tabindex="-1" class="envoltura pagina-sola">
<article class="prosa">
<header class="doc-cabeza">
  <h1 class="titular">${bilingue('Esta página no existe', 'This page does not exist')}</h1>
  <p class="entradilla">${bilingue('Puede que la dirección esté mal escrita o que la página se haya movido.', 'The address may be mistyped, or the page may have moved.')}</p>
</header>
<ul class="lista-enlaces">
  <li><a href="${URL_SITIO}">${bilingue('La portada de Tramtai', 'Tramtai’s home page')}</a></li>
  <li><a href="${URL_SITIO}ayuda/">${bilingue('La Ayuda, con su buscador', 'The Help, with its search box')}</a></li>
  <li><a href="${URL_SITIO}descargar.html">${bilingue('Descargar Tramtai', 'Download Tramtai')}</a></li>
</ul>
</article>
</main>`
  return pagina({ rel: URL_SITIO, ruta: null, actual: '' }, ['Página no encontrada', 'Page not found'], 'Esta página no existe.', main)
}

function sitemap(f: Fuente): string {
  const urls = ['', 'producto.html', 'agentes.html', 'servidor.html', 'desarrolladores.html', 'ayuda/', 'descargar.html', ...f.temas.map((t) => `ayuda/${t.id}.html`)]
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.map((u) => `  <url><loc>${URL_SITIO}${u}</loc></url>`).join('\n')}
</urlset>
`
}

// Entrada es una fila del buscador del sitio (buscar.json): u es la dirección desde la raíz con su ancla, ti el título, d
// dónde está («Ayuda › Agentes», «Servidor») y x el texto normalizado donde se busca.
// compactar deja el texto de una entrada en sus palabras distintas de más de 2 letras y 160 caracteres: la búsqueda es por palabras sueltas, y así buscar.json sigue pequeño.
const compactar = (x: string) => [...new Set(x.split(' '))].filter((w) => w.length > 2).join(' ').slice(0, 160)

interface Entrada { u: string; ti: string; d: string; x: string }

// soloIdioma deja de un HTML solo lo que se ve en un idioma: quita los elementos con lang del otro (un span, un div
// entero con su contenido). Supone un HTML bien cerrado, como el de las páginas del sitio.
function soloIdioma(html: string, idioma: 'es' | 'en'): string {
  const VACIOS = /^(?:img|br|hr|input|meta|link|source)$/i
  const pila: boolean[] = []
  let saltar = 0
  let salida = ''
  for (const [, etiqueta, texto] of html.replace(/<!--[\s\S]*?-->/g, '').matchAll(/(<[^>]+>)|([^<]+)/g)) {
    if (texto !== undefined) { if (!saltar) salida += texto; continue }
    const m = etiqueta.match(/^<(\/?)([a-z][a-z0-9]*)([^>]*?)(\/?)>$/i)
    if (!m) continue
    if (m[1]) { if (pila.pop()) saltar--; else if (!saltar) salida += etiqueta; continue }
    if (m[4] || VACIOS.test(m[2])) { if (!saltar) salida += etiqueta; continue }
    const otro = m[2] !== 'html' && m[3].includes(`lang="${idioma === 'es' ? 'en' : 'es'}"`)
    pila.push(otro)
    if (otro) saltar++
    else if (!saltar) salida += etiqueta
  }
  return salida
}

// apartados saca de una página escrita a mano cada <section id> (con el título que abre) o <h2|h3 id>, con el primer
// párrafo que sigue, en un idioma.
function apartados(pagina: string, idioma: 'es' | 'en'): { id: string; titulo: string; texto: string }[] {
  const html = soloIdioma(pagina, idioma)
  const limpio = (s: string) => desescapar(s.replace(/<[^>]+>/g, '')).replace(/\s+/g, ' ').trim()
  return [...html.matchAll(/<(?:section\b[^>]*\bid="([^"]+)"[^>]*>\s*<h[23]\b[^>]*>|h[23]\b[^>]*\bid="([^"]+)"[^>]*>)([\s\S]*?)<\/h[23]>/g)].map((m) => {
    const resto = html.slice(m.index + m[0].length)
    const corte = resto.search(/<(?:h[23]|section)\b/)
    const p = (corte < 0 ? resto : resto.slice(0, corte)).match(/<p\b[^>]*>([\s\S]*?)<\/p>/)
    return { id: m[1] ?? m[2], titulo: limpio(m[3]), texto: p ? limpio(p[1]) : '' }
  })
}
const DONDE: Record<string, [string, string]> = { 'servidor.html': ['Servidor', 'Server'], 'desarrolladores.html': ['Desarrolladores', 'Developers'] }

// indiceBusqueda es lo que busca sitio.js: por idioma, una entrada por tema y ancla de la Ayuda y otra por apartado de
// Servidor y Desarrolladores (leídos de site/, que da f.paginas), con su texto ya normalizado.
function indiceBusqueda(f: Fuente): string {
  const indice: Record<string, Entrada[]> = {}
  for (const [i, idioma] of (['es', 'en'] as const).entries()) {
    const textos = idioma === 'es' ? f.es : f.en
    const titulo = (c: string) => textos['error.' + c] ?? c
    const ayuda = idioma === 'es' ? 'Ayuda' : 'Help'
    indice[idioma] = f.temas.flatMap((tema) => {
      const nombre = textos[tema.titulo] ?? tema.id
      const porAncla = new Map<string, { ti: string; n: number; x: string[] }>()
      for (const c of campos(tema, textos, titulo)) {
        const e = porAncla.get(c.ancla) ?? { ti: c.donde, n: 0, x: [] }
        const x = normalizar(c.texto)
        // El título ya se busca en ti + d. El tema entero (sin ancla) guarda solo su introducción, la segunda: el resto del
        // texto está en las entradas de sus apartados.
        if (c.ancla ? x !== normalizar(e.ti) : e.n === 1) e.x.push(x)
        e.n++
        porAncla.set(c.ancla, e)
      }
      return [...porAncla].map(([a, e]) => ({ u: `ayuda/${tema.id}.html${a ? '#' + a : ''}`, ti: e.ti, d: a ? `${ayuda} › ${nombre}` : ayuda, x: compactar(e.x.join(' ')) }))
    })
    for (const [archivo, html] of Object.entries(f.paginas ?? {}))
      for (const a of apartados(html, idioma)) indice[idioma].push({ u: `${archivo}#${a.id}`, ti: a.titulo, d: DONDE[archivo][i], x: compactar(normalizar(a.texto)) })
  }
  return JSON.stringify(indice)
}

function paginaDePaso(): string {
  return `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<title>Ayuda · Tramtai</title>
<meta http-equiv="refresh" content="0; url=ayuda/">
<link rel="canonical" href="ayuda/">
<script>var ANCLAS_VIEJAS = ${JSON.stringify(ANCLAS_VIEJAS)};
${destinoAncla.toString().replace(/[ \t]+/g, ' ')}
location.replace(destinoAncla(location.hash));</script>
</head>
<body>
<p><a href="ayuda/">La ayuda de Tramtai se mudó a ayuda/ · Tramtai's help moved to ayuda/</a></p>
</body>
</html>
`
}

// generar devuelve cada archivo de la Ayuda del sitio por su ruta relativa a site/.
export function generar(f: Fuente): Map<string, string> {
  const archivos = new Map<string, string>()
  archivos.set('ayuda/index.html', paginaInicio(f))
  for (const tema of f.temas) archivos.set(`ayuda/${tema.id}.html`, paginaTema(f, tema))
  archivos.set('buscar.json', indiceBusqueda(f))
  archivos.set('ayuda.html', paginaDePaso())
  archivos.set('404.html', pagina404())
  archivos.set('sitemap.xml', sitemap(f))
  return archivos
}
