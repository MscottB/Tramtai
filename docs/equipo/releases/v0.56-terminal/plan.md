# La terminal a la par de la Web UI — plan de implementación

> **Para quien implemente:** usa superpowers:subagent-driven-development (recomendado) o superpowers:executing-plans,
> tarea por tarea. Los pasos llevan casillas (`- [ ]`) para marcar el avance.

**Objetivo:** que la terminal interactiva (`tramtai`) haga lo mismo que la Web UI en sus 8 páginas, con novedades y Acerca
de, y que Tramtai pueda arrancar solo como servicio, sin ventana ni bandeja.

**Arquitectura:**
- `internal/tui` se rehace como una aplicación con un menú por grupos. Cada página es un modelo propio en
  `internal/tui/paginas/<página>` y las piezas comunes van en `internal/tui/comp`.
- La terminal habla con el servicio solo por `internal/cliente`.
- Lo que hoy solo existe en `/ui` y necesita el servicio pasa a la API de administración. Lo que la CLI ya hace en el equipo
  se comparte desde `internal`.
- La Ayuda, las plantillas y los nombres de la Web UI salen de las fuentes TS con un generador de Node y se incrustan en
  Go.

**Tecnología:** Go 1.27 (toolchain 1.27.3), Bubble Tea v2 (`charm.land/bubbletea/v2`), y como novedad
`charm.land/bubbles/v2` y `charm.land/lipgloss/v2`. Node 24 para los generadores (`web/scripts`).

**Spec:** `docs/equipo/releases/v0.56-terminal/spec.md`

## Restricciones globales

- **Textos:** todos los de Go van con `idioma.T`/`Tf`, con un literal (nunca una variable). Su inglés va en
  `internal/idioma/en_terminal.go` (o en el `en_*.go` del paquete que toque). `TestCobertura` debe pasar.
- **Dependencias:** solo se añaden `charm.land/bubbles/v2` y `charm.land/lipgloss/v2`, en la última versión v2 publicada
  hace más de 7 días. `escritorio/go.mod` no cambia salvo lo que pida la Tarea 13.
- **Menú:** el orden y los grupos son los de `web/src/menu.ts`:
  - empezar: inicio, agentes, ejecutar;
  - conectar: conectar, historial;
  - organizar: espacios, configuración.
  - Los nombres que se ven son los de la Web UI (`menu.*` de `web/src/idiomas`), y Ayuda va aparte.
- **Teclas comunes:** `/` busca, `n` crea, `e` edita, `x` borra (siempre con confirmación), `?` abre la Ayuda de la página,
  `Esc` vuelve, `q` sale (si estás dentro de un campo, `q` es una letra). Del `1` al `7` se va a cada página por su orden.
- **Tamaño mínimo:** 60×20. Si la ventana es más pequeña, solo sale «Agranda la ventana (mínimo 60×20)». Con menos de 100
  columnas el menú se pliega y se abre con Tab.
- **Color:** tema claro y oscuro según el fondo (`tea.RequestBackgroundColor`). Con `NO_COLOR` no hay color, pero se ve
  igual de bien.
- **Conexión:** la terminal nunca usa `/ui/*`. Todo pasa por `internal/cliente` con el token de servicio y la cabecera
  `Tramtai-Instancia`.
- **Si el servicio no responde:** ninguna página se cierra. Se avisa con «El servicio no responde · r reintenta» y se
  sigue.
- **Capturas:** solo con nombres de ejemplo neutros (`mi-app`, «Notas», «Soporte»), nunca rutas reales del equipo.
- **Web:** si cambias `web/src`, corre `npm run build` y mete `internal/recursos/web` en el commit. Si cambias temas,
  textos o el sitio, corre `npm run sitio` (desde la Tarea 3 también regenera `internal/tui/datos`) y mete lo que cambie.
