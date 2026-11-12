package preparar

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/procesos"
)

// claveFalsa tiene forma de clave de Anthropic: no puede llegar ni al registro ni a Lineas.
var claveFalsa = "sk-ant-api03-" + strings.Repeat("Ab3", 10)

// TestMain hace de CLI falso: con --version dice su versión y, con PREPARAR_FALSO, de instalador.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("falso 1.2.3")
		os.Exit(0)
	}
	if modo := os.Getenv("PREPARAR_FALSO"); modo != "" {
		falso(modo)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func nombreFalso() string {
	if runtime.GOOS == "windows" {
		return "falso.exe"
	}
	return "falso"
}

// falso es el instalador falso: deja una copia de este programa en ~/bin (fuera del PATH), falla, no deja nada o se
// queda colgado.
func falso(modo string) {
	instalar := func() {
		casa, _ := os.UserHomeDir()
		yo, _ := os.Executable()
		datos, _ := os.ReadFile(yo)
		_ = os.MkdirAll(filepath.Join(casa, "bin"), 0o700)
		if err := os.WriteFile(filepath.Join(casa, "bin", nombreFalso()), datos, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Println("falso instalado")
	}
	switch modo {
	case "instalar":
		instalar()
	case "leer": // una pregunta interactiva: con la entrada cerrada sigue
		_, _ = io.ReadAll(os.Stdin)
		instalar()
	case "clave":
		fmt.Println("tu clave es " + claveFalsa)
		instalar()
	case "actualizar":
		fmt.Println("actualizado")
	case "fallar":
		fmt.Println("descargando")
		fmt.Println("no queda espacio en el disco")
		fmt.Fprintln(os.Stderr, "fallo grave")
		os.Exit(1)
	case "colgar": // un bucle que no termina: escribe un latido para saber si sigue vivo
		for {
			f, err := os.OpenFile(os.Getenv("PREPARAR_LATIDO"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err == nil {
				f.WriteString(".")
				f.Close()
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// fichaFalsa es una ficha cuyo instalador es este programa en el modo dado.
func fichaFalsa(t *testing.T, modo string, entorno ...string) adaptadores.Ficha {
	t.Helper()
	yo, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ruta := "~/bin/" + nombreFalso()
	return adaptadores.Ficha{Nombre: "falso", Programa: "Falso CLI", Comando: "falso", Receta: &adaptadores.Receta{
		Instalar: map[string]adaptadores.OrdenReceta{runtime.GOOS: {Programa: yo,
			Argumentos: []string{"{version}"}, Entorno: append([]string{"PREPARAR_FALSO=" + modo, "FALSO_VERSION={version}"}, entorno...)}},
		Actualizar: map[string]adaptadores.OrdenReceta{runtime.GOOS: {Programa: "falso", Argumentos: []string{"update"},
			Entorno: []string{"PREPARAR_FALSO=actualizar"}}},
		Borrar:   map[string][]string{runtime.GOOS: {ruta}},
		Rutas:    map[string][]string{runtime.GOOS: {ruta}},
		DatosCLI: []string{"~/.falso"},
	}}
}

type prueba struct {
	p          *Preparador
	casa       string
	datos      string
	mu         sync.Mutex
	guardadas  []string
	peticiones int
}

func (pr *prueba) guardadasHasta() []string {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return slices.Clone(pr.guardadas)
}

// nueva prepara un Preparador con la ficha dada, una casa temporal y unos datos temporales.
func nueva(t *testing.T, f adaptadores.Ficha) *prueba {
	t.Helper()
	pr := &prueba{casa: t.TempDir(), datos: t.TempDir()}
	t.Setenv("USERPROFILE", pr.casa)
	t.Setenv("HOME", pr.casa)
	pr.p = Nuevo(Opciones{
		Datos: pr.datos,
		Ficha: func(id string) (adaptadores.Ficha, bool) { return f, id == f.Nombre },
		EnCurso: func(string) int {
			pr.mu.Lock()
			defer pr.mu.Unlock()
			return pr.peticiones
		},
		Guardar: func(agente, ruta string) error {
			pr.mu.Lock()
			defer pr.mu.Unlock()
			pr.guardadas = append(pr.guardadas, agente+"="+ruta)
			return nil
		},
		UltimaVersion: func(context.Context, string) (string, error) { return "1.2.3", nil },
	})
	return pr
}

// final espera a que la preparación termine y devuelve su estado.
func final(t *testing.T, p *Preparador, agente string) contrato.EstadoPreparacion {
	t.Helper()
	limite := time.Now().Add(30 * time.Second)
	for {
		e, ok := p.Estado(agente)
		if ok && !enMarcha(e.Estado) {
			return e
		}
		if time.Now().After(limite) {
			t.Fatalf("la preparación no terminó: %+v", e)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func iniciar(t *testing.T, p *Preparador, accion string) contrato.EstadoPreparacion {
	t.Helper()
	if _, err := p.Iniciar("falso", contrato.PedidoPreparacion{Accion: accion}); err != nil {
		t.Fatal(err)
	}
	return final(t, p, "falso")
}

func codigo(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Codigo
	}
	return ""
}

// Revisión 1: el instalador deja el CLI fuera del PATH del servicio; se encuentra por la ruta de la receta y se guarda.
func TestInstalarYEncontrar(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	e := iniciar(t, pr.p, "instalar")
	ruta := filepath.Join(pr.casa, "bin", nombreFalso())
	if e.Estado != "listo" || e.VersionDespues != "falso 1.2.3" {
		t.Fatalf("estado %+v", e)
	}
	if g := pr.guardadasHasta(); !slices.Equal(g, []string{"falso=" + ruta}) {
		t.Fatalf("comandos_agentes recibió %q, se esperaba %s", g, ruta)
	}
	// Actualizar lanza el CLI suelto («falso update») por la ruta de la receta, no por el PATH.
	e = iniciar(t, pr.p, "actualizar")
	if e.Estado != "listo" || e.VersionAntes != "falso 1.2.3" || !slices.Contains(e.Lineas, "actualizado") {
		t.Fatalf("actualizar: %+v", e)
	}
}

func TestInstaladorFalla(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "fallar"))
	e := iniciar(t, pr.p, "instalar")
	if e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo || e.Mensaje == "" {
		t.Fatalf("estado %+v", e)
	}
	for _, l := range []string{"no queda espacio en el disco", "fallo grave"} {
		if !slices.Contains(e.Lineas, l) {
			t.Errorf("faltan las últimas líneas del instalador (%q): %q", l, e.Lineas)
		}
	}
}

func TestNoEncontradoTrasInstalar(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "nada"))
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "error" || e.MotivoCodigo != CodigoNoEncontrado {
		t.Fatalf("estado %+v", e)
	}
	if g := pr.guardadasHasta(); len(g) != 0 {
		t.Fatalf("no había nada que guardar: %q", g)
	}
}

// Revisión 2: un instalador que pregunta sigue porque su entrada va cerrada; uno colgado se corta con su plazo.
func TestInstaladorEsperaEntrada(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "leer"))
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "listo" {
		t.Fatalf("con la entrada cerrada debía terminar: %+v", e)
	}

	latido := filepath.Join(t.TempDir(), "latido")
	pr = nueva(t, fichaFalsa(t, "colgar", "PREPARAR_LATIDO="+latido))
	pr.p.plazo = 2 * time.Second
	e := iniciar(t, pr.p, "instalar")
	if e.Estado != "error" || e.MotivoCodigo != CodigoPlazoAgotado {
		t.Fatalf("estado %+v", e)
	}
	antes, _ := os.ReadFile(latido)
	time.Sleep(500 * time.Millisecond)
	if despues, _ := os.ReadFile(latido); len(antes) == 0 || len(despues) != len(antes) {
		t.Fatalf("el instalador colgado sigue vivo (latido %d → %d)", len(antes), len(despues))
	}
}

// Revisión 3: dos pantallas lanzan la misma preparación; la segunda ve la primera.
func TestYaEnCurso(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "colgar", "PREPARAR_LATIDO="+filepath.Join(t.TempDir(), "latido")))
	primero, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "instalar"})
	if err != nil {
		t.Fatal(err)
	}
	segundo, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "desinstalar"})
	if codigo(err) != CodigoYaEnCurso || segundo.Accion != "instalar" || segundo.Estado != "instalando" || !segundo.Inicio.Equal(*primero.Inicio) {
		t.Fatalf("se esperaba ya_en_curso con el estado de la primera: %+v %v", segundo, err)
	}
	if err := pr.p.Cancelar("falso"); err != nil {
		t.Fatal(err)
	}
	if e := final(t, pr.p, "falso"); e.Estado != "cancelado" {
		t.Fatalf("estado %+v", e)
	}
}

