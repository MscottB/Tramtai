<template>
  <span v-if="procedencia" class="q-ml-sm">
    <q-chip dense outline :color="procedencia === 'propio' ? 'primary' : 'grey-7'" :icon="procedencia === 'propio' ? 'o_folder' : 'o_public'">
      {{ procedencia === 'propio' ? t('herencia.propio') : t('herencia.global') }}
      <q-tooltip>{{ procedencia === 'propio' ? t('herencia.propioTexto') : t('herencia.globalTexto') }}</q-tooltip>
    </q-chip>
    <q-btn v-if="procedencia === 'propio'" flat dense no-caps size="sm" color="primary" :label="t('herencia.volver')" :loading="ocupado"
      @click="$emit('volver')" />
  </span>
</template>

<script setup lang="ts">
// ProcedenciaValor dice si un ajuste de un proyecto es suyo o viene de Global, y ofrece volver al de Global.
import type { Procedencia } from '../herencia'
import { t } from '../i18n'

defineProps<{ procedencia?: Procedencia; ocupado?: boolean }>()
defineEmits<{ volver: [] }>()
</script>
