// terminal-datos escribe en internal/tui/datos los datos que la terminal en Go incrusta (la Ayuda, las plantillas, los
// nombres del menú y los ejemplos de «Ver como código»), desde las mismas fuentes que la Web UI: cd web; npm run ayuda-terminal
// (npm run sitio también lo corre). El CI lo vuelve a correr y falla si lo guardado en el repo no coincide.
import { mkdirSync, writeFileSync } from 'node:fs'
import { idioma } from '../src/i18n.ts'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { plantillas } from '../src/plantillas.ts'
import { ejemplos } from '../src/ejemplos.ts'
import { generar } from './terminal-datos.ts'

const destino = new URL('../../internal/tui/datos/', import.meta.url)

function ejemplosDe(e) {
  idioma.value = 'es'
  return ejemplos({ url: e.url, modelo: e.modelo, contratos: ['nativa'], prueba: { archivos: [], solicitud: e.solicitud } })
    .map((x) => ({ id: x.id, codigo: x.codigo(e.clave) }))
}

const archivos = generar({ temas, plantillas, es, en, ejemplos: ejemplosDe })
mkdirSync(new URL('testdata/', destino), { recursive: true })
for (const [ruta, contenido] of archivos) writeFileSync(new URL(ruta, destino), contenido)
console.log(`${archivos.size} archivos en internal/tui/datos/`)
