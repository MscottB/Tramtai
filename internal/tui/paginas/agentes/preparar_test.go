package agentes_test

import (
	"maps"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/preparar"
	"tramtai/internal/servicio"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/paginas/agentes"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de preparar los CLI desde Agentes van contra el servicio real con una preparación que no instala nada
// (prueba.Preparacion): la prueba decide en qué estado está cada agente y lee lo que la página le pidió. El sondeo del
// estado va con un reloj de prueba.

var (
	sinSesion = detectado(adaptadores.Estado{Autenticacion: "suscripcion", Problema: "sin sesión", ProblemaCodigo: adaptadores.ProblemaFaltaClave})
	listoCLI  = detectado(adaptadores.Estado{Autenticacion: "suscripcion"})
)

// versiones es lo que dice GET /api/v1/agentes/versiones de un agente: lo que se puede hacer y si está comprobado.
func versiones(agente, instalada, ultima string, comprobadas map[string]bool, acciones ...string) contrato.VersionesAgente {
	return contrato.VersionesAgente{Agente: agente, Instalada: instalada, Ultima: ultima, Comprobadas: comprobadas, Acciones: acciones}
}

// preparando abre Agentes contra un servicio con esos agentes, la preparación falsa y esas versiones, y espera a verlas
// (el pie ofrece la acción de la fila elegida cuando ya llegaron).
func preparando(t *testing.T, mas map[string]adaptadores.Adaptador, antes func(*prueba.Preparacion), v ...contrato.VersionesAgente) (*prueba.App, *prueba.Preparacion, *prueba.Reloj) {
	t.Helper()
	f := prueba.NuevaPreparacion()
	f.FijarVersiones(v...)
	if antes != nil {
		antes(f)
	}
	cli, id := prueba.Ajustado(t, func(o *servicio.Opciones) {
		maps.Copy(o.Registro, mas)
		o.Preparacion = f
	})
	r := prueba.NuevoReloj()
	c := contexto(t, cli, id)
	a := prueba.Abrir(t, agentes.NuevaConReloj("es", r.Espera), c)
	a.Hasta(func() bool { return len(f.Forzados()) > 0 })
	a.Asentar()
	return a, f, r
}

// pie es el pie de la página: sus teclas con lo que hacen.
func pie(a *prueba.App) string {
	var l []string
	for _, k := range a.P.Teclas() {
		l = append(l, k.Tecla+" "+k.Accion)
	}
	return strings.Join(l, " · ")
}

// tic deja pasar el tiempo de un sondeo y entrega lo que traiga.
func tic(t *testing.T, a *prueba.App, r *prueba.Reloj) {
	t.Helper()
	a.Hasta(func() bool { return r.Esperando() > 0 })
	r.Pasar(agentes.CadaSondeo)
	a.Asentar()
}

func quiere(t *testing.T, v string, textos ...string) {
	t.Helper()
	for _, s := range textos {
		if !strings.Contains(v, s) {
			t.Errorf("falta %q:\n%s", s, v)
		}
	}
}

func noQuiere(t *testing.T, v string, textos ...string) {
	t.Helper()
	for _, s := range textos {
		if strings.Contains(v, s) {
			t.Errorf("sobra %q:\n%s", s, v)
		}
	}
}

// esperar espera a que la preparación falsa reciba n pedidos y los devuelve.
func esperar(t *testing.T, a *prueba.App, f *prueba.Preparacion, n int) []prueba.Pedido {
	t.Helper()
	a.Hasta(func() bool { return len(f.Pedidos()) >= n })
	return f.Pedidos()
}

// TestPrepararInstalaConI: en un agente sin CLI, i pide instalar y la página enseña que se está instalando; el pie ofrece
// la acción de la fila elegida y no otras.
func TestPrepararInstalaConI(t *testing.T) {
	a, f, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, nil,
		versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	a.Pulsar("down") // el simulado, listo, va primero
	quiere(t, pie(a), "i instalar")
	noQuiere(t, pie(a), "s iniciar sesión", "u actualizar", "x desinstalar")
	a.Pulsar("i")
	if p := esperar(t, a, f, 1); p[0].Agente != "claude" || p[0].Accion != "instalar" || p[0].Confirmar || p[0].BorrarDatos {
		t.Fatalf("pidió instalar claude: %+v", p)
	}
	a.HastaVer("Instalando Claude…")
	quiere(t, pie(a), "k cancela")
}

// TestPrepararIniciaSesionConS: un agente instalado sin sesión ofrece s, que pide iniciar sesión.
func TestPrepararIniciaSesionConS(t *testing.T) {
	a, f, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": sinSesion}, nil,
		versiones("claude", "2.1.0", "", map[string]bool{"iniciar_sesion": true}, "iniciar_sesion", "desinstalar"))
	a.Pulsar("down")
	quiere(t, pie(a), "s iniciar sesión", "x desinstalar")
	a.Pulsar("s")
	if p := esperar(t, a, f, 1); p[0].Agente != "claude" || p[0].Accion != "iniciar_sesion" {
		t.Fatalf("pidió iniciar sesión: %+v", p)
	}
	a.HastaVer("Comprobando Claude…")
}

