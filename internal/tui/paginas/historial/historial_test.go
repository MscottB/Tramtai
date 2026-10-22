package historial_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/historial"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas del Historial van por su interfaz: teclas, los mensajes de pagina y comp, y lo que enseña Vista(), contra
// el servicio de prueba. El sondeo y la reconexión esperan a que la prueba haga pasar el tiempo (prueba.Reloj).

var ctx = context.Background()

var aviso = prueba.Aviso

// nueva crea el Historial con un reloj de prueba.
func nueva(lengua string) (pagina.Pagina, *prueba.Reloj) {
	r := prueba.NuevoReloj()
	return historial.NuevaConReloj(lengua, r.Ahora, r.Espera), r
}

// abrir inicia el Historial con ese contexto: arranca con «Cargando…» y espera la lista.
func abrir(t *testing.T, c pagina.Contexto) (*prueba.App, *prueba.Reloj) {
	t.Helper()
	p, r := nueva(c.Idioma)
	a := prueba.Conducir(t, p)
	cmd := p.Iniciar(c)
	if v := p.Vista(); c.Cli != nil && !strings.Contains(v, strings.TrimRight(datos.Texto(c.Idioma, "comun.cargando"), ".…")) {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	a.Correr(cmd)
	return a, r
}

// abierta abre el Historial contra el servicio, en 60×17.
func abierta(t *testing.T, cli *cliente.Cliente, id string) *prueba.App {
	t.Helper()
	a, _ := abrir(t, prueba.Contexto(t, cli, id))
	return a
}

// alto es un contexto de 60 columnas y ese alto.
func alto(t *testing.T, cli *cliente.Cliente, id string, filas int) pagina.Contexto {
	c := prueba.Contexto(t, cli, id)
	c.Alto = filas
	return c
}

// teclasPie son las teclas de la página como se leen en el pie: «m Ver más».
func teclasPie(a *prueba.App) []string {
	var r []string
	for _, k := range a.P.Teclas() {
		r = append(r, k.Tecla+" "+k.Accion)
	}
	return r
}

// enPestana dice si la vista enseña el detalle con esa pestaña elegida (su nombre entre corchetes).
func enPestana(a *prueba.App, clave string) bool {
	return strings.Contains(a.Vista(), "["+datos.Texto("es", clave)+"]")
}

// enVivo dice si el detalle enseña que se sigue en vivo.
func enVivo(a *prueba.App) bool { return strings.Contains(a.Vista(), "en vivo") }

// filas son los textos de la lista que empiezan por prefijo, en el orden en que se ven.
func filas(a *prueba.App, prefijo string) []string {
	var r []string
	for _, l := range strings.Split(a.Vista(), "\n") {
		if i := strings.Index(l, prefijo); i >= 0 {
			r = append(r, strings.Fields(l[i:])[0]+" "+strings.Fields(l[i:])[1])
		}
	}
	return r
}

// cuantas son las filas de la lista que enseñan alguno de los textos.
func cuantas(a *prueba.App, textos ...string) int {
	n := 0
	for _, l := range strings.Split(a.Vista(), "\n") {
		if slices.ContainsFunc(textos, func(x string) bool { return strings.Contains(l, x) }) {
			n++
		}
	}
	return n
}

// terminar espera a que el trabajo acabe.
func terminar(t *testing.T, cli *cliente.Cliente, inst, id string) {
	t.Helper()
	if err := cli.Eventos(ctx, inst, id, 0, func(int, string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
}

// crear manda una petición y espera a que acabe.
func crear(t *testing.T, cli *cliente.Cliente, inst, texto string) nucleo.Trabajo {
	t.Helper()
	tr, err := cli.Crear(ctx, inst, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}})
	if err != nil {
		t.Fatal(err)
	}
	terminar(t, cli, inst, tr.ID)
	return tr
}

// crearVarias manda n peticiones («petición-000»…) de quince en quince, lo que cabe en la cola, y espera a que acaben.
func crearVarias(t *testing.T, cli *cliente.Cliente, inst string, n int) {
	t.Helper()
	for desde := 0; desde < n; desde += 15 {
		var ids []string
		for i := desde; i < min(desde+15, n); i++ {
			tr, err := cli.Crear(ctx, inst, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: fmt.Sprintf("petición-%03d", i)}}})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, tr.ID)
			time.Sleep(time.Millisecond) // cada una con su hora: la lista las ordena por cuándo se crearon
		}
		for _, j := range ids {
			terminar(t, cli, inst, j)
		}
	}
}

