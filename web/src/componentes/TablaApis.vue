<template>
  <div>
    <BloqueTabla :bloque="tablaApis" />
    <div class="text-weight-medium q-mt-md">{{ t('guia.porQue') }}</div>
    <div class="text-body2 texto-suave q-mt-xs">{{ t('guia.porQueTexto') }}</div>
    <div class="text-body2 texto-suave q-mt-sm">{{ t('guia.receta') }}</div>
  </div>
</template>

<script setup lang="ts">
// TablaApis compara lo que permite cada contrato (OpenAI, Anthropic y la API propia) y explica por qué no todos
// pueden lo mismo, en «¿Qué URL uso?» de Mis APIs. La tabla es la del tema «Conectar mi app» de la Ayuda.
import { t } from '../i18n'
import { tablaApis } from '../ayuda/temas/conectar'
import BloqueTabla from './ayuda/BloqueTabla.vue'
</script>
