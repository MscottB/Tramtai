package ejecutar_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/cliente"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/ejecutar"
	"tramtai/internal/tui/paginas/prueba"
	"tramtai/internal/tui/vivo"
)

// Las pruebas de Probar van por su interfaz: teclas, los mensajes de pagina y comp, y lo que enseña Vista(). Lo que la
// página manda al servicio se lee en el servicio. Lo que trae el servicio tras una tecla (el trabajo creado, las
// recientes, el error del esquema, el aviso de una acción) se espera con Hasta, HastaVer o HastaSalir antes de mirarlo
// (regla 20): Asentar no ve los temporizadores del servicio ni todas las esperas de E/S.

var ctx = context.Background()

// tx es un texto de la Web UI en español.
func tx(clave string, pares ...string) string { return datos.Textof("es", clave, pares...) }

// estado es el nombre de un estado como se ve.
func estado(e nucleo.Estado) string { return vivo.NombreEstado("es", e) }

// enEspanol pone el idioma activo en español mientras dura la prueba: los avisos de la página salen en español.
func enEspanol(t *testing.T) {
	antes := idioma.Activo
	idioma.Activo = "es"
	t.Cleanup(func() { idioma.Activo = antes })
}

// contexto es el de la página en un área alta, donde cabe el formulario entero, con su carpeta de datos.
func contexto(t *testing.T, cli *cliente.Cliente, instancia, carpeta string) pagina.Contexto {
	c := prueba.Contexto(t, cli, instancia)
	c.Ancho, c.Alto, c.Datos = 120, 400, carpeta
	return c
}

// abrir inicia Probar con ese contexto.
func abrir(t *testing.T, c pagina.Contexto) *prueba.App {
	t.Helper()
	a, _ := abrirConReloj(t, c)
	return a
}

// abrirConReloj inicia Probar con un reloj de prueba: el borrador y la reconexión esperan a que la prueba haga pasar
// el tiempo.
func abrirConReloj(t *testing.T, c pagina.Contexto) (*prueba.App, *prueba.Reloj) {
	t.Helper()
	enEspanol(t)
	r := prueba.NuevoReloj()
	return prueba.Abrir(t, ejecutar.NuevaConReloj(c.Idioma, r.Ahora, r.Espera), c), r
}

// guardarBorrador hace pasar la espera del borrador y deja que se escriba.
func guardarBorrador(a *prueba.App, r *prueba.Reloj) {
	r.Pasar(ejecutar.EsperaBorrador)
	a.Asentar()
}

// hayBorrador dice si el borrador está en la carpeta de datos, y qué mensaje guarda.
func hayBorrador(carpeta string) (string, bool) {
	datos, err := os.ReadFile(filepath.Join(carpeta, archivoBorrador))
	return string(datos), err == nil
}

// abierta abre Probar contra el servicio, con una carpeta de datos propia.
func abierta(t *testing.T, cli *cliente.Cliente, id string) *prueba.App {
	t.Helper()
	return abrir(t, contexto(t, cli, id, t.TempDir()))
}

// conBloqueado levanta el servicio con el agente bloqueado de prueba, que con «esperar» escribe «primera parte» y espera
// a que la prueba lo suelte (SoltarFin).
func conBloqueado(t *testing.T) (*cliente.Cliente, string, *prueba.Bloqueado) {
	t.Helper()
	cli, id, b := prueba.ConBloqueado(t)
	b.SoltarPaso()
	return cli, id, b
}

var aviso = prueba.Aviso

// copiado es lo que la página mandó copiar con OSC 52.
func copiado(fuera []tea.Msg) (string, bool) {
	for _, m := range fuera {
		if c, ok := m.(prueba.Portapapeles); ok {
			return string(c), true
		}
	}
	return "", false
}

// pregunta es la primera pregunta entre lo que salió hacia la app.
func pregunta(fuera []tea.Msg) (comp.Pregunta, bool) {
	for _, m := range fuera {
		if q, ok := m.(comp.Pregunta); ok {
			return q, true
		}
	}
	return comp.Pregunta{}, false
}

// errorServicio dice si la página avisó a la app de que el servicio no responde.
func errorServicio(fuera []tea.Msg) bool {
	return slices.ContainsFunc(fuera, func(m tea.Msg) bool { _, ok := m.(pagina.ErrorServicio); return ok })
}