// crearInvalida manda una petición cuya respuesta no cumple el esquema (el agente simulado devuelve el mensaje tal cual)
// y la conserva, para poder corregirla.
func crearInvalida(t *testing.T, cli *cliente.Cliente, inst string) nucleo.Trabajo {
	t.Helper()
	tr, err := cli.Crear(ctx, inst, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: `{"total":"diez"}`}},
		Salida: nucleo.ConfigSalida{Formato: "json", ConservarSalidaInvalida: true,
			Esquema: json.RawMessage(`{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	terminar(t, cli, inst, tr.ID)
	return tr
}

// conBloqueado levanta el servicio con el agente bloqueado y manda una petición «esperar»: vuelve cuando el agente ya la tiene.
func conBloqueado(t *testing.T) (*cliente.Cliente, string, *prueba.Bloqueado, nucleo.Trabajo) {
	t.Helper()
	cli, id, b := prueba.ConBloqueado(t)
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "esperar"}}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-b.Entro:
	case <-time.After(5 * time.Second):
		t.Fatal("el agente no recibió la petición")
	}
	return cli, id, b, tr
}

// seguido es el trabajo con que se va a Probar (reintentar y corregir lo siguen allí); "" si no se va.
func seguido(fuera []tea.Msg) string {
	for _, m := range fuera {
		if ir, ok := m.(pagina.IrA); ok && ir.Pagina == pagina.PaginaEjecutar {
			if s, ok := ir.Datos.(pagina.SeguirTrabajo); ok {
				return s.ID
			}
		}
	}
	return ""
}

func pregunta(fuera []tea.Msg) (comp.Pregunta, bool) {
	for _, m := range fuera {
		if q, ok := m.(comp.Pregunta); ok {
			return q, true
		}
	}
	return comp.Pregunta{}, false
}

func errorServicio(fuera []tea.Msg) bool {
	return slices.ContainsFunc(fuera, func(m tea.Msg) bool { _, ok := m.(pagina.ErrorServicio); return ok })
}

// cerrado espera a que no quede ningún flujo de eventos abierto: el seguimiento en vivo se soltó.
func cerrado(t *testing.T, a *prueba.App) {
	t.Helper()
	a.Hasta(func() bool { return prueba.FlujosAbiertos() == 0 })
}

func TestHistorialCarga(t *testing.T) {
	cli, id := prueba.Servicio(t)
	for _, texto := range []string{"primera", "segunda", "tercera"} {
		crear(t, cli, id, texto)
	}
	a := abierta(t, cli, id)
	v := a.Vista()
	for _, quiere := range []string{"Estado: Todas", "Origen: Todos los orígenes", "Completado", "tercera", "primera", "Mostrando las 3 más recientes", "hace "} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if !strings.Contains(v, "▸ ✓ Completado") {
		t.Fatalf("el cursor está en la primera fila:\n%s", v)
	}
	// Los más nuevos primero.
	if i, j := strings.Index(v, "tercera"), strings.Index(v, "primera"); i > j {
		t.Fatalf("la más nueva va primero:\n%s", v)
	}
	if a.P.Titulo() != "Historial" || a.P.Ayuda() != "historial" || a.P.EnCampo() {
		t.Fatalf("título %q, ayuda %q, en campo %v", a.P.Titulo(), a.P.Ayuda(), a.P.EnCampo())
	}
}

func TestHistorialSinPeticiones(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	v := a.Vista()
	for _, quiere := range []string{"Aún no hay peticiones", "p Ir a Probar"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	ir, ok := a.Pulsar("p")[0].(pagina.IrA)
	if !ok || ir.Pagina != pagina.PaginaEjecutar {
		t.Fatalf("p lleva a Probar: %+v", ir)
	}
	// Con un filtro el vacío es otro y se ofrece quitarlo.
	a.Pulsar("e")
	v = a.Vista()
	if !strings.Contains(v, "No hay peticiones con este filtro") || !strings.Contains(v, "l Quitar filtros") {
		t.Fatalf("vacío con filtro:\n%s", v)
	}
	if fuera := a.Pulsar("p"); len(fuera) != 0 {
		t.Fatalf("p solo vale sin filtros: %v", fuera)
	}
	a.Pulsar("l")
	if v := a.Vista(); !strings.Contains(v, "Estado: Todas") || !strings.Contains(v, "Aún no hay peticiones") {
		t.Fatalf("l quita los filtros:\n%s", v)
	}
}

// TestHistorialVerMasSinRepetir: «Ver más» pide la página siguiente con el cursor del último trabajo (antes y antes_id): cada
// petición sale una sola vez y en orden, hasta que no queda más. Una página es de 200.
func TestHistorialVerMasSinRepetir(t *testing.T) {
	cli, id := prueba.Servicio(t)
	const total = 401
	crearVarias(t, cli, id, total)
	var esperado []string
	for i := total - 1; i >= 0; i-- {
		esperado = append(esperado, fmt.Sprintf("petición-%03d", i))
	}
	a, _ := abrir(t, alto(t, cli, id, 600))
	lista := func() []string {
		var r []string
		for _, f := range filas(a, "petición-") {
			r = append(r, strings.Fields(f)[0])
		}
		return r
	}
	if got := lista(); !slices.Equal(got, esperado[:200]) || !slices.Contains(teclasPie(a), "m Ver más") {
		t.Fatalf("la primera página trae 200, las más nuevas, y hay más: %d %v", len(got), teclasPie(a))
	}
	if v := a.Vista(); !strings.Contains(v, "Mostrando las 200 más recientes") {
		t.Fatalf("dice cuántas muestra:\n%s", v)
	}
	a.Pulsar("m")
	if got := lista(); !slices.Equal(got, esperado[:400]) {
		t.Fatalf("la segunda página sigue a la primera sin repetir: %d", len(got))
	}
	if !slices.Contains(teclasPie(a), "m Ver más") {
		t.Fatal("una página llena puede tener más")
	}
	a.Pulsar("m")
	if got := lista(); !slices.Equal(got, esperado) {
		t.Fatalf("las %d, una vez cada una: %d", total, len(got))
	}
	if slices.Contains(teclasPie(a), "m Ver más") {
		t.Fatal("una página corta es la última")
	}
	if v := a.Vista(); !strings.Contains(v, fmt.Sprintf("Mostrando las %d más recientes", total)) {
		t.Fatalf("cuenta las %d", total)
	}
	a.Pulsar("m")
	if len(lista()) != total {
		t.Fatalf("m sin más páginas no hace nada: %d", len(lista()))
	}
	// Una página que llega tarde, de un filtro viejo, no se suma a la lista de ahora.
	b, _ := abrir(t, alto(t, cli, id, 600))
	tarde := b.Retener(func() { b.Pulsar("m") }) // pide la segunda página; aún no vuelve
	b.Entregar(pagina.FiltroPunto{Nombre: "p:no-existe"})
	b.Soltar(tarde)
	if v := b.Vista(); strings.Contains(v, "petición-") || strings.Contains(v, "Cargando") {
		t.Fatalf("la página del filtro viejo no se suma:\n%s", v)
	}
}

// TestHistorialFiltroPunto: llega con IrA{historial, FiltroPunto}: una API por su nombre, o un filtro ya armado como «o:terminal»
// (lo filtra el servicio); el selector de origen ofrece los orígenes, los grupos y las APIs.
func TestHistorialFiltroPunto(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "desde la terminal")
	if _, _, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "mi-app", Grupo: "web", Agente: "simulado"}); err != nil {
		t.Fatal(err)
	}
	_, token, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "otra", Grupo: "web", Agente: "simulado"})
	if err != nil {
		t.Fatal(err)
	}
	_, tokenApp, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "tienda", Agente: "simulado"})
	if err != nil {
		t.Fatal(err)
	}
	porApi := func(punto, token, texto string) {
		t.Helper()
		req, _ := http.NewRequest("POST", cli.Base+"/p/"+punto+"/api/v1/trabajos", strings.NewReader(`{"mensajes":[{"rol":"usuario","contenido":"`+texto+`"}]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode >= 300 {
			t.Fatalf("petición de la app: %v %v", resp, err)
		}
		var tr nucleo.Trabajo
		if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		terminar(t, cli, id, tr.ID)
	}
	porApi("otra", token, "de la otra")
	porApi("tienda", tokenApp, "de la tienda")
	textos := []string{"desde la terminal", "de la otra", "de la tienda"}

	a, _ := abrir(t, alto(t, cli, id, 30))
	if n := cuantas(a, textos...); n != 3 {
		t.Fatalf("sin filtro salen las 3: %d", n)
	}
	// IrA{historial, FiltroPunto{«tienda»}}: llega después de Iniciar y filtra por la API.
	a.Correr(a.P.Iniciar(alto(t, cli, id, 30)))
	a.Entregar(pagina.FiltroPunto{Nombre: "tienda"})
	v := a.Vista()
	if cuantas(a, textos...) != 1 || !strings.Contains(v, "de la tienda") || !strings.Contains(v, "Origen: API tienda") {
		t.Fatalf("solo la API tienda, y la barra dice el origen:\n%s", v)
	}
	// Un filtro ya armado.
	a.Entregar(pagina.FiltroPunto{Nombre: "o:terminal"})
	if v := a.Vista(); cuantas(a, textos...) != 1 || !strings.Contains(v, "desde la terminal") || !strings.Contains(v, "Origen: Terminal") {
		t.Fatalf("solo el origen terminal:\n%s", v)
	}
	// El selector: los cuatro orígenes, el grupo y las APIs.
	a.Pulsar("f")
	if !a.P.EnCampo() {
		t.Fatal("con el selector abierto las letras son suyas")
	}
	v = a.Vista()
	for _, quiere := range []string{"Todos los orígenes", "Probar", "Terminal", "CLI", "App (clave del espacio)", "Grupo web", "API mi-app", "API otra", "API tienda"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("el selector ofrece %q:\n%s", quiere, v)
		}
	}
	a.Escribir("grupo w")
	a.Pulsar("enter") // elige «Grupo web»: las APIs otra y mi-app
	if a.P.EnCampo() {
		t.Fatal("Intro cierra el selector")
	}
	if v := a.Vista(); cuantas(a, textos...) != 1 || !strings.Contains(v, "de la otra") || !strings.Contains(v, "Origen: Grupo web") {
		t.Fatalf("el grupo web:\n%s", v)
	}
	// Esc cierra el selector sin cambiar nada; l quita los filtros.
	a.Pulsar("f", "esc")
	if a.P.EnCampo() || !strings.Contains(a.Vista(), "Origen: Grupo web") {
		t.Fatalf("Esc cierra el selector y conserva el filtro:\n%s", a.Vista())
	}
	a.Pulsar("l")
	if cuantas(a, textos...) != 3 || !strings.Contains(a.Vista(), "Origen: Todos los orígenes") {
		t.Fatalf("l quita los filtros:\n%s", a.Vista())
	}
	// El detalle de una petición de una API dice la API y su grupo.
	a.Entregar(pagina.FiltroPunto{Nombre: "otra"})
	if a.Pulsar("enter"); !strings.Contains(a.Vista(), "Origen: API otra · web") {
		t.Fatalf("el origen con el grupo:\n%s", a.Vista())
	}
}

