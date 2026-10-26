package inicio_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/inicio"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de Inicio van por su interfaz: teclas, los mensajes de pagina y lo que enseña Vista(), contra el servicio de
// prueba. El reloj es el de la prueba (prueba.Reloj) y, donde hace falta un flujo que se corta a voluntad, el flujo del
// espacio es un guion de la prueba.

var ctx = context.Background()

// nueva crea Inicio con un reloj de prueba.
func nueva(lengua string) (pagina.Pagina, *prueba.Reloj) {
	r := prueba.NuevoReloj()
	return inicio.NuevaConReloj(lengua, r.Ahora, r.Espera), r
}

// abrir inicia Inicio con ese contexto (y ese flujo, si no es nil): arranca con «Cargando…» y espera lo del espacio.
func abrir(t *testing.T, c pagina.Contexto, f *ensayo) (*prueba.App, *prueba.Reloj) {
	t.Helper()
	p, r := nueva(c.Idioma)
	if f != nil {
		inicio.ConFlujo(p, f.abrir)
	}
	a := prueba.Conducir(t, p)
	cmd := p.Iniciar(c)
	if v := p.Vista(); c.Cli != nil && !strings.Contains(v, strings.TrimRight(datos.Texto(c.Idioma, "comun.cargando"), ".…")) {
		t.Fatalf("arranca con «Cargando…»:\n%s", v)
	}
	a.Correr(cmd)
	cargada(a, c)
	return a, r
}

// cargada espera a lo del espacio y a la comprobación de los agentes, que llega aparte con los Primeros pasos: hasta
// entonces la red y los pasos dicen «Comprobando…». Lo demás que trae el servicio tras una tecla se espera con Hasta o
// HastaVer antes de mirarlo (regla 20): Asentar no ve los temporizadores del servicio ni todas las esperas de E/S.
func cargada(a *prueba.App, c pagina.Contexto) {
	a.EsperarCarga()
	if c.Cli != nil {
		comprobando := strings.TrimRight(datos.Texto(c.Idioma, "inicio.comprobando"), ".…")
		a.Hasta(func() bool { return !strings.Contains(a.Vista(), comprobando) })
	}
}

// alto es el contexto de la prueba con ese alto.
func alto(t *testing.T, cli *cliente.Cliente, id string, filas int) pagina.Contexto {
	c := prueba.Contexto(t, cli, id)
	c.Alto = filas
	return c
}

// abierta abre Inicio contra el servicio real, con su flujo.
func abierta(t *testing.T) (*prueba.App, *prueba.Reloj, *cliente.Cliente, string) {
	t.Helper()
	cli, id := prueba.Servicio(t)
	a, r := abrir(t, prueba.Contexto(t, cli, id), nil)
	return a, r, cli, id
}

// irA es el IrA entre lo que salió hacia la app.
func irA(fuera []tea.Msg) (pagina.IrA, bool) {
	for _, m := range fuera {
		if ir, ok := m.(pagina.IrA); ok {
			return ir, true
		}
	}
	return pagina.IrA{}, false
}

// destino es la página a la que lleva lo que salió hacia la app.
func destino(fuera []tea.Msg) string {
	ir, _ := irA(fuera)
	return ir.Pagina
}

// sistema es el nombre del sistema operativo en el paso de arrancar con él.
func sistema() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	}
	return "Linux"
}

// ensayo es un flujo del espacio de mentira: cada vez que la página lo abre corre el siguiente paso del guion.
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

// trabajo es un evento «trabajo» del flujo.
func trabajo(e contrato.EventoTrabajo) []byte {
	datos, _ := json.Marshal(e)
	return datos
}

// esperarFin espera hasta que se corte el contexto, como un flujo que sigue abierto.
func esperarFin(ctx context.Context, _ func(string, []byte) error) error {
	<-ctx.Done()
	return ctx.Err()
}

// reconexiones son las esperas de la reconexión que ha pedido la página (sin la limpieza de la red ni el refresco).
func reconexiones(r *prueba.Reloj) []time.Duration {
	var l []time.Duration
	for _, d := range r.Pedidas() {
		if d != inicio.CadaLimpieza && d != inicio.CadaRefresco {
			l = append(l, d)
		}
	}
	return l
}

// pasarAlReconectar espera a que la página pida su espera de reconexión número n y la hace pasar.
func pasarAlReconectar(t *testing.T, a *prueba.App, r *prueba.Reloj, n int) {
	t.Helper()
	a.Hasta(func() bool { return len(reconexiones(r)) >= n })
	r.Pasar(reconexiones(r)[n-1])
	a.Asentar()
}

