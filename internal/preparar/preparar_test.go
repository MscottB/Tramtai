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
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
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
		// Como la descarga: por un temporal y colocar. En Windows el ejecutable recién usado (el inicio de sesión de la
		// corrida anterior) puede seguir retenido un momento; se reintenta un poco.
		destino := filepath.Join(casa, "bin", nombreFalso())
		err := os.WriteFile(destino+".nuevo", datos, 0o700)
		for limite := time.Now().Add(3 * time.Second); err == nil; time.Sleep(50 * time.Millisecond) {
			if err = colocar(destino+".nuevo", destino); err == nil || time.Now().After(limite) {
				break
			}
			err = nil
		}
		if err != nil {
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
	case "sesion", "sesion-trozos":
		falsoSesion(modo)
	case "actualizar":
		fmt.Println("actualizado")
	case "fallar":
		fmt.Println("descargando")
		fmt.Println("no queda espacio en el disco")
		fmt.Fprintln(os.Stderr, "fallo grave")
		os.Exit(1)
	case "colgar": // un instalador que no termina y deja un nieto: los dos escriben un latido para saber si siguen vivos
		yo, _ := os.Executable()
		nieto := exec.Command(yo)
		nieto.Env = append(os.Environ(), "PREPARAR_FALSO=latir", "PREPARAR_LATIDO="+os.Getenv("PREPARAR_LATIDO")+".nieto")
		if err := nieto.Start(); err != nil {
			os.Exit(3)
		}
		falso("latir")
	case "latir":
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
	// La casa va dentro de una carpeta propia de la prueba: lo que hay junto a ella también es de la prueba.
	pr := &prueba{casa: filepath.Join(t.TempDir(), "casa"), datos: t.TempDir()}
	if err := os.Mkdir(pr.casa, 0o700); err != nil {
		t.Fatal(err)
	}
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
		// Sin este gancho valdría el del preparador, con las Comprobadas de la ficha (la falsa no tiene).
		Comprobada: func(string, string, string) bool { return true },
		// El inicio de sesión falso deja ~/.falso-sesion al terminar bien.
		Comprobar: func(context.Context, string) adaptadores.SesionCLI {
			if _, err := os.Stat(filepath.Join(pr.casa, ".falso-sesion")); err == nil {
				return adaptadores.SesionCLI{Estado: adaptadores.SesionOK}
			}
			return adaptadores.SesionCLI{Estado: adaptadores.SesionSin}
		},
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
	// La versión va como número (como en Versiones); lo que imprimió --version, tal cual, solo en el registro.
	if e.Estado != "listo" || e.VersionDespues != "1.2.3" {
		t.Fatalf("estado %+v", e)
	}
	if registro, _ := os.ReadFile(filepath.Join(pr.datos, "agentes", "preparaciones", "falso.log")); !strings.Contains(string(registro), "falso 1.2.3") {
		t.Fatalf("el registro guarda lo que imprimió --version: %s", registro)
	}
	if g := pr.guardadasHasta(); !slices.Equal(g, []string{"falso=" + ruta}) {
		t.Fatalf("comandos_agentes recibió %q, se esperaba %s", g, ruta)
	}
	// Actualizar lanza el CLI suelto («falso update») por la ruta de la receta, no por el PATH.
	e = iniciar(t, pr.p, "actualizar")
	if e.Estado != "listo" || e.VersionAntes != "1.2.3" || !slices.Contains(e.Lineas, "actualizado") {
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
	if e.Estado != "error" || e.MotivoCodigo != CodigoPlazoAgotado || !strings.Contains(e.Mensaje, "1 min") {
		t.Fatalf("estado %+v", e)
	}
	muertos(t, latido, latido+".nieto")
}

// vivos espera a que los procesos de esos latidos hayan empezado a latir.
func vivos(t *testing.T, latidos ...string) {
	t.Helper()
	limite := time.Now().Add(15 * time.Second)
	for _, latido := range latidos {
		for datos, _ := os.ReadFile(latido); len(datos) == 0; datos, _ = os.ReadFile(latido) {
			if time.Now().After(limite) {
				t.Fatalf("%s: el proceso no llegó a arrancar", filepath.Base(latido))
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// muertos comprueba que los procesos que escriben esos latidos ya no viven: el instalador y su nieto.
func muertos(t *testing.T, latidos ...string) {
	t.Helper()
	for _, latido := range latidos {
		antes, _ := os.ReadFile(latido)
		time.Sleep(300 * time.Millisecond)
		if despues, _ := os.ReadFile(latido); len(antes) == 0 || len(despues) != len(antes) {
			t.Errorf("%s: el proceso sigue vivo (latido %d → %d)", filepath.Base(latido), len(antes), len(despues))
		}
	}
}

// Revisión 3: dos pantallas lanzan la misma preparación; la segunda ve la primera.
func TestYaEnCurso(t *testing.T) {
	latido := filepath.Join(t.TempDir(), "latido")
	pr := nueva(t, fichaFalsa(t, "colgar", "PREPARAR_LATIDO="+latido))
	type respuesta struct {
		e   contrato.EstadoPreparacion
		err error
	}
	salida := make(chan respuesta, 2)
	listos := make(chan struct{})
	for _, accion := range []string{"instalar", "instalar"} {
		go func() {
			<-listos
			e, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: accion})
			salida <- respuesta{e, err}
		}()
	}
	close(listos) // las dos a la vez
	a, b := <-salida, <-salida
	if a.err != nil {
		a, b = b, a
	}
	if a.err != nil || codigo(b.err) != CodigoYaEnCurso || b.e.Estado != "instalando" || !b.e.Inicio.Equal(*a.e.Inicio) {
		t.Fatalf("una se lanza y la otra recibe ya_en_curso con el estado de la primera: %+v %v / %+v %v", a.e, a.err, b.e, b.err)
	}
	vivos(t, latido, latido+".nieto")
	if err := pr.p.Cancelar("falso"); err != nil {
		t.Fatal(err)
	}
	if e := final(t, pr.p, "falso"); e.Estado != "cancelado" {
		t.Fatalf("estado %+v", e)
	}
	muertos(t, latido, latido+".nieto")
}

// Ruling 19: al parar el servicio, Cerrar corta las preparaciones en marcha y espera a que su árbol termine; quedan en
// error con interrumpida (también en disco) y después no se lanza ninguna más.
func TestCerrar(t *testing.T) {
	latido := filepath.Join(t.TempDir(), "latido")
	pr := nueva(t, fichaFalsa(t, "colgar", "PREPARAR_LATIDO="+latido))
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "instalar"}); err != nil {
		t.Fatal(err)
	}
	vivos(t, latido, latido+".nieto")
	pr.p.Cerrar()
	// Cerrar ya esperó: el estado es el final sin sondear y el árbol está cerrado.
	if e, _ := pr.p.Estado("falso"); e.Estado != "error" || e.MotivoCodigo != CodigoInterrumpida || e.Fin == nil {
		t.Fatalf("tras Cerrar: %+v", e)
	}
	muertos(t, latido, latido+".nieto")
	if g, err := pr.p.leer("falso"); err != nil || g.MotivoCodigo != CodigoInterrumpida || g.PID != 0 {
		t.Fatalf("en disco: %+v %v", g, err)
	}
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "instalar"}); nucleo.CodigoDe(err) != nucleo.CodigoServicioDetenido {
		t.Fatalf("tras Cerrar no se lanza ninguna preparación: %v", err)
	}
	pr.p.Cerrar() // dos veces, o sin nada en marcha, no espera
}

