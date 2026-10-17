# Actualizaciones: plan de implementación

> **Para agentes:** SUB-SKILL REQUERIDA: superpowers:executing-plans (o subagent-driven-development), tarea por
> tarea. Los pasos usan casillas (`- [ ]`).

**Objetivo:** borrar los instaladores que dejan las actualizaciones y mostrar, tras actualizar, un diálogo con las
novedades de todas las versiones saltadas.

**Arquitectura:** dos funciones nuevas en `internal/actualizacion` (`LimpiarDescargas` y `Novedades`), una ruta
interna `/ui/actualizacion/novedades` en `internal/servicio/web.go` y un diálogo en `web/src/App.vue` que reemplaza
el aviso corto. Sin cambios en el instalador ni en el esquema de datos.

**Stack:** Go 1.27, Vue 3 + Quasar (vue-tsc).

**Spec:** docs/equipo/actualizaciones.md

## Restricciones globales

- Solo se habla con GitHub y solo con el repositorio `MscottB/Tramtai`; las URL de página que no sean
  `https://github.com/MscottB/Tramtai/` se descartan (como en `Ultima`).
- Con `servicio.buscar_actualizaciones` desactivado no se consulta GitHub.
- Notas recortadas a 6000 runas; como mucho 10 versiones; carpetas de descarga borradas solo si tienen más de
  10 minutos.
- Textos nuevos en `web/src/idiomas/es.ts` y `en.ts` con las mismas claves.
- Tras cambiar Go: `go vet ./...` y `go test ./...`; tras cambiar la Web UI: `npm run build` en `web/` (escribe
  `internal/recursos/web`, que se versiona).

## Puntos de revisión

- **`desde` más nuevo que la instalada** (se volvió a una versión anterior): `Novedades` devuelve una lista vacía y
  el diálogo sale sin notas, nunca con versiones futuras. Prueba en la tarea 2.
- **Misma versión con otra fecha** (`v0.36.1+16102026` contra `v0.36.1+17102026`): no cuenta como versión nueva,
  porque `MasNueva` ignora la fecha. Prueba en la tarea 2.
- **GitHub limita o falla** (403, 500, JSON roto): `Novedades` devuelve error, la ruta responde 502 y la Web UI
  muestra el diálogo sin notas. Prueba del 403 en la tarea 2.
- **Una página de versión ajena en `html_url`**: queda `url` vacía y la Web UI no la enlaza. Prueba en la tarea 2.
- **Una carpeta en uso** (el instalador aún corriendo en Windows): `LimpiarDescargas` la salta sin error y la borra
  en el próximo arranque. No se puede bloquear un archivo de forma portable en una prueba: se revisa leyendo el
  código (los errores de `os.RemoveAll` se ignoran).

---

### Tarea 1: Limpieza de descargas

**Archivos:**
- Modificar: `internal/actualizacion/actualizacion.go` (función nueva y llamada al inicio de `Descargar`)
- Modificar: `internal/servicio/servicio.go` (`Nuevo`, línea ~111)
- Modificar: `cmd/tramtai/actualizar.go:48`, `internal/servicio/actualizacion.go:112` (prefijo común)
- Prueba: `internal/actualizacion/actualizacion_test.go`

**Interfaces:**
- Produce: `func LimpiarDescargas(antes time.Time)` y `const PrefijoCarpeta = "tramtai-actualizacion"`, que usan
  también los dos `os.MkdirTemp` de las descargas.

- [ ] **Paso 1: Prueba que falla**

```go
func TestLimpiarDescargas(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMPDIR", tmp)
	vieja, reciente, ajena := filepath.Join(tmp, "tramtai-actualizacion111"), filepath.Join(tmp, "tramtai-actualizacion222"), filepath.Join(tmp, "otra-cosa")
	for _, d := range []string{vieja, reciente, ajena} {
		os.MkdirAll(d, 0o755)
		os.WriteFile(filepath.Join(d, "instalador.exe"), []byte("x"), 0o644)
	}
	hace := time.Now().Add(-time.Hour)
	os.Chtimes(vieja, hace, hace)
	os.Chtimes(ajena, hace, hace)
	LimpiarDescargas(time.Now().Add(-10 * time.Minute))
	// esperado: vieja no existe; reciente y ajena siguen
}
```