// conEstados levanta el servicio con un agente que falla y otro que tarda, y deja una petición completada («uno»), una
// fallida («dos»), una en curso («tres») y una cancelada («cuatro»). Los filtros los aplica el servicio.
func conEstados(t *testing.T) *prueba.App {
	t.Helper()
	cli, id := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"roto": adaptadores.Simulado{Error: errors.New("falla")},
		"lento": adaptadores.Simulado{Demora: time.Minute}})
	crear(t, cli, id, "uno")
	dos, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "roto", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "dos"}}})
	if err != nil {
		t.Fatal(err)
	}
	terminar(t, cli, id, dos.ID)
	for _, texto := range []string{"tres", "cuatro"} {
		j, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "lento", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}})
		if err != nil {
			t.Fatal(err)
		}
		if texto == "cuatro" {
			if _, err := cli.Accion(ctx, id, j.ID, "cancelar"); err != nil {
				t.Fatal(err)
			}
			terminar(t, cli, id, j.ID)
		}
	}
	a, _ := abrir(t, alto(t, cli, id, 30))
	return a
}

// textos son las peticiones que enseña la lista, por su texto.
func textos(a *prueba.App) []string {
	var salida []string
	v := a.Vista()
	for _, x := range []string{"uno", "dos", "tres", "cuatro"} {
		if strings.Contains(v, " "+x) {
			salida = append(salida, x)
		}
	}
	return salida
}

func TestHistorialFiltroEstado(t *testing.T) {
	a := conEstados(t)
	casos := []struct {
		estado string
		quiere []string
	}{
		{nucleo.FiltroCompletado, []string{"uno"}},
		{nucleo.FiltroEnCurso, []string{"tres"}},
		{nucleo.FiltroConError, []string{"dos"}},
		{"todos", []string{"uno", "dos", "tres", "cuatro"}},
		{"nada", []string{"uno", "dos", "tres", "cuatro"}},
		{"", []string{"uno", "dos", "tres", "cuatro"}},
	}
	for _, c := range casos {
		a.Entregar(pagina.FiltroEstado{Estado: c.estado})
		if got := textos(a); !slices.Equal(got, c.quiere) { // los cancelados solo salen en «Todas»
			t.Fatalf("FiltroEstado %q: %v, quería %v\n%s", c.estado, got, c.quiere, a.Vista())
		}
	}
	// La tecla e recorre los estados como el selector de la Web UI: todas, en proceso, completadas y con error.
	for _, quiere := range []string{"En proceso", "Completadas", "Con error", "Todas"} {
		a.Pulsar("e")
		if v := a.Vista(); !strings.Contains(v, "Estado: "+quiere) {
			t.Fatalf("e pasa a %q:\n%s", quiere, v)
		}
	}
	// Con el estado puesto, un filtro que no deja nada dice que es del filtro, y l lo quita.
	a.Entregar(pagina.FiltroEstado{Estado: nucleo.FiltroCompletado})
	a.Pulsar("/", "d", "o", "s", "enter")
	if v := a.Vista(); !strings.Contains(v, "No hay peticiones con este filtro") {
		t.Fatalf("vacío por el filtro:\n%s", v)
	}
	a.Pulsar("l")
	if got := textos(a); len(got) != 4 {
		t.Fatalf("l deja todas: %v", got)
	}
}

func TestHistorialBusqueda(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "Factura de Soporte")
	crear(t, cli, id, "Notas de reunión")
	otra := crear(t, cli, id, "Otra cosa")
	a := abierta(t, cli, id)
	ves := func(quiere ...string) {
		t.Helper()
		v := a.Vista()
		for _, x := range []string{"Factura de Soporte", "Notas de reunión", "Otra cosa"} {
			if strings.Contains(v, x) != slices.Contains(quiere, x) {
				t.Fatalf("quería %v:\n%s", quiere, v)
			}
		}
	}
	a.Pulsar("/")
	if !a.P.EnCampo() {
		t.Fatal("con la búsqueda abierta las letras son suyas")
	}
	for _, c := range "REUNION" { // el servicio busca sin tildes ni mayúsculas
		a.Pulsar(string(c))
	}
	ves("Notas de reunión")
	if v := a.Vista(); !strings.Contains(v, "/ REUNION_") {
		t.Fatalf("la barra enseña lo que se escribe:\n%s", v)
	}
	a.Pulsar("backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace")
	ves("Factura de Soporte", "Notas de reunión", "Otra cosa")
	// También por identificador.
	for _, c := range otra.ID[len(otra.ID)-6:] {
		a.Pulsar(string(c))
	}
	ves("Otra cosa")
	// Intro la deja puesta y la barra la recuerda; Esc de una nueva vuelve a lo de antes.
	a.Pulsar("enter")
	if a.P.EnCampo() || !strings.Contains(a.Vista(), "/ "+otra.ID[len(otra.ID)-6:]) {
		t.Fatalf("Intro deja la búsqueda:\n%s", a.Vista())
	}
	a.Pulsar("/", "x", "esc")
	ves("Otra cosa")
	// Lo pegado entra entero, con los saltos de línea como espacios.
	a.Pulsar("/", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace")
	a.Pegar("Factura\nde Soporte")
	ves("Factura de Soporte")
	a.Pulsar("esc", "l")
	ves("Factura de Soporte", "Notas de reunión", "Otra cosa")
}

