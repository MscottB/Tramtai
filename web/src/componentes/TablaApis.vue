<template>
  <div>
    <div class="desplazable-x">
      <table class="guia-tabla text-body2">
        <thead><tr><th /><th v-for="c in tablaApis.columnas" :key="c">{{ t(c) }}</th></tr></thead>
        <tbody>
          <tr v-for="f in tablaApis.filas" :key="f.titulo"><th scope="row">{{ t(f.titulo) }}</th><td v-for="(c, i) in f.celdas" :key="i">{{ t(c) }}</td></tr>
        </tbody>
      </table>
    </div>
    <div class="text-weight-medium q-mt-md">{{ t('guia.porQue') }}</div>
    <div class="text-body2 texto-suave q-mt-xs">{{ t('guia.porQueTexto') }}</div>
    <div class="text-body2 texto-suave q-mt-sm">{{ t('guia.receta') }}</div>
  </div>
</template>

<script setup lang="ts">
// TablaApis compara lo que permite cada contrato (OpenAI, Anthropic y la API propia) y explica por qué no todos
// pueden lo mismo; la usan «¿Qué URL uso?» en Mis APIs y la Ayuda. Las filas son las del tema «Conectar mi app».
import { t } from '../i18n'
import { tablaApis } from '../ayuda/temas/conectar'
</script>

<style scoped>
.guia-tabla {
  border-collapse: collapse;
  min-width: 560px;
  width: 100%;
}
.guia-tabla th,
.guia-tabla td {
  border-bottom: 1px solid var(--borde);
  padding: 6px 8px;
  text-align: left;
  vertical-align: top;
}
.guia-tabla thead th {
  font-weight: 600;
}
.guia-tabla tbody th {
  font-weight: 400;
  color: var(--texto-suave);
}
</style>
