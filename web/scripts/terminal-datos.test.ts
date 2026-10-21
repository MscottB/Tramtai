// node --test web/scripts/*.test.ts: los datos que la terminal en Go incrusta salen de las fuentes de la Web UI.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import es from '../src/idiomas/es.ts'
import en from '../src/idiomas/en.ts'
import { temas } from '../src/ayuda/indice.ts'
import { plantillas } from '../src/plantillas.ts'
import { aEsquema } from '../src/esquemaCampos.ts'
import { ayudaDe, generar, textosDe, plantillasDe, URL_AYUDA, type Tema } from './terminal-datos.ts'

const fuente = { temas, plantillas, es, en, ejemplos: () => [{ id: 'curl-nativa', codigo: 'a' }, { id: 'python-nativa', codigo: 'b' },
  { id: 'node-nativa', codigo: 'c' }, { id: 'go-nativa', codigo: 'd' }, { id: 'python', codigo: 'e' }] }
const ayuda = { es: ayudaDe(temas, es), en: ayudaDe(temas, en) }

// cadenas recorre todas las cadenas de un valor JSON.
function* cadenas(v: unknown): Generator<string> {
  if (typeof v === 'string') yield v
  else if (Array.isArray(v)) for (const x of v) yield* cadenas(x)
  else if (v && typeof v === 'object') for (const x of Object.values(v)) yield* cadenas(x)
}

test('los 16 temas están en es y en con los mismos bloques', () => {
  assert.equal(ayuda.es.length, 16)
  assert.deepEqual(ayuda.en.map((t) => t.id), ayuda.es.map((t) => t.id))
  ayuda.es.forEach((t, i) => assert.deepEqual(ayuda.en[i].bloques.map((b) => b.tipo), t.bloques.map((b) => b.tipo), t.id))
})

test('la Ayuda es texto plano: ninguna cadena lleva < ni **', () => {
  for (const idioma of ['es', 'en'] as const) for (const s of cadenas(ayuda[idioma])) assert.doesNotMatch(s, /<|\*\*/, s)
})

test('el tema de Ejecutar trae su recorrido como bloque pantalla', () => {
  const tema = temas.find((t) => t.paginaApp === 'ejecutar')!
  const recorrido = tema.bloques.find((b) => b.tipo === 'recorrido')!
  for (const [lengua, textos] of [['es', es], ['en', en]] as const) {
    const t = ayuda[lengua].find((x) => x.id === tema.id) as Tema
    assert.equal(t.pagina, 'ejecutar')
    const pantalla = t.bloques.find((b) => b.tipo === 'pantalla')!
    assert.deepEqual(pantalla.pares, recorrido.pasos.map((p) => [textos[p.titulo], textos[p.texto]]))
  }
})

test('capturas, diagramas, descargas y enlaces ir pasan a un bloque web con la URL del tema', () => {
  const tema = temas.find((t) => t.bloques.some((b) => b.tipo === 'captura'))!
  const web = ayuda.es.find((t) => t.id === tema.id)!.bloques.filter((b) => b.tipo === 'web')
  assert.ok(web.length > 0)
  for (const b of web) assert.equal(b.url, `${URL_AYUDA}${tema.id}.html`)
  assert.equal(URL_AYUDA, 'https://mscottb.github.io/Tramtai/ayuda/')
})

test('un bloque que el generador no conoce lo hace fallar en lugar de salir como null', () => {
  const nuevo = { ...temas[0], bloques: [{ tipo: 'nuevo' }] } as unknown as typeof temas[number]
  assert.throws(() => ayudaDe([nuevo], es), /tipo de bloque sin mapear: nuevo/)
})

test('los errores llevan su título y qué hacer; las preguntas, pregunta y respuesta', () => {
  const problemas = ayuda.es.find((t) => t.id === 'problemas')!
  const errores = problemas.bloques.find((b) => b.tipo === 'errores')!
  assert.ok(errores.pares!.length > 10)
  assert.ok(errores.pares!.every(([titulo, solucion]) => titulo && solucion))
  const preguntas = ayuda.es.find((t) => t.id === 'preguntas')!.bloques.find((b) => b.tipo === 'preguntas')!
  assert.ok(preguntas.pares!.every(([p, r]) => p && r))
})

test('textos trae todas las menu.* y la Ayuda de la interfaz', () => {
  const textos = textosDe({ es, en })
  for (const [lengua, fuenteTextos] of [['es', es], ['en', en]] as const) {
    const menu = Object.keys(fuenteTextos).filter((k) => k.startsWith('menu.'))
    assert.ok(menu.length >= 11)
    for (const k of menu) assert.equal(textos[lengua][k], (fuenteTextos as Record<string, string>)[k], k)
    assert.ok(textos[lengua]['ayuda.buscar'])
  }
  assert.equal(textos.es['menu.inicio'], 'Inicio')
  assert.equal(textos.es['menu.grupo.empezar'], 'Empezar')
})

test('las plantillas salen con su nombre, instrucciones y el esquema de esquemaCampos', () => {
  const lista = plantillasDe(plantillas, { es, en })
  assert.equal(lista.es.length, 7)
  assert.equal(lista.en.length, 7)
  lista.en.forEach((p, i) => {
    assert.equal(p.id, plantillas[i].id)
    assert.equal(p.nombre, (en as Record<string, string>)[plantillas[i].nombre])
    assert.equal(p.instrucciones, plantillas[i].instrucciones('en'))
    assert.deepEqual(p.esquema, aEsquema(plantillas[i].campos('en')))
  })
})

test('generar escribe los cuatro archivos con LF y los ejemplos curl, python y node', () => {
  const archivos = generar(fuente)
  assert.deepEqual([...archivos.keys()], ['ayuda.json', 'plantillas.json', 'textos.json', 'testdata/ejemplos.json'])
  for (const texto of archivos.values()) assert.ok(texto.endsWith('}\n') && !texto.includes('\r'))
  const ejemplos = JSON.parse(archivos.get('testdata/ejemplos.json')!)
  assert.deepEqual(ejemplos.ejemplos, [{ id: 'curl', codigo: 'a' }, { id: 'python', codigo: 'b' }, { id: 'node', codigo: 'c' }])
  assert.deepEqual(Object.keys(ejemplos.entrada), ['url', 'clave', 'modelo', 'solicitud'])
  assert.equal(generar(fuente).get('ayuda.json'), archivos.get('ayuda.json'), 'misma salida cada vez')
})
