package ejecutar

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

var ctx = context.Background()

var tecla = prueba.Tecla

// portapapeles es lo que la página manda copiar con OSC 52 (el mensaje de tea.SetClipboard no se exporta).
type portapapeles string

// correr ejecuta un Cmd y alimenta a la página con lo que sale, como haría la app: el seguimiento en vivo se lee con
// hasta. Devuelve los mensajes que son de la app, y lo que se copia como portapapeles.
func correr(p *Pagina, cmd tea.Cmd) []tea.Msg {
	var fuera []tea.Msg
	cola := []tea.Cmd{cmd}
	for len(cola) > 0 {
		c := cola[0]
		cola = cola[1:]
		if c == nil {
			continue
		}
		switch m := c().(type) {
		case nil:
		case tea.BatchMsg:
			cola = append(cola, m...)
		case comp.Pregunta, comp.NuevoAviso, pagina.IrA, pagina.Volver, pagina.ErrorServicio:
			fuera = append(fuera, m)
		case comp.Enviado, comp.Cancelado: // los manda la app de vuelta a la página
			_, siguiente := p.Update(m)
			cola = append(cola, siguiente)
		case msgFlujo:
			p.Update(m)
		default:
			if fmt.Sprintf("%T", m) == "tea.setClipboardMsg" {
				fuera = append(fuera, portapapeles(fmt.Sprint(m)))
				continue
			}
			_, siguiente := p.Update(m)
			cola = append(cola, siguiente)
		}
	}
	return fuera
}

// hasta lee el seguimiento en vivo hasta que se cumpla la condición.
func hasta(t *testing.T, p *Pagina, cond func() bool) {
	t.Helper()
	limite := time.After(5 * time.Second)
	for !cond() {
		select {
		case m, ok := <-p.canal:
			if !ok {
				t.Fatalf("el seguimiento se cerró antes de tiempo:\n%s", p.Vista())
			}
			if p.vivoVigente(m) { // sin Update: su Cmd volvería a esperar el canal y bloquearía la prueba
				correr(p, p.alVivo(m))
			}
		case <-limite:
			t.Fatalf("no llegó lo esperado:\n%s", p.Vista())
		}
	}
}

func pulsar(p *Pagina, teclas ...string) []tea.Msg {
	var fuera []tea.Msg
	for _, k := range teclas {
		_, cmd := p.Update(tecla(k))
		fuera = append(fuera, correr(p, cmd)...)
	}
	return fuera
}

// escribir edita el campo id y escribe el texto letra a letra.
func escribir(p *Pagina, id, texto string) []tea.Msg {
	p.form.Enfocar(id)
	p.editando = true
	var fuera []tea.Msg
	for _, r := range texto {
		_, cmd := p.Update(tecla(string(r)))
		fuera = append(fuera, correr(p, cmd)...)
	}
	return fuera
}

func contexto(cli *cliente.Cliente, instancia, carpeta string) pagina.Contexto {
	return pagina.Contexto{Ctx: context.Background(), Cli: cli, Instancia: instancia, Idioma: "es",
		Estilos: &pagina.Estilos{Iconos: pagina.IconosUnicode}, Ancho: 60, Alto: 17, Datos: carpeta}
}

