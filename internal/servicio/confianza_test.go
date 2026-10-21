package servicio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

func confiable(t *testing.T, raiz string) bool {
	t.Helper()
	g, err := nucleo.CargarConfiguracion("")
	if err != nil {
		t.Fatal(err)
	}
	return g.ProyectoConfiable(raiz)
}

// conTramtai deja en dir un .tramtai/ que no creó la app (un repositorio clonado), con local.yaml opcional.
func conTramtai(t *testing.T, dir, local string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(dir, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Clonado\n"), 0o600)
	if local != "" {
		os.WriteFile(filepath.Join(dir, ".tramtai", "local.yaml"), []byte(local), 0o600)
	}
}

func TestAbrirCreaYConfia(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{})
	if err != nil {
		t.Fatal(err)
	}
	if !confiable(t, inst.Raiz) {
		t.Fatal("un proyecto que crea la app debe quedar confiado")
	}
	err = EditarPuntos(filepath.Join(inst.Raiz, ".tramtai", "local.yaml"), nucleo.OrigenLocal, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		return append(lista, ConfigPunto{Nombre: "asistente"}), nil
	})
	if err != nil {
		t.Fatalf("en un proyecto confiado se pueden crear APIs: %v", err)
	}
}

func TestAbrirExistentePideConfianza(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	_, err := s.AbrirProyecto(dir, AbrirOpciones{})
	var ec *ErrorConfianza
	if !errors.As(err, &ec) || strings.Join(ec.Claves, ",") != "comandos_agentes" || nucleo.CodigoDe(err) != nucleo.CodigoConfianzaRequerida {
		t.Fatalf("debía pedir confianza: %v", err)
	}
	si := true
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{Confiar: &si})
	if err != nil {
		t.Fatal(err)
	}
	if !confiable(t, inst.Raiz) {
		t.Fatal("con Confiar debía quedar confiado")
	}
}

func TestAbrirSinConfiarIgnoraClavesAdministradas(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	no := false
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{Confiar: &no})
	if err != nil {
		t.Fatalf("sin confiar debía abrirse igual: %v", err)
	}
	if confiable(t, inst.Raiz) {
		t.Fatal("no debía quedar confiado")
	}
	if inst.Config.Texto("comandos_agentes.claude") == "otro.exe" {
		t.Fatal("las claves administradas de un local.yaml no confiado no se aplican")
	}
	if e, _ := s.registro().buscar(inst.ID); !strings.Contains(e.UltimoError, "local.yaml") {
		t.Fatalf("debía avisar de las claves ignoradas: %+v", e)
	}
}

// Un proyecto confiado que se mueve sigue siendo el mismo espacio (su marca lo dice), pero la confianza no viaja sola:
// una marca se copia en un repositorio ajeno. Al abrirlo se pregunta diciendo de dónde viene; «Buscar» no confía.
func TestMoverProyectoConfiadoPreguntaDeNuevo(t *testing.T) {
	for _, camino := range []string{"abrir", "ubicar"} {
		t.Run(camino, func(t *testing.T) {
			s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
			base := t.TempDir()
			a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
			os.MkdirAll(a, 0o700)
			inst, err := s.AbrirProyecto(a, AbrirOpciones{})
			if err != nil {
				t.Fatal(err)
			}
			id, vieja := inst.ID, inst.Raiz
			if err := os.Rename(a, b); err != nil {
				t.Fatal(err)
			}
			if camino == "abrir" {
				_, err = s.AbrirProyecto(b, AbrirOpciones{})
				var ec *ErrorConfianza
				if !errors.As(err, &ec) || !MismaRuta(ec.Movido, vieja) {
					t.Fatalf("un proyecto movido debía preguntar, diciendo de dónde viene: %v", err)
				}
				si := true
				inst, err = s.AbrirProyecto(b, AbrirOpciones{Confiar: &si})
			} else {
				inst, err = s.Ubicar(id, b)
			}
			if err != nil {
				t.Fatal(err)
			}
			if real, _ := filepath.EvalSymlinks(b); !MismaRuta(inst.Raiz, real) {
				t.Fatalf("el espacio sigue en %s y no en la carpeta nueva %s", inst.Raiz, real)
			}
			if inst.ID != id || confiable(t, inst.Raiz) != (camino == "abrir") {
				t.Fatalf("id %s (era %s), confiable %v", inst.ID, id, confiable(t, inst.Raiz))
			}
		})
	}
}

// Una carpeta con .tramtai/local.yaml pero sin config.yaml no es «nueva»: Tramtai no creó su .tramtai/, así que
// pregunta antes de confiar y no pisa lo que ya traía.
func TestAbrirCarpetaConSoloLocalPreguntaYNoPisa(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(dir, ".tramtai", "local.yaml"), []byte("comandos_agentes:\n  claude: malo.exe\n"), 0o600)
	os.WriteFile(filepath.Join(dir, ".tramtai", ".gitignore"), []byte("propio\n"), 0o600)
	_, err := s.AbrirProyecto(dir, AbrirOpciones{})
	var ec *ErrorConfianza
	if !errors.As(err, &ec) || strings.Join(ec.Claves, ",") != "comandos_agentes" {
		t.Fatalf("debía pedir confianza: %v", err)
	}
	no := false
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{Confiar: &no})
	if err != nil {
		t.Fatal(err)
	}
	if confiable(t, inst.Raiz) || inst.Config.Texto("comandos_agentes.claude") == "malo.exe" {
		t.Fatal("no debía confiar ni aplicar sus claves")
	}
	// La marca se añade a su .gitignore (no se versiona), pero lo que traía se conserva.
	if g, _ := os.ReadFile(filepath.Join(dir, ".tramtai", ".gitignore")); !strings.HasPrefix(string(g), "propio\n") {
		t.Fatalf("pisó el .gitignore que ya traía: %q", g)
	}
}

