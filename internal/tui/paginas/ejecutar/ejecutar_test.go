package ejecutar_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/ejecutar"
	"tramtai/internal/tui/paginas/prueba"
)

// archivoBorrador es donde Probar guarda el borrador del mensaje, en la carpeta de datos.
const archivoBorrador = "terminal-borrador.json"

// TestEnviarYCancelar: Ctrl+S manda la receta como la Web UI y la respuesta llega en vivo; Esc vuelve sin cancelarla y c
// la cancela tras confirmar.
func TestEnviarYCancelar(t *testing.T) {
	cli, id, b := conBloqueado(t)
	a := abierta(t, cli, id)
	escribir(t, a, tx("receta.instrucciones"), "breve")
	abrirPliegue(t, a, "receta.opciones")
	escribir(t, a, tx("receta.respaldo"), "simulado")
	abrirPliegue(t, a, "comun.opcionesAvanzadas")
	escribir(t, a, tx("ejecutar.tiempo"), "60")
	conEstructurada(t, a)
	elegir(t, a, tx("receta.reintentos"), tx("receta.reintentosN", "n", "2"))
	escribir(t, a, tx("ejecutar.mensaje"), "esperar")
	if a.P.EnCampo() {
		t.Fatal("Esc deja el campo")
	}
	fuera := enviar(a)
	select {
	case <-b.Entro:
	case <-time.After(5 * time.Second):
		t.Fatalf("el agente no recibió la petición: %v", fuera)
	}
	if !pestana(a, "ejecutar.respuesta") {
		t.Fatalf("enviar enseña la respuesta:\n%s", a.Vista())
	}
	a.HastaVer("primera parte", "en vivo")
	s := b.Ultima()
	if s.Instrucciones != "breve" || len(s.Respaldo) != 1 || s.Ejecucion.TiempoLimiteSegundos != 60 ||
		s.Salida.Formato != "json" || s.Salida.Reintentos != 2 || !s.Salida.ConservarSalidaInvalida || !strings.Contains(string(s.Salida.Esquema), "resumen") {
		t.Fatalf("la solicitud lleva la receta como la Web UI: %+v", s)
	}
	if tr := ultimo(t, cli, id); tr.Origen != nucleo.DesdeTerminal {
		t.Fatalf("el historial dice que llegó de la terminal: %q", tr.Origen)
	}
	if fuera := enviar(a); len(fuera) != 0 || cuantos(t, cli, id) != 1 {
		t.Fatalf("con una en marcha no se envía otra: %v", fuera)
	}
	if !strings.Contains(pendiente(a), "petición en marcha") {
		t.Fatalf("salir de la terminal avisa de la petición en marcha: %q", pendiente(a))
	}
	// Esc vuelve y la petición sigue; c la cancela, después de confirmar.
	if fuera := a.Pulsar("esc"); len(fuera) != 1 || fuera[0] != (pagina.Volver{}) {
		t.Fatalf("en marcha, Esc vuelve: %v", fuera)
	}
	if tr := ultimo(t, cli, id); tr.Estado.Terminal() {
		t.Fatalf("Esc no la cancela: %s", tr.Estado)
	}
	if _, ok := pregunta(a.Pulsar("c")); !ok {
		t.Fatal("c pregunta antes de cancelar")
	}
	a.Entregar(comp.Confirmado{})
	if e := terminada(t, a); e != nucleo.Cancelado || pendiente(a) != "" {
		t.Fatalf("c la cancela y ya no hay nada en marcha: %s %q", e, pendiente(a))
	}
	if fuera := a.Pulsar("esc"); len(fuera) != 1 || fuera[0] != (pagina.Volver{}) {
		t.Fatalf("terminada, Esc vuelve: %v", fuera)
	}
	if n := cuantos(t, cli, id); n != 1 {
		t.Fatalf("una sola petición: %d", n)
	}
	a.Pulsar("right")
	if n := strings.Count(a.Vista(), "esperar"); !pestana(a, "ejecutar.recientes") || n != 1 {
		t.Fatalf("y queda en las recientes (%d):\n%s", n, a.Vista())
	}
}

