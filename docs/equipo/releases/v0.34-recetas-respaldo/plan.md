# Recetas y respaldo: plan de implementación

> **Para agentes:** usar superpowers:subagent-driven-development (recomendado) o superpowers:executing-plans
> tarea por tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** publicar una receta (instrucciones + agente + esquema + respaldo) como API desde Probar o Mis APIs,
y que el trabajo salte a otro agente si el primero falla antes de producir texto.

**Arquitectura:** la receta es un punto de acceso con dos campos nuevos (`esquema`, `respaldo`) que
`Servidor.ajustar` aplica a la solicitud. El planificador recorre los candidatos (principal y respaldo) dentro del
mismo trabajo. La Web UI comparte un componente `EditorReceta` entre Probar y Mis APIs.

**Tecnología:** Go 1.27 (SQLite modernc, yaml.v3, santhosh-tekuri/jsonschema), Vue 3 + Quasar 2.35 + Vite 8.

**Especificación:** [spec.md](spec.md)

## Restricciones globales

- Sin dependencias nuevas (ni Go ni npm).
- Todo texto de error o aviso en Go va con `idioma.T`/`idioma.Tf` y su traducción en `internal/idioma/en_*.go`
  (`TestCobertura` falla si falta). Los del núcleo, la API, el planificador y el servicio van en `en_servicio.go`.
- Textos de la Web UI en `web/src/idiomas/es.ts` y `en.ts` (este tipado con `Textos`).
- `respaldo`: máximo 3 entradas, cada una `agente` o `agente/modelo`, sin repetidos.
- Códigos que permiten saltar: `limite_uso_proveedor`, `autenticacion_agente`, `agente_no_disponible`,
  `modelo_no_disponible`, `error_agente`, `error_protocolo_agente`, `tiempo_ejecucion_agotado`; y solo si el
  intento no emitió ningún evento `texto` no vacío.
- El tiempo límite de ejecución cuenta por intento; el plazo en cola no se reinicia.
- Pruebas Go: `go test ./...` desde la raíz (Go está en `C:\Program Files\Go\bin`, fuera del PATH de algunas
  shells). Web: `npm run build` en `web/` (incluye `vue-tsc`; escribe en `internal/recursos/web`).
- Commits Conventional (`feat:`, `fix:`, `docs:`) con `git commit -F <archivo>`; el hook bloquea algunos `-m`.
- Tras cambiar código: `graphify update .`.

## Foco de revisión

1. **Respaldo que repite el agente principal** (`agente: codex`, `respaldo: [codex, gemini]`): el candidato igual
   a uno anterior se omite; codex no corre dos veces. Prueba en la tarea 3.
2. **Reintentar un trabajo que ya saltó:** la solicitud guardada tiene como agente el que respondió; el reintento
   empieza por él y omite su entrada repetida en el respaldo. Prueba en la tarea 3.
3. **Cancelar mientras el trabajo espera el semáforo del agente de respaldo:** termina en `cancelado`, no en
   `fallido`. Prueba en la tarea 3.
4. **PATCH con un esquema no válido:** responde `configuracion_invalida` y el archivo de configuración queda
   intacto. Prueba en la tarea 5.
5. **Punto con esquema llamado por la fachada OpenAI sin `response_format`:** la salida se valida igual contra
   el esquema del punto. Prueba en la tarea 4.

---

### Tarea 1: núcleo — campo `respaldo` y transición de vuelta a la espera

**Archivos:**
- Modificar: `internal/nucleo/solicitud.go`, `internal/nucleo/estados.go:27`, `internal/idioma/en_servicio.go`
- Pruebas: `internal/nucleo/solicitud_test.go`, `internal/nucleo/estados_test.go`

**Interfaces:**
- Produce: `SolicitudEjecucion.Respaldo []string` (`json:"respaldo,omitempty"`), situado tras `Esfuerzo`.
- Produce: `func ValidarRespaldo(lista []string) error` (`CodigoSolicitudInvalida`).
- Produce: `func DividirRespaldo(entrada string) (agente, modelo string)` (`strings.Cut` por `/`).
- Produce: transición `Ejecutando → EsperandoAgente` válida.

- [ ] **Paso 1: escribir las pruebas que fallan**