func nueva() *Pagina {
	p := Nueva("es").(*Pagina)
	p.esperaPara = func(time.Duration) <-chan time.Time {
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	return p
}

// abierta inicia la página contra el servicio y corre su carga.
func abierta(t *testing.T, cli *cliente.Cliente, id string) *Pagina {
	t.Helper()
	enEspanol(t)
	p := nueva()
	correr(p, p.Iniciar(contexto(cli, id, t.TempDir())))
	t.Cleanup(p.Ocultar)
	return p
}

var aviso = prueba.Aviso

// conBloqueado levanta el servicio con el agente bloqueado de prueba, que con «esperar» escribe «primera parte» y espera
// a que la prueba lo suelte (SoltarFin).
func conBloqueado(t *testing.T) (*cliente.Cliente, string, *prueba.Bloqueado) {
	t.Helper()
	cli, id, b := prueba.ConBloqueado(t)
	b.SoltarPaso()
	return cli, id, b
}

// TestEnviarYCancelar: Ctrl+S manda la receta como la Web UI y la respuesta llega en vivo; Esc la cancela.
func TestEnviarYCancelar(t *testing.T) {
	cli, id, b := conBloqueado(t)
	p := abierta(t, cli, id)
	escribir(p, "instrucciones", "breve")
	escribir(p, "respaldo", "simulado")
	escribir(p, "tiempo", "60")
	p.form.Enfocar("estructurada")
	pulsar(p, "space")
	p.form.Enfocar("reintentos")
	pulsar(p, "right", "right")
	escribir(p, "mensaje", "esperar")
	pulsar(p, "esc")
	if p.EnCampo() {
		t.Fatal("Esc deja el campo")
	}
	fuera := pulsar(p, "ctrl+s")
	select {
	case <-b.Entro:
	case <-time.After(5 * time.Second):
		t.Fatalf("el agente no recibió la petición: %v", fuera)
	}
	if p.pestana != pestRespuesta || p.trabajo == nil {
		t.Fatal("enviar enseña la respuesta")
	}
	hasta(t, p, func() bool { return p.salida == "primera parte" })
	if v := p.Vista(); !strings.Contains(v, "primera parte") || !strings.Contains(v, "en vivo") {
		t.Fatalf("la respuesta llega en vivo:\n%s", v)
	}
	s := b.Ultima()
	if s.Instrucciones != "breve" || len(s.Respaldo) != 1 || s.Ejecucion.TiempoLimiteSegundos != 60 ||
		s.Salida.Formato != "json" || s.Salida.Reintentos != 2 || !s.Salida.ConservarSalidaInvalida || !strings.Contains(string(s.Salida.Esquema), "resumen") {
		t.Fatalf("la solicitud lleva la receta como la Web UI: %+v", s)
	}
	tr, _ := cli.Trabajo(ctx, id, p.trabajo.ID)
	if tr.Origen != nucleo.DesdeTerminal {
		t.Fatalf("el historial dice que llegó de la terminal: %q", tr.Origen)
	}
	if fuera := pulsar(p, "ctrl+s"); len(fuera) != 0 || p.enviando {
		t.Fatal("con una en marcha no se envía otra")
	}
	pulsar(p, "esc")
	hasta(t, p, func() bool { return p.final != nil })
	if p.final.Estado != nucleo.Cancelado {
		t.Fatalf("Esc la cancela: %s", p.final.Estado)
	}
	if fuera := pulsar(p, "esc"); len(fuera) != 1 || fuera[0] != (pagina.Volver{}) {
		t.Fatalf("terminada, Esc vuelve: %v", fuera)
	}
	if lista, _ := cli.Trabajos(ctx, id, 10); len(lista) != 1 {
		t.Fatalf("una sola petición: %d", len(lista))
	}
	if len(p.recientes.Visibles()) != 1 {
		t.Fatalf("y queda en las recientes: %d", len(p.recientes.Visibles()))
	}
}

// TestCorregirYCopiar: una respuesta que no cumple el esquema se enseña con su error; o la corrige y y la copia.
func TestCorregirYCopiar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	p.form.Enfocar("estructurada")
	p.editando = true
	pulsar(p, "space", "esc")
	aJSON(p)
	p.form.Poner("esquema", `{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}`)
	p.form.Poner("mensaje", `{"total":"diez"}`)
	pulsar(p, "ctrl+s")
	hasta(t, p, func() bool { return p.final != nil })
	if p.final.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || p.salida != `{"total":"diez"}` {
		t.Fatalf("enseña la respuesta que no cumple: %+v %q", p.final, p.salida)
	}
	if v := p.Vista(); !strings.Contains(v, "Corregir") {
		t.Fatalf("ofrece corregir:\n%s", v)
	}
	fuera := pulsar(p, "y")
	if len(fuera) == 0 || fuera[0] != portapapeles(`{"total":"diez"}`) {
		t.Fatalf("y copia la respuesta con OSC 52: %v", fuera)
	}
	if a, ok := aviso(fuera); !ok || !strings.Contains(a.Texto, "OSC 52") {
		t.Fatalf("y avisa de que depende de la terminal: %v", fuera)
	}
	viejo := p.final.ID
	pulsar(p, "o")
	if p.trabajo == nil || p.trabajo.ID == viejo {
		t.Fatal("o crea otra petición y la sigue")
	}
	hasta(t, p, func() bool { return p.final != nil })
	nuevo, _ := cli.Trabajo(ctx, id, p.final.ID)
	if m := nuevo.Solicitud.Mensajes; !strings.HasPrefix(m[len(m)-1].Contenido, prefijoCorreccion) {
		t.Fatal("es una corrección, no un reintento")
	}
	// Una completada no se corrige.
	p.form.Poner("mensaje", `{"total":10}`)
	pulsar(p, "ctrl+s")
	hasta(t, p, func() bool { return p.final != nil && p.final.Estado == nucleo.Completado })
	if a, ok := aviso(pulsar(p, "o")); !ok || a.Texto != "Esta petición no se puede corregir" {
		t.Fatalf("avisa con el texto de la Web UI: %+v", a)
	}
	// n deja la página lista para otra prueba con la misma receta.
	pulsar(p, "n")
	if p.trabajo != nil || p.form.Valor("mensaje") != "" || p.pestana != pestReceta || !p.form.Marcado("estructurada") {
		t.Fatalf("Nueva prueba vacía el mensaje y conserva la receta: %+v", p.leer())
	}
	if pulsar(p, "n"); p.form.Valor("esquema") == "" {
		t.Fatal("sin respuesta, n no hace nada")
	}
}

