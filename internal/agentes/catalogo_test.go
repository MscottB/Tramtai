package agentes

import (
	"regexp"
	"slices"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes/claude"
	"tramtai/internal/agentes/gemini"
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
		// La clave de cada agente se retira del entorno que heredan los demás CLI.
		for _, v := range f.VariablesClave {
			if !adaptadores.EsVariableCredencial(v) {
				t.Errorf("%s: %s no está entre las variables de credenciales", f.Nombre, v)
			}
		}
	}
}

func TestRegistroYValoresPredeterminados(t *testing.T) {
	c := nucleo.Resolver()
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

func TestEsfuerzo(t *testing.T) {
	casos := []struct{ agente, nivel, valor string }{
		{"claude", "bajo", "low"}, {"claude", "maximo", "max"}, {"codex", "muy_alto", "xhigh"}, {"codex", "maximo", "xhigh"},
		{"gemini", "alto", ""}, {"claude", "", ""},
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
	for _, c := range [][3]string{{"claude", "claude-opus-5-5", "medio"}, {"claude", "claude-opus-4-7", "muy_alto"},
		{"claude", "claude-fable-5-1", "alto"}, {"claude", "modelo-nuevo", ""}, {"codex", "gpt-5.5", ""}, {"otro", "x", ""}} {
		if obtenido := EsfuerzoPredeterminado(c[0], c[1]); obtenido != c[2] {
			t.Errorf("%s %s: %q, se esperaba %q", c[0], c[1], obtenido, c[2])
		}
	}
}

func TestSuscripcionYApps(t *testing.T) {
	if !AdmiteSuscripcion("claude") || !AdmiteSuscripcion("codex") || AdmiteSuscripcion("gemini") || AdmiteSuscripcion("antigravity") {
		t.Fatal("solo Claude y Codex admiten la suscripción")
	}
	c := nucleo.Resolver()
	if PermitidoEnApps(c, "antigravity") || !PermitidoEnApps(c, "claude") || !PermitidoEnApps(c, "simulado") {
		t.Fatal("Antigravity no se permite en apps; los demás sí")
	}
	c = nucleo.Resolver(nucleo.Capa{Origen: nucleo.OrigenGlobal, Valores: map[string]any{"adaptadores.antigravity.permitir_apps": true}})
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
