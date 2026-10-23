// sitio arma la Ayuda del sitio (site/ayuda/, la página de paso site/ayuda.html, 404.html y sitemap.xml) desde los
// mismos temas que la app, y es la única fuente de la cabeza, la barra y el pie de todas las páginas: rellenar los pone
// al día en las páginas escritas a mano (PAGINAS_FIJAS). Son funciones puras: sitio-ayuda.mjs les da los textos, los
// ejemplos y las capturas y escribe los archivos.
import type { Bloque, Tema } from '../src/ayuda/tipos'
import { bloquesVisibles, capturaDeAncla } from '../src/ayuda/vista'
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
}

// URL_SITIO es la dirección pública del sitio (GitHub Pages): canonical, Open Graph, sitemap y 404 la necesitan absoluta.
export const URL_SITIO = 'https://mscottb.github.io/Tramtai/'
const REPO = 'https://github.com/MscottB/Tramtai'

// Lugar dice dónde está una página: rel lleva de ella a la raíz del sitio y ruta es su dirección desde la raíz (null en
// la 404, que no tiene una propia).
type Actual = 'inicio' | 'ayuda' | 'desarrolladores' | ''
interface Lugar { rel: string; ruta: string | null; actual: Actual }

// PAGINAS_FIJAS son las páginas escritas a mano; sitio-ayuda.mjs les pone al día las zonas comunes con rellenar.
export const PAGINAS_FIJAS: (Lugar & { archivo: string })[] = [
  { archivo: 'index.html', rel: '', ruta: '', actual: 'inicio' },
  { archivo: 'desarrolladores.html', rel: '', ruta: 'desarrolladores.html', actual: 'desarrolladores' },
]

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
<meta name="theme-color" content="#0a1116" media="(prefers-color-scheme: dark)">
<link rel="icon" href="${l.rel}favicon.svg" type="image/svg+xml">
<link rel="icon" href="${l.rel}favicon-32.png" sizes="32x32" type="image/png">
<link rel="apple-touch-icon" href="${l.rel}apple-touch-icon.png">
<link rel="manifest" href="${l.rel}manifest.webmanifest">
<link rel="preload" href="${l.rel}overpass.woff2" as="font" type="font/woff2" crossorigin>
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

// cabecera es el enlace para saltar al contenido y la barra: las mismas cinco secciones, idioma, tema y Descargar. En
// pantallas medianas y pequeñas las secciones, el idioma y el tema van juntos en el panel del menú (#menu), así cada
// control cabe con 44 px. Desde la portada, sus secciones son anclas de la misma página.
export function cabecera(rel: string, actual: Actual): string {
  const raiz = actual === 'inicio' ? '' : rel || './'
  const marca = (a: Actual) => (a === actual ? ' aria-current="page"' : '')
  const tema = (t: keyof typeof ICONO_TEMA, es: string, en: string) =>
    `<button type="button" data-tema="${t}" aria-pressed="${t === 'sistema'}">${svg(ICONO_TEMA[t])}${soloLectores(es, en)}</button>`
  return `<a class="saltar" href="#contenido">${bilingue('Saltar al contenido', 'Skip to content')}</a>
<header class="barra">
  <div class="envoltura">
    <a class="marca" href="${rel || './'}"><img src="${rel}favicon.svg" width="30" height="30" alt=""><span class="marca-texto">Tramtai</span></a>
    <button type="button" class="menu-boton" aria-expanded="false" aria-controls="menu" hidden>${svg('<path d="M4 7h16M4 12h16M4 17h16"/>')}${soloLectores('Menú', 'Menu')}</button>
    <div id="menu" class="menu-panel">
      <nav aria-label="Secciones" data-aria-en="Sections">
        <a href="${raiz}#como">${bilingue('Cómo funciona', 'How it works')}</a>
        <a href="${raiz}#agentes">${bilingue('Agentes', 'Agents')}</a>
        <a href="${raiz}#funciones">${bilingue('Funciones', 'Features')}</a>
        <a href="${rel}ayuda/"${marca('ayuda')}>${bilingue('Ayuda', 'Help')}</a>
        <a href="${rel}desarrolladores.html"${marca('desarrolladores')}>${bilingue('Desarrolladores', 'Developers')}</a>
      </nav>
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
    <a class="boton principal" href="${raiz}#descargar" data-descarga>${svg('<path d="M12 4v11m0 0-4.5-4.5M12 15l4.5-4.5M5 20h14"/>')}<span class="boton-texto">${bilingue('Descargar', 'Download')}</span></a>
  </div>
</header>`
}

