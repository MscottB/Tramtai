<template>
  <div class="pagina apilado">
    <CabeceraPagina tema="espacios" :titulo="t('menu.espacios')" :descripcion="t('espacios.subtitulo')">
      <template #acciones>
        <q-btn data-recorrido="espacios.actualizar" flat no-caps color="primary" icon="o_refresh" :label="t('espacios.actualizar')" :loading="cargando" @click="cargar" />
        <q-btn data-recorrido="espacios.abrir" unelevated no-caps color="primary" icon="o_create_new_folder" :label="t('proyecto.abrir')" @click="$emit('abrir-proyecto')" />
      </template>
    </CabeceraPagina>

    <div v-if="lista" class="apilado" data-captura="espacios" data-recorrido="espacios.lista">
      <q-card v-for="e in lista" :key="e.id" flat bordered>
        <q-card-section>
          <div style="min-width: 0">
            <!-- Estados como el resto de la app: punto y texto (.etiqueta), sin insignias de Quasar ni icono de cabecera. -->
            <div class="row items-center q-gutter-x-sm q-gutter-y-xs">
              <span class="text-weight-medium">{{ e.nombre }}</span>
              <span :class="'etiqueta etiqueta--' + claseEstado[e.estado]">{{ t(textoEstado[e.estado]) }}</span>
              <span v-if="e.ambito === 'proyecto' && e.estado !== 'no_encontrado'" class="etiqueta" :class="e.confiado ? 'etiqueta--ok' : 'etiqueta--aviso'">
                {{ e.confiado ? t('espacios.confiado') : t('espacios.sinConfiar') }}</span>
              <span v-if="e.id === sesion?.instancia" class="etiqueta etiqueta--info">{{ t('espacios.actual') }}</span>
            </div>
            <div class="text-caption texto-suave ellipsis q-mt-xs" :class="{ codigo: e.ruta }">{{ e.ruta || t('app.sinProyecto') }}</div>
            <div class="metadatos q-mt-xs">
              <span><q-icon name="o_hub" />{{ e.apis === 1 ? t('espacios.apisUno') : t('espacios.apis', { n: e.apis }) }}</span>
              <span><q-icon name="o_history" />{{ e.peticiones === 1 ? t('espacios.peticionesUno') : t('espacios.peticiones', { n: numero(e.peticiones) }) }}</span>
              <span v-if="e.ultimo_uso"><q-icon name="o_schedule" />{{ t('espacios.ultimoUso', { cuando: haceCuanto(e.ultimo_uso) }) }}</span>
            </div>
            <div v-if="e.ultimo_error" class="aviso text-body2 q-mt-sm">{{ e.ultimo_error }}</div>
          </div>
        </q-card-section>
        <q-card-actions class="q-px-md q-pb-md q-pt-none">
          <q-btn v-for="a in accionesDe(e, sesion?.instancia ?? '')" :key="a" no-caps :flat="a !== principal(e)" :unelevated="a === principal(e)"
            :color="a === 'olvidar' ? 'negative' : 'primary'" :label="t(textoAccion[a])" :loading="ocupado === e.id + a" @click="hacer(e, a)" />
        </q-card-actions>
      </q-card>
    </div>
    <q-skeleton v-else type="rect" height="160px" />
  </div>
</template>

<script setup lang="ts">
// Espacios lista todos los espacios de trabajo (también los detenidos y los que ya no encuentran su carpeta) y deja
// abrirlos, activarlos, detenerlos, confiar en ellos, reasociarlos u olvidarlos.
import { onMounted, ref } from 'vue'
import { useQuasar } from 'quasar'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import { api, avisar, avisarError } from '../api/cliente'
import type { EspacioUI } from '../api/tipos'
import { accionesDe, nombreConfirmado, ordenar, type Accion } from '../espacios'
import { sesion } from '../estado'
import { haceCuanto, numero } from '../formato'
import { t } from '../i18n'
import type { Clave } from '../idiomas/es'

const emit = defineEmits<{ ir: [pagina: string]; 'abrir-proyecto': [] }>()
const $q = useQuasar()
const lista = ref<EspacioUI[]>()
const cargando = ref(false)
const ocupado = ref('')

