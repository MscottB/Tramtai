<template>
  <q-dialog v-model="abierto" :maximized="$q.screen.xs" aria-labelledby="titulo-acerca">
    <q-card class="column no-wrap" style="width: 560px; max-width: 100vw">
      <q-card-section class="row items-center no-wrap q-gutter-x-md">
        <img src="/logo.png" alt="" width="56" height="56" style="border-radius: 14px" />
        <div class="col">
          <div id="titulo-acerca" class="titulo-seccion">{{ t('acerca.titulo') }}</div>
          <div class="text-body2 codigo">{{ version }}<span v-if="datos?.portable" class="texto-suave"> · {{ t('acerca.portable') }}</span></div>
          <div class="text-caption texto-suave">
            {{ t('acerca.desarrollado') }} <a class="enlace" :href="autor" target="_blank" rel="noopener noreferrer">Michael Scott B.</a>
          </div>
        </div>
        <q-btn v-close-popup flat round icon="close" :aria-label="t('comun.cerrar')" class="self-start" />
      </q-card-section>

      <q-card-section class="col scroll q-pt-none q-gutter-y-md">
        <q-banner rounded dense :class="{ 'info-caja': actualizacion?.disponible }">
          <template #avatar><q-icon :name="actualizacion?.disponible ? 'o_system_update_alt' : 'o_verified'" color="primary" /></template>
          {{ estadoActualizacion }}
          <template #action>
            <q-btn v-if="actualizacion?.disponible" flat no-caps color="primary" :label="t('actualizacion.boton')" @click="actualizar" />
            <q-btn v-else flat no-caps color="primary" :label="t('acerca.buscarAhora')" :loading="buscando" @click="buscar" />
            <q-btn flat no-caps color="primary" icon-right="o_open_in_new" :label="t('acerca.novedades')" :href="urlNovedades" target="_blank" rel="noopener noreferrer" />
          </template>
        </q-banner>

        <q-list bordered separator class="rounded-borders">
          <q-item v-for="e in enlaces" :key="e.etiqueta" clickable tag="a" :href="e.url" target="_blank" rel="noopener noreferrer">
            <q-item-section avatar><q-icon :name="e.icono" color="primary" /></q-item-section>
            <q-item-section>
              <q-item-label>{{ e.etiqueta }}</q-item-label>
              <q-item-label v-if="e.detalle" caption>{{ e.detalle }}</q-item-label>
            </q-item-section>
            <q-item-section side><q-icon name="o_open_in_new" size="16px" /></q-item-section>
          </q-item>
        </q-list>

        <q-expansion-item dense icon="o_info" :label="t('acerca.detalles')" header-class="q-px-none text-body2">
          <div v-if="!datos" class="text-caption texto-suave q-py-sm">{{ fallo ? t('acerca.sinDatos') : t('comun.cargando') }}</div>
          <template v-else>
            <div class="ficha text-body2 q-py-sm">
              <template v-for="d in detalles" :key="d[0]">
                <div>{{ d[0] }}</div>
                <a v-if="d[2]" class="enlace codigo romper" :href="d[2]" target="_blank" rel="noopener noreferrer">{{ d[1] }}</a>
                <div v-else class="codigo romper">{{ d[1] }}</div>
              </template>
            </div>
            <q-btn outline no-caps color="primary" icon="o_content_copy" :label="t('acerca.copiar')" @click="copiar(textoSoporte)" />
            <div class="text-caption texto-suave q-mt-xs">{{ t('acerca.copiarTexto') }}</div>
          </template>
        </q-expansion-item>
      </q-card-section>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
// AcercaDe muestra la versión, quién la desarrolla, el estado de las actualizaciones, los enlaces del proyecto y los
// datos técnicos para pedir ayuda (sin claves ni tokens).
import { computed, ref, watch } from 'vue'
import { api, buscarActualizacionAhora, copiar } from '../api/cliente'
import type { Acerca } from '../api/tipos'
import { actualizacion, dialogoActualizar, sesion } from '../estado'
import { fecha } from '../formato'
import { t } from '../i18n'

const abierto = defineModel<boolean>({ required: true })

