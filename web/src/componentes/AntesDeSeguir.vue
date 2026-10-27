<template>
  <div class="fullscreen flex flex-center q-pa-md" style="background: var(--fondo); overflow: auto">
    <q-card class="q-pa-md" style="width: 600px; max-width: 100%" role="region" aria-labelledby="titulo-terminos">
      <q-card-section>
        <img src="/logo.png" alt="" width="48" height="48" style="border-radius: 12px" />
        <h1 id="titulo-terminos" class="titulo-pagina q-mt-sm q-mb-none">{{ t('terminos.titulo') }}</h1>
        <p class="texto-suave q-mt-xs q-mb-none">{{ t('terminos.intro', { version: estado.version }) }}</p>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <ol class="q-pl-md q-my-none">
          <li v-for="riesgo in riesgosTexto()" :key="riesgo" class="q-mb-sm">{{ riesgo }}</li>
        </ol>
        <a :href="URL_TERMINOS" target="_blank" rel="noopener" class="text-primary">{{ t('terminos.leer') }}</a>
      </q-card-section>
      <q-card-actions align="between" class="q-px-md q-pb-md">
        <q-btn-toggle :model-value="idioma" flat dense no-caps toggle-color="primary" :aria-label="t('app.idioma')"
          :options="Object.entries(idiomas).map(([value, d]) => ({ label: d.nombre, value }))" @update:model-value="cambiarIdioma" />
        <div>
          <q-btn flat no-caps :label="t('terminos.salir')" :disable="aceptando" @click="salirDeTerminos" />
          <q-btn unelevated no-caps color="primary" class="q-ml-sm" :label="t('terminos.acepto')" :loading="aceptando" @click="aceptar" />
        </div>
      </q-card-actions>
    </q-card>
  </div>
</template>

<script setup lang="ts">
// AntesDeSeguir pide aceptar los términos de uso (con sus cinco riesgos) la primera vez que se abre la Web UI en este
// entorno y cuando cambia su versión mayor. «Salir» no usa nada: App.vue deja la pantalla «Has salido».
import { ref } from 'vue'
import { avisarError } from '../api/cliente'
import type { EstadoTerminos } from '../api/tipos'
import { cambiarIdioma, idioma, idiomas, t } from '../i18n'
import { aceptarTerminos, riesgosTexto, salirDeTerminos, URL_TERMINOS } from '../terminos'

defineProps<{ estado: EstadoTerminos }>()
const aceptando = ref(false)

async function aceptar() {
  aceptando.value = true
  try {
    await aceptarTerminos()
  } catch (e) {
    avisarError(e)
  } finally {
    aceptando.value = false
  }
}
</script>
