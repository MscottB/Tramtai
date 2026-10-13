<template>
  <div class="pagina apilado">
    <div>
      <div class="titulo-pagina">Conectar apps</div>
      <div class="texto-suave">
        Cualquier aplicación o SDK compatible con OpenAI o Anthropic puede usar tus agentes: cambia la URL base y usa la clave de aquí.
      </div>
    </div>

    <q-card>
      <q-card-section>
        <div class="titulo-seccion">Conexión directa</div>
        <div class="texto-suave text-body2">La misma URL y clave para todas tus aplicaciones de este equipo.</div>
      </q-card-section>
      <q-card-section class="row q-col-gutter-md q-pt-none">
        <div v-for="c in campos" :key="c.titulo" class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">{{ c.titulo }}</div>
          <div class="bloque-codigo">{{ c.valor }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="'Copiar ' + c.titulo" @click="copiar(c.valor)" /></div>
        </div>
        <div class="col-12 col-md-6">
          <div class="text-caption texto-suave q-mb-xs">Clave (API key)</div>
          <div class="bloque-codigo" style="padding-right: 80px">{{ clave ? (verClave ? clave : ocultar(clave)) : 'Cargando…' }}
            <div class="boton-copiar row no-wrap">
              <q-btn flat dense round size="sm" :icon="verClave ? 'visibility_off' : 'visibility'" :aria-label="verClave ? 'Ocultar la clave' : 'Mostrar la clave'" @click="verClave = !verClave" />
              <q-btn flat dense round size="sm" icon="content_copy" aria-label="Copiar la clave" :disable="!clave" @click="copiar(clave, 'Clave copiada')" />
            </div>
          </div>
        </div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <div class="text-caption texto-suave q-mb-xs">Modelo: el campo <span class="codigo">model</span> elige agente y modelo</div>
        <div class="row q-gutter-xs">
          <q-chip v-for="m in ejemplosModelo" :key="m" dense clickable outline color="primary" class="codigo" @click="copiar(m)">{{ m }}</q-chip>
        </div>
      </q-card-section>
      <q-separator />
      <q-tabs v-model="lenguaje" dense no-caps align="left" active-color="primary" indicator-color="primary" class="q-px-sm">
        <q-tab v-for="e in ejemplos" :key="e.id" :name="e.id" :label="e.titulo" />
      </q-tabs>
      <q-separator />
      <q-card-section>
        <div class="bloque-codigo sin-ajuste">{{ ejemplo.codigo(verClave ? clave : ocultar(clave)) }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" aria-label="Copiar el ejemplo" @click="copiar(ejemplo.codigo(clave), 'Ejemplo copiado con tu clave')" /></div>
      </q-card-section>
    </q-card>

    <q-card>
      <q-card-section class="row items-center q-col-gutter-md">
        <div class="col-12 col-sm">
          <div class="titulo-seccion">Puntos de acceso</div>
          <div class="texto-suave text-body2">
            Dale a una aplicación su propia URL y clave, con un modelo fijo o una lista de modelos permitidos. Puedes desactivarla
            sin afectar a las demás.
          </div>
        </div>
        <div class="col-12 col-sm-auto">
          <q-btn unelevated color="primary" icon="add" label="Nuevo punto de acceso" no-caps @click="abrirNuevo" />
        </div>
      </q-card-section>
      <q-separator />
      <q-list v-if="puntos.length" separator>
        <q-item v-for="p in puntos" :key="p.nombre" class="q-py-md">
          <q-item-section avatar><q-icon name="o_hub" :color="p.activo ? 'primary' : 'grey-5'" /></q-item-section>
          <q-item-section>
            <q-item-label class="text-weight-medium">{{ p.nombre }}</q-item-label>
            <q-item-label caption class="codigo">{{ p.url || 'sin dirección' }}</q-item-label>
            <q-item-label caption>
              {{ p.contratos.map((c) => nombresContrato[c] ?? c).join(' · ') }}
              <template v-if="p.agente || p.modelo"> · {{ [nombreAgente(p.agente), p.modelo].filter(Boolean).join(' · ') }}</template>
            </q-item-label>
            <q-item-label v-if="p.error && p.error !== 'desactivado'" caption class="text-negative">{{ p.error }}</q-item-label>
          </q-item-section>
          <q-item-section side>
            <div class="row items-center no-wrap q-gutter-x-xs">
              <q-toggle :model-value="p.error !== 'desactivado'" :aria-label="'Activar ' + p.nombre" @update:model-value="(v: boolean) => activar(p.nombre, v)">
                <q-tooltip>{{ p.error === 'desactivado' ? 'Desactivado' : 'Activo' }}</q-tooltip>
              </q-toggle>
              <q-btn v-if="p.url" flat dense round icon="content_copy" aria-label="Copiar la URL" @click="copiar(p.url)"><q-tooltip>Copiar la URL</q-tooltip></q-btn>
              <q-btn flat dense round icon="o_key" aria-label="Ver la clave" @click="mostrarClave(p)"><q-tooltip>Ver la clave</q-tooltip></q-btn>
              <q-btn flat dense round icon="o_delete" color="negative" aria-label="Eliminar" @click="eliminar(p.nombre)"><q-tooltip>Eliminar</q-tooltip></q-btn>
            </div>
          </q-item-section>
        </q-item>
      </q-list>
      <div v-else class="vacio">
        <q-icon name="o_hub" size="40px" />
        <div class="q-mt-sm">Todavía no hay puntos de acceso. No los necesitas para empezar: la conexión directa ya funciona.</div>
      </div>
    </q-card>

    <q-dialog v-model="nuevo">
      <q-card style="width: 600px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">Nuevo punto de acceso</div>
          <div class="texto-suave text-body2">Una URL y una clave propias para una aplicación.</div>
        </q-card-section>
        <q-form @submit="crear">
          <q-card-section class="q-gutter-y-md q-pt-none">
            <q-input v-model="formulario.nombre" outlined dense label="Nombre" autofocus hint="Por ejemplo: contabilidad o bot_soporte"
              :rules="[(v: string) => /^[a-z0-9][a-z0-9_-]{0,39}$/.test(v) || 'Minúsculas, números, _ y -; hasta 40 caracteres']" />
            <q-select v-model="formulario.modelo" :options="opciones" emit-value map-options outlined dense clearable label="Modelo predeterminado (opcional)">
              <template #option="scope">
                <q-item v-bind="scope.itemProps">
                  <q-item-section><q-item-label>{{ scope.opt.label }}</q-item-label><q-item-label caption>{{ scope.opt.caption }}</q-item-label></q-item-section>
                </q-item>
              </template>
            </q-select>
            <q-toggle v-model="formulario.forzar" :disable="!formulario.modelo" label="Usar siempre este modelo, aunque la aplicación pida otro" />
            <q-expansion-item dense icon="o_tune" label="Opciones avanzadas" header-class="q-px-none text-body2">
              <div class="q-gutter-y-md q-pt-sm">
                <div>
                  <div class="text-caption texto-suave">Dirección</div>
                  <q-option-group v-model="formulario.direccion" inline
                    :options="[{ label: 'Ruta en el puerto del servicio (recomendado)', value: 'ruta' }, { label: 'Puerto propio', value: 'puerto' }]" />
                  <q-input v-if="formulario.direccion === 'puerto'" v-model="formulario.puerto" outlined dense inputmode="numeric" label="Puerto"
                    class="q-mt-sm" style="max-width: 200px"
                    :rules="[(v: string) => (/^\d+$/.test(v) && +v >= 1024 && +v <= 65535) || 'Entre 1024 y 65535']" />
                </div>
                <div>
                  <div class="text-caption texto-suave">Compatibilidad</div>
                  <q-option-group v-model="formulario.contratos" type="checkbox" inline
                    :options="Object.entries(nombresContrato).map(([value, label]) => ({ label, value }))" />
                </div>
                <q-select v-model="formulario.permitidos" :options="opciones" emit-value map-options outlined dense multiple use-chips
                  label="Modelos permitidos" hint="Vacío: cualquiera" />
                <q-input v-model="formulario.alias" type="textarea" autogrow outlined dense label="Alias de modelos" input-class="codigo"
                  hint="Uno por línea, para apps que no dejan cambiar el modelo: gpt-4o-mini=claude/haiku" />
              </div>
            </q-expansion-item>
          </q-card-section>
          <q-card-actions align="right" class="q-pa-md">
            <q-btn flat no-caps label="Cancelar" v-close-popup />
            <q-btn unelevated color="primary" no-caps type="submit" label="Crear" :loading="guardando" />
          </q-card-actions>
        </q-form>
      </q-card>
    </q-dialog>

    <q-dialog v-model="dialogoClave">
      <q-card style="width: 560px; max-width: 95vw">
        <q-card-section>
          <div class="titulo-seccion">{{ recienCreado ? 'Punto de acceso listo' : 'Conexión de ' + elegido?.nombre }}</div>
          <div class="texto-suave text-body2">Configura tu aplicación con esta URL y esta clave.</div>
        </q-card-section>
        <q-card-section class="q-gutter-y-md q-pt-none">
          <div v-for="c in camposPunto" :key="c.titulo">
            <div class="text-caption texto-suave q-mb-xs">{{ c.titulo }}</div>
            <div class="bloque-codigo">{{ c.valor }}<q-btn flat dense round size="sm" icon="content_copy" class="boton-copiar" :aria-label="'Copiar ' + c.titulo" @click="copiar(c.valor)" /></div>
          </div>
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md"><q-btn unelevated color="primary" no-caps label="Hecho" v-close-popup /></q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useQuasar } from 'quasar'
import { api, aplicarModelo, avisarError, copiar, nombreAgente, opcionModelo, type ConfigPunto, type EstadoPunto } from '../api'

const $q = useQuasar()
const base = ref(location.origin)
const clave = ref('')
const verClave = ref(false)
const ids = ref<string[]>([])
const puntos = ref<EstadoPunto[]>([])
const nuevo = ref(false)
const guardando = ref(false)
const dialogoClave = ref(false)
const recienCreado = ref(false)
const elegido = ref<{ nombre: string; url: string; clave: string }>()
const lenguaje = ref('python')
const nombresContrato: Record<string, string> = { openai: 'OpenAI', anthropic: 'Anthropic', nativa: 'API de Tramtai' }

const campos = computed(() => [
  { titulo: 'URL base para OpenAI', valor: base.value + '/openai/v1' },
  { titulo: 'URL base para Anthropic', valor: base.value + '/anthropic' },
  { titulo: 'API propia de Tramtai', valor: base.value + '/api/v1' },
])
const ejemplosModelo = computed(() => {
  const perfiles = ids.value.filter((id) => id.startsWith('perfil/'))
  const modelos = ids.value.filter((id) => !id.startsWith('perfil/') && id.includes('/'))
  return [...perfiles, ...modelos].slice(0, 10)
})
const modeloEjemplo = computed(() => ids.value.find((id) => id === 'perfil/rapido') ?? ids.value.find((id) => id.includes('/')) ?? 'claude')

const ocultar = (c: string) => (c ? c.slice(0, 6) + '•'.repeat(18) : '')
const ejemplos = [
  { id: 'python', titulo: 'Python · OpenAI', codigo: (k: string) => `from openai import OpenAI

cliente = OpenAI(base_url="${base.value}/openai/v1", api_key="${k}")
r = cliente.chat.completions.create(
    model="${modeloEjemplo.value}",
    messages=[{"role": "user", "content": "Hola"}],
)
print(r.choices[0].message.content)` },
  { id: 'node', titulo: 'Node · OpenAI', codigo: (k: string) => `import OpenAI from 'openai'

const cliente = new OpenAI({ baseURL: '${base.value}/openai/v1', apiKey: '${k}' })
const r = await cliente.chat.completions.create({
  model: '${modeloEjemplo.value}',
  messages: [{ role: 'user', content: 'Hola' }],
})
console.log(r.choices[0].message.content)` },
  { id: 'anthropic', titulo: 'Python · Anthropic', codigo: (k: string) => `import anthropic

cliente = anthropic.Anthropic(base_url="${base.value}/anthropic", api_key="${k}")
m = cliente.messages.create(
    model="${modeloEjemplo.value}", max_tokens=1024,
    messages=[{"role": "user", "content": "Hola"}],
)
print(m.content[0].text)` },
  { id: 'curl', titulo: 'curl', codigo: (k: string) => `curl ${base.value}/openai/v1/chat/completions \\
  -H "Authorization: Bearer ${k}" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "${modeloEjemplo.value}", "messages": [{"role": "user", "content": "Hola"}]}'` },
  { id: 'entorno', titulo: 'Variables de entorno', codigo: (k: string) => `# Muchas apps y SDK leen estas variables sin tocar el código
OPENAI_BASE_URL=${base.value}/openai/v1
OPENAI_API_KEY=${k}
ANTHROPIC_BASE_URL=${base.value}/anthropic
ANTHROPIC_API_KEY=${k}` },
]
const ejemplo = computed(() => ejemplos.find((e) => e.id === lenguaje.value) ?? ejemplos[0])

const opciones = computed(() => ids.value.map(opcionModelo))
const vacio = () => ({ nombre: '', direccion: 'ruta', puerto: '', contratos: ['openai', 'anthropic', 'nativa'], modelo: null as string | null,
  permitidos: [] as string[], alias: '', forzar: false })
const formulario = reactive(vacio())

const camposPunto = computed(() => {
  const p = elegido.value
  if (!p) return []
  return [
    { titulo: 'URL base para OpenAI', valor: p.url + '/openai/v1' },
    { titulo: 'URL base para Anthropic', valor: p.url + '/anthropic' },
    { titulo: 'Clave (API key)', valor: p.clave },
  ]
})

async function cargarPuntos() {
  try {
    puntos.value = await api.puntos()
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
      avisarError(new Error(`Alias no válido: ${linea} (usa nombre=agente/modelo)`))
      return
    }
    alias[origen] = destino
  }
  const elegidoModelo = aplicarModelo({ mensajes: [] }, formulario.modelo ?? '')
  const punto: ConfigPunto = {
    nombre: formulario.nombre.trim(),
    puerto: formulario.direccion === 'puerto' ? Number(formulario.puerto) : undefined,
    contratos: formulario.contratos,
    agente: elegidoModelo.agente,
    modelo: elegidoModelo.modelo,
    perfil_modelo: elegidoModelo.perfil_modelo,
    modelos_permitidos: formulario.permitidos.length ? formulario.permitidos : undefined,
    alias_modelos: Object.keys(alias).length ? alias : undefined,
    forzar_modelo: (formulario.forzar && !!formulario.modelo) || undefined,
  }
  guardando.value = true
  try {
    const r = await api.crearPunto(punto)
    nuevo.value = false
    await cargarPuntos()
    const url = r.punto_acceso?.url ?? puntos.value.find((p) => p.nombre === punto.nombre)?.url ?? ''
    elegido.value = { nombre: punto.nombre, url, clave: r.token }
    recienCreado.value = true
    dialogoClave.value = true
  } catch (e) {
    avisarError(e)
  } finally {
    guardando.value = false
  }
}

async function activar(nombre: string, activo: boolean) {
  try {
    await api.activarPunto(nombre, activo)
    await cargarPuntos()
  } catch (e) {
    avisarError(e)
  }
}

function eliminar(nombre: string) {
  $q.dialog({ title: `Eliminar ${nombre}`, message: 'Las aplicaciones que usan este punto de acceso dejarán de funcionar.',
    cancel: { flat: true, label: 'Cancelar', noCaps: true }, ok: { color: 'negative', label: 'Eliminar', unelevated: true, noCaps: true } })
    .onOk(async () => {
      try {
        await api.eliminarPunto(nombre)
        await cargarPuntos()
      } catch (e) {
        avisarError(e)
      }
    })
}

async function mostrarClave(p: EstadoPunto) {
  try {
    elegido.value = { nombre: p.nombre, url: p.url ?? '', clave: await api.tokenPunto(p.nombre) }
    recienCreado.value = false
    dialogoClave.value = true
  } catch (e) {
    avisarError(e)
  }
}

onMounted(async () => {
  cargarPuntos()
  api.modelos().then((m) => (ids.value = m)).catch(avisarError)
  api.instancia().then((i) => (base.value = i.base)).catch(() => {})
  api.claveInstancia().then((r) => (clave.value = r.token)).catch(avisarError)
})
</script>
