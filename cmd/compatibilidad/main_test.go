package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/preparar"
)

// TestMain hace de CLI falso: con --version dice FALSO_VERSION y, con COMPATIBILIDAD_FALSO, de instalador o de inicio de
// sesión.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("falso " + os.Getenv("FALSO_VERSION"))
		os.Exit(0)
	}
	switch os.Getenv("COMPATIBILIDAD_FALSO") {
	case "":
		os.Exit(m.Run())
	case "instalar": // se copia a ~/bin, fuera del PATH
		casa, _ := os.UserHomeDir()
		yo, _ := os.Executable()
		datos, _ := os.ReadFile(yo)
		_ = os.MkdirAll(filepath.Join(casa, "bin"), 0o700)
		if err := os.WriteFile(filepath.Join(casa, "bin", nombreFalso()), datos, 0o700); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
	case "fallar": // la salida lleva algo con forma de código de dispositivo: el aviso lo tapa
		fmt.Println("descargando, código ABCD-1234")
		os.Exit(1)
	case "actualizar":
		fmt.Println("actualizado")
	case "sesion": // enlace, código y la petición de pegar; espera lo pegado hasta que lo cierran
		fmt.Println("Para entrar, visita https://ejemplo.test/login?state=secreto y escribe el código WXYZ-9876")
		fmt.Println("Pega aquí el código >")
		_, _ = io.Copy(io.Discard, os.Stdin)
	case "muda": // un inicio de sesión que cambió: termina sin enlace
		fmt.Println("nada que ver")
	}
	os.Exit(0)
}

func nombreFalso() string {
	if runtime.GOOS == "windows" {
		return "falso.exe"
	}
	return "falso"
}

// fichaFalsa es una ficha cuyo instalador y cuyo CLI son este programa: instala en el modo dado e inicia sesión en el
// modo sesion. Fija la versión, y su receta trae 1.0.0 comprobada para instalar.
func fichaFalsa(t *testing.T, instalar, sesion string) adaptadores.Ficha {
	t.Helper()
	yo, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ruta := "~/bin/" + nombreFalso()
	return adaptadores.Ficha{Nombre: "falso", Programa: "Falso CLI", Comando: "falso", Receta: &adaptadores.Receta{
		Instalar: map[string]adaptadores.OrdenReceta{runtime.GOOS: {Programa: yo, Argumentos: []string{"{version}"},
			Entorno: []string{"COMPATIBILIDAD_FALSO=" + instalar}}},
		Actualizar: map[string]adaptadores.OrdenReceta{runtime.GOOS: {Programa: "falso", Argumentos: []string{"update"},
			Entorno: []string{"COMPATIBILIDAD_FALSO=actualizar"}}},
		Borrar:      map[string][]string{runtime.GOOS: {ruta}},
		Rutas:       map[string][]string{runtime.GOOS: {ruta}},
		FijaVersion: true,
		Ultima:      adaptadores.FuenteVersion{Tipo: "url", Nombre: "https://ejemplo.test/ultima"},
		Sesion: &adaptadores.LectorSesion{
			Argumentos: []string{"login"},
			Entorno:    []string{"COMPATIBILIDAD_FALSO=" + sesion},
			Enlace:     regexp.MustCompile(`visita (https://\S+)`),
			Codigo:     regexp.MustCompile(`código ([A-Z0-9]{4}-[A-Z0-9]{4})\b`),
			PidePegar:  regexp.MustCompile(`Pega aquí el código >`),
		},
		Comprobadas: map[string][]string{"instalar": {"1.0.0"}},
	}}
}

// comprobarCon ejecuta «comprobar» con la ficha y la última versión dada (y el CLI falso en esa versión).
func comprobarCon(t *testing.T, f adaptadores.Ficha, version string) string {
	t.Helper()
	t.Setenv("FALSO_VERSION", version)
	ultima := func(context.Context, string) (string, error) { return version, nil }
	r := filepath.Join(t.TempDir(), "resultado.json")
	var salida bytes.Buffer
	if c := ejecutar(context.Background(), []adaptadores.Ficha{f}, ultima, []string{"comprobar", r}, &salida); c != 0 {
		t.Fatalf("comprobar terminó con %d: %s", c, salida.String())
	}
	return r
}

