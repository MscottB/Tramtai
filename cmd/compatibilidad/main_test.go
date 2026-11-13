package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
)

// TestMain hace de CLI falso: con --version dice FALSO_VERSION y, con COMPATIBILIDAD_FALSO, de instalador o de
// inicio de sesión.
func TestMain(m *testing.M) {
	// Con FALSO_ENTORNO, cada CLI falso anota el GITHUB_TOKEN que le llega: el del job no puede llegar a ninguno.
	if ruta := os.Getenv("FALSO_ENTORNO"); ruta != "" {
		if f, err := os.OpenFile(ruta, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintf(f, "%v %s GITHUB_TOKEN=%q\n", os.Args[1:], os.Getenv("COMPATIBILIDAD_FALSO"), os.Getenv("GITHUB_TOKEN"))
			f.Close()
		}
	}
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
	case "limite": // como copilot update sin token en un runner con la IP compartida
		fmt.Println("Error during update: GitHub API rate limit exceeded.")
		os.Exit(1)
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

// juntarCon ejecuta «juntar» con la ficha y los resultados («sistema=ruta»).
func juntarCon(t *testing.T, f adaptadores.Ficha, anterior, nueva string, pares ...string) (int, string) {
	t.Helper()
	var salida bytes.Buffer
	c := ejecutar(context.Background(), []adaptadores.Ficha{f}, nil, append([]string{"juntar", anterior, nueva}, pares...), &salida)
	return c, salida.String()
}

// resultadoDe escribe el resultado de un sistema con lo de falso y devuelve su «sistema=ruta».
func resultadoDe(t *testing.T, sistema string, c comprobacion) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), sistema+".json")
	if err := escribirJSON(ruta, resultado{Sistema: sistema, Agentes: map[string]comprobacion{"falso": c}}); err != nil {
		t.Fatal(err)
	}
	return sistema + "=" + ruta
}

// todo es una comprobación en la que pasan las cuatro acciones.
func todo(version string) comprobacion {
	return comprobacion{Ultima: version, Version: version,
		Acciones: map[string]bool{"instalar": true, "actualizar": true, "iniciar_sesion": true, "desinstalar": true}}
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

func leerResultado(t *testing.T, par string) comprobacion {
	t.Helper()
	_, ruta, _ := strings.Cut(par, "=")
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var r resultado
	if err := json.Unmarshal(datos, &r); err != nil {
		t.Fatal(err)
	}
	return r.Agentes["falso"]
}

func noHay(t *testing.T, ruta string) {
	t.Helper()
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatalf("se escribió %s: %v", ruta, err)
	}
}

// conSistemas cambia los sistemas de la matriz durante la prueba.
func conSistemas(t *testing.T, lista ...string) {
	antes := sistemas
	sistemas = lista
	t.Cleanup(func() { sistemas = antes })
}

