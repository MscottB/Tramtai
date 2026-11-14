import { test } from 'node:test'
import assert from 'node:assert/strict'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, readFileSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { fileURLToPath } from 'node:url'
import { join } from 'node:path'
import { conDescargas } from './descargas.mjs'

const release = {
  tagName: 'v0.56.0+22102026',
  assets: [
    { name: 'Tramtai-v0.56.0+22102026-instalador-windows-x64.exe', url: 'https://ejemplo.test/instalador.exe' },
    { name: 'tramtai-v0.56.0+22102026-linux-amd64.deb', url: 'https://ejemplo.test/amd64.deb' },
    { name: 'tramtai-v0.56.0+22102026-linux-arm64.deb', url: 'https://ejemplo.test/arm64.deb' },
    // El paquete del servidor termina igual y va después por orden alfabético: el botón sigue siendo el de escritorio.
    { name: 'Tramtai-v0.56.0+22102026-servidor-linux-amd64.deb', url: 'https://ejemplo.test/servidor-amd64.deb' },
    { name: 'SHA256SUMS.txt', url: 'https://ejemplo.test/sumas' },
  ],
}
const html = `<a data-archivo="instalador-windows-x64.exe" href="x">i</a>
<a data-archivo="linux-amd64.deb" data-arm="linux-arm64.deb" href="x">d</a>
<a data-archivo="macos.dmg" href="x">m</a>
<span data-version></span>
<script type="application/json" id="descargas">{}</script>`

test('la publicación escribe la versión y el archivo de cada botón, también el de ARM', () => {
  const salida = conDescargas(html, release)
  assert.match(salida, /<span data-version> · v0\.56\.0<\/span>/)
  const datos = JSON.parse(salida.match(/<script type="application\/json" id="descargas">([^<]*)<\/script>/)![1])
  assert.equal(datos.version, 'v0.56.0')
  assert.deepEqual(datos.archivos, {
    'instalador-windows-x64.exe': 'https://ejemplo.test/instalador.exe',
    'linux-amd64.deb': 'https://ejemplo.test/amd64.deb',
    'linux-arm64.deb': 'https://ejemplo.test/arm64.deb',
  })
})

test('sin release no cambia nada, y una segunda pasada deja lo mismo', () => {
  assert.equal(conDescargas(html, null), html)
  const una = conDescargas(html, release)
  assert.equal(conDescargas(una, release), una)
})

test('un nombre de archivo raro no rompe el JSON ni el HTML', () => {
  const salida = conDescargas(html, { tagName: 'v1</script>', assets: [{ name: 'x-macos.dmg', url: 'https://ejemplo.test/a</script>' }] })
  assert.equal((salida.match(/<\/script>/g) ?? []).length, 1)
})

test('las páginas guardadas no llevan una versión escrita a mano ni consultan la API de GitHub', () => {
  for (const pagina of ['index.html', 'descargar.html']) {
    const html = readFileSync(new URL(`../../site/${pagina}`, import.meta.url), 'utf8')
    assert.match(html, /<span data-version><\/span>/, pagina)
    assert.match(html, /<script type="application\/json" id="descargas">\{\}<\/script>/, pagina)
  }
  assert.doesNotMatch(readFileSync(new URL('../../site/sitio.js', import.meta.url), 'utf8'), /api\.github\.com/)
})

const sitio = (p: string) => readFileSync(new URL(`../../site/${p}`, import.meta.url), 'utf8')

test('descargas.mjs rellena varias páginas', () => {
  const dir = mkdtempSync(join(tmpdir(), 'descargas-'))
  const datos = join(dir, 'release.json')
  writeFileSync(datos, JSON.stringify(release))
  const paginas = ['index.html', 'descargar.html'].map((p) => {
    const ruta = join(dir, p)
    writeFileSync(ruta, sitio(p))
    return ruta
  })
  execFileSync(process.execPath, [fileURLToPath(new URL('./descargas.mjs', import.meta.url)), datos, ...paginas])
  for (const ruta of paginas) {
    const salida = readFileSync(ruta, 'utf8')
    assert.match(salida, /<span data-version> · v0\.56\.0<\/span>/, ruta)
    const json = JSON.parse(salida.match(/<script type="application\/json" id="descargas">([^<]*)<\/script>/)![1])
    assert.equal(json.archivos['instalador-windows-x64.exe'], 'https://ejemplo.test/instalador.exe', ruta)
    // Inicio solo pide los archivos de su botón; Descargar los pide todos.
    if (ruta.endsWith('descargar.html')) assert.equal(json.archivos['linux-arm64.deb'], 'https://ejemplo.test/arm64.deb', ruta)
  }
})