- **Commits:** en español, terminados en una línea en blanco y `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
  Sin push.
- **Entorno:**
  - El hook GateGuard pide, antes del primer Write o Edit de cada archivo y del primer Bash: quién lo importa, qué API
    toca, qué datos maneja y la cita textual del usuario. Respóndelo y reintenta.
  - Los heredocs de Bash con comillas fallan: usa Write o Edit.

## Foco de revisión

1. **Ventana muy estrecha, muy baja o redimensionada a mitad:** nada se corta sin avisar.
   - Bajo 60×20 sale el aviso.
   - Al volver a crecer, la página sigue donde estaba (Tarea 4: `TestTamanoMinimo` y `TestRedimensionarConservaEstado`).
2. **El servicio se detiene a mitad, o el espacio cambia desde la Web UI:** la página avisa, `r` reintenta y nada se cierra.
   - Prueba: un servicio `httptest` cerrado tras cargar (Tarea 4: `TestServicioCaidoNoCierra`).
   - El flujo en vivo de Inicio se reconecta (Tarea 6: `TestRedReconecta`).
3. **Pegar textos largos o con saltos de línea** en Probar y en las instrucciones.
   - El formulario los acepta enteros y no los manda al pulsar Intro en un área de texto: se envía con `Ctrl+Enter`, como en
     la Web UI.
   - Tarea 8: `TestPegarVariasLineas` (un `tea.PasteMsg` de 3 líneas y 5000 caracteres).
4. **La terminal clásica de Windows:** caracteres de dibujo, tildes y anchos.
   - Los anchos se miden con `lipgloss.Width`, nunca con `len`.
   - Los iconos tienen alternativa ASCII si `TRAMTAI_ASCII=1` o si la consola no es UTF-8.
   - Tarea 4: `TestAnchosConTildes`.
   - Comprobación manual en la Tarea 16.
5. **La terminal y la Web UI dicen cosas distintas.**
   - Los nombres del menú y de las páginas salen de las mismas claves (Tarea 3 las exporta; Tarea 4:
     `TestNombresComoLaWeb`).
   - El consumo usa los mismos campos de `/api/v1/puntos` (Tarea 9: `TestConsumoComoLaWeb`).
   - Los errores muestran el `mensaje` del servicio sin traducirlo de nuevo.

---

### Tarea 1: Lo común de la CLI pasa a `internal`

**Archivos:**
- Crear: `internal/cliente/espacio.go` y `internal/cliente/espacio_test.go`.
- Crear: `internal/servicio/puerto.go` y `internal/servicio/puerto_test.go`.
- Modificar:
  - `cmd/tramtai/ejecutar_servicio.go:42` (usa la función movida);
  - `internal/servicio/web_servicio.go:224` (`fijarPuerto` llama a la común);
  - `cmd/tramtai/servicio.go:273-314` (`servicio puerto` llama a la común);
  - `internal/tui/ejecutar.go:133` (borrar `rutaPermitida`, usar `nucleo.Configuracion.RutaPermitida`).

**Interfaces que produce:**
- `cliente.ElegirEspacio(raiz string, activos []contrato.EstadoInstancia, preferida string, global bool) (string, error)`.
  Es la lógica tal cual de `elegirEspacio`, con la misma prioridad y el mismo `CodigoEspacioDetenido`.
- `servicio.FijarPuerto(rutaGlobal, host string, puerto int, automatico bool) error`:
  - acepta puertos de 1024 a 65535;
  - comprueba `PuertoLibre(host, puerto)` salvo que `automatico`;
  - escribe `servicio.puerto` y `servicio.puerto_automatico` con `FijarValor` en `OrigenGlobal`.
  - La usan el servicio, la CLI y la terminal.

**Pasos:**
- [ ] **Paso 1.** Prueba `TestElegirEspacio` en `internal/cliente`, que lleva los casos de la CLI:
  - la preferida gana;
  - `global` o `raiz==""` da `"global"`;
  - si hay un proyecto activo con `MismaRuta`, da su id;
  - si no hay nada, da `CodigoEspacioDetenido`.
- [ ] **Paso 2.** Prueba `TestFijarPuerto`:
  - 80 da error;
  - un puerto ocupado sin `automatico` da error;
  - `automatico` escribe `puerto_automatico: true`.
- [ ] **Paso 3.** `go test ./internal/cliente ./internal/servicio -run 'ElegirEspacio|FijarPuerto'`: FALLA porque las
  funciones no existen.
- [ ] **Paso 4.** Mueve las funciones y cambia los tres llamadores. No quedan copias.
- [ ] **Paso 5.** `go vet ./... && TRAMTAI_IDIOMA=es go test ./...`: todo en verde.
- [ ] **Paso 6.** Commit: `refactor: elegir espacio y fijar el puerto, compartidos por la CLI, el servicio y la terminal`.

### Tarea 2: Rutas de administración y cliente completo

**Archivos:**
- Modificar `internal/servicio/admin.go` (rutas nuevas con el patrón de `POST /api/v1/servicio/agentes/{agente}/conexion`).
- Crear `internal/servicio/admin_espacios_test.go`.
- Modificar `internal/cliente/cliente.go`, y partir lo nuevo en `cliente_puntos.go`, `cliente_trabajos.go`,
  `cliente_instancia.go` y `cliente_eventos.go`.
- Pruebas en `internal/cliente/cliente_test.go`.
- Documentar las rutas nuevas en `docs/api.md`, en la sección de administración.

**Rutas nuevas** (solo con el token de servicio; todas reutilizan el método del `Servicio` que ya usa `/ui`):

| Ruta | Llama a | Responde |
|---|---|---|
| `POST /api/v1/servicio/espacios/{id}/activar` | `ActivarEspacio(id)`, `cambiaronEspacios()` | `200 contrato.Espacio` |
| `POST /api/v1/servicio/espacios/{id}/detener` | `DetenerEspacio(id)`, `cambiaronEspacios()` | `204` |
| `POST /api/v1/servicio/espacios/{id}/confianza` `{confiar}` | `FijarConfianza(raiz, confiar)` | `204` |
| `POST /api/v1/servicio/espacios/{id}/ubicacion` `{ruta}` | `Ubicar(id, ruta)`, `cambiaronEspacios()` | `200 contrato.Espacio` |
| `POST /api/v1/servicio/proyectos` `{raiz, confiar?, crear_aqui}` | `AbrirProyecto`, `guardarActivas()`, `actualizarDescriptor()` | `200 contrato.Espacio` |
| `PUT /api/v1/servicio/agentes/{agente}/autenticacion` `{modo}` | `cambiarAutenticacion(agente, modo)` | `204` |

- **Validación de `confianza`:** la misma que en `espacios_ui.go:254`. Si no es un proyecto con ruta, da `400`.
- **Errores de `proyectos`:** responde `409` con `*ErrorConfianza`/`*ErrorDentroDeProyecto`, con el mismo cuerpo que
  `web_servicio.go:48-62`.

**Métodos nuevos del cliente:**
- `(c *Cliente) ActivarEspacio(ctx, id string) (contrato.Espacio, error)`
- `DetenerEspacio(ctx, id string) error`
- `Confiar(ctx, id string, confiar bool) error`
- `Ubicar(ctx, id, ruta string) (contrato.Espacio, error)`
- `AbrirProyecto(ctx, raiz string, confiar *bool, crearAqui bool) (contrato.Espacio, error)`
- `CambiarAutenticacion(ctx, agente, modo string) error`
- `Puntos(ctx, inst string) (contrato.ListaPuntos, error)`: `GET /api/v1/puntos` con `uso` y `presupuestos`. Define
  `ListaPuntos` en `internal/contrato` con los campos que devuelve hoy `web_instancia.go:92`.
- `CambiarPunto(ctx, inst, nombre string, cambios map[string]any) (contrato.EstadoPunto, error)`: `PATCH` parcial.
- `PresupuestoInstancia(ctx, inst string) (contrato.EstadoPresupuesto, error)`
- `ReiniciarPresupuestoInstancia(ctx, inst string) error`
- `TrabajosFiltrados(ctx, inst string, f FiltroTrabajos) ([]nucleo.Trabajo, error)`, con
  `type FiltroTrabajos struct{ Limite int; Punto, Grupo, Origen string; Antes time.Time; AntesID string }`. El `Trabajos`
  antiguo pasa a llamar a este.
- `Registro(ctx, inst, id string) ([]nucleo.EventoRegistro, error)`
- `VaciarTrabajos(ctx, inst string) (int, error)`
- `ConfiguracionInstancia(ctx, inst string) ([]contrato.EntradaConfiguracion, error)`
- `Heredar(ctx, inst, clave string) error`
- `FijarPredeterminado(ctx, inst, modelo, esfuerzo string) error`
- `Retencion(ctx, inst string) (contrato.Retencion, error)` y `FijarRetencion(ctx, inst string, r contrato.Retencion) error`
- `ValidarEsquema(ctx, inst string, esquema json.RawMessage) ([]string, error)`: devuelve los problemas legibles.
- `Limites(ctx, inst string) (contrato.Limites, error)`
- `AjustesConversor(ctx, inst string, a contrato.AjustesConversor) error`
- `EventosInstancia(ctx, inst string, alEvento func(tipo string, datos []byte) error) error`:
  - SSE de `GET /api/v1/eventos`;
  - el mismo lector de `id:`/`event:`/`data:` que `Eventos`, extraído a una función común `leerSSE`;
  - ignora los latidos;
  - termina con `ctx`.

Si alguno de esos tipos ya existe en `contrato` o `nucleo` con otro nombre, se usa el que existe. No se crea uno paralelo.

**Pasos:**
- [ ] **Paso 1.** En `admin_espacios_test.go`, un servicio real y una prueba por ruta:
  - `TestAdminConfiarProyecto`;
  - `TestAdminUbicar`;
  - `TestAdminAbrirProyecto409Confianza`;
  - `TestAdminAutenticacion`;
  - `TestAdminActivarDetener`;
  - `TestRutasAdminRechazanClaveDeApp`: las seis rutas con una clave de app dan `401`.
- [ ] **Paso 2.** En `cliente_test.go`, contra ese servicio:
  - `TestClientePuntosConUso`;
  - `TestClienteCambiarPunto`;
  - `TestClienteTrabajosFiltrados`: dos páginas con cursor, sin repetidos;
  - `TestClienteVaciar`;
  - `TestClienteHeredar`;
  - `TestClienteEventosInstancia`: recibe `foto` y un `trabajo` al crear uno; termina al cancelar el `ctx`.
- [ ] **Paso 3.** `go test ./internal/servicio ./internal/cliente`: FALLA.
- [ ] **Paso 4.** Implementa las rutas y los métodos.
- [ ] **Paso 5.** `go vet ./... && TRAMTAI_IDIOMA=es go test ./...`: en verde.
- [ ] **Paso 6.** Commit: `feat(servicio): espacios, proyectos y autenticación por la API de administración; el cliente,
  completo`.

### Tarea 3: Datos generados para la terminal (Ayuda, plantillas, nombres y ejemplos)

**Archivos:**
- Crear:
  - `web/scripts/terminal-datos.mjs` (generador);
  - `web/scripts/terminal-datos.ts` (funciones puras);
  - `web/scripts/terminal-datos.test.ts`.
- Modificar:
  - `web/package.json` (`"ayuda-terminal"`, y que `"sitio"` lo llame al final);
  - `.github/workflows/ci.yml:213-219` (el paso «El sitio generado coincide con sus fuentes» compara también
    `internal/tui/datos`).
- Generados: `internal/tui/datos/ayuda.json`, `internal/tui/datos/plantillas.json`, `internal/tui/datos/textos.json` y
  `internal/tui/datos/testdata/ejemplos.json`.
- Crear: `internal/tui/datos/datos.go` (`go:embed`) y `internal/tui/datos/datos_test.go`.

**Formato, que ambos lados deben respetar:**
- `ayuda.json`:
  - `{"es": [Tema], "en": [Tema]}`;
  - `Tema = {id, titulo, pagina?, resumen, bloques: [Bloque]}`;
  - `Bloque = {tipo: "texto"|"pasos"|"tabla"|"preguntas"|"errores"|"pantalla"|"web", titulo?, lineas?: [string],
    filas?: [[string]], pares?: [[string, string]], url?}`.
  - Todo es texto plano, sin Markdown.
  - Las capturas, los diagramas, las descargas y los enlaces `ir` pasan a un bloque `web` con la URL del tema en el sitio:
    `https://mscottb.github.io/Tramtai/ayuda/<id>.html`, con el mismo enlace de idioma que use hoy el sitio.
  - El bloque `recorrido` pasa a `pantalla`, con sus pasos (título y texto).
