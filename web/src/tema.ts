// Apariencia de la Web UI: clara, oscura o como el sistema. Se recuerda en este navegador.
import { ref } from 'vue'
import { Dark } from 'quasar'

export type Tema = 'sistema' | 'claro' | 'oscuro'

const guardado = 'tramtai.tema'

function inicial(): Tema {
  try {
    const g = localStorage.getItem(guardado)
    if (g === 'sistema' || g === 'claro' || g === 'oscuro') return g
  } catch {
    // sin almacenamiento: como el sistema
  }
  return 'sistema'
}

export const tema = ref<Tema>(inicial())

// modoQuasar es el valor de config.dark de Quasar para un tema.
export const modoQuasar = (t: Tema = tema.value) => (t === 'sistema' ? 'auto' : t === 'oscuro')

export function cambiarTema(nuevo: Tema) {
  tema.value = nuevo
  Dark.set(modoQuasar(nuevo))
  try {
    localStorage.setItem(guardado, nuevo)
  } catch {
    // la preferencia no se recuerda, pero el cambio se aplica
  }
}