// TestCorregirYCopiar: una respuesta que no cumple el esquema se enseña con su error; o la corrige y y la copia.
func TestCorregirYCopiar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	conEstructurada(t, a)
	aJSON(t, a)
	poner(t, a, tx("receta.esquema"), `{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}`)
	poner(t, a, tx("ejecutar.mensaje"), `{"total":"diez"}`)
	enviar(a)
	terminada(t, a)
	if tr := ultimo(t, cli, id); tr.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || !strings.Contains(a.Vista(), `{"total":"diez"}`) {
		t.Fatalf("enseña la respuesta que no cumple: %+v\n%s", tr, a.Vista())
	}
	if v := a.Vista(); !strings.Contains(v, "Corregir") {
		t.Fatalf("ofrece corregir:\n%s", v)
	}
	fuera := a.Pulsar("y")
	if c, _ := copiado(fuera); c != `{"total":"diez"}` {
		t.Fatalf("y copia la respuesta con OSC 52: %v", fuera)
	}
	if a, ok := aviso(fuera); !ok || !strings.Contains(a.Texto, "OSC 52") {
		t.Fatalf("y avisa de que depende de la terminal: %v", fuera)
	}
	viejo := ultimo(t, cli, id).ID
	a.Pulsar("o")
	a.Hasta(func() bool { return cuantos(t, cli, id) == 2 })
	terminada(t, a)
	nuevo := ultimo(t, cli, id)
	if m := nuevo.Solicitud.Mensajes; nuevo.ID == viejo || !strings.HasPrefix(m[len(m)-1].Contenido, "Tu respuesta no cumple el formato pedido:") {
		t.Fatal("o crea otra petición, una corrección y no un reintento, y la sigue")
	}
	// Una completada no se corrige.
	a.Pulsar("left")
	poner(t, a, tx("ejecutar.mensaje"), `{"total":10}`)
	enviar(a)
	if e := terminada(t, a); e != nucleo.Completado {
		t.Fatalf("se completa: %s", e)
	}
	if a, ok := aviso(a.Pulsar("o")); !ok || a.Texto != "Esta petición no se puede corregir: vuelve a enviarla" {
		t.Fatalf("avisa con el texto de la Web UI: %+v", a)
	}
	// n deja la página lista para otra prueba con la misma receta.
	a.Pulsar("n")
	if !pestana(a, "ejecutar.receta") || strings.Contains(valor(t, a, tx("ejecutar.mensaje")), "total") || !marcado(t, a, tx("receta.salidaEstructurada")) {
		t.Fatalf("Nueva prueba vacía el mensaje y conserva la receta:\n%s", a.Vista())
	}
	if a.Pulsar("n"); !strings.Contains(valor(t, a, tx("receta.esquema")), "total") {
		t.Fatal("sin respuesta, n no hace nada")
	}
}