func TestHistorialDetalle(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "hola mundo")
	a := abierta(t, cli, id)
	a.Pulsar("enter")
	v := a.Vista()
	for _, quiere := range []string{"✓ Completado", "En cola ▸ Preparación ▸ Ejecución ▸ Validación ▸ [Llegada]", "[Mensaje]", "Respuesta", "Registro",
		"hola mundo", "Agente y modelo: ", "Origen: Terminal", "Tokens: 0 → 0", "Duración: ", "Identificador: trabajo_"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q en el detalle:\n%s", quiere, v)
		}
	}
	if enVivo(a) || prueba.FlujosAbiertos() != 0 {
		t.Fatal("un trabajo terminado no se sigue en vivo")
	}
	// La respuesta.
	a.Pulsar("right")
	if v := a.Vista(); !strings.Contains(v, "[Respuesta]") || !strings.Contains(v, "hola mundo") {
		t.Fatalf("pestaña Respuesta:\n%s", v)
	}
	// El registro: la hora de cada paso, con el resumen de lo que duró cada etapa.
	a.Pulsar("right")
	v = a.Vista()
	for _, quiere := range []string{"[Registro]", "En cola", "Ejecutando", "Completado", " +"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q en el registro:\n%s", quiere, v)
		}
	}
	if !strings.Contains(v, "simulado") {
		t.Fatalf("el intento dice con qué agente:\n%s", v)
	}
	a.Pulsar("right") // no pasa de la última
	if !enPestana(a, "registro.titulo") {
		t.Fatalf("la última pestaña es la del registro:\n%s", a.Vista())
	}
	a.Pulsar("left", "left", "left")
	if !enPestana(a, "historial.solicitud") {
		t.Fatalf("← vuelve al mensaje:\n%s", a.Vista())
	}
	// El pie del detalle.
	pie := teclasPie(a)
	for _, quiere := range []string{"Esc vuelve", "←→ pestañas", "/ busca", "t Reintentar", "x Eliminar"} {
		if !slices.Contains(pie, quiere) {
			t.Fatalf("el pie del detalle ofrece %q: %v", quiere, pie)
		}
	}
	// Esc cierra el detalle y, sin nada abierto, vuelve.
	if fuera := a.Pulsar("esc"); len(fuera) != 0 || enPestana(a, "historial.solicitud") {
		t.Fatalf("Esc cierra el detalle: %v\n%s", fuera, a.Vista())
	}
	if _, ok := a.Pulsar("esc")[0].(pagina.Volver); !ok {
		t.Fatal("Esc sin nada abierto vuelve")
	}
	// Buscar en el visor.
	a.Pulsar("enter", "/")
	if !a.P.EnCampo() {
		t.Fatal("la búsqueda del detalle tiene el foco")
	}
	a.Escribir("mundo")
	a.Pulsar("enter")
	if a.P.EnCampo() {
		t.Fatal("Intro aplica la búsqueda del detalle")
	}
}

func TestHistorialDetalleConError(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crearInvalida(t, cli, id)
	a := abierta(t, cli, id)
	if v := a.Vista(); !strings.Contains(v, "Falló") {
		t.Fatalf("la fila dice cómo acabó:\n%s", v)
	}
	a.Pulsar("enter")
	v := a.Vista()
	if !strings.Contains(v, "✗ La respuesta no cumple el JSON Schema: ") || !strings.Contains(v, "[Falló]") {
		t.Fatalf("el título del código y el mensaje del servicio, tal cual:\n%s", v)
	}
	a.Pulsar("right")
	if v := a.Vista(); !strings.Contains(v, "diez") || !strings.Contains(v, "La respuesta no cumple el JSON Schema") {
		t.Fatalf("la respuesta que no cumplía sale con el error:\n%s", v)
	}
	if !slices.Contains(teclasPie(a), "o Corregir") {
		t.Fatalf("Corregir solo en una que no cumplía el esquema: %v", teclasPie(a))
	}
}

// TestHistorialEnVivo: una petición en marcha se sigue con los eventos SSE del trabajo: la respuesta se va escribiendo, el
// estado cambia y, al terminar, el detalle queda con el trabajo final. El seguimiento se corta al terminar.
func TestHistorialEnVivo(t *testing.T) {
	cli, id, b, _ := conBloqueado(t)
	a, r := abrir(t, prueba.Contexto(t, cli, id))
	if v := a.Vista(); cuantas(a, "esperar") != 1 {
		t.Fatalf("la lista tiene la petición en marcha:\n%s", v)
	}
	a.Pulsar("enter")
	if v := a.Vista(); !enVivo(a) || !strings.Contains(v, "●") || prueba.FlujosAbiertos() != 1 {
		t.Fatalf("abre el detalle y sigue en vivo:\n%s", v)
	}
	a.Pulsar("right")
	if v := a.Vista(); !strings.Contains(v, "Todavía en curso…") {
		t.Fatalf("aún no hay respuesta:\n%s", v)
	}
	// El primer tramo llega mientras sigue en marcha.
	b.SoltarPaso()
	a.HastaVer("primera parte")
	if v := a.Vista(); strings.Contains(v, "segunda") || !enVivo(a) {
		t.Fatalf("se va escribiendo:\n%s", v)
	}
	// El resto, y el final: el detalle pasa a ser el del trabajo terminado y el seguimiento se cierra.
	b.SoltarFin()
	a.HastaVer("primera parte segunda parte", "✓ Completado")
	cerrado(t, a)
	if enVivo(a) {
		t.Fatalf("termina con la respuesta entera, ya sin seguir:\n%s", a.Vista())
	}
	// El registro ya lo tiene completo.
	a.Pulsar("right")
	if v := a.Vista(); !strings.Contains(v, "Ejecutando") || !strings.Contains(v, "Validando") || !strings.Contains(v, "Completado") {
		t.Fatalf("el registro del trabajo terminado:\n%s", v)
	}
	// Los cambios de estado vuelven a leer el registro un momento después; si para entonces el detalle se cerró, nada.
	a.Pulsar("esc")
	r.Pasar(time.Second)
	if a.Asentar(); len(a.Fuera) != 0 || enPestana(a, "historial.solicitud") {
		t.Fatalf("con el detalle cerrado, nada:\n%s", a.Vista())
	}
}

// TestHistorialEnVivoSeCorta: el seguimiento se corta al cerrar el detalle (Esc), al ocultar la página (Ocultar), al cancelarse
// el contexto de la página y al iniciarla otra vez; Mostrar lo reabre. Lo que llega del seguimiento con la página ya
// oculta no se aplica.
func TestHistorialEnVivoSeCorta(t *testing.T) {
	cli, id, b, _ := conBloqueado(t)
	a := abierta(t, cli, id)
	a.Pulsar("enter")
	if prueba.FlujosAbiertos() != 1 {
		t.Fatal("en vivo")
	}
	a.Pulsar("esc")
	cerrado(t, a)
	if enVivo(a) {
		t.Fatal("Esc corta el seguimiento")
	}
	// El primer tramo sale cuando la página ya va a ocultarse: llega a destiempo y no se aplica.
	a.Pulsar("enter", "right")
	b.SoltarPaso()
	a.EsperarLlegada()
	a.P.(pagina.Ocultable).Ocultar()
	a.Asentar()
	cerrado(t, a)
	if strings.Contains(a.Vista(), "primera parte") {
		t.Fatalf("oculta, lo que llega tarde no se aplica:\n%s", a.Vista())
	}
	// Mostrar reabre el seguimiento del mismo detalle, con su pestaña.
	a.Correr(a.P.(pagina.Ocultable).Mostrar())
	a.HastaVer("primera parte")
	if !enVivo(a) || prueba.FlujosAbiertos() != 1 || !enPestana(a, "ejecutar.respuesta") {
		t.Fatalf("Mostrar reabre el seguimiento y conserva la pestaña:\n%s", a.Vista())
	}
	// Otro Iniciar cancela el seguimiento de antes y empieza de cero, sin detalle.
	a.Correr(a.P.Iniciar(prueba.Contexto(t, cli, id)))
	cerrado(t, a)
	if strings.Contains(a.Vista(), "["+datos.Texto("es", "historial.solicitud")+"]") || strings.Contains(a.Vista(), "["+datos.Texto("es", "ejecutar.respuesta")+"]") {
		t.Fatalf("Iniciar empieza de cero, sin detalle:\n%s", a.Vista())
	}
	// El contexto de la página (cambio de espacio, reinicio) también lo corta.
	cli2, id2, _, _ := conBloqueado(t)
	c := prueba.Contexto(t, cli2, id2)
	vivo, cancelar := context.WithCancel(c.Ctx)
	c.Ctx = vivo
	a2, _ := abrir(t, c)
	a2.Pulsar("enter")
	if prueba.FlujosAbiertos() != 1 {
		t.Fatal("en vivo")
	}
	cancelar()
	cerrado(t, a2)
}

