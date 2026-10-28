// descargas escribe en la portada publicada la versión y el enlace de cada archivo del último release, para que ninguna
// visita tenga que preguntar a la API de GitHub. Lo corre .github/workflows/paginas.yml al publicar el sitio:
//   gh release view --json tagName,assets > release.json
//   node web/scripts/descargas.mjs release.json [site/index.html]
// En el repositorio la portada lleva <span data-version></span> y #descargas vacío; sin release se queda así.
import { readFileSync, writeFileSync } from 'node:fs'
import { pathToFileURL } from 'node:url'

const DATOS = /<script type="application\/json" id="descargas">[^<]*<\/script>/

// conDescargas devuelve el HTML con la versión (sin la fecha de la etiqueta) en [data-version] y, en #descargas, la URL
// de cada archivo que piden data-archivo o data-arm: el que se llama Tramtai-<etiqueta>-<archivo> (sin distinguir
// mayúsculas). Por la terminación sola no basta: Tramtai-<etiqueta>-servidor-linux-amd64.deb también acaba igual.
export function conDescargas(html, release) {
  if (!release || !release.tagName) return html
  const version = String(release.tagName).split('+')[0]
  const pedidos = new Set([...html.matchAll(/data-(?:archivo|arm)="([^"]+)"/g)].map((m) => m[1]))
  const archivos = {}
  const prefijo = `tramtai-${release.tagName}-`.toLowerCase()
  for (const f of release.assets ?? [])
    for (const p of pedidos) if (f.name.toLowerCase() === prefijo + p.toLowerCase()) archivos[p] = f.url
  // < escapado: un nombre con </script> no puede cerrar el bloque.
  const json = JSON.stringify({ version, archivos }).replace(/</g, '\\u003c')
  const texto = version.replace(/&/g, '&amp;').replace(/</g, '&lt;')
  return html
    .replace(/<span data-version>[^<]*<\/span>/g, `<span data-version> · ${texto}</span>`)
    .replace(DATOS, `<script type="application/json" id="descargas">${json}</script>`)
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) {
  const [release, portada = new URL('../../site/index.html', import.meta.url)] = process.argv.slice(2)
  writeFileSync(portada, conDescargas(readFileSync(portada, 'utf8'), JSON.parse(readFileSync(release, 'utf8'))))
}
