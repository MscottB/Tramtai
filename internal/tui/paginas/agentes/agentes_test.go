package agentes

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

var tecla = prueba.Tecla

// correr ejecuta un Cmd y alimenta a la página con lo que sale, como haría la app. Devuelve los mensajes que son de la
// app: preguntas, avisos, IrA, Volver y ErrorServicio.
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
		default:
			_, siguiente := p.Update(m)
			cola = append(cola, siguiente)
		}
	}
	return fuera
}

func contexto(cli *cliente.Cliente, instancia string) pagina.Contexto {
	return pagina.Contexto{Ctx: context.Background(), Cli: cli, Instancia: instancia, Idioma: "es",
		Estilos: &pagina.Estilos{Iconos: pagina.IconosUnicode}, Ancho: 60, Alto: 17}
}

// abierta inicia la página contra el servicio real y espera la lista.
func abierta(t *testing.T) (*Pagina, *cliente.Cliente, string) {
	t.Helper()
	cli, id := prueba.Servicio(t)
	p := Nueva("es").(*Pagina)
	cmd := p.Iniciar(contexto(cli, id))
	if v := p.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	correr(p, cmd)
	return p, cli, id
}

var aviso = prueba.Aviso

func TestAgentesCarga(t *testing.T) {
	p, _, _ := abierta(t)
	v := p.Vista()
	for _, quiere := range []string{"Agentes", "simulado", "Listo", "Listos (1)", "Falta conectar (0)"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if p.Titulo() != "Agentes" || p.Ayuda() != "agentes" || p.EnCampo() {
		t.Fatalf("título %q, ayuda %q", p.Titulo(), p.Ayuda())
	}
}

func TestAgentesDesconectar(t *testing.T) {
	p, cli, id := abierta(t)
	_, cmd := p.Update(tecla("d"))
	fuera := correr(p, cmd)
	if len(fuera) != 1 {
		t.Fatalf("d pregunta antes de desconectar: %v", fuera)
	}
	if q, ok := fuera[0].(comp.Pregunta); !ok || !strings.Contains(q.Texto, "¿Desconectar simulado?") {
		t.Fatalf("la pregunta nombra al agente: %v", fuera[0])
	}
	// Cancelar no toca nada.
	p.Update(comp.Cancelado{})
	if r, _ := cli.Capacidades(context.Background(), id); r["adaptadores"].([]any)[0].(map[string]any)["desconectado"] == true {
		t.Fatal("cancelar no desconecta")
	}
	// Confirmar desconecta en el servicio y la página lo ve al volver a comprobar.
	_, cmd = p.Update(tecla("d"))
	correr(p, cmd)
	_, cmd = p.Update(comp.Confirmado{})
	fuera = correr(p, cmd)
	if a, ok := aviso(fuera); !ok || a.Texto != "simulado está desconectado" || a.Tipo != comp.Exito {
		t.Fatalf("avisa: %+v", fuera)
	}
	r, _ := cli.Capacidades(context.Background(), id)
	if r["adaptadores"].([]any)[0].(map[string]any)["desconectado"] != true {
		t.Fatal("el servicio lo tiene desconectado")
	}
	if v := p.Vista(); !strings.Contains(v, "Desconectado") || strings.Contains(v, "Listo ") {
		t.Fatalf("la lista dice Desconectado:\n%s", v)
	}
	// Conectar no pide confirmación.
	_, cmd = p.Update(tecla("d"))
	fuera = correr(p, cmd)
	if a, ok := aviso(fuera); !ok || a.Texto != "simulado está conectado" {
		t.Fatalf("conecta sin preguntar: %+v", fuera)
	}
	if v := p.Vista(); !strings.Contains(v, "Listo") {
		t.Fatalf("vuelve a Listo:\n%s", v)
	}
}

func TestAgentesAutenticacionSoloSiAdmite(t *testing.T) {
	p, _, _ := abierta(t)
	// El simulado no admite la suscripción: a avisa y no pregunta ni cambia nada.
	_, cmd := p.Update(tecla("a"))
	fuera := correr(p, cmd)
	if a, ok := aviso(fuera); !ok || !strings.Contains(a.Texto, "no admite la suscripción") || len(fuera) != 1 {
		t.Fatalf("avisa que no admite: %+v", fuera)
	}
	if p.pend != nil || p.ocupado {
		t.Fatal("no queda nada pendiente")
	}

	// Claude sí la admite: con clave de API y listo, pasar a la suscripción pide aceptar el riesgo.
	claude := adaptador{Agente: "claude", Autenticacion: "clave_api"}
	p.agentes = []adaptador{claude}
	p.lista.Poner(p.agentes)
	_, cmd = p.Update(tecla("a"))
	fuera = correr(p, cmd)
	q, ok := fuera[0].(comp.Pregunta)
	if len(fuera) != 1 || !ok || !strings.Contains(q.Texto, "Ese riesgo es tuyo") || !strings.Contains(q.Texto, "Usar tu suscripción con Claude") {
		t.Fatalf("pregunta con el riesgo: %+v", fuera)
	}
	if p.pend == nil || p.pend.accion != accionSuscripcion {
		t.Fatal("recuerda qué iba a hacer")
	}

	// Con suscripción, pasar a la clave es directo, sin pregunta.
	p.pend = nil
	p.agentes = []adaptador{{Agente: "claude", Autenticacion: "suscripcion"}}
	p.lista.Poner(p.agentes)
	_, cmd = p.Update(tecla("a"))
	if !p.ocupado {
		t.Fatal("pasar a la clave sale directo")
	}
	fuera = correr(p, cmd) // el servicio de pruebas no tiene Claude: la respuesta es su error, tal cual
	if a, ok := aviso(fuera); !ok || a.Tipo != comp.Fallo || a.Texto == "" {
		t.Fatalf("el error del servicio se ve: %+v", fuera)
	}
	if p.ocupado {
		t.Fatal("la respuesta libera la página")
	}

	// Sin instalar o desconectado, no se cambia aunque lo admita.
	for _, a := range []adaptador{{Agente: "claude", Problema: "x", ProblemaCodigo: "no_instalado"}, {Agente: "claude", Desconectado: true}} {
		p.agentes = []adaptador{a}
		p.lista.Poner(p.agentes)
		_, cmd = p.Update(tecla("a"))
		fuera = correr(p, cmd)
		if _, ok := aviso(fuera); !ok || len(fuera) != 1 || p.ocupado || p.pend != nil {
			t.Fatalf("%+v no se cambia: %+v", a, fuera)
		}
	}
}

func TestAgentesProbarConEste(t *testing.T) {
	p, _, _ := abierta(t)
	_, cmd := p.Update(tecla("p"))
	fuera := correr(p, cmd)
	if len(fuera) != 1 || fuera[0] != (pagina.IrA{Pagina: "ejecutar", Datos: pagina.ConAgente{Agente: "simulado"}}) {
		t.Fatalf("p lleva a Probar con el agente: %+v", fuera)
	}
	// Uno que no está listo no se puede probar.
	p.agentes = []adaptador{{Agente: "codex", Problema: "x", ProblemaCodigo: "no_instalado"}}
	p.lista.Poner(p.agentes)
	_, cmd = p.Update(tecla("p"))
	if _, ok := correr(p, cmd)[0].(comp.NuevoAviso); !ok {
		t.Fatal("avisa que aún no está listo")
	}
}

func TestAgentesDetalles(t *testing.T) {
	p, _, _ := abierta(t)
	_, cmd := p.Update(tecla("enter"))
	if v := p.Vista(); !strings.Contains(v, "simulado") || !strings.Contains(v, "Versión") || !strings.Contains(v, "Cargando…") {
		t.Fatalf("detalle con el catálogo pendiente:\n%s", v)
	}
	correr(p, cmd)
	v := p.Vista()
	for _, quiere := range []string{"Listo · usa tu clave de API", "Modelos: predeterminado", "Respuesta en vivo: Al terminar", "Respuesta JSON con esquema: No"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	// Las acciones valen dentro del detalle: p lleva a Probar con este agente.
	_, cmd = p.Update(tecla("p"))
	if f := correr(p, cmd); len(f) != 1 || f[0] != (pagina.IrA{Pagina: "ejecutar", Datos: pagina.ConAgente{Agente: "simulado"}}) {
		t.Fatalf("p en el detalle: %+v", f)
	}
}

func TestAgentesNoInstaladoExplicaComoInstalarlo(t *testing.T) {
	p, _, _ := abierta(t)
	p.agentes = ordenar([]adaptador{{Agente: "codex", Problema: "no está instalado", ProblemaCodigo: "no_instalado"}, {Agente: "claude"}})
	p.lista.Poner(p.agentes)
	if p.agentes[0].Agente != "claude" || !strings.Contains(p.Vista(), "No instalado") {
		t.Fatalf("los no instalados van al final:\n%s", p.Vista())
	}
	p.lista.Update(tecla("j"))
	p.Update(tecla("enter"))
	v := p.Vista()
	if !strings.Contains(v, "comandos_agentes.codex") || !strings.Contains(v, "Cómo instalarlo: http") {
		t.Fatalf("el detalle dice cómo instalarlo:\n%s", v)
	}
}

func TestEscVuelve(t *testing.T) {
	p, _, _ := abierta(t)
	p.Update(tecla("enter"))
	if _, cmd := p.Update(tecla("esc")); cmd != nil || p.detalle != "" {
		t.Fatal("Esc cierra el detalle")
	}
	_, cmd := p.Update(tecla("esc"))
	if f := correr(p, cmd); len(f) != 1 || f[0] != (pagina.Volver{}) {
		t.Fatalf("sin nada abierto, Esc vuelve: %+v", f)
	}
}

func TestAgentesServicioCaido(t *testing.T) {
	p := Nueva("es").(*Pagina)
	if f := correr(p, p.Iniciar(contexto(nil, "global"))); len(f) != 1 {
		t.Fatalf("sin cliente: ErrorServicio: %+v", f)
	}
	cli, id, cerrar := prueba.Arrancar(t)
	p = Nueva("es").(*Pagina)
	correr(p, p.Iniciar(contexto(cli, id)))
	cerrar()
	_, cmd := p.Update(tecla("r"))
	f := correr(p, cmd)
	if len(f) != 1 {
		t.Fatalf("con el servicio cerrado: ErrorServicio, y la página no se cierra: %+v", f)
	}
	if e, ok := f[0].(pagina.ErrorServicio); !ok || !pagina.NoResponde(e.Err) {
		t.Fatalf("error de conexión: %+v", f[0])
	}
	if v := p.Vista(); !strings.Contains(v, "El servicio no responde · r reintenta") {
		t.Fatalf("la página lo dice:\n%s", v)
	}
}

func TestAgentesIgnoraRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	ctx, cancelar := context.WithCancel(context.Background())
	c := contexto(cli, id)
	c.Ctx = ctx
	p := Nueva("es").(*Pagina)
	cmd := p.Iniciar(c)
	cancelar() // la app reinició la página o cambió de espacio
	if f := correr(p, cmd); len(f) != 0 || p.cargado {
		t.Fatalf("una respuesta con el Ctx cancelado se descarta: %+v", f)
	}
	// Otra instancia: tampoco vale.
	p = Nueva("es").(*Pagina)
	cmd = p.Iniciar(contexto(cli, id))
	p.c.Instancia = "otro"
	if correr(p, cmd); p.cargado {
		t.Fatal("una respuesta de otro espacio se descarta")
	}
	// De dos comprobaciones seguidas solo vale la última.
	p = Nueva("es").(*Pagina)
	primera := p.Iniciar(contexto(cli, id))
	p.cargando = false
	segunda := p.comprobar()
	correr(p, primera)
	if p.cargado {
		t.Fatal("la primera ya es vieja")
	}
	correr(p, segunda)
	if !p.cargado {
		t.Fatal("la última sí")
	}
}

func TestAgentesEstados(t *testing.T) {
	pe := Nueva("es").(*Pagina)
	casos := []struct {
		a        adaptador
		quiere   estado
		etiqueta string
	}{
		{adaptador{Agente: "x"}, estListo, "Listo"},
		{adaptador{Agente: "x", Problema: "p", ProblemaCodigo: "aviso"}, estListo, "Listo"},
		{adaptador{Agente: "x", Desconectado: true}, estDesconectado, "Desconectado"},
		{adaptador{Agente: "x", Desconectado: true, Problema: "p", ProblemaCodigo: "no_instalado"}, estNoInstalado, "No instalado"},
		{adaptador{Agente: "x", Problema: "p", ProblemaCodigo: "no_instalado"}, estNoInstalado, "No instalado"},
		{adaptador{Agente: "x", Problema: "p", ProblemaCodigo: "falta_clave", Autenticacion: "clave_api"}, estFaltaClave, "Sin conectar"},
		{adaptador{Agente: "x", Problema: "p", ProblemaCodigo: "falta_clave", Autenticacion: "suscripcion"}, estSinSesion, "Sin conectar"},
		{adaptador{Agente: "x", Problema: "p", ProblemaCodigo: "usa_clave_api"}, estUsaClave, "Revisar"},
		{adaptador{Agente: "x", Problema: "p", ProblemaCodigo: "sesion_no_responde"}, estSesionNoResponde, "Revisar"},
		{adaptador{Agente: "x", Problema: "p"}, estRevisar, "Revisar"},
	}
	for _, c := range casos {
		if got := estadoDe(c.a); got != c.quiere || pe.etiqueta(got) != c.etiqueta {
			t.Errorf("%+v: estado %d (%s), esperaba %d (%s)", c.a, got, pe.etiqueta(got), c.quiere, c.etiqueta)
		}
	}
}

// TestAgentesCabe comprueba que la vista, también la del detalle, cabe en la ventana mínima y en una estrecha.
func TestAgentesCabe(t *testing.T) {
	p, _, _ := abierta(t)
	p.agentes = ordenar([]adaptador{{Agente: "claude", Autenticacion: "suscripcion", Version: "2.1.0"}, {Agente: "codex", Problema: "Codex no está instalado", ProblemaCodigo: "no_instalado"}, {Agente: "simulado"}})
	p.lista.Poner(p.agentes)
	for _, tam := range [][2]int{{60, 17}, {40, 10}} {
		p.Update(tea.WindowSizeMsg{Width: tam[0], Height: tam[1]})
		comprobar := func(donde string) {
			v := p.Vista()
			if lipgloss.Height(v) > tam[1] || lipgloss.Width(v) > tam[0] {
				t.Errorf("%s a %dx%d mide %dx%d:\n%s", donde, tam[0], tam[1], lipgloss.Width(v), lipgloss.Height(v), v)
			}
		}
		comprobar("lista")
		p.Update(tecla("enter"))
		comprobar("detalle")
		p.Update(tecla("esc"))
	}
}
