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

// Los colores de Quasar (primary, positive, negative, warning, info) salen de los tokens de estilos.css, con su par de
// noche: aquí no se fija ninguno, porque Quasar los escribiría en línea en body y no cambiarían con el tema.
createApp(App)
  .use(Quasar, { plugins: { Notify, Dialog }, lang: quasarInicial(), config: { dark: modoQuasar() } })
  .mount('#app')