// TestBorradorSobreviveReinicio: el mensaje se guarda al escribir y vuelve al abrir la terminal otra vez; se borra al enviar.
func TestBorradorSobreviveReinicio(t *testing.T) {
	enEspanol(t)
	cli, id := prueba.Servicio(t)
	carpeta := t.TempDir()
	p := nueva()
	correr(p, p.Iniciar(contexto(cli, id, carpeta)))
	escribir(p, "mensaje", "a medias")
	if got := leerBorrador(carpeta); got != "a medias" {
		t.Fatalf("el borrador se guarda: %q", got)
	}
	p.Ocultar()
	otra := nueva()
	correr(otra, otra.Iniciar(contexto(cli, id, carpeta)))
	t.Cleanup(otra.Ocultar)
	if otra.form.Valor("mensaje") != "a medias" {
		t.Fatalf("al volver a abrir, el mensaje es el del borrador: %q", otra.form.Valor("mensaje"))
	}
	pulsar(otra, "ctrl+s")
	if _, err := os.Stat(filepath.Join(carpeta, archivoBorrador)); !os.IsNotExist(err) {
		t.Fatalf("al enviar se borra: %v", err)
	}
	hasta(t, otra, func() bool { return otra.final != nil })
}

// TestSeguirTrabajo: lo que manda el Historial al reintentar o corregir abre ese trabajo con su receta y lo sigue en vivo.
func TestSeguirTrabajo(t *testing.T) {
	cli, id, b := conBloqueado(t)
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "simulado", Instrucciones: "corto", Esfuerzo: "",
		Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "esperar"}}})
	if err != nil {
		t.Fatal(err)
	}
	<-b.Entro
	p := abierta(t, cli, id)
	_, cmd := p.Update(pagina.SeguirTrabajo{ID: tr.ID})
	correr(p, cmd)
	if p.trabajo == nil || p.trabajo.ID != tr.ID || p.pestana != pestRespuesta {
		t.Fatal("abre el trabajo en la respuesta")
	}
	if p.form.Valor("mensaje") != "esperar" || p.form.Valor("instrucciones") != "corto" || p.valorOpcion("modelo") != "simulado" {
		t.Fatalf("con su receta y su mensaje: %+v", p.leer())
	}
	hasta(t, p, func() bool { return p.salida == "primera parte" })
	b.SoltarFin()
	hasta(t, p, func() bool { return p.final != nil && p.final.Estado == nucleo.Completado })
}

