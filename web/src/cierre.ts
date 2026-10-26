// Cerrar un diálogo con Esc sin perder lo que no se puede recuperar: lo escrito en un formulario o una clave que solo
// se ve una vez. Esos diálogos llevan persistent (un clic fuera no los cierra) y llaman a esto desde el @keyup.esc del q-dialog (no de la tarjeta: recién abierto, el foco está fuera de ella).
import { Dialog } from 'quasar'
import { t } from './i18n'

// escaparConAviso cierra el diálogo; si se perdería algo, antes pregunta. clave elige el texto de la clave de un solo
// uso. Con un menú abierto (la lista de un selector), Esc solo cierra el menú.
// El keyup no sigue hasta window: allí el Esc de Quasar cerraría en el acto el aviso que se acaba de abrir.
export function escaparConAviso(evento: Event, perderia: boolean, cerrar: () => void, clave = false) {
  if (typeof document !== 'undefined' && document.querySelector('.q-menu')) return
  evento.stopPropagation()
  if (!perderia) return cerrar()
  Dialog.create({ title: t(clave ? 'cierre.claveTitulo' : 'cierre.titulo'), message: t(clave ? 'cierre.clave' : 'cierre.texto'),
    cancel: { flat: true, noCaps: true, label: t('cierre.seguir') }, ok: { color: 'negative', unelevated: true, noCaps: true, label: t('comun.cerrar') } })
    .onOk(cerrar)
}
