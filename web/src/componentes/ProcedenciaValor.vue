<template>
  <span v-if="procedencia" class="q-ml-sm">
    <!-- El chip se puede enfocar: con el teclado (o tocándolo un momento) se ve el tooltip, y el lector lo lee siempre. -->
    <q-chip dense outline tabindex="0" :aria-describedby="id" :color="procedencia === 'propio' ? 'primary' : 'neutro'" :icon="procedencia === 'propio' ? 'o_folder' : 'o_public'">
      {{ procedencia === 'propio' ? t('herencia.propio') : t('herencia.global') }}
      <q-tooltip>{{ texto }}</q-tooltip>
    </q-chip>
    <span :id="id" class="solo-lectores">{{ texto }}</span>
    <q-btn v-if="procedencia === 'propio'" flat dense no-caps size="sm" color="primary" :label="t('herencia.volver')" :loading="ocupado"
      @click="$emit('volver')" />
  </span>
</template>

<script setup lang="ts">
// ProcedenciaValor dice si un ajuste de un proyecto es suyo o viene de Global, y ofrece volver al de Global.
import { computed, useId } from 'vue'
import type { Procedencia } from '../herencia'
import { t } from '../i18n'

const props = defineProps<{ procedencia?: Procedencia; ocupado?: boolean }>()
defineEmits<{ volver: [] }>()
const id = useId()
const texto = computed(() => (props.procedencia === 'propio' ? t('herencia.propioTexto') : t('herencia.globalTexto')))
</script>