const repositorio = 'https://github.com/MscottB/Tramtai'
const autor = 'https://github.com/MscottB'
const datos = ref<Acerca>()
const fallo = ref(false)
const buscando = ref(false)

watch(abierto, async (si) => {
  if (!si) return
  try {
    datos.value = await api.acerca()
    fallo.value = false
  } catch {
    fallo.value = true
  }
})

const version = computed(() => datos.value?.version ?? sesion.value?.version ?? '')
// Una versión publicada lleva su fecha de compilación (+DDMMAAAA) y tiene página propia; una compilación de desarrollo
// no, y se enlaza la lista de versiones.
const urlNovedades = computed(() => `${repositorio}/releases` + (/\+\d{8}$/.test(version.value) ? `/tag/${encodeURIComponent(version.value)}` : ''))

const estadoActualizacion = computed(() => {
  const a = actualizacion.value
  if (a?.disponible) return t('acerca.hayNueva', { version: a.version })
  if (a?.fallo) return t('actualizacion.falloConsulta')
  if (!a) return t('acerca.sinComprobar')
  if (!a.buscar) return t('acerca.sinBuscar')
  return t('acerca.alDia')
})
async function buscar() {
  buscando.value = true
  try {
    await buscarActualizacionAhora()
  } finally {
    buscando.value = false
  }
}
function actualizar() {
  abierto.value = false
  dialogoActualizar.value = true
}

const enlaces = computed(() => [
  { etiqueta: t('acerca.sitio'), detalle: 'mscottb.github.io/Tramtai', url: 'https://mscottb.github.io/Tramtai/', icono: 'o_public' },
  { etiqueta: t('acerca.repositorio'), detalle: 'github.com/MscottB/Tramtai', url: repositorio, icono: 'o_code' },
  { etiqueta: t('acerca.problema'), detalle: t('acerca.problemaDetalle'), url: `${repositorio}/issues/new`, icono: 'o_bug_report' },
  { etiqueta: t('acerca.licencia'), detalle: 'PolyForm Noncommercial 1.0.0', url: `${repositorio}/blob/main/LICENSE.md`, icono: 'o_gavel' },
  { etiqueta: t('acerca.terminos'), detalle: '', url: `${repositorio}/blob/main/TERMINOS.md`, icono: 'o_description' },
])

const sistemas: Record<string, string> = { windows: 'Windows', darwin: 'macOS', linux: 'Linux' }
// detalles: [etiqueta, valor, enlace opcional]; son también el texto que se copia para soporte.
const detalles = computed<[string, string, string?][]>(() => {
  const d = datos.value
  if (!d) return []
  const lista: [string, string, string?][] = [[t('acerca.version'), d.version + (d.portable ? ` (${t('acerca.portable')})` : '')]]
  if (d.commit) {
    lista.push([t('acerca.commit'), d.commit.slice(0, 12) + (d.modificado ? ` (${t('acerca.modificado')})` : ''), `${repositorio}/commit/${d.commit}`])
  }
  lista.push([t('acerca.sistema'), `${sistemas[d.sistema] ?? d.sistema} · ${d.arquitectura}`], ['Go', d.go],
    [t('acerca.servicio'), `${d.servicio.direccion} · pid ${d.servicio.pid} · ${t('acerca.desde', { fecha: fecha(d.servicio.iniciado_en) })}`],
    [t('acerca.claves'), d.almacen])
  if (d.rutas) {
    lista.push([t('acerca.datos'), d.rutas.datos], [t('acerca.configuracion'), d.rutas.configuracion], [t('acerca.registro'), d.rutas.registro])
  }
  return lista
})
// Se pega en una incidencia pública: la carpeta del usuario va como ~ para no publicar su nombre (también con el
// nombre corto de Windows, como ADMINI~1).
const carpetaUsuario = /(?:[A-Za-z]:\\Users\\|\/home\/|\/Users\/)[^\\/\n]+/g
const textoSoporte = computed(() => ['Tramtai', ...detalles.value.map((d) => `${d[0]}: ${d[1]}`)].join('\n').replace(carpetaUsuario, '~'))
</script>
