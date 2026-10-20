// sitio arma la Ayuda del sitio (site/ayuda/ y la página de paso site/ayuda.html) desde los mismos temas que la app.
// Son funciones puras: sitio-ayuda.mjs les da los textos, los ejemplos y las capturas y escribe los archivos.
import type { Bloque, Tema } from '../src/ayuda/tipos'
import { bloquesVisibles } from '../src/ayuda/vista'
import { campos, normalizar } from '../src/ayuda/buscar'
import { ERRORES } from '../src/ayuda/temas/problemas'

type Textos = Record<string, string>
export interface Fuente {
  temas: Tema[]
  es: Textos
  en: Textos
  // ejemplos da el código de un bloque ejemplo en un idioma (el mensaje de ejemplo cambia con el idioma).
  ejemplos: (b: Extract<Bloque, { tipo: 'ejemplo' }>, idioma: 'es' | 'en') => { id: string; titulo: string; codigo: string }[]
  // capturas son los archivos de web/src/ayuda/capturas que existen.
  capturas: Set<string>
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

function pagina(f: Fuente, titulo: string, descripcion: string, cuerpo: string): string {
  return `<!doctype html>
<html lang="es">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${escapar(titulo)} · Tramtai</title>
<meta name="description" content="${escapar(descripcion)}">
<meta name="theme-color" content="#0b6e64" media="(prefers-color-scheme: light)">
<meta name="theme-color" content="#0a1116" media="(prefers-color-scheme: dark)">
<link rel="icon" href="../logo.png">
<link rel="stylesheet" href="../estilo.css">
<script>try { var t = localStorage.getItem('tramtai.tema'); if (t === 'claro' || t === 'oscuro') document.documentElement.dataset.tema = t } catch (e) {}</script>
</head>
<body>
<header class="barra">
  <div class="envoltura">
    <a class="marca" href="../index.html"><img src="../logo.png" alt="">Tramtai</a>
    <nav aria-label="Secciones">
      <a href="../index.html#como">${bilingue('Cómo funciona', 'How it works')}</a>
      <a href="../index.html#funciones">${bilingue('Funciones', 'Features')}</a>
      <a href="index.html" aria-current="page">${bilingue('Ayuda', 'Help')}</a>
      <a href="../desarrolladores.html">${bilingue('Desarrolladores', 'Developers')}</a>
    </nav>
    <a class="github" href="https://github.com/MscottB/Tramtai" aria-label="GitHub" title="GitHub">
      <svg viewBox="0 0 16 16" aria-hidden="true"><path fill="currentColor" d="M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.9.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z"/></svg>
    </a>
    <div class="idioma" role="group" aria-label="Idioma / Language">
      <button type="button" data-idioma="es" aria-pressed="true">ES</button>
      <button type="button" data-idioma="en" aria-pressed="false">EN</button>
    </div>
    <div class="idioma tema" role="group" aria-label="Tema / Theme">
      <button type="button" data-tema="sistema" aria-pressed="true" aria-label="Tema del sistema / System theme" title="Sistema / System">◐</button>
      <button type="button" data-tema="claro" aria-pressed="false" aria-label="Tema claro / Light theme" title="Claro / Light">☀</button>
      <button type="button" data-tema="oscuro" aria-pressed="false" aria-label="Tema oscuro / Dark theme" title="Oscuro / Dark">☾</button>
    </div>
    <a class="boton principal" href="../index.html#descargar">${bilingue('Descargar', 'Download')}</a>
  </div>
</header>

<main class="envoltura ayuda-temas">
${cuerpo}
</main>

<footer>
  <div class="envoltura">
    <span>© 2026 Michael Scott B. · Tramtai · <a href="https://github.com/MscottB/Tramtai/blob/main/TERMINOS.md">${bilingue('Términos de uso', 'Terms of use')}</a></span>
    <span>
      ${bilingue('Claude, Codex, GitHub Copilot, Gemini y Antigravity son marcas de sus respectivos dueños. Tramtai no está afiliado a ellos.',
        'Claude, Codex, GitHub Copilot, Gemini and Antigravity are trademarks of their respective owners. Tramtai is not affiliated with them.')}
    </span>
  </div>
</footer>

<script src="../sitio.js"></script>
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

function bloque(f: Fuente, tema: Tema, b: Bloque, n: number): string {
  const tx = (k: string) => bilingue(f.es[k] ?? k, f.en[k] ?? k)
  switch (b.tipo) {
    case 'diagrama':
      return `<ol class="diagrama" style="--paradas: ${b.paradas.length}">${b.paradas.map((p, i) => `
  <li><strong>${i + 1}. ${tx(p.titulo)}</strong>${p.nota ? `<span>${tx(p.nota)}</span>` : ''}</li>`).join('')}
</ol>${(b.ramas ?? []).map((r) => `\n<p class="diagrama-rama">↳ ${r.desde + 1} → ${r.hacia + 1}: ${tx(r.etiqueta)}</p>`).join('')}`
    case 'pasos':
      return `<ol class="pasos">${b.pasos.map((p) => `\n  <li>${tx(p)}</li>`).join('')}\n</ol>`
    case 'captura': {
      const imgs = (['claro', 'oscuro'] as const).flatMap((c) => (['es', 'en'] as const).map((i) => {
        const archivo = `${b.id}.${c}.${i}.webp`
        const alt = (i === 'es' ? f.es : f.en)[b.alt] ?? b.alt
        return f.capturas.has(archivo) ? `<img class="solo-${c}" lang="${i}" src="capturas/${archivo}" loading="lazy" alt="${escapar(alt)}">` : ''
      })).join('')
      return imgs ? `<figure class="captura">${imgs}</figure>` : ''
    }
    case 'ejemplo': {
      const porIdioma = { es: f.ejemplos(b, 'es'), en: f.ejemplos(b, 'en') }
      const id = (e: string) => `e${n}-${e}`
      const lista = porIdioma.es.map((e, i) => `<button type="button" role="tab" id="${id(e.id)}" aria-controls="${id(e.id)}-panel" aria-selected="${i === 0}" tabindex="${i === 0 ? 0 : -1}" data-lenguaje="${e.id}">${escapar(e.titulo)}</button>`).join('')
      const paneles = porIdioma.es.map((e, i) => `
  <div role="tabpanel" id="${id(e.id)}-panel" aria-labelledby="${id(e.id)}">
    <pre class="codigo" lang="es"><code>${escapar(e.codigo)}</code></pre>
    <pre class="codigo" lang="en"><code>${escapar(porIdioma.en[i]?.codigo ?? e.codigo)}</code></pre>
  </div>`).join('')
      return `<div class="pestanas" data-pestanas>\n  <div class="pestanas-lista" role="tablist" aria-label="Lenguaje / Language">${lista}</div>${paneles}\n</div>`
    }
    case 'tabla':
      return `<table>
  <thead><tr><th></th>${b.columnas.map((c) => `<th>${tx(c)}</th>`).join('')}</tr></thead>
  <tbody>${b.filas.map((fila) => `\n    <tr><th scope="row">${tx(fila.titulo)}</th>${fila.celdas.map((c) => `<td>${tx(c)}</td>`).join('')}</tr>`).join('')}
  </tbody>
</table>`
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
      return `<p><a class="boton principal" href="../index.html#descargar">${bilingue('Descargar Tramtai', 'Download Tramtai')}</a></p>`
    default:
      return ''
  }
}

function paginaTema(f: Fuente, tema: Tema): string {
  const tx = (k: string) => bilingue(f.es[k] ?? k, f.en[k] ?? k)
  const relacionados = tema.relacionados.map((r) => f.temas.find((t) => t.id === r)).filter((t): t is Tema => !!t)
  const bloques = bloquesVisibles(tema, 'sitio').map((b, i) => bloque(f, tema, b, i)).filter(Boolean).join('\n\n')
  const guia = GUIA.has(tema.id)
    ? `\n<p class="aviso"><a href="../desarrolladores.html">${bilingue('Guía del desarrollador', 'Developer guide')}</a>: ${bilingue('el ciclo completo en Python, JavaScript, Go y curl.', 'the full cycle in Python, JavaScript, Go and curl.')}</p>`
    : ''
  const cuerpo = `<nav class="migas" aria-label="Ruta / Breadcrumb"><a href="index.html">${bilingue('Ayuda', 'Help')}</a> › ${tx(tema.titulo)}</nav>
<article class="doc tema-ayuda">
<header>
  <h1>${tx(tema.titulo)}</h1>
  <p class="entradilla">${tx(tema.resumen)}</p>
</header>

${bloques}${guia}
${relacionados.length ? `\n<h2>${bilingue('Temas relacionados', 'Related topics')}</h2>\n${tarjetas(f, relacionados)}` : ''}
</article>`
  return pagina(f, f.es[tema.titulo] ?? tema.id, f.es[tema.resumen] ?? '', cuerpo)
}

function paginaInicio(f: Fuente): string {
  const cuerpo = `<article class="doc tema-ayuda">
<header>
  <h1>${bilingue('Ayuda', 'Help')}</h1>
  <p class="entradilla">${bilingue('Qué hace Tramtai, cómo empezar, cada función y qué hacer si algo falla.', 'What Tramtai does, how to start, every feature and what to do if something fails.')}</p>
</header>
<div class="buscar-ayuda" hidden>
  <input type="search" id="buscar-ayuda" placeholder="Buscar en la ayuda / Search help" aria-label="Buscar en la ayuda / Search help">
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
  return pagina(f, 'Ayuda', 'Ayuda de Tramtai: instalar, conectar agentes y apps, archivos, JSON, recetas, herramientas y errores.', cuerpo)
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
  return archivos
}
