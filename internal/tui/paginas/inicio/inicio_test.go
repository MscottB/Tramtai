package inicio

import (
	"context"
	"errors"
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
	"tramtai/internal/tui/datos"
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

// hasta lee el flujo de la página hasta que se cumpla la condición.
func hasta(t *testing.T, p *Pagina, cond func() bool) {
	t.Helper()
	limite := time.After(5 * time.Second)
	for !cond() {
		select {
		case m, ok := <-p.canal:
			if !ok {
				t.Fatal("el flujo se cerró antes de lo esperado")
			}
			p.Update(m)
		case <-limite:
			t.Fatalf("no llegó lo esperado:\n%s", p.Vista())
		}
	}
}

// reloj es el reloj de la página en las pruebas.
type reloj struct{ t time.Time }

func (r *reloj) ahora() time.Time { return r.t }

// nueva crea la página con el reloj controlado, las esperas instantáneas (apuntadas) y el arranque automático disponible.
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
	p.autoarranque = func() (bool, bool) { return true, false }
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

// origen es lo que llevan los mensajes propios para saber si siguen valiendo.
func (p *Pagina) origenActual() origen {
	return origen{ctx: p.c.Ctx, instancia: p.c.Instancia, gen: p.gen}
}

// evento manda un evento «trabajo» a la página como si llegara del flujo.
func (p *Pagina) evento(e evento) {
	p.Update(msgRed{origen: p.origenActual(), tipo: "trabajo", ev: e})
}

func TestInicioCarga(t *testing.T) {
	p, _, cli, _ := abierta(t)
	v := p.Vista()
	for _, quiere := range []string{"Inicio", "Tus apps se conectan a " + cli.Base, "Primeros pasos", "Completadas", "En proceso", "Con error",
		"1 de 1 agentes listos"} {
		if !strings.Contains(v, quiere) {
			// Los pasos están más abajo: se ven al desplazar.
			p.Update(tecla("end"))
			if !strings.Contains(p.Vista(), quiere) {
				t.Fatalf("falta %q:\n%s", quiere, v)
			}
			p.Update(tecla("home"))
		}
	}
	if p.Titulo() != "Inicio" || p.Ayuda() != "inicio" || p.EnCampo() {
		t.Fatalf("título %q, ayuda %q", p.Titulo(), p.Ayuda())
	}
}

// TestRedAparecenYSeVan: una petición aparece en la red con su API, su agente, su etapa y su tiempo, y las que terminan
// se quedan 2 s (4 si fallan) y se van. El flujo es el del servicio real; el reloj, el de la prueba.
func TestRedAparecenYSeVan(t *testing.T) {
	cli, id := prueba.Servicio(t)
	hola := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}
	if _, err := cli.Crear(context.Background(), id, hola); err != nil {
		t.Fatal(err)
	}
	p, r, _ := nueva("es")
	correr(p, p.Iniciar(contexto(cli, id)))
	t.Cleanup(p.Ocultar)
	hasta(t, p, func() bool { return p.enVivo && len(p.red) == 1 })
	var tr tranvia
	for _, x := range p.red {
		tr = x
	}
	if tr.Agente != "simulado" || tr.Origen != nucleo.DesdeTerminal {
		t.Fatalf("la fila trae el agente y de dónde salió: %+v", tr)
	}
	hasta(t, p, func() bool { return p.red[tr.ID].Etapa == "llegada" })
	p.Update(tecla("down")) // sin efecto aquí: la vista cabe
	v := p.Vista()
	for _, quiere := range []string{"Clave del espacio", "simulado", "terminada", "0 s"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	// Llegó: sigue 2 s y luego se va.
	r.t = r.t.Add(1999 * time.Millisecond)
	p.Update(msgTic{origen: p.origenActual()})
	if len(p.red) != 1 {
		t.Fatalf("a los 1999 ms sigue: %v", p.red)
	}
	r.t = r.t.Add(time.Millisecond)
	p.Update(msgTic{origen: p.origenActual()})
	if len(p.red) != 0 || strings.Contains(p.Vista(), "simulado") {
		t.Fatalf("a los 2 s se va: %v\n%s", p.red, p.Vista())
	}
	if !p.borrados[tr.ID] {
		t.Fatal("y no vuelve por un evento tardío")
	}

	// Una que falla se queda 4 s.
	p.evento(ev(con("f", "fallido", "llegada")))
	r.t = r.t.Add(3999 * time.Millisecond)
	p.Update(msgTic{origen: p.origenActual()})
	if v := p.Vista(); !strings.Contains(v, "falló") {
		t.Fatalf("a los 3999 ms se ve que falló:\n%s", v)
	}
	r.t = r.t.Add(time.Millisecond)
	p.Update(msgTic{origen: p.origenActual()})
	if len(p.red) != 0 {
		t.Fatalf("a los 4 s se va: %v", p.red)
	}

	// Una petición nueva del servicio aparece en vivo, con la API que la mandó.
	if _, err := cli.Crear(context.Background(), id, hola); err != nil {
		t.Fatal(err)
	}
	hasta(t, p, func() bool { return len(p.red) == 1 })
}

// TestRedComoMucho12: la red enseña como mucho 12 filas, las más recientes, y dice cuántas quedan fuera.
func TestRedComoMucho12(t *testing.T) {
	p, r, _ := nueva("es")
	c := contexto(nil, "global")
	c.Alto = 40
	p.Iniciar(c)
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Base: "http://127.0.0.1:1"}}) // sin servicio: se da por cargada
	for i := 0; i < 15; i++ {
		r.t = r.t.Add(time.Second)
		p.evento(ev(con(fmt.Sprintf("t%02d", i), "ejecutando", "ejecucion"), func(e *evento) { e.API = fmt.Sprintf("api%02d", i); e.Creado = r.t }))
	}
	v := p.Vista()
	filas := 0
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "en ejecución") {
			filas++
		}
	}
	if filas != 12 || !strings.Contains(v, "+3") || !strings.Contains(v, "api14") || strings.Contains(v, "api02 ") || !strings.Contains(v, "15 peticiones en curso.") {
		t.Fatalf("%d filas:\n%s", filas, v)
	}
}

