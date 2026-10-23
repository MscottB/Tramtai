package inicio

// Las pruebas de cómo se pintan estados que el servicio de prueba no da: espacios con un problema, el conversor con un error
// de red o preparándose, contadores de cinco cifras, peticiones de hace diez días o con la entrada ya borrada por la
// retención. Se le dan a la página los mensajes de su carga hechos a mano. Lo demás de Inicio se prueba por su interfaz,
// contra el servicio (inicio_test.go).

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

var tecla = prueba.Tecla

// correr ejecuta un Cmd y alimenta a la página con lo que sale, como haría la app. Los mensajes del flujo y de los
// temporizadores se le dan a la página pero no se vuelven a esperar (esperarían para siempre): las pruebas los leen de p.canal
// o los mandan a mano. Devuelve los mensajes que son de la app.
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
		case msgFlujo, msgRed, msgTic, msgRefresco:
			p.Update(m)
		default:
			_, siguiente := p.Update(m)
			cola = append(cola, siguiente)
		}
	}
	return fuera
}

// reloj es el reloj de la página en las pruebas.
type reloj struct{ t time.Time }

func (r *reloj) ahora() time.Time { return r.t }

// nueva crea la página con el reloj controlado y las esperas instantáneas (apuntadas).
func nueva(lengua string) (*Pagina, *reloj, *apuntes) {
	p := Nueva(lengua).(*Pagina)
	r := &reloj{t: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	esperas := &apuntes{}
	p.ahora = r.ahora
	p.esperaPara = func(d time.Duration) <-chan time.Time {
		if d != cadaLimpieza && d != cadaRefresco { // solo las esperas de la reconexión
			esperas.add(d)
		}
		c := make(chan time.Time, 1)
		c <- time.Time{}
		return c
	}
	return p, r, esperas
}

// apuntes guarda duraciones que apunta la goroutine del flujo y lee la prueba.
type apuntes struct {
	mu sync.Mutex
	d  []time.Duration
}

func (a *apuntes) add(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.d = append(a.d, d)
}

func (a *apuntes) lista() []time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.d)
}

func contexto(cli *cliente.Cliente, instancia string) pagina.Contexto {
	return pagina.Contexto{Ctx: context.Background(), Cli: cli, Instancia: instancia, Idioma: "es",
		Estilos: &pagina.Estilos{Iconos: pagina.IconosUnicode}, Ancho: 60, Alto: 17}
}

