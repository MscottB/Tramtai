<template>
  <q-btn flat dense no-caps size="sm" color="primary" icon="o_help_outline" :label="t(queHago ? 'ayuda.ui.queHago' : 'ayuda.ui.saberMas')"
    @click="abrirAyuda(tema, ancla)" />
</template>

<script setup lang="ts">
// BotonAyuda abre un tema en el panel de la Ayuda desde donde hace falta: «¿Qué hago?» junto a un error y
// «Saber más» junto a una función.
import { t } from '../../i18n'
import { abrirAyuda } from '../../ayuda/panel'

defineProps<{ tema: string; ancla?: string; queHago?: boolean }>()
</script>
