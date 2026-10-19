<template>
  <div v-if="conversor && conversor.estado !== 'listo'" class="aviso aviso-conversor q-pa-sm" role="status">
    <div class="row items-start no-wrap q-gutter-x-sm">
      <q-icon :name="conversor.estado === 'error' ? 'o_error_outline' : 'o_description'" size="20px" class="q-mt-xs" />
      <div class="col text-body2">
        <div>{{ textoConversor(conversor) }}</div>
        <div v-if="conversor.estado === 'error'" class="text-caption texto-suave q-mt-xs">{{ t('conversor.redEmpresa') }}</div>
        <q-linear-progress v-if="conversor.estado === 'preparando'" :value="(conversor.progreso ?? 0) / 100" rounded class="q-mt-sm" />
      </div>
    </div>
    <div v-if="conversor.estado !== 'preparando'" class="row justify-end q-gutter-x-sm q-mt-xs">
      <q-btn v-if="enlaceAjustes" flat dense no-caps color="primary" :label="t('conversor.irAjustes')" @click="$emit('ajustes')" />
      <q-btn flat dense no-caps color="primary" icon="o_download" :label="t('conversor.preparar')" @click="prepararConversor" />
    </div>
  </div>
</template>

<script setup lang="ts">
// AvisoConversor avisa, antes de enviar, de que falta el conversor de archivos (o se está preparando) y deja prepararlo.
import { conversor, prepararConversor, textoConversor } from '../conversor'
import { t } from '../i18n'

defineProps<{ enlaceAjustes?: boolean }>()
defineEmits<{ ajustes: [] }>()
</script>