func TestInicioCarga(t *testing.T) {
	a, _, cli, _ := abierta(t)
	for _, quiere := range []string{"Tus apps se conectan a " + cli.Base, "Red de Tramtai: 1 origen y 1 de 1", "Primeros pasos", "Completadas", "En curso", "Con error",
		"1 de 1 agentes listos"} {
		if !strings.Contains(a.Vista(), quiere) {
			// Los pasos están más abajo: se ven al desplazar.
			a.Pulsar("end")
			if !strings.Contains(a.Vista(), quiere) {
				t.Fatalf("falta %q:\n%s", quiere, a.Vista())
			}
			a.Pulsar("home")
		}
	}
	if strings.HasPrefix(a.Vista(), "Inicio") {
		t.Fatalf("el título ya está en la cabecera de la app:\n%s", a.Vista())
	}
	if a.P.Titulo() != "Inicio" || a.P.Ayuda() != "inicio" || a.P.EnCampo() {
		t.Fatalf("título %q, ayuda %q", a.P.Titulo(), a.P.Ayuda())
	}
}

// TestRedAparecenYSeVan: una petición aparece en la red con su API, su agente, su etapa y su tiempo, y las que terminan
// se quedan 2 s (4 si fallan) y se van. El flujo es el del servicio real; el reloj, el de la prueba.
func TestRedAparecenYSeVan(t *testing.T) {
	cli, id := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"roto": adaptadores.Simulado{Error: errors.New("falla")}})
	hola := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}
	if _, err := cli.Crear(ctx, id, hola); err != nil {
		t.Fatal(err)
	}
	a, r := abrir(t, prueba.Contexto(t, cli, id), nil)
	a.HastaVer("terminada")
	for _, quiere := range []string{"Clave del espacio", "simulado", "terminada", "0 s"} {
		if !strings.Contains(a.Vista(), quiere) {
			t.Fatalf("falta %q:\n%s", quiere, a.Vista())
		}
	}
	// Llegó: sigue 2 s y luego se va.
	r.Pasar(1999 * time.Millisecond)
	if a.Asentar(); !strings.Contains(a.Vista(), "terminada") {
		t.Fatalf("a los 1999 ms sigue:\n%s", a.Vista())
	}
	r.Pasar(inicio.CadaLimpieza)
	if a.Asentar(); strings.Contains(a.Vista(), "terminada") {
		t.Fatalf("pasados los 2 s se va:\n%s", a.Vista())
	}
	// Una que falla se queda 4 s.
	if _, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "roto", Mensajes: hola.Mensajes}); err != nil {
		t.Fatal(err)
	}
	a.HastaVer("falló")
	r.Pasar(3999 * time.Millisecond)
	if a.Asentar(); !strings.Contains(a.Vista(), "falló") {
		t.Fatalf("a los 3999 ms se ve que falló:\n%s", a.Vista())
	}
	r.Pasar(inicio.CadaLimpieza)
	if a.Asentar(); strings.Contains(a.Vista(), "falló") {
		t.Fatalf("pasados los 4 s se va:\n%s", a.Vista())
	}
	// Una petición nueva del servicio aparece en vivo.
	if _, err := cli.Crear(ctx, id, hola); err != nil {
		t.Fatal(err)
	}
	a.HastaVer("terminada")
}

// TestRedComoMucho12: la red enseña como mucho 12 filas, las más recientes, y dice cuántas quedan fuera.
func TestRedComoMucho12(t *testing.T) {
	cli, id := prueba.Servicio(t)
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(ctx context.Context, al func(string, []byte) error) error {
			for i := 0; i < 15; i++ {
				_ = al("trabajo", trabajo(contrato.EventoTrabajo{ID: fmt.Sprintf("t%02d", i), Estado: "ejecutando", Etapa: "ejecucion", Agente: "claude",
					API: fmt.Sprintf("api%02d", i), Creado: time.Now().Add(time.Duration(i) * time.Second)}))
			}
			foto(al)
			return esperarFin(ctx, al)
		},
	}}
	a, _ := abrir(t, alto(t, cli, id, 40), f)
	a.HastaVer("api14")
	v := a.Vista()
	filas := 0
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "en ejecución") {
			filas++
		}
	}
	if filas != 12 || !strings.Contains(v, "+3") || strings.Contains(v, "api02 ") || !strings.Contains(v, "15 peticiones en curso.") {
		t.Fatalf("%d filas:\n%s", filas, v)
	}
}

