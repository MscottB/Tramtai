<template>
  <div class="apilado">
    <div class="text-weight-medium">{{ t('ayuda.ui.recorridoTitulo') }}</div>
    <ol class="pasos-guia">
      <li v-for="(paso, i) in bloque.pasos" :key="paso.ancla">
        <span class="numero-estacion">{{ i + 1 }}</span>
        <span><strong>{{ t(paso.titulo) }}</strong> {{ t(paso.texto) }}</span>
      </li>
    </ol>
    <div><q-btn outline color="primary" no-caps icon="o_tour" :label="t('ayuda.ui.mostrar')" @click="$emit('mostrar')" /></div>
  </div>
</template>

<script setup lang="ts">
// BloqueRecorrido lista lo que se verá en el recorrido guiado de la página y ofrece empezarlo en la propia pantalla.
import { t } from '../../i18n'
import type { Bloque } from '../../ayuda/tipos'

defineProps<{ bloque: Extract<Bloque, { tipo: 'recorrido' }> }>()
defineEmits<{ mostrar: [] }>()
</script>
