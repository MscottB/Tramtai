package main

import (
	"bytes"
	"strings"
	"testing"

	"tramtai/internal/terminos"
)

// ejecutarRaiz corre una orden de tramtai sobre una carpeta de datos nueva y devuelve su stdout, su stderr y su error.
func ejecutarRaiz(t *testing.T, datos string, args ...string) (salida, errores string, err error) {
	t.Helper()
	anterior := dirDatos
	dirDatos = func() (string, error) { return datos, nil }
	t.Cleanup(func() { dirDatos = anterior })
	raiz := nuevaRaiz()
	var o, e bytes.Buffer
	raiz.SetOut(&o)
	raiz.SetErr(&e)
	raiz.SetArgs(args)
	err = raiz.Execute()
	return o.String(), e.String(), err
}

// TestCLISinAceptarSigueFuncionando: la CLI sin interfaz no se bloquea (los scripts de quien actualiza siguen); solo
// avisa, en una línea de stderr, y la salida normal no cambia.
func TestCLISinAceptarSigueFuncionando(t *testing.T) {
	datos := t.TempDir()
	salida, errores, err := ejecutarRaiz(t, datos, "version")
	if err != nil {
		t.Fatalf("version sin aceptar debe terminar bien: %v", err)
	}
	if !strings.HasPrefix(salida, "tramtai ") || strings.Count(salida, "\n") != 1 {
		t.Errorf("la salida normal no cambia: %q", salida)
	}
	if lineas := strings.Split(strings.TrimSpace(errores), "\n"); len(lineas) != 1 || !strings.Contains(lineas[0], "tramtai terminos") {
		t.Errorf("stderr debe llevar una sola línea con «tramtai terminos»: %q", errores)
	}
	// Aceptados, el aviso desaparece.
	if err := terminos.Aceptar(datos, terminos.ComoCLI); err != nil {
		t.Fatal(err)
	}
	if _, errores, _ := ejecutarRaiz(t, datos, "version"); errores != "" {
		t.Errorf("con los términos aceptados no avisa: %q", errores)
	}
}

// TestTerminosNoAvisaDeSiMismo: leer o aceptar los términos no repite el aviso de que faltan.
func TestTerminosNoAvisaDeSiMismo(t *testing.T) {
	datos := t.TempDir()
	salida, errores, err := ejecutarRaiz(t, datos, "terms")
	if err != nil || errores != "" || salida != terminos.Texto {
		t.Errorf("tramtai terminos imprime TERMINOS.md sin avisos: err=%v stderr=%q", err, errores)
	}
	if _, errores, err := ejecutarRaiz(t, datos, "terminos", "aceptar"); err != nil || errores != "" {
		t.Errorf("aceptar: err=%v stderr=%q", err, errores)
	}
}

func TestTerminosAceptarCLI(t *testing.T) {
	datos := t.TempDir()
	if _, _, err := ejecutarRaiz(t, datos, "terminos", "aceptar"); err != nil {
		t.Fatal(err)
	}
	if a, _ := terminos.Leer(datos); a.Como != "cli" || a.Version != terminos.Version {
		t.Errorf("por defecto «cli»: %+v", a)
	}
	if _, _, err := ejecutarRaiz(t, datos, "terms", "accept", "--como", "instalador"); err != nil {
		t.Fatal(err)
	}
	if a, _ := terminos.Leer(datos); a.Como != "instalador" {
		t.Errorf("con --como instalador: %+v", a)
	}
	if _, _, err := ejecutarRaiz(t, datos, "terminos", "aceptar", "--como", "otro"); err == nil {
		t.Error("un --como desconocido se rechaza")
	}
}
