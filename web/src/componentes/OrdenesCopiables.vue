<template>
  <!-- Órdenes que se ejecutan una tras otra: cada una en su bloque con su botón de copiar, y «Después:» entre ellas. Nunca
       juntas en un bloque: en una consola sin pegado entre corchetes (cmd.exe, sh) la segunda línea se quedaría en el búfer
       y la primera, si pregunta algo (un código, la contraseña de sudo, [s/N]), la leería como respuesta (Ruling 35). -->
  <div class="column q-gutter-y-xs">
    <template v-for="(orden, i) in ordenes" :key="orden">
      <div v-if="i" class="text-caption texto-suave">{{ t('comun.despues') }}</div>
      <div class="bloque-codigo">{{ orden }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
        :aria-label="t('agentes.copiarOrden', { orden })" @click="copiar(orden)" /></div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { copiar } from '../api/cliente'
import { t } from '../i18n'

defineProps<{ ordenes: string[] }>()
</script>
