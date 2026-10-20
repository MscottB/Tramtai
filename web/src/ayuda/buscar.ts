// buscar encuentra en todos los temas de la Ayuda: títulos, pasos, notas, preguntas y errores. Es la misma búsqueda
// en la app y en el sitio, sin expresiones regulares (el texto del usuario nunca se interpreta).
import type { Tema } from './tipos'
import { ERRORES } from './temas/problemas'

export interface Resultado {
  tema: string
  // ancla es el id de la pregunta o e-<código> del error; vacía para el tema entero.
  ancla: string
  titulo: string
  donde: string
  puntos: number
}

const MAXIMO = 30

// normalizar deja minúsculas sin tildes y con un espacio entre palabras; el guion bajo de los códigos cuenta como
// espacio, así «cola llena» encuentra cola_llena.
export function normalizar(s: string): string {
  return s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/[\s_]+/g, ' ').trim()
}

// campos son los textos donde se busca, cada uno con su ancla, lo que se muestra y cuánto pesa; el sitio arma con
// ellos su índice de búsqueda.
export function campos(tema: Tema, textos: Record<string, string>, tituloError: (c: string) => string) {
  const t = (k: string) => textos[k] ?? ''
  const titulo = t(tema.titulo)
  const lista: { ancla: string; donde: string; texto: string; peso: number }[] = [
    { ancla: '', donde: titulo, texto: titulo, peso: 3 },
    { ancla: '', donde: titulo, texto: t(tema.resumen), peso: 1 },
  ]
  for (const b of tema.bloques) {
    if (b.tipo === 'pasos') for (const p of b.pasos) lista.push({ ancla: '', donde: titulo, texto: t(p), peso: 1 })
    if (b.tipo === 'nota') lista.push({ ancla: '', donde: titulo, texto: t(b.texto), peso: 1 })
    if (b.tipo === 'diagrama')
      for (const p of b.paradas) lista.push({ ancla: '', donde: titulo, texto: `${t(p.titulo)} ${p.nota ? t(p.nota) : ''}`, peso: 1 })
    if (b.tipo === 'preguntas')
      for (const p of b.preguntas) {
        lista.push({ ancla: p.id, donde: t(p.pregunta), texto: t(p.pregunta), peso: 2 })
        lista.push({ ancla: p.id, donde: t(p.pregunta), texto: t(p.respuesta), peso: 1 })
      }
    if (b.tipo === 'errores')
      for (const c of b.codigos) {
        const e = ERRORES[c]
        const nombre = e?.titulo ? t(e.titulo) : tituloError(c)
        lista.push({ ancla: 'e-' + c, donde: nombre, texto: `${nombre} ${c}`, peso: 2 })
        if (e) lista.push({ ancla: 'e-' + c, donde: nombre, texto: t(e.solucion), peso: 1 })
      }
  }
  return lista
}

export function buscar(consulta: string, temas: Tema[], textos: Record<string, string>, tituloError: (c: string) => string): Resultado[] {
  // Coincide un texto que tenga todas las palabras buscadas, en cualquier orden.
  const palabras = normalizar(consulta).split(' ').filter(Boolean)
  if (!palabras.length) return []
  // mejores guarda, por tema y ancla, el campo que más pesa de los que coinciden.
  const mejores = new Map<string, Resultado & { orden: number }>()
  temas.forEach((tema, orden) => {
    for (const c of campos(tema, textos, tituloError)) {
      const texto = normalizar(c.texto)
      if (!palabras.every((p) => texto.includes(p))) continue
      const clave = tema.id + '#' + c.ancla
      const previo = mejores.get(clave)
      if (!previo || c.peso > previo.puntos)
        mejores.set(clave, { tema: tema.id, ancla: c.ancla, titulo: textos[tema.titulo] ?? tema.id, donde: c.donde, puntos: c.peso, orden })
    }
  })
  return [...mejores.values()]
    .sort((a, b) => b.puntos - a.puntos || a.orden - b.orden)
    .slice(0, MAXIMO)
    .map(({ orden: _orden, ...r }) => r)
}