- `plantillas.json`: `{"es": [{id, nombre, instrucciones, esquema}], "en": [...]}`, que sale de `plantillas` y
  `esquemaCampos` (la misma fuente que `internal/salidas/testdata/plantillas.json`).
- `textos.json`: los nombres que tienen que coincidir con la Web UI:
  - `{"es": {"menu.inicio": "…", …}, "en": {…}}`;
  - solo las claves `menu.*` y las de una lista fija en `terminal-datos.ts`.
- `testdata/ejemplos.json`: lo que devuelve `ejemplos()` para una prueba fija (URL, clave, modelo, instrucciones, esquema y
  mensaje), solo con curl, Python y JavaScript de la API nativa. La Tarea 8 compara con él su versión en Go.

**API Go:**
- `datos.Ayuda(idioma string) []Tema`
- `datos.Plantillas(idioma string) []Plantilla`
- `datos.Texto(idioma, clave string) string`
- Tipos `Tema`, `Bloque` y `Plantilla` con los campos de arriba.

**Pasos:**
- [ ] **Paso 1.** `terminal-datos.test.ts`:
  - los 16 temas existen en es/en con el mismo número de bloques;
  - ninguna cadena tiene `<` ni `**`;
  - el tema `ejecutar` tiene su bloque `pantalla` con los pasos de su `recorrido`;
  - `textos` trae todas las `menu.*`.