// TestRedReconecta: si el flujo se corta, la página vuelve a conectar con las esperas de vivo.ts (1 s, 2 s…), un flujo
// mudo (ErrSinActividad) cuenta como cortado, y la foto del flujo nuevo quita lo que ya no existe.
func TestRedReconecta(t *testing.T) {
	cli, id := prueba.Servicio(t)
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(_ context.Context, al func(string, []byte) error) error { // 1: manda uno y la foto, y se corta
			_ = al("trabajo", []byte(`{"id":"a","estado":"ejecutando","etapa":"ejecucion","agente":"codex","api":"de-la-prueba"}`))
			foto(al)
			return errors.New("se cortó")
		},
		func(context.Context, func(string, []byte) error) error { return cliente.ErrSinActividad }, // 2: ni la foto
		func(_ context.Context, al func(string, []byte) error) error { // 3: vuelve, sin el trabajo a, y se corta otra vez
			foto(al)
			return errors.New("se cortó")
		},
		func(ctx context.Context, al func(string, []byte) error) error { // 4: vuelve y sigue
			foto(al)
			return esperarFin(ctx, al)
		},
	}}
	a, r := abrir(t, prueba.Contexto(t, cli, id), f)
	a.HastaVer("de-la-prueba", "Sin conexión en vivo")
	pasarAlReconectar(t, a, r, 1)
	pasarAlReconectar(t, a, r, 2)
	pasarAlReconectar(t, a, r, 3) // tras una foto, la cuenta de cortes vuelve a empezar: otra vez 1 s
	a.Hasta(func() bool {
		n, _ := f.cuenta()
		v := a.Vista()
		return n == 4 && !strings.Contains(v, "Sin conexión en vivo") && !strings.Contains(v, "de-la-prueba")
	})
	if got := reconexiones(r); !slices.Equal(got, []time.Duration{time.Second, 2 * time.Second, time.Second}) {
		t.Fatalf("esperas %v", got)
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
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(_ context.Context, al func(string, []byte) error) error {
			_ = al("trabajo", []byte(`{"id":"fantasma","estado":"ejecutando","etapa":"ejecucion","agente":"codex","api":"fantasma"}`))
			return errors.New("sin flujo")
		},
		func(context.Context, func(string, []byte) error) error { return errors.New("sin flujo") },
	}}
	a, r := abrir(t, prueba.Contexto(t, cli, id), f)
	a.HastaVer("fantasma", "Sin conexión en vivo")
	pasarAlReconectar(t, a, r, 1) // 1 s
	pasarAlReconectar(t, a, r, 2) // 2 s: van 3 cortes
	a.Hasta(func() bool { n, _ := f.cuenta(); return n == 3 && len(reconexiones(r)) == 3 })
	if !strings.Contains(a.Vista(), "fantasma") {
		t.Fatalf("sin consulta, la fila sigue:\n%s", a.Vista())
	}
	r.Pasar(2 * time.Second) // a los 5 s, el refresco consulta los trabajos
	a.Asentar()
	a.Hasta(func() bool {
		v := a.Vista()
		return !strings.Contains(v, "fantasma") && strings.Contains(v, "Sin conexión en vivo")
	})
	if strings.Contains(a.Vista(), "fantasma") || !strings.Contains(a.Vista(), "Sin conexión en vivo") {
		t.Fatalf("la consulta no trae el trabajo fantasma:\n%s", a.Vista())
	}
}

// TestRedSeCierraAlSalir: el flujo se cierra al salir de Inicio (Ocultar), al cancelarse el contexto de la página
// (cambio de espacio, reinicio) y se vuelve a abrir al volver (Mostrar); otro Iniciar cierra el de antes.
func TestRedSeCierraAlSalir(t *testing.T) {
	abiertos := func(a *prueba.App, n int) {
		t.Helper()
		a.Hasta(func() bool { return prueba.FlujosAbiertos() == n })
	}
	cli, id := prueba.Servicio(t)
	c := prueba.Contexto(t, cli, id)
	vivo, cancelar := context.WithCancel(c.Ctx)
	c.Ctx = vivo
	a, _ := abrir(t, c, nil)
	abiertos(a, 1)
	a.P.(pagina.Ocultable).Ocultar()
	abiertos(a, 0)
	a.Correr(a.P.(pagina.Ocultable).Mostrar())
	abiertos(a, 1)
	cancelar() // el contexto de la página: cambió el espacio o se reinició
	abiertos(a, 0)

	// Otro Iniciar cancela el flujo de antes y abre el suyo.
	cli2, id2 := prueba.Servicio(t)
	b, _ := abrir(t, prueba.Contexto(t, cli2, id2), nil)
	abiertos(b, 1)
	b.Correr(b.P.Iniciar(prueba.Contexto(t, cli2, id2)))
	abiertos(b, 1)
	b.P.(pagina.Ocultable).Ocultar()
	abiertos(b, 0)
}

