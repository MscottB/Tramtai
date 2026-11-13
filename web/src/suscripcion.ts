// suscripcion: «Usar mi suscripción» con su aviso de riesgo, el mismo en Agentes, en el panel de preparación y en la
// bienvenida (Ruling 34: tras iniciar sesión se ofrece ahí mismo, nunca se cambia solo).
import { Dialog } from 'quasar'
import { api, avisar, avisarError } from './api/cliente'
import { fichaDe, nombreAgente, refrescarAgentes } from './agentes'
import { t } from './i18n'

// aceptarRiesgoSuscripcion enseña el aviso de riesgo (TERMINOS.md, punto 4) y dice si el usuario lo aceptó marcando la
// casilla; sin marcarla, avisa y no.
export function aceptarRiesgoSuscripcion(agente: string): Promise<boolean> {
  const cli = fichaDe(agente)?.programa ?? agente
  return new Promise((resolver) => Dialog.create({ title: t('agentes.confirmarTitulo', { agente: nombreAgente(agente) }),
    message: t('agentes.confirmarSuscripcion', { cli }) + '\n\n' + t('agentes.riesgoSuscripcion'),
    style: 'white-space: pre-line',
    options: { type: 'checkbox', model: [], items: [{ label: t('agentes.aceptoRiesgo'), value: 'acepto' }] },
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('agentes.usarSuscripcion'), unelevated: true, noCaps: true } })
    .onOk((elegidas: string[]) => {
      if (!elegidas.includes('acepto')) avisarError(new Error(t('agentes.faltaAceptar')))
      resolver(elegidas.includes('acepto'))
    })
    .onDismiss(() => resolver(false))) // tras onOk no cambia nada: la promesa ya está resuelta
}

// usarSuscripcion pasa el agente a la suscripción si el usuario acepta el riesgo y vuelve a comprobar los agentes; dice si
// quedó hecho.
export async function usarSuscripcion(agente: string): Promise<boolean> {
  if (!(await aceptarRiesgoSuscripcion(agente))) return false
  try {
    await api.cambiarAutenticacion(agente, 'suscripcion')
    avisar(t('agentes.suscripcionActivada', { agente: nombreAgente(agente) }))
  } catch (e) {
    avisarError(e)
    return false
  }
  await refrescarAgentes()
  return true
}
