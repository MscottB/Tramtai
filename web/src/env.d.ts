/// <reference types="vite/client" />
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const componente: DefineComponent<object, object, unknown>
  export default componente
}
declare module 'quasar/lang/es'
declare module 'quasar/lang/en-US'
// Las hojas de estilo se importan solo por su efecto (TypeScript 6 exige declararlas).
declare module '*.css'