// pie lleva a todo lo que no está en la barra: empezar, el código, las versiones, la licencia y los términos.
export function pie(rel: string): string {
  return `<footer>
  <div class="envoltura">
    <nav class="pie-enlaces" aria-label="Pie de página" data-aria-en="Footer">
      <a href="${rel || './'}#descargar">${bilingue('Descargar', 'Download')}</a>
      <a href="${rel}ayuda/empezar.html">${bilingue('Primeros pasos', 'Getting started')}</a>
      <a href="${rel}ayuda/">${bilingue('Ayuda', 'Help')}</a>
      <a href="${rel}desarrolladores.html">${bilingue('Desarrolladores', 'Developers')}</a>
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

// rellenar pone al día las zonas comunes (<!-- comun:x --> … <!-- /comun:x -->) de una página escrita a mano, con su
// título y su descripción.
export function rellenar(html: string, l: Lugar): string {
  const titulo = desescapar(html.match(/<title[^>]*>([^<]*)<\/title>/)?.[1] ?? 'Tramtai')
  const descripcion = desescapar(html.match(/<meta name="description" content="([^"]*)">/)?.[1] ?? '')
  const zonas: Record<string, string> = { cabeza: cabeza(l, titulo, descripcion), cabecera: cabecera(l.rel, l.actual), pie: pie(l.rel) }
  for (const [zona, contenido] of Object.entries(zonas)) {
    const re = new RegExp(`<!-- comun:${zona} -->[\\s\\S]*?<!-- /comun:${zona} -->`)
    if (!re.test(html)) throw new Error(`falta la zona comun:${zona}`)
    html = html.replace(re, () => `<!-- comun:${zona} -->\n${contenido}\n<!-- /comun:${zona} -->`)
  }
  return html
}

// ANCLAS_VIEJAS lleva cada sección de la ayuda de una sola página (hasta la v0.51) a su tema.
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

export function escapar(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}

function bilingue(es: string, en: string): string {
  return `<span lang="es">${escapar(es)}</span><span lang="en">${escapar(en)}</span>`
}

const GUIA = new Set(['conectar', 'herramientas', 'archivos'])

// AVISO_DESCARGA es lo que hay que saber antes de descargar; la portada repite lo mismo bajo Descargar.
export const AVISO_DESCARGA = {
  es: 'Antes necesitas el CLI de al menos un agente (Claude Code, Codex o GitHub Copilot CLI) o una clave de la Gemini API. En Windows puede salir «Windows protegió su PC», porque el instalador aún no está firmado.',
  en: 'You first need the CLI of at least one agent (Claude Code, Codex or GitHub Copilot CLI) or a Gemini API key. On Windows you may see “Windows protected your PC”, because the installer is not signed yet.',
}

// pagina es el armazón de cada página generada; clase es la del <main> (con índice lateral, «ayuda»).
function pagina(l: Lugar, titulo: [string, string], descripcion: string, cuerpo: string, clase = 'ayuda-temas'): string {
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

<main id="contenido" tabindex="-1" class="envoltura ${clase}">
${cuerpo}
</main>

${pie(l.rel)}
</body>
</html>
`
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
      return `<p><a class="boton principal" href="../index.html#descargar">${bilingue('Descargar Tramtai', 'Download Tramtai')}</a></p>
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
    const titulo = SECCIONES[b.tipo]
    if (titulo && !secciones.some((s) => s.id === 's-' + b.tipo)) secciones.push({ id: 's-' + b.tipo, titulo, partes: [] })
    const html = bloque(f, tema, b, i)
    if (html) secciones[secciones.length - 1].partes.push(html)
  })
  if (GUIA.has(tema.id)) secciones[secciones.length - 1].partes.push(`<p class="aviso"><a href="../desarrolladores.html">${bilingue('Guía del desarrollador', 'Developer guide')}</a>: ${bilingue('el ciclo completo en Python, JavaScript, Go y curl.', 'the full cycle in Python, JavaScript, Go and curl.')}</p>`)
  if (relacionados.length) secciones.push({ id: 's-relacionados', titulo: ['Temas relacionados', 'Related topics'], partes: [tarjetas(f, relacionados)] })
  const conTitulo = secciones.filter((s) => s.id && s.partes.length)
  const indice = conTitulo.length >= 3
    ? `<nav class="indice" aria-label="En esta página" data-aria-en="On this page">
  <ol>${conTitulo.map((s) => `\n    <li><a href="#${s.id}">${bilingue(...s.titulo)}</a></li>`).join('')}
  </ol>
</nav>
`
    : ''
  const cuerpo = secciones.filter((s) => s.partes.length).map((s) => (s.id
    ? `<section id="${s.id}"><h2>${bilingue(...s.titulo)}</h2>\n${s.partes.join('\n\n')}\n</section>`
    : s.partes.join('\n\n'))).join('\n\n')
  const articulo = `${indice}<article class="doc tema-ayuda">
<nav class="migas" aria-label="Ruta" data-aria-en="Breadcrumb"><a href="index.html">${bilingue('Ayuda', 'Help')}</a> › ${tx(tema.titulo)} · <a href="index.html#buscar-ayuda">${bilingue('Buscar', 'Search')}</a></nav>
<header>
  <h1>${tx(tema.titulo)}</h1>
  <p class="entradilla">${tx(tema.resumen)}</p>
</header>

${cuerpo}
</article>`
  return pagina({ rel: '../', ruta: `ayuda/${tema.id}.html`, actual: 'ayuda' }, [f.es[tema.titulo] ?? tema.id, f.en[tema.titulo] ?? tema.id],
    f.es[tema.resumen] ?? '', articulo, indice ? 'ayuda' : 'ayuda-temas')
}

function paginaInicio(f: Fuente): string {
  const cuerpo = `<article class="doc tema-ayuda">
<header>
  <h1>${bilingue('Ayuda', 'Help')}</h1>
  <p class="entradilla">${bilingue('Qué hace Tramtai, cómo empezar, cada función y qué hacer si algo falla.', 'What Tramtai does, how to start, every feature and what to do if something fails.')}</p>
</header>
<div class="buscar-ayuda" hidden>
  <input type="search" id="buscar-ayuda" placeholder="Buscar en la ayuda" data-placeholder-en="Search help" aria-label="Buscar en la ayuda" data-aria-en="Search help">
  <p id="cuenta-ayuda" class="solo-lectores" aria-live="polite"></p>
  <ol id="resultados-ayuda"></ol>
</div>
${tarjetas(f, f.temas)}
<h2>${bilingue('Más documentación', 'More documentation')}</h2>
<ul>
  <li><a href="../desarrolladores.html">${bilingue('Guía del desarrollador', 'Developer guide')}</a></li>
  <li><a href="https://github.com/MscottB/Tramtai/blob/main/docs/inicio.md">${bilingue('Documentación completa en GitHub', 'Full documentation on GitHub')}</a></li>
</ul>
</article>`
  return pagina({ rel: '../', ruta: 'ayuda/', actual: 'ayuda' }, ['Ayuda', 'Help'],
    'Ayuda de Tramtai: instalar, conectar agentes y apps, archivos, JSON, recetas, herramientas y errores.', cuerpo)
}

// pagina404 la sirve GitHub Pages en cualquier ruta que no existe: por eso todo va con la dirección absoluta.
function pagina404(): string {
  const cuerpo = `<article class="doc tema-ayuda">
<header>
  <h1>${bilingue('Esta página no existe', 'This page does not exist')}</h1>
  <p class="entradilla">${bilingue('Puede que la dirección esté mal escrita o que la página se haya movido.', 'The address may be mistyped, or the page may have moved.')}</p>
</header>
<ul>
  <li><a href="${URL_SITIO}">${bilingue('La portada de Tramtai', 'Tramtai’s home page')}</a></li>
  <li><a href="${URL_SITIO}ayuda/">${bilingue('La Ayuda, con su buscador', 'The Help, with its search box')}</a></li>
  <li><a href="${URL_SITIO}#descargar">${bilingue('Descargar Tramtai', 'Download Tramtai')}</a></li>
</ul>
</article>`
  return pagina({ rel: URL_SITIO, ruta: null, actual: '' }, ['Página no encontrada', 'Page not found'], 'Esta página no existe.', cuerpo)
}

function sitemap(f: Fuente): string {
  const urls = ['', 'desarrolladores.html', 'ayuda/', ...f.temas.map((t) => `ayuda/${t.id}.html`)]
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls.map((u) => `  <url><loc>${URL_SITIO}${u}</loc></url>`).join('\n')}
</urlset>
`
}

// indiceBusqueda es lo que busca sitio.js: por idioma, una entrada por tema y ancla con su texto ya normalizado.
function indiceBusqueda(f: Fuente): string {
  const indice: Record<string, { t: string; a: string; ti: string; d: string; x: string }[]> = {}
  for (const idioma of ['es', 'en'] as const) {
    const textos = idioma === 'es' ? f.es : f.en
    const titulo = (c: string) => textos['error.' + c] ?? c
    indice[idioma] = f.temas.flatMap((tema) => {
      const porAncla = new Map<string, { d: string; x: string[] }>()
      for (const c of campos(tema, textos, titulo)) {
        const e = porAncla.get(c.ancla) ?? { d: c.donde, x: [] }
        e.x.push(normalizar(c.texto))
        porAncla.set(c.ancla, e)
      }
      return [...porAncla].map(([a, e]) => ({ t: tema.id, a, ti: textos[tema.titulo] ?? tema.id, d: e.d, x: e.x.join(' ') }))
    })
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
  archivos.set('ayuda/indice.json', indiceBusqueda(f))
  archivos.set('ayuda.html', paginaDePaso())
  archivos.set('404.html', pagina404())
  archivos.set('sitemap.xml', sitemap(f))
  return archivos
}
