<template>
  <div class="fullscreen flex flex-center q-pa-md" style="background: var(--fondo)">
    <q-card class="q-pa-md" style="width: 440px; max-width: 100%">
      <q-card-section class="text-center">
        <img src="/logo.png" alt="" width="56" height="56" style="border-radius: 14px" />
        <div class="titulo-pagina q-mt-sm">{{ t('entrar.titulo') }}</div>
        <div class="texto-suave q-mt-xs">{{ t('entrar.noConfirmado') }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-banner v-if="aviso || error" rounded class="error-caja q-mb-md" dense>
          <template #avatar><q-icon name="error_outline" /></template>
          {{ error || aviso }}
        </q-banner>

        <div class="text-weight-medium q-mb-xs">{{ t('entrar.rapida') }}</div>
        <div class="texto-suave text-body2 q-mb-sm">{{ t('entrar.rapidaTexto') }}</div>
        <div class="bloque-codigo">tramtai abrir<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar')" @click="copiar('tramtai abrir')" /></div>

        <q-separator class="q-my-lg" />

        <div class="text-weight-medium q-mb-sm">{{ t('entrar.conClave') }}</div>
        <q-form @submit="entrar">
          <q-input v-model="token" :type="ver ? 'text' : 'password'" outlined dense placeholder="tt_…" autocomplete="off" :aria-label="t('entrar.clave')">
            <template #append>
              <q-btn flat round dense :icon="ver ? 'visibility_off' : 'visibility'" :aria-label="ver ? t('conectar.ocultarClave') : t('conectar.mostrarClave')"
                @click="ver = !ver" />
            </template>
          </q-input>
          <div class="text-caption texto-suave q-mt-xs">{{ t('entrar.claveAyuda') }}</div>
          <q-btn class="q-mt-md full-width" type="submit" color="primary" unelevated no-caps :label="t('entrar.boton')" :loading="enviando" :disable="!token.trim()" />
        </q-form>
      </q-card-section>
      <q-card-section class="row justify-center q-pt-none">
        <q-btn-toggle :model-value="idioma" flat dense no-caps toggle-color="primary" :aria-label="t('app.idioma')"
          :options="Object.entries(idiomas).map(([value, d]) => ({ label: d.nombre, value }))" @update:model-value="cambiarIdioma" />
      </q-card-section>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { api, copiar, ErrorApi, mensajeError } from '../api'
import { cambiarIdioma, idioma, idiomas, t } from '../i18n'

defineProps<{ aviso: string }>()
const token = ref('')
const ver = ref(false)
const error = ref('')
const enviando = ref(false)

async function entrar() {
  enviando.value = true
  error.value = ''
  try {
    await api.entrar(token.value)
    token.value = ''
  } catch (e) {
    error.value = e instanceof ErrorApi && e.estado === 401 ? t('entrar.claveInvalida') : mensajeError(e)
  } finally {
    enviando.value = false
  }
}
</script>