func TestCompatibilidad(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	conSistemas(t, runtime.GOOS) // el CLI falso solo corre en este sistema
	// El token del job está en el entorno de la prueba: ningún instalador ni CLI falso puede verlo.
	entorno := filepath.Join(t.TempDir(), "entorno.txt")
	t.Setenv("FALSO_ENTORNO", entorno)
	t.Setenv("GITHUB_TOKEN", "ghs_token_del_job_que_no_debe_llegar")
	defer func() {
		datos, err := os.ReadFile(entorno)
		if err != nil || !strings.Contains(string(datos), "login") || strings.Contains(string(datos), "ghs_token") {
			t.Errorf("lo que vieron los CLI falsos = %q, %v; se esperaba que se lanzaran y sin el token", datos, err)
		}
	}()
	dir := t.TempDir()
	comprobarEn := func(f adaptadores.Ficha, version string) string {
		return runtime.GOOS + "=" + comprobarCon(t, f, version)
	}

	// Una versión que pasa: se añade para las cuatro acciones, sobre lo que trae la receta, y la lista empieza en 1.
	r := comprobarEn(fichaFalsa(t, "instalar", "sesion"), "1.2.3")
	primera := filepath.Join(dir, "primera.json")
	if c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), filepath.Join(dir, "no-hay.json"), primera, r); c != 0 {
		t.Fatalf("juntar terminó con %d: %s", c, salida)
	}
	l := leerLista(t, primera)
	if l.Version != 1 {
		t.Fatalf("version = %d, se esperaba 1", l.Version)
	}
	for _, accion := range acciones {
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
	noHay(t, igual)

	// Un inicio de sesión que cambió: 1.2.4 entra para lo que pasó, no para iniciar sesión, y sale con 2.
	muda := fichaFalsa(t, "instalar", "muda")
	r = comprobarEn(muda, "1.2.4")
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
	r = comprobarEn(fichaFalsa(t, "fallar", "sesion"), "1.2.5")
	tercera := filepath.Join(dir, "tercera.json")
	c, salida = juntarCon(t, fichaFalsa(t, "fallar", "sesion"), segunda, tercera, r)
	if c != salidaFallos || !strings.Contains(salida, "instalar") || strings.Contains(salida, "ABCD-1234") || !strings.Contains(salida, "XXXX-XXXX") {
		t.Fatalf("juntar = %d, %q; se esperaba %d con el instalador que falló y el código tapado", c, salida, salidaFallos)
	}
	noHay(t, tercera)

	// actualizar choca con el límite de la API de GitHub: actualizar y desinstalar quedan sin concluir, sin fallo; entra
	// lo que pasó y no se avisa (Ruling 32).
	limite := fichaFalsa(t, "instalar", "sesion")
	limite.Receta.Actualizar[runtime.GOOS] = adaptadores.OrdenReceta{Programa: "falso", Entorno: []string{"COMPATIBILIDAD_FALSO=limite"}}
	r = comprobarEn(limite, "1.2.6")
	if res := leerResultado(t, r); res.Fallo != "" || !slices.Equal(res.SinConcluir, []string{"actualizar", "desinstalar"}) {
		t.Fatalf("resultado = %+v; se esperaba actualizar y desinstalar sin concluir y sin fallo", res)
	}
	cuarta := filepath.Join(dir, "cuarta.json")
	if c, salida := juntarCon(t, limite, segunda, cuarta, r); c != 0 || salida != "" {
		t.Fatalf("juntar = %d, %q; se esperaba 0 sin aviso", c, salida)
	}
	l = leerLista(t, cuarta)
	if !slices.Contains(l.Agentes["falso"]["iniciar_sesion"], "1.2.6") || slices.Contains(l.Agentes["falso"]["actualizar"], "1.2.6") ||
		slices.Contains(l.Agentes["falso"]["desinstalar"], "1.2.6") {
		t.Fatalf("lista = %+v; se esperaba 1.2.6 sin actualizar ni desinstalar", l)
	}
}

// Con los tres sistemas: lo que falla en uno no entra (lo demás sí); una versión distinta en uno no deja entrar nada.
func TestJuntarTresSistemas(t *testing.T) {
	conSistemas(t, "linux", "darwin", "windows")
	dir := t.TempDir()
	f := fichaFalsa(t, "instalar", "sesion")
	darwin := todo("2.0.0")
	darwin.Acciones["iniciar_sesion"], darwin.Fallo = false, "iniciar_sesion: error salida_no_reconocida"
	nueva := filepath.Join(dir, "nueva.json")
	c, salida := juntarCon(t, f, "no-hay.json", nueva, resultadoDe(t, "linux", todo("2.0.0")), resultadoDe(t, "darwin", darwin),
		resultadoDe(t, "windows", todo("2.0.0")))
	l := leerLista(t, nueva)
	if c != salidaFallos || !strings.Contains(salida, "darwin") || !slices.Contains(l.Agentes["falso"]["instalar"], "2.0.0") ||
		slices.Contains(l.Agentes["falso"]["iniciar_sesion"], "2.0.0") {
		t.Fatalf("juntar = %d, %q, %+v; se esperaba instalar sin iniciar_sesion y 2", c, salida, l)
	}

	otra := filepath.Join(dir, "otra.json")
	c, salida = juntarCon(t, f, nueva, otra, resultadoDe(t, "linux", todo("2.0.1")), resultadoDe(t, "darwin", todo("2.0.1")),
		resultadoDe(t, "windows", todo("2.0.2")))
	if c != salidaFallos || !strings.Contains(salida, "versiones distintas") {
		t.Fatalf("juntar = %d, %q; se esperaba %d con versiones distintas", c, salida, salidaFallos)
	}
	noHay(t, otra)
}