// TestPrepararActualizaConU: con una versión más nueva que la instalada, u pide actualizar; sin ella, u avisa y no pide nada.
func TestPrepararActualizaConU(t *testing.T) {
	a, f, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": listoCLI}, nil,
		versiones("claude", "2.9.9", "2.10.0", map[string]bool{"actualizar": true}, "actualizar", "desinstalar"))
	quiere(t, pie(a), "u actualizar")
	a.Pulsar("enter")
	a.HastaVer("Hay una versión nueva: 2.10.0 (tienes 2.9.9)")
	a.Pulsar("esc", "u")
	if p := esperar(t, a, f, 1); p[0].Accion != "actualizar" || p[0].Agente != "claude" {
		t.Fatalf("pidió actualizar: %+v", p)
	}
	a.HastaVer("Actualizando Claude…")

	b, g, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": listoCLI}, nil,
		versiones("claude", "2.10.0", "2.10.0", map[string]bool{}, "desinstalar"))
	noQuiere(t, pie(b), "u actualizar")
	if av, ok := aviso(b.Pulsar("u")); !ok || av.Texto == "" || len(g.Pedidos()) != 0 {
		t.Fatalf("sin versión nueva, u avisa y no pide: %+v %v", av, g.Pedidos())
	}
}

// TestPrepararDesinstalaConConfirmacion: x pregunta antes y desinstala solo si se confirma, dejando su sesión y ajustes; X
// pregunta por borrarlos también. Cancelar no pide nada.
func TestPrepararDesinstalaConConfirmacion(t *testing.T) {
	a, f, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": listoCLI}, nil,
		versiones("claude", "2.1.0", "2.1.0", map[string]bool{"desinstalar": true}, "desinstalar"))
	q, ok := pregunta(a.Pulsar("x"))
	if !ok || !strings.Contains(q.Texto, "¿Desinstalar Claude?") {
		t.Fatalf("x pregunta: %+v", q)
	}
	quiere(t, q.Texto, "Borrar también su sesión y ajustes: No", "pulsa X")
	a.Entregar(comp.Cancelado{})
	if len(f.Pedidos()) != 0 {
		t.Fatalf("cancelar no desinstala: %+v", f.Pedidos())
	}
	a.Pulsar("x")
	a.Entregar(comp.Confirmado{})
	if p := esperar(t, a, f, 1); p[0].Accion != "desinstalar" || p[0].BorrarDatos || p[0].Confirmar {
		t.Fatalf("desinstala sin borrar sus datos: %+v", p)
	}
}