// TestAvisoApiPropia: con más de 20 peticiones de apps con la clave del espacio en 24 h se sugiere una API propia, y c
// lleva a Conectar. Con 20 o menos no se dice nada.
func TestAvisoApiPropia(t *testing.T) {
	a, _, cli, id := abierta(t)
	if strings.Contains(a.Vista(), "con la clave del espacio") {
		t.Fatalf("sin peticiones no hay aviso:\n%s", a.Vista())
	}
	clave, err := cli.RotarClave(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	comoApp(t, cli, id, clave, 20)
	a.Correr(a.P.Iniciar(prueba.Contexto(t, cli, id)))
	cargada(a, prueba.Contexto(t, cli, id))
	if strings.Contains(a.Vista(), "con la clave del espacio") {
		t.Fatal("20 no basta")
	}
	if fuera := a.Pulsar("c"); len(fuera) != 0 {
		t.Fatal("sin aviso, c no hace nada")
	}
	comoApp(t, cli, id, clave, 1)
	a.Correr(a.P.Iniciar(prueba.Contexto(t, cli, id)))
	a.HastaVer("21 peticiones de apps llegaron con la clave del espacio", "últimas 24 h", "Crear una API")
	v := a.Vista()
	for _, quiere := range []string{"21 peticiones de apps llegaron con la clave del espacio", "últimas 24 h", "Crear una API"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	if d := destino((a.Pulsar("c"))); d != pagina.PaginaConectar {
		t.Fatalf("c lleva a Conectar: %q", d)
	}
}

// comoApp manda n peticiones como una app con la clave del espacio y espera a que terminen: mientras corren, la red de
// Inicio no deja de cambiar y la página no se asienta (en un runner lento, más de los 10 s de Asentar).
func comoApp(t *testing.T, cli *cliente.Cliente, id, clave string, n int) {
	t.Helper()
	for range n {
		req, _ := http.NewRequest("POST", cli.Base+"/api/v1/ejecutar", strings.NewReader(`{"mensajes":[{"rol":"usuario","contenido":"hola"}]}`))
		req.Header.Set("Authorization", "Bearer "+clave)
		r, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
	}
	for fin := time.Now().Add(30 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		trabajos, err := cli.Trabajos(ctx, id, 100)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.ContainsFunc(trabajos, func(tr nucleo.Trabajo) bool { return !tr.Estado.Terminal() }) {
			return
		}
		if time.Now().After(fin) {
			t.Fatalf("las %d peticiones de la app no terminan", n)
		}
	}
}

// pasosDe abre Inicio contra un servicio con Claude listo y Codex desconectado, una API y el arranque automático
// del sistema como diga auto; con completada, también una petición hecha. Los pasos los calcula el servicio.
func pasosDe(t *testing.T, auto func() (bool, bool), completada bool) *prueba.App {
	t.Helper()
	cli, id := prueba.Ajustado(t, func(o *servicio.Opciones) {
		o.Registro["claude"], o.Registro["codex"] = prueba.Detectado{}, prueba.Detectado{}
		o.Autoarranque = auto
	})
	if err := cli.ConectarAgente(ctx, "codex", false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cli.CrearPunto(ctx, id, contrato.ConfigPunto{Nombre: "mi-app", Agente: "simulado"}); err != nil {
		t.Fatal(err)
	}
	if completada {
		j, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}})
		if err == nil {
			err = cli.Eventos(ctx, id, j.ID, 0, func(int, string, string) error { return nil })
		}
		if err != nil {
			t.Fatalf("la petición no terminó: %v", err)
		}
	}
	a, _ := abrir(t, prueba.Contexto(t, cli, id), nil)
	a.Pulsar("end")
	return a
}

