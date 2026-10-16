<template>
  <q-btn flat round dense icon="more_horiz" :aria-label="t('comun.masAcciones')">
    <q-tooltip>{{ t('comun.masAcciones') }}</q-tooltip>
    <q-menu anchor="bottom right" self="top right">
      <q-list style="min-width: 220px">
        <q-item v-for="a in visibles" :key="a.etiqueta" v-close-popup clickable :class="{ 'text-negative': a.peligro }" @click="emit('elegir', a.indice)">
          <q-item-section avatar style="min-width: 36px"><q-icon :name="a.icono" /></q-item-section>
          <q-item-section>{{ a.etiqueta }}</q-item-section>
        </q-item>
      </q-list>
    </q-menu>
  </q-btn>
</template>

<script setup lang="ts">
// MenuAcciones agrupa las acciones de una fila tras «⋯», cada una con icono y texto; las destructivas van al final.
import { computed } from 'vue'
import { t } from '../i18n'

export interface Accion {
  etiqueta: string
  icono: string
  peligro?: boolean
  oculto?: boolean
}

const props = defineProps<{ acciones: Accion[] }>()
const emit = defineEmits<{ elegir: [indice: number] }>()
const visibles = computed(() =>
  props.acciones.map((a, indice) => ({ ...a, indice })).filter((a) => !a.oculto).sort((a, b) => Number(!!a.peligro) - Number(!!b.peligro)))
</script>
