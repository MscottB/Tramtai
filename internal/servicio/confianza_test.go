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
