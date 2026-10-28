package agentes

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes/claude"
	"tramtai/internal/agentes/gemini"
	"tramtai/internal/configuracion"
	"tramtai/internal/credenciales"
	"tramtai/internal/nucleo"
)

func TestFichasCompletas(t *testing.T) {
	color := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	codigo := regexp.MustCompile(`^[A-Z]{2}$`)
	vistos := map[string]bool{}
	for _, f := range Todos() {
		if f.Nombre == "" || f.NombreVisible == "" || f.Programa == "" || !codigo.MatchString(f.Codigo) || f.Orden <= 0 ||
			!color.MatchString(f.ColorClaro) || !color.MatchString(f.ColorOscuro) || f.Crear == nil || f.URLInstalacion == "" ||
			(f.Streaming != "tokens" && f.Streaming != "final") || len(f.Autenticaciones) == 0 || len(f.VariablesClave) == 0 {
			t.Errorf("ficha incompleta: %+v", f)
		}
		for _, clave := range []string{"n:" + f.Nombre, "c:" + f.Codigo, "o:" + string(rune(f.Orden)), "cc:" + f.ColorClaro, "co:" + f.ColorOscuro} {
			if vistos[clave] {
				t.Errorf("%s repetido en %s", clave, f.Nombre)
			}
			vistos[clave] = true
		}
		if len(f.ValoresEsfuerzo) > len(nucleo.NivelesEsfuerzo) {
			t.Errorf("%s: más valores de esfuerzo que niveles", f.Nombre)
		}
		// La clave de cada agente se retira del entorno que heredan los demás CLI, y la primera (la que muestra
		// Agentes en «tramtai claves guardar») la admite el almacén de credenciales.
		for _, v := range f.VariablesClave {
			if !adaptadores.EsVariableCredencial(v) {
				t.Errorf("%s: %s no está entre las variables de credenciales", f.Nombre, v)
			}
		}
		if len(f.VariablesClave) > 0 && !credenciales.Admitido(f.VariablesClave[0]) {
			t.Errorf("%s: el almacén no admite %s (credenciales.Nombres)", f.Nombre, f.VariablesClave[0])
		}
		// Admitir la suscripción en la ficha exige que el adaptador la cambie en marcha (ConAutenticacion).
		if _, cambia := f.Crear(configuracion.Resolver(Internos())).(adaptadores.ConAutenticacion); cambia != slices.Contains(f.Autenticaciones, adaptadores.AutenticacionSuscripcion) {
			t.Errorf("%s: la ficha dice suscripción=%v y el adaptador ConAutenticacion=%v", f.Nombre, !cambia, cambia)
		}
	}
}

func TestRegistroYValoresPredeterminados(t *testing.T) {
	c := configuracion.Resolver(Internos())
	registro, limites := Registro(c)
	for _, f := range Todos() {
		if registro[f.Nombre] == nil || limites[f.Nombre] != f.Concurrencia {
			t.Errorf("%s: adaptador %v, límite %d", f.Nombre, registro[f.Nombre], limites[f.Nombre])
		}
		if f.Comando != "" && c.Texto("comandos_agentes."+f.Nombre) != f.Comando {
			t.Errorf("%s: comando predeterminado %q", f.Nombre, c.Texto("comandos_agentes."+f.Nombre))
		}
	}
}