- [ ] **Paso 2.** `datos_test.go`:
  - `TestAyudaCargada`: 16 temas en es y en;
  - `TestPlantillas`: 7 por idioma, cada una con un esquema JSON válido;
  - `TestTextosMenu`: están `menu.inicio`, `menu.grupo.empezar` y las demás.
- [ ] **Paso 3.** `cd web && npm test` y `go test ./internal/tui/datos`: FALLAN.
- [ ] **Paso 4.** Implementa. El generador copia los imports de `sitio-ayuda.mjs` (`i18n.ts`, `idiomas/*`,
  `ayuda/indice.ts`, `plantillas.ts`, `esquemaCampos.ts`, `ejemplos.ts`) y escribe con `\n` y orden estable, para que el
  CI pueda compararlo.
- [ ] **Paso 5.** `cd web && npm run sitio && npm test` y `go test ./internal/tui/datos`: en verde. Un segundo
  `npm run sitio` no cambia nada.
- [ ] **Paso 6.** Commit: `feat(web): la Ayuda, las plantillas y los nombres de la Web UI, generados para la terminal`.

### Tarea 4: Esqueleto nuevo de la terminal y componentes

**Archivos:**
- Reemplazar `internal/tui/{tui.go, vista.go, inicio.go, ejecutar.go, trabajos.go, accesos.go, tui_test.go}` por:
  - `internal/tui/app.go`: modelo raíz, páginas, mensajes y tamaño;
  - `internal/tui/menu.go`: grupos y orden;
  - `internal/tui/tema.go`: estilos de `lipgloss`, claro, oscuro, sin color y ASCII;
  - `internal/tui/cabecera.go`: espacio activo, cambio rápido y estado del servicio;
  - `internal/tui/app_test.go`.
- Crear:
  - `internal/tui/pagina/pagina.go`: la interfaz común y los mensajes;
  - `internal/tui/comp/{lista.go, formulario.go, tabla.go, visor.go, selector.go, confirmar.go, aviso.go}`, cada uno
    con su `_test.go`;
  - `internal/tui/paginas/prueba/servicio.go`: `prueba.Servicio(t *testing.T) (*cliente.Cliente, string)`, que levanta el
    servicio real en `httptest` con `TRAMTAI_CONFIG_GLOBAL` temporal y el adaptador `simulado`, como el
    `TestAccesosPorElServicio` actual, y devuelve el cliente y el id `"global"`.
- Las páginas viven en `internal/tui/paginas/<página>/`. Hasta su tarea, cada página muestra «Próximamente». La rama no se
  publica a medias.
- `cmd/tramtai/terminal.go` sigue llamando a `tui.Ejecutar(o)`, y `Opciones` no cambia.

**Interfaces que produce** (`internal/tui/pagina`):
```go
type Contexto struct {
    Cli         *cliente.Cliente
    Instancia   string // id del espacio activo
    Idioma      string // "es" | "en"
    Estilos     *Estilos
    Ancho, Alto int    // el área de la página, sin cabecera, menú ni pie
}
type Pagina interface {
    Titulo() string
    Iniciar(Contexto) tea.Cmd           // carga sus datos
    Update(tea.Msg) (Pagina, tea.Cmd)
    Vista() string
    Teclas() []Tecla                    // para el pie
    EnCampo() bool                      // un campo de texto tiene el foco: las teclas comunes no se aplican
    Ayuda() string                      // id del tema de la Ayuda
}
type Tecla struct{ Tecla, Accion string }
type IrA struct{ Pagina string; Datos any } // p. ej. IrA{"ejecutar", ConPunto{"mi-app"}}, IrA{"historial", FiltroPunto{"mi-app"}}
type ConPunto struct{ Nombre string }
type ConAgente struct{ Agente string }
type FiltroPunto struct{ Nombre string }
type CambioEspacio struct{ ID string }
type ErrorServicio struct{ Err error }
```
`Estilos` va en `internal/tui/pagina/estilos.go`, para que las páginas no importen `internal/tui`. Lo rellena `tema.go`.