// ensayo es un flujo falso: cada llamada ejecuta el siguiente guion.
type ensayo struct {
	mu       sync.Mutex // la goroutine del flujo escribe y la prueba lee
	llamadas int
	plazos   []time.Duration
	guion    []func(ctx context.Context, alEvento func(string, []byte) error) error
}

func (e *ensayo) abrir(ctx context.Context, _ string, plazo time.Duration, alEvento func(string, []byte) error) error {
	e.mu.Lock()
	e.plazos = append(e.plazos, plazo)
	paso := e.guion[min(e.llamadas, len(e.guion)-1)]
	e.llamadas++
	e.mu.Unlock()
	return paso(ctx, alEvento)
}

func (e *ensayo) cuenta() (llamadas int, plazos []time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.llamadas, slices.Clone(e.plazos)
}

func foto(alEvento func(string, []byte) error) { _ = alEvento("foto", []byte("{}")) }

// TestRedReconecta: si el flujo se corta, la página vuelve a conectar con las esperas de vivo.ts (1 s, 2 s…), un flujo
// mudo (ErrSinActividad) cuenta como cortado, y la foto del flujo nuevo quita lo que ya no existe.
func TestRedReconecta(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p, _, esperas := nueva("es")
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(_ context.Context, al func(string, []byte) error) error { // 1: manda uno y la foto, y se corta
			_ = al("trabajo", []byte(`{"id":"a","estado":"ejecutando","etapa":"ejecucion","agente":"claude"}`))
			foto(al)
			return errors.New("se cortó")
		},
		func(context.Context, func(string, []byte) error) error { return cliente.ErrSinActividad }, // 2: ni la foto
		func(ctx context.Context, al func(string, []byte) error) error { // 3: vuelve, sin el trabajo a
			foto(al)
			<-ctx.Done()
			return ctx.Err()
		},
	}}
	p.abrir = f.abrir
	correr(p, p.Iniciar(contexto(cli, id)))
	t.Cleanup(p.Ocultar)
	hasta(t, p, func() bool { return len(p.red) == 1 && p.enVivo })
	hasta(t, p, func() bool { n, _ := f.cuenta(); return n == 3 && p.enVivo && len(p.red) == 0 })
	if !slices.Equal(esperas.lista(), []time.Duration{time.Second, 2 * time.Second}) {
		t.Fatalf("esperas %v", esperas.lista())
	}
	if p.fallos != 0 {
		t.Fatalf("la foto del flujo nuevo cuenta como recuperado: %d fallos", p.fallos)
	}
	_, plazos := f.cuenta()
	for _, plazo := range plazos {
		if plazo != 45*time.Second {
			t.Fatalf("el flujo se vigila a los 45 s: %v", plazos)
		}
	}
}

