package espacios_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/espacios"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de Espacios van por su interfaz, con el arnés de las páginas (prueba.App): teclas, los mensajes de pagina y
// comp, y lo que enseña Vista(). Lo que trae el servicio tras una tecla se espera con HastaVer o Hasta antes de mirarlo, y
// lo que la página cambia en el servicio se lee en el servicio.

var (
	aviso    = prueba.Aviso
	contexto = prueba.Contexto
)

func tx(clave string, pares ...string) string { return datos.Textof("es", clave, pares...) }

// abierta inicia la página contra ese servicio: arranca con «Cargando…» y espera la lista.
func abierta(t *testing.T, cli *cliente.Cliente, id string) *prueba.App {
	t.Helper()
	a := prueba.Conducir(t, espacios.Nueva("es"))
	cmd := a.P.Iniciar(contexto(t, cli, id))
	if v := a.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	a.Correr(cmd)
	a.EsperarCarga()
	return a
}

// clonado es una carpeta con la marca de Tramtai que no creó este equipo (un repositorio clonado): su local.yaml trae una
// clave de seguridad, así que abrirla pide confianza.
func clonado(t *testing.T, nombre string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), strings.ToLower(nombre))
	os.MkdirAll(filepath.Join(dir, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(dir, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: "+nombre+"\n"), 0o600)
	os.WriteFile(filepath.Join(dir, ".tramtai", "local.yaml"), []byte("comandos_agentes:\n  claude: otro.exe\n"), 0o600)
	return dir
}

// proyecto abre en el servicio una carpeta nueva como proyecto (confiado, porque lo crea Tramtai) y devuelve su espacio.
func proyecto(t *testing.T, cli *cliente.Cliente, nombre string) (contrato.Espacio, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), strings.ToLower(nombre))
	os.MkdirAll(dir, 0o700)
	e, err := cli.AbrirProyecto(context.Background(), dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return e, dir
}

// espacio es el espacio de ese nombre según el servicio.
func espacio(t *testing.T, cli *cliente.Cliente, nombre string) (contrato.Espacio, bool) {
	t.Helper()
	l, err := cli.Espacios(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(l, func(e contrato.Espacio) bool { return e.Nombre == nombre })
	if i < 0 {
		return contrato.Espacio{}, false
	}
	return l[i], true
}

// hastaServicio espera, entregando a la página lo que llegue, a que el espacio cumpla cond en el servicio.
func hastaServicio(t *testing.T, a *prueba.App, cli *cliente.Cliente, nombre string, cond func(contrato.Espacio, bool) bool) {
	t.Helper()
	a.Hasta(func() bool { return cond(espacio(t, cli, nombre)) })
}

// linea es la primera línea de la vista que contiene texto; "" si ninguna.
func linea(a *prueba.App, texto string) string {
	for _, l := range strings.Split(a.Vista(), "\n") {
		if strings.Contains(l, texto) {
			return l
		}
	}
	return ""
}

// hastaFila espera a que la fila del espacio diga todos esos textos.
func hastaFila(t *testing.T, a *prueba.App, nombre string, textos ...string) {
	t.Helper()
	a.Hasta(func() bool {
		l := linea(a, nombre)
		for _, s := range textos {
			if !strings.Contains(l, s) {
				return false
			}
		}
		return l != ""
	})
}

// elegir baja el cursor hasta la fila del espacio.
func elegir(t *testing.T, a *prueba.App, nombre string) {
	t.Helper()
	a.Pulsar("home")
	for range 20 {
		if strings.Contains(linea(a, "▸"), nombre) {
			return
		}
		a.Pulsar("down")
	}
	t.Fatalf("no se llega a %s:\n%s", nombre, a.Vista())
}

func pregunta(fuera []tea.Msg) (comp.Pregunta, bool) {
	for _, m := range fuera {
		if q, ok := m.(comp.Pregunta); ok {
			return q, true
		}
	}
	return comp.Pregunta{}, false
}

// hastaAviso hace la acción y espera, entregando a la página lo que llegue, al primer aviso que sale tras ella.
func hastaAviso(a *prueba.App, accion func()) comp.NuevoAviso {
	desde := len(a.Fuera)
	accion()
	var av comp.NuevoAviso
	a.Hasta(func() bool {
		var ok bool
		av, ok = aviso(a.Fuera[desde:])
		return ok
	})
	return av
}

// TestEspaciosLista: la global y los proyectos, con su estado, si se confía en ellos y dónde estás; la ruta, las APIs y las
// peticiones del elegido, como las tarjetas de la Web UI.
func TestEspaciosLista(t *testing.T) {
	cli, id := prueba.Servicio(t)
	notas, _ := proyecto(t, cli, "Notas")
	a := abierta(t, cli, id)
	hastaFila(t, a, "Global", tx("espacios.estadoActivo"), tx("espacios.actual"))
	hastaFila(t, a, notas.Nombre, tx("espacios.estadoActivo"), tx("espacios.confiado"))
	if strings.Contains(linea(a, notas.Nombre), tx("espacios.actual")) {
		t.Fatalf("solo la global es la actual:\n%s", a.Vista())
	}
	if l := linea(a, "Global"); strings.Contains(l, tx("espacios.confiado")) {
		t.Fatalf("la global no dice si es de confianza: %q", l)
	}
	a.Pulsar("home")
	a.HastaVer(tx("app.sinProyecto"), tx("espacios.apis", "n", "0"), tx("espacios.peticiones", "n", "0"))
	elegir(t, a, notas.Nombre)
	a.HastaVer(filepath.Base(notas.Ruta))
}

// TestEspaciosConfiar: c pregunta antes de confiar y dice qué controlan las claves de seguridad de su local.yaml, nunca
// su nombre interno; confirmado, el servicio confía. Dejar de confiar también pregunta, y cancelar no cambia nada.
func TestEspaciosConfiar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	si := true
	e, err := cli.AbrirProyecto(context.Background(), clonado(t, "Soporte"), &si, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Confiar(context.Background(), e.ID, false); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	hastaFila(t, a, "Soporte", tx("espacios.sinConfiar"))
	elegir(t, a, "Soporte")
	q, ok := pregunta(a.Pulsar("c"))
	if !ok || !strings.Contains(q.Texto, tx("espacios.confiarTitulo", "nombre", "Soporte")) ||
		!strings.Contains(q.Texto, tx("espacios.confiarClaves", "claves", "el programa de cada agente")) || strings.Contains(q.Texto, "comandos_agentes") {
		t.Fatalf("c pregunta con las claves por lo que controlan: %+v", q)
	}
	a.Entregar(comp.Confirmado{})
	hastaServicio(t, a, cli, "Soporte", func(e contrato.Espacio, _ bool) bool { return e.Confiado })
	hastaFila(t, a, "Soporte", tx("espacios.confiado"))

	q, ok = pregunta(a.Pulsar("c"))
	if !ok || !strings.Contains(q.Texto, tx("espacios.dejarDeConfiarTitulo", "nombre", "Soporte")) || !strings.Contains(q.Texto, tx("espacios.dejarDeConfiarTexto")) {
		t.Fatalf("dejar de confiar pregunta antes: %+v", q)
	}
	a.Entregar(comp.Cancelado{})
	if e, _ := espacio(t, cli, "Soporte"); !e.Confiado {
		t.Fatal("cancelar no deja de confiar")
	}
	pregunta(a.Pulsar("c"))
	a.Entregar(comp.Confirmado{})
	hastaServicio(t, a, cli, "Soporte", func(e contrato.Espacio, _ bool) bool { return !e.Confiado })
	hastaFila(t, a, "Soporte", tx("espacios.sinConfiar"))

	if f := a.Pulsar("home", "c"); len(f) != 0 {
		t.Fatalf("en la global, c no hace nada: %v", f)
	}
}

// TestEspaciosAbrirConConfianza: n abre un proyecto; Tab completa la ruta con las carpetas del disco. Si la carpeta ya
// traía la marca de Tramtai, pregunta como la Web UI, con las claves de seguridad, y c confía y lo abre. Una carpeta dentro
// de otro proyecto ofrece abrir ese.
func TestEspaciosAbrirConConfianza(t *testing.T) {
	cli, id := prueba.Servicio(t)
	dir := clonado(t, "Soporte")
	a := abierta(t, cli, id)
	a.Pulsar("n")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), tx("proyecto.titulo")) || !strings.Contains(a.Vista(), tx("proyecto.ruta")) {
		t.Fatalf("n abre el formulario del proyecto:\n%s", a.Vista())
	}
	a.Pegar(filepath.Dir(dir) + string(filepath.Separator) + "sop")
	a.Pulsar("tab") // sin completar, «sop» no existe y no habría pregunta
	a.Pulsar("enter")
	a.HastaVer(tx("proyecto.confiarTitulo"), "el programa de cada agente", tx("proyecto.confiarYAbrir"), tx("proyecto.abrirSinConfiar"))
	if _, ok := espacio(t, cli, "Soporte"); ok || a.P.EnCampo() {
		t.Fatalf("sin decidir, no se abre; sin campo de texto, las teclas comunes valen (EnCampo %v)", a.P.EnCampo())
	}
	if av := hastaAviso(a, func() { a.Pulsar("c") }); av.Texto != tx("proyecto.abierto", "nombre", "Soporte") || av.Tipo != comp.Exito {
		t.Fatalf("avisa de que se abrió: %+v", av)
	}
	if e, ok := espacio(t, cli, "Soporte"); !ok || !e.Confiado || e.Estado != "activo" {
		t.Fatalf("abierto y confiado: %+v", e)
	}
	if a.P.EnCampo() || strings.Contains(a.Vista(), tx("proyecto.titulo")) {
		t.Fatalf("el formulario se cierra:\n%s", a.Vista())
	}

	// Sin confiar: se abre, pero no se confía.
	otro := clonado(t, "Notas")
	a.Pulsar("n")
	a.Pegar(otro)
	a.Pulsar("enter")
	a.HastaVer(tx("proyecto.abrirSinConfiar"))
	hastaAviso(a, func() { a.Pulsar("a") })
	if e, ok := espacio(t, cli, "Notas"); !ok || e.Confiado {
		t.Fatalf("abierto sin confiar: %+v", e)
	}

	// Dentro de otro proyecto: ofrece abrir ese, y Esc vuelve a la ruta.
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o700)
	a.Pulsar("n")
	a.Pegar(sub)
	a.Pulsar("enter")
	a.HastaVer(tx("proyecto.abrirEse"), tx("proyecto.crearAqui"))
	if f := a.Pulsar("esc"); len(f) != 0 || !a.P.EnCampo() || strings.Contains(a.Vista(), tx("proyecto.abrirEse")) {
		t.Fatalf("Esc vuelve a la ruta: %v\n%s", f, a.Vista())
	}
	a.Pulsar("enter")
	a.HastaVer(tx("proyecto.abrirEse"))
	if av := hastaAviso(a, func() { a.Pulsar("a") }); av.Texto != tx("proyecto.abierto", "nombre", "Soporte") {
		t.Fatalf("abre el de arriba: %+v", av)
	}
}