// TestUsarDeNuevo: las recientes son las de la terminal; u pone su mensaje y su receta; Intro la abre; h va al Historial.
func TestUsarDeNuevo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	otro := *cli
	otro.Origen = nucleo.DesdeCLI
	if _, err := otro.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "de la CLI"}}}); err != nil {
		t.Fatal(err)
	}
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "simulado", Respaldo: []string{"simulado"}, Cargas: nil,
		Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "otra vez"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Eventos(ctx, id, tr.ID, 0, func(int, string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	p := abierta(t, cli, id)
	pulsar(p, "right", "right")
	if v := p.Vista(); len(p.recientes.Visibles()) != 1 || !strings.Contains(v, "otra vez") || strings.Contains(v, "de la CLI") {
		t.Fatalf("solo las de la terminal:\n%s", v)
	}
	pulsar(p, "u")
	if p.pestana != pestReceta || p.form.Valor("mensaje") != "otra vez" || p.form.Valor("respaldo") != "simulado" {
		t.Fatalf("u trae mensaje y receta: %+v", p.leer())
	}
	pulsar(p, "left", "enter")
	if p.trabajo == nil || p.trabajo.ID != tr.ID || p.salida != "otra vez" {
		t.Fatalf("Intro la abre en la respuesta: %+v %q", p.trabajo, p.salida)
	}
	fuera := pulsar(p, "h")
	if len(fuera) != 1 || fuera[0] != (pagina.IrA{Pagina: pagina.PaginaHistorial, Datos: pagina.FiltroPunto{Nombre: "o:terminal"}}) {
		t.Fatalf("h va al Historial con las de la terminal: %v", fuera)
	}
}

// TestEscVuelve: fuera de un campo Esc vuelve; dentro, solo deja el campo.
func TestEscVuelve(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	if p.EnCampo() {
		t.Fatal("al llegar no se está dentro de un campo: ?, q y 1…7 son de la app")
	}
	if fuera := pulsar(p, "esc"); len(fuera) != 1 || fuera[0] != (pagina.Volver{}) {
		t.Fatalf("Esc fuera de campo vuelve: %v", fuera)
	}
	pulsar(p, "enter")
	if !p.EnCampo() {
		t.Fatal("Intro edita el campo")
	}
	if fuera := pulsar(p, "esc"); len(fuera) != 0 || p.EnCampo() {
		t.Fatalf("Esc en un campo solo lo deja: %v", fuera)
	}
}

func TestServicioCaido(t *testing.T) {
	enEspanol(t)
	p := nueva()
	cmd := p.Iniciar(contexto(nil, "global", ""))
	if fuera := correr(p, cmd); len(fuera) != 1 {
		t.Fatalf("sin cliente avisa a la app: %v", fuera)
	} else if _, ok := fuera[0].(pagina.ErrorServicio); !ok {
		t.Fatalf("con ErrorServicio: %v", fuera)
	}
	if v := p.Vista(); !strings.Contains(v, "Mensaje") {
		t.Fatalf("el formulario se ve igual:\n%s", v)
	}
	cli, id, cerrar := prueba.Arrancar(t)
	p2 := nueva()
	correr(p2, p2.Iniciar(contexto(cli, id, "")))
	t.Cleanup(p2.Ocultar)
	cerrar()
	p2.form.Poner("mensaje", "hola")
	fuera := pulsar(p2, "ctrl+s")
	if len(fuera) != 1 {
		t.Fatalf("con el servicio caído tras cargar, avisa a la app: %v", fuera)
	}
	if _, ok := fuera[0].(pagina.ErrorServicio); !ok || p2.enviando {
		t.Fatalf("y no se queda enviando: %v", fuera)
	}
}

// TestRespuestasViejas: lo que llega de otra versión de la página, de otro espacio o con el contexto cancelado no se aplica.
func TestRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	cancelado, cancelar := context.WithCancel(context.Background())
	cancelar()
	viejos := []origen{{ctx: p.c.Ctx, instancia: id, gen: p.gen - 1}, {ctx: p.c.Ctx, instancia: "otro", gen: p.gen}, {ctx: cancelado, instancia: id, gen: p.gen}}
	for _, o := range viejos {
		p.Update(msgCreado{origen: o, t: nucleo.Trabajo{ID: "viejo", Estado: nucleo.Completado}})
		p.Update(msgRecientes{origen: o, lista: []nucleo.Trabajo{{ID: "viejo"}, {ID: "otro"}}})
		p.Update(msgCatalogo{origen: o, lista: nil})
		p.Update(msgEsquema{origen: o, texto: "", error: "viejo"})
		p.Update(msgGuardado{origen: o, p: punto("vieja")})
		p.Update(msgVivo{Origen: o, Clase: "evento", Tipo: "texto", Datos: "x"})
	}
	if p.trabajo != nil || len(p.recientes.Visibles()) != 0 || len(p.catalogo) == 0 || p.errEsquema != "" || p.api != "" || p.salida != "" {
		t.Fatalf("una respuesta vieja se aplicó: %+v", p)
	}
}

