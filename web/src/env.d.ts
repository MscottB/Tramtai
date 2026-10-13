declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const componente: DefineComponent<object, object, unknown>
  export default componente
}
declare module 'quasar/lang/es'
declare module 'quasar/lang/en-US'