// lineas son las líneas de la vista, sin los códigos de color ni el del cursor.
func lineas(a *prueba.App) []string { return strings.Split(ansi.Strip(a.Vista()), "\n") }

// pestana dice si la vista enseña esa pestaña como la elegida (su nombre entre corchetes).
func pestana(a *prueba.App, clave string) bool { return strings.Contains(lineas(a)[0], tx(clave)+"]") }

// sinMarcas quita de una línea del formulario el cursor y la marca de un pliegue.
func sinMarcas(l string) string {
	l = strings.TrimPrefix(strings.TrimPrefix(l, "▸ "), "  ")
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "▸ "), "▾ "))
}

// foco es la etiqueta del campo con el foco (la línea del «▸»); "" si no se ve.
func foco(a *prueba.App) string {
	for _, l := range lineas(a)[1:] {
		if strings.HasPrefix(l, "▸ ") {
			return sinMarcas(l)
		}
	}
	return ""
}

// aLaReceta pasa a la pestaña de la receta.
func aLaReceta(t *testing.T, a *prueba.App) {
	t.Helper()
	for range 3 {
		if pestana(a, "ejecutar.receta") {
			return
		}
		a.Pulsar("left")
	}
	t.Fatalf("no se llega a la receta:\n%s", a.Vista())
}

// enfocar lleva el foco al campo con esa etiqueta con ↓, como quien recorre el formulario, fuera de la edición.
func enfocar(t *testing.T, a *prueba.App, etiqueta string) {
	t.Helper()
	if a.P.EnCampo() {
		a.Pulsar("esc")
	}
	aLaReceta(t, a)
	for range 100 {
		if strings.HasPrefix(foco(a), etiqueta) {
			return
		}
		a.Pulsar("down")
	}
	t.Fatalf("no se llega al campo %q:\n%s", etiqueta, a.Vista())
}

// editar enfoca el campo y entra a editarlo con Intro.
func editar(t *testing.T, a *prueba.App, etiqueta string) {
	t.Helper()
	enfocar(t, a, etiqueta)
	a.Pulsar("enter")
	if !a.P.EnCampo() {
		t.Fatalf("Intro no edita %q:\n%s", etiqueta, a.Vista())
	}
}

// escribir edita el campo, escribe el texto y deja el campo con Esc.
func escribir(t *testing.T, a *prueba.App, etiqueta, texto string) []tea.Msg {
	t.Helper()
	editar(t, a, etiqueta)
	fuera := a.Escribir(texto)
	return append(fuera, a.Pulsar("esc")...)
}

// vaciar borra lo que tiene el campo que se edita: va al final y borra, línea a línea, todo lo de antes.
func vaciar(a *prueba.App) {
	teclas := []string{"ctrl+end", "end"}
	for range 100 {
		teclas = append(teclas, "ctrl+u", "backspace")
	}
	a.Teclear(teclas...)
}

// poner deja en el campo solo ese texto, pegado de una vez, y lo deja con Esc.
func poner(t *testing.T, a *prueba.App, etiqueta, texto string) []tea.Msg {
	t.Helper()
	editar(t, a, etiqueta)
	vaciar(a)
	var fuera []tea.Msg
	if texto != "" {
		fuera = a.Pegar(texto)
	}
	return append(fuera, a.Pulsar("esc")...)
}

// valor es lo que enseña el campo bajo su etiqueta: una línea o, en un área, las suyas; de una opción, la elegida.
func valor(t *testing.T, a *prueba.App, etiqueta string) string {
	t.Helper()
	l := lineas(a)
	for i, x := range l {
		if sinMarcas(x) != etiqueta || i+1 >= len(l) {
			continue
		}
		var partes []string
		for _, y := range l[i+1:] {
			if strings.TrimSpace(y) == "" || strings.HasPrefix(y, "↑↓ ") { // tras el campo, otro o la posición del foco
				break
			}
			if r, ok := strings.CutPrefix(strings.TrimSpace(y), "‹ "); ok && len(partes) == 0 { // una opción: su valor
				return strings.TrimSuffix(r, " ›")
			}
			y = strings.TrimSpace(y)
			if r, ok := strings.CutPrefix(y, "│"); ok { // una línea de un área
				y = strings.TrimSpace(r)
			}
			partes = append(partes, y)
		}
		v := strings.TrimSpace(strings.Join(partes, "\n"))
		if r, ok := strings.CutPrefix(v, "‹ "); ok {
			v = strings.TrimSuffix(r, " ›")
		}
		return v
	}
	t.Fatalf("no se ve el campo %q:\n%s", etiqueta, a.Vista())
	return ""
}