```go
func TestValidarRespaldo(t *testing.T) {
	for _, ok := range [][]string{nil, {"codex"}, {"codex/gpt-5", "gemini"}, {"a", "b", "c"}} {
		if err := ValidarRespaldo(ok); err != nil {
			t.Fatalf("%v: %v", ok, err)
		}
	}
	for _, mal := range [][]string{{"a", "b", "c", "d"}, {"perfil/rapido"}, {""}, {"codex/"}, {"a b"}, {"codex", "codex"}} {
		if CodigoDe(ValidarRespaldo(mal)) != CodigoSolicitudInvalida {
			t.Fatalf("%v debía rechazarse", mal)
		}
	}
	s := SolicitudEjecucion{Agente: "x", Mensajes: []Mensaje{{Rol: "usuario", Contenido: "h"}}, Respaldo: []string{"perfil/r"}}
	if s.Validar() == nil {
		t.Fatal("Validar debe aplicar ValidarRespaldo")
	}
}
```

En `estados_test.go`, añadir el caso `ValidarTransicion(Ejecutando, EsperandoAgente) == nil`.

- [ ] **Paso 2:** `go test ./internal/nucleo/` → FALLA (`ValidarRespaldo` no definido).
- [ ] **Paso 3:** implementar. Cada parte de la entrada debe cumplir `reIdentificador` sin `/`; el agente
  `perfil` está reservado. Mensajes: «respaldo: máximo 3 agentes», «respaldo %q con formato no válido (agente o
  agente/modelo)», «respaldo repetido: %s», con su traducción. `Validar` llama a `ValidarRespaldo`. Añadir
  `EsperandoAgente` a la lista de `Ejecutando` en `estados.go` con el comentario de por qué (salto de respaldo).
- [ ] **Paso 4:** `go test ./internal/nucleo/ ./internal/idioma/` → PASA.
- [ ] **Paso 5:** commit `feat(nucleo): respaldo en la solicitud`.

### Tarea 2: persistencia — `CambiarAgente`

**Archivos:**
- Modificar: `internal/persistencia/almacen.go` (junto a `RegistrarProceso`)
- Pruebas: `internal/persistencia/almacen_test.go`

**Interfaces:**
- Produce: `func (a *Almacen) CambiarAgente(ctx context.Context, id, agente, modeloEnviado string) error`.
  Actualiza `solicitud` con `json_set(solicitud, '$.agente', ?)` y `modelo_enviado`; llama a `a.avisar()`.
  `no_encontrado` si no afecta a ninguna fila.

- [ ] **Paso 1: prueba `TestCambiarAgente`.** Acepta un trabajo con agente `simulado` y modelo `m1`, llama a
  `CambiarAgente(ctx, id, "otro", "m2")` y comprueba con `Obtener` que `Solicitud.Agente == "otro"`,
  `ModeloEnviado == "m2"` y que `Huella` no cambió. Con un id inexistente, `CodigoDe(err) == CodigoNoEncontrado`.
- [ ] **Paso 2:** `go test ./internal/persistencia/ -run TestCambiarAgente` → FALLA.
- [ ] **Paso 3:** implementar.
- [ ] **Paso 4:** `go test ./internal/persistencia/` → PASA.
- [ ] **Paso 5:** commit `feat(persistencia): cambiar el agente de un trabajo`.

### Tarea 3: planificador — salto entre candidatos

**Archivos:**
- Modificar: `internal/planificador/planificador.go` (`aceptar`, `procesar`), `internal/idioma/en_servicio.go`
- Pruebas: `internal/planificador/planificador_test.go`

**Interfaces:**
- Consume: `nucleo.DividirRespaldo`, `Almacen.CambiarAgente`, la transición de la tarea 1.
- Produce (internas): `type candidato struct{ agente, modelo string }`;
  `func (p *Planificador) candidatos(t nucleo.Trabajo) []candidato` (el primero es
  `{t.Solicitud.Agente, t.ModeloEnviado}`; después el respaldo sin los iguales a uno anterior);
  `func saltable(c nucleo.CodigoError) bool`.

- [ ] **Paso 1: escribir las pruebas que fallan.** Añadir el ayudante
  `iniciarVarios(t, alm, map[string]adaptadores.Adaptador, opc) *Planificador` (como `iniciar`, con semáforo 2
  por agente) y un adaptador de prueba `trasTexto{err error}` que emite `texto` "parcial" y devuelve `err`.

