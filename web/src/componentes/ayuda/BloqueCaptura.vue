<template>
  <img v-if="url" :src="url" :alt="t(bloque.alt)" class="captura-ayuda" loading="lazy" />
</template>

<script setup lang="ts">
// BloqueCaptura muestra la variante de la captura que coincide con el tema de colores y el idioma de la app; si aún
// no se ha sacado, no muestra nada.
import { computed } from 'vue'
import { useQuasar } from 'quasar'
import { idioma, t } from '../../i18n'
import { varianteCaptura } from '../../ayuda/vista'
import type { Bloque } from '../../ayuda/tipos'

const props = defineProps<{ bloque: Extract<Bloque, { tipo: 'captura' }> }>()
const archivos = import.meta.glob(['../../ayuda/capturas/*.webp', '!../../ayuda/capturas/recorrido-*.webp'], { eager: true, query: '?url', import: 'default' }) as Record<string, string>
const $q = useQuasar()
const url = computed(() => archivos['../../ayuda/capturas/' + varianteCaptura(props.bloque.id, $q.dark.isActive, idioma.value)])
</script>