// TestPrimerosPasos: los 4 pasos con su estado, y «Ver la Ayuda de Empezar» abre el tema que acompaña a Inicio.
func TestPrimerosPasos(t *testing.T) {
	a := pasosDe(t, func() (bool, bool) { return true, false }, false)
	v := a.Vista()
	for _, quiere := range []string{"Primeros pasos", "✓ Conecta al menos un agente", "2 de 3 agentes listos", "○ Prueba un agente", "✓ Conecta tu aplicación",
		"○ Arranca Tramtai con " + sistema(), "Tus apps funcionan al encender el equipo", "2 de 4 pasos hechos"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	// Sin arranque automático en este sistema, el servicio manda 3 pasos.
	if v := pasosDe(t, func() (bool, bool) { return false, false }, false).Vista(); strings.Contains(v, "Arranca Tramtai") || !strings.Contains(v, "2 de 3 pasos hechos") {
		t.Fatalf("tres pasos:\n%s", v)
	}
	tema := ""
	for _, x := range datos.Ayuda("es") {
		if x.ID == "empezar" {
			tema = x.Pagina
		}
	}
	if tema != "inicio" || !strings.Contains(a.Vista(), "Ver la Ayuda de Empezar") {
		t.Fatalf("el tema empezar acompaña a %q:\n%s", tema, a.Vista())
	}
	ir, ok := irA((a.Pulsar("a")))
	if d, _ := ir.Datos.(pagina.ConAyuda); !ok || ir.Pagina != pagina.PaginaAyuda || d.Pagina != tema {
		t.Fatalf("a abre la Ayuda de Empezar: %+v", ir)
	}
	if d := destino((a.Pulsar("g"))); d != pagina.PaginaAgentes {
		t.Fatalf("g lleva a Agentes: %q", d)
	}
	// Con todo hecho, la guía se pliega en «Todo listo».
	if v := pasosDe(t, func() (bool, bool) { return true, true }, true).Vista(); !strings.Contains(v, "Todo listo") || strings.Contains(v, "○") {
		t.Fatalf("todo listo:\n%s", v)
	}
}

// lento es un agente que, mientras bloquear esté puesto, no termina de comprobarse hasta que la prueba lo suelta.
type lento struct {
	adaptadores.Simulado
	bloquear *atomic.Bool
	soltar   chan struct{}
}

func (l lento) Detectar(ctx context.Context) adaptadores.Estado {
	if l.bloquear.Load() {
		select {
		case <-l.soltar:
		case <-ctx.Done():
		}
	}
	return adaptadores.Estado{}
}

// TestPrimerosPasosComprobando: mientras el servicio comprueba los agentes, la red y los Primeros pasos dicen
// «Comprobando…»; al llegar la comprobación, sus pasos.
func TestPrimerosPasosComprobando(t *testing.T) {
	l := lento{bloquear: &atomic.Bool{}, soltar: make(chan struct{})}
	cli, id := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"lento": l})
	l.bloquear.Store(true)
	t.Cleanup(func() { l.bloquear.Store(false) })
	p, _ := nueva("es")
	a := prueba.Conducir(t, p)
	a.Lanzar(p.Iniciar(alto(t, cli, id, 60)))
	a.HastaVer("Tus apps se conectan a")
	if v := a.Vista(); strings.Count(v, "Comprobando…") != 2 {
		t.Fatalf("la red y los pasos esperan a la comprobación:\n%s", v)
	}
	close(l.soltar)
	a.HastaVer("Conecta al menos un agente")
	if strings.Contains(a.Vista(), "Comprobando…") {
		t.Fatalf("ya comprobados:\n%s", a.Vista())
	}
}

// TestContadores: los contadores son los que agrupa el servicio (contadores del resumen), con los nombres de la Web UI; el
// periodo sale de la retención del servicio.
func TestContadores(t *testing.T) {
	a, r, cli, id := abierta(t)
	v := a.Vista()
	for _, quiere := range []string{"Completadas", "respondidas correctamente", "En curso", "Con error", "los últimos 7 días"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	// Un trabajo hecho suma uno a las completadas cuando la página se refresca.
	j, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}})
	if err == nil {
		err = cli.Eventos(ctx, id, j.ID, 0, func(int, string, string) error { return nil })
	}
	if err != nil {
		t.Fatal(err)
	}
	r.Pasar(inicio.CadaRefresco)
	a.Asentar()
	a.HastaVer("Completadas", " 1")
	if !strings.Contains(a.Vista(), "Completadas") || !strings.Contains(a.Vista(), " 1") {
		t.Fatalf("suma uno:\n%s", a.Vista())
	}
	// Con la retención de un día, «el último día».
	cli2, id2 := prueba.Ajustado(t, func(o *servicio.Opciones) {
		o.Config.Fijar(configuracion.OrigenGlobal, map[string]any{"retencion.horas_metadatos": 24})
	})
	b, _ := abrir(t, prueba.Contexto(t, cli2, id2), nil)
	b.HastaVer("el último día") // la retención llega aparte de la carga
	if !strings.Contains(b.Vista(), "el último día") {
		t.Fatalf("el periodo sale de la retención:\n%s", b.Vista())
	}
}

func TestEscVuelve(t *testing.T) {
	a, _, _, _ := abierta(t)
	if fuera := a.Pulsar("esc"); len(fuera) != 1 || fuera[0] != (pagina.Volver{}) {
		t.Fatalf("Esc sin nada abierto vuelve: %v", fuera)
	}
}

