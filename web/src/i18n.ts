// Idiomas de la Web UI: español (principal) e inglés. Para añadir otro, crea idiomas/<código>.ts con el tipo
// Textos (TypeScript avisa si falta alguna clave) y añádelo a la lista de abajo con el paquete de Quasar.
import { ref } from 'vue'
import { Lang } from 'quasar'
import quasarEs from 'quasar/lang/es'
import quasarEn from 'quasar/lang/en-US'
import es, { type Clave } from './idiomas/es'
import en from './idiomas/en'
import { buscarTexto } from './textos'

export const idiomas = {
  es: { nombre: 'Español', textos: es, quasar: quasarEs },
  en: { nombre: 'English', textos: en, quasar: quasarEn },
}
export type Idioma = keyof typeof idiomas

const guardado = 'tramtai.idioma'

function inicial(): Idioma {
  try {
    const g = localStorage.getItem(guardado)
    if (g && g in idiomas) return g as Idioma
  } catch {
    // sin almacenamiento (ventana privada): se usa el del navegador
  }
  const navegador = (navigator.language || 'es').slice(0, 2)
  return navegador in idiomas ? (navegador as Idioma) : 'es'
}

// idioma es también el código para Intl (fechas y tiempos relativos).
export const idioma = ref<Idioma>(inicial())

export function cambiarIdioma(nuevo: Idioma) {
  idioma.value = nuevo
  Lang.set(idiomas[nuevo].quasar)
  document.documentElement.lang = nuevo
  try {
    localStorage.setItem(guardado, nuevo)
  } catch {
    // la preferencia no se recuerda, pero el cambio se aplica
  }
}

// t traduce una clave; {nombre} en el texto se sustituye por parametros.nombre. Con n igual a 1 se usa la
// variante en singular «<clave>Uno» si existe (por ejemplo, «1 petición» en lugar de «1 peticiones»).
export function t(clave: Clave, parametros?: Record<string, string | number>): string {
  const texto = buscarTexto(idiomas[idioma.value].textos, clave, parametros?.n)
  return parametros ? texto.replace(/\{(\w+)\}/g, (_, p) => String(parametros[p] ?? `{${p}}`)) : texto
}

export function quasarInicial() {
  document.documentElement.lang = idioma.value
  return idiomas[idioma.value].quasar
}