// TestEspaciosOlvidarNoBorraCarpeta: x pide escribir el nombre del espacio; con otro nombre no hace nada y, con el suyo, el
// servicio lo olvida y la carpeta del proyecto sigue en su sitio. Un espacio activo no se olvida.
func TestEspaciosOlvidarNoBorraCarpeta(t *testing.T) {
	cli, id := prueba.Servicio(t)
	e, dir := proyecto(t, cli, "Notas")
	a := abierta(t, cli, id)
	elegir(t, a, e.Nombre)
	if f := a.Pulsar("x"); len(f) != 0 || a.P.EnCampo() {
		t.Fatalf("activo, x no hace nada: %v", f)
	}
	if err := cli.DetenerEspacio(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	a.Pulsar("r")
	hastaFila(t, a, e.Nombre, tx("espacios.estadoDetenido"))
	a.Pulsar("x")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), tx("espacios.olvidarTitulo", "nombre", e.Nombre)) {
		t.Fatalf("x pide el nombre:\n%s", a.Vista())
	}
	a.Escribir("otro")
	a.Pulsar("enter")
	if _, ok := espacio(t, cli, e.Nombre); !ok || !a.P.EnCampo() {
		t.Fatalf("con otro nombre no se olvida:\n%s", a.Vista())
	}
	for range 4 {
		a.Pulsar("backspace")
	}
	a.Escribir(strings.ToUpper(e.Nombre) + " ")
	if av := hastaAviso(a, func() { a.Pulsar("enter") }); av.Texto != tx("espacios.olvidado", "nombre", e.Nombre) {
		t.Fatalf("avisa de que se olvidó: %+v", av)
	}
	hastaServicio(t, a, cli, e.Nombre, func(_ contrato.Espacio, ok bool) bool { return !ok })
	a.Hasta(func() bool { return linea(a, e.Nombre) == "" })
	if _, err := os.Stat(filepath.Join(dir, ".tramtai", "config.yaml")); err != nil {
		t.Fatalf("la carpeta del proyecto sigue: %v", err)
	}
}

