import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
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

test('la portada guardada no lleva una versión escrita a mano ni consulta la API de GitHub', () => {
  const portada = readFileSync(new URL('../../site/index.html', import.meta.url), 'utf8')
  assert.match(portada, /<span data-version><\/span>/)
  assert.match(portada, /<script type="application\/json" id="descargas">\{\}<\/script>/)
  assert.doesNotMatch(readFileSync(new URL('../../site/sitio.js', import.meta.url), 'utf8'), /api\.github\.com/)
})
