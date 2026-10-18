<template>
  <div class="pagina apilado">
    <CabeceraPagina :titulo="t('menu.conectar')" :descripcion="t('conectar.subtitulo')">
      <template #acciones>
        <q-btn unelevated color="primary" icon="add" :label="t('puntos.nuevo')" no-caps @click="abrirNuevo" />
      </template>
    </CabeceraPagina>


    <q-card>
      <q-card-section>
        <div class="titulo-seccion">{{ t('puntos.titulo') }}</div>
        <div class="texto-suave text-body2">{{ t('puntos.texto') }}</div>
      </q-card-section>
      <q-card-section v-if="puntos.length > 1 || busqueda || filtroGrupo !== null" class="row q-col-gutter-sm q-pt-none">
        <div class="col-12 col-sm">
          <q-input v-model="busqueda" dense outlined clearable debounce="150" :placeholder="t('puntos.buscarApi')" :aria-label="t('puntos.buscarApi')">
            <template #prepend><q-icon name="search" /></template>
          </q-input>
        </div>
        <div v-if="hayGrupos" class="col-6 col-sm-3">
          <q-select v-model="filtroGrupo" :options="opcionesGrupo" emit-value map-options dense outlined :label="t('puntos.filtroGrupo')" />
        </div>
        <div class="col-6 col-sm-3">
          <q-select v-model="orden" :options="opcionesOrden" emit-value map-options dense outlined :label="t('puntos.orden')" />
        </div>
      </q-card-section>
      <template v-if="presupuestoEspacio">
        <q-separator />
        <q-card-section>
          <div class="text-weight-medium">{{ t('presupuesto.espacio') }}</div>
          <div class="text-caption texto-suave">{{ t('presupuesto.espacioTexto') }}</div>
          <BarrasPresupuesto :estado="presupuestoEspacio" @reiniciar="reiniciarPresupuesto()" />
        </q-card-section>
      </template>
      <q-separator />
      <template v-if="puntos.length">
        <div v-for="g in grupos" :key="g.nombre">
          <div v-if="hayGrupos" class="row items-center q-px-md q-pt-md q-pb-xs cabecera-grupo">
            <q-btn flat dense round size="sm" :icon="plegados.has(g.nombre) ? 'chevron_right' : 'expand_more'" class="q-mr-xs"
              :aria-expanded="!plegados.has(g.nombre)" :aria-label="t(plegados.has(g.nombre) ? 'puntos.desplegar' : 'puntos.plegar', { grupo: g.nombre || t('puntos.sinGrupo') })"
              @click="plegar(g.nombre)" />
            <q-icon :name="g.nombre ? 'o_folder' : 'o_folder_off'" class="q-mr-sm texto-suave" />
            <div class="text-weight-medium cursor-pointer" @click="plegar(g.nombre)">{{ g.nombre || t('puntos.sinGrupo') }}</div>
            <div class="text-caption texto-suave q-ml-sm">{{ t('puntos.activos', { activos: g.activos, total: g.total }) }}</div>
            <q-space />
            <q-btn flat dense no-caps size="sm" color="primary" icon="o_play_arrow" :label="$q.screen.gt.xs ? t('puntos.activarTodos') : undefined"
              :aria-label="t('puntos.activarTodos')" :disable="g.activos === g.total" :loading="ocupado === g.nombre + ':1'" @click="activarGrupo(g.nombre, true)">
              <q-tooltip v-if="$q.screen.xs">{{ t('puntos.activarTodos') }}</q-tooltip>
            </q-btn>
            <q-btn flat dense no-caps size="sm" color="grey-8" icon="o_stop" :label="$q.screen.gt.xs ? t('puntos.desactivarTodos') : undefined"
              :aria-label="t('puntos.desactivarTodos')" :disable="g.activos === 0" :loading="ocupado === g.nombre + ':0'" @click="activarGrupo(g.nombre, false)">
              <q-tooltip v-if="$q.screen.xs">{{ t('puntos.desactivarTodos') }}</q-tooltip>
            </q-btn>
          </div>
          <div v-if="!hayGrupos || !plegados.has(g.nombre)">
            <div v-for="p in g.puntos" :key="p.nombre" class="fila-api cursor-pointer" @click="abrirDetalle(p)">
              <span class="linea__chapa linea__chapa--grande" :class="['linea--' + (p.agente ?? 'otro'), { 'linea__chapa--apagada': !activo(p) }]" aria-hidden="true">{{ p.agente ? codigoLinea(p.agente) : 'API' }}</span>
              <div class="fila-api__info">
                <div class="row items-center q-gutter-x-sm">
                  <span class="text-weight-medium">{{ p.nombre }}</span>
                  <span class="etiqueta" :class="!activo(p) ? 'etiqueta--neutro' : p.error ? 'etiqueta--error' : 'etiqueta--ok'">{{ activo(p) ? t('puntos.activo') : t('puntos.desactivado') }}</span>
                  <span v-if="p.esquema" class="etiqueta etiqueta--info">{{ t('puntos.json') }}</span>
                  <span v-if="p.respaldo?.length" class="etiqueta etiqueta--neutro">{{ t('puntos.respaldoDe', { agentes: p.respaldo.map(etiquetaDe).join(', ') }) }}</span>
                </div>
                <div class="row items-center no-wrap codigo texto-suave">
                  <span class="ellipsis">{{ p.url || t('puntos.sinDireccion') }}</span>
                  <q-btn v-if="p.url" flat dense round size="sm" icon="content_copy" :aria-label="t('puntos.copiarUrl')" @click.stop="copiar(p.url)"><q-tooltip>{{ t('puntos.copiarUrl') }}</q-tooltip></q-btn>
                </div>
                <div class="text-caption texto-suave">
                  <template v-if="idModelo(p)">{{ etiquetaDe(idModelo(p)!) }} · </template>{{ p.contratos.map(nombreContrato).join(' · ') }}
                  <template v-if="p.esfuerzo"> · {{ t('esfuerzo.titulo') }} {{ nombreEsfuerzo(p.esfuerzo).toLowerCase() }}</template>
                </div>
                <div v-if="p.instrucciones" class="text-caption texto-suave row no-wrap items-start">
                  <q-icon name="o_notes" class="q-mr-xs q-mt-xs" /><span class="recortar-2">{{ p.instrucciones }}</span>
                </div>
                <div v-if="p.error && p.error !== 'desactivado'" class="text-caption text-negative">{{ p.error }}</div>
              </div>
              <div class="fila-api__uso text-caption">
                <a v-if="uso[p.nombre]" href="#historial" class="enlace" @click.prevent.stop="verPeticiones(p.nombre)">{{ textoUso(p.nombre) }}</a>
                <span v-else class="texto-suave">{{ t('puntos.sinUso') }}</span>
                <BarrasPresupuesto v-if="presupuestos[p.nombre]" :estado="presupuestos[p.nombre]" sin-boton />
              </div>
              <div class="fila-api__acciones row items-center no-wrap q-gutter-x-xs">
                <q-btn unelevated dense no-caps color="primary" icon="o_cable" class="q-px-sm" :label="t('puntos.conectar')" @click.stop="abrirDetalle(p)" />
              </div>
            </div>
          </div>
        </div>
      </template>
      <div v-if="puntos.length && !grupos.length" class="q-pa-md texto-suave">{{ t('puntos.sinResultados') }}</div>
      <EstadoVacio v-if="!puntos.length" :titulo="t('puntos.vacio')" :accion="t('puntos.nuevo')" @accion="abrirNuevo" />
    </q-card>

    <q-card>
      <q-card-section class="seccion">
        <div>
          <div class="titulo-seccion">{{ t('conectar.directa') }}</div>
          <div class="texto-suave text-body2">{{ t('conectar.directaTexto') }}</div>
        </div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <GuiaUrls :url="base" :contratos="contratos" />
      </q-card-section>
      <q-card-section class="row q-col-gutter-md q-pt-none">
        <div class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
          <div class="bloque-codigo" style="padding-right: 112px">{{ clave ? (verClave ? clave : ocultar(clave)) : t('comun.cargando') }}
            <div class="boton-copiar row no-wrap">
              <q-btn flat dense round size="sm" :icon="verClave ? 'visibility_off' : 'visibility'" :aria-label="verClave ? t('conectar.ocultarClave') : t('conectar.mostrarClave')" @click="verClave = !verClave" />
              <q-btn flat dense round size="sm" icon="content_copy" :aria-label="t('conectar.copiarClave')" :disable="!clave" @click="copiar(clave, t('conectar.claveCopiada'))" />
              <q-btn flat dense round size="sm" icon="o_autorenew" :aria-label="t('conectar.regenerarClave')" :disable="!clave" @click="regenerarClaveInstancia">
                <q-tooltip>{{ t('conectar.regenerarClave') }}</q-tooltip>
              </q-btn>
            </div>
          </div>
        </div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.modeloAntes') }} <span class="codigo">model</span> {{ t('conectar.modeloDespues') }}</div>
        <div v-for="g in gruposModelo" :key="g.titulo" class="row items-center q-gutter-xs q-mb-xs">
          <span class="grupo-modelo">
            <span v-if="g.agente" class="linea__chapa" :class="'linea--' + g.agente" aria-hidden="true">{{ codigoLinea(g.agente) }}</span>{{ g.titulo }}
          </span>
          <q-chip v-for="m in g.ids" :key="m" dense clickable outline color="primary" class="codigo" @click="copiar(m)">{{ m }}</q-chip>
        </div>
      </q-card-section>
      <q-separator />
      <div class="q-px-md q-pt-md text-weight-medium">{{ t('conectar.ejemplos') }}</div>
      <div class="q-px-md q-pt-sm">
        <div class="desplazable-x"><q-btn-toggle v-model="lenguaje" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
          :options="ejemplos.map((e) => ({ label: e.titulo(), value: e.id }))" /></div>
      </div>
      <q-card-section>
        <div class="bloque-codigo sin-ajuste">{{ ejemplo.codigo(verClave ? clave : ocultar(clave)) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarEjemplo')" @click="copiar(ejemplo.codigo(clave), t('conectar.ejemploCopiado'))" /></div>
      </q-card-section>
    </q-card>

    <q-dialog v-model="nuevo">
      <q-card style="width: 600px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ t('puntos.nuevo') }}</div>
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

    <q-dialog v-model="dialogoInstrucciones">
      <q-card style="width: 560px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ t('puntos.ajustesDe', { nombre: editando }) }}</div>
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

    <!-- Detalle de una API: cómo conectarla, sus acciones y la zona de peligro. Al cerrarse se olvida la clave
         mostrada: no queda en memoria de la página. -->
    <q-dialog v-model="dialogoDetalle" :maximized="$q.screen.lt.sm" aria-labelledby="titulo-detalle" @hide="claveVista = ''">
      <q-card v-if="puntoDetalle" style="width: 760px; max-width: 96vw">
        <q-card-section class="row items-center no-wrap q-gutter-x-sm">
          <span class="linea__chapa linea__chapa--grande" :class="['linea--' + (puntoDetalle.agente ?? 'otro'), { 'linea__chapa--apagada': !activo(puntoDetalle) }]" aria-hidden="true">{{ puntoDetalle.agente ? codigoLinea(puntoDetalle.agente) : 'API' }}</span>
          <div class="col">
            <div id="titulo-detalle" class="titulo-seccion">{{ recienCreado ? t('puntos.listo') : puntoDetalle.nombre }}</div>
            <div class="row items-center q-gutter-x-sm text-caption texto-suave">
              <span class="etiqueta" :class="!activo(puntoDetalle) ? 'etiqueta--neutro' : puntoDetalle.error ? 'etiqueta--error' : 'etiqueta--ok'">{{ activo(puntoDetalle) ? t('puntos.activo') : t('puntos.desactivado') }}</span>
              <span v-if="puntoDetalle.grupo"><q-icon name="o_folder" /> {{ puntoDetalle.grupo }}</span>
            </div>
          </div>
          <q-btn v-close-popup flat round dense icon="close" :aria-label="t('comun.cerrar')" />
        </q-card-section>
        <q-separator />
        <q-card-section>
          <div class="text-weight-medium">{{ t('puntos.conectar') }}</div>
          <div class="texto-suave text-body2 q-mb-sm">{{ t('puntos.conectarTexto') }}</div>
          <template v-if="puntoDetalle.url">
          <GuiaUrls :url="puntoDetalle.url" :contratos="puntoDetalle.contratos" :nombre="puntoDetalle.nombre" class="q-mb-md" />
          <div class="row q-col-gutter-md">
            <div class="col-12">
              <div class="text-caption texto-suave q-mb-xs">{{ t('conectar.clave') }}</div>
              <template v-if="claveVista">
                <div class="bloque-codigo">{{ claveVista }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarClave')" @click="copiar(claveVista, t('conectar.claveCopiada'))" /></div>
                <q-banner rounded dense class="aviso-caja q-mt-sm">
                  <template #avatar><q-icon name="o_lock" color="warning" /></template>{{ t('puntos.claveUnaVez') }}
                </q-banner>
              </template>
              <div v-else class="row items-center q-gutter-sm">
                <span class="text-body2 texto-suave col-12 col-sm">{{ t('puntos.claveOculta') }}</span>
                <q-btn outline dense no-caps color="primary" icon="o_key" class="q-px-sm" :label="t('puntos.regenerarClave')" @click="regenerarClave(puntoDetalle)" />
              </div>
            </div>
          </div>
          <template v-if="ejemplosDetalle.length">
          <div class="text-caption texto-suave q-mt-md q-mb-xs">{{ t('conectar.ejemplos') }}</div>
          <div class="desplazable-x"><q-btn-toggle v-model="lenguaje" no-caps flat dense toggle-color="primary" class="segmentado inline-flex"
            :options="ejemplosDetalle.map((e) => ({ label: e.titulo(), value: e.id }))" /></div>
          <div class="bloque-codigo sin-ajuste q-mt-sm">{{ ejemploDetalle.codigo(claveVista || t('puntos.claveMarcador', { nombre: puntoDetalle.nombre })) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="t('conectar.copiarEjemplo')" @click="copiar(ejemploDetalle.codigo(claveVista || t('puntos.claveMarcador', { nombre: puntoDetalle.nombre })))" /></div>
          </template>
          </template>
          <div v-else class="row items-center q-gutter-sm">
            <span class="text-body2 texto-suave col-12 col-sm">{{ t('puntos.activaParaConectar') }}</span>
            <q-btn outline dense no-caps color="primary" icon="o_play_arrow" class="q-px-sm" :label="t('puntos.accionActivar')" @click="activar(puntoDetalle.nombre, true)" />
          </div>
        </q-card-section>
        <q-separator />
        <q-card-section>
          <div class="text-weight-medium q-mb-sm">{{ t('puntos.acciones') }}</div>
          <div class="row q-gutter-sm">
            <q-btn outline no-caps color="primary" icon="o_tune" :label="t('puntos.ajustes')" @click="abrirAjustes(puntoDetalle)" />
            <q-btn outline no-caps color="primary" icon="o_science" :label="t('puntos.abrirProbar')" @click="abrirEnProbar(puntoDetalle.nombre)" />
            <q-btn outline no-caps color="primary" icon="o_history" :label="t('puntos.verPeticiones')" @click="verPeticiones(puntoDetalle.nombre)" />
            <q-btn outline no-caps color="primary" :icon="activo(puntoDetalle) ? 'o_pause' : 'o_play_arrow'"
              :label="activo(puntoDetalle) ? t('puntos.accionDesactivar') : t('puntos.accionActivar')" @click="activar(puntoDetalle.nombre, !activo(puntoDetalle))" />
            <q-btn v-if="presupuestos[puntoDetalle.nombre]" outline no-caps color="primary" icon="o_restart_alt" :label="t('presupuesto.reiniciar')" @click="reiniciarPresupuesto(puntoDetalle.nombre)" />
          </div>
        </q-card-section>
        <q-separator />
        <q-card-section>
          <div class="text-weight-medium text-negative">{{ t('puntos.peligro') }}</div>
          <div class="row items-center q-gutter-sm q-mt-xs">
            <span class="text-body2 texto-suave col-12 col-sm">{{ t('puntos.peligroTexto') }}</span>
            <q-btn outline no-caps color="negative" icon="o_delete" :label="t('puntos.eliminarApi')" @click="eliminar(puntoDetalle.nombre)" />
          </div>
        </q-card-section>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useQuasar } from 'quasar'
import BarrasPresupuesto from '../componentes/BarrasPresupuesto.vue'
import CabeceraPagina from '../componentes/CabeceraPagina.vue'
import EstadoVacio from '../componentes/EstadoVacio.vue'
import EditorPresupuesto from '../componentes/EditorPresupuesto.vue'
import EditorReceta from '../componentes/EditorReceta.vue'
import GuiaUrls from '../componentes/GuiaUrls.vue'
import SelectorModelo from '../componentes/SelectorModelo.vue'
import { ajustesDeReceta, api, avisar, avisarError, codigoLinea, nombreAgente, ordenAgentes, configDeReceta, copiar, dividirRespaldo, etiquetaModelo, haceCuanto, idModeloDe as idModelo, nombreEsfuerzo, opcionModelo,
  cargarDiasHistorial, periodoHistorial, grupoInicial, origenHistorial, presupuestoLimpio, recetaDePunto, recetaInicial, recetaVacia, type ConfigPunto, type EstadoPresupuesto, type EstadoPunto, type Modelo,
  type Presupuesto, type UsoPunto } from '../api'
import { ejemplos as ejemplosDe } from '../ejemplos'
import { t } from '../i18n'

const $q = useQuasar()
const base = ref(location.origin)
const clave = ref('')
const verClave = ref(false)
const catalogo = ref<Modelo[]>([])
const ids = computed(() => catalogo.value.map((m) => m.id))
const puntos = ref<EstadoPunto[]>([])
const nuevo = ref(false)
const guardando = ref(false)
const ocupado = ref('')
const dialogoDetalle = ref(false)
const recienCreado = ref(false)
// detalle es la API abierta; claveVista, su clave recién creada o regenerada (solo se ve esta vez).
const detalle = ref('')
const claveVista = ref('')
// respaldoDetalle es la API con que se abrió el detalle, por si la lista aún no la trae (recién creada y sin recargar).
const respaldoDetalle = ref<EstadoPunto>()
const puntoDetalle = computed(() => puntos.value.find((p) => p.nombre === detalle.value) ?? respaldoDetalle.value)
// Si la API desaparece (borrada desde otro sitio), el detalle se cierra en vez de quedar vacío.
watch(puntos, (lista) => {
  if (dialogoDetalle.value && !lista.some((p) => p.nombre === detalle.value)) {
    if (respaldoDetalle.value?.nombre !== detalle.value) dialogoDetalle.value = false
  } else respaldoDetalle.value = undefined
})
function abrirDetalle(p: EstadoPunto, clave = '', creada = false) {
  respaldoDetalle.value = p
  detalle.value = p.nombre
  claveVista.value = clave
  recienCreado.value = creada
  dialogoDetalle.value = true
}
const lenguaje = ref('python')
const contratos = ['openai', 'anthropic', 'nativa']
const nombreContrato = (c: string) => ({ openai: 'OpenAI', anthropic: 'Anthropic', nativa: t('contrato.nativa') })[c] ?? c
const activo = (p: EstadoPunto) => p.error !== 'desactivado'

// leer y guardar recuerdan en este navegador los grupos plegados y el orden; sin almacenamiento, lo predeterminado.
function leer<T>(clave: string, porDefecto: T): T {
  try {
    const v = localStorage.getItem(clave)
    return v === null ? porDefecto : (JSON.parse(v) as T)
  } catch {
    return porDefecto
  }
}
function guardar(clave: string, valor: unknown) {
  try {
    localStorage.setItem(clave, JSON.stringify(valor))
  } catch {
    // sin almacenamiento solo no se recuerda
  }
}
const busqueda = ref<string | null>('')
const filtroGrupo = ref<string | null>(null) // null: todos; '': sin grupo
const orden = ref<'nombre' | 'uso' | 'ultima'>(leer('tramtai.ordenApis', 'nombre'))
watch(orden, (v) => guardar('tramtai.ordenApis', v))
const plegados = ref(new Set<string>(leer<string[]>('tramtai.gruposPlegados', [])))
function plegar(grupo: string) {
  const nuevos = new Set(plegados.value)
  if (!nuevos.delete(grupo)) nuevos.add(grupo)
  plegados.value = nuevos
  guardar('tramtai.gruposPlegados', [...nuevos])
}
const opcionesOrden = computed(() => [
  { label: t('puntos.ordenNombre'), value: 'nombre' }, { label: t('puntos.ordenUso'), value: 'uso' }, { label: t('puntos.ordenUltima'), value: 'ultima' },
])
const comparar = (a: EstadoPunto, b: EstadoPunto) => {
  if (orden.value === 'uso') return (uso.value[b.nombre]?.peticiones ?? 0) - (uso.value[a.nombre]?.peticiones ?? 0) || a.nombre.localeCompare(b.nombre)
  if (orden.value === 'ultima') return (uso.value[b.nombre]?.ultima ?? '').localeCompare(uso.value[a.nombre]?.ultima ?? '') || a.nombre.localeCompare(b.nombre)
  return a.nombre.localeCompare(b.nombre)
}
// grupos ordena los puntos por grupo (los que no tienen, al final), aplica el buscador, el grupo elegido y el orden;
// activos y total cuentan el grupo entero, porque activar o detener todas actúa sobre todo el grupo.
const grupos = computed(() => {
  const mapa = new Map<string, EstadoPunto[]>()
  for (const p of puntos.value) mapa.set(p.grupo ?? '', [...(mapa.get(p.grupo ?? '') ?? []), p])
  const texto = (busqueda.value ?? '').trim().toLowerCase()
  return [...mapa.entries()]
    .sort(([a], [b]) => (!a ? 1 : !b ? -1 : a.localeCompare(b)))
    .filter(([nombre]) => filtroGrupo.value === null || filtroGrupo.value === nombre)
    .map(([nombre, lista]) => ({ nombre, total: lista.length, activos: lista.filter(activo).length,
      puntos: lista.filter((p) => !texto || p.nombre.toLowerCase().includes(texto)).sort(comparar) }))
    .filter((g) => g.puntos.length)
})
const hayGrupos = computed(() => puntos.value.some((p) => p.grupo))
const opcionesGrupo = computed(() => [
  { label: t('puntos.todosGrupos'), value: null },
  ...[...new Set(puntos.value.map((p) => p.grupo).filter((g): g is string => !!g))].sort().map((g) => ({ label: g, value: g })),
  { label: t('puntos.sinGrupo'), value: '' },
])
const sugerencias = ref<string[]>([])
function filtrarGrupos(texto: string, actualizar: (f: () => void) => void) {
  actualizar(() => {
    const todos = grupos.value.map((g) => g.nombre).filter(Boolean)
    sugerencias.value = todos.filter((g) => g.toLowerCase().includes(texto.toLowerCase()))
  })
}

// gruposModelo agrupa los valores de model: primero los perfiles y luego cada agente en el orden fijo de la red.
const gruposModelo = computed(() => [
  { agente: '', titulo: t('conectar.perfiles'), ids: ids.value.filter((id) => id.startsWith('perfil/')) },
  ...ordenAgentes.value.map((a) => ({ agente: a, titulo: nombreAgente(a), ids: ids.value.filter((id) => id.startsWith(a + '/')) })),
].filter((g) => g.ids.length))

const modeloEjemplo = computed(() => ids.value.find((id) => id === 'perfil/rapido') ?? ids.value.find((id) => id.includes('/')) ?? ids.value[0] ?? '')

const ocultar = (c: string) => (c ? c.slice(0, 6) + '•'.repeat(18) : '')
const ejemplos = computed(() => ejemplosDe({ url: base.value, modelo: modeloEjemplo.value, contratos }))
const ejemplo = computed(() => ejemplos.value.find((e) => e.id === lenguaje.value) ?? ejemplos.value[0])
const ejemplosDetalle = computed(() => (puntoDetalle.value
  ? ejemplosDe({ url: puntoDetalle.value.url ?? '', modelo: idModelo(puntoDetalle.value) ?? modeloEjemplo.value, contratos: puntoDetalle.value.contratos,
    muestra: { archivos: ['informe.pdf'], solicitud: { mensajes: [{ rol: 'usuario', contenido: t('ejemplos.mensaje') }] } } }) : []))
const ejemploDetalle = computed(() => ejemplosDetalle.value.find((e) => e.id === lenguaje.value) ?? ejemplosDetalle.value[0])

const opciones = computed(() => catalogo.value.map(opcionModelo))
// etiquetaDe da el nombre legible de un modelo o de una entrada de respaldo con su esfuerzo propio («Codex · alto»).
const etiquetaDe = (id: string) => {
  const { base, esfuerzo } = dividirRespaldo(id)
  const nombre = opciones.value.find((o) => o.value === base)?.label ?? etiquetaModelo(base)
  return esfuerzo ? `${nombre} · ${nombreEsfuerzo(esfuerzo).toLowerCase()}` : nombre
}
const vacio = () => ({ nombre: '', grupo: '', direccion: 'ruta', puerto: '', contratos: [...contratos], permitidos: [] as string[], alias: '',
  receta: recetaVacia(), presupuesto: {} as Presupuesto })
const formulario = reactive(vacio())


// uso son las peticiones de cada API que conserva la retención, contadas por el servicio.
const uso = ref<Record<string, UsoPunto>>({})
// presupuestos es el consumo frente a los límites de cada API con presupuesto; presupuestoEspacio, el del espacio.
const presupuestos = ref<Record<string, EstadoPresupuesto>>({})
const presupuestoEspacio = ref<EstadoPresupuesto>()
const emit = defineEmits<{ ir: [pagina: string] }>()

function textoUso(nombre: string) {
  const u = uso.value[nombre]
  return [u.peticiones === 1 ? t('puntos.usoUno', { periodo: periodoHistorial.value }) : t('puntos.uso', { n: u.peticiones, periodo: periodoHistorial.value }), u.errores ? t('puntos.usoErrores', { n: u.errores }) : '',
    u.ultima ? t('puntos.usoUltima', { cuando: haceCuanto(u.ultima) }) : ''].filter(Boolean).join(' · ')
}

function verPeticiones(nombre: string) {
  origenHistorial.value = nombre
  emit('ir', 'historial')
}

async function cargarPuntos() {
  try {
    const r = await api.puntosConUso()
    puntos.value = r.puntos_acceso
    uso.value = r.uso ?? {}
    presupuestos.value = r.presupuestos ?? {}
    presupuestoEspacio.value = await api.presupuestoInstancia()
  } catch (e) {
    avisarError(e)
  }
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
    await cargarPuntos()
    // Con la clave recién creada en los ejemplos: es la única vez que se ve, así que el detalle se abre siempre.
    const creado = puntos.value.find((p) => p.nombre === punto.nombre) ?? r.punto_acceso
    if (creado) abrirDetalle(creado, r.token, true)
    else avisar(t('puntos.claveCreada', { clave: r.token }))
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

async function activar(nombre: string, valor: boolean) {
  try {
    await api.activarPunto(nombre, valor)
    await cargarPuntos()
  } catch (e) {
    avisarError(e)
  }
}

// activarGrupo cambia de uno en uno los puntos del grupo que no están ya en el estado pedido.
async function activarGrupo(grupo: string, valor: boolean) {
  ocupado.value = grupo + (valor ? ':1' : ':0')
  try {
    for (const p of puntos.value.filter((p) => (p.grupo ?? '') === grupo && activo(p) !== valor)) await api.activarPunto(p.nombre, valor)
  } catch (e) {
    avisarError(e)
  } finally {
    ocupado.value = ''
    await cargarPuntos()
  }
}

function eliminar(nombre: string) {
  $q.dialog({ title: t('puntos.eliminarTitulo', { nombre }), message: t('puntos.eliminarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('comun.eliminar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.eliminarPunto(nombre)
        if (detalle.value === nombre) dialogoDetalle.value = false
        await cargarPuntos()
      } catch (e) {
        avisarError(e)
      }
    })
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

// abrirEnProbar lleva la receta de la API a Probar para probarla y actualizarla.
function abrirEnProbar(nombre: string) {
  recetaInicial.value = nombre
  emit('ir', 'ejecutar')
}

async function guardarAjustes() {
  guardando.value = true
  try {
    await api.ajustesPunto(editando.value, { ...ajustesDeReceta(ajustes.receta), modelos_permitidos: ajustes.permitidos,
      presupuesto: presupuestoLimpio(ajustes.presupuesto) ?? null })
    dialogoInstrucciones.value = false
    avisar(t('puntos.ajustesGuardados'))
    await cargarPuntos()
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

// reiniciarPresupuesto pone a cero el consumo del día y del mes de una API, o del espacio sin nombre.
function reiniciarPresupuesto(nombre?: string) {
  $q.dialog({ title: nombre ? t('presupuesto.reiniciarTitulo', { nombre }) : t('presupuesto.reiniciarEspacio'), message: nombre ? t('presupuesto.reiniciarTexto') : t('presupuesto.reiniciarEspacioTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('presupuesto.reiniciar'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.reiniciarPresupuesto(nombre)
        avisar(t('presupuesto.reiniciado'))
        await cargarPuntos()
      } catch (e) {
        avisarError(e)
      }
    })
}

// regenerarClaveInstancia cambia la clave de «Conectar» por si se filtró; las aplicaciones que la usan dejan de
// conectar hasta que se les ponga la nueva.
function regenerarClaveInstancia() {
  $q.dialog({ title: t('conectar.regenerarTitulo'), message: t('conectar.regenerarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'negative', label: t('conectar.regenerarClave'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        clave.value = (await api.regenerarClaveInstancia()).token
        verClave.value = true
        avisar(t('conectar.claveRegenerada'))
      } catch (e) {
        avisarError(e)
      }
    })
}

// regenerarClave crea una clave nueva y la muestra esta única vez: Tramtai solo guarda su huella.
function regenerarClave(p: EstadoPunto) {
  $q.dialog({ title: t('puntos.regenerarTitulo', { nombre: p.nombre }), message: t('puntos.regenerarTexto'),
    cancel: { flat: true, label: t('comun.cancelar'), noCaps: true }, ok: { color: 'primary', label: t('puntos.regenerarClave'), unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        abrirDetalle(p, await api.regenerarClave(p.nombre))
      } catch (e) {
        avisarError(e)
      }
    })
}

onMounted(async () => {
  cargarDiasHistorial()
  if (grupoInicial.value) {
    filtroGrupo.value = grupoInicial.value // llegó desde el mapa de Inicio
    grupoInicial.value = ''
  }
  cargarPuntos()
  api.catalogo().then((m) => (catalogo.value = m)).catch(avisarError)
  api.instancia().then((i) => (base.value = i.base)).catch(() => {})
  api.claveInstancia().then((r) => (clave.value = r.token)).catch(avisarError)
})
</script>
