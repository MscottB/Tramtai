package agentes

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// quitarANSI es la expresión de m0.md que el lector aplica antes de buscar el enlace y el código.
var quitarANSI = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;?]*[ -/]*[@-~]`)

func TestRecetasCompletas(t *testing.T) {
	for _, id := range []string{"claude", "codex", "copilot", "antigravity"} {
		f, _ := Buscar(id)
		if f.Receta == nil {
			t.Fatalf("%s: sin receta", id)
		}
		for _, goos := range []string{"windows", "darwin", "linux"} {
			_, orden := f.Receta.Instalar[goos]
			_, descarga := f.Receta.Descarga[goos]
			if !orden && !descarga {
				t.Errorf("%s/%s: ni Instalar ni Descarga", id, goos)
			}
			if len(f.Receta.Rutas[goos]) == 0 {
				t.Errorf("%s/%s: sin Rutas", id, goos)
			}
		}
	}
	if f, _ := Buscar("gemini"); f.Receta != nil {
		t.Error("gemini no tiene receta")
	}
}

func TestRecetasSinSesionAntigravity(t *testing.T) {
	for _, f := range Todos() {
		if f.Receta == nil {
			continue
		}
		if (f.Nombre == "antigravity") != (f.Receta.Sesion == nil) {
			t.Errorf("%s: Sesion nil = %v", f.Nombre, f.Receta.Sesion == nil)
		}
		if f.Receta.Sesion != nil && !slices.Equal(f.InicioSesion, f.Receta.Sesion.Argumentos) {
			t.Errorf("%s: InicioSesion %v no sale de la receta %v", f.Nombre, f.InicioSesion, f.Receta.Sesion.Argumentos)
		}
	}
	if f, _ := Buscar("copilot"); !slices.Equal(f.InicioSesion, []string{"login", "--device-code"}) {
		t.Errorf("copilot: %v", f.InicioSesion)
	}
}

func TestLectoresConSalidaReal(t *testing.T) {
	casos := []struct {
		id, archivo, enlace, codigo string
		pegar                       bool
	}{
		{"claude", "sesion-windows.txt", "https://claude.com/cai/oauth/authorize?code=true&", "", true},
		{"claude", "sesion-linux-pty.txt", "https://claude.com/cai/oauth/authorize?code=true&", "", true},
		{"codex", "sesion-windows.txt", "https://auth.openai.com/codex/device", "ABCD-12345", false},
		{"codex", "sesion-sin-color.txt", "https://auth.openai.com/codex/device", "ABCD-12345", false},
		{"codex", "sesion-linux-pty.txt", "https://auth.openai.com/codex/device", "ABCD-12345", false},
		{"copilot", "sesion-windows.txt", "https://github.com/login/device", "1A2B-3C4D", false},
		{"copilot", "sesion-linux.txt", "https://github.com/login/device", "1A2B-3C4D", false},
		{"copilot", "sesion-linux-sin-navegador.txt", "https://github.com/login/device", "1A2B-3C4D", false},
		{"copilot", "sesion-linux-pty.txt", "https://github.com/login/device", "1A2B-3C4D", false},
	}
	for _, c := range casos {
		t.Run(c.id+"/"+c.archivo, func(t *testing.T) {
			f, _ := Buscar(c.id)
			l := f.Receta.Sesion
			bruto, err := os.ReadFile(filepath.Join(c.id, "testdata", c.archivo))
			if err != nil {
				t.Fatal(err)
			}
			texto := quitarANSI.ReplaceAllString(string(bruto), "")
			if m := l.Enlace.FindStringSubmatch(texto); m == nil || !strings.HasPrefix(m[1], c.enlace) || strings.ContainsAny(m[1], "\r\x1b") {
				t.Errorf("enlace: %q", m)
			}
			if c.codigo == "" {
				if l.Codigo != nil {
					t.Error("este CLI no imprime código")
				}
			} else if m := l.Codigo.FindStringSubmatch(texto); m == nil || m[1] != c.codigo {
				t.Errorf("código: %q", m)
			}
			if got := l.PidePegar != nil && l.PidePegar.MatchString(texto); got != c.pegar {
				t.Errorf("pide pegar = %v, quería %v", got, c.pegar)
			}
		})
	}
}

func TestComprobadasTienenLaDelM0(t *testing.T) {
	m0 := map[string][]string{
		"claude": {"2.1.311", "2.1.323"}, "codex": {"0.167.1", "0.167.2"},
		"copilot": {"1.0.108", "1.0.109"}, "antigravity": {"1.5.7"},
	}
	for id, versiones := range m0 {
		f, _ := Buscar(id)
		acciones := []string{"instalar", "actualizar", "iniciar_sesion", "desinstalar"}
		if f.Receta.Sesion == nil {
			acciones = []string{"instalar", "actualizar", "desinstalar"}
		}
		for _, a := range acciones {
			for _, v := range versiones {
				if !slices.Contains(f.Receta.Comprobadas[a], v) {
					t.Errorf("%s/%s: falta la versión %s del M0", id, a, v)
				}
			}
		}
		if f.Receta.Sesion == nil && len(f.Receta.Comprobadas["iniciar_sesion"]) != 0 {
			t.Errorf("%s: sin sesión no hay iniciar_sesion comprobada", id)
		}
	}
}