// TestEspaciosDetenerActivarYBuscar: d pregunta antes de detener; a lo vuelve a activar; uno cuya carpeta ya no está se
// busca con u en su ruta nueva.
func TestEspaciosDetenerActivarYBuscar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	e, dir := proyecto(t, cli, "Notas")
	a := abierta(t, cli, id)
	elegir(t, a, e.Nombre)
	q, ok := pregunta(a.Pulsar("d"))
	if !ok || !strings.Contains(q.Texto, tx("espacios.detenerTitulo", "nombre", e.Nombre)) || !strings.Contains(q.Texto, tx("espacios.detenerTexto")) {
		t.Fatalf("d pregunta: %+v", q)
	}
	a.Entregar(comp.Confirmado{})
	hastaServicio(t, a, cli, e.Nombre, func(e contrato.Espacio, _ bool) bool { return e.Estado == "detenido" })
	hastaFila(t, a, e.Nombre, tx("espacios.estadoDetenido"))

	a.Pulsar("a")
	hastaServicio(t, a, cli, e.Nombre, func(e contrato.Espacio, _ bool) bool { return e.Estado == "activo" })
	hastaFila(t, a, e.Nombre, tx("espacios.estadoActivo"))

	if err := cli.DetenerEspacio(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	movida := dir + "-movida"
	if err := os.Rename(dir, movida); err != nil {
		t.Fatal(err)
	}
	a.Pulsar("r")
	hastaFila(t, a, e.Nombre, tx("espacios.estadoNoEncontrado"))
	a.Pulsar("u")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), tx("espacios.buscarTitulo", "nombre", e.Nombre)) {
		t.Fatalf("u pide la ruta:\n%s", a.Vista())
	}
	a.Pegar(movida)
	a.Pulsar("enter")
	hastaServicio(t, a, cli, e.Nombre, func(e contrato.Espacio, _ bool) bool {
		return e.Estado == "activo" && filepath.Base(e.Ruta) == filepath.Base(movida)
	})
	hastaFila(t, a, e.Nombre, tx("espacios.estadoActivo"))
}