// Sin el resultado de algún sistema (o de ninguno) no se añade nada y sale con 2.
func TestJuntarSinUnSistema(t *testing.T) {
	conSistemas(t, "linux", "darwin", "windows")
	dir := t.TempDir()
	anterior := filepath.Join(dir, "anterior.json")
	if err := escribirJSON(anterior, lista{Version: 4, Agentes: map[string]map[string][]string{"falso": {"instalar": {"1.0.0"}}}}); err != nil {
		t.Fatal(err)
	}
	nueva := filepath.Join(dir, "nueva.json")
	c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), anterior, nueva, resultadoDe(t, "linux", todo("2.0.0")))
	if c != salidaFallos || !strings.Contains(salida, "darwin") || !strings.Contains(salida, "windows") {
		t.Fatalf("juntar = %d, %q; se esperaba %d sin el resultado de macOS ni Windows", c, salida, salidaFallos)
	}
	c, salida = juntarCon(t, fichaFalsa(t, "instalar", "sesion"), anterior, nueva)
	if c != salidaFallos || !strings.Contains(salida, "linux") {
		t.Fatalf("juntar sin resultados = %d, %q; se esperaba %d", c, salida, salidaFallos)
	}
	noHay(t, nueva)
}

// Un runner no habla por otro: no se acepta un sistema desconocido, repetido o que no es el suyo, ni acciones
// desconocidas.
func TestJuntarRechaza(t *testing.T) {
	conSistemas(t, "linux", "darwin", "windows")
	linux := resultadoDe(t, "linux", todo("2.0.0"))
	_, rutaLinux, _ := strings.Cut(linux, "=")
	rara := todo("2.0.0")
	rara.Acciones["borrar_todo"] = true
	sinConcluirRara := todo("2.0.0")
	sinConcluirRara.SinConcluir = []string{"formatear"}
	for nombre, pares := range map[string][]string{
		"sistema desconocido":      {"plan9=" + rutaLinux},
		"sin sistema":              {rutaLinux},
		"repetido":                 {linux, linux},
		"de otro sistema":          {"darwin=" + rutaLinux},
		"acción desconocida":       {resultadoDe(t, "linux", rara)},
		"sin concluir desconocida": {resultadoDe(t, "linux", sinConcluirRara)},
	} {
		nueva := filepath.Join(t.TempDir(), "nueva.json")
		if c, _ := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), "no-hay.json", nueva, pares...); c != 1 {
			t.Errorf("%s: juntar = %d, se esperaba 1", nombre, c)
		}
		noHay(t, nueva)
	}
}

// Una versión instalada con otra forma no se firma: no entra y se avisa, sin «"falso":{}» en la lista.
func TestJuntarVersionNoValida(t *testing.T) {
	conSistemas(t, "linux")
	anterior := filepath.Join(t.TempDir(), "anterior.json")
	if err := escribirJSON(anterior, lista{Version: 3, Agentes: map[string]map[string][]string{"otro": {"instalar": {"1.0.0"}}}}); err != nil {
		t.Fatal(err)
	}
	nueva := filepath.Join(t.TempDir(), "nueva.json")
	c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), anterior, nueva, resultadoDe(t, "linux", todo("2.0.0.*")))
	if c != salidaFallos || !strings.Contains(salida, "forma válida") || strings.Contains(salida, "2.0.0.*") {
		t.Fatalf("juntar = %d, %q; se esperaba %d con la versión no válida, sin enseñarla", c, salida, salidaFallos)
	}
	noHay(t, nueva)
}