// TestRedConsultaSinFlujo: tras 3 cortes seguidos la página consulta los trabajos cada 5 s (como la Web UI): lo que ya
// no sale en la consulta se va.
func TestRedConsultaSinFlujo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p, _, _ := nueva("es")
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(context.Context, func(string, []byte) error) error { return errors.New("sin flujo") },
	}}
	p.abrir = f.abrir
	correr(p, p.Iniciar(contexto(cli, id)))
	t.Cleanup(p.Ocultar)
	hasta(t, p, func() bool { return p.fallos >= 3 })
	if v := p.Vista(); !strings.Contains(v, "Sin conexión en vivo") {
		t.Fatalf("avisa de que no hay flujo:\n%s", v)
	}
	p.evento(ev(con("fantasma", "ejecutando", "ejecucion")))
	correr(p, p.refrescar())
	if len(p.red) != 0 {
		t.Fatalf("la consulta no trae el trabajo fantasma: %v", p.red)
	}
}

// TestRedSeCierraAlSalir: el flujo se cierra al salir de Inicio (Ocultar), al cancelarse el contexto de la página
// (cambio de espacio, reinicio) y se vuelve a abrir al volver (Mostrar).
func TestRedSeCierraAlSalir(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p, _, _ := nueva("es")
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	c := contexto(cli, id)
	c.Ctx = ctx
	correr(p, p.Iniciar(c))
	hasta(t, p, func() bool { return p.enVivo })
	cerrado := func(canal <-chan msgRed) {
		t.Helper()
		limite := time.After(5 * time.Second)
		for {
			select {
			case _, ok := <-canal:
				if !ok {
					return
				}
			case <-limite:
				t.Fatal("el flujo sigue abierto")
			}
		}
	}
	primero := p.canal
	p.Ocultar()
	cerrado(primero)
	if p.visible {
		t.Fatal("oculta")
	}

	correr(p, p.Mostrar())
	hasta(t, p, func() bool { return p.enVivo })
	if p.canal == primero {
		t.Fatal("al volver abre un flujo nuevo")
	}
	segundo := p.canal
	cancelar() // el contexto de la página: cambió el espacio o se reinició
	cerrado(segundo)

	// Otro Iniciar cancela el flujo de antes y abre el suyo.
	cli2, id2 := prueba.Servicio(t)
	p2, _, _ := nueva("es")
	correr(p2, p2.Iniciar(contexto(cli2, id2)))
	hasta(t, p2, func() bool { return p2.enVivo })
	anterior := p2.canal
	correr(p2, p2.Iniciar(contexto(cli2, id2)))
	cerrado(anterior)
	hasta(t, p2, func() bool { return p2.enVivo })
	p2.Ocultar()
}

