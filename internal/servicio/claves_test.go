package servicio

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
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

// La clave de servicio no la usa ninguna app (los clientes la releen en cada conexión): si está dañada, el servicio crea
// otra y lo anota en lugar de no arrancar. Las claves de los espacios y de las APIs siguen dando error.
func TestClaveDeServicioDanadaSeRota(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", filepath.Join(dir, "config.yaml"))
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	ruta := filepath.Join(dir, "token-servicio")
	os.WriteFile(ruta, []byte("corta\n"), 0o600)
	var registro strings.Builder
	s, err := Nuevo(Opciones{Datos: dir, Config: c, Registros: &registro})
	if err != nil {
		t.Fatalf("el servicio no arrancó por una clave de servicio dañada: %v", err)
	}
	t.Cleanup(s.Cerrar)
	if datos, _ := os.ReadFile(ruta); strings.TrimSpace(string(datos)) != s.tokenAdmin || len(s.tokenAdmin) < 32 {
		t.Fatalf("la clave nueva no se guardó: %q", datos)
	}
	if !strings.Contains(registro.String(), "token-servicio") {
		t.Fatalf("el cambio tiene que quedar en el registro: %q", registro.String())
	}
}

// La clave global que llega por variable de entorno exige un mínimo de 32 caracteres, sin romper a quien ya usa una de
// 64 en hexadecimal.
func TestClaveDeEntornoCorta(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", filepath.Join(dir, "config.yaml"))
	c, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", "1234")
	if _, _, err := HuellaInstancia(dir, "global", c); err == nil || !strings.Contains(err.Error(), "TRAMTAI_TOKEN_GLOBAL") {
		t.Fatalf("una clave corta por variable de entorno se rechaza con un mensaje claro: %v", err)
	}
	hex := strings.Repeat("a1", 32)
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", hex)
	if huella, _, err := HuellaInstancia(dir, "global", c); err != nil || huella != api.HuellaToken(hex) {
		t.Fatalf("una clave de 64 caracteres en hexadecimal sigue valiendo: %q %v", huella, err)
	}
	generada, _ := RotarToken(filepath.Join(dir, "otra"))
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", generada)
	if huella, origen, err := HuellaInstancia(dir, "global", c); err != nil || huella != api.HuellaToken(generada) || origen != "variable TRAMTAI_TOKEN_GLOBAL" {
		t.Fatalf("una clave como las generadas vale: %q %q %v", huella, origen, err)
	}
}

// Un archivo con la clave en claro (de antes de las huellas) pasa a huella al leerlo y la misma clave sigue entrando.
func TestHuellaInstanciaMigra(t *testing.T) {
	dir := t.TempDir()
	ruta := RutaTokenInstancia(dir, "pry")
	clave := "tt_" + strings.Repeat("ab", 32)
	os.MkdirAll(filepath.Dir(ruta), 0o755)
	os.WriteFile(ruta, []byte(clave+"\n"), 0o600)
	for i := 0; i < 2; i++ {
		huella, origen, err := HuellaInstancia(dir, "pry", nil)
		if err != nil || huella != api.HuellaToken(clave) || origen != ruta {
			t.Fatalf("llamada %d: %q %q %v", i+1, huella, origen, err)
		}
		if datos, _ := os.ReadFile(ruta); string(datos) != api.HuellaToken(clave)+"\n" {
			t.Fatalf("llamada %d: el archivo tenía que quedar con la huella: %q", i+1, datos)
		}
	}
}

// Un espacio sin archivo no tiene clave: se dice así y no se crea ninguna.
func TestHuellaInstanciaSinArchivo(t *testing.T) {
	dir := t.TempDir()
	huella, origen, err := HuellaInstancia(dir, "pry", nil)
	if err != nil || huella != "" || origen != RutaTokenInstancia(dir, "pry") {
		t.Fatalf("sin archivo: %q %q %v", huella, origen, err)
	}
	if _, err := os.Stat(origen); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("no se tenía que crear el archivo: %v", err)
	}
}

// Un archivo dañado no se toca: uno corto da error y uno con huella y basura detrás no deja entrar ninguna clave.
func TestHuellaInstanciaDanada(t *testing.T) {
	dir := t.TempDir()
	ruta := RutaTokenInstancia(dir, "pry")
	os.MkdirAll(filepath.Dir(ruta), 0o755)
	os.WriteFile(ruta, []byte("corta\n"), 0o600)
	if _, _, err := HuellaInstancia(dir, "pry", nil); err == nil {
		t.Fatal("un archivo corto tenía que dar error")
	}
	if datos, _ := os.ReadFile(ruta); string(datos) != "corta\n" {
		t.Fatalf("se reescribió el archivo: %q", datos)
	}
	basura := "sha256:" + strings.Repeat("zz", 32)
	os.WriteFile(ruta, []byte(basura+"\n"), 0o600)
	if huella, _, err := HuellaInstancia(dir, "pry", nil); err != nil || huella != basura {
		t.Fatalf("una huella con basura se devuelve tal cual: %q %v", huella, err)
	}
	if datos, _ := os.ReadFile(ruta); string(datos) != basura+"\n" {
		t.Fatalf("se reescribió el archivo: %q", datos)
	}
}

// La clave nueva se devuelve entera y en disco solo queda su huella.
func TestNuevaClave(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "pry")
	clave, err := NuevaClave(ruta)
	if err != nil || len(clave) != len("tt_")+64 || !strings.HasPrefix(clave, "tt_") {
		t.Fatalf("clave: %q %v", clave, err)
	}
	if datos, _ := os.ReadFile(ruta); string(datos) != api.HuellaToken(clave)+"\n" {
		t.Fatalf("en el archivo solo va la huella: %q", datos)
	}
}