// La última versión de GitHub: el límite de su API (403/429) deja las versiones distintas sin concluir, sin aviso; un
// 404, una etiqueta que no es una versión o sin red se avisan, y unas versiones distintas sin ese límite también
// (Rulings 32 y 33).
func TestJuntarSinUltimaDeGitHub(t *testing.T) {
	conSistemas(t, "linux", "darwin", "windows")
	f := fichaFalsa(t, "instalar", "sesion")
	f.Receta.Ultima = adaptadores.FuenteVersion{Tipo: "github", Nombre: "falso/falso"}
	anterior := filepath.Join(t.TempDir(), "anterior.json")
	// 1.0.0 ya está para todo: lo único nuevo sería 2.0.0, que no entra en ningún caso.
	if err := escribirJSON(anterior, lista{Version: 2, Agentes: map[string]map[string][]string{"falso": {"instalar": {"1.0.0"},
		"actualizar": {"1.0.0"}, "iniciar_sesion": {"1.0.0"}, "desinstalar": {"1.0.0"}}}}); err != nil {
		t.Fatal(err)
	}
	// sin es una comprobación que no supo la última versión e instaló la de la receta.
	sin := func(limite bool, fallo string) comprobacion {
		c := todo("1.0.0")
		c.Ultima, c.UltimaLimite, c.UltimaFallo = "", limite, fallo
		return c
	}
	for nombre, caso := range map[string]struct {
		linux, darwin, windows comprobacion
		aviso                  string
	}{
		"límite en un sistema":       {todo("2.0.0"), todo("2.0.0"), sin(true, ""), ""},
		"límite en todos":            {sin(true, ""), sin(true, ""), sin(true, ""), ""},
		"404 en todos":               {sin(false, "respondió 404"), sin(false, "respondió 404"), sin(false, "respondió 404"), "404"},
		"etiqueta no válida":         {sin(false, `la etiqueta "x" no es una versión válida`), sin(false, "x"), sin(false, "x"), "etiqueta"},
		"sin red en un sistema":      {todo("2.0.0"), todo("2.0.0"), sin(false, "dial tcp: sin red"), "versiones distintas"},
		"vacía sin motivo, distinta": {todo("2.0.0"), todo("2.0.0"), sin(false, ""), "versiones distintas"},
	} {
		nueva := filepath.Join(t.TempDir(), "nueva.json")
		c, salida := juntarCon(t, f, anterior, nueva, resultadoDe(t, "linux", caso.linux), resultadoDe(t, "darwin", caso.darwin),
			resultadoDe(t, "windows", caso.windows))
		switch {
		case caso.aviso == "" && (c != 0 || salida != ""):
			t.Errorf("%s: juntar = %d, %q; se esperaba 0 sin aviso", nombre, c, salida)
		case caso.aviso != "" && (c != salidaFallos || !strings.Contains(salida, caso.aviso)):
			t.Errorf("%s: juntar = %d, %q; se esperaba %d con %q", nombre, c, salida, salidaFallos, caso.aviso)
		}
		noHay(t, nueva)
	}
}

