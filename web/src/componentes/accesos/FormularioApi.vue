<template>
  <q-dialog v-model="nuevo" aria-labelledby="titulo-nueva-api">
    <q-card style="width: 600px; max-width: 95vw">
      <q-card-section>
        <div id="titulo-nueva-api" class="titulo-seccion">{{ t('puntos.nuevo') }}</div>
        <div class="texto-suave text-body2">{{ t('puntos.nuevoTexto') }}</div>
      </q-card-section>
      <q-form @submit="crear">
        <q-card-section class="q-gutter-y-md q-pt-none">
          <q-input v-model="formulario.nombre" outlined dense :label="t('puntos.nombre')" autofocus :hint="t('puntos.nombreHint')"
            :rules="[(v: string) => /^[a-z0-9][a-z0-9_-]{0,39}$/.test(v) || t('puntos.nombreRegla')]" />
          <q-select :model-value="formulario.grupo" :options="sugerencias" outlined dense use-input hide-selected fill-input input-debounce="0"
            :label="t('puntos.grupo')" :hint="t('puntos.grupoHint')" maxlength="40" hide-dropdown-icon
            @input-value="(v: string) => (formulario.grupo = v)" @filter="filtrarGrupos" @update:model-value="(v: string) => (formulario.grupo = v)" />
          <EditorReceta v-model="formulario.receta" :catalogo="catalogo" para-api />
          <q-expansion-item dense icon="o_tune" :label="t('comun.opcionesAvanzadas')" header-class="q-px-none text-body2">
            <div class="q-gutter-y-md q-pt-sm">
              <div class="text-caption texto-suave">{{ t('puntos.avanzadasTexto') }}</div>
              <div>
                <div class="text-caption texto-suave">{{ t('puntos.direccion') }}</div>
                <q-option-group v-model="formulario.direccion" inline
                  :options="[{ label: t('puntos.ruta'), value: 'ruta' }, { label: t('puntos.puertoPropio'), value: 'puerto' }]" />
                <q-input v-if="formulario.direccion === 'puerto'" v-model="formulario.puerto" outlined dense inputmode="numeric" :label="t('puntos.puerto')"
                  class="q-mt-sm" style="max-width: 200px"
                  :rules="[(v: string) => (/^\d+$/.test(v) && +v >= 1024 && +v <= 65535) || t('puntos.puertoRegla')]" />
              </div>
              <div>
                <div class="text-caption texto-suave">{{ t('puntos.compatibilidad') }}</div>
                <q-option-group v-model="formulario.contratos" type="checkbox" inline
                  :options="contratos.map((value) => ({ label: nombreContrato(value), value }))" />
              </div>
              <SelectorModelo v-model="formulario.permitidos" :opciones="opciones" multiple icono="o_rule" :label="t('puntos.permitidos')" :hint="t('puntos.permitidosHint')" />
              <q-input v-model="formulario.alias" type="textarea" autogrow outlined dense :label="t('puntos.alias')" input-class="codigo"
                :hint="t('puntos.aliasHint')" />
              <EditorPresupuesto v-model="formulario.presupuesto" />
            </div>
          </q-expansion-item>
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md">
          <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
          <q-btn unelevated color="primary" no-caps type="submit" :label="t('comun.crear')" :loading="guardando" />
        </q-card-actions>
      </q-form>
    </q-card>
  </q-dialog>

  <q-dialog v-model="dialogoInstrucciones" aria-labelledby="titulo-ajustes-api">
    <q-card style="width: 560px; max-width: 95vw">
      <q-card-section>
        <div id="titulo-ajustes-api" class="titulo-seccion">{{ t('puntos.ajustesDe', { nombre: editando }) }}</div>
      </q-card-section>
      <q-card-section class="q-pt-none q-gutter-y-md">
        <EditorReceta v-model="ajustes.receta" :catalogo="catalogo" para-api />
        <SelectorModelo v-model="ajustes.permitidos" :opciones="opciones" multiple icono="o_rule" :label="t('puntos.permitidos')" :hint="t('puntos.permitidosHint')" />
        <EditorPresupuesto v-model="ajustes.presupuesto" />
      </q-card-section>
      <q-card-actions align="right" class="q-pa-md">
        <q-btn v-close-popup flat no-caps :label="t('comun.cancelar')" />
        <q-btn unelevated color="primary" no-caps :label="t('comun.guardar')" :loading="guardando" @click="guardarAjustes" />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import EditorPresupuesto from '../EditorPresupuesto.vue'
