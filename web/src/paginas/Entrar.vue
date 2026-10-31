<template>
  <div class="fullscreen flex flex-center q-pa-md" style="background: var(--fondo)">
    <q-card class="q-pa-md" style="width: 440px; max-width: 100%" data-captura="entrar">
      <q-card-section class="text-center">
        <img src="/logo.png" alt="" width="56" height="56" style="border-radius: 14px" />
        <div class="titulo-pagina q-mt-sm">{{ t('entrar.titulo') }}</div>
        <div class="texto-suave q-mt-xs">{{ vista === 'formulario' ? t('entrar.servidor') : t('entrar.noConfirmado') }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-banner v-if="aviso || error" rounded class="error-caja q-mb-md" dense role="alert">
          <template #avatar><q-icon name="error_outline" /></template>
          {{ error || aviso }}
        </q-banner>

        <!-- El servidor con administrador, por HTTPS: la contraseña y, si lo pide, el código. -->
        <q-form v-if="vista === 'formulario'" class="column q-gutter-md" @submit="entrar">
          <q-input v-model="contrasena" outlined type="password" autocomplete="current-password" :label="t('entrar.contrasena')" autofocus />
          <q-input v-if="estadoEntrada?.totp" v-model="codigo" outlined autocomplete="one-time-code" :label="t('entrar.codigo')"
            :hint="t('entrar.codigoAyuda')" />
          <q-btn type="submit" unelevated color="primary" no-caps :label="t('entrar.entrar')" :loading="entrando" :disable="!contrasena" />
        </q-form>

        <!-- Con administrador pero sin HTTPS: la contraseña viajaría en claro; queda el código de un solo uso. -->
        <template v-else-if="vista === 'sinHttps'">
          <div class="text-weight-medium q-mb-xs">{{ t('entrar.sinHttpsTitulo') }}</div>
          <div class="texto-suave text-body2 q-mb-sm">{{ t('entrar.sinHttps') }}</div>
          <div class="text-caption texto-suave q-mb-xs">{{ t('entrar.codigoServidor') }}</div>
          <div class="bloque-codigo">tramtai-servidor abrir<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar')" @click="copiar('tramtai-servidor abrir')" /></div>
        </template>

        <template v-else-if="vista === 'codigo'">
          <div class="text-weight-medium q-mb-xs">{{ t('entrar.rapida') }}</div>
          <div class="texto-suave text-body2 q-mb-sm">{{ t('entrar.rapidaTexto') }}</div>
          <div class="bloque-codigo">tramtai abrir<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar')" @click="copiar('tramtai abrir')" /></div>

          <q-separator class="q-my-lg" />

          <div class="text-weight-medium q-mb-xs">{{ t('entrar.app') }}</div>
          <div class="texto-suave text-body2">{{ t('entrar.appTexto') }}</div>
          <div class="text-caption texto-suave q-mt-md row no-wrap items-start">
            <q-icon name="o_lock" size="16px" class="q-mr-xs q-mt-xs" /><span>{{ t('entrar.sinClave') }}</span>
          </div>
        </template>
      </q-card-section>
      <q-card-section class="row justify-center q-pt-none">
        <q-btn-toggle :model-value="idioma" flat dense no-caps toggle-color="primary" :aria-label="t('app.idioma')"
          :options="Object.entries(idiomas).map(([value, d]) => ({ label: d.nombre, value }))" @update:model-value="cambiarIdioma" />
      </q-card-section>
    </q-card>
  </div>
</template>

<script setup lang="ts">
// Entrar se muestra cuando no se puede comprobar que quien abre la Web UI es el dueño del servicio: la clave de las
// aplicaciones no abre la Web UI. En un servidor con administrador, por HTTPS, se entra con su contraseña (y su código
// TOTP); si no, se explica cómo entrar con un código de un solo uso.
import { computed, onMounted, ref } from 'vue'
import { copiar } from '../api/cliente'
import { acceder, consultarEntrada, estadoEntrada, vistaEntrada } from '../entrada'
import { cambiarIdioma, idioma, idiomas, t } from '../i18n'

defineProps<{ aviso: string }>()

const vista = computed(() => vistaEntrada(estadoEntrada.value))
const contrasena = ref('')
const codigo = ref('')
const entrando = ref(false)
const error = ref('')

onMounted(consultarEntrada)

async function entrar() {
  entrando.value = true
  error.value = ''
  try {
    await acceder(contrasena.value, codigo.value)
  } catch (e) {
    error.value = (e as Error).message
    codigo.value = ''
  } finally {
    entrando.value = false
  }
}
</script>