func juntarCon(t *testing.T, f adaptadores.Ficha, anterior, nueva, resultado string) (int, string) {
	t.Helper()
	var salida bytes.Buffer
	c := ejecutar(context.Background(), []adaptadores.Ficha{f}, nil, []string{"juntar", anterior, nueva, resultado}, &salida)
	return c, salida.String()
}

func leerLista(t *testing.T, ruta string) lista {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var l lista
	if err := json.Unmarshal(datos, &l); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestCompatibilidad(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	antes := sistemas
	sistemas = []string{runtime.GOOS} // el CLI falso solo corre en este sistema
	t.Cleanup(func() { sistemas = antes })
	// La lista publicada de verdad no se baja: falla al momento, sin salir a la red.
	antesURL := preparar.URLLista
	preparar.URLLista = "https://127.0.0.1:1/compatibilidad.json"
	t.Cleanup(func() { preparar.URLLista = antesURL })
	dir := t.TempDir()

	// Una versión que pasa: se añade para las cuatro acciones, sobre lo que trae la receta, y la lista empieza en 1.
	r := comprobarCon(t, fichaFalsa(t, "instalar", "sesion"), "1.2.3")
	primera := filepath.Join(dir, "primera.json")
	if c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), filepath.Join(dir, "no-hay.json"), primera, r); c != 0 {
		t.Fatalf("juntar terminó con %d: %s", c, salida)
	}
	l := leerLista(t, primera)
	if l.Version != 1 {
		t.Fatalf("version = %d, se esperaba 1", l.Version)
	}
	for _, accion := range []string{"instalar", "actualizar", "iniciar_sesion", "desinstalar"} {
		if !slices.Contains(l.Agentes["falso"][accion], "1.2.3") {
			t.Fatalf("1.2.3 no está para %s: %v", accion, l.Agentes["falso"])
		}
	}
	if !slices.Equal(l.Agentes["falso"]["instalar"], []string{"1.0.0", "1.2.3"}) {
		t.Fatalf("instalar = %v, se esperaba lo de la receta y lo nuevo, en orden", l.Agentes["falso"]["instalar"])
	}

	// Nada nuevo: no se escribe otra lista (el workflow no publica).
	igual := filepath.Join(dir, "igual.json")
	if c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), primera, igual, r); c != 0 {
		t.Fatalf("juntar sin nada nuevo terminó con %d: %s", c, salida)
	}
	if _, err := os.Stat(igual); !os.IsNotExist(err) {
		t.Fatalf("sin nada nuevo se escribió una lista: %v", err)
	}

	// Un inicio de sesión que cambió: 1.2.4 entra para lo que pasó, no para iniciar sesión, y sale con 2.
	muda := fichaFalsa(t, "instalar", "muda")
	r = comprobarCon(t, muda, "1.2.4")
	segunda := filepath.Join(dir, "segunda.json")
	c, salida := juntarCon(t, muda, primera, segunda, r)
	if c != salidaFallos || !strings.Contains(salida, "iniciar_sesion") {
		t.Fatalf("juntar = %d, %q; se esperaba %d con el inicio de sesión que falló", c, salida, salidaFallos)
	}
	l = leerLista(t, segunda)
	if l.Version != 2 || slices.Contains(l.Agentes["falso"]["iniciar_sesion"], "1.2.4") ||
		!slices.Contains(l.Agentes["falso"]["instalar"], "1.2.4") || !slices.Contains(l.Agentes["falso"]["iniciar_sesion"], "1.2.3") ||
		slices.Contains(l.Agentes["falso"]["desinstalar"], "1.2.4") { // tras el fallo no se siguió: desinstalar no se probó
		t.Fatalf("lista = %+v; se esperaba la version 2 con 1.2.4 sin iniciar sesión", l)
	}

	// Un instalador que falla: no se añade nada, no hay lista nueva, sale con 2 y el aviso tapa el código.
	r = comprobarCon(t, fichaFalsa(t, "fallar", "sesion"), "1.2.5")
	tercera := filepath.Join(dir, "tercera.json")
	c, salida = juntarCon(t, fichaFalsa(t, "fallar", "sesion"), segunda, tercera, r)
	if c != salidaFallos || !strings.Contains(salida, "instalar") || strings.Contains(salida, "ABCD-1234") || !strings.Contains(salida, "XXXX-XXXX") {
		t.Fatalf("juntar = %d, %q; se esperaba %d con el instalador que falló y el código tapado", c, salida, salidaFallos)
	}
	if _, err := os.Stat(tercera); !os.IsNotExist(err) {
		t.Fatalf("sin nada que añadir se escribió una lista: %v", err)
	}
}