// EnCurso puede preguntar por el estado: no se llama con el cerrojo tomado.
func TestEnCursoSinCerrojo(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	pr.p.o.EnCurso = func(agente string) int {
		pr.p.Estado(agente)
		return 1
	}
	hecho := make(chan error, 1)
	go func() {
		_, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "instalar"})
		hecho <- err
	}()
	select {
	case err := <-hecho:
		if codigo(err) != CodigoPeticionesEnCurso {
			t.Fatalf("se esperaba peticiones_en_curso: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Iniciar se bloqueó: EnCurso se llamó con el cerrojo tomado")
	}
	if e, ok := pr.p.Estado("falso"); ok {
		t.Fatalf("la reserva se deshace: %+v", e)
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
	if e := iniciar(t, pr.p, "desinstalar"); e.Estado != "listo" || e.VersionAntes != "1.2.3" {
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
	pid, firma, terminado, latido := colgado(t)
	ahora := time.Now()
	if err := pr.p.guardar("falso", guardado{EstadoPreparacion: contrato.EstadoPreparacion{Agente: "falso", Accion: "instalar",
		Estado: "instalando", Inicio: &ahora}, PID: pid, Firma: firma}); err != nil {
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
	muertos(t, latido, latido+".nieto")
}

// colgado lanza fuera del preparador un instalador colgado con su nieto (como uno que dejó un servicio caído) y espera a
// que los dos latan; terminado se cierra cuando acaba.
func colgado(t *testing.T) (pid int, firma string, terminado chan struct{}, latido string) {
	t.Helper()
	yo, _ := os.Executable()
	iniciado := make(chan struct{})
	ctx := procesos.ConAlIniciar(context.Background(), func(p int, f string) { pid, firma = p, f; close(iniciado) })
	terminado = make(chan struct{})
	latido = filepath.Join(t.TempDir(), "latido")
	go func() {
		defer close(terminado)
		_, _ = procesos.Ejecutar(ctx, procesos.Orden{Programa: yo, Entorno: append(os.Environ(), "PREPARAR_FALSO=colgar",
			"PREPARAR_LATIDO="+latido)}, func(string) error { return nil })
	}()
	select {
	case <-iniciado:
	case <-time.After(10 * time.Second):
		t.Fatal("el proceso no arrancó")
	}
	vivos(t, latido, latido+".nieto")
	return pid, firma, terminado, latido
}

// M1: un proceso de la corrida anterior que no confirmó su cierre (su PID solo queda en memoria: el primer fijar de la
// siguiente lo pisa en disco) se termina al lanzar la siguiente.
func TestHuerfanoDeLaAnterior(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	pid, firma, terminado, latido := colgado(t)
	pr.p.mu.Lock()
	pr.p.tareas["falso"] = &tarea{g: guardado{EstadoPreparacion: contrato.EstadoPreparacion{Agente: "falso", Accion: "instalar",
		Estado: "cancelado"}, PID: pid, Firma: firma}}
	pr.p.mu.Unlock()
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "listo" {
		t.Fatalf("estado %+v", e)
	}
	select {
	case <-terminado:
	case <-time.After(10 * time.Second):
		t.Fatal("el proceso de la corrida anterior sigue vivo")
	}
	muertos(t, latido, latido+".nieto")
}

// M2: mientras Iniciar pregunta por las peticiones en curso, Estado da la preparación anterior (o ninguna), no la que
// quizá se deshaga: el servicio no frena peticiones por ella.
func TestReservaNoSeVe(t *testing.T) {
	pr := nueva(t, fichaFalsa(t, "instalar"))
	var vistos []string
	conPeticiones := func(agente string) int {
		e, ok := pr.p.Estado(agente)
		vistos = append(vistos, fmt.Sprint(ok, " ", e.Accion, " ", e.Estado))
		return 1
	}
	// Alguien más mira el estado a la vez (con -race, sin carreras).
	parar := make(chan struct{})
	var mirando sync.WaitGroup
	mirando.Go(func() {
		for {
			select {
			case <-parar:
				return
			default:
				pr.p.Estado("falso")
			}
		}
	})
	defer func() { close(parar); mirando.Wait() }()
	pr.p.o.EnCurso = conPeticiones
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "instalar"}); codigo(err) != CodigoPeticionesEnCurso {
		t.Fatalf("se esperaba peticiones_en_curso: %v", err)
	}
	pr.p.o.EnCurso = func(string) int { return 0 }
	iniciar(t, pr.p, "instalar")
	pr.p.o.EnCurso = conPeticiones
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "actualizar"}); codigo(err) != CodigoPeticionesEnCurso {
		t.Fatalf("se esperaba peticiones_en_curso: %v", err)
	}
	if !slices.Equal(vistos, []string{"false  ", "true instalar listo"}) {
		t.Fatalf("durante la reserva se vio %q", vistos)
	}
	if e, _ := pr.p.Estado("falso"); e.Accion != "instalar" || e.Estado != "listo" {
		t.Fatalf("la reserva deshecha deja la anterior: %+v", e)
	}
}