// TestHistorialEnVivoReconecta: si el flujo se corta sin que el trabajo termine, avisa, espera y se reabre desde el último
// evento visto.
func TestHistorialEnVivoReconecta(t *testing.T) {
	cli, id, b, _ := conBloqueado(t)
	p, r := nueva("es")
	var mu sync.Mutex
	var desdes []int
	historial.ConEventos(p, func(ctx context.Context, inst, trabajoID string, desde int, al func(int, string, string) error) error {
		mu.Lock()
		desdes = append(desdes, desde)
		n := len(desdes)
		mu.Unlock()
		if n == 1 { // el primero manda un evento y se corta
			_ = al(7, "texto", "uno ")
			return errors.New("se cortó")
		}
		return cli.Eventos(ctx, inst, trabajoID, desde, al)
	})
	a := prueba.Abrir(t, p, prueba.Contexto(t, cli, id))
	a.Pulsar("enter", "right")
	a.HastaVer("uno ", "reconectando…")
	r.Pasar(2 * time.Second) // la espera tras el corte
	a.Hasta(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(desdes) >= 2 && !strings.Contains(a.Vista(), "reconectando…")
	})
	mu.Lock()
	if desdes[0] != 0 || desdes[1] != 7 {
		t.Fatalf("reabre desde el último evento visto: %v", desdes)
	}
	mu.Unlock()
	b.SoltarPaso()
	b.SoltarFin()
	a.HastaVer("✓ Completado", "primera parte segunda parte")
}

func TestHistorialReintentar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	original := crear(t, cli, id, "otra vez")
	a := abierta(t, cli, id)
	fuera := a.Pulsar("t")
	if av, ok := aviso(fuera); !ok || av.Texto != "Petición enviada de nuevo" || av.Tipo != comp.Exito {
		t.Fatalf("avisa: %+v", fuera)
	}
	lista, err := cli.Trabajos(ctx, id, 10)
	if err != nil || len(lista) != 2 {
		t.Fatalf("el servicio tiene otro trabajo: %d %v", len(lista), err)
	}
	nuevo := seguido(fuera)
	if nuevo == "" || nuevo == original.ID || lista[0].ID != nuevo {
		t.Fatalf("lleva a Probar con el trabajo nuevo, como la Web UI: %v", fuera)
	}
	if n := cuantas(a, "otra vez"); n != 2 {
		t.Fatalf("y la lista lo trae: %d", n)
	}
	// Una en marcha no se reintenta: se cancela.
	cli2, id2, _, run := conBloqueado(t)
	a2 := abierta(t, cli2, id2)
	if fuera := a2.Pulsar("t"); len(fuera) != 0 {
		t.Fatalf("t no vale en una que sigue en marcha: %v", fuera)
	}
	if lista, _ := cli2.Trabajos(ctx, id2, 10); len(lista) != 1 || lista[0].ID != run.ID {
		t.Fatalf("no creó nada: %v", lista)
	}
}

func TestHistorialCancelar(t *testing.T) {
	cli, id, _, tr := conBloqueado(t)
	a := abierta(t, cli, id)
	pie := teclasPie(a)
	if !slices.Contains(pie, "c Cancelar") || slices.Contains(pie, "t Reintentar") || slices.Contains(pie, "x Eliminar") {
		t.Fatalf("en marcha se ofrece cancelar: %v", pie)
	}
	if fuera := a.Pulsar("x"); len(fuera) != 0 {
		t.Fatalf("x no borra una en marcha: %v", fuera)
	}
	a.Pulsar("enter", "c")
	if v := a.Vista(); !strings.Contains(v, "Cancelación solicitada") && !strings.Contains(v, "Cancelado") {
		t.Fatalf("el detalle toma el estado que devuelve el servicio:\n%s", v)
	}
	a.HastaVer("[" + datos.Texto("es", "estado.cancelado") + "]") // la llegada de la ruta
	if t2, err := cli.Trabajo(ctx, id, tr.ID); err != nil || t2.Estado != nucleo.Cancelado {
		t.Fatalf("el servicio la cancela: %+v %v", t2, err)
	}
}

func TestHistorialEliminar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "se queda")
	borrar := crear(t, cli, id, "se borra")
	a := abierta(t, cli, id)
	if q, ok := pregunta(a.Pulsar("x")); !ok || !strings.Contains(q.Texto, "Eliminar petición") || !strings.Contains(q.Texto, "No se puede deshacer") {
		t.Fatalf("x pregunta: %+v", q)
	}
	// Cancelar no toca nada, y después confirmar tampoco: ya no hay pregunta pendiente.
	a.Entregar(comp.Cancelado{})
	a.Entregar(comp.Confirmado{})
	if lista, _ := cli.Trabajos(ctx, id, 10); len(lista) != 2 {
		t.Fatalf("seguía todo: %d", len(lista))
	}
	// Confirmar borra la de la fila elegida (la más nueva).
	a.Pulsar("x")
	a.Entregar(comp.Confirmado{})
	lista, _ := cli.Trabajos(ctx, id, 10)
	if len(lista) != 1 || lista[0].ID == borrar.ID {
		t.Fatalf("borró la elegida: %v", lista)
	}
	if v := a.Vista(); cuantas(a, "se queda", "se borra") != 1 || !strings.Contains(v, "se queda") {
		t.Fatalf("la lista se pone al día:\n%s", v)
	}
	// Una que ya no está en el servicio: el error de la API al reintentarla sale como aviso.
	ida := crear(t, cli, id, "ya borrada")
	a.Pulsar("r")
	if err := cli.Eliminar(ctx, id, ida.ID); err != nil {
		t.Fatal(err)
	}
	a.Pulsar("up", "up")                                              // la fila de la más nueva
	if av, ok := aviso(a.Pulsar("t")); !ok || av.Tipo != comp.Fallo { // reintentarla
		t.Fatalf("avisa del error del servicio: %+v", av)
	}
	a.Pulsar("r")
	// Desde el detalle: borra la abierta y lo cierra.
	a.Pulsar("enter", "x")
	a.Entregar(comp.Confirmado{})
	if l, _ := cli.Trabajos(ctx, id, 10); enPestana(a, "historial.solicitud") || cuantas(a, "se queda") != 0 || len(l) != 0 {
		t.Fatalf("borra la abierta, cierra el detalle y la lista queda vacía:\n%s", a.Vista())
	}
}

func TestHistorialCorregir(t *testing.T) {
	cli, id := prueba.Servicio(t)
	mala := crearInvalida(t, cli, id)
	a := abierta(t, cli, id)
	a.Pulsar("enter")
	fuera := a.Pulsar("o")
	if av, ok := aviso(fuera); !ok || av.Texto != "Corrección enviada" || av.Tipo != comp.Exito {
		t.Fatalf("avisa: %+v", fuera)
	}
	lista, _ := cli.Trabajos(ctx, id, 10)
	id3 := seguido(fuera)
	if len(lista) != 2 || id3 == "" || id3 == mala.ID {
		t.Fatalf("es otra petición y la sigue en Probar: %d %v", len(lista), fuera)
	}
	// Corregir no es reintentar: el trabajo nuevo lleva la conversación con el mensaje de corrección del planificador.
	nuevo, err := cli.Trabajo(ctx, id, id3)
	if err != nil || len(nuevo.Solicitud.Mensajes) < 2 ||
		!strings.HasPrefix(nuevo.Solicitud.Mensajes[len(nuevo.Solicitud.Mensajes)-1].Contenido, "Tu respuesta no cumple el formato pedido:") {
		t.Fatalf("la corrección termina con el mensaje de corrección: %+v %v", nuevo.Solicitud.Mensajes, err)
	}
	if len(nuevo.Solicitud.Mensajes) <= len(mala.Solicitud.Mensajes) {
		t.Fatalf("y añade mensajes a la solicitud: %d contra %d", len(nuevo.Solicitud.Mensajes), len(mala.Solicitud.Mensajes))
	}
	// Una completada no se corrige: avisa con el texto de la Web UI y no manda nada.
	cli2, id2 := prueba.Servicio(t)
	crear(t, cli2, id2, "bien")
	a2 := abierta(t, cli2, id2)
	fuera = a2.Pulsar("enter", "o")
	if av, ok := aviso(fuera); !ok || av.Texto != "Esta petición no se puede corregir" || av.Tipo != comp.Advertencia {
		t.Fatalf("avisa: %+v", fuera)
	}
	if lista, _ := cli2.Trabajos(ctx, id2, 10); len(lista) != 1 {
		t.Fatalf("no mandó nada: %d", len(lista))
	}
}