| Prueba | Montaje | Esperado |
|---|---|---|
| `TestRespaldoSaltaPorLimite` | `simulado` con `Error: NuevoError(CodigoLimiteUsoProveedor, …)`, `otro` con `Respuesta: "ok"`; `Respaldo: ["otro"]` | `completado`, `Salida == "ok"`, `Solicitud.Agente == "otro"`, un evento `aviso` que contiene `simulado` y `otro` |
| `TestRespaldoNoSaltaTrasTexto` | `simulado` = `trasTexto{limite}`, `otro` ok | `fallido`, `limite_uso_proveedor`, agente sigue `simulado` |
| `TestRespaldoSinCandidatosQuedaElUltimo` | `simulado` falla con `error_agente`, `otro` falla con `agente_no_disponible` | `fallido` con `agente_no_disponible` |
| `TestRespaldoOmiteRepetidos` | `Agente: "simulado"`, `Respaldo: ["simulado", "otro"]`; `simulado` falla con límite y cuenta sus llamadas | `simulado` se ejecutó 1 vez, completa `otro` |
| `TestRespaldoCanceladoEnEspera` | `otro` con semáforo 1 ocupado por un trabajo largo; el primero salta y queda en `esperando_agente`; `Cancelar` | `cancelado` |
| `TestRespaldoAgenteDesconocido` | `Respaldo: ["nadie"]` | `Enviar` devuelve `solicitud_invalida` |
| `TestReintentoTrasSalto` | tras `TestRespaldoSaltaPorLimite`, `Reintentar` | el nuevo trabajo empieza por `otro` (sin evento `aviso`) |

- [ ] **Paso 2:** `go test ./internal/planificador/ -run Respaldo` → FALLA.
- [ ] **Paso 3: implementar.**
  - En `aceptar`, tras `Validar`: todo agente de `Respaldo` debe estar en `p.adaptadores`; si no,
    `CodigoSolicitudInvalida` «agente de respaldo no disponible: %s».
  - En `procesar`, desde la toma de semáforos hasta la ejecución, recorrer `candidatos(t)`. Por intento: tomar
    punto → agente del candidato → plaza y soltarlos al terminar el intento (no con `defer` del método); envolver
    `agrupador.emitir` para marcar `huboTexto`. Si el error es `saltable`, no hubo texto y queda otro candidato:
    `cambiar(id, Ejecutando, EsperandoAgente)`, `CambiarAgente`, evento `aviso` «%s falló (%s); sigue con %s» y
    siguiente vuelta. El tiempo agotado cuenta como `tiempo_ejecucion_agotado` saltable; la cancelación y el
    drenaje siguen como hoy.
  - Si `procesar` supera ~100 líneas tras el cambio, extraer el intento a
    `func (p *Planificador) intentar(...)` en el mismo archivo.
- [ ] **Paso 4:** `go test -race ./internal/planificador/` (con llvm-mingw) o `go test ./internal/planificador/` → PASA,
  y las pruebas anteriores siguen pasando.
- [ ] **Paso 5:** commit `feat(planificador): respaldo entre agentes`.

### Tarea 4: API — esquema y respaldo del punto en `ajustar`

**Archivos:**
- Modificar: `internal/api/punto.go`
- Pruebas: `internal/api/api_test.go`

**Interfaces:**
- Produce: `api.Punto.Esquema json.RawMessage` y `api.Punto.Respaldo []string`.

- [ ] **Paso 1: pruebas.**
  - `TestPuntoAplicaEsquemaYRespaldo`: un punto con `Esquema` `{"type":"object"}` y `Respaldo ["otro"]`; una
    solicitud sin ellos sale de `ajustar` con `Salida.Formato == "json"`, el esquema del punto y
    `Respaldo == ["otro"]`. Una solicitud con su propio esquema y respaldo los conserva.
  - `TestPuntoRespaldoNoPermitido`: `ModelosPermitidos ["simulado"]`, `Respaldo ["otro"]` →
    `modelo_no_permitido`.
  - `TestFachadaOpenAIConEsquemaDelPunto`: `POST /openai/v1/chat/completions` contra el punto con un agente
    simulado que responde `no es json` → el error es `salida_no_cumple_esquema`.
- [ ] **Paso 2:** `go test ./internal/api/ -run 'Punto|Fachada'` → FALLA.
- [ ] **Paso 3:** implementar en `ajustar`, después de las instrucciones. La comprobación de permitidos del
  respaldo usa `permitido` con `nucleo.SolicitudEjecucion{Agente: a, Modelo: m}` de `DividirRespaldo`.