// TestRecetaEmpiezaArriba: Probar abre la receta arriba, con lo esencial (instrucciones, modelo y esfuerzo) y las opciones
// plegadas como en la Web UI; la posición del foco se ve. Intro abre el pliegue y una receta con opciones lo abre sola.
func TestRecetaEmpiezaArriba(t *testing.T) {
	cli, id := prueba.Servicio(t)
	if _, _, err := cli.CrearPunto(ctx, id, contratoPunto()); err != nil {
		t.Fatal(err)
	}
	c := prueba.Contexto(t, cli, id)
	c.Datos = t.TempDir()
	a := abrir(t, c)
	v := a.Vista()
	if l := lineas(a); !strings.Contains(l[1], "Instrucciones") {
		t.Fatalf("la receta empieza arriba:\n%s", v)
	}
	for _, quiere := range []string{"Opciones de la receta", "↑↓ 1/"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if strings.Contains(v, "Respaldo") || strings.Contains(v, "Respuesta con forma") {
		t.Fatalf("lo avanzado va plegado:\n%s", v)
	}
	a.Pulsar("down", "down", "down", "enter")
	if v := a.Vista(); !strings.Contains(v, "Respaldo") || a.P.EnCampo() {
		t.Fatalf("Intro abre las opciones sin entrar a editar:\n%s", v)
	}
	a.Pulsar("enter")
	if strings.Contains(a.Vista(), "Respaldo") {
		t.Fatalf("Intro las vuelve a plegar:\n%s", a.Vista())
	}
	a.Entregar(pagina.ConPunto{Nombre: "notas"})
	if enfocar(t, a, tx("receta.opciones")); !strings.Contains(a.Vista(), "▾ Opciones de la receta") {
		t.Fatalf("una receta con salida estructurada abre las opciones:\n%s", a.Vista())
	}
}

// TestBorradorSobreviveReinicio: el mensaje se guarda al escribir y vuelve al abrir la terminal otra vez; se borra al enviar.
func TestBorradorSobreviveReinicio(t *testing.T) {
	cli, id := prueba.Servicio(t)
	carpeta := t.TempDir()
	a := abrir(t, contexto(t, cli, id, carpeta))
	escribir(t, a, tx("ejecutar.mensaje"), "a medias")
	a.Hasta(func() bool {
		datos, _ := os.ReadFile(filepath.Join(carpeta, archivoBorrador))
		return strings.Contains(string(datos), "a medias")
	})
	ocultar(a)
	otra := abrir(t, contexto(t, cli, id, carpeta))
	if v := valor(t, otra, tx("ejecutar.mensaje")); v != "a medias" {
		t.Fatalf("al volver a abrir, el mensaje es el del borrador: %q", v)
	}
	enviar(otra)
	otra.Hasta(func() bool {
		_, err := os.Stat(filepath.Join(carpeta, archivoBorrador))
		return os.IsNotExist(err)
	})
	terminada(t, otra)
}

// TestSeguirTrabajo: lo que manda el Historial al reintentar o corregir abre ese trabajo con su receta y lo sigue en vivo.
func TestSeguirTrabajo(t *testing.T) {
	cli, id, b := conBloqueado(t)
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "simulado", Instrucciones: "corto",
		Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "esperar"}}})
	if err != nil {
		t.Fatal(err)
	}
	<-b.Entro
	a := abierta(t, cli, id)
	a.Entregar(pagina.SeguirTrabajo{ID: tr.ID})
	if !pestana(a, "ejecutar.respuesta") {
		t.Fatalf("abre el trabajo en la respuesta:\n%s", a.Vista())
	}
	a.HastaVer("primera parte")
	a.Pulsar("left")
	if valor(t, a, tx("ejecutar.mensaje")) != "esperar" || valor(t, a, tx("receta.instrucciones")) != "corto" || valor(t, a, tx("receta.modelo")) != "simulado" {
		t.Fatalf("con su receta y su mensaje:\n%s", a.Vista())
	}
	a.Pulsar("right")
	b.SoltarFin()
	if e := terminada(t, a); e != nucleo.Completado {
		t.Fatalf("termina en vivo: %s", e)
	}
}