**Componentes** (`internal/tui/comp`):
- `Lista[T]`, con `Buscar(texto)`, `Filtrar(func(T) bool)`, `Ordenar(func(a, b T) int)`, `Agrupar(func(T) string)`
  (grupos plegables) y `Seleccion() (T, bool)`.
- `Formulario`, con campos `Texto`, `Area` (varias líneas; `Ctrl+Enter` envía), `Opcion`, `Interruptor` y `Numero`, y
  `Validar func() map[string]string` por campo.
- `Visor`, con desplazamiento y búsqueda.
- `Confirmar(pregunta string) tea.Cmd`, que responde `Confirmado{}` o `Cancelado{}`.
- `Aviso(texto string, tipo TipoAviso)`, que se borra solo a los 4 s.

**Pasos:**
- [ ] **Paso 1.** Añade `bubbles` y `lipgloss` v2 (`go get`).
- [ ] **Paso 2.** Pruebas en `app_test.go`, con el modelo directo como en el `tui_test.go` actual
  (`m.Update(tecla("…"))` y `m.View().Content`):
  - `TestMenuComoLaWeb`: el orden es inicio, agentes, ejecutar, conectar, historial, espacios, configuración y después
    Ayuda;
  - `TestNombresComoLaWeb`: cada página del menú se titula con `datos.Texto(idioma, "menu.<página>")` y cada grupo con
    `menu.grupo.<grupo>`, en es y en;
  - `TestAtajosNumericos`;
  - `TestTamanoMinimo`: 59×20 enseña el aviso y 60×20 no;
  - `TestRedimensionarConservaEstado`;
  - `TestMenuPlegadoBajo100`;
  - `TestServicioCaidoNoCierra`: cerrar el `httptest` y pulsar `r` deja el aviso y la app sigue;
  - `TestTeclasComunesNoEnCampo`: con `EnCampo()`, `q` escribe una q;
  - `TestAnchosConTildes`: «Configuración» mide 13;
  - `TestSinColor`: `NO_COLOR` quita las secuencias de color.
- [ ] **Paso 3.** Las pruebas de cada componente:
  - `TestListaBuscaYFiltra`;
  - `TestListaGruposPlegables`;
  - `TestFormularioValida`;
  - `TestAreaCtrlEnter`;
  - `TestConfirmarEsc`;
  - `TestVisorBusca`.
- [ ] **Paso 4.** `go test ./internal/tui/...`: FALLA.
- [ ] **Paso 5.** Implementa. Borra los archivos viejos. Añade los textos a `en_terminal.go` y quita los que ya no se usan,
  porque `TestCobertura` falla con traducciones sin uso.
- [ ] **Paso 6.** `go vet ./... && TRAMTAI_IDIOMA=es go test ./...`: en verde.
- [ ] **Paso 7.** Commit: `feat(tui): esqueleto por páginas con menú por grupos y componentes compartidos`.

### Tareas 5 a 12: las páginas

Reglas comunes a todas:
- Cada página va en `internal/tui/paginas/<página>/` con `pagina.go`, `vista.go` y `<página>_test.go`.
- Implementan `pagina.Pagina` y se prueban contra `prueba.Servicio(t)`.
- Cada página arranca con «Cargando…» y vuelve a cargar con `r`.
- Lo que borra pide confirmación.
- `?` abre la Ayuda en su tema.
- La tarea registra la página en `internal/tui/app.go`, en lugar de «Próximamente».
- Commit al final de cada tarea: `feat(tui): <página> a la par de la Web UI`.

**Tarea 5: Agentes** (`agentes`)
- La lista viene de `cli.Capacidades`, con estado: listo, sin sesión, desconectado o sin instalar.
- Acciones:
  - `c` comprueba;
  - `d` conecta o desconecta (`ConectarAgente`);
  - `a` cambia entre suscripción y clave (`CambiarAutenticacion`, solo si el agente lo admite);
  - Intro abre los detalles y el catálogo (`Modelos`) y explica cómo instalarlo;
  - `p` lleva a Probar (`IrA{"ejecutar", ConAgente{…}}`).
- Pruebas:
  - `TestAgentesDesconectar`;
  - `TestAgentesAutenticacionSoloSiAdmite`;
  - `TestAgentesProbarConEste`.

**Tarea 6: Inicio** (`inicio`)
- Estado del espacio (dirección, la API, el servicio y los agentes listos) y contadores.
- **La red en vivo:** `EventosInstancia` en una goroutine que manda `tea.Msg`, cancelada al salir de la página. Muestra
  como mucho 12 filas, con la API, el agente, la etapa y el tiempo. Las que terminan quedan 2 s marcadas como hechas y
  4 s si fallan, con las mismas cuentas de `web/src/vivo.ts` (`limpiar`, `podar`, `reconciliar` y `etapaDe`), portadas
  a Go con sus pruebas.
