package servidor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tramtai/internal/idioma"
)

// conRutaPublica cambia, durante la prueba, dónde se publica el puerto: una carpeta temporal en lugar de /run/tramtai o
// %ProgramData%\Tramtai\publico. En Linux la carpeta la crea systemd (RuntimeDirectory); aquí, la prueba.
func conRutaPublica(t *testing.T) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "publico", "puerto")
	if runtime.GOOS != "windows" {
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	anterior := rutaPublica
	rutaPublica = func() string { return ruta }
	t.Cleanup(func() { rutaPublica = anterior })
	t.Setenv("TRAMTAI_SERVIDOR", "") // la prueba hace de Tramtai del usuario
	return ruta
}

// TestPuertoPublicado: el servicio del sistema publica su puerto y el Tramtai del usuario lo lee; lo leído no se cree a
// ciegas: solo un número de puerto (1-65535), y nada más.
func TestPuertoPublicado(t *testing.T) {
	ruta := conRutaPublica(t)
	if p, ok := PuertoDelSistema(); ok {
		t.Fatalf("sin publicar no hay puerto: %d", p)
	}
	for _, malo := range []int{0, -1, 65536} {
		if err := PublicarPuerto(malo); err == nil {
			t.Errorf("publicar el puerto %d", malo)
		}
	}
	if err := PublicarPuerto(17374); err != nil {
		t.Fatal(err)
	}
	if p, ok := PuertoDelSistema(); !ok || p != 17374 {
		t.Fatalf("publicado: %d %v", p, ok)
	}
	if err := PublicarPuerto(17375); err != nil { // al volver a arrancar en otro puerto se sustituye
		t.Fatal(err)
	}
	if p, _ := PuertoDelSistema(); p != 17375 {
		t.Fatalf("sustituido: %d", p)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(ruta); err != nil || info.Mode().Perm() != 0o644 {
			t.Errorf("el archivo lo leen todos (0644): %v %v", info.Mode(), err)
		}
	}
	for _, malo := range []string{"", "0", "65536", "-80", "+80", "80 81", "ochenta", "17374\n17375", strings.Repeat("1", 4096), "\x00"} {
		if err := os.WriteFile(ruta, []byte(malo), 0o644); err != nil {
			t.Fatal(err)
		}
		if p, ok := PuertoDelSistema(); ok {
			t.Errorf("%q no es un puerto: %d", malo, p)
		}
	}
	if err := os.WriteFile(ruta, []byte(" 8443\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, ok := PuertoDelSistema(); !ok || p != 8443 {
		t.Errorf("con espacios alrededor: %d %v", p, ok)
	}
}

// TestRetirarPuerto: al parar con normalidad, el servicio retira su puerto y el Tramtai del usuario deja de avisar;
// retirarlo sin haberlo publicado no es un error.
func TestRetirarPuerto(t *testing.T) {
	ruta := conRutaPublica(t)
	if err := PublicarPuerto(17374); err != nil {
		t.Fatal(err)
	}
	if err := RetirarPuerto(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Errorf("el archivo sigue: %v", err)
	}
	if a := AvisoCruzado(); a != "" {
		t.Errorf("sin puerto no hay aviso: %q", a)
	}
	if err := RetirarPuerto(); err != nil {
		t.Errorf("otra vez, sin archivo: %v", err)
	}
}

// TestAvisoCruzado: con un Tramtai del sistema en el equipo, el del usuario avisa de que es otro entorno, en el idioma de
// la interfaz; sin él, o en el propio servidor, no dice nada.
func TestAvisoCruzado(t *testing.T) {
	conRutaPublica(t)
	if a := AvisoCruzado(); a != "" {
		t.Fatalf("sin servidor no avisa: %q", a)
	}
	if err := PublicarPuerto(17374); err != nil {
		t.Fatal(err)
	}
	quiere := idioma.Tf("Hay otro Tramtai, del sistema, en el puerto %d: es otro entorno, con sus propias APIs y claves", 17374)
	if a := AvisoCruzado(); a != quiere || !strings.Contains(a, "17374") {
		t.Errorf("aviso: %q", a)
	}
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	if a := AvisoCruzado(); a != "" {
		t.Errorf("el propio servidor no se avisa a sí mismo: %q", a)
	}
}