// TestUsarDeNuevo: las recientes son las de la terminal; u pone su mensaje y su receta; Intro la abre; h va al Historial.
func TestUsarDeNuevo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	otro := *cli
	otro.Origen = nucleo.DesdeCLI
	if _, err := otro.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "de la CLI"}}}); err != nil {
		t.Fatal(err)
	}
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "simulado", Respaldo: []string{"simulado"},
		Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "otra vez"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Eventos(ctx, id, tr.ID, 0, func(int, string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	a.Pulsar("right", "right")
	if v := a.Vista(); strings.Count(v, "otra vez") != 1 || strings.Contains(v, "de la CLI") {
		t.Fatalf("solo las de la terminal:\n%s", v)
	}
	a.Pulsar("u")
	if !pestana(a, "ejecutar.receta") || valor(t, a, tx("ejecutar.mensaje")) != "otra vez" || valor(t, a, tx("receta.respaldo")) != "simulado" {
		t.Fatalf("u trae mensaje y receta:\n%s", a.Vista())
	}
	a.Pulsar("left", "enter")
	if !pestana(a, "ejecutar.respuesta") || !strings.Contains(a.Vista(), "otra vez") || terminada(t, a) != nucleo.Completado {
		t.Fatalf("Intro la abre en la respuesta:\n%s", a.Vista())
	}
	fuera := a.Pulsar("h")
	if len(fuera) != 1 || fuera[0] != (pagina.IrA{Pagina: pagina.PaginaHistorial, Datos: pagina.FiltroPunto{Nombre: "o:terminal"}}) {
		t.Fatalf("h va al Historial con las de la terminal: %v", fuera)
	}
}

// TestEscVuelve: fuera de un campo Esc vuelve; dentro, solo deja el campo.
func TestEscVuelve(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	if a.P.EnCampo() {
		t.Fatal("al llegar no se está dentro de un campo: ?, q y 1…7 son de la app")
	}
	if fuera := a.Pulsar("esc"); len(fuera) != 1 || fuera[0] != (pagina.Volver{}) {
		t.Fatalf("Esc fuera de campo vuelve: %v", fuera)
	}
	a.Pulsar("enter")
	if !a.P.EnCampo() {
		t.Fatal("Intro edita el campo")
	}
	if fuera := a.Pulsar("esc"); len(fuera) != 0 || a.P.EnCampo() {
		t.Fatalf("Esc en un campo solo lo deja: %v", fuera)
	}
}

func TestServicioCaido(t *testing.T) {
	a := abrir(t, contexto(t, nil, "global", ""))
	if fuera := a.Fuera; len(fuera) != 1 || !errorServicio(fuera) {
		t.Fatalf("sin cliente avisa a la app con ErrorServicio: %v", fuera)
	}
	if v := a.Vista(); !strings.Contains(v, "Instrucciones") {
		t.Fatalf("el formulario se ve igual:\n%s", v)
	}
	cli, id, cerrar := prueba.Arrancar(t)
	b := abierta(t, cli, id)
	cerrar()
	poner(t, b, tx("ejecutar.mensaje"), "hola")
	if fuera := enviar(b); len(fuera) != 1 || !errorServicio(fuera) {
		t.Fatalf("con el servicio caído tras cargar, avisa a la app: %v", fuera)
	}
	if fuera := enviar(b); !errorServicio(fuera) {
		t.Fatalf("y no se queda enviando: %v", fuera)
	}
}

// TestRespuestasViejas: lo que llega de otra versión de la página, de otro espacio o con el contexto cancelado no se aplica.
// Las respuestas viejas son de verdad: la página las pidió y llegan tarde.
func TestRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "vieja"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := cli.Eventos(ctx, id, tr.ID, 0, func(int, string, string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	// pedir deja la página con respuestas en camino: las recientes, el trabajo que se sigue, el esquema y la API nueva.
	pedir := func(t *testing.T, a *prueba.App) []tea.Cmd {
		conEstructurada(t, a)
		aJSON(t, a)
		editar(t, a, tx("receta.esquema"))
		vaciar(a)
		a.Pegar(`{"type":"object","properties":{"x":{"type":"nada"}}}`)
		viejos := a.Retener(func() { a.Entregar(comp.Cancelado{}) }) // dejar el campo: el servicio comprueba el esquema
		viejos = append(viejos, a.Retener(func() { a.Pulsar("r") })...)
		viejos = append(viejos, a.Retener(func() { a.Entregar(pagina.SeguirTrabajo{ID: tr.ID}) })...)
		a.Pulsar("g")
		a.Escribir("vieja")
		viejos = append(viejos, a.Retener(func() { a.Entregar(comp.Enviado{}) })...)
		a.Pulsar("esc")
		return viejos
	}
	// sinAplicar comprueba que nada de lo viejo se ve: ni avisos, ni el trabajo, ni la API.
	sinAplicar := func(t *testing.T, a *prueba.App, fuera []tea.Msg) {
		t.Helper()
		if len(fuera) != 0 {
			t.Fatalf("una respuesta vieja avisó: %v", fuera)
		}
		if v := a.Vista(); pestana(a, "ejecutar.respuesta") || strings.Contains(v, "Editando la API") {
			t.Fatalf("una respuesta vieja se aplicó:\n%s", v)
		}
	}
	t.Run("otra versión", func(t *testing.T) {
		a := abierta(t, cli, id)
		viejos := pedir(t, a)
		ocultar(a)
		mostrar(a)
		antes := a.Vista()
		sinAplicar(t, a, a.Soltar(viejos))
		if a.Vista() != antes {
			t.Fatalf("la vista cambió:\n%s\n%s", antes, a.Vista())
		}
	})
	t.Run("otro espacio", func(t *testing.T) {
		a := abierta(t, cli, id)
		viejos := pedir(t, a)
		a.Correr(a.P.Iniciar(contexto(t, cli, "otro", t.TempDir())))
		a.Pulsar("right", "right")
		antes := a.Vista()
		a.Soltar(viejos)
		if a.Vista() != antes || strings.Contains(a.Vista(), "vieja") {
			t.Fatalf("lo del otro espacio no se aplica:\n%s\n%s", antes, a.Vista())
		}
	})
	t.Run("contexto cancelado", func(t *testing.T) {
		c := contexto(t, cli, id, t.TempDir())
		vivo, cancelar := context.WithCancel(c.Ctx)
		c.Ctx = vivo
		a := abrir(t, c)
		viejos := pedir(t, a)
		cancelar()
		antes := a.Vista()
		sinAplicar(t, a, a.Soltar(viejos))
		if a.Vista() != antes {
			t.Fatalf("la vista cambió:\n%s\n%s", antes, a.Vista())
		}
	})
}

// TestOcultarCortaElVivo: al dejar la página se suelta el seguimiento y al volver se reabre.
func TestOcultarCortaElVivo(t *testing.T) {
	cli, id, b := conBloqueado(t)
	a := abierta(t, cli, id)
	poner(t, a, tx("ejecutar.mensaje"), "esperar")
	enviar(a)
	<-b.Entro
	a.HastaVer("primera parte", "en vivo")
	ocultar(a)
	if strings.Contains(a.Vista(), "en vivo") {
		t.Fatalf("se suelta el seguimiento:\n%s", a.Vista())
	}
	mostrar(a)
	a.HastaVer("en vivo")
	b.SoltarFin()
	if e := terminada(t, a); e != nucleo.Completado {
		t.Fatalf("y llega al final: %s", e)
	}
}

// TestCabeEn60x20: a 60×17 y a 60×20, con la salida estructurada activa en los dos modos (campos y JSON) y una plantilla
// puesta, cada campo con el foco se ve: la línea del «▸» es la de su etiqueta y debajo caben su valor o las líneas de su
// área. La respuesta, las recientes, el código y «Guardar como API» enseñan lo suyo.
func TestCabeEn60x20(t *testing.T) {
	cli, id := prueba.Servicio(t)
	c := prueba.Contexto(t, cli, id)
	c.Datos = t.TempDir()
	a := abrir(t, c)
	ancho, alto := 60, 17
	vista := func(que string) string {
		t.Helper()
		v := a.Vista()
		if lipgloss.Height(v) > alto || lipgloss.Width(v) > ancho {
			t.Fatalf("%s mide %dx%d en %dx%d:\n%s", que, lipgloss.Width(v), lipgloss.Height(v), ancho, alto, v)
		}
		return v
	}
	// enfocado pone el foco en el campo y comprueba que su etiqueta está en la línea del «▸» y que sus lineas de valor caben
	// antes de la posición («↑↓ n/total»), que va tras el formulario; si se da, que entre ellas está fin.
	enfocado := func(etiqueta string, lineas int, fin string) {
		t.Helper()
		enfocar(t, a, etiqueta)
		l := strings.Split(vista(etiqueta), "\n")
		i := slices.IndexFunc(l, func(s string) bool { return strings.HasPrefix(s, "▸ ") })
		if i < 0 || !strings.Contains(l[i], etiqueta) {
			t.Fatalf("el «▸» está en la etiqueta de %s:\n%s", etiqueta, strings.Join(l, "\n"))
		}
		ultima := slices.IndexFunc(l, func(s string) bool { return strings.HasPrefix(s, "↑↓ ") }) - 1
		if i+lineas > ultima {
			t.Fatalf("el valor de %s no cabe (etiqueta en %d, %d líneas, el formulario acaba en %d):\n%s", etiqueta, i, lineas, ultima, strings.Join(l, "\n"))
		}
		if fin != "" && !slices.ContainsFunc(l[i+1:i+1+lineas], func(s string) bool { return strings.Contains(s, fin) }) {
			t.Fatalf("el área de %s enseña %q:\n%s", etiqueta, fin, strings.Join(l, "\n"))
		}
	}
	comunes := func() {
		t.Helper()
		enfocado(tx("receta.instrucciones"), 6, "")
		enfocado(tx("receta.modelo"), 1, "")
		enfocado(tx("esfuerzo.titulo"), 1, "")
		enfocado(tx("receta.respaldo"), 1, "")
		enfocado(tx("receta.salidaEstructurada"), 1, "[x]")
		enfocado(tx("receta.modo"), 1, "")
		enfocado(tx("receta.plantillaCorta"), 1, "")
		enfocado(tx("receta.reintentos"), 1, "")
		enfocado(tx("ejecutar.mensaje"), 6, "")
		enfocado(tx("ejecutar.adjuntar"), 1, "")
		enfocado(tx("ejecutar.perfilTrabajo"), 1, "")
		enfocado(tx("ejecutar.tiempo"), 1, "")
	}
	abrirPliegue(t, a, "comun.opcionesAvanzadas")
	conEstructurada(t, a)
	editar(t, a, tx("receta.plantillaCorta"))
	a.Pulsar("enter", "esc") // «Extraer datos», sobre el esquema de ejemplo: no pregunta
	a.Entregar(tea.WindowSizeMsg{Width: ancho, Height: 400})
	if v := a.Vista(); valor(t, a, "1 · "+tx("campos.nombre")) != "nombre" || !strings.Contains(v, "3 · "+tx("campos.nombre")) || strings.Contains(v, "4 · ") {
		t.Fatalf("la plantilla está en campos, con sus tres campos:\n%s", v)
	}
	for _, h := range []int{17, 20} {
		alto = h
		a.Entregar(tea.WindowSizeMsg{Width: ancho, Height: alto})
		elegir(t, a, tx("receta.modo"), tx("receta.modoCampos"))
		comunes()
		enfocado("1 · "+tx("campos.nombre"), 1, "nombre")
		enfocado("1 · "+tx("campos.tipo"), 1, tx("campos.tipo.texto"))
		enfocado("3 · "+tx("campos.descripcion"), 1, "")
		enfocado("3 · "+tx("campos.maximo"), 1, "")
		aJSON(t, a)
		comunes()
		enfocado(tx("receta.esquema"), 6, `"type": "object"`)
	}
	// El esquema de «Extraer datos» ocupa más que su área de 6 líneas: lo de arriba se ve y el resto se desplaza.
	var sangrado bytes.Buffer
	if json.Indent(&sangrado, datos.Plantillas("es")[0].Esquema, "", "  "); strings.Count(sangrado.String(), "\n") < 8 {
		t.Fatalf("el esquema de la plantilla ocupa más que el área:\n%s", sangrado.String())
	}
	poner(t, a, tx("ejecutar.mensaje"), strings.Repeat("una línea larga de mensaje ", 40))
	enviar(a)
	e := terminada(t, a)
	if v := vista("respuesta"); !strings.Contains(v, estado(e)) || !strings.Contains(v, tx("ruta.cola")) {
		t.Fatalf("la respuesta enseña el estado y la ruta:\n%s", v)
	}
	a.Pulsar("right")
	if v := vista("recientes"); !strings.Contains(v, "una línea larga") {
		t.Fatalf("las recientes enseñan la prueba:\n%s", v)
	}
	a.Pulsar("k")
	if v := vista("código"); !strings.Contains(v, "[Python") || !strings.Contains(v, "import") {
		t.Fatalf("«Ver como código» enseña Python:\n%s", v)
	}
	a.Pulsar("esc", "g")
	if v := vista("guardar"); !strings.Contains(v, "▸ "+tx("puntos.nombre")) || !strings.Contains(v, "Por ejemplo: contabilidad") {
		t.Fatalf("«Guardar como API» enseña el nombre y su pista:\n%s", v)
	}
	a.Pulsar("esc")
}

// TestTextosComoLaWeb: los nombres que se ven son los de la Web UI, también en inglés.
func TestTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	if v := a.Vista(); !strings.Contains(v, "[Receta]") || !strings.Contains(v, "Respuesta") {
		t.Fatalf("pestañas en español:\n%s", v)
	}
	c := contexto(t, cli, id, "")
	c.Idioma = "en"
	en := prueba.Abrir(t, ejecutar.Nueva("en"), c)
	if en.P.Titulo() != "Playground" {
		t.Fatalf("el título es el del menú en inglés: %q", en.P.Titulo())
	}
	if v := en.Vista(); !strings.Contains(v, "Recipe") || !strings.Contains(v, "Response") {
		t.Fatalf("en inglés:\n%s", v)
	}
}