func TestNoActualizaConPeticiones(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	pr.peticiones = 1
	for _, accion := range []string{"actualizar", "desinstalar"} {
		if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: accion}); codigo(err) != CodigoPeticionesEnCurso {
			t.Fatalf("%s: se esperaba peticiones_en_curso: %v", accion, err)
		}
	}
	if e, ok := pr.p.Estado("falso"); ok {
		t.Fatalf("no debía lanzarse nada: %+v", e)
	}
}

func TestDesinstalarNoBorraDatos(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	ruta := filepath.Join(pr.casa, "bin", nombreFalso())
	sesion := filepath.Join(pr.casa, ".falso", "sesion.json")
	if err := os.MkdirAll(filepath.Dir(sesion), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sesion, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	iniciar(t, pr.p, "instalar")
	if e := iniciar(t, pr.p, "desinstalar"); e.Estado != "listo" || e.VersionAntes != "falso 1.2.3" {
		t.Fatalf("estado %+v", e)
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatalf("el ejecutable sigue: %v", err)
	}
	if _, err := os.Stat(sesion); err != nil {
		t.Fatalf("sin borrar_datos la sesión del CLI se queda: %v", err)
	}
	if g := pr.guardadasHasta(); !slices.Equal(g, []string{"falso=" + ruta, "falso="}) {
		t.Fatalf("se quita de comandos_agentes la ruta que puso Tramtai: %q", g)
	}

	iniciar(t, pr.p, "instalar")
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "desinstalar", BorrarDatos: true}); err != nil {
		t.Fatal(err)
	}
	if e := final(t, pr.p, "falso"); e.Estado != "listo" {
		t.Fatalf("estado %+v", e)
	}
	if _, err := os.Stat(filepath.Dir(sesion)); !os.IsNotExist(err) {
		t.Fatalf("con borrar_datos la carpeta del CLI se borra: %v", err)
	}
}