func TestHistorialVaciar(t *testing.T) {
	cli, id, b, _ := conBloqueado(t)
	crear(t, cli, id, "uno")
	crear(t, cli, id, "dos")
	a := abierta(t, cli, id)
	todas := []string{"esperar", " uno", " dos"}
	if n := cuantas(a, todas...); n != 3 {
		t.Fatalf("tres: %d", n)
	}
	q, ok := pregunta(a.Pulsar("V"))
	if !ok || !strings.Contains(q.Texto, "Vaciar el historial") || !strings.Contains(q.Texto, "Las que están en proceso se conservan") {
		t.Fatalf("pregunta antes de borrar: %+v", q)
	}
	// Cancelar no borra nada.
	a.Entregar(comp.Cancelado{})
	a.Pulsar("r")
	if n := cuantas(a, todas...); n != 3 {
		t.Fatalf("cancelar no borra: %d", n)
	}
	// Confirmar borra las terminadas y conserva la que sigue en marcha.
	a.Pulsar("V")
	fuera := a.Entregar(comp.Confirmado{})
	if av, ok := aviso(fuera); !ok || av.Texto != "Peticiones eliminadas: 2" || av.Tipo != comp.Exito {
		t.Fatalf("avisa de cuántas: %+v", fuera)
	}
	if v := a.Vista(); cuantas(a, todas...) != 1 || !strings.Contains(v, "esperar") {
		t.Fatalf("solo queda la que sigue en marcha:\n%s", v)
	}
	if lista, _ := cli.Trabajos(ctx, id, 10); len(lista) != 1 {
		t.Fatalf("en el servicio también: %d", len(lista))
	}
	// Sin nada terminado no hay qué vaciar.
	fuera = a.Pulsar("V")
	if av, ok := aviso(fuera); !ok || av.Tipo != comp.Advertencia || av.Texto != "No hay peticiones terminadas" {
		t.Fatalf("avisa: %+v", fuera)
	}
	if _, ok := pregunta(fuera); ok {
		t.Fatal("no pregunta por nada")
	}
	b.SoltarPaso()
	b.SoltarFin()
}

func TestEscVuelve(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "uno")
	a := abierta(t, cli, id)
	if _, ok := a.Pulsar("esc")[0].(pagina.Volver); !ok {
		t.Fatal("Esc fuera de campo vuelve")
	}
	// Esc cierra la búsqueda y el selector antes de volver.
	a.Pulsar("/")
	if fuera := a.Pulsar("esc"); len(fuera) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc cierra la búsqueda: %v", fuera)
	}
	a.Pulsar("f")
	if fuera := a.Pulsar("esc"); len(fuera) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc cancela el selector y lo cierra: %v", fuera)
	}
	if _, ok := a.Pulsar("esc")[0].(pagina.Volver); !ok {
		t.Fatal("y entonces Esc vuelve")
	}
}

func TestServicioCaido(t *testing.T) {
	// Sin cliente, la página avisa y no se cierra.
	a, _ := abrir(t, prueba.Contexto(t, nil, "global"))
	if len(a.Fuera) != 1 {
		t.Fatalf("sin cliente: %v", a.Fuera)
	} else if e, ok := a.Fuera[0].(pagina.ErrorServicio); !ok || !errors.Is(e.Err, cliente.ErrServicioInactivo) {
		t.Fatalf("sin cliente: %v", a.Fuera[0])
	}
	if v := a.Vista(); !strings.Contains(v, "El servicio no responde") {
		t.Fatalf("avisa:\n%s", v)
	}
	// Con el servicio parado a mitad: lo cargado se conserva, el aviso sale y r reintenta.
	cli, id, cerrar := prueba.Arrancar(t)
	crear(t, cli, id, "antes de la caída")
	a = abierta(t, cli, id)
	cerrar()
	fuera := a.Pulsar("r")
	if len(fuera) == 0 {
		t.Fatal("r sin servicio avisa a la app")
	}
	if e, ok := fuera[0].(pagina.ErrorServicio); !ok || !pagina.NoResponde(e.Err) {
		t.Fatalf("ErrorServicio: %+v", fuera[0])
	}
	if v := a.Vista(); !strings.Contains(v, "El servicio no responde") {
		t.Fatalf("la página lo dice y sigue:\n%s", v)
	}
	// Las demás teclas siguen respondiendo: cambiar el filtro lo pide al servicio y avisa de que no responde, y Esc vuelve.
	fuera = a.Pulsar("e", "l", "esc")
	if len(fuera) != 3 || fuera[2] != (pagina.Volver{}) || !errorServicio(fuera[:1]) || !errorServicio(fuera[1:2]) {
		t.Fatalf("la página sigue viva y el filtro sin servicio avisa: %v", fuera)
	}
	cli3, id3, cerrar3 := prueba.Arrancar(t)
	crear(t, cli3, id3, "hay uno")
	b := abierta(t, cli3, id3)
	cerrar3()
	if fuera := b.Pulsar("enter"); !errorServicio(fuera) || enPestana(b, "historial.solicitud") {
		t.Fatalf("abrir el detalle sin servicio avisa y no deja un detalle a medias: %v\n%s", fuera, b.Vista())
	}
}

// TestRespuestasViejas: una lista o un detalle que llegan de otro espacio, de un contexto cancelado, de una consulta superada
// o de otra versión de la página no se aplican. Son respuestas de verdad que la página pidió y llegan tarde.
func TestRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "uno")
	// pedir deja en camino una lista (r) y un detalle (Intro, que luego se cierra); después llega otra petición.
	pedir := func(a *prueba.App) []tea.Cmd {
		viejos := a.Retener(func() { a.Pulsar("r") })
		viejos = append(viejos, a.Retener(func() { a.Pulsar("enter") })...)
		a.Pulsar("esc")
		return viejos
	}
	sinAplicar := func(t *testing.T, a *prueba.App, antes string, fuera []tea.Msg) {
		t.Helper()
		if v := a.Vista(); v != antes || len(fuera) != 0 {
			t.Fatalf("una respuesta vieja se aplicó: %v\n%s\nantes:\n%s", fuera, v, antes)
		}
	}
	t.Run("otra versión", func(t *testing.T) {
		a := abierta(t, cli, id)
		viejos := pedir(a)
		crear(t, cli, id, "nueva")
		a.P.(pagina.Ocultable).Ocultar()
		antes := a.Vista()
		sinAplicar(t, a, antes, a.Soltar(viejos))
	})
	t.Run("otro espacio", func(t *testing.T) {
		a := abierta(t, cli, id)
		viejos := pedir(a)
		a.Correr(a.P.Iniciar(prueba.Contexto(t, cli, "otro")))
		antes := a.Vista()
		sinAplicar(t, a, antes, a.Soltar(viejos))
	})
	t.Run("contexto cancelado", func(t *testing.T) {
		c := prueba.Contexto(t, cli, id)
		vivo, cancelar := context.WithCancel(c.Ctx)
		c.Ctx = vivo
		a, _ := abrir(t, c)
		viejos := pedir(a)
		cancelar()
		antes := a.Vista()
		sinAplicar(t, a, antes, a.Soltar(viejos))
	})
	t.Run("consulta superada", func(t *testing.T) {
		a := abierta(t, cli, id)
		vieja := a.Retener(func() { a.Pulsar("r") }) // la lista de antes del filtro
		a.Entregar(pagina.FiltroPunto{Nombre: "p:no-existe"})
		antes := a.Vista()
		sinAplicar(t, a, antes, a.Soltar(vieja))
	})
}