// TestPrepararDesinstalaYBorraDatos: X pide confirmar y manda borrar_datos.
func TestPrepararDesinstalaYBorraDatos(t *testing.T) {
	a, f, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": listoCLI}, nil,
		versiones("claude", "2.1.0", "2.1.0", map[string]bool{"desinstalar": false}, "desinstalar"))
	q, ok := pregunta(a.Pulsar("X"))
	if !ok {
		t.Fatal("X pregunta")
	}
	quiere(t, q.Texto, "¿Desinstalar Claude?", "Borrar también su sesión y ajustes: Sí", "aún no está comprobada")
	a.Entregar(comp.Confirmado{})
	if p := esperar(t, a, f, 1); p[0].Accion != "desinstalar" || !p[0].BorrarDatos || !p[0].Confirmar {
		t.Fatalf("desinstala y borra: con la versión sin comprobar el diálogo ya confirma: %+v", p)
	}
}

// TestPrepararEsperaAlUsuario: con esperando_usuario se ven el enlace (solo https) y el código como texto, el aviso de Codex
// y un campo para pegar que no enseña lo escrito, envía el código y se vacía; lo pegado no vuelve a verse.
func TestPrepararEsperaAlUsuario(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"codex": sinSesion}, nil,
		versiones("codex", "0.160.0", "", map[string]bool{"iniciar_sesion": true}, "iniciar_sesion"))
	a.Pulsar("down")
	a.Pulsar("s")
	esperar(t, a, f, 1)
	f.Fijar("codex", contrato.EstadoPreparacion{Accion: "iniciar_sesion", Estado: "esperando_usuario",
		Enlace: "https://auth.openai.example/device?x=1", Codigo: "ABCD-1234", PidePegar: true})
	tic(t, a, r)
	v := strings.Join(strings.Fields(a.Vista()), " ")
	quiere(t, v, "https://auth.openai.example/device?x=1", "ABCD-1234", "Abre el enlace, inicia sesión en Codex y pega aquí el código",
		"código de dispositivo", "https://chatgpt.com/#settings/Security")
	if !a.P.EnCampo() {
		t.Fatal("con el campo para pegar a la vista, las teclas son suyas")
	}
	a.Pegar("secreto-123")
	if v := a.Vista(); strings.Contains(v, "secreto") || !strings.Contains(v, "***") {
		t.Fatalf("lo escrito se oculta:\n%s", v)
	}
	a.Pulsar("enter")
	a.Hasta(func() bool { return len(f.Codigos()) == 1 })
	if c := f.Codigos(); c[0] != "secreto-123" {
		t.Fatalf("el servicio recibe lo pegado: %q", c)
	}
	if v := a.Vista(); strings.Contains(v, "secreto") || strings.Contains(v, "***") {
		t.Fatalf("el campo se vacía al enviar:\n%s", v)
	}
	a.Pulsar("enter")
	a.Asentar()
	if len(f.Codigos()) != 1 {
		t.Fatalf("un campo vacío no envía nada: %q", f.Codigos())
	}
}

// TestPrepararEsperaSinPegar: un CLI que no pide pegar (código de dispositivo) enseña el enlace y el código, sin campo; un
// enlace que no es https no se enseña.
func TestPrepararEsperaSinPegar(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"claude": sinSesion}, nil,
		versiones("claude", "2.1.0", "", map[string]bool{"iniciar_sesion": true}, "iniciar_sesion"))
	a.Pulsar("down", "s")
	esperar(t, a, f, 1)
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "iniciar_sesion", Estado: "esperando_usuario", Enlace: "javascript:alert(1)", Codigo: "WXYZ-9999"})
	tic(t, a, r)
	v := a.Vista()
	quiere(t, strings.Join(strings.Fields(v), " "), "WXYZ-9999", "Abre el enlace e introduce este código para iniciar sesión en Claude")
	noQuiere(t, v, "javascript", "Pega aquí el código")
	if a.P.EnCampo() {
		t.Fatal("sin campo para pegar, las teclas no son del campo")
	}
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "iniciar_sesion", Estado: "esperando_usuario", Enlace: "https://claude.example/oauth/authorize?code=true&client_id=" + strings.Repeat("a", 120)})
	tic(t, a, r)
	if v := strings.ReplaceAll(a.Vista(), "\n", ""); !strings.Contains(v, strings.Repeat("a", 120)) {
		t.Fatalf("el enlace largo se parte en líneas, entero:\n%s", a.Vista())
	}
}