// Revisión 5: el servicio cayó con un instalador en marcha; al volver no queda huérfano y la preparación sale interrumpida.
func TestRecuperarTrasCaida(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	yo, _ := os.Executable()
	type inicio struct {
		pid   int
		firma string
	}
	iniciado := make(chan inicio, 1)
	ctx := procesos.ConAlIniciar(context.Background(), func(pid int, firma string) { iniciado <- inicio{pid, firma} })
	terminado := make(chan struct{})
	go func() {
		defer close(terminado)
		_, _ = procesos.Ejecutar(ctx, procesos.Orden{Programa: yo, Entorno: append(os.Environ(), "PREPARAR_FALSO=colgar",
			"PREPARAR_LATIDO="+filepath.Join(t.TempDir(), "latido"))}, func(string) error { return nil })
	}()
	var i inicio
	select {
	case i = <-iniciado:
	case <-time.After(10 * time.Second):
		t.Fatal("el proceso no arrancó")
	}
	ahora := time.Now()
	if err := pr.p.guardar("falso", guardado{EstadoPreparacion: contrato.EstadoPreparacion{Agente: "falso", Accion: "instalar",
		Estado: "instalando", Inicio: &ahora}, PID: i.pid, Firma: i.firma}); err != nil {
		t.Fatal(err)
	}

	p := Nuevo(pr.p.o)
	p.Recuperar()
	select {
	case <-terminado:
	case <-time.After(10 * time.Second):
		t.Fatal("Recuperar no terminó el instalador huérfano")
	}
	e, ok := p.Estado("falso")
	if !ok || e.Estado != "error" || e.MotivoCodigo != CodigoInterrumpida || e.Fin == nil {
		t.Fatalf("estado %+v", e)
	}
}

func TestRegistroRedactado(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "clave"))
	e := iniciar(t, pr.p, "instalar")
	registro, err := os.ReadFile(filepath.Join(pr.datos, "agentes", "preparaciones", "falso.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(registro), claveFalsa) || strings.Contains(strings.Join(e.Lineas, "\n"), claveFalsa) {
		t.Fatalf("la clave llegó al registro o a Lineas:\n%s\n%q", registro, e.Lineas)
	}
	if !strings.Contains(string(registro), "[REDACTADO]") || !strings.Contains(string(registro), "listo") {
		t.Fatalf("el registro debía tener la línea tapada y el resultado:\n%s", registro)
	}
}