// abierta inicia la página contra el servicio real y espera a que lleguen sus datos.
func abierta(t *testing.T) (*Pagina, *reloj, *cliente.Cliente, string) {
	t.Helper()
	cli, id := prueba.Servicio(t)
	p, r, _ := nueva("es")
	cmd := p.Iniciar(contexto(cli, id))
	if v := p.Vista(); !strings.Contains(v, "Cargando…") {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	correr(p, cmd)
	t.Cleanup(p.Ocultar)
	return p, r, cli, id
}

// evento manda un evento «trabajo» a la página como si llegara del flujo.
func (p *Pagina) evento(e contrato.EventoTrabajo) {
	p.Update(msgRed{origen: p.origen(), tipo: "trabajo", ev: e})
}

// TestAvisoEspaciosYConversor: los espacios con problema y el conversor que falta salen como avisos, con su tecla.
func TestAvisoEspaciosYConversor(t *testing.T) {
	p, _, _, _ := abierta(t)
	p.Update(msgEspacios{origen: p.origen(), lista: []contrato.Espacio{{ID: "a", Nombre: "Notas", UltimoError: "no se encuentra"}, {ID: "b", Nombre: "Soporte"}}})
	if v := p.Vista(); !strings.Contains(v, "El espacio Notas tiene un problema.") || !strings.Contains(v, "Ver espacios") {
		t.Fatalf("un espacio:\n%s", v)
	}
	p.Update(msgEspacios{origen: p.origen(), lista: []contrato.Espacio{{Nombre: "Notas", UltimoError: "x"}, {Nombre: "Soporte", UltimoError: "y"}}})
	if v := p.Vista(); !strings.Contains(v, "2 espacios tienen un problema y no se abrieron.") {
		t.Fatalf("varios:\n%s", v)
	}
	_, cmd := p.Update(tecla("v"))
	if ir, ok := correr(p, cmd)[0].(pagina.IrA); !ok || ir.Pagina != pagina.PaginaEspacios {
		t.Fatalf("v lleva a Espacios: %v", ir)
	}

	p.Update(msgConversor{origen: p.origen(), e: contrato.EstadoConversor{Estado: "falta"}})
	v := p.Vista()
	for _, quiere := range []string{"Falta el conversor de archivos", "Preparar archivos", "Ajustes"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	_, cmd = p.Update(tecla("s"))
	if ir, ok := correr(p, cmd)[0].(pagina.IrA); !ok || ir.Pagina != pagina.PaginaConfiguracion {
		t.Fatalf("s lleva a Ajustes: %v", ir)
	}
	if _, cmd := p.Update(tecla("p")); cmd == nil {
		t.Fatal("p prepara el conversor") // no se ejecuta: descargaría Python
	}
	p.Update(msgConversor{origen: p.origen(), e: contrato.EstadoConversor{Estado: "error", Mensaje: "sin red", Paquete: true}})
	if v := p.Vista(); !strings.Contains(v, "No se pudo preparar el conversor de archivos: sin red") || !strings.Contains(v, "Si la red de tu empresa") {
		t.Fatalf("error:\n%s", v)
	}
	// Sin paquete del conversor para este sistema (solo lo hay para Windows x64), no se ofrece.
	p.Update(msgConversor{origen: p.origen(), e: contrato.EstadoConversor{Estado: "error", Mensaje: "sin red"}})
	if v := p.Vista(); !strings.Contains(v, "sin red") || strings.Contains(v, "Si la red de tu empresa") {
		t.Fatalf("error sin paquete:\n%s", v)
	}
	// Preparándose o listo: sin aviso, como la Web UI.
	for _, estado := range []string{"preparando", "listo"} {
		p.Update(msgConversor{origen: p.origen(), e: contrato.EstadoConversor{Estado: estado}})
		if strings.Contains(p.Vista(), "Preparar archivos") {
			t.Fatalf("%s: sin aviso", estado)
		}
	}
}

// TestCabeEn60x20: con todos los avisos, 12 filas y los pasos, la vista mide como mucho el área y nada se sale por el
// ancho, también con tildes; desplazar llega hasta el final.
func TestCabeEn60x20(t *testing.T) {
	p, r, cli, _ := abierta(t)
	p.Update(msgInstancia{origen: p.origen(), r: contrato.ResumenInstancia{Base: cli.Base, AppConClave24h: 50, SugerirApiPropia: true, Contadores: map[string]int{nucleo.FiltroCompletado: 12345}}})
	p.Update(msgEspacios{origen: p.origen(), lista: []contrato.Espacio{{Nombre: "Notas con un nombre muy largo y tildes áéíóú", UltimoError: "x"}}})
	p.Update(msgConversor{origen: p.origen(), e: contrato.EstadoConversor{Estado: "error", Mensaje: "sin red"}})
	p.Update(msgPasos{origen: p.origen(), r: contrato.PrimerosPasos{Adaptadores: []contrato.Adaptador{{Agente: "claude", Estado: contrato.AgenteListo}, {Agente: "codex", Estado: contrato.AgenteDesconectado}},
		Pasos: []contrato.Paso{{Pagina: "agentes", Hecho: true, Listos: 1, Total: 2}, {Pagina: "ejecutar"}, {Pagina: "conectar"}, {Pagina: "configuracion"}}}})
	var recientes []nucleo.Trabajo
	for i := 0; i < 5; i++ {
		recientes = append(recientes, trabajoDe(fmt.Sprint("r", i), "una pregunta bastante larga con tildes áéíóú que no cabe en una sola línea de sesenta columnas", "claude", "claude/modelo-con-nombre-largo", "completado", r.t))
	}
	p.Update(msgRecientes{origen: p.origen(), lista: recientes})
	for i := 0; i < 14; i++ {
		r.t = r.t.Add(time.Second)
		p.evento(ev(con(fmt.Sprintf("t%02d", i), "validando_salida", "validacion"), func(e *contrato.EventoTrabajo) {
			e.API = "aplicación-ñandú-" + fmt.Sprint(i)
			e.Agente = "claude"
			e.Creado = r.t
		}))
	}
	veces := 0
	for ; veces < 40; veces++ {
		v := p.Vista()
		if h := lipgloss.Height(v); h > 17 {
			t.Fatalf("mide %d filas y el área es de 17:\n%s", h, v)
		}
		for _, l := range strings.Split(v, "\n") {
			if w := lipgloss.Width(l); w > 60 {
				t.Fatalf("la línea mide %d columnas y caben 60: %q", w, l)
			}
		}
		p.Update(tecla("pgdown"))
	}
	p.Update(tecla("end"))
	fin := p.Vista()
	if !strings.Contains(fin, "Ver historial") {
		t.Fatalf("End llega al final:\n%s", fin)
	}
	p.Update(tecla("home"))
	if v := p.Vista(); !strings.HasPrefix(v, "Tus apps se conectan") {
		t.Fatalf("Home vuelve al principio:\n%s", v)
	}
	// Y la ventana puede cambiar a mitad sin perder el sitio ni salirse.
	p.Update(tecla("down"))
	p.Update(tea.WindowSizeMsg{Width: 78, Height: 30})
	p.Update(tea.WindowSizeMsg{Width: 60, Height: 17})
	if h := lipgloss.Height(p.Vista()); h > 17 {
		t.Fatalf("tras redimensionar mide %d", h)
	}
}

// trabajoDe es un trabajo reciente de la prueba: lo que se escribió, el agente, el modelo, cómo acabó y hace cuánto.
func trabajoDe(id, texto, agente, modelo, estado string, creado time.Time) nucleo.Trabajo {
	j := nucleo.Trabajo{ID: id, Estado: nucleo.Estado(estado), ModeloEfectivo: modelo, CreadoEn: creado}
	j.Solicitud.Agente = agente
	j.Solicitud.Mensajes = []nucleo.Mensaje{{Rol: "usuario", Contenido: "antes"}, {Rol: "usuario", Contenido: texto}}
	return j
}

// TestPeticionesRecientes: las 5 últimas, con lo que escribiste, el agente y el modelo, el estado y hace cuánto, como en
// Panel.vue; h lleva al Historial.
func TestPeticionesRecientes(t *testing.T) {
	p, r, _, _ := abierta(t)
	p.c.Alto = 60
	if v := p.Vista(); !strings.Contains(v, "Peticiones recientes") || !strings.Contains(v, "Todavía no hay peticiones") {
		t.Fatalf("vacío:\n%s", v)
	}
	var lista []nucleo.Trabajo
	for i := 0; i < 7; i++ {
		lista = append(lista, trabajoDe(fmt.Sprint("j", i), fmt.Sprintf("pregunta %d\ncon salto", i), "claude", "claude/sonnet", "completado", r.t.Add(-time.Duration(i+1)*time.Minute)))
	}
	lista[1].Estado = "fallido"
	lista[2].Solicitud.Mensajes, lista[2].EntradasEliminadas = nil, true
	lista[3].CreadoEn = r.t.Add(-30 * time.Second)
	lista[4].CreadoEn = r.t.Add(-3 * time.Hour)
	p.Update(msgRecientes{origen: p.origen(), lista: lista})
	v := p.Vista()
	for _, quiere := range []string{"pregunta 0 con salto", "Claude claude/sonnet", "hace 1 min", "Completado", "Falló", "(entrada eliminada por la retención)",
		"hace 30 s", "hace 3 h", "Ver historial"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if strings.Contains(v, "pregunta 5") || strings.Contains(v, "pregunta 6") {
		t.Fatalf("solo las 5 últimas:\n%s", v)
	}
	_, cmd := p.Update(tecla("h"))
	if ir, ok := correr(p, cmd)[0].(pagina.IrA); !ok || ir.Pagina != pagina.PaginaHistorial {
		t.Fatalf("h lleva al Historial: %v", ir)
	}
	// Los viejos de una semana se fechan.
	lista[0].CreadoEn = r.t.Add(-10 * 24 * time.Hour)
	p.Update(msgRecientes{origen: p.origen(), lista: lista[:1]})
	if v := p.Vista(); !strings.Contains(v, r.t.Add(-10*24*time.Hour).Local().Format("2006-01-02")) {
		t.Fatalf("más de una semana, la fecha:\n%s", v)
	}
}
