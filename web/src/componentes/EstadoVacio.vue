<template>
  <div class="estado-vacio">
    <svg viewBox="0 0 120 24" width="120" height="24" aria-hidden="true">
      <path d="M 4 12 L 92 12" fill="none" stroke="var(--borde-fuerte)" stroke-width="4" stroke-linecap="round" stroke-dasharray="0 9" />
      <circle cx="104" cy="12" r="8" fill="var(--superficie)" stroke="var(--borde-fuerte)" stroke-width="3.5" />
    </svg>
    <div class="estado-vacio__titulo">{{ titulo }}</div>
    <div v-if="texto" class="texto-suave">{{ texto }}</div>
    <q-btn v-if="accion" outline color="primary" no-caps class="q-mt-sm" :label="accion" :loading="cargando" @click="emit('accion')" />
    <div v-if="$slots.default" class="row justify-center q-gutter-sm q-mt-xs"><slot /></div>
  </div>
</template>

<script setup lang="ts">
// EstadoVacio es el estado vacío común: una estación apagada al final de un tramo sin servicio, un titular que
// invita a actuar y, si hace falta, la acción (y otras secundarias en la ranura). También es el estado de error de
// una carga: la vía sin servicio, qué falló y «Reintentar».
defineProps<{ titulo: string; texto?: string; accion?: string; cargando?: boolean }>()
const emit = defineEmits<{ accion: [] }>()
</script>
