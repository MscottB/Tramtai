package preparar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/contrato"
)

// falsoSesion es el inicio de sesión falso: imprime PREPARAR_SALIDA (o, en «sesion-trozos», el enlace de Copilot en tres
// trozos con ANSI) y espera. Con PREPARAR_PEGADO lee una línea de la entrada y la deja en ese archivo; si no, late en
// PREPARAR_LATIDO hasta que exista PREPARAR_APROBADO. Al terminar bien deja ~/.falso-sesion.
func falsoSesion(modo string) {
	if modo == "sesion-trozos" {
		for _, trozo := range []string{"To authenticate, visit \x1b[1mhttps://git", "hub.com/login/dev",
			"ice\x1b[0m and enter code \x1b[1m1A2B-3C4D\x1b[0m\r\nWaiting for authorization...\r\n"} {
			_, _ = os.Stdout.WriteString(trozo)
			time.Sleep(200 * time.Millisecond)
		}
	} else {
		datos, _ := os.ReadFile(os.Getenv("PREPARAR_SALIDA"))
		_, _ = os.Stdout.Write(datos)
	}
	if os.Getenv("PREPARAR_RECHAZAR") != "" { // como Claude con un código rechazado
		fmt.Fprintln(os.Stderr, "Login failed: That code wasn't accepted.")
		os.Exit(1)
	}
	if pegado := os.Getenv("PREPARAR_PEGADO"); pegado != "" {
		linea, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			os.Exit(4)
		}
		linea = strings.TrimRight(linea, "\r\n")
		_ = os.WriteFile(pegado, []byte(linea), 0o600)
		// Repite lo leído: el registro y las líneas lo tienen que tapar.
		fmt.Println("leído: " + linea)
		fmt.Fprintln(os.Stderr, "leído: "+linea)
	} else {
		for aprobado := os.Getenv("PREPARAR_APROBADO"); ; time.Sleep(50 * time.Millisecond) {
			if f, err := os.OpenFile(os.Getenv("PREPARAR_LATIDO"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
				_, _ = f.WriteString(".")
				f.Close()
			}
			if _, err := os.Stat(aprobado); aprobado != "" && err == nil {
				break
			}
		}
	}
	casa, _ := os.UserHomeDir()
	_ = os.WriteFile(filepath.Join(casa, ".falso-sesion"), nil, 0o600)
}

// fichaSesion es la ficha falsa con el lector del CLI real id; su inicio de sesión es este programa en el modo dado.
func fichaSesion(t *testing.T, id, modo string, entorno ...string) adaptadores.Ficha {
	t.Helper()
	f := fichaFalsa(t, "instalar")
	real, _ := agentes.Buscar(id)
	l := *real.Receta.Sesion
	l.Argumentos = []string{"login"}
	l.Entorno = append([]string{"PREPARAR_FALSO=" + modo}, entorno...)
	f.Receta.Sesion = &l
	return f
}

// salidaDe es la ruta de una salida real del M0.
func salidaDe(t *testing.T, id, archivo string) string {
	t.Helper()
	ruta, err := filepath.Abs(filepath.Join("..", "agentes", id, "testdata", archivo))
	if err != nil {
		t.Fatal(err)
	}
	return ruta
}

