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
		{"claude", "sesion-windows-codigo-rechazado.txt", "https://claude.com/cai/oauth/authorize?code=true&", "", true},
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

func TestLectoresNoCasanSinSalida(t *testing.T) {
	sinSalida := map[string][]string{
		"claude":  {"", "Opening browser to sign in…\n", "Login failed: That code wasn't accepted. Sign in again to get a new code.\n"},
		"codex":   {"Not logged in\n", "Welcome to Codex [v0.167.2]\n"},
		"copilot": {"Waiting for authorization...\n", "Login failed: Error: Failed to request device code: Bad Request\n"},
	}
	for id, textos := range sinSalida {
		f, _ := Buscar(id)
		for _, texto := range textos {
			if m := f.Receta.Sesion.Enlace.FindStringSubmatch(texto); m != nil {
				t.Errorf("%s: %q no tiene enlace y casó %q", id, texto, m)
			}
			if c := f.Receta.Sesion.Codigo; c != nil && c.MatchString(texto) {
				t.Errorf("%s: %q no tiene código y casó", id, texto)
			}
		}
	}
}

func TestComodinDeComprobadas(t *testing.T) {
	formato := regexp.MustCompile(`^\d+\.\d+\.(\d+|\*)$`)
	for _, f := range Todos() {
		if f.Receta == nil {
			continue
		}
		for accion, versiones := range f.Receta.Comprobadas {
			for _, v := range versiones {
				if !formato.MatchString(v) {
					t.Errorf("%s/%s: versión %q sin forma exacta o 2.1.*", f.Nombre, accion, v)
				}
			}
		}
	}
}

func TestFijaVersionYURLsSeguras(t *testing.T) {
	for _, f := range Todos() {
		r := f.Receta
		if r == nil {
			continue
		}
		lleva := false
		for goos, o := range r.Instalar {
			lleva = lleva || slices.ContainsFunc(append(slices.Clone(o.Argumentos), o.Entorno...), func(s string) bool { return strings.Contains(s, "{version}") })
			if !strings.Contains(strings.Join(o.Argumentos, " "), "http") {
				continue // órdenes sin URL (copilot update…) no se miran
			}
			for _, a := range o.Argumentos {
				for _, url := range regexp.MustCompile(`https?://\S+`).FindAllString(a, -1) {
					if !strings.HasPrefix(url, "https://") {
						t.Errorf("%s/%s: %s no es https", f.Nombre, goos, url)
					}
				}
			}
		}
		for goos, d := range r.Descarga {
			lleva = lleva || strings.Contains(d.URL, "{version}")
			for _, u := range []string{d.URL, d.URLSumas} {
				if !strings.HasPrefix(u, "https://") {
					t.Errorf("%s/%s: %s no es https", f.Nombre, goos, u)
				}
			}
			if strings.Contains(d.URLSumas, "{arq}") {
				t.Errorf("%s/%s: URLSumas solo lleva {version}", f.Nombre, goos)
			}
		}
		if lleva != r.FijaVersion {
			t.Errorf("%s: FijaVersion = %v pero {version} en Instalar/Descarga = %v", f.Nombre, r.FijaVersion, lleva)
		}
		if !strings.HasPrefix(r.Ultima.Nombre, "https://") && r.Ultima.Tipo == "url" {
			t.Errorf("%s: Ultima no es https", f.Nombre)
		}
	}
	f, _ := Buscar("copilot")
	if u := f.Receta.Descarga["windows"].URL; !strings.Contains(u, "{arq}") || !strings.Contains(u, "{version}") || !strings.HasPrefix(u, "https://") {
		t.Errorf("copilot/windows: %s", u)
	}
}