// TestPrepararVersionNoComprobada: instalar con una versión sin comprobar llega como error version_no_comprobada y se ofrece
// «Seguir» (y), que repite con confirmar; iniciar sesión nunca se salta: se enseña su motivo con la orden para la consola.
func TestPrepararVersionNoComprobada(t *testing.T) {
	a, f, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, func(f *prueba.Preparacion) {
		f.Regla = func(p prueba.Pedido) (contrato.EstadoPreparacion, error) {
			if p.Confirmar {
				return contrato.EstadoPreparacion{Agente: p.Agente, Accion: p.Accion, Estado: "instalando"}, nil
			}
			return contrato.EstadoPreparacion{Agente: p.Agente, Accion: p.Accion, Estado: "error", MotivoCodigo: "version_no_comprobada", Mensaje: "versión 3.0.0 sin comprobar"}, nil
		}
	}, versiones("claude", "", "", map[string]bool{"instalar": false}, "instalar"))
	a.Pulsar("down", "enter")
	quiere(t, a.Vista(), "Esta versión de Claude aún no está comprobada con Tramtai")
	a.Pulsar("i")
	esperar(t, a, f, 1)
	a.HastaVer("Si lo instalas, el inicio de sesión automático")
	quiere(t, pie(a), "y seguir")
	a.Pulsar("y")
	if p := esperar(t, a, f, 2); p[1].Accion != "instalar" || !p[1].Confirmar {
		t.Fatalf("Seguir repite la acción con confirmar: %+v", p)
	}
	a.HastaVer("Instalando Claude…")
	noQuiere(t, pie(a), "y seguir")

	b, g, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": sinSesion}, func(f *prueba.Preparacion) {
		f.Regla = func(p prueba.Pedido) (contrato.EstadoPreparacion, error) {
			return contrato.EstadoPreparacion{Agente: p.Agente, Accion: p.Accion, Estado: "error", MotivoCodigo: "version_no_comprobada",
				Mensaje: "Inicia sesión en una consola con: claude auth login"}, nil
		}
	}, versiones("claude", "2.1.0", "", map[string]bool{"iniciar_sesion": true}, "iniciar_sesion"))
	b.Pulsar("down", "s")
	esperar(t, b, g, 1)
	b.HastaVer("claude auth login")
	noQuiere(t, pie(b), "y seguir")
	b.Pulsar("y")
	b.Asentar()
	if len(g.Pedidos()) != 1 {
		t.Fatalf("iniciar sesión no se salta: %+v", g.Pedidos())
	}
}

// TestPrepararListoConMotivo: un listo con motivo (instalado sin iniciar sesión) no está «listo para usar»: enseña su aviso.
func TestPrepararListoConMotivo(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, nil,
		versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	a.Pulsar("down", "i")
	esperar(t, a, f, 1)
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "instalar", Estado: "listo", VersionDespues: "2.1.0",
		MotivoCodigo: "version_no_comprobada", Mensaje: "Inicia sesión en una consola con: claude auth login"})
	tic(t, a, r)
	v := a.Vista()
	quiere(t, v, "Claude está instalado (2.1.0), pero sin iniciar sesión.", "claude auth login")
	noQuiere(t, v, "Sesión iniciada")

	b, g, s := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, nil,
		versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	b.Pulsar("down", "i")
	esperar(t, b, g, 1)
	g.Fijar("claude", contrato.EstadoPreparacion{Accion: "instalar", Estado: "listo", VersionDespues: "2.1.0"})
	tic(t, b, s)
	quiere(t, b.Vista(), "Claude está instalado (2.1.0).")
}