// TestServicioCaido: sin cliente, o con el servicio parado a mitad, la página avisa y no se cierra.
func TestServicioCaido(t *testing.T) {
	a, _ := abrir(t, prueba.Contexto(t, nil, "global"), nil)
	if len(a.Fuera) != 1 {
		t.Fatalf("sin cliente: %v", a.Fuera)
	}
	if e, ok := a.Fuera[0].(pagina.ErrorServicio); !ok || !errors.Is(e.Err, cliente.ErrServicioInactivo) {
		t.Fatalf("sin cliente: %v", a.Fuera[0])
	}
	if v := a.Vista(); !strings.Contains(v, "El servicio no responde · r reintenta") && !strings.Contains(v, "Cargando…") {
		t.Fatalf("sin cliente la página sigue:\n%s", v)
	}
	if a.P.(pagina.Ocultable).Ocultar(); a.P.(pagina.Ocultable).Mostrar() != nil {
		t.Fatal("sin cliente, volver a Inicio no pide nada")
	}

	cli, id, cerrar := prueba.Arrancar(t)
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{esperarFin}} // cerrar el servicio no espera al flujo
	q, r := abrir(t, prueba.Contexto(t, cli, id), f)
	cerrar()
	r.Pasar(inicio.CadaRefresco)
	q.Asentar()
	q.Hasta(func() bool { return len(q.Fuera) != 0 })
	if len(q.Fuera) == 0 {
		t.Fatal("con el servicio parado, el refresco avisa")
	}
	if _, ok := q.Fuera[0].(pagina.ErrorServicio); !ok {
		t.Fatalf("%v", q.Fuera[0])
	}
	if v := q.Vista(); !strings.Contains(v, "El servicio no responde · r reintenta") {
		t.Fatalf("la página lo dice y sigue:\n%s", v)
	}
	if q.P.Titulo() != "Inicio" {
		t.Fatal("no se cerró")
	}
}

// TestRespuestasViejas: lo que la página pidió y llega cuando ya es de otra versión de la página, de otro espacio o de un
// contexto cancelado se descarta.
func TestRespuestasViejas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	hecha := func(texto string) {
		t.Helper()
		j, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}})
		if err == nil {
			err = cli.Eventos(ctx, id, j.ID, 0, func(int, string, string) error { return nil })
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Run("otra versión", func(t *testing.T) {
		// Un flujo callado: la petición nueva no llega por él, solo por la carga.
		callado := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
			func(ctx context.Context, al func(string, []byte) error) error { foto(al); return esperarFin(ctx, al) }}}
		a, _ := abrir(t, alto(t, cli, id, 60), callado)
		viejos := a.Retener(func() { a.Pulsar("r") }) // se pide antes de ocultar y responde con la petición nueva
		a.P.(pagina.Ocultable).Ocultar()
		// Lo vigente cambia algo que se ve: Mostrar trae una petición hecha antes, con su cuenta, y se espera a verla. Así
		// nada de Mostrar llega después de «antes» y no hace falta esperar a que no pase nada.
		hecha("previa")
		a.Correr(a.P.(pagina.Ocultable).Mostrar())
		a.Hasta(func() bool {
			for _, l := range strings.Split(a.Vista(), "\n") {
				if strings.Contains(l, "Completadas") && strings.HasSuffix(strings.TrimSpace(l), " 1") {
					return strings.Contains(a.Vista(), "previa")
				}
			}
			return false
		})
		hecha("otra")
		antes := a.Vista()
		if fuera := a.Soltar(viejos); a.Vista() != antes || len(fuera) != 0 {
			t.Fatalf("se descarta: %v\n%s\nantes:\n%s", fuera, a.Vista(), antes)
		}
	})
	t.Run("otro espacio", func(t *testing.T) {
		a, _ := abrir(t, alto(t, cli, id, 60), nil)
		viejos := a.Retener(func() { a.Pulsar("r") })
		a.Correr(a.P.Iniciar(alto(t, cli, "otro", 60)))
		cargada(a, alto(t, cli, "otro", 60)) // lo que contesta el otro espacio, antes de mirar
		antes := a.Vista()
		if fuera := a.Soltar(viejos); a.Vista() != antes || len(fuera) != 0 {
			t.Fatalf("se descarta: %v\n%s\nantes:\n%s", fuera, a.Vista(), antes)
		}
	})
	t.Run("contexto cancelado", func(t *testing.T) {
		c := alto(t, cli, id, 60)
		vivo, cancelar := context.WithCancel(c.Ctx)
		c.Ctx = vivo
		a, _ := abrir(t, c, nil)
		viejos := a.Retener(func() { a.Pulsar("r") })
		cancelar()
		antes := a.Vista()
		if fuera := a.Soltar(viejos); a.Vista() != antes || len(fuera) != 0 {
			t.Fatalf("se descarta: %v\n%s\nantes:\n%s", fuera, a.Vista(), antes)
		}
	})
}

