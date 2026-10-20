<template>
  <ol class="pasos-guia">
    <li v-for="(paso, i) in bloque.pasos" :key="paso"><span class="numero-estacion">{{ i + 1 }}</span><span>{{ t(paso) }}</span></li>
  </ol>
</template>

<script setup lang="ts">
import { t } from '../../i18n'
import type { Bloque } from '../../ayuda/tipos'

defineProps<{ bloque: Extract<Bloque, { tipo: 'pasos' }> }>()
</script>