// TestPrepararFalla: un error con código se explica y enseña el detalle del servicio y las últimas líneas de la salida.
func TestPrepararFalla(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, nil,
		versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	a.Pulsar("down", "i")
	esperar(t, a, f, 1)
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "instalar", Estado: "error", MotivoCodigo: "instalador_fallo", Mensaje: "salió con 1",
		Lineas: []string{"descargando…", "curl: (6) no resuelve el host"}})
	tic(t, a, r)
	quiere(t, a.Vista(), "No se pudo instalar Claude", "El instalador terminó con un error", "salió con 1", "curl: (6) no resuelve el host")
	quiere(t, pie(a), "i instalar") // se puede volver a intentar
}

// TestPrepararVerRegistro: l enseña las últimas líneas de la última preparación del agente, aunque la haya lanzado otra
// pantalla; sin ninguna, lo dice.
func TestPrepararVerRegistro(t *testing.T) {
	a, _, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": listoCLI}, func(f *prueba.Preparacion) {
		f.Fijar("claude", contrato.EstadoPreparacion{Accion: "actualizar", Estado: "listo", Mensaje: "hecho", Lineas: []string{"bajando 2.1.0", "https://claude.example/notas"}})
	}, versiones("claude", "2.1.0", "2.1.0", map[string]bool{}, "desinstalar"))
	a.Pulsar("l")
	a.HastaVer("Registro de Claude")
	quiere(t, a.Vista(), "bajando 2.1.0", "https://claude.example/notas")
	a.Pulsar("esc")
	noQuiere(t, a.Vista(), "Registro de Claude")

	b, _, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": listoCLI}, nil,
		versiones("claude", "2.1.0", "2.1.0", map[string]bool{}, "desinstalar"))
	b.Pulsar("l")
	b.HastaVer("Aún no hay ninguna preparación de este agente.")
}

// TestPrepararCancela: k corta la preparación en marcha.
func TestPrepararCancela(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, nil,
		versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	a.Pulsar("down", "i")
	esperar(t, a, f, 1)
	a.HastaVer("Instalando Claude…")
	a.Pulsar("k")
	a.Hasta(func() bool { return len(f.Canceladas()) == 1 })
	tic(t, a, r)
	quiere(t, a.Vista(), "Se canceló la preparación de Claude.")
	noQuiere(t, pie(a), "k cancela")
}

// TestPrepararYaEnCurso: si otra pantalla ya lanzó la acción, la página sigue esa preparación en lugar de fallar.
func TestPrepararYaEnCurso(t *testing.T) {
	a, f, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, nil,
		versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "instalar", Estado: "instalando", Etapa: "descarga"})
	a.Pulsar("down", "i")
	a.HastaVer("Instalando Claude… · descargando")
	if len(f.Pedidos()) != 0 {
		t.Fatalf("no se lanzó otra: %+v", f.Pedidos())
	}
}

// TestPrepararSigueLaQueEstabaEnMarcha: al abrir la página se sigue la preparación que ya estaba en marcha.
func TestPrepararSigueLaQueEstabaEnMarcha(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, func(f *prueba.Preparacion) {
		f.Fijar("claude", contrato.EstadoPreparacion{Accion: "instalar", Estado: "instalando"})
	}, versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	a.HastaVer("Instalando Claude…")
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "instalar", Estado: "listo", VersionDespues: "2.1.0"})
	tic(t, a, r)
	quiere(t, a.Vista(), "Claude está instalado (2.1.0).")
}

// TestPrepararPeticionesEnCurso: si el agente tiene peticiones en curso, el servicio no instala y la página lo dice.
func TestPrepararPeticionesEnCurso(t *testing.T) {
	a, _, _ := preparando(t, map[string]adaptadores.Adaptador{"claude": listoCLI}, func(f *prueba.Preparacion) {
		f.Regla = func(prueba.Pedido) (contrato.EstadoPreparacion, error) {
			return contrato.EstadoPreparacion{}, &preparar.Error{Codigo: preparar.CodigoPeticionesEnCurso, Mensaje: "Claude Code tiene peticiones en curso: espera a que terminen"}
		}
	}, versiones("claude", "2.1.0", "2.1.0", map[string]bool{}, "desinstalar"))
	a.Pulsar("x")
	a.Entregar(comp.Confirmado{})
	a.HastaVer("peticiones en curso")
}