// ultimaGitHub lleva el token del job y distingue el límite de la API (sin concluir) de lo que se avisa.
func TestUltimaGitHub(t *testing.T) {
	var autorizacion string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		autorizacion = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/repos/limite/x/releases/latest":
			w.WriteHeader(http.StatusForbidden)
		case "/repos/demasiadas/x/releases/latest":
			w.WriteHeader(http.StatusTooManyRequests)
		case "/repos/mala/x/releases/latest":
			fmt.Fprint(w, `{"tag_name":"v1.0.0; rm -rf"}`)
		case "/repos/buena/x/releases/latest":
			fmt.Fprint(w, `{"tag_name":"rust-v0.169.0"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	antes, antesCliente := baseGitHub, clienteGitHub
	baseGitHub, clienteGitHub = srv.URL+"/repos/", srv.Client()
	t.Cleanup(func() { baseGitHub, clienteGitHub = antes, antesCliente })
	consultar := func(nombre string) comprobacion {
		v, err := ultimaGitHub(context.Background(), adaptadores.FuenteVersion{Tipo: "github", Nombre: nombre, Prefijo: "rust-v"}, "ghs_token")
		var c comprobacion
		anotarUltima(&c, v, err)
		return c
	}
	if c := consultar("buena/x"); c.Ultima != "0.169.0" || autorizacion != "Bearer ghs_token" {
		t.Fatalf("buena = %+v, Authorization %q", c, autorizacion)
	}
	for _, nombre := range []string{"limite/x", "demasiadas/x"} {
		if c := consultar(nombre); !c.UltimaLimite || c.UltimaFallo != "" || c.Ultima != "" {
			t.Errorf("%s = %+v; se esperaba el límite", nombre, c)
		}
	}
	for nombre, motivo := range map[string]string{"no-hay/x": "404", "mala/x": "no es una versión válida"} {
		if c := consultar(nombre); c.UltimaLimite || !strings.Contains(c.UltimaFallo, motivo) {
			t.Errorf("%s = %+v; se esperaba un fallo con %q", nombre, c, motivo)
		}
	}
}

// comprobar anota la última versión que no se supo con su motivo: el límite aparte.
func TestComprobarAnotaLaUltima(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("FALSO_VERSION", "1.0.0")
	f := fichaFalsa(t, "fallar", "sesion") // no hace falta instalar nada
	for _, caso := range []struct {
		err   error
		mirar func(comprobacion) bool
	}{
		{errorLimite{fmt.Errorf("respondió 429")}, func(c comprobacion) bool { return c.UltimaLimite && c.UltimaFallo == "" }},
		{fmt.Errorf("respondió 404"), func(c comprobacion) bool { return !c.UltimaLimite && strings.Contains(c.UltimaFallo, "404") }},
	} {
		r := filepath.Join(t.TempDir(), "resultado.json")
		ultima := func(context.Context, string) (string, error) { return "", caso.err }
		if c := ejecutar(context.Background(), []adaptadores.Ficha{f}, ultima, []string{"comprobar", r}, io.Discard); c != 0 {
			t.Fatalf("comprobar terminó con %d", c)
		}
		if c := leerResultado(t, runtime.GOOS+"="+r); !caso.mirar(c) {
			t.Errorf("con %v: resultado = %+v", caso.err, c)
		}
	}
}
func TestTapar(t *testing.T) {
	texto := tapar("visita https://github.com/login/device?code=1 y escribe 1A2B-3C4D (o QWER-TYUIO); el 2026-11-13", "")
	if strings.Contains(texto, "1A2B-3C4D") || strings.Contains(texto, "QWER-TYUIO") || strings.Contains(texto, "code=1") ||
		!strings.Contains(texto, "https://github.com/login/device") || !strings.Contains(texto, "2026-11-13") {
		t.Fatalf("tapar = %q", texto)
	}
}

// La valla del bloque es más larga que cualquier serie de comillas invertidas del texto: no se puede cerrar antes.
func TestBloque(t *testing.T) {
	if b := bloque("hola"); b != "```\nhola\n```" {
		t.Fatalf("bloque = %q", b)
	}
	if b := bloque("a ```` b ``` c"); !strings.HasPrefix(b, "`````\n") || !strings.HasSuffix(b, "\n`````") {
		t.Fatalf("bloque = %q", b)
	}
}

// Una versión con otra forma no se firma: se cae de la lista anterior (sin dejar «"otro":{}») y la nueva sube.
func TestJuntarDescartaVersionesMalFormadas(t *testing.T) {
	conSistemas(t, "linux")
	dir := t.TempDir()
	anterior := filepath.Join(dir, "anterior.json")
	if err := escribirJSON(anterior, lista{Version: 1, Agentes: map[string]map[string][]string{
		"falso": {"instalar": {"1.0.0", "1.0.111.", "1.1.*"}, "iniciar_sesion": {"1.0.111."}},
		"otro":  {"instalar": {"x."}}}}); err != nil {
		t.Fatal(err)
	}
	nueva := filepath.Join(dir, "nueva.json")
	r := resultadoDe(t, "linux", comprobacion{Ultima: "1.0.0", Version: "1.0.0", Acciones: map[string]bool{"instalar": true}})
	if c, salida := juntarCon(t, fichaFalsa(t, "instalar", "sesion"), anterior, nueva, r); c != 0 {
		t.Fatalf("juntar = %d, %q", c, salida)
	}
	l := leerLista(t, nueva)
	_, hayOtro := l.Agentes["otro"]
	_, haySesion := l.Agentes["falso"]["iniciar_sesion"]
	if l.Version != 2 || !slices.Equal(l.Agentes["falso"]["instalar"], []string{"1.0.0", "1.1.*"}) || haySesion || hayOtro {
		t.Fatalf("lista = %+v; se esperaba la version 2 sin 1.0.111. ni otro", l)
	}
}