// TestAvisoApiPropia: con más de 20 peticiones de apps con la clave del espacio en 24 h se sugiere una API propia, y c
// lleva a Conectar. Con 20 o menos no se dice nada.
func TestAvisoApiPropia(t *testing.T) {
	p, _, cli, id := abierta(t)
	if strings.Contains(p.Vista(), "con la clave del espacio") {
		t.Fatalf("sin peticiones no hay aviso:\n%s", p.Vista())
	}
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Base: cli.Base, AppConClave24h: 20}})
	if strings.Contains(p.Vista(), "con la clave del espacio") {
		t.Fatal("20 no basta")
	}
	if _, cmd := p.Update(tecla("c")); cmd != nil {
		t.Fatal("sin aviso, c no hace nada")
	}
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Base: cli.Base, AppConClave24h: 21}})
	v := p.Vista()
	for _, quiere := range []string{"21 peticiones de apps llegaron con la clave del espacio", "últimas 24 h", "Crear una API"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	_, cmd := p.Update(tecla("c"))
	if ir, ok := correr(p, cmd)[0].(pagina.IrA); !ok || ir.Pagina != pagina.PaginaConectar {
		t.Fatalf("c lleva a Conectar: %v", ir)
	}
	_ = id
}

// TestAvisoEspaciosYConversor: los espacios con problema y el conversor que falta salen como avisos, con su tecla.
func TestAvisoEspaciosYConversor(t *testing.T) {
	p, _, _, _ := abierta(t)
	p.Update(msgEspacios{origen: p.origenActual(), lista: []contrato.Espacio{{ID: "a", Nombre: "Notas", UltimoError: "no se encuentra"}, {ID: "b", Nombre: "Soporte"}}})
	if v := p.Vista(); !strings.Contains(v, "El espacio Notas tiene un problema.") || !strings.Contains(v, "Ver espacios") {
		t.Fatalf("un espacio:\n%s", v)
	}
	p.Update(msgEspacios{origen: p.origenActual(), lista: []contrato.Espacio{{Nombre: "Notas", UltimoError: "x"}, {Nombre: "Soporte", UltimoError: "y"}}})
	if v := p.Vista(); !strings.Contains(v, "2 espacios tienen un problema y no se abrieron.") {
		t.Fatalf("varios:\n%s", v)
	}
	_, cmd := p.Update(tecla("v"))
	if ir, ok := correr(p, cmd)[0].(pagina.IrA); !ok || ir.Pagina != pagina.PaginaEspacios {
		t.Fatalf("v lleva a Espacios: %v", ir)
	}

	p.Update(msgConversor{origen: p.origenActual(), e: contrato.EstadoConversor{Estado: "falta"}})
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
	p.Update(msgConversor{origen: p.origenActual(), e: contrato.EstadoConversor{Estado: "error", Mensaje: "sin red"}})
	if v := p.Vista(); !strings.Contains(v, "No se pudo preparar el conversor de archivos: sin red") || !strings.Contains(v, "Si la red de tu empresa") {
		t.Fatalf("error:\n%s", v)
	}
	// Preparándose o listo: sin aviso, como la Web UI.
	for _, estado := range []string{"preparando", "listo"} {
		p.Update(msgConversor{origen: p.origenActual(), e: contrato.EstadoConversor{Estado: estado}})
		if strings.Contains(p.Vista(), "Preparar archivos") {
			t.Fatalf("%s: sin aviso", estado)
		}
	}
}