test('Descargar tiene cada archivo del release', () => {
  const archivos = new Set([...sitio('descargar.html').matchAll(/data-(?:archivo|arm)="([^"]+)"/g)].map((m) => m[1]))
  for (const f of ['instalador-windows-x64.exe', 'portable-windows-x64.zip', 'conversor-windows-x64.zip', 'macos.dmg', 'macos-amd64.tar.gz', 'macos-arm64.tar.gz'])
    assert.ok(archivos.has(f), f)
  for (const arq of ['amd64', 'arm64'])
    for (const f of [`linux-${arq}.deb`, `linux-${arq}.rpm`, `linux-${arq}.tar.gz`, `servidor-linux-${arq}.deb`, `servidor-linux-${arq}.rpm`]) assert.ok(archivos.has(f), f)
  // Tramtai no está en el catálogo de winget: no se ofrece.
  assert.doesNotMatch(sitio('descargar.html'), /winget/)
})

test('pestanaDe: Linux en cualquier arquitectura, y ninguna en móvil, tableta o ChromeOS', () => {
  const js = sitio('sitio.js')
  const f = (n: string) => js.match(new RegExp(`\\nfunction ${n}\\([\\s\\S]*?\\n\\}`))![0]
  const pestanaDe = new Function(`${f('sistemaDe')}\n${f('pestanaDe')}\nreturn pestanaDe`)()
  const casos: [string, string, string | null][] = [
    ['Mozilla/5.0 (X11; Linux aarch64; rv:128.0) Gecko/20100101 Firefox/128.0', 'Linux aarch64', 'linux'],
    ['Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/126 Safari/537.36', 'Linux x86_64', 'linux'],
    ['Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/126 Mobile Safari/537.36', 'Linux armv81', null],
    ['Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 Chrome/126 Safari/537.36', 'Linux x86_64', null],
    ['Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1', 'iPad', null],
    ['Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126 Safari/537.36', 'Win32', 'windows'],
  ]
  for (const [ua, plataforma, esperado] of casos) assert.equal(pestanaDe(ua, plataforma), esperado, ua)
  // El iPad con Safari de escritorio se presenta como un Mac: sitio.js lo anula con maxTouchPoints.
  assert.match(js, /if \(tuya === 'macos' && navigator\.maxTouchPoints > 1\) tuya = null;/)
})

test('las pestañas de Descargar', () => {
  const grupo = sitio('descargar.html').match(/<div class="[^"]*" data-pestanas="descargar">([\s\S]*?)<script type="application\/json" id="descargas">/)?.[1] ?? ''
  assert.match(grupo, /<div class="pestanas-lista" role="tablist"[^>]* hidden>/, 'sin JavaScript se ven las tres, una debajo de otra')
  const pestanas = [...grupo.matchAll(/<button type="button" role="tab" id="([^"]+)" aria-controls="([^"]+)" aria-selected="(true|false)" tabindex="(0|-1)" data-lenguaje="([^"]+)">/g)]
  assert.deepEqual(pestanas.map((p) => p[5]), ['windows', 'macos', 'linux'])
  assert.deepEqual(pestanas.map((p) => p[3]), ['true', 'false', 'false'])
  for (const [, id, panel, , , sistema] of pestanas)
    assert.match(grupo, new RegExp(`<article class="tarjeta" id="${panel}" aria-labelledby="${id}" tabindex="0" data-sistema="${sistema}">`))
  // Linux enseña a la vez .deb y .rpm, sin elegir por la persona.
  const linux = grupo.slice(grupo.indexOf('data-sistema="linux"'))
  assert.match(linux, /data-archivo="linux-amd64\.deb"/)
  assert.match(linux, /data-archivo="linux-amd64\.rpm"/)
  // El sistema detectado abre su pestaña (sitio.js) y no se recuerda: manda siempre el detectado.
  const js = sitio('sitio.js')
  assert.match(js, /\[data-pestanas="descargar"\] \[role="tab"\]\[data-lenguaje="' \+ tuya/)
  assert.match(js, /nombre !== 'formas' && nombre !== 'descargar'/)
})
