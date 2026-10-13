<template>
  <span class="linea" :class="'linea--' + (agente ?? 'otro')">
    <span class="linea__barra" aria-hidden="true" />
    <span class="linea__nombre">{{ nombreAgente(agente) }}</span>
    <span v-if="modelo" class="linea__modelo">{{ modelo }}</span>
  </span>
</template>

<script setup lang="ts">
// Linea muestra un agente como una línea de la red: su barra de color, su nombre y, si hay, el modelo.
import { nombreAgente } from '../api'

defineProps<{ agente?: string; modelo?: string }>()
</script>
