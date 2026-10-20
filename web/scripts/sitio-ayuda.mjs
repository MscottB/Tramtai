// sitio-ayuda escribe la Ayuda del sitio (site/ayuda/ y site/ayuda.html) desde los temas de la app:
// cd web; npm run sitio. El CI lo vuelve a correr y falla si lo guardado en el repo no coincide.
import { copyFileSync, existsSync, mkdirSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { idioma, t } from '../src/i18n.ts'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { ejemplos } from '../src/ejemplos.ts'
import { generar } from './sitio.ts'

const sitio = new URL('../../site/', import.meta.url)
const origen = new URL('../src/ayuda/capturas/', import.meta.url)
const capturas = existsSync(origen) ? readdirSync(origen).filter((a) => a.endsWith('.webp')) : []

function ejemplosDe(b, lengua) {
  idioma.value = lengua
  return ejemplos({ url: 'http://127.0.0.1:17373', modelo: 'perfil/rapido', contratos: b.contratos,
    prueba: { archivos: b.archivos, solicitud: { mensajes: [{ rol: 'usuario', contenido: t('ejemplos.mensaje') }] } } })
    .map((e) => ({ id: e.id, titulo: e.titulo(), codigo: e.codigo(t('codigo.tuClave')) }))
}

const archivos = generar({ temas, es, en, ejemplos: ejemplosDe, capturas: new Set(capturas) })
// Se borra lo generado antes, para que un tema o una captura que ya no existen no queden publicados.
rmSync(new URL('ayuda/', sitio), { recursive: true, force: true })
mkdirSync(new URL('ayuda/capturas/', sitio), { recursive: true })
for (const [ruta, contenido] of archivos) writeFileSync(new URL(ruta, sitio), contenido)
for (const a of capturas) copyFileSync(new URL(a, origen), new URL('ayuda/capturas/' + a, sitio))
console.log(`${archivos.size} archivos y ${capturas.length} capturas en site/`)