func TestDescargaSumaNoCoincide(t *testing.T) {
	binario := []byte("binario de prueba")
	var zipeado bytes.Buffer
	z := zip.NewWriter(&zipeado)
	w, _ := z.Create(nombreFalso())
	w.Write(binario)
	z.Close()
	suma := sha256.Sum256(zipeado.Bytes())
	sumas := hex.EncodeToString(suma[:])
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
			fmt.Fprintf(w, "%s  otro.zip\n%s  falso-%s.zip\n", strings.Repeat("0", 64), sumas, arquitecturas[runtime.GOARCH])
		case strings.HasPrefix(r.URL.Path, "/v1.2.3/falso-"):
			w.Write(zipeado.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	con := func(t *testing.T, base string) *prueba {
		f := fichaFalsa(t, "instalar")
		f.Receta.Instalar = nil
		f.Receta.Descarga = map[string]adaptadores.DescargaVerificada{runtime.GOOS: {URL: base + "/v{version}/falso-{arq}.zip",
			URLSumas: base + "/v{version}/SHA256SUMS.txt"}}
		pr := nueva(t, f)
		pr.p.cliente.Transport = srv.Client().Transport
		return pr
	}

	pr := con(t, srv.URL)
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "listo" {
		t.Fatalf("con la suma buena: %+v", e)
	}
	if datos, err := os.ReadFile(filepath.Join(pr.casa, "bin", nombreFalso())); err != nil || !bytes.Equal(datos, binario) {
		t.Fatalf("el binario extraído: %q %v", datos, err)
	}

	sumas = strings.Repeat("f", 64)
	pr = con(t, srv.URL)
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo {
		t.Fatalf("con la suma mala: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(pr.casa, "bin", nombreFalso())); !os.IsNotExist(err) {
		t.Fatalf("con la suma mala no se deja el binario: %v", err)
	}

	pr = con(t, strings.Replace(srv.URL, "https://", "http://", 1))
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo {
		t.Fatalf("por http no se descarga: %+v", e)
	}
}

// {version}: sin versión se omite el argumento y la variable no se hereda; con versión se sustituye.
func TestVersionEnLaOrden(t *testing.T) {
	t.Setenv("FALSO_VERSION", "heredada")
	o := adaptadores.OrdenReceta{Argumentos: []string{"-c", "x", "{version}"}, Entorno: []string{"A=1", "FALSO_VERSION={version}"}}
	args, extra := conVersion(o, "")
	entorno := entornoDe(extra)
	if !slices.Equal(args, []string{"-c", "x"}) || !slices.Contains(entorno, "A=1") ||
		slices.ContainsFunc(entorno, func(par string) bool { return strings.HasPrefix(par, "FALSO_VERSION=") }) {
		t.Fatalf("sin versión: %q %q", args, extra)
	}
	args, extra = conVersion(o, "2.0.1")
	if !slices.Equal(args, []string{"-c", "x", "2.0.1"}) || !slices.Contains(entornoDe(extra), "FALSO_VERSION=2.0.1") {
		t.Fatalf("con versión: %q %q", args, extra)
	}
}

func TestAgenteDesconocido(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	if _, err := pr.p.Iniciar("../otro", contrato.PedidoPreparacion{Accion: "instalar"}); err == nil {
		t.Fatal("un agente que no está en el catálogo se rechaza")
	}
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "borrar todo"}); err == nil {
		t.Fatal("una acción desconocida se rechaza")
	}
}

// Codex/bin es una junction: desinstalar quita el enlace, nunca lo que hay detrás.
func TestBorrarJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("las junction son de Windows")
	}
	dir := t.TempDir()
	destino := filepath.Join(dir, "current", "bin")
	if err := os.MkdirAll(destino, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destino, "codex.exe"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	enlace := filepath.Join(dir, "bin")
	if salida, err := exec.Command("cmd", "/c", "mklink", "/J", enlace, destino).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v %s", err, salida)
	}
	if err := borrar(enlace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(enlace); !os.IsNotExist(err) {
		t.Fatalf("la junction sigue: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destino, "codex.exe")); err != nil {
		t.Fatalf("se borró lo que había detrás de la junction: %v", err)
	}
}