- Primeros pasos con sus 4 pasos. «Ver la Ayuda de Empezar» abre el tema `empezar`.
- Avisos: espacios con problema, conversor y la sugerencia de una API propia (`app_con_clave_24h`).
- Pruebas:
  - `TestRedAparecenYSeVan`;
  - `TestRedComoMucho12`;
  - `TestRedReconecta`: cortar el flujo vuelve a conectar con espera, como `vivo.ts`;
  - `TestRedSeCierraAlSalir`;
  - `TestAvisoApiPropia`.

**Tarea 7: Historial** (`historial`)
- Filtros de origen, API, grupo y texto. «Ver más» usa `TrabajosFiltrados` con el cursor.
- Intro abre el detalle en un `comp.Visor`:
  - pestañas Solicitud, Salida y Registro (`Registro`);
  - «en vivo» con `Eventos` si sigue en marcha.
- Acciones:
  - `t` reintenta;
  - `o` corrige (reintentar con `corregir: true`, como la Web UI);
  - `c` cancela;
  - `x` elimina;
  - `V` vacía, con confirmación.
- Llega con `IrA{"historial", FiltroPunto{…}}`.
- Pruebas:
  - `TestHistorialVerMasSinRepetir`;
  - `TestHistorialFiltroPunto`;
  - `TestHistorialVaciar`;
  - `TestHistorialEnVivo`.

**Tarea 8: Probar** (`ejecutar`)
- El formulario de la receta:
  - instrucciones (área de texto), modelo (`Modelos`), forzar, esfuerzo, respaldo y reintentos;
  - salida estructurada, como campos o como JSON, con las plantillas de `datos.Plantillas` y la validación de
    `ValidarEsquema` al salir del campo;
  - perfil y tiempo máximo.
- Mensaje (área de texto) y archivos: la ruta con autocompletado, `SubirCarga` y `Limites`, que se muestran.
- Respuesta en vivo con `Eventos`:
  - `Esc` cancela;
  - `o` corrige;
  - `y` copia con OSC 52 (si la terminal no lo admite, avisa y no rompe nada);
  - `k` abre «Ver como código».
- **«Ver como código»:** curl, Python y JavaScript de la API nativa, en
  `paginas/ejecutar/codigo.go`: `Ejemplos(url, clave string, s nucleo.SolicitudEjecucion) []Ejemplo`.
- **«Guardar como API»:** el nombre y la dirección, y después `CrearPunto` o `CambiarPunto` si ya existe.
- Recientes, con «Usar de nuevo» (los 10 últimos de `TrabajosFiltrados{Origen:"terminal"}`).
- El borrador se guarda en `<datos>/terminal-borrador.json`, que se escribe al cambiar el texto (con espera) y se borra al
  enviar.
- Llega con `IrA{"ejecutar", ConPunto|ConAgente}`.
- Pruebas:
  - `TestPegarVariasLineas` (foco de revisión 3);
  - `TestPlantillaRellenaEsquema`;
  - `TestEsquemaInvalidoAvisa`;
  - `TestEnviarYCancelar`;
  - `TestGuardarComoApi`;
  - `TestEjemplosComoLaWeb`: el resultado de `Ejemplos` con la prueba fija es igual al de
    `internal/tui/datos/testdata/ejemplos.json`;
  - `TestBorradorSobreviveReinicio`.

**Tarea 9: Mis APIs** (`conectar`)
- **Pestaña APIs:**
  - `Puntos` con `uso` y `presupuestos`, en una `comp.Lista` con grupos plegables;
  - búsqueda y filtros (grupo, estado, agente, contrato, uso) y orden;
  - `g` activa o desactiva un grupo entero.
- **Crear y editar** (`n`/`e`): nombre, grupo, ruta o puerto, contratos, permitidos, alias, receta y presupuesto (día y mes:
  peticiones, tokens, USD). Usa `CrearPunto` o `CambiarPunto`.
- **Detalle** (Intro):
  - la URL y la clave, con `y` para copiar;
  - `t` regenera la clave;
  - el consumo frente al presupuesto con barras;
  - `R` reinicia el consumo;
  - `h` va al historial de esa API;
  - `p` la abre en Probar;
  - `x` la elimina.
- **Pestaña «Conexión directa»:** la clave del espacio (ver, copiar y regenerar con `RotarClave`), ejemplos de URL y el
  presupuesto del espacio (`PresupuestoInstancia`, `ReiniciarPresupuestoInstancia`).
- Pruebas:
  - `TestApisPorElServicio`: crear, activar, regenerar y eliminar, como el `TestAccesosPorElServicio` antiguo;
  - `TestApisFiltrosYGrupos`;
  - `TestEditarPresupuesto`;
  - `TestConsumoComoLaWeb`: las barras usan los mismos campos y cuentas que `BarrasPresupuesto.vue`;
  - `TestClaveDelEspacio`.

**Tarea 10: Espacios** (`espacios`)
- La lista viene de `Espacios`.
- Acciones:
  - `a` activa;
  - `d` detiene;
  - `c` confía o deja de confiar;
  - `u` busca la carpeta;
  - `x` olvida (con confirmación; nunca borra la carpeta del proyecto);
  - `n` abre un proyecto, con autocompletado de rutas del sistema.
- El `409 confianza_requerida` enseña las claves y pregunta si confiar. `dentro_de_proyecto` ofrece abrir el de arriba.
- Al cambiar de espacio emite `CambioEspacio`.
- Pruebas:
  - `TestEspaciosConfiar`;
  - `TestEspaciosAbrirConConfianza`;
  - `TestEspaciosOlvidarNoBorraCarpeta`;
  - `TestCambioEspacioRecargaPaginas`.

