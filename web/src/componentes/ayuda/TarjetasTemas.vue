<template>
  <div class="row q-col-gutter-md">
    <div v-for="tema in temas" :key="tema.id" class="col-12 col-sm-6 col-lg-4">
      <button type="button" class="tarjeta-tema full-height" @click="$emit('elegir', tema.id)">
        <q-icon :name="tema.icono" color="primary" size="24px" aria-hidden="true" />
        <span class="text-weight-medium">{{ t(tema.titulo) }}</span>
        <span class="text-body2 texto-suave">{{ t(tema.resumen) }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
// TarjetasTemas es el inicio de la Ayuda y los temas relacionados: una tarjeta por tema con su resumen.
import { t } from '../../i18n'
import type { Tema } from '../../ayuda/tipos'

defineProps<{ temas: Tema[] }>()
defineEmits<{ elegir: [id: string] }>()
</script>