- [ ] **Paso 2: Ejecutar y ver que falla**

Ejecutar: `go test ./internal/actualizacion -run TestLimpiarDescargas`
Esperado: FAIL, `undefined: LimpiarDescargas`.

- [ ] **Paso 3: Implementar `LimpiarDescargas(antes time.Time)` en `actualizacion.go`**

`os.ReadDir(os.TempDir())`; para cada carpeta con prefijo `PrefijoCarpeta` y `ModTime()` anterior a `antes`,
`os.RemoveAll` ignorando el error. Llamarla al principio de `Descargar` con `time.Now().Add(-10 * time.Minute)`: cubre
la app y `tramtai actualizar`, que descargan por ahí, y la carpeta de esa descarga es nueva y no se toca. Cambiar los
dos `os.MkdirTemp("", "tramtai-actualizacion")` por `os.MkdirTemp("", actualizacion.PrefijoCarpeta)`.

- [ ] **Paso 4: Llamarla al arrancar el servicio**

En `servicio.Nuevo`: `go actualizacion.LimpiarDescargas(time.Now().Add(-10 * time.Minute))`, en segundo plano para no
retrasar el arranque.

- [ ] **Paso 5: Ejecutar las pruebas**

Ejecutar: `go test ./internal/actualizacion ./internal/servicio ./cmd/tramtai`
Esperado: PASS.

- [ ] **Paso 6: Commit**

```bash
git add internal/actualizacion internal/servicio/servicio.go internal/servicio/actualizacion.go cmd/tramtai/actualizar.go
git commit -m "fix(actualizacion): borrar los instaladores que dejan las actualizaciones"
```

### Tarea 2: Novedades desde GitHub

**Archivos:**
- Modificar: `internal/actualizacion/actualizacion.go` (variable `urlVersiones`, función `Novedades`)
- Modificar: `internal/servicio/web.go` (ruta nueva junto a `/ui/actualizacion/progreso`, línea ~329)
- Modificar: `internal/idioma/en_actualizacion.go` (traducción de los mensajes de error nuevos, si los hay)
- Prueba: `internal/actualizacion/actualizacion_test.go`, `internal/servicio/web_test.go` (`TestWebUI`)

**Interfaces:**
- Consume: `Version`, `MasNueva`, `numeros`, `recortar`, `bajar` (ya existen).
- Produce: `func Novedades(ctx context.Context, desde, hasta string) ([]Version, error)`; ruta
  `GET /ui/actualizacion/novedades?desde=<versión>` → `{"versiones": [{"version", "url", "notas"}]}`.

- [ ] **Paso 1: Pruebas que fallan**

Un servidor `httptest` en `/versiones` sirve un arreglo con `v0.37.0+1`, `v0.36.1+1`, `v0.36.0+1` (con `html_url`
de otro sitio), `v0.35.1+1`, un borrador (`"draft": true`, `v0.36.2+1`) y una previa (`"prerelease": true`,
`v0.36.3+1`); `urlVersiones` apunta a él durante la prueba (como hace `servidor` con `urlUltima`).