// TestOcultarCortaElVivo: al dejar la página se suelta el seguimiento y al volver se reabre.
func TestOcultarCortaElVivo(t *testing.T) {
	cli, id, b := conBloqueado(t)
	p := abierta(t, cli, id)
	p.form.Poner("mensaje", "esperar")
	pulsar(p, "ctrl+s")
	<-b.Entro
	hasta(t, p, func() bool { return p.salida != "" })
	canal := p.canal
	p.Ocultar()
	if p.cancelar != nil {
		t.Fatal("se suelta el seguimiento")
	}
	for range canal { // el canal viejo se cierra
	}
	correr(p, p.Mostrar())
	if p.cancelar == nil || p.canal == nil {
		t.Fatal("al volver, se reabre")
	}
	b.SoltarFin()
	hasta(t, p, func() bool { return p.final != nil })
}

// TestCabeEn60x20: a 60×17 y a 60×20, con la salida estructurada activa en los dos modos (campos y JSON) y una plantilla
// puesta, cada campo con el foco se ve: la línea del «▸» es la de su etiqueta y debajo caben su valor o las líneas de su
// área. La respuesta, las recientes, el código y «Guardar como API» enseñan lo suyo.
func TestCabeEn60x20(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	vista := func(que string) string {
		t.Helper()
		v := p.Vista()
		if lipgloss.Height(v) > p.c.Alto || lipgloss.Width(v) > p.c.Ancho {
			t.Fatalf("%s mide %dx%d en %dx%d:\n%s", que, lipgloss.Width(v), lipgloss.Height(v), p.c.Ancho, p.c.Alto, v)
		}
		return v
	}
	// enfocado pone el foco en el campo y comprueba que su etiqueta está en la línea del «▸» y que sus lineas de valor caben
	// antes de la pista; si se da, que entre ellas está fin.
	enfocado := func(id, etiqueta string, lineas int, fin string) {
		t.Helper()
		p.form.Enfocar(id)
		if p.form.Foco() != id {
			t.Fatalf("el campo %s existe y tiene el foco: %q", id, p.form.Foco())
		}
		l := strings.Split(vista(id), "\n")
		i := slices.IndexFunc(l, func(s string) bool { return strings.Contains(s, "▸") })
		if i < 0 || !strings.Contains(l[i], etiqueta) {
			t.Fatalf("el «▸» está en la etiqueta de %s (%q):\n%s", id, etiqueta, strings.Join(l, "\n"))
		}
		ultima := len(l) - len(p.envolver(p.pista(), 3)) - 2 // la última línea del formulario: luego van la línea vacía y la pista
		if i+lineas > ultima {
			t.Fatalf("el valor de %s no cabe (etiqueta en %d, %d líneas, el formulario acaba en %d):\n%s", id, i, lineas, ultima, strings.Join(l, "\n"))
		}
		if fin != "" && !slices.ContainsFunc(l[i+1:i+1+lineas], func(s string) bool { return strings.Contains(s, fin) }) {
			t.Fatalf("el área de %s enseña %q:\n%s", id, fin, strings.Join(l, "\n"))
		}
	}
	comunes := func() {
		t.Helper()
		enfocado("instrucciones", p.tx("receta.instrucciones"), 6, "")
		enfocado("modelo", p.tx("receta.modelo"), 1, "")
		enfocado("esfuerzo", p.tx("esfuerzo.titulo"), 1, "")
		enfocado("respaldo", p.tx("receta.respaldo"), 1, "")
		enfocado("estructurada", p.tx("receta.salidaEstructurada"), 1, "[x]")
		enfocado("modo", p.tx("receta.modo"), 1, "")
		enfocado("plantilla", p.tx("receta.plantillaCorta"), 1, "")
		enfocado("reintentos", p.tx("receta.reintentos"), 1, "")
		enfocado("mensaje", p.tx("ejecutar.mensaje"), 6, "")
		enfocado("archivos", p.tx("ejecutar.adjuntar"), 1, "")
		enfocado("perfil", p.tx("ejecutar.perfilTrabajo"), 1, "")
		enfocado("tiempo", p.tx("ejecutar.tiempo"), 1, "")
	}
	for _, alto := range []int{17, 20} {
		p.Update(tea.WindowSizeMsg{Width: 60, Height: alto})
		if !p.form.Marcado("estructurada") {
			conEstructurada(p)
			p.form.Enfocar("plantilla")
			p.editando = true
			pulsar(p, "enter", "esc") // «Extraer datos», sobre el esquema de ejemplo: no pregunta
		}
		elegir(t, p, "modo", modoCampos)
		if p.form.Valor("c0.nombre") != "nombre" || len(p.campos) != 3 {
			t.Fatalf("la plantilla está en campos: %+v", p.campos)
		}
		comunes()
		enfocado("c0.nombre", "1 · "+p.tx("campos.nombre"), 1, "nombre")
		enfocado("c0.tipo", "1 · "+p.tx("campos.tipo"), 1, p.tx("campos.tipo.texto"))
		enfocado("c2.descripcion", "3 · "+p.tx("campos.descripcion"), 1, "")
		enfocado("c2.maximo", "3 · "+p.tx("campos.maximo"), 1, "")
		elegir(t, p, "modo", modoJSON)
		comunes()
		if n := strings.Count(p.form.Valor("esquema"), "\n"); n < 8 {
			t.Fatalf("el esquema de la plantilla ocupa más que el área: %d líneas", n)
		}
		enfocado("esquema", p.tx("receta.esquema"), 6, `"type": "object"`)
	}
	p.form.Poner("mensaje", strings.Repeat("una línea larga de mensaje ", 40))
	pulsar(p, "ctrl+s")
	hasta(t, p, func() bool { return p.final != nil })
	if v := vista("respuesta"); !strings.Contains(v, p.nombreEstado(p.final.Estado)) || !strings.Contains(v, p.tx("ruta.cola")) {
		t.Fatalf("la respuesta enseña el estado y la ruta:\n%s", v)
	}
	pulsar(p, "right")
	if v := vista("recientes"); !strings.Contains(v, "una línea larga") {
		t.Fatalf("las recientes enseñan la prueba:\n%s", v)
	}
	pulsar(p, "k")
	if v := vista("código"); !strings.Contains(v, "[Python") || !strings.Contains(v, "import") {
		t.Fatalf("«Ver como código» enseña Python:\n%s", v)
	}
	pulsar(p, "esc", "g")
	if v := vista("guardar"); !strings.Contains(v, "▸ "+p.tx("puntos.nombre")) || !strings.Contains(v, "Por ejemplo: contabilidad") {
		t.Fatalf("«Guardar como API» enseña el nombre y su pista:\n%s", v)
	}
	pulsar(p, "esc")
}

// TestTextosComoLaWeb: los nombres que se ven son los de la Web UI, también en inglés.
func TestTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	if v := p.Vista(); !strings.Contains(v, "[Receta]") || !strings.Contains(v, "Respuesta") {
		t.Fatalf("pestañas en español:\n%s", v)
	}
	en := Nueva("en").(*Pagina)
	c := contexto(cli, id, "")
	c.Idioma = "en"
	correr(en, en.Iniciar(c))
	t.Cleanup(en.Ocultar)
	if en.Titulo() != "Try" {
		t.Fatalf("el título es el del menú en inglés: %q", en.Titulo())
	}
	if v := en.Vista(); !strings.Contains(v, "Recipe") || !strings.Contains(v, "Answer") {
		t.Fatalf("en inglés:\n%s", v)
	}
}