- [ ] **Paso 4:** `go test ./internal/api/` → PASA.
- [ ] **Paso 5:** commit `feat(api): esquema y respaldo por punto de acceso`.

### Tarea 5: servicio — configuración y Web UI de los puntos

**Archivos:**
- Modificar: `internal/servicio/puntos.go` (`ConfigPunto`, `Normalizar`, `politica`, `EstadoPunto` y donde se
  rellena en la línea ~356), `internal/servicio/web.go` (`puntoUI` PATCH)
- Pruebas: `internal/servicio/servicio_test.go`, `internal/servicio/web_test.go`

**Interfaces:**
- Produce: `ConfigPunto.Esquema map[string]any` (`json/yaml:"esquema,omitempty"`) y
  `ConfigPunto.Respaldo []string` (`respaldo,omitempty`); los mismos campos en `EstadoPunto`.
- Produce: PATCH `/api/v1/puntos/<n>` acepta `esquema` (`json.RawMessage`: ausente = sin cambio; `null` o `{}`
  lo quita; objeto lo fija) y `respaldo` (`*[]string`).

- [ ] **Paso 1: pruebas.**
  - `Normalizar` rechaza un esquema no compilable (`{"type": 5}`) y un respaldo con `perfil/x`, ambos con
    `configuracion_invalida`.
  - En la tabla de `web_test.go:112`, casos de alta con `esquema` y `respaldo` que se leen igual en `puntos_acceso`.
  - PATCH con `{"esquema": {"type": 5}}` → 400 `configuracion_invalida` y el YAML del punto sin cambios
    (leer el archivo antes y después); PATCH con `{"esquema": null}` lo quita; PATCH con `{"respaldo": ["otro"]}`
    lo fija.
- [ ] **Paso 2:** `go test ./internal/servicio/` → FALLA.
- [ ] **Paso 3:** implementar. En `Normalizar`: `json.Marshal(p.Esquema)` → `salidas.Compilar`, y
  `nucleo.ValidarRespaldo`, envolviendo el mensaje con el `invalido` existente. `politica` copia ambos
  (el esquema serializado). En PATCH, tras aplicar el cambio dentro de `editar`, llamar a `Normalizar` del punto
  editado y devolver su error para que `EditarPuntos` no escriba.
- [ ] **Paso 4:** `go test ./internal/...` → PASA.
- [ ] **Paso 5:** commit `feat(servicio): recetas en los puntos de acceso`.

### Tarea 6: Web — tipos y `EditorReceta`

**Archivos:**
- Modificar: `web/src/api.ts`
- Crear: `web/src/componentes/EditorReceta.vue`
- Modificar: `web/src/idiomas/es.ts`, `web/src/idiomas/en.ts` (claves `receta.*`)

**Interfaces:**
- Produce en `api.ts`:
  - `Solicitud` gana `instrucciones?: string` y `respaldo?: string[]`; `EstadoPunto` y `ConfigPunto` ganan
    `esquema?: Record<string, unknown>` y `respaldo?: string[]`; `ajustesPunto` acepta `esquema` (objeto o
    `null`) y `respaldo`.
  - `export interface Receta { instrucciones: string; modelo: string | null; forzar: boolean; esfuerzo: string;
    respaldo: string[]; esquema: string }` (esquema como texto del editor; `''` = sin salida estructurada).
  - `export const recetaVacia: () => Receta`, `export function recetaDePunto(p: EstadoPunto): Receta`,
    `export function esquemaDeReceta(r: Receta): Record<string, unknown> | undefined` (lanza un `Error` con
    `t('receta.esquemaInvalido')` si no es un objeto JSON).
  - `export const recetaInicial = ref('')`: el nombre de la API que Probar debe abrir.
- Produce `EditorReceta.vue`: `v-model` de tipo `Receta`, prop `catalogo: Modelo[]`. Campos en este orden:
  instrucciones, `SelectorModelo` + «Forzar», `SelectorEsfuerzo`, respaldo (`q-select` múltiple de las opciones
  `agente` y `agente/modelo` del catálogo, máximo 3, conserva el orden), interruptor «Salida estructurada» que
  muestra el esquema con el mismo placeholder de Probar y valida al escribir.

