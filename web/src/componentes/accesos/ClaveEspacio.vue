<template>
  <!-- La clave del espacio en Mis APIs: Tramtai solo guarda su huella, así que no hay nada que enseñar salvo al crearla o
       regenerarla. Entonces sale en un diálogo (como la clave de una API en DetalleApi) con su ejemplo; al cerrarse se
       olvida: no queda en memoria de la página. -->
  <div>
    <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
    <div v-if="error" class="row items-center q-gutter-sm" role="alert">
      <span class="text-body2">{{ t('comun.errorCarga') }}: {{ error }}</span>
      <q-btn flat dense no-caps color="primary" icon="o_refresh" :label="t('comun.reintentar')" @click="cargar" />
    </div>
    <div v-else-if="!estado" class="text-body2 texto-suave">{{ t('comun.cargando') }}</div>
    <div v-else-if="estado.origen === 'variable'" class="text-body2 texto-suave">{{ t('conectar.claveVariable', { variable: estado.variable ?? '' }) }}</div>
    <template v-else-if="!estado.tiene_clave">
      <div class="text-body2 texto-suave">{{ t('conectar.sinClave') }}</div>
      <!-- La primera clave no pierde nada: se crea sin preguntar. -->
      <q-btn outline dense no-caps color="primary" icon="o_key" class="q-mt-sm q-px-sm" :label="t('conectar.crearClave')" :loading="ocupado" @click="crear" />
    </template>
    <template v-else>
      <div class="text-body2 texto-suave">{{ t('conectar.conClave') }}</div>
      <!-- Regenerar invalida la clave en todas las apps: con su nombre escrito y en color de peligro. -->
      <q-btn flat dense no-caps color="negative" icon="o_autorenew" class="q-mt-sm" :label="t('conectar.regenerarClave')" :loading="ocupado" @click="regenerar" />
    </template>

    <!-- persistent: un clic fuera no tira la clave; Esc pregunta antes (escaparConAviso). -->
    <q-dialog v-model="abierto" persistent :maximized="$q.screen.lt.sm" aria-labelledby="titulo-clave-espacio" @hide="alCerrar">
      <q-card style="width: 640px; max-width: 96vw" @keyup.esc="escaparConAviso($event, true, () => (abierto = false), true)">
        <q-card-section class="row items-center no-wrap q-gutter-x-sm">
          <h2 id="titulo-clave-espacio" class="titulo-seccion col">{{ t('conectar.claveCreada') }}</h2>
          <q-btn v-close-popup flat round dense icon="close" :aria-label="t('comun.cerrar')" />
        </q-card-section>
        <q-separator />
        <q-card-section>
          <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
          <div class="bloque-codigo">{{ nueva }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarClave')" @click="copiar(nueva, t('conectar.claveCopiada'))" /></div>
          <q-banner rounded dense class="aviso-caja q-mt-sm">
            <template #avatar><q-icon name="o_lock" color="warning" /></template>{{ t('puntos.claveUnaVez') }}
          </q-banner>
          <div class="text-caption texto-suave q-mt-md q-mb-xs">{{ t('conectar.ejemplos') }}</div>
          <div class="bloque-codigo sin-ajuste">{{ codigo(nueva) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarEjemplo')" @click="copiar(codigo(nueva), t('conectar.ejemploCopiado'))" /></div>
        </q-card-section>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, avisar, avisarError, copiar, mensajeError } from '../../api/cliente'
import type { EstadoClave } from '../../api/tipos'
import { t } from '../../i18n'
import { escaparConAviso } from '../../cierre'

// codigo es el ejemplo del lenguaje elegido en la página, con la clave que se le da.
defineProps<{ codigo: (clave: string) => string }>()
const $q = useQuasar()

const estado = ref<EstadoClave>()
const error = ref('')
// nueva es la clave recién creada o regenerada, solo mientras el diálogo está abierto.
const nueva = ref('')
const abierto = ref(false)
const ocupado = ref(false)

function cargar() {
  error.value = ''
  return api.estadoClaveInstancia().then((e) => (estado.value = e)).catch((e) => (error.value = mensajeError(e)))
}

function mostrar(token: string) {
  nueva.value = token
  abierto.value = true
}

// alCerrar olvida la clave y vuelve a pedir el estado: ahora el espacio ya tiene clave.
function alCerrar() {
  nueva.value = ''
  cargar()
}

// crear vuelve a mirar el estado justo antes: si otra vía (la CLI, otra pestaña) ya la creó, regenerarla sin preguntar
// dejaría fuera a las apps.
async function crear() {
  ocupado.value = true
  try {
    const e = await api.estadoClaveInstancia()
    if (e.tiene_clave) {
      estado.value = e
      $q.notify({ type: 'warning', message: t('conectar.conClave'), position: 'top' })
      return
    }
    mostrar((await api.regenerarClaveInstancia()).token)
  } catch (e) {
    avisarError(e)
  } finally {
    ocupado.value = false
  }
}

// regenerar cambia la clave por si se filtró; las aplicaciones que la usan dejan de conectar hasta que se les ponga la nueva.
function regenerar() {
  $q.dialog({ title: t('conectar.regenerarTitulo'), message: t('conectar.regenerarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('conectar.regenerarClave'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      ocupado.value = true
      try {
        mostrar((await api.regenerarClaveInstancia()).token)
        avisar(t('conectar.claveRegenerada'))
      } catch (e) {
        avisarError(e)
      } finally {
        ocupado.value = false
      }
    })
}

onMounted(cargar)
</script>