```go
func TestNovedades(t *testing.T) {
	// servidorVersiones(t, http.StatusOK)
	vs, err := Novedades(context.Background(), "v0.35.1+9", "v0.36.1+9")
	// esperado: err == nil; etiquetas == ["v0.36.1+1", "v0.36.0+1"] (sin borrador, previa ni la v0.37.0)
	// esperado: la v0.36.0+1 trae Pagina == "" (html_url ajena)
	vs, _ = Novedades(context.Background(), "v0.37.0+1", "v0.36.1+1") // se volvió atrás
	// esperado: len(vs) == 0
	vs, _ = Novedades(context.Background(), "v0.36.1+16102026", "v0.36.1+17102026") // misma versión, otra fecha
	// esperado: len(vs) == 0
	vs, _ = Novedades(context.Background(), "", "v0.36.1+1") // sin desde: solo la instalada
	// esperado: etiquetas == ["v0.36.1+1"]
}

func TestNovedadesTope(t *testing.T) {
	// 15 versiones de v0.20.0 a v0.34.0: Novedades("v0.1.0", "v0.34.0") devuelve 10, la primera v0.34.0
}

func TestNovedadesFallo(t *testing.T) {
	// el servidor responde 403: Novedades devuelve error
}
```

En `TestWebUI`, con la sesión ya abierta (la búsqueda está desactivada en `entorno`):

```go
if r, cuerpo := n.pedir("GET", "/ui/actualizacion/novedades?desde=v0.1.0", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, `"versiones":[]`) {
	t.Fatalf("sin búsqueda no se consulta GitHub: %d %s", r.StatusCode, cuerpo)
}
```

- [ ] **Paso 2: Ejecutar y ver que fallan**

Ejecutar: `go test ./internal/actualizacion -run Novedades` y `go test ./internal/servicio -run TestWebUI`
Esperado: FAIL (`undefined: Novedades`; la ruta no existe).

- [ ] **Paso 3: Implementar `Novedades` en `actualizacion.go`**

`urlVersiones = "https://api.github.com/repos/" + repositorio + "/releases?per_page=30"` junto a `urlUltima`.
`bajar(ctx, urlVersiones, 4<<20, "application/vnd.github+json")`; decodificar `tag_name`, `html_url`, `body`,
`draft` y `prerelease`; quedarse con las publicadas de número válido que cumplan
`MasNueva(etiqueta, desde) && !MasNueva(etiqueta, hasta)` o, si `desde` no tiene número válido, las del mismo número
que `hasta`; ordenar de la más nueva a la más vieja (`slices.SortFunc` comparando `numeros`) y cortar en 10. `Pagina`
solo con el prefijo del repositorio y `Notas` con `recortar(…, 6000)`, como en `Ultima`: extraer ese armado a una
función común en vez de repetirlo.

- [ ] **Paso 4: Ruta en `web.go`**

Cualquier sesión (`s.sesionDe(r)`, como `/ui/actualizacion/progreso`). Si `!s.buscar.Load()`, responder
`{"versiones": []}` sin consultar. Si no, `actualizacion.Novedades(ctx, r.URL.Query().Get("desde"), version.Texto())`
con un plazo de 8 s; un error responde 502 con el mensaje. La lista vacía sale como `[]`, nunca `null`.

- [ ] **Paso 5: Ejecutar las pruebas**

Ejecutar: `go test ./internal/actualizacion ./internal/servicio ./internal/idioma`
Esperado: PASS (la prueba de idioma exige la traducción de todo mensaje nuevo).

- [ ] **Paso 6: Commit**

```bash
git add internal/actualizacion internal/servicio/web.go internal/servicio/web_test.go internal/idioma
git commit -m "feat(actualizacion): novedades de las versiones saltadas"
```

### Tarea 3: Diálogo de novedades en la Web UI

**Archivos:**
- Modificar: `web/src/api.ts` (tipo y método)
- Modificar: `web/src/App.vue` (el `watch` de `sesion.value?.version`, línea ~380, y un `q-dialog` nuevo)
- Modificar: `web/src/idiomas/es.ts`, `web/src/idiomas/en.ts`

**Interfaces:**
- Consume: la ruta de la tarea 2.
- Produce: `type Novedad = { version: string; url?: string; notas?: string }` y
  `api.novedades(desde: string): Promise<Novedad[]>` (`GET /ui/actualizacion/novedades?desde=` con
  `encodeURIComponent`, devuelve `r.versiones`).

