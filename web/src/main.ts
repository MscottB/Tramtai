import { createApp } from 'vue'
import { Quasar, Notify, Dialog } from 'quasar'
import idioma from 'quasar/lang/es'
import '@quasar/extras/material-icons/material-icons.css'
import '@quasar/extras/material-icons-outlined/material-icons-outlined.css'
import 'quasar/dist/quasar.css'
import './estilos.css'
import App from './App.vue'

// Colores del icono de Tramtai: verde azulado del fondo y blanco del tranvía.
const marca = {
  primary: '#0f766e',
  secondary: '#14b8a6',
  accent: '#f59e0b',
  positive: '#15803d',
  negative: '#dc2626',
  info: '#0369a1',
  warning: '#b45309',
}

createApp(App)
  .use(Quasar, { plugins: { Notify, Dialog }, lang: idioma, config: { dark: 'auto', brand: marca } })
  .mount('#app')