// Sin el resultado de algún sistema no se añade nada y sale con 2.
func TestJuntarSinUnSistema(t *testing.T) {
	antes := sistemas
	sistemas = []string{"linux", "darwin", "windows"}
	t.Cleanup(func() { sistemas = antes })
	dir := t.TempDir()
	anterior := filepath.Join(dir, "anterior.json")
	if err := escribirJSON(anterior, lista{Version: 4, Agentes: map[string]map[string][]string{"falso": {"instalar": {"1.0.0"}}}}); err != nil {
		t.Fatal(err)
	}
	r := filepath.Join(dir, "linux.json")
	if err := escribirJSON(r, resultado{Sistema: "linux", Agentes: map[string]comprobacion{"falso": {Ultima: "2.0.0", Version: "2.0.0",
		Acciones: map[string]bool{"instalar": true}}}}); err != nil {
		t.Fatal(err)
	}
	nueva := filepath.Join(dir, "nueva.json")
	c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), anterior, nueva, r)
	if c != salidaFallos || !strings.Contains(salida, "darwin") || !strings.Contains(salida, "windows") {
		t.Fatalf("juntar = %d, %q; se esperaba %d sin el resultado de macOS ni Windows", c, salida, salidaFallos)
	}
	if _, err := os.Stat(nueva); !os.IsNotExist(err) {
		t.Fatalf("con un sistema sin resultado se escribió una lista: %v", err)
	}
}

func TestTapar(t *testing.T) {
	texto := tapar("visita https://github.com/login/device?code=1 y escribe 1A2B-3C4D (o QWER-TYUIO); el 2026-11-13", "")
	if strings.Contains(texto, "1A2B-3C4D") || strings.Contains(texto, "QWER-TYUIO") || strings.Contains(texto, "code=1") ||
		!strings.Contains(texto, "https://github.com/login/device") || !strings.Contains(texto, "2026-11-13") {
		t.Fatalf("tapar = %q", texto)
	}
}

// Una versión con otra forma no se firma: se cae de la lista anterior y la nueva sube de version.
func TestJuntarDescartaVersionesMalFormadas(t *testing.T) {
	antes := sistemas
	sistemas = []string{"linux"}
	t.Cleanup(func() { sistemas = antes })
	dir := t.TempDir()
	anterior := filepath.Join(dir, "anterior.json")
	if err := escribirJSON(anterior, lista{Version: 1, Agentes: map[string]map[string][]string{
		"falso": {"instalar": {"1.0.0", "1.0.111."}, "iniciar_sesion": {"1.0.111."}}}}); err != nil {
		t.Fatal(err)
	}
	r := filepath.Join(dir, "linux.json")
	if err := escribirJSON(r, resultado{Sistema: "linux", Agentes: map[string]comprobacion{"falso": {Ultima: "1.0.0", Version: "1.0.0",
		Acciones: map[string]bool{"instalar": true}}}}); err != nil {
		t.Fatal(err)
	}
	nueva := filepath.Join(dir, "nueva.json")
	if c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), anterior, nueva, r); c != 0 {
		t.Fatalf("juntar = %d, %q", c, salida)
	}
	l := leerLista(t, nueva)
	if l.Version != 2 || !slices.Equal(l.Agentes["falso"]["instalar"], []string{"1.0.0"}) || l.Agentes["falso"]["iniciar_sesion"] != nil {
		t.Fatalf("lista = %+v; se esperaba la version 2 sin 1.0.111.", l)
	}
}