// TestSimultaneasTope: adaptadores.<agente>.suscripcion_simultaneas va de 1 a 3 (1 por defecto); fuera de ahí la
// configuración no carga.
func TestSimultaneasTope(t *testing.T) {
	global := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("TRAMTAI_CONFIG_GLOBAL", global)
	if c, err := configuracion.CargarConfiguracion("", Internos()); err != nil || c.Entero("adaptadores.claude.suscripcion_simultaneas") != 1 {
		t.Fatalf("por defecto: %v", err)
	}
	for valor, vale := range map[string]bool{"0": false, "1": true, "3": true, "4": false} {
		if err := os.WriteFile(global, []byte("adaptadores:\n  claude:\n    suscripcion_simultaneas: "+valor+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := configuracion.CargarConfiguracion("", Internos())
		if vale != (err == nil) || !vale && (!strings.Contains(err.Error(), "suscripcion_simultaneas") || !strings.Contains(err.Error(), "1 a 3")) {
			t.Errorf("suscripcion_simultaneas: %s: %v", valor, err)
		}
	}
}

// La capa de los agentes no pisa los valores predeterminados de configuracion, que no saben de agentes.
func TestInternosNoPisanLosDeConfiguracion(t *testing.T) {
	propios := configuracion.Internos().Valores
	for clave := range Internos().Valores {
		if _, repetida := propios[clave]; repetida {
			t.Errorf("%s está en las dos capas internas", clave)
		}
	}
	if len(Internos().Valores) == 0 {
		t.Fatal("la capa de los agentes está vacía")
	}
}

func TestEsfuerzo(t *testing.T) {
	casos := []struct{ agente, nivel, valor string }{
		{"claude", "bajo", "low"},
		{"claude", "muy_alto", "xhigh"},
		{"claude", "maximo", "max"},
		{"codex", "medio", "medium"},
		{"codex", "maximo", "xhigh"}, // por encima de su máximo, el máximo
		{"gemini", "alto", ""},       // no admite esfuerzo
		{"claude", "", ""},
	}
	for _, c := range casos {
		f, _ := Buscar(c.agente)
		if v := adaptadores.ValorEsfuerzo(f.ValoresEsfuerzo, c.nivel); v != c.valor {
			t.Errorf("%s %s: %q, se esperaba %q", c.agente, c.nivel, v, c.valor)
		}
	}
	if n := Niveles("codex"); !slices.Equal(n, []string{"bajo", "medio", "alto", "muy_alto"}) {
		t.Errorf("niveles de codex: %v", n)
	}
	if n := Niveles("gemini"); len(n) != 0 {
		t.Errorf("gemini no admite esfuerzo: %v", n)
	}
}

func TestEsfuerzoPredeterminado(t *testing.T) {
	casos := map[[2]string]string{
		{"claude", "claude-opus-5-5"}:   "medio",
		{"claude", "claude-sonnet-5-5"}: "medio",
		{"claude", "claude-haiku-5-5"}:  "medio",
		{"claude", "claude-opus-4-7"}:   "muy_alto",
		{"claude", "claude-fable-5-1"}:  "alto",
		{"claude", "claude-opus-4-6"}:   "alto",
		{"claude", "claude-sonnet-6-0"}: "", // un modelo sin comprobar no recibe un valor supuesto
		{"claude", "claude-haiku-4-5"}:  "",
		{"claude", "opus"}:              "", // un alias puede cambiar de modelo: sin la versión real no se sabe
		{"claude", ""}:                  "",
		{"codex", "gpt-5.6-sol"}:        "", // Codex depende del modelo y no lo documenta por modelo
		{"gemini", "gemini-pro-latest"}: "",
		{"otro", "x"}:                   "",
	}
	for c, esperado := range casos {
		if obtenido := EsfuerzoPredeterminado(c[0], c[1]); obtenido != esperado {
			t.Errorf("%s/%s: %q, se esperaba %q", c[0], c[1], obtenido, esperado)
		}
	}
}

func TestNivelesDeModelo(t *testing.T) {
	if n := NivelesDeModelo("copilot", "auto"); n != nil {
		t.Fatalf("copilot/auto no admite esfuerzo: %v", n)
	}
	if n := NivelesDeModelo("copilot", ""); n != nil {
		t.Fatalf("el predeterminado de copilot es auto: %v", n)
	}
	if n := NivelesDeModelo("copilot", "gpt-6-luna"); len(n) != 5 {
		t.Fatalf("copilot con un modelo concreto: %v", n)
	}
	if n := NivelesDeModelo("claude", ""); len(n) != 5 {
		t.Fatalf("claude: %v", n)
	}
}

func TestSuscripcionYApps(t *testing.T) {
	if !AdmiteSuscripcion("claude") || !AdmiteSuscripcion("codex") || !AdmiteSuscripcion("copilot") || AdmiteSuscripcion("gemini") || AdmiteSuscripcion("antigravity") {
		t.Fatal("solo Claude, Codex y Copilot admiten la suscripción")
	}
	c := configuracion.Resolver()
	if PermitidoEnApps(c, "antigravity") || !PermitidoEnApps(c, "claude") || !PermitidoEnApps(c, "simulado") {
		t.Fatal("Antigravity no se permite en apps; los demás sí")
	}
	c = configuracion.Resolver(configuracion.Capa{Origen: configuracion.OrigenGlobal, Valores: map[string]any{"adaptadores.antigravity.permitir_apps": true}})
	if !PermitidoEnApps(c, "antigravity") {
		t.Fatal("permitir_apps habilita Antigravity")
	}
}

func TestCambiarAutenticacion(t *testing.T) {
	c := claude.Claude{Autenticacion: adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI)}
	registro := map[string]adaptadores.Adaptador{"claude": c, "gemini": gemini.Gemini{}}
	if err := adaptadores.CambiarAutenticacion(registro, "claude", adaptadores.AutenticacionSuscripcion); err != nil ||
		c.Autenticacion.Valor() != adaptadores.AutenticacionSuscripcion {
		t.Fatalf("%v %s", err, c.Autenticacion.Valor())
	}
	if adaptadores.CambiarAutenticacion(registro, "gemini", adaptadores.AutenticacionSuscripcion) == nil ||
		adaptadores.CambiarAutenticacion(registro, "claude", "otra") == nil {
		t.Fatal("gemini no admite suscripción y un modo desconocido se rechaza")
	}
	if (claude.Claude{}).Autenticacion.Valor() != adaptadores.AutenticacionClaveAPI {
		t.Fatal("sin modo, clave de API")
	}
}