// TestPrimerosPasos: los 4 pasos con su estado, y «Ver la Ayuda de Empezar» abre el tema que acompaña a Inicio.
func TestPrimerosPasos(t *testing.T) {
	p, _, _, _ := abierta(t)
	p.Update(msgAgentes{origen: p.origenActual(), lista: []adaptador{{Agente: "claude"}, {Agente: "codex", Desconectado: true}}})
	p.Update(msgPuntos{origen: p.origenActual(), lista: []contrato.EstadoPunto{{Nombre: "mi-app"}}})
	p.Update(tecla("end"))
	v := p.Vista()
	for _, quiere := range []string{"Primeros pasos", "✓ Conecta al menos un agente", "1 de 2 agentes listos", "○ Prueba un agente", "✓ Conecta tu aplicación",
		"○ Arranca Tramtai con " + sistema(), "2 de 4 pasos hechos"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	// Sin arranque automático en este sistema, son 3 pasos.
	p.autoarranque = func() (bool, bool) { return false, false }
	p.Update(msgAutoarranque{origen: p.origenActual(), disponible: false})
	if v := p.Vista(); strings.Contains(v, "Arranca Tramtai") || !strings.Contains(v, "2 de 3 pasos hechos") {
		t.Fatalf("tres pasos:\n%s", v)
	}

	tema := ""
	for _, x := range datos.Ayuda("es") {
		if x.ID == "empezar" {
			tema = x.Pagina
		}
	}
	if tema != "inicio" || !strings.Contains(p.Vista(), "Ver la Ayuda de Empezar") {
		t.Fatalf("el tema empezar acompaña a %q:\n%s", tema, p.Vista())
	}
	_, cmd := p.Update(tecla("a"))
	ir, ok := correr(p, cmd)[0].(pagina.IrA)
	if !ok || ir.Pagina != pagina.PaginaAyuda || ir.Datos != (pagina.ConAyuda{Pagina: tema}) {
		t.Fatalf("a abre la Ayuda de Empezar: %+v", ir)
	}
	_, cmd = p.Update(tecla("g"))
	if ir, ok := correr(p, cmd)[0].(pagina.IrA); !ok || ir.Pagina != pagina.PaginaAgentes {
		t.Fatalf("g lleva a Agentes: %v", ir)
	}

	// Con todo hecho, la guía se pliega en «Todo listo».
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Trabajos: map[string]int{"completado": 3}}})
	p.Update(msgAgentes{origen: p.origenActual(), lista: []adaptador{{Agente: "claude"}}})
	if v := p.Vista(); !strings.Contains(v, "Todo listo") || strings.Contains(v, "○") {
		t.Fatalf("todo listo:\n%s", v)
	}
}

// TestContadores: los contadores salen de los trabajos del espacio, con los mismos nombres de la Web UI.
func TestContadores(t *testing.T) {
	p, _, cli, id := abierta(t)
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Base: cli.Base,
		Trabajos: map[string]int{"completado": 5, "fallido": 2, "interrumpido": 1, "ejecutando": 1, "en_cola": 2, "cancelado": 4}}})
	v := p.Vista()
	for _, quiere := range []string{"Completadas", "respondidas correctamente", "5", "En proceso", "3", "Con error", "3", "los últimos 7 días"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	// Y con el servicio real: un trabajo hecho suma uno.
	if _, err := cli.Crear(context.Background(), id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}); err != nil {
		t.Fatal(err)
	}
	limite := time.Now().Add(5 * time.Second)
	for {
		r, err := cli.Instancia(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Trabajos["completado"] == 1 {
			break
		}
		if time.Now().After(limite) {
			t.Fatalf("el trabajo no terminó: %v", r.Trabajos)
		}
		time.Sleep(20 * time.Millisecond)
	}
	correr(p, p.refrescar())
	if v := p.Vista(); !strings.Contains(v, "Completadas") || !strings.Contains(v, " 1") {
		t.Fatalf("suma uno:\n%s", v)
	}
	p.Update(msgRetencion{origen: p.origenActual(), horas: 24})
	if v := p.Vista(); !strings.Contains(v, "el último día") {
		t.Fatalf("el periodo sale de la retención:\n%s", v)
	}
}

func TestEscVuelve(t *testing.T) {
	p, _, _, _ := abierta(t)
	_, cmd := p.Update(tecla("esc"))
	if _, ok := cmd().(pagina.Volver); !ok {
		t.Fatal("Esc sin nada abierto vuelve")
	}
}

