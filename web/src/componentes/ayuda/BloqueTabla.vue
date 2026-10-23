<template>
  <div class="desplazable-x">
    <table class="tabla-ayuda text-body2">
      <thead><tr><td /><th v-for="c in bloque.columnas" :key="c">{{ t(c) }}</th></tr></thead>
      <tbody>
        <tr v-for="f in bloque.filas" :key="f.titulo"><th scope="row">{{ t(f.titulo) }}</th><td v-for="(c, i) in f.celdas" :key="i">{{ t(c) }}</td></tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { t } from '../../i18n'
import type { Bloque } from '../../ayuda/tipos'

defineProps<{ bloque: Extract<Bloque, { tipo: 'tabla' }> }>()
</script>