const textoEstado: Record<EspacioUI['estado'], Clave> = { activo: 'espacios.estadoActivo', detenido: 'espacios.estadoDetenido', no_encontrado: 'espacios.estadoNoEncontrado' }
const claseEstado: Record<EspacioUI['estado'], string> = { activo: 'ok', detenido: 'neutro', no_encontrado: 'error' }
const textoAccion: Record<Accion, Clave> = { abrir: 'espacios.abrir', detener: 'espacios.detener', activar: 'espacios.activar',
  buscar: 'espacios.buscar', olvidar: 'espacios.olvidar', confiar: 'espacios.confiar', dejarDeConfiar: 'espacios.dejarDeConfiar' }

// principal es la acción que se destaca en cada tarjeta.
function principal(e: EspacioUI): Accion | undefined {
  return accionesDe(e, sesion.value?.instancia ?? '').find((a) => a === 'abrir' || a === 'activar' || a === 'buscar')
}

async function cargar() {
  cargando.value = true
  try {
    lista.value = ordenar(await api.espacios())
  } catch (e) {
    avisarError(e)
  } finally {
    cargando.value = false
  }
}

// confirmar pide el visto bueno (o un texto, con prompt) en un diálogo; devuelve undefined si se cancela.
function confirmar(titulo: string, mensaje: string, prompt?: { valido?: (v: string) => boolean }): Promise<string | undefined> {
  return new Promise((resolver) => {
    $q.dialog({ title: titulo, message: mensaje, cancel: { flat: true, noCaps: true, label: t('comun.cancelar') }, persistent: false,
      ok: { unelevated: true, noCaps: true, label: t('comun.aceptar') },
      ...(prompt ? { prompt: { model: '', type: 'text', isValid: prompt.valido ?? ((v: string) => v.trim() !== '') } } : {}) })
      .onOk((v?: string) => resolver(v ?? ''))
      .onCancel(() => resolver(undefined))
      .onDismiss(() => resolver(undefined))
  })
}

async function hacer(e: EspacioUI, accion: Accion) {
  ocupado.value = e.id + accion
  try {
    switch (accion) {
      case 'abrir':
        await api.cambiarInstancia(e.id)
        emit('ir', 'inicio')
        return
      case 'activar':
        lista.value = ordenar(await api.activarEspacio(e.id))
        break
      case 'detener':
        if ((await confirmar(t('espacios.detenerTitulo', { nombre: e.nombre }), t('espacios.detenerTexto'))) === undefined) return
        lista.value = ordenar(await api.detenerEspacio(e.id))
        break
      case 'confiar':
        if ((await confirmar(t('espacios.confiarTitulo', { nombre: e.nombre }), t('espacios.confiarTexto')
          + (e.claves?.length ? ' ' + t('espacios.confiarClaves', { claves: e.claves.join(', ') }) : ''))) === undefined) return
        lista.value = ordenar(await api.confiarEspacio(e.id, true))
        break
      case 'dejarDeConfiar':
        lista.value = ordenar(await api.confiarEspacio(e.id, false))
        break
      case 'buscar': {
        const ruta = await confirmar(t('espacios.buscarTitulo', { nombre: e.nombre }), t('espacios.buscarTexto'), {})
        if (ruta === undefined) return
        lista.value = ordenar(await api.ubicarEspacio(e.id, ruta.trim()))
        break
      }
      case 'olvidar': {
        const nombre = await confirmar(t('espacios.olvidarTitulo', { nombre: e.nombre }), t('espacios.olvidarTexto', { nombre: e.nombre }),
          { valido: (v) => nombreConfirmado(e, v) })
        if (nombre === undefined) return
        lista.value = ordenar(await api.olvidarEspacio(e.id, nombre))
        avisar(t('espacios.olvidado', { nombre: e.nombre }))
        break
      }
    }
    await api.sesion() // el selector de arriba se pone al día
  } catch (err) {
    avisarError(err)
  } finally {
    ocupado.value = ''
  }
}

onMounted(cargar)
</script>
