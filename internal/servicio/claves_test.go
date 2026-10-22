package servicio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
)

// Una clave que existe pero no se puede usar (truncada, a medias) no se rota en silencio: las apps recibirían 401 sin
// causa visible y la clave vieja no volvería. Solo se crea si el archivo no existe.
func TestClaveIlegibleNoSeRota(t *testing.T) {
	dir := t.TempDir()
	for nombre, leer := range map[string]func(string) (string, error){"token": LeerOCrearToken, "huella": LeerHuellaPunto} {
		ruta := filepath.Join(dir, nombre)
		os.WriteFile(ruta, []byte("corta\n"), 0o600)
		if _, err := leer(ruta); err == nil {
			t.Errorf("%s: una clave truncada tenía que dar error", nombre)
		}
		if datos, _ := os.ReadFile(ruta); string(datos) != "corta\n" {
			t.Errorf("%s: se reescribió la clave: %q", nombre, datos)
		}
		nueva := filepath.Join(dir, nombre+"-nueva")
		if valor, err := leer(nueva); err != nil || valor == "" {
			t.Errorf("%s: sin archivo se crea una clave: %q %v", nombre, valor, err)
		}
	}
}

// La clave global que llega por variable de entorno exige un mínimo de 32 caracteres, sin romper a quien ya usa una de
// 64 en hexadecimal.
func TestClaveDeEntornoCorta(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", filepath.Join(dir, "config.yaml"))
	c, err := nucleo.CargarConfiguracion("")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", "1234")
	if _, _, err := TokenInstancia(dir, "global", c); err == nil || !strings.Contains(err.Error(), "TRAMTAI_TOKEN_GLOBAL") {
		t.Fatalf("una clave corta por variable de entorno se rechaza con un mensaje claro: %v", err)
	}
	hex := strings.Repeat("a1", 32)
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", hex)
	if valor, _, err := TokenInstancia(dir, "global", c); err != nil || valor != hex {
		t.Fatalf("una clave de 64 caracteres en hexadecimal sigue valiendo: %q %v", valor, err)
	}
	generada, _ := RotarToken(filepath.Join(dir, "otra"))
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", generada)
	if valor, origen, err := TokenInstancia(dir, "global", c); err != nil || valor != generada || origen != "variable TRAMTAI_TOKEN_GLOBAL" {
		t.Fatalf("una clave como las generadas vale: %q %q %v", valor, origen, err)
	}
}
