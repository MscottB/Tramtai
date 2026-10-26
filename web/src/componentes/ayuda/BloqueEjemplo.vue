<template>
  <div>
    <div class="desplazable-x"><q-btn-toggle v-model="elegido" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
      :options="lista.map((e) => ({ label: e.titulo(), value: e.id }))" /></div>
    <div class="bloque-codigo sin-ajuste q-mt-sm">{{ codigo }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
      :aria-label="t('conectar.copiarEjemplo')" @click="copiar(codigo)" /></div>
  </div>
</template>

<script setup lang="ts">
// BloqueEjemplo muestra el código de un mensaje (con sus archivos) por cada API, con la dirección predeterminada y la
// clave por poner.
import { computed, ref } from 'vue'
import { copiar } from '../../api/cliente'
import { ejemplos } from '../../ejemplos'
import { t } from '../../i18n'
import type { Bloque } from '../../ayuda/tipos'

const props = defineProps<{ bloque: Extract<Bloque, { tipo: 'ejemplo' }> }>()
const lista = computed(() => ejemplos({ url: 'http://127.0.0.1:17373', modelo: 'perfil/rapido', contratos: props.bloque.contratos,
  prueba: { archivos: props.bloque.archivos, solicitud: { mensajes: [{ rol: 'usuario', contenido: t('ejemplos.mensaje') }] } } }))
const elegido = ref('python')
const codigo = computed(() => (lista.value.find((e) => e.id === elegido.value) ?? lista.value[0]).codigo(t('codigo.tuClave')))
</script>