// ponerFalso deja el CLI falso instalado en ~/bin, la ruta de su receta.
func ponerFalso(t *testing.T, casa string) {
	t.Helper()
	yo, _ := os.Executable()
	datos, err := os.ReadFile(yo)
	if err == nil {
		err = os.MkdirAll(filepath.Join(casa, "bin"), 0o700)
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(casa, "bin", nombreFalso()), datos, 0o700)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// ordenFalso es la orden a mano del CLI falso: con su ruta completa, porque ~/bin no está en el PATH.
func ordenFalso(casa string) string {
	return enConsola(runtime.GOOS, filepath.Join(casa, "bin", nombreFalso()), []string{"login"})
}

// I1: en el escritorio, la orden a mano lleva la ruta completa del CLI si la consola no lo encuentra por su nombre (el
// instalador de Claude no toca el PATH), entre comillas si hace falta; si lo encuentra, el nombre basta.
func TestOrdenConsolaEscritorio(t *testing.T) {
	for _, c := range []struct{ goos, cli, quiere string }{
		{"windows", `C:\Users\ana\.local\bin\claude.exe`, `C:\Users\ana\.local\bin\claude.exe auth login`},
		{"windows", `C:\Users\ADMINI~1\bin\claude.exe`, `C:\Users\ADMINI~1\bin\claude.exe auth login`},
		{"windows", `C:\Users\Ana María\.local\bin\claude.exe`, `& "C:\Users\Ana María\.local\bin\claude.exe" auth login`},
		{"windows", `C:\Users\a$b\claude.exe`, "& \"C:\\Users\\a`$b\\claude.exe\" auth login"},
		{"windows", "claude", "claude auth login"},
		{"linux", "/home/ana/.local/bin/claude", "/home/ana/.local/bin/claude auth login"},
		{"darwin", "/Users/Ana María/.local/bin/claude", "'/Users/Ana María/.local/bin/claude' auth login"},
		{"linux", "/home/o'hara/bin/claude", `'/home/o'\''hara/bin/claude' auth login`},
	} {
		if got := enConsola(c.goos, c.cli, []string{"auth", "login"}); got != c.quiere {
			t.Errorf("%s %q: %q, se esperaba %q", c.goos, c.cli, got, c.quiere)
		}
	}
	pr := nueva(t, fichaSesion(t, "codex", "sesion"))
	f, _ := pr.p.o.Ficha("falso")
	if orden := pr.p.ordenConsola(f); orden != "falso login" {
		t.Fatalf("sin el CLI, su nombre: %q", orden)
	}
	ponerFalso(t, pr.casa)
	if orden := pr.p.ordenConsola(f); orden != ordenFalso(pr.casa) || !strings.Contains(orden, pr.casa) {
		t.Fatalf("fuera del PATH, la ruta completa: %q", orden)
	}
	t.Setenv("PATH", filepath.Join(pr.casa, "bin"))
	if orden := pr.p.ordenConsola(f); orden != "falso login" {
		t.Fatalf("en el PATH, su nombre: %q", orden)
	}
}

// esperando espera al estado esperando_usuario (y a que pida pegar, si pegar).
func esperando(t *testing.T, p *Preparador, pegar bool) contrato.EstadoPreparacion {
	t.Helper()
	limite := time.Now().Add(15 * time.Second)
	for {
		e, _ := p.Estado("falso")
		if e.Estado == "esperando_usuario" && (!pegar || e.PidePegar) {
			return e
		}
		if !enMarcha(e.Estado) && e.Estado != "" || time.Now().After(limite) {
			t.Fatalf("no llegó a esperando_usuario: %+v", e)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func lanzarSesion(t *testing.T, p *Preparador) {
	t.Helper()
	if _, err := p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "iniciar_sesion"}); err != nil {
		t.Fatal(err)
	}
}

func aprobar(t *testing.T, archivo string) {
	t.Helper()
	if err := os.WriteFile(archivo, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSesionCodigoDispositivo(t *testing.T) {
	aprobado := filepath.Join(t.TempDir(), "aprobado")
	pr := nueva(t, fichaSesion(t, "codex", "sesion", "PREPARAR_SALIDA="+salidaDe(t, "codex", "sesion-windows.txt"),
		"PREPARAR_APROBADO="+aprobado))
	var mu sync.Mutex
	var pedidas []string
	pr.p.o.Comprobada = func(agente, accion, version string) bool {
		mu.Lock()
		defer mu.Unlock()
		pedidas = append(pedidas, agente+" "+accion+" "+version)
		return true
	}
	ponerFalso(t, pr.casa)
	lanzarSesion(t, pr.p)
	e := esperando(t, pr.p, false)
	if e.Enlace != "https://auth.openai.com/codex/device" || e.Codigo != "ABCD-12345" || e.PidePegar || e.Accion != "iniciar_sesion" {
		t.Fatalf("estado %+v", e)
	}
	if err := pr.p.Codigo("falso", "abc-12345"); err == nil {
		t.Fatal("sin PidePegar no se acepta nada pegado")
	}
	aprobar(t, aprobado)
	if e = final(t, pr.p, "falso"); e.Estado != "listo" || e.PidePegar {
		t.Fatalf("estado %+v", e)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(pedidas, []string{"falso iniciar_sesion 1.2.3"}) {
		t.Fatalf("Comprobada se pregunta con la versión instalada antes de lanzar: %q", pedidas)
	}
}

// Tras instalar se sigue con el inicio de sesión, salvo si ya la hay o su versión no está comprobada; tras actualizar, no.
func TestInstalarSigueConSesion(t *testing.T) {
	aprobado := filepath.Join(t.TempDir(), "aprobado")
	pr := nueva(t, fichaSesion(t, "copilot", "sesion", "PREPARAR_SALIDA="+salidaDe(t, "copilot", "sesion-windows.txt"),
		"PREPARAR_APROBADO="+aprobado))
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "instalar"}); err != nil {
		t.Fatal(err)
	}
	e := esperando(t, pr.p, false)
	if e.Accion != "instalar" || e.Enlace != "https://github.com/login/device" || e.Codigo != "1A2B-3C4D" || e.VersionDespues != "1.2.3" {
		t.Fatalf("estado %+v", e)
	}
	aprobar(t, aprobado)
	if e = final(t, pr.p, "falso"); e.Estado != "listo" {
		t.Fatalf("estado %+v", e)
	}
	for _, accion := range []string{"instalar", "actualizar"} { // con la sesión ya iniciada
		if e = iniciar(t, pr.p, accion); e.Estado != "listo" || e.Enlace != "" {
			t.Fatalf("%s: %+v", accion, e)
		}
	}
	if err := os.Remove(filepath.Join(pr.casa, ".falso-sesion")); err != nil {
		t.Fatal(err)
	}
	if e = iniciar(t, pr.p, "actualizar"); e.Estado != "listo" || e.Enlace != "" {
		t.Fatalf("actualizar no lanza el inicio de sesión: %+v", e)
	}
	pr.p.o.Comprobada = func(_, accion, _ string) bool { return accion != "iniciar_sesion" }
	e = iniciar(t, pr.p, "instalar")
	if lineas := strings.Join(e.Lineas, "\n"); e.Estado != "listo" || e.Enlace != "" ||
		!strings.Contains(lineas, "Falso CLI 1.2.3 aún no está comprobado") || !strings.Contains(lineas, "en una consola con "+ordenFalso(pr.casa)) {
		t.Fatalf("sin la versión comprobada se instala, no se inicia sesión y se dice la orden para la consola: %+v", e)
	}
	// El estado listo lo dice también (las pantallas no cuentan el agente como listo para usar).
	if e.MotivoCodigo != CodigoVersionNoComprobada || !strings.Contains(e.Mensaje, "en una consola con "+ordenFalso(pr.casa)) || e.OrdenConsola != ordenFalso(pr.casa) {
		t.Fatalf("el aviso va en el estado: %+v", e)
	}
	// Una corrida sin aviso no hereda el de la anterior.
	pr.p.o.Comprobada = func(string, string, string) bool { return true }
	if e = iniciar(t, pr.p, "actualizar"); e.Estado != "listo" || e.MotivoCodigo != "" || e.Mensaje != "" {
		t.Fatalf("sin aviso: %+v", e)
	}
}

func TestSesionPegarCodigo(t *testing.T) {
	pegado := filepath.Join(t.TempDir(), "pegado")
	pr := nueva(t, fichaSesion(t, "claude", "sesion", "PREPARAR_SALIDA="+salidaDe(t, "claude", "sesion-windows.txt"),
		"PREPARAR_PEGADO="+pegado))
	ponerFalso(t, pr.casa)
	lanzarSesion(t, pr.p)
	e := esperando(t, pr.p, true)
	if !strings.HasPrefix(e.Enlace, "https://claude.com/cai/oauth/authorize?code=true&") || strings.ContainsAny(e.Enlace, "\x1b\r\n") || e.Codigo != "" {
		t.Fatalf("estado %+v", e)
	}
	for _, malo := range []string{"", "   ", "abc-123", "abc\n123", "abc 123", "abc\x1b[1m", strings.Repeat("a", 2000)} {
		if err := pr.p.Codigo("falso", malo); err == nil || strings.TrimSpace(malo) != "" && strings.Contains(err.Error(), malo) {
			t.Errorf("%q: se esperaba un error sin lo pegado: %v", malo, err)
		}
	}
	if err := pr.p.Codigo("otro", "abc-12345"); err == nil {
		t.Error("un agente desconocido no recibe nada")
	}
	if err := pr.p.Codigo("falso", "abc-12345"); err != nil {
		t.Fatal(err)
	}
	if e = final(t, pr.p, "falso"); e.Estado != "listo" {
		t.Fatalf("estado %+v", e)
	}
	// El falso repitió lo pegado por stdout y por stderr: las dos líneas llegan tapadas.
	if n := strings.Count(strings.Join(e.Lineas, "\n"), "leído: [REDACTADO]"); n != 2 {
		t.Fatalf("lo repetido por el CLI debía llegar tapado dos veces: %q", e.Lineas)
	}
	if datos, err := os.ReadFile(pegado); err != nil || string(datos) != "abc-12345" {
		t.Fatalf("el CLI recibió %q (%v)", datos, err)
	}
	if err := pr.p.Codigo("falso", "abc-12345"); err == nil {
		t.Error("terminado, ya no se acepta nada pegado")
	}
	estado, _ := json.Marshal(e)
	carpeta := filepath.Join(pr.datos, "agentes", "preparaciones")
	registro, _ := os.ReadFile(filepath.Join(carpeta, "falso.log"))
	guardado, _ := os.ReadFile(filepath.Join(carpeta, "falso.json"))
	for nombre, texto := range map[string][]byte{"estado": estado, "registro": registro, "guardado": guardado} {
		if len(texto) == 0 || strings.Contains(string(texto), "abc-12345") {
			t.Errorf("%s vacío o con lo pegado:\n%s", nombre, texto)
		}
	}
}

// Revisión 4: el enlace llega partido y con ANSI.
func TestSesionSalidaPartidaYANSI(t *testing.T) {
	aprobado := filepath.Join(t.TempDir(), "aprobado")
	pr := nueva(t, fichaSesion(t, "copilot", "sesion-trozos", "PREPARAR_APROBADO="+aprobado))
	ponerFalso(t, pr.casa)
	lanzarSesion(t, pr.p)
	e := esperando(t, pr.p, false)
	if e.Enlace != "https://github.com/login/device" || e.Codigo != "1A2B-3C4D" {
		t.Fatalf("estado %+v", e)
	}
	aprobar(t, aprobado)
	e = final(t, pr.p, "falso")
	if e.Estado != "listo" || !slices.Contains(e.Lineas, "To authenticate, visit https://github.com/login/device and enter code 1A2B-3C4D") {
		t.Fatalf("estado %+v", e)
	}
}

// El lector, con las salidas reales del M0 de byte en byte: nunca da un enlace o un código a medias.
func TestLectorDeByteEnByte(t *testing.T) {
	casos := []struct {
		id, archivo, enlace, codigo string
		pegar                       bool
	}{
		{"claude", "sesion-windows.txt", "https://claude.com/cai/oauth/authorize?code=true&", "", true},
		{"claude", "sesion-linux-pty.txt", "https://claude.com/cai/oauth/authorize?code=true&", "", true},
		{"claude", "sesion-windows-codigo-rechazado.txt", "https://claude.com/cai/oauth/authorize?code=true&", "", true},
		{"codex", "sesion-windows.txt", "https://auth.openai.com/codex/device", "ABCD-12345", false},
		{"codex", "sesion-sin-color.txt", "https://auth.openai.com/codex/device", "ABCD-12345", false},
		{"codex", "sesion-linux-pty.txt", "https://auth.openai.com/codex/device", "ABCD-12345", false},
		{"copilot", "sesion-windows.txt", "https://github.com/login/device", "1A2B-3C4D", false},
		{"copilot", "sesion-linux.txt", "https://github.com/login/device", "1A2B-3C4D", false},
		{"copilot", "sesion-linux-sin-navegador.txt", "https://github.com/login/device", "1A2B-3C4D", false},
		{"copilot", "sesion-linux-pty.txt", "https://github.com/login/device", "1A2B-3C4D", false},
	}
	type aviso struct {
		enlace, codigo string
		pegar          bool
	}
	for _, c := range casos {
		t.Run(c.id+"/"+c.archivo, func(t *testing.T) {
			f, _ := agentes.Buscar(c.id)
			bruto, err := os.ReadFile(salidaDe(t, c.id, c.archivo))
			if err != nil {
				t.Fatal(err)
			}
			var avisos []aviso
			x := &lector{l: f.Receta.Sesion, avisar: func(enlace, codigo string, pegar bool) { avisos = append(avisos, aviso{enlace, codigo, pegar}) }}
			for i := range bruto {
				_, _ = x.Write(bruto[i : i+1])
			}
			var deUnaVez []aviso
			y := &lector{l: f.Receta.Sesion, avisar: func(enlace, codigo string, pegar bool) { deUnaVez = append(deUnaVez, aviso{enlace, codigo, pegar}) }}
			_, _ = y.Write(bruto)
			if len(avisos) == 0 || len(deUnaVez) != 1 || avisos[len(avisos)-1] != deUnaVez[0] || deUnaVez[0].pegar != c.pegar {
				t.Fatalf("de byte en byte %+v, de una vez %+v", avisos, deUnaVez)
			}
			for _, a := range avisos {
				if a.enlace != deUnaVez[0].enlace || !strings.HasPrefix(a.enlace, c.enlace) || a.codigo != c.codigo || strings.ContainsAny(a.enlace, "\x1b\x07\r\n") {
					t.Errorf("aviso a medias: %+v", a)
				}
			}
		})
	}
}

func TestSesionNoReconocida(t *testing.T) {
	otra := filepath.Join(t.TempDir(), "otra.txt")
	if err := os.WriteFile(otra, []byte("Welcome\nnada que ver\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	latido := filepath.Join(t.TempDir(), "latido")
	pr := nueva(t, fichaSesion(t, "copilot", "sesion", "PREPARAR_SALIDA="+otra, "PREPARAR_LATIDO="+latido))
	pr.p.plazoLector = 5 * time.Second // con -race en Windows, 2 s no siempre llegan a la primera línea
	ponerFalso(t, pr.casa)
	inicio := time.Now()
	lanzarSesion(t, pr.p)
	vivos(t, latido) // el plazo cuenta desde que arranca: late antes de que se corte
	e := final(t, pr.p, "falso")
	if e.Estado != "error" || e.MotivoCodigo != CodigoSalidaNoReconocida || !slices.Contains(e.Lineas, "nada que ver") ||
		!strings.Contains(e.Mensaje, ordenFalso(pr.casa)) || e.OrdenConsola != ordenFalso(pr.casa) || time.Since(inicio) > 30*time.Second {
		t.Fatalf("estado %+v", e)
	}
	muertos(t, latido)
	registro, _ := os.ReadFile(filepath.Join(pr.datos, "agentes", "preparaciones", "falso.log"))
	if !strings.Contains(string(registro), "marcar 1.2.3") {
		t.Fatalf("Marcar se llama con la versión instalada:\n%s", registro)
	}
}

func TestSesionCancelar(t *testing.T) {
	latido := filepath.Join(t.TempDir(), "latido")
	pr := nueva(t, fichaSesion(t, "codex", "sesion", "PREPARAR_SALIDA="+salidaDe(t, "codex", "sesion-sin-color.txt"),
		"PREPARAR_LATIDO="+latido))
	ponerFalso(t, pr.casa)
	lanzarSesion(t, pr.p)
	esperando(t, pr.p, false)
	vivos(t, latido)
	if err := pr.p.Cancelar("falso"); err != nil {
		t.Fatal(err)
	}
	if e := final(t, pr.p, "falso"); e.Estado != "cancelado" {
		t.Fatalf("estado %+v", e)
	}
	muertos(t, latido)
}

// Sin lector (Antigravity), sin el CLI o con su versión sin comprobar, el inicio de sesión no se lanza.
func TestSesionNoSeLanza(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	ponerFalso(t, pr.casa)
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "iniciar_sesion"}); err == nil || codigo(err) != "" {
		t.Fatalf("sin lector es un error de entrada: %v", err)
	}

	// Sin el CLI se dice eso, aunque su versión (ninguna) tampoco esté comprobada.
	pr = nueva(t, fichaSesion(t, "codex", "sesion"))
	pr.p.o.Comprobada = func(string, string, string) bool { return false }
	if e := iniciar(t, pr.p, "iniciar_sesion"); e.Estado != "error" || e.MotivoCodigo != CodigoNoEncontrado {
		t.Fatalf("sin el CLI: %+v", e)
	}

	ponerFalso(t, pr.casa)
	if e := iniciar(t, pr.p, "iniciar_sesion"); e.Estado != "error" || e.MotivoCodigo != CodigoVersionNoComprobada || len(e.Lineas) != 0 ||
		e.Mensaje != "Falso CLI 1.2.3 aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, inicia sesión en una consola con "+ordenFalso(pr.casa) {
		t.Fatalf("sin comprobar, el aviso de la tarjeta con la orden para la consola: %+v", e)
	}
	// En el servidor, la orden es la de la cuenta del servicio con la ruta del CLI instalado (fuera de su PATH); sin
	// consola para esa cuenta (Windows Server), queda la clave de API.
	f, _ := pr.p.o.Ficha("falso")
	pr.p.o.OrdenConsola = func(cli string, args []string) string { return "como-servicio " + cli + " " + strings.Join(args, " ") }
	if e := iniciar(t, pr.p, "iniciar_sesion"); !strings.HasSuffix(e.Mensaje, "Mientras, inicia sesión en una consola con como-servicio "+buscar(f)+" login") || buscar(f) == "" {
		t.Fatalf("en el servidor, la orden como la cuenta del servicio: %+v", e)
	}
	pr.p.o.OrdenConsola = func(string, []string) string { return "" }
	if e := iniciar(t, pr.p, "iniciar_sesion"); !strings.HasSuffix(e.Mensaje, "Mientras, con la cuenta del servicio no se puede iniciar sesión en una consola: usa su clave de API (tramtai-servidor claves guardar)") || e.OrdenConsola != "" {
		t.Fatalf("sin consola para la cuenta del servicio, la clave de API: %+v", e)
	}
	pr.p.o.OrdenConsola = nil
	// Sin versión que decir (un instalador que no la fija y la última sin consultar), el mensaje no queda con dos espacios.
	pr.p.o.UltimaVersion = func(context.Context, string) (string, error) { return "", errors.New("sin red") }
	if e := iniciar(t, pr.p, "instalar"); e.MotivoCodigo != CodigoVersionNoComprobada || e.Mensaje != "Falso CLI aún no está comprobado con Tramtai" {
		t.Fatalf("sin versión: %+v", e)
	}
}

// Un inicio de sesión que termina con error (Claude con un código rechazado) sale con inicio_sesion_fallo, sus líneas y la
// orden para la consola.
func TestSesionFalla(t *testing.T) {
	pr := nueva(t, fichaSesion(t, "claude", "sesion", "PREPARAR_SALIDA="+salidaDe(t, "claude", "sesion-windows.txt"),
		"PREPARAR_RECHAZAR=1"))
	ponerFalso(t, pr.casa)
	e := iniciar(t, pr.p, "iniciar_sesion")
	if e.Estado != "error" || e.MotivoCodigo != CodigoInicioSesionFallo || !strings.Contains(e.Mensaje, "código 1") ||
		!strings.Contains(e.Mensaje, ordenFalso(pr.casa)) || e.OrdenConsola != ordenFalso(pr.casa) || !slices.Contains(e.Lineas, "Login failed: That code wasn't accepted.") {
		t.Fatalf("estado %+v", e)
	}
}

// Un inicio de sesión que el servicio dejó esperando al usuario sale interrumpido, sin su enlace, su código ni el pegar.
func TestRecuperarSesion(t *testing.T) {
	pr := nueva(t, fichaSesion(t, "claude", "sesion"))
	ahora := time.Now()
	if err := pr.p.guardar("falso", guardado{EstadoPreparacion: contrato.EstadoPreparacion{Agente: "falso", Accion: "iniciar_sesion",
		Estado: "esperando_usuario", Enlace: "https://claude.com/x", Codigo: "ABCD-1234", PidePegar: true, Inicio: &ahora}}); err != nil {
		t.Fatal(err)
	}
	p := Nuevo(pr.p.o)
	p.Recuperar()
	if e, _ := p.Estado("falso"); e.Estado != "error" || e.MotivoCodigo != CodigoInterrumpida || e.Enlace != "" || e.Codigo != "" || e.PidePegar {
		t.Fatalf("estado %+v", e)
	}
}

// Un ESC que no se reconoce (ESC ESC, un OSC sin terminador) no esconde lo que le sigue; el tope de 64 KB guarda lo que cabe.
func TestLectorEscapesRarosYTope(t *testing.T) {
	f, _ := agentes.Buscar("copilot")
	linea := "To authenticate, visit https://github.com/login/device and enter code 1A2B-3C4D\n"
	leer := func(trozos ...string) (enlace, codigo string) {
		x := &lector{l: f.Receta.Sesion, avisar: func(e, c string, _ bool) { enlace, codigo = e, c }}
		for _, trozo := range trozos {
			_, _ = x.Write([]byte(trozo))
		}
		return enlace, codigo
	}
	for nombre, trozos := range map[string][]string{
		"ESC ESC":          {"\x1b\x1b[1mhola\x1b[0m\n", linea},
		"OSC sin terminar": {"\x1b]0;título sin fin\n" + linea},
		"tope a mitad":     {strings.Repeat("x", maximoLector-len(linea)-10) + "\n", linea + strings.Repeat("y", 500)},
		"ESC en la cola":   {"\x1b]8;;https://github.com/login/device\x07visit https://github.com/login/device", " and enter code 1A2B-3C4D\x1b]8;;\x07\n"},
	} {
		if enlace, codigo := leer(trozos...); enlace != "https://github.com/login/device" || codigo != "1A2B-3C4D" {
			t.Errorf("%s: %q %q", nombre, enlace, codigo)
		}
	}
	if enlace, _ := leer(strings.Repeat("x", maximoLector) + "\n" + linea); enlace != "" {
		t.Error("lo que pasa del tope no se mira")
	}
}

// El número sale de lo que imprime --version, sin el punto final de una frase (Copilot, en el job de compatibilidad).
func TestNumeroVersion(t *testing.T) {
	for texto, numero := range map[string]string{
		"2.1.311 (Claude Code)":       "2.1.311",
		"codex-cli 0.169.0":           "0.169.0",
		"GitHub Copilot CLI 1.0.111.": "1.0.111",
		"agy 1.5.8\n":                 "1.5.8",
		"falso 2.0.0-beta.1":          "2.0.0-beta.1",
		"sin versión":                 "",
	} {
		if got := numeroVersion(texto); got != numero {
			t.Errorf("numeroVersion(%q) = %q, se esperaba %q", texto, got, numero)
		}
	}
}