// TestEspaciosAbrirOtro: Intro en un espacio activo que no es el actual cambia a él y lleva a Inicio, como Abrir de la
// Web UI; en el actual no hace nada.
func TestEspaciosAbrirOtro(t *testing.T) {
	cli, id := prueba.Servicio(t)
	e, _ := proyecto(t, cli, "Notas")
	a := abierta(t, cli, id)
	if f := a.Pulsar("home", "enter"); len(f) != 0 {
		t.Fatalf("en el actual, Intro no hace nada: %v", f)
	}
	elegir(t, a, e.Nombre)
	var cambio []tea.Msg
	cmds := a.Retener(func() { a.Pulsar("enter") })
	cambio = a.Recoger(cmds)
	i, j := slices.Index(cambio, tea.Msg(pagina.CambioEspacio{ID: e.ID})), slices.Index(cambio, tea.Msg(pagina.IrA{Pagina: pagina.PaginaInicio}))
	if i < 0 || j < i {
		t.Fatalf("cambia de espacio y después va a Inicio: %v", cambio)
	}
}

// TestEspaciosAvisaALaApp: tras un cambio con éxito, la página pide a la app que relea los espacios, como api.sesion() en la
// Web UI.
func TestEspaciosAvisaALaApp(t *testing.T) {
	cli, id := prueba.Servicio(t)
	e, _ := proyecto(t, cli, "Notas")
	if err := cli.DetenerEspacio(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	elegir(t, a, e.Nombre)
	respuesta := a.Recoger(a.Retener(func() { a.Pulsar("a") }))
	if len(respuesta) != 1 {
		t.Fatalf("activar responde una vez: %v", respuesta)
	}
	tras := a.Recoger(a.Retener(func() { a.Entregar(respuesta[0]) }))
	if !slices.Contains(tras, tea.Msg(pagina.EspaciosCambiados{})) {
		t.Fatalf("pide a la app que relea los espacios: %v", tras)
	}
}

func TestEscVuelve(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	a.Pulsar("n")
	if f := a.Pulsar("esc"); len(f) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc cierra el formulario: %v", f)
	}
	if f := a.Pulsar("esc"); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("sin nada abierto, Esc vuelve: %+v", f)
	}
}

