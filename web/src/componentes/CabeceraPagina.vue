<template>
  <header class="cabecera-pagina">
    <button v-if="nav?.anterior.value" type="button" class="cabecera-pagina__volver" @click="nav.volver()">
      <q-icon name="arrow_back" size="16px" aria-hidden="true" />{{ t('app.volverA', { pagina: nav.anterior.value }) }}
    </button>
    <div class="row items-start q-col-gutter-md">
      <div class="col-12 col-sm">
        <h1 class="titulo-pagina q-ma-none">{{ titulo }}</h1>
        <div v-if="descripcion || $slots.descripcion" class="texto-suave q-mt-xs"><slot name="descripcion">{{ descripcion }}</slot></div>
      </div>
      <div v-if="$slots.acciones" class="col-12 col-sm-auto row items-center q-gutter-sm"><slot name="acciones" /></div>
    </div>
  </header>
</template>

<script setup lang="ts">
// CabeceraPagina es la cabecera común: Volver (un paso atrás, si hay de dónde volver), título, descripción y
// acciones siempre en el mismo sitio.
import { inject, type ComputedRef } from 'vue'
import { t } from '../i18n'

defineProps<{ titulo: string; descripcion?: string }>()
const nav = inject<{ anterior: ComputedRef<string>; volver: () => void }>('navegacion')
</script>