import EditorReceta from '../EditorReceta.vue'
import SelectorModelo from '../SelectorModelo.vue'
import { api, avisar, avisarError } from '../../api/cliente'
import type { ConfigPunto, EstadoPunto, Modelo, Presupuesto } from '../../api/tipos'
import type { OpcionModelo } from '../../agentes'
import { ajustesDeReceta, configDeReceta, presupuestoLimpio, recetaDePunto, recetaVacia } from '../../recetas'
import { t } from '../../i18n'
import { contratos, nombreContrato } from './comun'

// grupos son las sugerencias del campo Grupo; recargar vuelve a pedir las APIs antes de seguir (creada abre el detalle).
const props = defineProps<{ catalogo: Modelo[]; opciones: OpcionModelo[]; grupos: string[]; recargar: () => Promise<void> }>()
const emit = defineEmits<{ creada: [punto: ConfigPunto, respuesta: { punto_acceso?: EstadoPunto; token: string }] }>()
defineExpose({ abrirNuevo, abrirAjustes })

const nuevo = ref(false)
const guardando = ref(false)
const vacio = () => ({ nombre: '', grupo: '', direccion: 'ruta', puerto: '', contratos: [...contratos], permitidos: [] as string[], alias: '',
  receta: recetaVacia(), presupuesto: {} as Presupuesto })
const formulario = reactive(vacio())
const sugerencias = ref<string[]>([])
function filtrarGrupos(texto: string, actualizar: (f: () => void) => void) {
  actualizar(() => {
    sugerencias.value = props.grupos.filter((g) => g.toLowerCase().includes(texto.toLowerCase()))
  })
}

function abrirNuevo() {
  Object.assign(formulario, vacio())
  nuevo.value = true
}

async function crear() {
  const alias: Record<string, string> = {}
  for (const linea of formulario.alias.split('\n').map((l) => l.trim()).filter(Boolean)) {
    const [origen, destino] = linea.split('=').map((p) => p.trim())
    if (!origen || !destino) {
      avisarError(new Error(t('puntos.aliasInvalido', { linea })))
      return
    }
    alias[origen] = destino
  }
  guardando.value = true
  try {
    const punto: ConfigPunto = {
      nombre: formulario.nombre.trim(),
      grupo: formulario.grupo.trim() || undefined,
      puerto: formulario.direccion === 'puerto' ? Number(formulario.puerto) : undefined,
      contratos: formulario.contratos,
      modelos_permitidos: formulario.permitidos.length ? formulario.permitidos : undefined,
      alias_modelos: Object.keys(alias).length ? alias : undefined,
      presupuesto: presupuestoLimpio(formulario.presupuesto),
      ...configDeReceta(formulario.receta), // lanza un error legible si el esquema no es válido
    }
    const r = await api.crearPunto(punto)
    nuevo.value = false
    await props.recargar()
    emit('creada', punto, r)
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

const dialogoInstrucciones = ref(false)
const editando = ref('')
const ajustes = reactive({ receta: recetaVacia(), permitidos: [] as string[], presupuesto: {} as Presupuesto })

function abrirAjustes(p: EstadoPunto) {
  editando.value = p.nombre
  Object.assign(ajustes, { receta: recetaDePunto(p), permitidos: [...(p.modelos_permitidos ?? [])],
    presupuesto: JSON.parse(JSON.stringify(p.presupuesto ?? {})) as Presupuesto })
  dialogoInstrucciones.value = true
}

async function guardarAjustes() {
  guardando.value = true
  try {
    await api.ajustesPunto(editando.value, { ...ajustesDeReceta(ajustes.receta), modelos_permitidos: ajustes.permitidos,
      presupuesto: presupuestoLimpio(ajustes.presupuesto) ?? null })
    dialogoInstrucciones.value = false
    avisar(t('puntos.ajustesGuardados'))
    await props.recargar()
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}
</script>
