<template>
  <q-banner rounded dense class="info-caja">
    <template #avatar><q-icon name="o_info" color="primary" /></template>{{ t(bloque.texto) }}
  </q-banner>
</template>

<script setup lang="ts">
import { t } from '../../i18n'
import type { Bloque } from '../../ayuda/tipos'

defineProps<{ bloque: Extract<Bloque, { tipo: 'nota' }> }>()
</script>
