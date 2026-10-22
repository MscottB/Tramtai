package agentes

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/adaptadores"
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

// conAgentes abre la página contra un servicio con estos agentes además del simulado, ya comprobados: los estados los
// calcula el servicio y la página los pinta.
func conAgentes(t *testing.T, agentes map[string]adaptadores.Adaptador) (*Pagina, *cliente.Cliente, string) {
	t.Helper()
	cli, id := prueba.ConAgentes(t, agentes)
	p := Nueva("es").(*Pagina)
	correr(p, p.Iniciar(contexto(cli, id)))
	return p, cli, id
}

// detectado es un agente que se detecta así; sinCLI, uno cuyo CLI no está.
func detectado(e adaptadores.Estado) adaptadores.Adaptador { return prueba.Detectado{Estado: e} }

var sinCLI = detectado(adaptadores.Estado{Problema: "no está instalado", ProblemaCodigo: adaptadores.ProblemaNoInstalado})

func TestAgentesCarga(t *testing.T) {
	p, _, _ := abierta(t)
	v := p.Vista()
	for _, quiere := range []string{"simulado", "Listo", "Listos (1)", "Falta conectar (0)"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if p.Titulo() != "Agentes" || p.Ayuda() != "agentes" || p.EnCampo() {
		t.Fatalf("título %q, ayuda %q", p.Titulo(), p.Ayuda())
	}
}

// TestColorDeLinea: con color, las dos letras de cada agente van en el color de su línea (DESIGN.md: un color de línea por
// agente); sin color quedan las dos letras.
func TestColorDeLinea(t *testing.T) {
	cli, id := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{})})
	c := contexto(cli, id)
	c.Estilos = &pagina.Estilos{Iconos: pagina.IconosUnicode, Color: true, Oscuro: true}
	p := Nueva("es").(*Pagina)
	correr(p, p.Iniciar(c))
	p.Update(tecla("down")) // la fila elegida va resaltada entera; Claude queda sin elegir
	if v := p.Vista(); !strings.Contains(v, "CL") || !strings.Contains(v, "242;138;79") {
		t.Fatalf("el código va en su color; de noche, linea-claude-noche (#f28a4f): %q", v)
	}
}

// TestFiltroDeAgentes: la línea de los estados es el filtro de la Web UI; ← → lo cambia y la lista enseña solo esos.
func TestFiltroDeAgentes(t *testing.T) {
	p, _, _ := abierta(t)
	if v := p.Vista(); !strings.Contains(v, "[Todos (1)]") {
		t.Fatalf("empieza con todos:\n%s", v)
	}
	p.Update(tecla("right"))
	if v := p.Vista(); !strings.Contains(v, "[Listos (1)]") || !strings.Contains(v, "simulado") {
		t.Fatalf("→ deja los listos:\n%s", v)
	}
	p.Update(tecla("right"))
	if v := p.Vista(); !strings.Contains(v, "[Falta conectar (0)]") || strings.Contains(v, "simulado") {
		t.Fatalf("→ deja los que falta conectar:\n%s", v)
	}
	p.Update(tecla("left"))
	p.Update(tecla("left"))
	p.Update(tecla("left"))
	if v := p.Vista(); !strings.Contains(v, "[No instalados (0)]") {
		t.Fatalf("← da la vuelta:\n%s", v)
	}
	var pie []string
	for _, k := range p.Teclas() {
		pie = append(pie, k.Tecla+" "+k.Accion)
	}
	if !strings.Contains(strings.Join(pie, " · "), "← → filtro") {
		t.Fatalf("el pie dice cómo se filtra: %v", pie)
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
	if r, _ := cli.Capacidades(context.Background(), id); r.Adaptadores[0].Desconectado {
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
	if !r.Adaptadores[0].Desconectado {
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
	if _, cmd := p.Update(comp.Confirmado{}); cmd != nil {
		t.Fatal("no queda nada pendiente que confirmar")
	}

	// Claude sí la admite: con clave de API y listo (va primero), pasar a la suscripción pide aceptar el riesgo y, al
	// confirmar, lo pide al servicio.
	p, _, _ = conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "clave_api"})})
	_, cmd = p.Update(tecla("a"))
	fuera = correr(p, cmd)
	q, ok := fuera[0].(comp.Pregunta)
	if len(fuera) != 1 || !ok || !strings.Contains(q.Texto, "Ese riesgo es tuyo") || !strings.Contains(q.Texto, "Usar tu suscripción con Claude") {
		t.Fatalf("pregunta con el riesgo: %+v", fuera)
	}
	_, cmd = p.Update(comp.Confirmado{})
	if _, ok := aviso(correr(p, cmd)); !ok {
		t.Fatal("recuerda qué iba a hacer: al confirmar lo hace y avisa")
	}

	// Con suscripción, pasar a la clave es directo, sin pregunta.
	p, _, _ = conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "suscripcion"})})
	_, cmd = p.Update(tecla("a"))
	fuera = correr(p, cmd)
	if a, ok := aviso(fuera); !ok || len(fuera) != 1 || a.Texto == "" {
		t.Fatalf("pasar a la clave sale directo y avisa: %+v", fuera)
	}

	// Sin instalar o desconectado, no se cambia aunque lo admita.
	sinInstalar, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": sinCLI})
	desconectado, cli, id := conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "clave_api"})})
	if err := cli.ConectarAgente(context.Background(), "claude", false); err != nil {
		t.Fatal(err)
	}
	correr(desconectado, desconectado.Iniciar(contexto(cli, id)))
	for nombre, p := range map[string]*Pagina{"sin instalar": sinInstalar, "desconectado": desconectado} {
		p.Update(tecla("down")) // el simulado, listo, va primero
		_, cmd = p.Update(tecla("a"))
		fuera = correr(p, cmd)
		if _, ok := aviso(fuera); !ok || len(fuera) != 1 {
			t.Fatalf("%s no se cambia: %+v", nombre, fuera)
		}
		if _, cmd := p.Update(comp.Confirmado{}); cmd != nil {
			t.Fatalf("%s: no queda nada pendiente", nombre)
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
	p, _, _ = conAgentes(t, map[string]adaptadores.Adaptador{"codex": sinCLI})
	p.Update(tecla("down"))
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
	p, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"codex": sinCLI, "claude": detectado(adaptadores.Estado{})})
	if v := p.Vista(); strings.Index(v, "Claude") > strings.Index(v, "Codex") || !strings.Contains(v, "No instalado") {
		t.Fatalf("los no instalados van al final:\n%s", v)
	}
	p.Update(tecla("down"))
	p.Update(tecla("down"))
	p.Update(tecla("enter"))
	v := p.Vista()
	if !strings.Contains(v, "comandos_agentes.codex") || !strings.Contains(v, "Cómo instalarlo: http") {
		t.Fatalf("el detalle dice cómo instalarlo:\n%s", v)
	}
}