**Tarea 11: Ajustes** (`configuracion`), más novedades y Acerca de
- **Pestaña «Este espacio»:**
  - modelo y esfuerzo predeterminados (`FijarPredeterminado`);
  - configuración efectiva con su origen y `h` para heredar (`Heredar`);
  - retención (`FijarRetencion`);
  - datos: eliminar APIs y vaciar, con confirmación.
- **Pestaña «Todo Tramtai»:**
  - el puerto, con `servicio.FijarPuerto` de la Tarea 1, en el equipo; avisa de que hay que reiniciar el servicio;
  - el conversor (`Conversor`, `PrepararConversor`, `AjustesConversor`);
  - el arranque automático: activar o desactivar la app con `autoarranque` (la Tarea 13 añade «Solo el servicio»);
  - buscar actualizaciones, que lleva a Novedades;
  - Acerca de: versión, licencia, autor y enlaces, lo mismo que la Web UI.
- **Novedades** (`internal/tui/novedades.go`):
  - al arrancar, `actualizacion.Comprobador` en segundo plano;
  - si hay versión nueva, un aviso en la cabecera;
  - `u` abre las notas en el idioma de la terminal (`Novedades`) y ofrece instalar, con el avance de `Descargar`.
  - La lógica de `cmd/tramtai/actualizar.go` pasa a `internal/actualizacion` como
    `Aplicar(ctx context.Context, v Version, avance Avance) error`, que llaman la CLI y la terminal.
- Pruebas:
  - `TestAjustesHeredar`;
  - `TestAjustesPredeterminado`;
  - `TestPuertoFueraDeRango`;
  - `TestNovedadesAviso`, con un `Comprobador` falso.

**Tarea 12: Ayuda** (`ayuda`)
- La lista de temas sale de `datos.Ayuda(idioma)`, con búsqueda (`/`) que no distingue tildes ni mayúsculas.
- El tema se ve en un `comp.Visor`: pasos numerados, tablas, preguntas, errores con «Qué hago» y «Qué verás en
  pantalla».
- `w` abre el bloque `web` en el navegador con `internal/navegador.Abrir`. Si no hay navegador, enseña la URL para
  copiarla.
- El `?` de cualquier página abre su tema directamente (`Pagina.Ayuda()`).
- Pruebas:
  - `TestAyudaBuscaSinTildes`;
  - `TestInterrogacionAbreSuTema`, para cada una de las 7 páginas;
  - `TestAyudaVerEnLaWebSinNavegador`.

### Tarea 13: Arranque «Solo el servicio»

**Archivos:**
- Modificar `internal/autoarranque/{autoarranque.go, autoarranque_windows.go, autoarranque_darwin.go,
  autoarranque_linux.go, autoarranque_otros.go}` y sus pruebas.
- Modificar `escritorio/main.go`: la opción `--solo-servicio`, que se comprueba antes de iniciar Wails.
- Modificar `cmd/tramtai/servicio.go:245-271`: `servicio autoarranque activar --modo app|servicio`.
- Modificar `internal/servicio/web_servicio.go:66`: `/ui/autoarranque` acepta `{activo, modo}`.
- Web UI: `web/src/paginas/Configuracion.vue` (la elección «La app en la bandeja» / «Solo el servicio») y los textos en
  `idiomas/{es,en}.ts`.
- Terminal: `internal/tui/paginas/configuracion` (la misma elección).

**Interfaces:**
- `type Modo string`, con `const ModoApp Modo = "app"` y `ModoServicio Modo = "servicio"`.
- `autoarranque.Elegir(modo Modo) error` calcula las rutas él mismo y registra:
  - **Windows:** ModoApp como hoy. ModoServicio registra `"<tramtai-escritorio.exe>" --solo-servicio`, que es un programa
    sin consola: arranca el servicio en segundo plano con `cliente.IniciarEnSegundoPlano` y termina, sin abrir la ventana
    ni la bandeja.
  - **macOS:** el mismo LaunchAgent con `ProgramArguments [<tramtai>, servicio, iniciar, -d]`.
  - **Linux:** el mismo `.desktop` con `Exec="<tramtai>" servicio iniciar -d`.
- `autoarranque.Configurado() (Modo, error)` devuelve `""` si no hay nada registrado.
- `Activar(escritorio)` deja de ser público, porque `Elegir(ModoApp)` lo sustituye para los llamadores.

**Pasos:**
- [ ] **Paso 1.** Pruebas en cada sistema, con las rutas cambiadas a una carpeta temporal mediante las variables que ya
  usen sus pruebas:
  - `TestElegirServicioWindows`: el valor de `Run` contiene `--solo-servicio`;
  - `TestElegirServicioLinux`: el `Exec` contiene `servicio iniciar -d`;
  - `TestElegirServicioDarwin`: los `ProgramArguments`;
  - `TestConfiguradoDistingueModos`.
- [ ] **Paso 2.** `TestAutoarranqueUIModo`: `/ui/autoarranque` con `modo: "servicio"`.
- [ ] **Paso 3.** Las pruebas fallan.
- [ ] **Paso 4.** Implementa: la terminal, la CLI y la Web UI ofrecen la misma elección.
- [ ] **Paso 5.** `go vet ./... && TRAMTAI_IDIOMA=es go test ./...`; `cd escritorio && go vet ./... && go test ./...`;
  `cd web && npm test && npm run build`. Todo en verde.
