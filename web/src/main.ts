import { createApp } from 'vue'
import { Quasar, Notify, Dialog } from 'quasar'
import idioma from 'quasar/lang/es'
import '@quasar/extras/material-icons/material-icons.css'
import 'quasar/dist/quasar.css'
import './estilos.css'
import App from './App.vue'

createApp(App)
  .use(Quasar, { plugins: { Notify, Dialog }, lang: idioma, config: { dark: 'auto' } })
  .mount('#app')
