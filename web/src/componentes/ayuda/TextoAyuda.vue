<template>
  <template v-for="(p, i) in partesOpciones(texto)" :key="i"><span v-if="p.opcion" class="opcion-cli">{{ p.texto }}</span><template v-else>{{ p.texto }}</template></template>
</template>

<script setup lang="ts">
// TextoAyuda pinta un texto de la Ayuda con sus opciones de la CLI sin partir (partesOpciones).
import { partesOpciones } from '../../ayuda/vista'

defineProps<{ texto: string }>()
</script>