// TestListaConError: el error de la API se ve tal cual en la página, sin perder lo cargado; el de conexión es del servicio, y
// en segundo plano (el sondeo) no se enseña ni se avisa.
func TestListaConError(t *testing.T) {
	cli, id, cerrar := prueba.Arrancar(t)
	crear(t, cli, id, "uno")
	a, r := abrir(t, prueba.Contexto(t, cli, id))
	a.Pulsar("/")
	a.Pegar(strings.Repeat("x", 201)) // el servicio admite hasta 200 caracteres
	if v := a.Vista(); !strings.Contains(v, "200") || !strings.Contains(v, "uno") {
		t.Fatalf("el mensaje del servicio sin traducir, y lo cargado se conserva:\n%s", v)
	}
	a.Pulsar("esc")
	cerrar()
	r.Pasar(5 * time.Second) // el sondeo
	a.Asentar()
	if v := a.Vista(); len(a.Fuera) != 0 || strings.Contains(v, "El servicio no responde") {
		t.Fatalf("el sondeo no avisa de un error: %v\n%s", a.Fuera, v)
	}
	if fuera := a.Pulsar("r"); !errorServicio(fuera) {
		t.Fatalf("de conexión, al pedirla: %v", fuera)
	}
}

// TestHistorialSondeo: cada 5 s la lista se vuelve a leer, salvo con el detalle abierto o con páginas extra cargadas.
func TestHistorialSondeo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "primera")
	a, r := abrir(t, prueba.Contexto(t, cli, id))
	crear(t, cli, id, "segunda")
	sondear := func() {
		t.Helper()
		r.Pasar(5 * time.Second)
		a.Asentar()
	}
	if sondear(); cuantas(a, "primera", "segunda") != 2 {
		t.Fatalf("el sondeo trae lo nuevo:\n%s", a.Vista())
	}
	// Con el detalle abierto no recarga.
	a.Pulsar("enter")
	crear(t, cli, id, "tercera")
	sondear()
	a.Pulsar("esc")
	if cuantas(a, "tercera") != 0 {
		t.Fatalf("con el detalle abierto el sondeo no recarga:\n%s", a.Vista())
	}
	if sondear(); cuantas(a, "primera", "segunda", "tercera") != 3 {
		t.Fatalf("al cerrarlo vuelve a leer:\n%s", a.Vista())
	}
}

// TestHistorialSondeoConMasPaginas: con páginas extra cargadas (Ver más) el sondeo no recarga: la recarga solo trae la primera.
func TestHistorialSondeoConMasPaginas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crearVarias(t, cli, id, 201)
	a, r := abrir(t, alto(t, cli, id, 300))
	a.Pulsar("m")
	crear(t, cli, id, "petición-nueva")
	r.Pasar(5 * time.Second)
	a.Asentar()
	if cuantas(a, "petición-nueva") != 0 || cuantas(a, "petición-000") != 1 {
		t.Fatalf("con más de una página el sondeo no recarga:\n%s", a.Vista())
	}
}

func TestHistorialOcultarYMostrar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "uno")
	a := abierta(t, cli, id)
	a.Pulsar("e", "/", "u", "enter")
	a.P.(pagina.Ocultable).Ocultar()
	crear(t, cli, id, "dos")
	a.Correr(a.P.(pagina.Ocultable).Mostrar())
	if v := a.Vista(); cuantas(a, " uno", " dos") != 0 || !strings.Contains(v, "/ u") || strings.Contains(v, "Estado: Todas") { // el filtro de estado y el texto se conservan
		t.Fatalf("Mostrar conserva los filtros:\n%s", v)
	}
	a.Pulsar("e", "e", "e") // todas
	if cuantas(a, " uno", " dos") != 1 || !strings.Contains(a.Vista(), "uno") {
		t.Fatalf("Mostrar la pone al día (y solo «uno» contiene la u):\n%s", a.Vista())
	}
	// Iniciar sí empieza de cero.
	a.Correr(a.P.Iniciar(prueba.Contexto(t, cli, id)))
	if v := a.Vista(); !strings.Contains(v, "Estado: Todas") || strings.Contains(v, "/ u") || cuantas(a, " uno", " dos") != 2 {
		t.Fatalf("Iniciar quita los filtros:\n%s", v)
	}
}

// TestCabeEn60x20: ninguna vista se sale del área, con tildes y con textos largos; la confirmación cabe en 60×20.
func TestCabeEn60x20(t *testing.T) {
	cli, id := prueba.Servicio(t)
	largo := "una pregunta bastante larga con tildes áéíóú y ñandú que no cabe en una sola línea de sesenta columnas, " + strings.Repeat("más texto ", 30)
	crear(t, cli, id, largo)
	crear(t, cli, id, "corta")
	crearInvalida(t, cli, id)
	for _, h := range []int{17, 20} {
		ancho, alto := 60, h
		c := prueba.Contexto(t, cli, id)
		c.Alto = alto
		a, _ := abrir(t, c)
		comprobar := func(donde string) {
			t.Helper()
			v := a.Vista()
			if h := lipgloss.Height(v); h > alto {
				t.Fatalf("%s a 60×%d mide %d filas:\n%s", donde, alto, h, v)
			}
			for _, l := range strings.Split(v, "\n") {
				if w := lipgloss.Width(l); w > ancho {
					t.Fatalf("%s a %d×%d: la línea mide %d columnas: %q", donde, ancho, alto, w, l)
				}
			}
		}
		comprobar("lista")
		a.Pulsar("down", "down", "enter")
		for range 3 {
			comprobar("detalle")
			a.Pulsar("right")
		}
		a.Pulsar("esc", "f")
		comprobar("selector")
		a.Pulsar("esc")
		a.Pulsar("/", "x", "y", "z")
		comprobar("búsqueda")
		a.Pulsar("esc", "e")
		comprobar("filtro")
		ancho = 78
		a.Entregar(tea.WindowSizeMsg{Width: ancho, Height: 30})
		comprobar("más grande")
		ancho = 60
		a.Entregar(tea.WindowSizeMsg{Width: ancho, Height: alto})
		comprobar("de vuelta")

		// Las confirmaciones, que pinta la app con comp.Confirmacion.
		for _, tecla := range []string{"V", "x"} {
			a.Pulsar("l") // vuelve a pedir la lista entera
			q, ok := pregunta(a.Pulsar(tecla))
			if !ok {
				t.Fatalf("%s pregunta", tecla)
			}
			a.Entregar(comp.Cancelado{})
			caja := (&comp.Confirmacion{Texto: q.Texto}).Vista(c.Estilos, 60, 20)
			if h := lipgloss.Height(caja); h > 20 {
				t.Fatalf("la confirmación de %s mide %d filas", tecla, h)
			}
			for _, l := range strings.Split(caja, "\n") {
				if w := lipgloss.Width(l); w > 60 {
					t.Fatalf("la confirmación de %s: línea de %d columnas", tecla, w)
				}
			}
		}
	}
}