// TestTextosComoLaWeb: los nombres salen de los textos de la Web UI, en el idioma de la sesión.
func TestTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(ctx context.Context, al func(string, []byte) error) error {
			_ = al("trabajo", []byte(`{"id":"a","estado":"ejecutando","etapa":"ejecucion","agente":"claude"}`))
			foto(al)
			return esperarFin(ctx, al)
		},
	}}
	c := alto(t, cli, id, 40)
	c.Idioma = "en"
	a, _ := abrir(t, c, f)
	textos := []string{"Your apps connect to", "Your network", "Departures", "Completed", "In progress", "With errors", "Getting started",
		"Connect at least one agent", "1 of 1 agents ready", "Try an agent", "Connect your app", "running"}
	a.Hasta(func() bool {
		v := strings.ToLower(a.Vista())
		for _, quiere := range textos {
			if !strings.Contains(v, strings.ToLower(quiere)) {
				return false
			}
		}
		return true
	})
	v := a.Vista()
	for _, quiere := range textos {
		if !strings.Contains(v, quiere) && !strings.Contains(strings.ToLower(v), strings.ToLower(quiere)) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
}

// TestPeticionesRecientesDelServicio: Iniciar las pide al servicio (las 5 últimas), las más nuevas primero.
func TestPeticionesRecientesDelServicio(t *testing.T) {
	cli, id := prueba.Servicio(t)
	for i := 0; i < 6; i++ {
		j, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: fmt.Sprint("mensaje número ", i)}}})
		if err == nil {
			err = cli.Eventos(ctx, id, j.ID, 0, func(int, string, string) error { return nil })
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	a, _ := abrir(t, alto(t, cli, id, 60), nil)
	a.Hasta(func() bool { return strings.Count(a.Vista(), "mensaje número ") == 5 }) // las recientes llegan aparte de la carga
	v := a.Vista()
	if n := strings.Count(v, "mensaje número "); n != 5 {
		t.Fatalf("pide 5: %d\n%s", n, v)
	}
	if !strings.Contains(v, "mensaje número 5") || strings.Contains(v, "mensaje número 0") {
		t.Fatalf("las más nuevas primero:\n%s", v)
	}
}

// TestPasosConSuAtajo: cada paso dice con qué tecla (1 a 7) se va a su página, como el botón de la Web UI.
func TestPasosConSuAtajo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a, _ := abrir(t, alto(t, cli, id, 60), nil)
	v := a.Vista()
	for _, quiere := range []string{"2 Agentes", "3 Probar", "4 Mis APIs", "7 Ajustes"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	for clave, quiere := range map[string]string{"inicio": "1", "agentes": "2", "ejecutar": "3", "conectar": "4", "historial": "5", "espacios": "6", "configuracion": "7"} {
		if inicio.Atajo(clave) != quiere {
			t.Fatalf("el atajo de %s es %s, no %s", clave, quiere, inicio.Atajo(clave))
		}
	}
}

// TestSoloAscii: con TRAMTAI_ASCII=1 (o una consola que no es UTF-8) la página no usa caracteres de dibujo, ni con la
// sugerencia de una API propia ni con la red llena de estados.
func TestSoloAscii(t *testing.T) {
	cli, id := prueba.Servicio(t)
	clave, err := cli.RotarClave(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	comoApp(t, cli, id, clave, 30)
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(ctx context.Context, al func(string, []byte) error) error {
			for i, estado := range []string{"ejecutando", "completado", "fallido", "cancelado"} {
				_ = al("trabajo", trabajo(contrato.EventoTrabajo{ID: fmt.Sprint(i), Estado: estado, Etapa: nucleo.Estado(estado).Etapa(), Agente: "claude",
					Creado: time.Now()}))
			}
			foto(al)
			return esperarFin(ctx, al)
		},
	}}
	c := prueba.Contexto(t, cli, id)
	c.Estilos = &pagina.Estilos{ASCII: true, Iconos: pagina.IconosASCII}
	a, _ := abrir(t, c, f)
	a.HastaVer("Crear una API")
	for _, tecla := range []string{"home", "end"} {
		a.Pulsar(tecla)
		for _, r := range a.Vista() {
			if r > 127 && !strings.ContainsRune("áéíóúüñÁÉÍÓÚÜÑ¿¡×", r) {
				t.Fatalf("sale %q en ASCII:\n%s", r, a.Vista())
			}
		}
	}
	for _, t2 := range a.P.Teclas() {
		if strings.ContainsAny(t2.Tecla, "↑↓") {
			t.Fatalf("la tecla %q no es ASCII", t2.Tecla)
		}
	}
}

