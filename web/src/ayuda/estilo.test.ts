// La regla del estilo de la Ayuda hecha prueba: textos cortos, diagrama donde hay un recorrido, un tema por página
// de la app y cada clave escrita en los dos idiomas.
import { test } from 'node:test'
import { CODIGOS_ERROR } from './temas/problemas.ts'
import assert from 'node:assert/strict'
import es from '../idiomas/es.ts'
import en from '../idiomas/en.ts'
import ayudaEs from '../idiomas/ayuda.es.ts'
import { CLAVES_UI, temas, temaDe, temaPorId } from './indice.ts'
import { PAGINAS } from './tipos.ts'
import type { Bloque } from './tipos.ts'

const idiomas: Record<string, Record<string, string>> = { es, en }
const problemas = await import('./temas/problemas.ts').catch(() => null)

// cortas son las claves de un bloque que deben caber en 200 caracteres.
function cortas(b: Bloque): string[] {
  switch (b.tipo) {
    case 'diagrama':
      return [...b.paradas.flatMap((p) => [p.titulo, ...(p.nota ? [p.nota] : [])]), ...(b.ramas ?? []).map((r) => r.etiqueta)]
    case 'pasos':
      return b.pasos
    case 'captura':
      return [b.alt]
    case 'tabla':
      return [...b.columnas, ...b.filas.flatMap((f) => [f.titulo, ...f.celdas])]
    case 'preguntas':
      return b.preguntas.flatMap((p) => [p.pregunta, p.respuesta])
    case 'nota':
      return [b.texto]
    case 'recorrido':
      return b.pasos.flatMap((p) => [p.titulo, p.texto])
    default:
      return []
  }
}

test('cada tema tiene un resumen de una línea en los dos idiomas', () => {
  for (const tema of temas)
    for (const [idioma, textos] of Object.entries(idiomas)) {
      for (const clave of [tema.titulo, tema.resumen]) assert.ok(textos[clave], `${tema.id}: falta ${clave} en ${idioma}`)
      assert.ok(textos[tema.resumen].length <= 140, `${tema.id}: el resumen en ${idioma} pasa de 140 caracteres`)
    }
})

test('pasos, notas, paradas y respuestas caben en 200 caracteres en los dos idiomas', () => {
  for (const tema of temas)
    for (const bloque of tema.bloques)
      for (const clave of cortas(bloque))
        for (const [idioma, textos] of Object.entries(idiomas)) {
          assert.ok(textos[clave], `${tema.id}: falta ${clave} en ${idioma}`)
          assert.ok(textos[clave].length <= 200, `${tema.id}: ${clave} en ${idioma} tiene ${textos[clave].length} caracteres`)
        }
})

test('los títulos de los pasos de un recorrido caben en 60 caracteres en los dos idiomas', () => {
  const titulos = [
    ...temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'recorrido' ? b.pasos.map((p) => p.titulo) : []))),
    'recorrido.finTitulo' as const,
  ]
  for (const clave of titulos)
    for (const [idioma, textos] of Object.entries(idiomas))
      assert.ok(textos[clave] && textos[clave].length <= 60, `${clave} en ${idioma} tiene ${textos[clave]?.length} caracteres`)
})

test('el texto del paso de cierre de la bienvenida cabe en 200 caracteres en los dos idiomas', () => {
  for (const [idioma, textos] of Object.entries(idiomas)) assert.ok(textos['recorrido.finTexto'].length <= 200, `recorrido.finTexto en ${idioma}`)
})

test('un tema que explica un recorrido tiene su diagrama', () => {
  for (const tema of temas.filter((t) => t.conDiagrama)) assert.ok(tema.bloques.some((b) => b.tipo === 'diagrama'), tema.id)
})

test('cada página principal de la app tiene su tema', () => {
  for (const pagina of PAGINAS) assert.ok(temaDe(pagina), `la página ${pagina} no tiene tema`)
})

test('relacionados, errores e ids son válidos', () => {
  const ids = temas.map((t) => t.id)
  assert.equal(new Set(ids).size, ids.length, 'ids de tema repetidos')
  for (const tema of temas) {
    assert.match(tema.id, /^[a-z]+$/)
    for (const b of tema.bloques)
      if (b.tipo === 'errores' && problemas)
        for (const c of b.codigos) assert.ok(problemas.CODIGOS_ERROR.includes(c), `${tema.id}: error ${c} desconocido`)
  }
})

test('están los 16 temas y sus relacionados existen', () => {
  assert.equal(temas.length, 16)
  for (const tema of temas) for (const r of tema.relacionados) assert.ok(temaPorId(r), `${tema.id}: relacionado ${r} no existe`)
})

test('cada pregunta tiene un ancla única en toda la Ayuda y su tema existe', () => {
  const anclas = temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'preguntas' ? b.preguntas : [])))
  const ids = anclas.map((p) => p.id)
  assert.equal(new Set(ids).size, ids.length, `anclas repetidas: ${ids.filter((x, i) => ids.indexOf(x) !== i)}`)
  for (const p of anclas) {
    assert.match(p.id, /^[a-z][a-z0-9-]*$/)
    assert.ok(!p.id.startsWith('e-'), `${p.id}: e- es de los errores`)
    if (p.tema) assert.ok(temaPorId(p.tema), `${p.id}: el tema ${p.tema} no existe`)
  }
})

test('ningún texto de la Ayuda queda sin usar', () => {
  const usadas = new Set<string>(CLAVES_UI)
  for (const tema of temas) {
    usadas.add(tema.titulo)
    usadas.add(tema.resumen)
    for (const b of tema.bloques) for (const c of cortas(b)) usadas.add(c)
  }
  for (const e of Object.values(problemas?.ERRORES ?? {})) {
    usadas.add(e.solucion)
    if (e.titulo) usadas.add(e.titulo)
  }
  assert.deepEqual(Object.keys(ayudaEs).filter((k) => !usadas.has(k)), [])
})

test('no se pierde lo que decía la ayuda vieja (rutas, pendrive, claves, créditos)', () => {
  const anclas = new Set(temas.flatMap((t) => t.bloques.flatMap((b) => (b.tipo === 'preguntas' ? b.preguntas.map((p) => p.id) : []))))
  for (const id of ['pendrive', 'datos-ruta', 'portable-datos', 'copilot-creditos', 'gemini-clave', 'linux-claves'])
    assert.ok(anclas.has(id), `falta la pregunta ${id}`)
})

test('los errores más comunes del servicio tienen su solución', () => {
  for (const c of ['error_agente', 'tiempo_cola_agotado', 'modelo_no_permitido', 'modelo_no_configurado', 'error_protocolo_agente', 'cuota_cargas_agotada'])
    assert.ok(problemas?.CODIGOS_ERROR.includes(c), `falta ${c}`)
})

test('cada error tiene título y una solución corta en los dos idiomas', () => {
  assert.ok(problemas, 'falta temas/problemas.ts')
  for (const [codigo, e] of Object.entries(problemas.ERRORES))
    for (const [idioma, textos] of Object.entries(idiomas)) {
      if (e.titulo) assert.ok(textos[e.titulo], `${codigo}: falta el título en ${idioma}`)
      else assert.ok(textos['error.' + codigo], `${codigo}: no hay error.${codigo} en ${idioma}`)
      assert.ok(textos[e.solucion] && textos[e.solucion].length <= 200, `${codigo}: solución en ${idioma}`)
    }
})

test('los errores de espacios y agentes tienen su respuesta', () => {
  for (const codigo of ['agente_desconectado', 'confianza_requerida', 'espacio_detenido']) assert.ok(CODIGOS_ERROR.includes(codigo), codigo)
})