// TestTextosComoLaWeb: los nombres salen de los textos de la Web UI, en el idioma de la sesión.
func TestTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "hello")
	c := alto(t, cli, id, 30)
	c.Idioma = "en"
	a, _ := abrir(t, c)
	v := strings.Join(strings.Fields(a.Vista()), " ") // un texto largo ocupa varias líneas
	for _, quiere := range []string{datos.Texto("en", "historial.estado") + ": " + datos.Texto("en", "historial.todos"),
		datos.Texto("en", "historial.origen") + ": " + datos.Texto("en", "historial.todosOrigenes"), datos.Textof("en", "historial.mostrando", "n", "1"),
		datos.Texto("en", "estado.completado")} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	a.Pulsar("enter")
	v = a.Vista()
	for _, quiere := range []string{"[" + datos.Texto("en", "historial.solicitud") + "]", datos.Texto("en", "ejecutar.respuesta"), datos.Texto("en", "registro.titulo"),
		datos.Texto("en", "ruta.cola"), datos.Texto("en", "historial.agenteModelo") + ": ", datos.Texto("en", "historial.identificador") + ": "} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q en el detalle:\n%s", quiere, v)
		}
	}
	// En español, los de la Web UI en español.
	e := abierta(t, cli, id)
	e.Pulsar("enter")
	if v := e.Vista(); !strings.Contains(v, "[Mensaje]") || !strings.Contains(v, "Respuesta") || !strings.Contains(v, "Registro") {
		t.Fatalf("pestañas en español:\n%s", v)
	}
	// Y los pies, de los textos de la Web UI.
	e.Pulsar("esc")
	pie := teclasPie(e)
	for _, quiere := range []string{"e Estado", "f Origen", "t Reintentar", "x Eliminar", "V Vaciar historial"} {
		if !slices.Contains(pie, quiere) {
			t.Fatalf("el pie ofrece %q: %v", quiere, pie)
		}
	}
}

func TestSoloAscii(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, "hola")
	crearInvalida(t, cli, id)
	c := prueba.Contexto(t, cli, id)
	c.Estilos = &pagina.Estilos{ASCII: true, Iconos: pagina.IconosASCII}
	a, _ := abrir(t, c)
	sinDibujo := func(donde string) {
		t.Helper()
		v := a.Vista()
		for _, r := range v {
			if r > 127 && !strings.ContainsRune("áéíóúüñÁÉÍÓÚÜÑ¿¡×", r) {
				t.Fatalf("%s: sale %q en ASCII:\n%s", donde, r, v)
			}
		}
	}
	sinDibujo("lista")
	a.Pulsar("enter")
	for range 3 {
		sinDibujo("detalle")
		a.Pulsar("right")
	}
	for _, k := range a.P.Teclas() {
		if strings.ContainsAny(k.Tecla, "←→") {
			t.Fatalf("la tecla %q no es ASCII", k.Tecla)
		}
	}
}

// TestDetalleParteLasPalabras: el texto del detalle se parte por palabras al ancho de la página, y sigue al redimensionar.
func TestDetalleParteLasPalabras(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, strings.Repeat("palabra ", 40))
	a := abierta(t, cli, id)
	a.Pulsar("enter")
	for _, ancho := range []int{60, 40, 60} {
		a.Entregar(tea.WindowSizeMsg{Width: ancho, Height: 17})
		lineas := strings.Split(a.Vista(), "\n")
		if len(lineas) > 17 {
			t.Fatalf("a %d mide %d filas", ancho, len(lineas))
		}
		for i, l := range lineas {
			if lipgloss.Width(l) > ancho {
				t.Fatalf("a %d la línea %d mide %d: %q", ancho, i, lipgloss.Width(l), l)
			}
			if i >= 3 {
				for _, campo := range strings.Fields(l) {
					if strings.Contains(l, "palabra") && campo != "palabra" && !strings.HasSuffix(campo, ":") && !strings.HasPrefix(campo, "Agente") {
						t.Fatalf("a %d se partió una palabra: %q", ancho, l)
					}
				}
			}
		}
	}
}

// TestRespuestasViejasDelDetalle: lo que llega del detalle, del registro o del seguimiento en vivo de un detalle ya cerrado
// no se aplica, y un mensaje de un seguimiento ya cortado no vuelve a esperar sobre el canal nuevo (quedarían dos lectores y
// el texto en vivo podría llegar desordenado); tampoco el flujo viejo toma el canal. Una acción de otra versión de la
// página o de otro espacio no avisa ni lleva a Probar.
func TestRespuestasViejasDelDetalle(t *testing.T) {
	cli, id, b, _ := conBloqueado(t)
	a := abierta(t, cli, id)
	// El detalle se abre a mano, paso a paso, con lo que la página pide: el trabajo, el registro y el seguimiento.
	detalle := a.Recoger(a.Retener(func() { a.Pulsar("enter") }))
	registroYFlujo := a.Recoger(a.Retener(func() { a.Entregar(detalle[0]) }))
	lector := a.Retener(func() {
		for _, m := range registroYFlujo {
			a.Entregar(m)
		}
	})
	if len(lector) != 1 {
		t.Fatalf("el flujo deja un lector sobre su canal: %d Cmd", len(lector))
	}
	vivo := a.Recoger(lector) // el primer mensaje del seguimiento, que no se entrega
	// Se cierra el detalle y se abre otra vez: lo de antes ya no vale.
	a.Pulsar("esc")
	a.Pulsar("enter", "right")
	b.SoltarPaso()
	a.HastaVer("primera parte")
	antes := a.Vista()
	viejos := append(append(append([]tea.Msg{}, detalle...), registroYFlujo...), vivo...)
	if otros := a.Retener(func() {
		for _, m := range viejos {
			a.Entregar(m)
		}
	}); len(otros) != 0 {
		t.Fatalf("lo viejo no vuelve a leer ni toma el canal: %d Cmd", len(otros))
	}
	if a.Vista() != antes || len(a.Fuera) != 0 {
		t.Fatalf("ni cambia la vista:\n%s\n%s", antes, a.Vista())
	}
	b.SoltarFin()
	a.HastaVer("✓ Completado")

	// Reintentar (una acción) desde una versión o un espacio que ya no son los de ahora.
	for nombre, cambiar := range map[string]func(*prueba.App){
		"otra versión": func(a *prueba.App) {
			a.P.(pagina.Ocultable).Ocultar()
			a.Correr(a.P.(pagina.Ocultable).Mostrar())
		},
		"otro espacio": func(a *prueba.App) { a.Correr(a.P.Iniciar(prueba.Contexto(t, cli, "otro"))) },
	} {
		a := abierta(t, cli, id)
		reintento := a.Retener(func() { a.Pulsar("t") })
		cambiar(a)
		desde := len(a.Fuera)
		if fuera := a.Soltar(reintento); len(fuera) != 0 || len(a.Fuera) != desde {
			t.Fatalf("una acción de %s no avisa ni lleva a Probar: %v", nombre, fuera)
		}
	}
}