func TestDesplaza(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a, _ := abrir(t, alto(t, cli, id, 6), nil)
	arriba := a.Vista()
	segunda := strings.Split(arriba, "\n")[1]
	if a.Pulsar("down"); strings.Split(a.Vista(), "\n")[0] != segunda {
		t.Fatalf("↓ baja una línea: la primera es la que era segunda:\n%s", a.Vista())
	}
	if a.Pulsar("up", "up"); a.Vista() != arriba {
		t.Fatalf("no sube más allá del principio:\n%s", a.Vista())
	}
	a.Pulsar("end")
	fin := a.Vista()
	if a.Pulsar("down"); a.Vista() != fin || fin == arriba {
		t.Fatalf("no baja más allá del final:\n%s", a.Vista())
	}
}

// TestOcultarYMostrar: Ocultar suelta el flujo y los temporizadores (lo que llega de antes ya no vale ni se vuelve a armar)
// y Mostrar los recupera.
func TestOcultarYMostrar(t *testing.T) {
	cli, id := prueba.Servicio(t)
	otro := make(chan struct{})
	f := &ensayo{guion: []func(context.Context, func(string, []byte) error) error{
		func(ctx context.Context, al func(string, []byte) error) error { // 1: una en marcha y, cuando la prueba diga, otra
			_ = al("trabajo", []byte(`{"id":"a","estado":"ejecutando","etapa":"ejecucion","agente":"claude","api":"primera-api"}`))
			foto(al)
			select {
			case <-otro:
				_ = al("trabajo", []byte(`{"id":"b","estado":"ejecutando","etapa":"ejecucion","agente":"claude","api":"segunda-api"}`))
			case <-ctx.Done():
			}
			return esperarFin(ctx, al)
		},
		func(ctx context.Context, al func(string, []byte) error) error { // 2: al volver, la foto sin ninguna
			foto(al)
			return esperarFin(ctx, al)
		},
	}}
	a, r := abrir(t, prueba.Contexto(t, cli, id), f)
	a.HastaVer("primera-api")
	// El evento de b sale del flujo, pero la página se oculta antes de recibirlo.
	close(otro)
	a.EsperarLlegada()
	a.P.(pagina.Ocultable).Ocultar()
	if a.Asentar(); strings.Contains(a.Vista(), "segunda-api") {
		t.Fatalf("el flujo de antes ya no vale:\n%s", a.Vista())
	}
	// El temporizador de la limpieza vence con la página oculta: no se vuelve a armar.
	pedidas := len(r.Pedidas())
	r.Pasar(inicio.CadaLimpieza)
	if a.Asentar(); len(r.Pedidas()) != pedidas {
		t.Fatalf("un temporizador de antes de ocultar no se vuelve a armar: %v", r.Pedidas()[pedidas:])
	}
	a.Correr(a.P.(pagina.Ocultable).Mostrar())
	a.Hasta(func() bool { n, _ := f.cuenta(); return n == 2 && !strings.Contains(a.Vista(), "primera-api") })
	if v := a.Vista(); strings.Contains(v, "Cargando…") {
		t.Fatalf("Mostrar conserva lo cargado:\n%s", v)
	}
}

// TestContadoresLlevanAlHistorial: cada contador abre el Historial con su estado (curso, ok o error), como los de Panel.vue,
// y la fila dice con qué tecla.
func TestContadoresLlevanAlHistorial(t *testing.T) {
	a, _, _, _ := abierta(t)
	v := a.Vista()
	for _, quiere := range []string{"o Completadas", "e En curso", "f Con error"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("la fila dice su tecla, falta %q:\n%s", quiere, v)
		}
	}
	for tecla, estado := range map[string]string{"o": nucleo.FiltroCompletado, "e": nucleo.FiltroEnCurso, "f": nucleo.FiltroConError} {
		ir, ok := irA((a.Pulsar(tecla)))
		if !ok || ir.Pagina != pagina.PaginaHistorial || ir.Datos != (pagina.FiltroEstado{Estado: estado}) {
			t.Fatalf("%s lleva al Historial con el estado %q: %+v", tecla, estado, ir)
		}
	}
}
