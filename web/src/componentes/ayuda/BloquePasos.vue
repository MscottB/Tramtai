<template>
  <div class="apilado">
    <h3 v-if="bloque.titulo" :id="titulo" class="text-weight-medium text-body1 q-my-none">{{ t(bloque.titulo) }}</h3>
    <ol class="pasos-guia" :aria-labelledby="bloque.titulo ? titulo : undefined">
      <li v-for="(paso, i) in bloque.pasos" :key="paso"><span class="numero-estacion">{{ i + 1 }}</span><span>{{ t(paso) }}</span></li>
    </ol>
  </div>
</template>

<script setup lang="ts">
// BloquePasos lista unos pasos; con titulo van en su propia sección, con el título que la nombra.
import { useId } from 'vue'
import { t } from '../../i18n'
import type { Bloque } from '../../ayuda/tipos'

defineProps<{ bloque: Extract<Bloque, { tipo: 'pasos' }> }>()
const titulo = useId()
</script>
