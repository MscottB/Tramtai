<template>
  <div class="fullscreen flex flex-center q-pa-md" style="background: var(--fondo)">
    <q-card class="q-pa-md" style="width: 440px; max-width: 100%">
      <q-card-section class="text-center">
        <img src="/logo.png" alt="" width="56" height="56" style="border-radius: 14px" />
        <div class="titulo-pagina q-mt-sm">{{ t('entrar.titulo') }}</div>
        <div class="texto-suave q-mt-xs">{{ t('entrar.noConfirmado') }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-banner v-if="aviso" rounded class="error-caja q-mb-md" dense>
          <template #avatar><q-icon name="error_outline" /></template>
          {{ aviso }}
        </q-banner>

        <div class="text-weight-medium q-mb-xs">{{ t('entrar.rapida') }}</div>
        <div class="texto-suave text-body2 q-mb-sm">{{ t('entrar.rapidaTexto') }}</div>
        <div class="bloque-codigo">tramtai abrir<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('comun.copiar')" @click="copiar('tramtai abrir')" /></div>

        <q-separator class="q-my-lg" />

        <div class="text-weight-medium q-mb-xs">{{ t('entrar.app') }}</div>
        <div class="texto-suave text-body2">{{ t('entrar.appTexto') }}</div>
        <div class="text-caption texto-suave q-mt-md row no-wrap items-start">
          <q-icon name="o_lock" size="16px" class="q-mr-xs q-mt-xs" /><span>{{ t('entrar.sinClave') }}</span>
        </div>
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
// aplicaciones no abre la Web UI, así que se explica cómo entrar con un código de un solo uso.
import { copiar } from '../api/cliente'
import { cambiarIdioma, idioma, idiomas, t } from '../i18n'

defineProps<{ aviso: string }>()
</script>
