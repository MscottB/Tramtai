// Las anclas de los recorridos guiados: cada paso de la Ayuda señala un data-recorrido que existe en la página, y cada
// data-recorrido de la interfaz tiene su paso. Sin esto, un cambio en una página dejaría un recorrido señalando el aire.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { temas } from './indice.ts'
import { PAGINAS } from './tipos.ts'
import type { Pagina } from './tipos.ts'
import { pasosBienvenida } from '../recorrido.ts'

// ARCHIVO es el .vue donde vive el contenido de cada página de la app.
const ARCHIVO: Record<Pagina, string> = {
  inicio: 'paginas/Panel.vue', agentes: 'paginas/Agentes.vue', ejecutar: 'paginas/Ejecutar.vue', conectar: 'paginas/Accesos.vue',
  historial: 'paginas/Trabajos.vue', espacios: 'paginas/Espacios.vue', configuracion: 'paginas/Configuracion.vue',
}
// COMUNES son los archivos de las anclas `app.*`, las de la cabecera que está en todas las páginas.
const COMUNES = ['App.vue', 'componentes/CabeceraPagina.vue']

const leer = (ruta: string) => readFileSync(new URL(`../${ruta}`, import.meta.url), 'utf8')
const marcadas = (texto: string) => [...texto.matchAll(/data-recorrido="([^"]+)"/g)].map((m) => m[1])
const pasosDe = (pagina: Pagina) => temas.filter((t) => t.paginaApp === pagina).flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'recorrido' ? b.pasos : [])))

test('cada ancla es app.x o <página>.x, en minúsculas y con guiones', () => {
  for (const pagina of PAGINAS)
    for (const { ancla } of pasosDe(pagina)) assert.match(ancla, new RegExp(`^(app|${pagina})\\.[a-z-]+$`), `${pagina}: ancla «${ancla}» mal formada`)
})

test('cada ancla de un paso está como data-recorrido en el .vue de su página, o en la cabecera si es app.*', () => {
  const comunes = COMUNES.flatMap((r) => marcadas(leer(r)))
  for (const pagina of PAGINAS) {
    const propias = marcadas(leer(ARCHIVO[pagina]))
    for (const { ancla } of pasosDe(pagina))
      assert.ok((ancla.startsWith('app.') ? comunes : propias).includes(ancla), `${pagina}: falta data-recorrido="${ancla}"`)
  }
})

test('ningún data-recorrido de la interfaz queda sin paso', () => {
  const conPaso = new Set(PAGINAS.flatMap((p) => pasosDe(p).map((s) => s.ancla)))
  for (const ruta of [...Object.values(ARCHIVO), ...COMUNES])
    for (const ancla of marcadas(leer(ruta))) assert.ok(conPaso.has(ancla), `${ruta}: data-recorrido="${ancla}" no tiene paso`)
})

test('las siete páginas tienen un recorrido de 3 a 5 pasos', () => {
  for (const pagina of PAGINAS) {
    const n = pasosDe(pagina).length
    assert.ok(n >= 3 && n <= 5, `${pagina} tiene ${n} pasos`)
  }
})

test('la bienvenida son 14 pasos: las 13 anclas en su orden y el cierre sin ancla', () => {
  const pasos = pasosBienvenida()
  assert.deepEqual(pasos.map((p) => p.ancla), [
    'inicio.mapa', 'inicio.primeros-pasos', 'app.espacio', 'app.ayuda', 'agentes.tarjeta', 'agentes.conectar', 'ejecutar.receta',
    'ejecutar.mensaje', 'ejecutar.codigo', 'conectar.clave-espacio', 'conectar.nueva', 'espacios.lista', 'espacios.abrir', undefined,
  ])
})