// TestPrepararVersiones: las versiones se piden al abrir y con c (forzando), no en cada sondeo de una preparación.
func TestPrepararVersiones(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"claude": sinCLI}, nil,
		versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	if n := len(f.Forzados()); n != 1 || f.Forzados()[0] {
		t.Fatalf("al abrir, una vez y sin forzar: %v", f.Forzados())
	}
	a.Pulsar("down", "i")
	esperar(t, a, f, 1)
	for range 3 {
		tic(t, a, r)
	}
	if n := len(f.Forzados()); n != 1 {
		t.Fatalf("el sondeo no vuelve a pedir las versiones: %v", f.Forzados())
	}
	// El servicio no repite una comprobación forzada hasta pasado un minuto, salvo que una preparación haya terminado después.
	fin := time.Now().Add(time.Hour)
	f.Fijar("codex", contrato.EstadoPreparacion{Accion: "instalar", Estado: "listo", Fin: &fin})
	a.Pulsar("c")
	a.Hasta(func() bool { return slicesContains(f.Forzados(), true) })
}

func slicesContains(l []bool, v bool) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

// TestPrepararEnElServidor: en el servidor se prepara para la cuenta del servicio, y se dice.
func TestPrepararEnElServidor(t *testing.T) {
	f := prueba.NuevaPreparacion()
	f.FijarVersiones(versiones("claude", "", "", map[string]bool{"instalar": true}, "instalar"))
	cli, id := prueba.Ajustado(t, func(o *servicio.Opciones) {
		o.Registro["claude"] = sinCLI
		o.Preparacion = f
		o.Entorno = "servidor"
	})
	c := contexto(t, cli, id)
	c.Entorno = "servidor"
	r := prueba.NuevoReloj()
	a := prueba.Abrir(t, agentes.NuevaConReloj("es", r.Espera), c)
	a.Hasta(func() bool { return len(f.Forzados()) > 0 })
	a.Asentar()
	a.Pulsar("down", "i")
	esperar(t, a, f, 1)
	a.HastaVer("Para la cuenta del servicio (")
}

// TestPrepararEscCierraLoTerminado: Esc quita primero la preparación que terminó y solo después sale de la página; con el
// campo para pegar, Esc suelta el foco y e lo recupera.
func TestPrepararEscCierraLoTerminado(t *testing.T) {
	a, f, r := preparando(t, map[string]adaptadores.Adaptador{"claude": sinSesion}, nil,
		versiones("claude", "2.1.0", "", map[string]bool{"iniciar_sesion": true}, "iniciar_sesion"))
	a.Pulsar("down", "s")
	esperar(t, a, f, 1)
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "iniciar_sesion", Estado: "esperando_usuario", Enlace: "https://claude.example/a", PidePegar: true})
	tic(t, a, r)
	if !a.P.EnCampo() {
		t.Fatal("el campo para pegar tiene el foco")
	}
	a.Pulsar("esc")
	if a.P.EnCampo() || !strings.Contains(pie(a), "e pegar código") {
		t.Fatalf("Esc suelta el campo y e lo recupera: %v %s", a.P.EnCampo(), pie(a))
	}
	a.Pulsar("e")
	if !a.P.EnCampo() {
		t.Fatal("e vuelve al campo")
	}
	a.Pulsar("esc")
	f.Fijar("claude", contrato.EstadoPreparacion{Accion: "iniciar_sesion", Estado: "listo"})
	tic(t, a, r)
	quiere(t, a.Vista(), "Sesión iniciada en Claude: ya puedes usarlo.")
	if fuera := a.Pulsar("esc"); len(fuera) != 0 {
		t.Fatalf("el primer Esc cierra la preparación sin salir: %+v", fuera)
	}
	noQuiere(t, a.Vista(), "Sesión iniciada en Claude")
	a.Pulsar("esc")
	a.Hasta(func() bool { return len(a.Fuera) > 0 })
}
