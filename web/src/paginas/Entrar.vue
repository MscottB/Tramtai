<template>
  <div class="fullscreen flex flex-center q-pa-md" style="background: var(--fondo)">
    <q-card class="q-pa-md" style="width: 440px; max-width: 100%">
      <q-card-section class="text-center">
        <img src="/logo.png" alt="" width="56" height="56" style="border-radius: 14px" />
        <div class="titulo-pagina q-mt-sm">Entrar en Tramtai</div>
        <div class="texto-suave q-mt-xs">No pudimos confirmar que este navegador es tuyo.</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-banner v-if="aviso || error" rounded class="error-caja q-mb-md" dense>
          <template #avatar><q-icon name="error_outline" /></template>
          {{ error || aviso }}
        </q-banner>

        <div class="text-weight-medium q-mb-xs">La forma más rápida</div>
        <div class="texto-suave text-body2 q-mb-sm">Ejecuta esto en una terminal; abre esta página ya dentro:</div>
        <div class="bloque-codigo">tramtai abrir<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" aria-label="Copiar" @click="copiar('tramtai abrir')" /></div>

        <q-separator class="q-my-lg" />

        <div class="text-weight-medium q-mb-sm">O entra con la clave de la instancia</div>
        <q-form @submit="entrar">
          <q-input v-model="token" :type="ver ? 'text' : 'password'" outlined dense placeholder="tt_…" autocomplete="off" aria-label="Clave">
            <template #append>
              <q-icon :name="ver ? 'visibility_off' : 'visibility'" class="cursor-pointer" @click="ver = !ver" />
            </template>
          </q-input>
          <div class="text-caption texto-suave q-mt-xs">
            La muestra <span class="codigo">tramtai global token</span> (o <span class="codigo">tramtai proyecto token</span> en un proyecto).
          </div>
          <q-btn class="q-mt-md full-width" type="submit" color="primary" unelevated no-caps label="Entrar" :loading="enviando" :disable="!token.trim()" />
        </q-form>
      </q-card-section>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { api, copiar, ErrorApi, mensajeError, type Sesion } from '../api'

defineProps<{ aviso: string }>()
const emit = defineEmits<{ entrar: [sesion: Sesion] }>()
const token = ref('')
const ver = ref(false)
const error = ref('')
const enviando = ref(false)

async function entrar() {
  enviando.value = true
  error.value = ''
  try {
    emit('entrar', await api.entrar(token.value))
    token.value = ''
  } catch (e) {
    error.value = e instanceof ErrorApi && e.estado === 401 ? 'Esa clave no corresponde a ninguna instancia activa.' : mensajeError(e)
  } finally {
    enviando.value = false
  }
}
</script>
