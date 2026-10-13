import { createApp } from 'vue'
import { Quasar, Notify, Dialog } from 'quasar'
import '@quasar/extras/material-icons/material-icons.css'
import '@quasar/extras/material-icons-outlined/material-icons-outlined.css'
import 'quasar/dist/quasar.css'
import '@fontsource-variable/overpass'
import './estilos.css'
import { quasarInicial } from './i18n'
import { modoQuasar } from './tema'
import App from './App.vue'

// Colores del icono de Tramtai: verde azulado del fondo y blanco del tranvía.
const marca = {
  primary: '#0b6e64',
  secondary: '#14b8a6',
  accent: '#f59e0b',
  positive: '#15803d',
  negative: '#c42b2b',
  info: '#0369a1',
  warning: '#a16207',
}

createApp(App)
  .use(Quasar, { plugins: { Notify, Dialog }, lang: quasarInicial(), config: { dark: modoQuasar(), brand: marca } })
  .mount('#app')