- [ ] **Paso 1:** implementar tipos y ayudantes, el componente y los textos.
- [ ] **Paso 2:** `npm run build` en `web/` → sin errores de `vue-tsc`.
- [ ] **Paso 3:** commit `feat(web): editor de recetas`.

### Tarea 7: Web — Probar como taller

**Archivos:**
- Modificar: `web/src/paginas/Ejecutar.vue`, `es.ts`, `en.ts` (claves `ejecutar.*`)

**Interfaces:**
- Consume: `EditorReceta`, `Receta`, `recetaDePunto`, `esquemaDeReceta`, `recetaInicial`, `api.crearPunto`,
  `api.ajustesPunto`, `api.tokenPunto`.

- [ ] **Paso 1: implementar.**
  - Columna izquierda: `EditorReceta`; debajo, mensaje y adjuntos; «Opciones avanzadas» conserva perfil y tiempo
    (el esquema sale de ahí: ahora está en el editor).
  - `ejecutar` envía `instrucciones`, `respaldo`, esfuerzo, modelo y, si hay esquema, `salida`.
  - Botón «Guardar como API»: diálogo con nombre (misma regla que Mis APIs) y grupo; `crearPunto` con la receta
    y muestra URL, token y un `curl` a `/openai/v1/chat/completions` de esa API.
  - Si `recetaInicial` tiene nombre: carga la API con `api.puntos()`, rellena el editor, muestra
    «Editando: <nombre>» con un botón para dejar de editar, y el botón principal pasa a «Actualizar <nombre>»
    (`ajustesPunto` con instrucciones, modelo, forzar, esfuerzo, respaldo y esquema, o `null`).
  - Escuchar eventos SSE `aviso` y mostrarlos sobre la respuesta como `q-banner` informativo.
- [ ] **Paso 2:** `npm run build` → PASA.
- [ ] **Paso 3:** commit `feat(web): Probar como taller de recetas`.

### Tarea 8: Web — Mis APIs con recetas

**Archivos:**
- Modificar: `web/src/paginas/Accesos.vue`, `es.ts`, `en.ts`

- [ ] **Paso 1: implementar.**
  - El diálogo «Nueva API» usa `EditorReceta` para instrucciones, modelo, forzar, esfuerzo, respaldo y esquema;
    nombre, grupo y «Opciones avanzadas» (dirección, compatibilidad, permitidos, alias) se quedan.
  - El diálogo de ajustes usa `EditorReceta` más «permitidos».
  - En cada API: etiqueta «JSON» si tiene esquema, «Respaldo: a, b» si tiene respaldo, y botón «Abrir en Probar»
    (`recetaInicial.value = p.nombre; emit('ir', 'ejecutar')`).
- [ ] **Paso 2:** `npm run build` → PASA.
- [ ] **Paso 3:** commit `feat(web): Mis APIs con recetas y respaldo`.

### Tarea 9: ayuda, README y verificación de extremo a extremo

**Archivos:**
- Modificar: `web/src/paginas/Ayuda.vue`, `es.ts`, `en.ts`, `README.md`

- [ ] **Paso 1:** sección de Ayuda «Recetas y respaldo»: qué es una receta, cómo publicarla desde Probar, cuándo
  salta el respaldo (solo antes de producir texto) y que el tiempo límite cuenta por intento. Párrafo
  equivalente en el README, sin hitos ni pendientes.
- [ ] **Paso 2:** `go test ./...` y `npm run build` → PASAN; `graphify update .`.
- [ ] **Paso 3: verificación en vivo** con el servicio de desarrollo y el navegador integrado:
  1. En Probar: instrucciones «Devuelve un JSON con un campo saludo», esquema
     `{"type":"object","properties":{"saludo":{"type":"string"}},"required":["saludo"]}`, Enviar → JSON validado.
  2. «Guardar como API» `prueba-receta` → la URL y el token aparecen.
  3. `curl` a esa API por `/openai/v1/chat/completions` → respuesta JSON válida.
  4. «Abrir en Probar» desde Mis APIs → editor relleno y «Actualizar prueba-receta» funciona.
  5. Respaldo: API con un agente principal sin clave (`autenticacion_agente`) y un respaldo que funcione → el
     aviso aparece en Probar y el Historial muestra el agente de respaldo.
  Captura de pantalla como prueba; borrar `prueba-receta` al terminar.
- [ ] **Paso 4:** commit `docs: recetas y respaldo en la ayuda y el README`.
