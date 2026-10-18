<template>
  <q-list class="q-mt-sm">
    <q-expansion-item v-for="(n, i) in versiones" :key="n.version" :label="n.version" :default-opened="i === 0" dense header-class="q-px-none text-weight-medium">
      <NotasVersion :texto="n.notas ?? ''" :pagina="n.url" />
    </q-expansion-item>
  </q-list>
</template>

<script setup lang="ts">
// ListaNovedades muestra lo que trae cada versión por separado, la más nueva abierta; la usan los avisos de antes y
// de después de actualizar.
import type { Novedad } from '../api'
import NotasVersion from './NotasVersion.vue'

defineProps<{ versiones: Novedad[] }>()
</script>