- [ ] **Paso 6.** Comprobación manual en Windows:
  - `tramtai servicio autoarranque activar --modo servicio`;
  - el valor de `HKCU\…\Run` es el esperado;
  - al ejecutarlo a mano no se abre ninguna ventana y `tramtai servicio estado` dice «en marcha».
  - Después, desactívalo.
- [ ] **Paso 7.** Commit: `feat: arranque «Solo el servicio», sin ventana ni bandeja`.

### Tarea 14: Capturas de la terminal

**Archivos:**
- Crear `internal/tui/captura/main.go`, un programa que se ejecuta con `go run ./internal/tui/captura <página> <tema>
  <idioma> <ancho> <alto>`:
  - levanta el servicio de prueba con los datos de demostración que usa `capturas.mjs` (APIs `mi-app`, «Notas»,
    «Soporte»);
  - pinta la página y escribe la salida ANSI en stdout.
- Modificar `web/scripts/capturas.mjs`:
  - un modo nuevo `--terminal`, incluido también en la corrida completa;
  - convierte el ANSI a HTML (SGR 16, 256 y truecolor, en una tipografía monoespaciada, con el fondo del tema) con una
    función pura, `ansiAHtml` en `capturas-lista.ts`;
  - hace la captura con Edge, como las demás.
- Generados: `site/capturas/terminal-<inicio|apis>-<claro|oscuro>-<es|en>-<960|1440>.webp`.

**Pasos:**
- [ ] **Paso 1.** `capturas.test.ts`:
  - `ansiAHtml`: un color, negrita y restablecer;
  - las 16 capturas de la terminal existen (2 páginas × 2 temas × 2 idiomas × 2 anchos);
  - el peso total de las capturas de la portada sigue por debajo de su tope.
- [ ] **Paso 2.** FALLA.
- [ ] **Paso 3.** Implementa y corre `npm run capturas -- --terminal`. Abre 2 capturas y comprueba que solo hay nombres
  de ejemplo.
- [ ] **Paso 4.** `npm test`: en verde.
- [ ] **Paso 5.** Commit: `feat(sitio): capturas de la terminal`.

### Tarea 15: Ayuda, web y documentación

**Archivos:**
- `web/src/ayuda/temas/*.ts` y `idiomas/ayuda.{es,en}.ts`:
  - Empezar: «Solo con la terminal»;
  - cada tema de página: una línea «En la terminal: …» con su atajo;
  - Ajustes: «Solo el servicio».
- `site/index.html` y `site/estilo.css`:
  - la novedad «Todo desde la terminal»;
  - una tarjeta en Funciones;
  - «Así se ve» con las capturas de la Tarea 14 (`srcset` 960/1440, tamaño declarado y carga diferida).
- `site/desarrolladores.html` y `docs/desarrolladores.md`: la sección «Administrar desde la terminal» (atajos,
  `tramtai terminal` y SSH al equipo de Tramtai), en es/en.
- `README.md`, `docs/inicio.md` y `docs/operacion.md`: la terminal completa, «Solo el servicio» y los atajos.
- `docs/equipo/desarrollo.md`: «Terminal: cómo añadir una página» (interfaz `Pagina`, helper `prueba.Servicio`, textos en
  `en_terminal.go`) y la regla «cada función nueva llega también a la terminal».
- `docs/equipo/pendientes.md`: la v0.56 ✅.
- `docs/api.md`: las rutas de administración de la Tarea 2, si no se documentaron allí.

**Pasos:**
- [ ] **Paso 1.** Escribe los textos. Las pruebas que ya existen los limitan: `estilo.test` (títulos ≤ 60, textos ≤ 200),
  `sitio.test` y `portada.test`. Añade a `portada.test` la prueba de la novedad y de las capturas de la terminal.
- [ ] **Paso 2.** `cd web && npm test && npm run build && npm run sitio`, y un segundo `npm run sitio` sin cambios.
  `go test ./internal/tui/datos` (la Ayuda regenerada).
- [ ] **Paso 3.** Commit: `docs: la terminal completa en la Ayuda, la web y la documentación`.

### Tarea 16: Comprobación de punta a punta

- [ ] **Paso 1.** `go vet ./...` (también con `-tags conversor`), `TRAMTAI_IDIOMA=es go test ./...`, el `go test ./...` de
  `escritorio`, y en `web` `npm test`, `npm run build` y `npm run sitio` sin nada pendiente. Después, `graphify update .`.
- [ ] **Paso 2.** Con el binario nuevo y datos portables (no los del usuario), en Windows Terminal y en PowerShell clásico
  (`conhost`):
  - recorre las 8 páginas haciendo lo mismo que en la Web UI y comprueba que los nombres, el consumo y los errores
    coinciden;
  - repite a 60×20, a 80 columnas y en una ventana grande, en claro y oscuro, con `TRAMTAI_IDIOMA=en`, con `NO_COLOR=1`
    y con `TRAMTAI_ASCII=1`;
  - detén el servicio a mitad y vuelve con `r`;
  - pega un texto de varias líneas en Probar;
  - cambia de espacio desde la Web UI con la terminal abierta.
- [ ] **Paso 3.** Si hay un Linux disponible por SSH, `tramtai` por SSH. Si no, dilo en el informe.
- [ ] **Paso 4.** Arreglos con su prueba (fallo primero), en un commit por arreglo: `fix(tui): …`.