// elegir cambia una opción con → hasta que enseña ese valor.
func elegir(t *testing.T, a *prueba.App, etiqueta, quiere string) []tea.Msg {
	t.Helper()
	editar(t, a, etiqueta)
	var fuera []tea.Msg
	for i := 0; i < 20 && valor(t, a, etiqueta) != quiere; i++ {
		fuera = append(fuera, a.Pulsar("right")...)
	}
	if v := valor(t, a, etiqueta); v != quiere {
		t.Fatalf("%s no llega a %q: %q", etiqueta, quiere, v)
	}
	return append(fuera, a.Pulsar("esc")...)
}

// marcado dice si un interruptor está puesto.
func marcado(t *testing.T, a *prueba.App, etiqueta string) bool {
	return valor(t, a, etiqueta) == "[x]"
}

// abrirPliegue abre un pliegue del formulario, si estaba cerrado.
func abrirPliegue(t *testing.T, a *prueba.App, clave string) {
	t.Helper()
	enfocar(t, a, tx(clave))
	if !strings.Contains(a.Vista(), "▸ ▾ "+tx(clave)) {
		a.Pulsar("enter")
	}
}

// conEstructurada activa la salida estructurada.
func conEstructurada(t *testing.T, a *prueba.App) {
	t.Helper()
	abrirPliegue(t, a, "receta.opciones")
	if enfocar(t, a, tx("receta.salidaEstructurada")); !marcado(t, a, tx("receta.salidaEstructurada")) {
		a.Pulsar("enter", "space", "esc")
	}
}

// aJSON pasa el esquema a JSON.
func aJSON(t *testing.T, a *prueba.App) { elegir(t, a, tx("receta.modo"), tx("receta.modoJson")) }

// enviar manda la prueba con Ctrl+S.
func enviar(a *prueba.App) []tea.Msg { return a.Pulsar("ctrl+s") }

// terminada espera a que la respuesta enseñe un estado final, con el trabajo ya cerrado (el pie ofrece «Nueva prueba»), y
// lo devuelve.
func terminada(t *testing.T, a *prueba.App) nucleo.Estado {
	t.Helper()
	var final nucleo.Estado
	a.Hasta(func() bool {
		if !slices.Contains(a.P.Teclas(), pagina.Tecla{Tecla: "n", Accion: tx("ejecutar.nueva")}) {
			return false
		}
		cabecera := strings.Join(lineas(a)[:min(3, len(lineas(a)))], "\n")
		for _, e := range nucleo.Terminales {
			if strings.Contains(cabecera, estado(e)) {
				final = e
				return true
			}
		}
		return false
	})
	return final
}

// ultimo es el trabajo más reciente del espacio, con su solicitud.
func ultimo(t *testing.T, cli *cliente.Cliente, id string) nucleo.Trabajo {
	t.Helper()
	lista, err := cli.Trabajos(ctx, id, 1)
	if err != nil || len(lista) == 0 {
		t.Fatalf("no hay trabajos: %v", err)
	}
	tr, err := cli.Trabajo(ctx, id, lista[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// cuantos son los trabajos del espacio.
func cuantos(t *testing.T, cli *cliente.Cliente, id string) int {
	t.Helper()
	lista, err := cli.Trabajos(ctx, id, 50)
	if err != nil {
		t.Fatal(err)
	}
	return len(lista)
}

// compacto es el JSON sin espacios.
func compacto(t *testing.T, texto string) string {
	t.Helper()
	var v json.RawMessage
	if err := json.Unmarshal([]byte(texto), &v); err != nil {
		t.Fatalf("%v: %s", err, texto)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// pendiente es lo que la página diría al salir de la terminal.
func pendiente(a *prueba.App) string { return a.P.(pagina.Pendiente).Pendiente() }

// ocultar y mostrar son lo que hace la app al dejar la página y al volver a ella.
func ocultar(a *prueba.App) { a.P.(pagina.Ocultable).Ocultar() }

func mostrar(a *prueba.App) []tea.Msg { return a.Correr(a.P.(pagina.Ocultable).Mostrar()) }