// TestServicioCaido: sin cliente, o con el servicio parado a mitad, la página avisa y no se cierra.
func TestServicioCaido(t *testing.T) {
	p, _, _ := nueva("es")
	fuera := correr(p, p.Iniciar(contexto(nil, "global")))
	if len(fuera) != 1 {
		t.Fatalf("sin cliente: %v", fuera)
	}
	if e, ok := fuera[0].(pagina.ErrorServicio); !ok || !errors.Is(e.Err, cliente.ErrServicioInactivo) {
		t.Fatalf("sin cliente: %v", fuera[0])
	}
	if v := p.Vista(); !strings.Contains(v, "El servicio no responde · r reintenta") && !strings.Contains(v, "Cargando…") {
		t.Fatalf("sin cliente la página sigue:\n%s", v)
	}

	cli, id, cerrar := prueba.Arrancar(t)
	q, _, _ := nueva("es")
	q.abrir = func(ctx context.Context, _ string, _ time.Duration, _ func(string, []byte) error) error { // sin conexión real: cerrar el servicio no espera
		<-ctx.Done()
		return ctx.Err()
	}
	correr(q, q.Iniciar(contexto(cli, id)))
	t.Cleanup(q.Ocultar)
	cerrar()
	fuera = correr(q, q.refrescar())
	if len(fuera) == 0 {
		t.Fatal("con el servicio parado, el refresco avisa")
	}
	if _, ok := fuera[0].(pagina.ErrorServicio); !ok {
		t.Fatalf("%v", fuera[0])
	}
	if v := q.Vista(); !strings.Contains(v, "El servicio no responde · r reintenta") {
		t.Fatalf("la página lo dice y sigue:\n%s", v)
	}
	if q.Titulo() != "Inicio" {
		t.Fatal("no se cerró")
	}
}

// TestRespuestasViejas: una respuesta de otro espacio, de un contexto cancelado o de una versión anterior de la página se
// descarta.
func TestRespuestasViejas(t *testing.T) {
	p, _, cli, id := abierta(t)
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Base: cli.Base, Trabajos: map[string]int{"completado": 1}}})
	vieja := p.origenActual()
	vieja.instancia = "otro-" + id
	p.Update(msgInstancia{origen: vieja, r: cliente.ResumenInstancia{Trabajos: map[string]int{"completado": 99}}})
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	cancelada := p.origenActual()
	cancelada.ctx = ctx
	p.Update(msgInstancia{origen: cancelada, r: cliente.ResumenInstancia{Trabajos: map[string]int{"completado": 98}}})
	anterior := p.origenActual()
	anterior.gen--
	p.Update(msgInstancia{origen: anterior, r: cliente.ResumenInstancia{Trabajos: map[string]int{"completado": 97}}})
	p.Update(msgRed{origen: vieja, tipo: "trabajo", ev: ev()})
	if p.inst.Trabajos["completado"] != 1 || len(p.red) != 0 {
		t.Fatalf("se descartan: %v %v", p.inst.Trabajos, p.red)
	}
	p.Update(msgTrabajos{origen: cancelada})
	// Y otros mensajes que no son suyos no le hacen nada.
	if _, cmd := p.Update(comp.Cancelado{}); cmd != nil {
		t.Fatal("ignora lo que no es suyo")
	}
}

