<template>
  <div>
    <div class="desplazable-x">
      <table class="guia-tabla text-body2">
        <thead><tr><th /><th>OpenAI</th><th>Anthropic</th><th>{{ t('ejemplos.nativa') }}</th></tr></thead>
        <tbody>
          <tr v-for="f in filas" :key="f[0]"><th scope="row">{{ t(f[0]) }}</th><td v-for="(c, i) in f.slice(1)" :key="i">{{ t(c) }}</td></tr>
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
// pueden lo mismo; la usan «¿Qué URL uso?» en Mis APIs y la Ayuda.
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const filas: Clave[][] = [
  ['guia.fSdk', 'guia.si', 'guia.si', 'guia.noHttp'],
  ['guia.fVivo', 'guia.si', 'guia.si', 'guia.siRetoma'],
  ['guia.fJson', 'guia.siResponseFormat', 'guia.no', 'guia.siEsquema'],
  ['guia.fArchivos', 'guia.siBase64', 'guia.siBase64', 'guia.si50'],
  ['guia.fTareas', 'guia.no', 'guia.no', 'guia.si'],
  ['guia.fRespaldo', 'guia.soloEsfuerzo', 'guia.no', 'guia.si'],
  ['guia.fRegistro', 'guia.no', 'guia.no', 'guia.si'],
  ['guia.fLimites', 'guia.no', 'guia.no', 'guia.si'],
]
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