// TestServicioCaido: sin cliente o con el servicio cerrado, la página no se cierra y la app avisa.
func TestServicioCaido(t *testing.T) {
	hastaError := func(a *prueba.App) {
		a.Hasta(func() bool {
			return slices.ContainsFunc(a.Fuera, func(m tea.Msg) bool { e, ok := m.(pagina.ErrorServicio); return ok && pagina.NoResponde(e.Err) })
		})
	}
	sin := prueba.Conducir(t, espacios.Nueva("es"))
	sin.Correr(sin.P.Iniciar(contexto(t, nil, "global")))
	hastaError(sin)
	cli, id, cerrar := prueba.Arrancar(t)
	a := abierta(t, cli, id)
	cerrar()
	a.Pulsar("r")
	hastaError(a)
	a.HastaVer("El servicio no responde · r reintenta")
}

// TestEstadosDeCarga: si la lista no llega, se dice con el mensaje del servicio y r reintenta; sin espacios, el vacío
// invita a abrir un proyecto; con la lista a la vista, un fallo al recargar es un aviso.
func TestEstadosDeCarga(t *testing.T) {
	cli, id := prueba.Servicio(t)
	proxy, fallar := prueba.ConFallo(t, cli)
	fallar("la lista no se pudo leer")
	a := abierta(t, proxy, id)
	if v := a.Vista(); !strings.Contains(v, tx("comun.errorCarga")+": la lista no se pudo leer") || !strings.Contains(v, "r "+tx("comun.reintentar")) {
		t.Fatalf("el error de carga:\n%s", v)
	}
	fallar("")
	a.Pulsar("r")
	a.HastaVer("Global")
	fallar("otra vez no")
	if av := hastaAviso(a, func() { a.Pulsar("r") }); av.Texto != "otra vez no" || av.Tipo != comp.Fallo {
		t.Fatalf("un aviso de fallo: %+v", av)
	}
	if !strings.Contains(a.Vista(), "Global") {
		t.Fatalf("la lista sigue:\n%s", a.Vista())
	}

	vacio := *cli
	vacio.Base = prueba.Servidor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"espacios": []any{}})
	}))
	b := abierta(t, &vacio, id)
	if v := b.Vista(); !strings.Contains(strings.ReplaceAll(v, "\n", " "), tx("espacios.vacio")) || !strings.Contains(v, "n "+tx("proyecto.abrir")) {
		t.Fatalf("el vacío invita a abrir un proyecto:\n%s", v)
	}
}

