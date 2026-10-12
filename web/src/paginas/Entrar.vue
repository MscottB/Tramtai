<template>
  <div class="fullscreen flex flex-center q-pa-md">
    <q-card style="width: 420px; max-width: 100%">
      <q-card-section>
        <div class="text-h5">Tramtai</div>
        <div class="text-subtitle2 text-grey-7">Entra con el token de una instancia</div>
      </q-card-section>
      <q-card-section>
        <q-banner v-if="aviso || error" class="bg-negative text-white q-mb-md" rounded>{{ error || aviso }}</q-banner>
        <q-form @submit="entrar">
          <q-input v-model="token" type="password" label="Token" outlined autofocus autocomplete="off" />
          <q-btn class="q-mt-md full-width" type="submit" color="primary" label="Entrar" :loading="enviando" :disable="!token" />
        </q-form>
      </q-card-section>
      <q-card-section class="text-caption text-grey-7">
        Más cómodo: <span class="codigo">tramtai global abrir</span> (o <span class="codigo">tramtai proyecto abrir</span> en un proyecto)
        abre esta página ya autenticada. El token está en <span class="codigo">tramtai global token</span>.
      </q-card-section>
    </q-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { api, mensajeError, type Sesion } from '../api'

defineProps<{ aviso: string }>()
const emit = defineEmits<{ entrar: [sesion: Sesion] }>()
const token = ref('')
const error = ref('')
const enviando = ref(false)

async function entrar() {
  enviando.value = true
  error.value = ''
  try {
    emit('entrar', await api.entrar(token.value))
    token.value = ''
  } catch (e) {
    error.value = mensajeError(e)
  } finally {
    enviando.value = false
  }
}
</script>
