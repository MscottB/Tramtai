<template>
  <!-- «⋯» de la tarjeta de un agente: comprobar la versión, actualizar, el registro y, aparte, desinstalar. -->
  <q-btn flat round dense icon="more_horiz" :loading="comprobando" :aria-label="t('preparar.acciones', { agente: nombreAgente(agente) })">
    <q-menu>
      <q-list style="min-width: 220px">
        <q-item v-close-popup clickable @click="emit('comprobar')">
          <q-item-section avatar><q-icon name="o_refresh" /></q-item-section>
          <q-item-section>{{ t('preparar.comprobarVersion') }}</q-item-section>
        </q-item>
        <q-item v-if="versiones?.acciones.includes('actualizar')" v-close-popup clickable @click="emit('preparar', 'actualizar')">
          <q-item-section avatar><q-icon name="o_upgrade" /></q-item-section>
          <q-item-section>{{ t('preparar.actualizar') }}</q-item-section>
        </q-item>
        <q-item v-close-popup clickable @click="emit('registro')">
          <q-item-section avatar><q-icon name="o_receipt_long" /></q-item-section>
          <q-item-section>{{ t('preparar.verRegistro') }}</q-item-section>
        </q-item>
        <!-- La destructiva, aparte y al final, en color de error. -->
        <template v-if="versiones?.acciones.includes('desinstalar')">
          <q-separator />
          <q-item v-close-popup clickable class="text-negative" @click="emit('desinstalar')">
            <q-item-section avatar><q-icon name="o_delete" color="negative" /></q-item-section>
            <q-item-section>{{ t('preparar.desinstalar') }}</q-item-section>
          </q-item>
        </template>
      </q-list>
    </q-menu>
  </q-btn>
</template>

<script setup lang="ts">
import type { VersionesAgente } from '../api/tipos'
import { nombreAgente } from '../agentes'
import { t } from '../i18n'

defineProps<{ agente: string; versiones?: VersionesAgente; comprobando: boolean }>()
const emit = defineEmits<{ comprobar: []; preparar: [accion: 'actualizar']; registro: []; desinstalar: [] }>()
</script>