// TestRespuestasViejas: una carga pedida antes de volver a iniciar la página se descarta aunque llegue después; un cambio
// ya hecho en el servicio en el mismo espacio sí se aplica.
func TestRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	vieja := a.Recoger(a.Retener(func() { a.Pulsar("r") })) // la carga de antes, sin Notas
	e, _ := proyecto(t, cli, "Notas")
	a.Correr(a.P.Iniciar(contexto(t, cli, id)))
	a.HastaVer(e.Nombre)
	if len(vieja) != 1 {
		t.Fatalf("la carga vieja trae un mensaje: %v", vieja)
	}
	a.Entregar(vieja[0])
	if !strings.Contains(a.Vista(), e.Nombre) {
		t.Fatalf("la carga vieja se descarta:\n%s", a.Vista())
	}

	// Una carga de otro espacio tampoco vale.
	otro := a.Recoger(a.Retener(func() { a.Pulsar("r") }))
	c := contexto(t, cli, id)
	c.Instancia = e.ID
	a.P.Iniciar(c) // la app cambió de espacio; su carga no se corre
	a.Entregar(otro[0])
	if !strings.Contains(a.Vista(), "Cargando…") {
		t.Fatalf("la carga de otro espacio se descarta:\n%s", a.Vista())
	}

	// Un cambio retenido del mismo espacio se aplica tras volver a iniciar.
	a = abierta(t, cli, id)
	elegir(t, a, e.Nombre)
	pregunta(a.Pulsar("d"))
	accion := a.Retener(func() { a.Entregar(comp.Confirmado{}) })
	a.Correr(a.P.Iniciar(contexto(t, cli, id)))
	a.Soltar(accion)
	hastaServicio(t, a, cli, e.Nombre, func(e contrato.Espacio, _ bool) bool { return e.Estado == "detenido" })
	hastaFila(t, a, e.Nombre, tx("espacios.estadoDetenido"))
}

// TestCabe: la lista, los formularios y la pregunta de abrir un proyecto caben en el área de 60×20, también con una ventana
// más estrecha, y las confirmaciones caben en la caja de la app.
func TestCabe(t *testing.T) {
	cli, id := prueba.Servicio(t)
	si := true
	e, err := cli.AbrirProyecto(context.Background(), clonado(t, "Soporte-con-un-nombre-bastante-largo-para-la-fila"), &si, false)
	if err != nil {
		t.Fatal(err)
	}
	cli.Confiar(context.Background(), e.ID, false)
	notas, _ := proyecto(t, cli, "Notas")
	cli.DetenerEspacio(context.Background(), notas.ID)
	otra := clonado(t, "Otra")
	a := abierta(t, cli, id)
	for _, tam := range [][2]int{{60, 17}, {40, 10}} {
		a.Entregar(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		comprobar := func(donde string) {
			t.Helper()
			v := a.Vista()
			if lipgloss.Height(v) > tam[1] || lipgloss.Width(v) > tam[0] {
				t.Errorf("%s a %dx%d mide %dx%d:\n%s", donde, tam[0], tam[1], lipgloss.Width(v), lipgloss.Height(v), v)
			}
		}
		comprobar("lista")
		a.Pulsar("n")
		comprobar("abrir un proyecto")
		a.Pegar(otra)
		a.Pulsar("enter")
		a.HastaVer(tx("proyecto.confiarTitulo"))
		comprobar("la pregunta de confianza")
		a.Pulsar("esc", "esc")
		elegir(t, a, notas.Nombre)
		a.Pulsar("x")
		comprobar("olvidar")
		a.Pulsar("esc")
	}
	a.Entregar(tea.WindowSizeMsg{Width: 60, Height: 17})
	var preguntas []string
	for _, paso := range []struct{ nombre, tecla string }{{"Soporte-con-un", "c"}, {notas.Nombre, "a"}} {
		elegir(t, a, paso.nombre)
		if paso.tecla == "a" {
			a.Pulsar("a")
			hastaFila(t, a, notas.Nombre, tx("espacios.estadoActivo"))
			for _, k := range []string{"d", "c"} {
				q, _ := pregunta(a.Pulsar(k))
				a.Entregar(comp.Cancelado{})
				preguntas = append(preguntas, q.Texto)
			}
			continue
		}
		q, _ := pregunta(a.Pulsar(paso.tecla))
		a.Entregar(comp.Cancelado{})
		preguntas = append(preguntas, q.Texto)
	}
	for _, texto := range preguntas {
		if texto == "" {
			t.Fatal("falta una pregunta")
		}
		v := (&comp.Confirmacion{Texto: texto}).Vista(&pagina.Estilos{Iconos: pagina.IconosUnicode}, 58, 17)
		if h := lipgloss.Height(v); h > 17 || !strings.Contains(v, "s confirma · Esc cancela") {
			t.Fatalf("la pregunta mide %d filas:\n%s", h, v)
		}
	}
}
