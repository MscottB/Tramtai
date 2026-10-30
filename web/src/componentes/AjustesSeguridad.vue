<template>
  <div class="apilado">
    <q-card>
      <q-card-section class="row items-start q-col-gutter-md">
        <div class="col-12 col-md">
          <h2 class="titulo-seccion">{{ t('seguridad.contrasenaTitulo') }}</h2>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('seguridad.contrasenaTexto') }}</div>
        </div>
        <q-form class="col-12 col-md-5 column q-gutter-sm" @submit="cambiarContrasena">
          <q-input v-model="actual" outlined dense type="password" autocomplete="current-password" :label="t('seguridad.actual')" />
          <q-input v-model="nueva" outlined dense type="password" autocomplete="new-password" :label="t('seguridad.nueva')"
            :rules="[(v: string) => !v || [...v].length >= 12 || t('seguridad.corta')]" lazy-rules />
          <q-input v-model="repetida" outlined dense type="password" autocomplete="new-password" :label="t('seguridad.repetir')"
            :rules="[(v: string) => v === nueva || t('seguridad.noCoinciden')]" lazy-rules />
          <div><q-btn type="submit" unelevated color="primary" no-caps :label="t('seguridad.cambiar')" :loading="ocupado === 'contrasena'"
            :disable="!actual || [...nueva].length < 12 || nueva !== repetida" /></div>
        </q-form>
      </q-card-section>
    </q-card>

    <q-card>
      <q-card-section>
        <div class="row items-center q-gutter-sm">
          <h2 class="titulo-seccion">{{ t('seguridad.totpTitulo') }}</h2>
          <q-badge v-if="conTOTP !== null" :color="conTOTP ? 'positive' : 'grey-7'" :label="conTOTP ? t('seguridad.totpActiva') : t('seguridad.totpInactiva')" />
        </div>
        <div class="texto-suave text-body2 q-mt-xs">{{ t('seguridad.totpTexto') }}</div>
        <!-- En Linux el servicio no escribe en systemd-creds: se hace en el servidor, con la orden. -->
        <template v-if="linux">
          <div class="text-body2 q-mt-sm">{{ t('seguridad.linux') }}</div>
          <div class="bloque-codigo q-mt-xs">{{ ordenLinux }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar')" @click="copiar(ordenLinux)" /></div>
        </template>
        <template v-else-if="nuevo">
          <div class="text-body2 q-mt-sm">{{ t('seguridad.secreto') }}</div>
          <div class="bloque-codigo q-mt-xs">{{ nuevo.secreto }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar')" @click="copiar(nuevo.secreto)" /></div>
          <a class="enlace text-body2" :href="nuevo.uri">{{ t('seguridad.enlace') }}</a>
          <q-form class="row q-col-gutter-sm items-start q-mt-sm" @submit="confirmar">
            <div class="col-12 col-sm-6"><q-input v-model="codigo" outlined dense inputmode="numeric" autocomplete="one-time-code" maxlength="6" :label="t('seguridad.codigo')" /></div>
            <div class="col-auto"><q-btn type="submit" unelevated color="primary" no-caps :label="t('seguridad.confirmar')" :loading="ocupado === 'totp'" :disable="codigo.length !== 6" /></div>
            <div class="col-auto"><q-btn flat no-caps :label="t('comun.cancelar')" @click="nuevo = null" /></div>
          </q-form>
        </template>
        <div v-else class="row q-gutter-sm q-mt-sm">
          <q-btn v-if="!conTOTP" unelevated color="primary" no-caps :label="t('seguridad.activar')" :loading="ocupado === 'totp'" @click="empezarTOTP" />
          <q-btn v-else flat color="negative" no-caps :label="t('seguridad.quitar')" :loading="ocupado === 'totp'" @click="quitarTOTP" />
        </div>
      </q-card-section>
      <template v-if="conTOTP || codigos.length">
        <q-separator />
        <q-card-section>
          <h3 class="titulo-seccion">{{ t('seguridad.recuperacionTitulo') }}</h3>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('seguridad.recuperacionTexto') }}</div>
          <template v-if="codigos.length">
            <div class="aviso text-body2 q-mt-sm">{{ t('seguridad.guardalos') }}</div>
            <div class="bloque-codigo q-mt-xs" style="white-space: pre">{{ codigos.join('\n') }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar')" @click="copiar(codigos.join('\n'))" /></div>
          </template>
          <q-btn class="q-mt-sm" flat color="primary" no-caps icon="o_refresh" :label="t('seguridad.regenerar')" :loading="ocupado === 'codigos'" @click="regenerar" />
        </q-card-section>
      </template>
    </q-card>

    <q-card>
      <q-card-section class="row items-center q-col-gutter-md">
        <div class="col-12 col-md">
          <h2 class="titulo-seccion">{{ t('seguridad.sesionesTitulo') }}</h2>
          <div class="texto-suave text-body2 q-mt-xs">{{ t('seguridad.sesionesTexto') }}</div>
        </div>
        <div class="col-auto">
          <q-btn outline color="negative" no-caps icon="o_logout" :label="t('seguridad.cerrarSesiones')" :loading="ocupado === 'sesiones'" @click="cerrarSesiones" />
        </div>
      </q-card-section>
    </q-card>
  </div>
</template>

<script setup lang="ts">
// Ajustes → Seguridad (solo en el modo servidor): la contraseña del administrador, la verificación en dos pasos (TOTP,
// que se activa en dos pasos: el secreto y su confirmación), los códigos de recuperación y «Cerrar todas las sesiones».
import { computed, onMounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, copiar } from '../api/cliente'
import type { SecretoTOTP } from '../api/tipos'
import { t } from '../i18n'

const $q = useQuasar()
const actual = ref('')
const nueva = ref('')
const repetida = ref('')
const conTOTP = ref<boolean | null>(null)
const linux = ref(false)
const nuevo = ref<SecretoTOTP | null>(null)
const codigo = ref('')
const codigos = ref<string[]>([])
const ocupado = ref<'' | 'contrasena' | 'totp' | 'codigos' | 'sesiones'>('')
const ordenLinux = computed(() => `sudo tramtai-servidor administrador totp ${conTOTP.value ? 'quitar' : 'activar'}`)

onMounted(async () => {
  try {
    const [entrada, acerca] = await Promise.all([api.entrada(), api.acerca()])
    conTOTP.value = entrada.totp
    linux.value = acerca.sistema === 'linux'
  } catch (e) {
    avisarError(e)
  }
})

// hacer marca qué está ocupado mientras dura la acción y avisa del error, si lo hay.
async function hacer(que: typeof ocupado.value, accion: () => Promise<void>) {
  ocupado.value = que
  try {
    await accion()
  } catch (e) {
    avisarError(e)
  } finally {
    ocupado.value = ''
  }
}

// Cambiar la contraseña cierra todas las sesiones: la Web UI vuelve a la entrada.
const cambiarContrasena = () => hacer('contrasena', async () => {
  await api.cambiarContrasena(actual.value, nueva.value)
  avisar(t('seguridad.cambiada'))
})

// pedir pide un dato antes de un cambio de seguridad (la contraseña o el código); null si se cancela.
function pedir(texto: string, tipo: 'password' | 'text', etiqueta: string): Promise<string | null> {
  return new Promise((resolver) => {
    $q.dialog({ title: t('seguridad.confirmarTitulo'), message: texto, cancel: true,
      prompt: { model: '', type: tipo, label: etiqueta, isValid: (v: string) => v.trim().length > 0 } })
      .onOk((v: string) => resolver(v))
      .onDismiss(() => resolver(null))
  })
}

// conContrasena hace la acción con la contraseña actual y, con TOTP, un código de la app o de recuperación (sin él, una
// sesión robada con la contraseña no quitaría el segundo factor); nada si se cancela.
const conContrasena = (que: typeof ocupado.value, accion: (actual: string, codigo: string) => Promise<void>) => async () => {
  const actual = await pedir(t('seguridad.confirmarTexto'), 'password', t('seguridad.actual'))
  if (actual === null) return
  const codigo = conTOTP.value ? await pedir(t('seguridad.codigoActualTexto'), 'text', t('seguridad.codigoActual')) : ''
  if (codigo !== null) await hacer(que, () => accion(actual, codigo))
}

const empezarTOTP = conContrasena('totp', async (actual) => {
  codigo.value = ''
  nuevo.value = await api.nuevoTOTP(actual)
})

const confirmar = () => hacer('totp', async () => {
  codigos.value = await api.confirmarTOTP(codigo.value)
  nuevo.value = null
  conTOTP.value = true
  avisar(t('seguridad.activada'))
})

const quitarTOTP = conContrasena('totp', async (actual, codigo) => {
  await api.quitarTOTP(actual, codigo)
  conTOTP.value = false
  codigos.value = []
  avisar(t('seguridad.quitada'))
})

const regenerar = conContrasena('codigos', async (actual, codigo) => {
  codigos.value = await api.codigosRecuperacion(actual, codigo)
})

const cerrarSesiones = () => hacer('sesiones', async () => {
  await api.cerrarSesiones()
})
</script>