// En un proyecto no confiado cuyo local.yaml trae claves de seguridad se pueden guardar los ajustes del espacio (esas
// claves siguen ignoradas); lo que no se puede es añadir otra, como una API.
func TestProyectoNoConfiadoGuardaSusAjustes(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	no := false
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{Confiar: &no})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.guardarValores(inst, map[string]any{"predeterminados.modelo": "m1"}); err != nil {
		t.Fatalf("guardar el modelo del espacio: %v", err)
	}
	if err := s.heredar(inst, "predeterminados.modelo"); err != nil {
		t.Fatalf("usar el de Global: %v", err)
	}
	err = EditarPuntos(filepath.Join(inst.Raiz, ".tramtai", "local.yaml"), nucleo.OrigenLocal, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		return append(lista, ConfigPunto{Nombre: "asistente"}), nil
	})
	if err == nil {
		t.Fatal("en un proyecto no confiado no se pueden crear APIs")
	}
}

// El aviso de las claves ignoradas sigue tras reiniciar el servicio.
func TestAvisoDeClavesIgnoradasSobreviveAlReinicio(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	no := false
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{Confiar: &no})
	if err != nil {
		t.Fatal(err)
	}
	s.guardarActivas()
	s.Cerrar()
	s2, err := Nuevo(s.opc)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Cerrar()
	s2.Restaurar()
	if e, _ := s2.registro().buscar(inst.ID); !strings.Contains(e.UltimoError, "local.yaml") {
		t.Fatalf("tras reiniciar se perdió el aviso: %+v", e)
	}
}

// Una carpeta movida que no era confiable sigue sin serlo.
func TestMoverProyectoSinConfianzaSiguePreguntando(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	base := t.TempDir()
	a, b := filepath.Join(base, "a"), filepath.Join(base, "b")
	conTramtai(t, a, "comandos_agentes:\n  claude: otro.exe\n")
	no := false
	if _, err := s.AbrirProyecto(a, AbrirOpciones{Confiar: &no}); err != nil {
		t.Fatal(err)
	}
	os.Rename(a, b)
	var ec *ErrorConfianza
	if _, err := s.AbrirProyecto(b, AbrirOpciones{}); !errors.As(err, &ec) {
		t.Fatalf("debía seguir pidiendo confianza: %v", err)
	}
}

func TestAbrirDentroDeOtroProyecto(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	sub := filepath.Join(proyecto, "sub")
	os.MkdirAll(sub, 0o700)
	_, err := s.AbrirProyecto(sub, AbrirOpciones{})
	var ed *ErrorDentroDeProyecto
	real, _ := filepath.EvalSymlinks(proyecto)
	if !errors.As(err, &ed) || !MismaRuta(ed.Proyecto, real) || nucleo.CodigoDe(err) != nucleo.CodigoDentroDeProyecto {
		t.Fatalf("debía avisar de que está dentro de otro proyecto: %v", err)
	}
	inst, err := s.AbrirProyecto(sub, AbrirOpciones{CrearAqui: true})
	if err != nil {
		t.Fatal(err)
	}
	if realSub, _ := filepath.EvalSymlinks(sub); !MismaRuta(inst.Raiz, realSub) || !esProyecto(sub) {
		t.Fatalf("con CrearAqui debía crear el proyecto en la subcarpeta: %s", inst.Raiz)
	}
}

func TestDejarDeConfiar(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.AbrirProyecto(t.TempDir(), AbrirOpciones{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FijarConfianza(inst.Raiz, false); err != nil {
		t.Fatal(err)
	}
	if confiable(t, inst.Raiz) {
		t.Fatal("debía dejar de ser confiable")
	}
	if err := s.FijarConfianza(inst.Raiz, true); err != nil || !confiable(t, inst.Raiz) {
		t.Fatalf("volver a confiar: %v", err)
	}
}

func TestAbrirProyectoUI409(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	n := abrirSesion(t, s, ts.URL)
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	r, cuerpo := n.pedir("POST", "/ui/proyectos", `{"raiz":`+jsonTexto(dir)+`}`, "X-CSRF-Token", n.csrf)
	if r.StatusCode != 409 || !strings.Contains(cuerpo, "confianza_requerida") || !strings.Contains(cuerpo, `"claves":["comandos_agentes"]`) {
		t.Fatalf("respuesta: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo = n.pedir("POST", "/ui/proyectos", `{"raiz":`+jsonTexto(dir)+`,"confiar":true}`, "X-CSRF-Token", n.csrf)
	if r.StatusCode != 200 {
		t.Fatalf("con confiar: %d %s", r.StatusCode, cuerpo)
	}
}

func jsonTexto(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
