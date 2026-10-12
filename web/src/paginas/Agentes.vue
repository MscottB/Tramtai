<template>
  <div v-if="capacidades" class="row q-col-gutter-md">
    <div v-for="a in capacidades.adaptadores" :key="a.agente" class="col-12 col-md-6">
      <q-card class="full-height">
        <q-card-section>
          <div class="text-h6">{{ a.agente }}</div>
          <div class="text-caption text-grey-7">{{ a.version_detectada || 'versión no detectada' }} · {{ a.autenticacion }}</div>
        </q-card-section>
        <q-list dense>
          <q-item><q-item-section>Streaming</q-item-section><q-item-section side>{{ a.capacidades.streaming }}</q-item-section></q-item>
          <q-item><q-item-section>Salida estructurada</q-item-section><q-item-section side>{{ a.capacidades.salida_estructurada ? 'sí' : 'no' }}</q-item-section></q-item>
          <q-item><q-item-section>Informa el modelo efectivo</q-item-section><q-item-section side>{{ a.capacidades.modelo_efectivo_informado ? 'sí' : 'no' }}</q-item-section></q-item>
          <q-item><q-item-section>Uso de tokens</q-item-section><q-item-section side>{{ a.capacidades.uso_tokens }}</q-item-section></q-item>
        </q-list>
        <q-card-section class="text-caption">Términos: {{ a.terminos }}</q-card-section>
        <q-banner v-if="a.problema" class="bg-warning text-dark q-ma-sm" rounded>{{ a.problema }}</q-banner>
      </q-card>
    </div>
    <div class="col-12 text-caption text-grey-7">Formato de conversación: {{ capacidades.formato_serializacion }} · entradas: {{ capacidades.entradas?.join(', ') }}</div>
  </div>
  <div v-else class="flex flex-center q-pa-xl"><q-spinner size="40px" color="primary" /></div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, mensajeError } from '../api'

const $q = useQuasar()
const capacidades = ref<Record<string, any>>()

onMounted(async () => {
  try {
    capacidades.value = await api.capacidades()
  } catch (e) {
    $q.notify({ type: 'negative', message: mensajeError(e) })
  }
})
</script>