// TestCabeEn60x20: con todos los avisos, 12 filas y los pasos, la vista mide como mucho el área y nada se sale por el
// ancho, también con tildes; desplazar llega hasta el final.
func TestCabeEn60x20(t *testing.T) {
	p, r, cli, _ := abierta(t)
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Base: cli.Base, AppConClave24h: 50, Trabajos: map[string]int{"completado": 12345}}})
	p.Update(msgEspacios{origen: p.origenActual(), lista: []contrato.Espacio{{Nombre: "Notas con un nombre muy largo y tildes áéíóú", UltimoError: "x"}}})
	p.Update(msgConversor{origen: p.origenActual(), e: contrato.EstadoConversor{Estado: "error", Mensaje: "sin red"}})
	p.Update(msgAgentes{origen: p.origenActual(), lista: []adaptador{{Agente: "claude"}, {Agente: "codex", Desconectado: true}}})
	var recientes []nucleo.Trabajo
	for i := 0; i < 5; i++ {
		recientes = append(recientes, trabajoDe(fmt.Sprint("r", i), "una pregunta bastante larga con tildes áéíóú que no cabe en una sola línea de sesenta columnas", "claude", "claude/modelo-con-nombre-largo", "completado", r.t))
	}
	p.Update(msgRecientes{origen: p.origenActual(), lista: recientes})
	for i := 0; i < 14; i++ {
		r.t = r.t.Add(time.Second)
		p.evento(ev(con(fmt.Sprintf("t%02d", i), "validando_salida", "validacion"), func(e *evento) {
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
	if v := p.Vista(); !strings.HasPrefix(v, "Inicio") {
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

// TestTextosComoLaWeb: los nombres salen de los textos de la Web UI, en el idioma de la sesión.
func TestTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p, _, _ := nueva("en")
	c := contexto(cli, id)
	c.Idioma, c.Alto = "en", 40
	correr(p, p.Iniciar(c))
	t.Cleanup(p.Ocultar)
	p.evento(ev(con("a", "ejecutando", "ejecucion")))
	v := p.Vista()
	for _, quiere := range []string{"Home", "Your apps connect to", "Your network", "Departures", "Completed", "In progress", "With errors", "Getting started",
		"Connect at least one agent", "1 of 1 agents ready", "Try an agent", "Connect your app", "running"} {
		if !strings.Contains(v, quiere) && !strings.Contains(strings.ToLower(v), strings.ToLower(quiere)) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
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
	p.Update(msgRecientes{origen: p.origenActual(), lista: lista})
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
	p.Update(msgRecientes{origen: p.origenActual(), lista: lista[:1]})
	if v := p.Vista(); !strings.Contains(v, r.t.Add(-10*24*time.Hour).Local().Format("2006-01-02")) {
		t.Fatalf("más de una semana, la fecha:\n%s", v)
	}
}

// TestPeticionesRecientesDelServicio: Iniciar las pide al servicio (las 5 últimas) y el refresco las pone al día.
func TestPeticionesRecientesDelServicio(t *testing.T) {
	cli, id := prueba.Servicio(t)
	for i := 0; i < 6; i++ {
		j, err := cli.Crear(context.Background(), id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: fmt.Sprint("mensaje número ", i)}}})
		if err != nil {
			t.Fatal(err)
		}
		for limite := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
			if g, err := cli.Trabajo(context.Background(), id, j.ID); err == nil && g.Estado.Terminal() {
				break
			}
			if time.Now().After(limite) {
				t.Fatal("el trabajo no terminó")
			}
		}
	}
	p, _, _ := nueva("es")
	p.c.Alto = 60
	correr(p, p.Iniciar(contexto(cli, id)))
	t.Cleanup(p.Ocultar)
	p.c.Alto = 60
	if n := len(p.recientes); n != 5 {
		t.Fatalf("pide 5: %d", n)
	}
	if v := p.Vista(); !strings.Contains(v, "mensaje número 5") || strings.Contains(v, "mensaje número 0") {
		t.Fatalf("las más nuevas primero:\n%s", v)
	}
}

// TestPasosConSuAtajo: cada paso dice con qué tecla (1 a 7) se va a su página, como el botón de la Web UI.
func TestPasosConSuAtajo(t *testing.T) {
	p, _, _, _ := abierta(t)
	p.c.Alto = 60
	v := p.Vista()
	for _, quiere := range []string{"2 Agentes", "3 Probar", "4 Mis APIs", "7 Ajustes"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	for clave, quiere := range map[string]string{"inicio": "1", "agentes": "2", "ejecutar": "3", "conectar": "4", "historial": "5", "espacios": "6", "configuracion": "7"} {
		if Atajo(clave) != quiere {
			t.Fatalf("el atajo de %s es %s, no %s", clave, quiere, Atajo(clave))
		}
	}
}

// TestSoloAscii: con TRAMTAI_ASCII=1 (o una consola que no es UTF-8) la página no usa caracteres de dibujo.
func TestSoloAscii(t *testing.T) {
	p, r, cli, _ := abierta(t)
	p.estilos.Iconos, p.estilos.ASCII = pagina.IconosASCII, true
	p.Update(msgInstancia{origen: p.origenActual(), r: cliente.ResumenInstancia{Base: cli.Base, AppConClave24h: 30, Trabajos: map[string]int{"completado": 1}}})
	for i, estado := range []string{"ejecutando", "completado", "fallido", "cancelado"} {
		p.evento(ev(con(fmt.Sprint(i), estado, etapaDe(estado)), func(e *evento) { e.Creado = r.t }))
	}
	for _, desp := range []int{0, 99} {
		p.desp = desp
		for _, c := range p.Vista() {
			if c > 127 && !strings.ContainsRune("áéíóúüñÁÉÍÓÚÜÑ¿¡×", c) {
				t.Fatalf("sale %q en ASCII:\n%s", c, p.Vista())
			}
		}
	}
	for _, t2 := range p.Teclas() {
		if strings.ContainsAny(t2.Tecla, "↑↓") {
			t.Fatalf("la tecla %q no es ASCII", t2.Tecla)
		}
	}
}

func TestDesplaza(t *testing.T) {
	p, _, _, _ := abierta(t)
	p.c.Alto = 6
	p.Update(tecla("down"))
	if p.desp != 1 {
		t.Fatalf("↓ baja una línea: %d", p.desp)
	}
	p.Update(tecla("up"))
	p.Update(tecla("up"))
	if p.desp != 0 {
		t.Fatalf("no sube más allá del principio: %d", p.desp)
	}
	p.Update(tecla("end"))
	fin := p.desp
	p.Update(tecla("down"))
	if p.desp != fin || fin == 0 {
		t.Fatalf("no baja más allá del final: %d", p.desp)
	}
}

// TestOcultarYMostrar: Ocultar suelta el flujo y los temporizadores (sus mensajes ya no valen) y Mostrar los recupera.
func TestOcultarYMostrar(t *testing.T) {
	p, r, _, _ := abierta(t)
	p.evento(ev(con("a", "ejecutando", "ejecucion")))
	viejo := p.origenActual()
	p.Ocultar()
	if _, cmd := p.Update(msgTic{origen: viejo}); cmd != nil {
		t.Fatal("un temporizador de antes de ocultar no se vuelve a armar")
	}
	if _, cmd := p.Update(msgRed{origen: viejo, tipo: "trabajo", ev: ev(con("b", "ejecutando", "ejecucion"))}); cmd != nil || len(p.red) != 1 {
		t.Fatal("el flujo de antes ya no vale")
	}
	r.t = r.t.Add(time.Minute)
	correr(p, p.Mostrar())
	hasta(t, p, func() bool { return p.enVivo })
	if len(p.red) != 0 {
		t.Fatalf("al volver, la foto pone la red al día: %v", p.red)
	}
	if v := p.Vista(); strings.Contains(v, "Cargando…") {
		t.Fatalf("Mostrar conserva lo cargado:\n%s", v)
	}
}

// TestContadoresLlevanAlHistorial: cada contador abre el Historial con su estado (curso, ok o error), como los de Panel.vue,
// y la fila dice con qué tecla.
func TestContadoresLlevanAlHistorial(t *testing.T) {
	p, _, _, _ := abierta(t)
	v := p.Vista()
	for _, quiere := range []string{"o Completadas", "e En proceso", "f Con error"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("la fila dice su tecla, falta %q:\n%s", quiere, v)
		}
	}
	for tecla_, estado := range map[string]string{"o": "ok", "e": "curso", "f": "error"} {
		_, cmd := p.Update(tecla(tecla_))
		if cmd == nil {
			t.Fatalf("%s no hace nada", tecla_)
		}
		ir, ok := cmd().(pagina.IrA)
		if !ok || ir.Pagina != pagina.PaginaHistorial || ir.Datos != (pagina.FiltroEstado{Estado: estado}) {
			t.Fatalf("%s lleva al Historial con el estado %q: %+v", tecla_, estado, ir)
		}
	}
}