- [ ] **Paso 1: Textos**

`actualizacion.hecha` se queda como título del diálogo. Claves nuevas:
- `actualizacion.sinNotas`: «Las notas de esta versión están en la página de versiones.» / «The notes for this
  version are on the releases page.»
- `actualizacion.verTodas`: «Ver todas las versiones» / «See all releases»
- `comun.aceptar`: «Aceptar» / «OK»

- [ ] **Paso 2: Diálogo en `App.vue`**

En el `watch` actual, donde hoy llama a `avisar(t('actualizacion.hecha', …))`: si `vista && vista !== version &&
/^v\d/.test(version)`, abrir el diálogo con `novedades = await api.novedades(vista).catch(() => [])`. Se mantienen la
escritura de `tramtai.version` y el `try/catch` de `localStorage`.

El diálogo (`q-dialog`, ancho 540 px como el de actualizar):
- título `t('actualizacion.hecha', { version })`;
- sin novedades: `t('actualizacion.sinNotas')`;
- con una: `NotasVersion` con sus notas;
- con varias: un `q-expansion-item` por versión con `label` igual a su número, `default-opened` solo en la primera, y
  `NotasVersion` dentro;
- acciones: enlace plano `t('actualizacion.verTodas')` a `https://github.com/MscottB/Tramtai/releases`
  (`target="_blank" rel="noopener"`) y `q-btn` `t('comun.aceptar')` con `v-close-popup`.

- [ ] **Paso 3: Compilar**

Ejecutar: `cd web && npm run build`
Esperado: termina con «built in», sin errores de vue-tsc.

- [ ] **Paso 4: Verificar en el navegador**

Con el servicio de prueba (puerto 17999) y su búsqueda de actualizaciones activada:
- `localStorage.setItem('tramtai.version', 'v0.30.0+1')` y recargar: el diálogo muestra de la v0.31.0 a la
  instalada (como mucho 10), la primera abierta; Aceptar lo cierra; al recargar no vuelve a salir.
- Con la búsqueda desactivada, repetir: el diálogo sale con «Las notas de esta versión…» y el enlace.
- Claro y oscuro, y a 375 px de ancho, sin desbordes.

- [ ] **Paso 5: Commit**

```bash
git add web/src internal/recursos/web
git commit -m "feat(web): novedades de la versión al actualizar"
```

### Tarea 4: Documentación y cierre

**Archivos:**
- Modificar: `docs/operacion.md` (sección «Actualizaciones», línea ~64)
- Modificar: `docs/equipo/pendientes.md`

- [ ] **Paso 1: `operacion.md`**

Dos frases en «Actualizaciones»: al arrancar y antes de cada descarga se borran los instaladores de actualizaciones
anteriores (carpetas `tramtai-actualizacion*` del temporal con más de 10 minutos); tras actualizar, la Web UI
muestra las novedades de las versiones saltadas (hasta 10), si la búsqueda de actualizaciones está activada.

- [ ] **Paso 2: `pendientes.md`**

- Quitar de «Rediseño (posterior)» la frase de la pantalla en blanco y anotar que quedó resuelta en v0.36.0 (avance
  al instante), verificado por el usuario al actualizar de la 0.36.0 a la 0.36.1.
- Hoja de ruta: fila nueva antes de la de grupos, «Actualizaciones: se borran los instaladores descargados y, tras
  actualizar, un diálogo con las novedades de las versiones saltadas», sin número (lo pone el script al etiquetar).

- [ ] **Paso 3: Comprobaciones finales**

Ejecutar: `go vet ./...`, `go test ./...`, `cd escritorio && go vet ./...`, `cd web && npm run build`,
`graphify update .`
Esperado: todo sin errores; `git status` limpio salvo lo que se versiona.

- [ ] **Paso 4: Commit**

```bash
git add docs
git commit -m "docs: limpieza de descargas y novedades tras actualizar"
```