// M7: en Windows no se puede pisar un ejecutable en marcha (copilot.exe abierto en una consola) pero sí renombrarlo:
// colocar aparta el viejo y deja el nuevo.
func TestColocarSobreUnoEnMarcha(t *testing.T) {
	dir := t.TempDir()
	destino := filepath.Join(dir, nombreFalso())
	yo, _ := os.Executable()
	datos, err := os.ReadFile(yo)
	if err == nil {
		err = os.WriteFile(destino, datos, 0o700)
	}
	if err != nil {
		t.Fatal(err)
	}
	latido := filepath.Join(dir, "latido")
	cmd := exec.Command(destino)
	cmd.Env = append(os.Environ(), "PREPARAR_FALSO=latir", "PREPARAR_LATIDO="+latido)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	vivos(t, latido)
	for _, contenido := range []string{"nuevo", "otro más"} { // dos veces: el .viejo de la primera sigue en marcha
		if err := os.WriteFile(destino+".nuevo", []byte(contenido), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := colocar(destino+".nuevo", destino); err != nil {
			t.Fatalf("colocar sobre uno en marcha: %v", err)
		}
		if leido, _ := os.ReadFile(destino); string(leido) != contenido {
			t.Fatalf("en destino quedó %q", leido)
		}
	}
	vivos(t, latido) // el que estaba en marcha sigue
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
	var pedidos atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		switch {
		case strings.HasPrefix(r.URL.Path, "/a-http/"): // una redirección que deja HTTPS
			http.Redirect(w, r, "http://"+r.Host+strings.TrimPrefix(r.URL.Path, "/a-http"), http.StatusFound)
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

	// sinTemporales: ni los de esta descarga ni los que dejó una anterior cortada.
	sinTemporales := func(t *testing.T, pr *prueba) {
		t.Helper()
		sobrantes, _ := filepath.Glob(filepath.Join(pr.casa, "bin", ".descarga-*"))
		if _, err := os.Stat(filepath.Join(pr.casa, "bin", nombreFalso()+".nuevo")); err == nil {
			sobrantes = append(sobrantes, nombreFalso()+".nuevo")
		}
		if len(sobrantes) > 0 {
			t.Errorf("quedan temporales: %q", sobrantes)
		}
	}
	cortada := func(t *testing.T, pr *prueba) {
		t.Helper()
		os.MkdirAll(filepath.Join(pr.casa, "bin"), 0o700)
		os.WriteFile(filepath.Join(pr.casa, "bin", ".descarga-123.zip"), []byte("a medias"), 0o600)
		os.WriteFile(filepath.Join(pr.casa, "bin", nombreFalso()+".nuevo"), []byte("a medias"), 0o600)
	}

	pr := con(t, srv.URL)
	cortada(t, pr)
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "listo" {
		t.Fatalf("con la suma buena: %+v", e)
	}
	if datos, err := os.ReadFile(filepath.Join(pr.casa, "bin", nombreFalso())); err != nil || !bytes.Equal(datos, binario) {
		t.Fatalf("el binario extraído: %q %v", datos, err)
	}
	sinTemporales(t, pr)

	sumas = strings.Repeat("f", 64)
	pr = con(t, srv.URL)
	cortada(t, pr)
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo {
		t.Fatalf("con la suma mala: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(pr.casa, "bin", nombreFalso())); !os.IsNotExist(err) {
		t.Fatalf("con la suma mala no se deja el binario: %v", err)
	}
	sinTemporales(t, pr)

	pr = con(t, strings.Replace(srv.URL, "https://", "http://", 1))
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo {
		t.Fatalf("por http no se descarga: %+v", e)
	}

	pr = con(t, srv.URL+"/a-http")
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo || !strings.Contains(e.Mensaje, "HTTPS") {
		t.Fatalf("una redirección a http se rechaza: %+v", e)
	}
	sinTemporales(t, pr)

	// Una versión con otra forma no llega a ninguna URL.
	pr = con(t, srv.URL)
	pr.p.o.UltimaVersion = func(context.Context, string) (string, error) { return "1.0;iex x", nil }
	antes := pedidos.Load()
	if e := iniciar(t, pr.p, "instalar"); e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo || pedidos.Load() != antes {
		t.Fatalf("una versión no válida no se descarga: %+v (%d pedidos)", e, pedidos.Load()-antes)
	}
}

// {version}: sin versión se omite el argumento y la variable no se hereda; con versión se sustituye.
func TestVersionEnLaOrden(t *testing.T) {
	t.Setenv("FALSO_VERSION", "heredada")
	o := adaptadores.OrdenReceta{Argumentos: []string{"-c", "x", "{version}"}, Entorno: []string{"A=1", "FALSO_VERSION={version}"}}
	args, extra, err := conVersion(o, "")
	entorno := entornoDe(extra)
	if err != nil || !slices.Equal(args, []string{"-c", "x"}) || !slices.Contains(entorno, "A=1") ||
		slices.ContainsFunc(entorno, func(par string) bool { return strings.HasPrefix(par, "FALSO_VERSION=") }) {
		t.Fatalf("sin versión: %q %q %v", args, extra, err)
	}
	args, extra, err = conVersion(o, "2.0.1")
	if err != nil || !slices.Equal(args, []string{"-c", "x", "2.0.1"}) || !slices.Contains(entornoDe(extra), "FALSO_VERSION=2.0.1") {
		t.Fatalf("con versión: %q %q %v", args, extra, err)
	}
	for _, mala := range []string{"1.0;iex x", "1.0 --x", "$(rm)", "v1.0", "1..0", "-1"} {
		if _, _, err := conVersion(o, mala); codigo(err) != CodigoInstaladorFallo {
			t.Errorf("%q no es una versión: %v", mala, err)
		}
	}
}

// Una versión que no vale no lanza nada.
func TestVersionNoValidaNoLanza(t *testing.T) {
	f := fichaFalsa(t, "instalar") // su orden lleva {version} y, si se lanzara, dejaría el CLI en ~/bin
	pr := nueva(t, f)
	c := &corrida{p: pr.p, t: &tarea{}, agente: "falso", f: f, r: f.Receta, ctx: context.Background()}
	if err := c.ejecutar(f.Receta.Instalar[runtime.GOOS], "1.0;iex x"); codigo(err) != CodigoInstaladorFallo {
		t.Fatalf("se esperaba instalador_fallo: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pr.casa, "bin", nombreFalso())); !os.IsNotExist(err) {
		t.Fatal("la orden se lanzó con una versión que no vale")
	}
}

func TestFormaSecreto(t *testing.T) {
	c := &corrida{p: &Preparador{}, t: &tarea{pegados: []string{"abc-123#estado"}}}
	if tapado := c.redactar("pegaste abc-123#estado"); strings.Contains(tapado, "abc-123") {
		t.Errorf("lo pegado se tapa: %q", tapado)
	}
	for _, secreto := range []string{claveFalsa, "ghp_" + strings.Repeat("a1", 15), "npm_" + strings.Repeat("Ab1", 12),
		"AKIAABCDEFGHIJKLMNOP", "ya29.a0AfH6SMBx-abc_def", "glpat-" + strings.Repeat("x9", 10), "AIza" + strings.Repeat("B", 35),
		"xoxb-1234567890-abc", "eyJhbGciOiJIUzI1.eyJzdWIiOiIxMjM0.firma", "Bearer abcdefghijklmnop1234"} {
		if tapado := c.redactar("salida " + secreto + " fin"); strings.Contains(tapado, secreto) || !strings.Contains(tapado, "[REDACTADO]") {
			t.Errorf("no se tapó %q: %q", secreto, tapado)
		}
	}
	if tapado := c.redactar("https://usuario:clave@proxy.local"); strings.Contains(tapado, "clave") {
		t.Errorf("una URL con contraseña: %q", tapado)
	}
	if c.redactar("introduce el código ABCD-1234 en github.com/login/device") != "introduce el código ABCD-1234 en github.com/login/device" {
		t.Error("un código de dispositivo no es un secreto")
	}
}

// Desinstalar solo borra dentro de la casa y nunca una carpeta compartida; si una ruta no vale, no se borra ninguna.
func TestBorrarSoloEnLaCasa(t *testing.T) {
	base := t.TempDir()
	fuera := t.TempDir()
	testigo := func(dir string) string {
		ruta := filepath.Join(dir, "testigo")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return ruta
	}
	for _, mala := range []string{"~", "~/.", "~/..", "~/../otro", fuera, "rel", "~/.local", "~/AppData/Local", "~/bin/../.."} {
		t.Run(mala, func(t *testing.T) {
			f := fichaFalsa(t, "instalar")
			pr := nueva(t, f)
			t.Chdir(base)
			bueno := testigo(filepath.Join(pr.casa, "bueno"))
			testigos := []string{bueno, testigo(pr.casa), testigo(filepath.Dir(pr.casa)), testigo(filepath.Join(filepath.Dir(pr.casa), "otro")),
				testigo(fuera), testigo(filepath.Join(base, "rel")), testigo(filepath.Join(pr.casa, ".local")),
				testigo(filepath.Join(pr.casa, "AppData", "Local"))}
			f.Receta.Borrar[runtime.GOOS] = []string{"~/bueno", mala} // la buena va primero: tampoco se borra
			if e := iniciar(t, pr.p, "desinstalar"); e.Estado != "error" || e.MotivoCodigo != CodigoInstaladorFallo {
				t.Fatalf("estado %+v", e)
			}
			for _, ruta := range testigos {
				if _, err := os.Stat(ruta); err != nil {
					t.Errorf("se borró %s: %v", ruta, err)
				}
			}
		})
	}
}

// enlazar deja en enlace un enlace a destino: una junction en Windows (como Codex/bin), un enlace simbólico en los demás.
func enlazar(t *testing.T, destino, enlace string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if salida, err := exec.Command("cmd", "/c", "mklink", "/J", enlace, destino).CombinedOutput(); err != nil {
			t.Fatalf("mklink /J: %v %s", err, salida)
		}
		return
	}
	if err := os.Symlink(destino, enlace); err != nil {
		t.Fatal(err)
	}
}

// Desinstalar quita los enlaces (también los de DatosCLI, y lo dice) sin tocar lo que hay detrás; una ruta que pasa por un
// enlace a fuera de la casa no se borra.
func TestDesinstalarEnlaces(t *testing.T) {
	f := fichaFalsa(t, "instalar")
	pr := nueva(t, f)
	programa, datos := t.TempDir(), t.TempDir()
	tras := []string{filepath.Join(programa, "falso"), filepath.Join(datos, "sesion.json")}
	for _, ruta := range tras {
		os.WriteFile(ruta, []byte("x"), 0o600)
	}
	os.MkdirAll(filepath.Join(pr.casa, "bin"), 0o700)
	enlazar(t, programa, filepath.Join(pr.casa, "bin", "enlace"))
	enlazar(t, datos, filepath.Join(pr.casa, ".falso"))
	f.Receta.Borrar[runtime.GOOS] = []string{"~/bin/enlace"}
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "desinstalar", BorrarDatos: true}); err != nil {
		t.Fatal(err)
	}
	e := final(t, pr.p, "falso")
	// El aviso de DatosCLI nombra el enlace (en el idioma que toque).
	if e.Estado != "listo" || !slices.ContainsFunc(e.Lineas, func(l string) bool { return strings.Contains(l, ".falso") }) {
		t.Fatalf("estado %+v", e)
	}
	for _, ruta := range []string{filepath.Join(pr.casa, "bin", "enlace"), filepath.Join(pr.casa, ".falso")} {
		if _, err := os.Lstat(ruta); !os.IsNotExist(err) {
			t.Errorf("el enlace %s sigue: %v", ruta, err)
		}
	}
	for _, ruta := range tras {
		if _, err := os.Stat(ruta); err != nil {
			t.Errorf("se borró lo que había detrás del enlace: %v", err)
		}
	}

	// ~/.falso/sesion.json pasa por un enlace a fuera de la casa: no se borra.
	enlazar(t, datos, filepath.Join(pr.casa, ".falso"))
	f.Receta.Borrar[runtime.GOOS] = []string{"~/.falso/sesion.json"}
	if e := iniciar(t, pr.p, "desinstalar"); e.Estado != "error" {
		t.Fatalf("estado %+v", e)
	}
	if _, err := os.Stat(tras[1]); err != nil {
		t.Fatalf("se borró fuera de la casa a través de un enlace: %v", err)
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
	enlazar(t, destino, enlace)
	if esEnlace, err := borrar(enlace); err != nil || !esEnlace {
		t.Fatalf("borrar: %v %v", esEnlace, err)
	}
	if _, err := os.Lstat(enlace); !os.IsNotExist(err) {
		t.Fatalf("la junction sigue: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destino, "codex.exe")); err != nil {
		t.Fatalf("se borró lo que había detrás de la junction: %v", err)
	}
}