func TestEscVuelve(t *testing.T) {
	p, _, _ := abierta(t)
	p.Update(tecla("enter"))
	if _, cmd := p.Update(tecla("esc")); cmd != nil || !strings.Contains(p.Vista(), "[Todos (1)]") {
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
	cargando := func() bool { return strings.Contains(p.Vista(), "Cargando…") }
	if f := correr(p, cmd); len(f) != 0 || !cargando() {
		t.Fatalf("una respuesta con el Ctx cancelado se descarta: %+v", f)
	}
	// Otra instancia: tampoco vale.
	p = Nueva("es").(*Pagina)
	cmd = p.Iniciar(contexto(cli, id))
	p.Iniciar(contexto(cli, "otro")) // la app cambió de espacio; su comprobación no se corre
	if correr(p, cmd); !cargando() {
		t.Fatal("una respuesta de otro espacio se descarta")
	}
	// De dos comprobaciones seguidas solo vale la última.
	p = Nueva("es").(*Pagina)
	primera := p.Iniciar(contexto(cli, id))
	segunda := p.Iniciar(contexto(cli, id))
	if correr(p, primera); !cargando() {
		t.Fatal("la primera ya es vieja")
	}
	if correr(p, segunda); cargando() {
		t.Fatal("la última sí")
	}
}

// TestAgentesPintaElEstadoDelServicio: la página no clasifica a los agentes; pinta la etiqueta del estado que llega.
func TestAgentesPintaElEstadoDelServicio(t *testing.T) {
	casos := map[adaptadores.Estado]string{
		{}: "Listo",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaAviso}:                                    "Listo",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaNoInstalado}:                              "No instalado",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaFaltaClave, Autenticacion: "clave_api"}:   "Sin conectar",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaFaltaClave, Autenticacion: "suscripcion"}: "Sin conectar",
		{Problema: "p", ProblemaCodigo: adaptadores.ProblemaUsaClave}:                                 "Revisar",
		{Problema: "p"}: "Revisar",
	}
	for e, etiqueta := range casos {
		p, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(e)})
		if v := p.Vista(); !strings.Contains(v, "Claude") || !strings.Contains(v, etiqueta) {
			t.Errorf("%+v: falta %q:\n%s", e, etiqueta, v)
		}
	}
}

// TestAgentesCabe comprueba que la vista, también la del detalle, cabe en la ventana mínima y en una estrecha.
func TestAgentesCabe(t *testing.T) {
	p, _, _ := conAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: "suscripcion", Version: "2.1.0"}),
		"codex": detectado(adaptadores.Estado{Problema: "Codex no está instalado", ProblemaCodigo: adaptadores.ProblemaNoInstalado})})
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
