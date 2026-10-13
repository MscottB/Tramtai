<template>
  <span class="linea" :class="'linea--' + (agente ?? 'otro')">
    <span class="linea__chapa" aria-hidden="true">{{ codigoLinea(agente) }}</span>
    <span class="linea__nombre">{{ nombreAgente(agente) }}</span>
    <span v-if="modelo" class="linea__modelo">{{ modelo }}</span>
  </span>
</template>

<script setup lang="ts">
// Linea muestra un agente como una línea de la red: su distintivo de color, su nombre y, si hay, el modelo.
import { codigoLinea, nombreAgente } from '../api'

defineProps<{ agente?: string; modelo?: string }>()
</script>
