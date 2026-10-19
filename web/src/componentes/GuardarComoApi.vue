<template>
  <q-dialog v-model="dialogoGuardar" aria-labelledby="titulo-guardar-api">
    <q-card style="width: 520px; max-width: 95vw">
      <q-card-section>
        <div id="titulo-guardar-api" class="titulo-seccion">{{ t('ejecutar.guardarApi') }}</div>
        <div class="texto-suave text-body2">{{ t('ejecutar.guardarTexto') }}</div>
      </q-card-section>
      <q-form @submit="guardarComoApi">
        <q-card-section class="q-gutter-y-md q-pt-none">
          <q-input v-model="nueva.nombre" outlined dense autofocus :label="t('puntos.nombre')" :hint="t('puntos.nombreHint')"
            :rules="[(v: string) => /^[a-z0-9][a-z0-9_-]{0,39}$/.test(v) || t('puntos.nombreRegla')]" />
          <q-input v-model="nueva.grupo" outlined dense maxlength="40" :label="t('puntos.grupo')" :hint="t('puntos.grupoHint')" />
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md">
          <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
          <q-btn unelevated color="primary" no-caps type="submit" :label="t('comun.crear')" :loading="guardando" />
        </q-card-actions>
      </q-form>
    </q-card>
  </q-dialog>

  <!-- Al cerrarse se olvida la clave mostrada: no queda en memoria de la página. -->
  <q-dialog v-model="dialogoCreada" aria-labelledby="titulo-api-creada" @hide="creada = undefined">
    <q-card style="width: 640px; max-width: 95vw">
      <q-card-section>
        <div id="titulo-api-creada" class="titulo-seccion">{{ t('ejecutar.apiCreada', { nombre: creada?.nombre ?? '' }) }}</div>
        <div class="texto-suave text-body2">{{ t('ejecutar.apiCreadaTexto') }}</div>
        <q-banner rounded dense class="aviso-caja q-mt-sm">
          <template #avatar><q-icon name="o_lock" color="warning" /></template>{{ t('puntos.claveUnaVez') }}
        </q-banner>
      </q-card-section>
      <q-card-section class="q-gutter-y-md q-pt-none">
        <div v-for="c in camposCreada" :key="c.titulo">
          <div class="text-caption texto-suave q-mb-xs">{{ c.titulo }}</div>
          <div class="bloque-codigo" :class="{ 'sin-ajuste': c.codigo }">{{ c.valor }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar"
            :aria-label="t('comun.copiar') + ' ' + c.titulo" @click="copiar(c.valor)" /></div>
        </div>
      </q-card-section>
      <q-card-actions align="right" class="q-pa-md"><q-btn v-close-popup unelevated color="primary" no-caps :label="t('comun.hecho')" /></q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { api, avisarError, copiar } from '../api/cliente'
import type { EstadoPunto, Modelo } from '../api/tipos'
import { configDeReceta, esquemaDeReceta, type Receta } from '../recetas'
import { t } from '../i18n'

// creada avisa de la API nueva para que Probar pase a editarla.
const props = defineProps<{ receta: Receta; catalogo: Modelo[] }>()
const emit = defineEmits<{ creada: [nombre: string, punto?: EstadoPunto] }>()
defineExpose({ abrirGuardar })

// Guardar como API crea un punto de acceso con la receta tal como está en Probar.
const dialogoGuardar = ref(false)
const dialogoCreada = ref(false)
const guardando = ref(false)
const nueva = reactive({ nombre: '', grupo: '' })
const creada = ref<{ nombre: string; url: string; clave: string; modelo: string }>()

function abrirGuardar() {
  try {
    esquemaDeReceta(props.receta)
  } catch (e) {
    avisarError(e)
    return
  }
  Object.assign(nueva, { nombre: '', grupo: '' })
  dialogoGuardar.value = true
}

async function guardarComoApi() {
  guardando.value = true
  try {
    const nombre = nueva.nombre.trim()
    const r = await api.crearPunto({ nombre, grupo: nueva.grupo.trim() || undefined, ...configDeReceta(props.receta) })
    const modelo = props.receta.modelo || props.catalogo.find((m) => m.predeterminado)?.id || props.catalogo[0]?.id || ''
    creada.value = { nombre, url: r.punto_acceso?.url ?? '', clave: r.token, modelo }
    dialogoGuardar.value = false
    dialogoCreada.value = true
    emit('creada', nombre, r.punto_acceso)
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

const camposCreada = computed(() => {
  const c = creada.value
  if (!c) return []
  return [
    { titulo: t('conectar.urlOpenai'), valor: c.url + '/openai/v1', codigo: false },
    { titulo: t('conectar.clave'), valor: c.clave, codigo: false },
    { titulo: 'curl', codigo: true, valor: `curl ${c.url}/openai/v1/chat/completions \\
  -H "Authorization: Bearer ${c.clave}" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${c.modelo}", "messages": [{"role": "user", "content": "${t('conectar.hola')}"}]}'` },
  ]
})
</script>
