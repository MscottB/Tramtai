package nucleo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escribir(t *testing.T, contenido string) string {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(ruta, []byte(contenido), 0o600); err != nil {
		t.Fatal(err)
	}
	return ruta
}

func cargar(t *testing.T, contenido string, origen Origen) Capa {
	t.Helper()
	capa, err := CargarCapa(escribir(t, contenido), origen)
	if err != nil {
		t.Fatalf("cargar capa %s: %v", origen, err)
	}
	return capa
}

func TestPrecedenciaUnica(t *testing.T) {
	global := cargar(t, "concurrencia:\n  maximo_tamano_cola: 10\n  maximo_trabajos_instancia: 3\npredeterminados:\n  agente: codex\n", OrigenGlobal)
	proyecto := cargar(t, "version: 1\npredeterminados:\n  agente: claude\n  perfil_modelo: equilibrado\n", OrigenProyecto)
	local := cargar(t, "concurrencia:\n  maximo_tamano_cola: 5\nservicio:\n  permitir_remoto: false\n", OrigenLocal)
	entorno, err := CapaEntorno([]string{"TRAMTAI_CONCURRENCIA__MAXIMO_TAMANO_COLA=7", "TRAMTAI_TOKEN_GLOBAL=x", "PATH=y"})
	if err != nil {
		t.Fatal(err)
	}
	c := Resolver(entorno, local, proyecto, global) // orden invertido a propósito
	casos := []struct {
		clave  string
		valor  any
		origen Origen
	}{
		{"concurrencia.maximo_tamano_cola", 7, OrigenEntorno},
		{"concurrencia.maximo_trabajos_instancia", 3, OrigenGlobal},
		{"predeterminados.agente", "claude", OrigenProyecto},
		{"servicio.permitir_remoto", false, OrigenLocal},
		{"servicio.puerto", 17373, OrigenInterno},
	}
	for _, caso := range casos {
		if got := c.valores[caso.clave]; got != caso.valor || c.Origen(caso.clave) != caso.origen {
			t.Errorf("%s = %v (%s); se esperaba %v (%s)", caso.clave, got, c.Origen(caso.clave), caso.valor, caso.origen)
		}
	}
	if efectiva := c.Efectiva(); efectiva[0].Clave > efectiva[1].Clave {
		t.Error("la vista efectiva debe estar ordenada por clave")
	}
}

func TestClavesRechazadas(t *testing.T) {
	casos := []struct {
		nombre, contenido string
		origen            Origen
		mensaje           string
	}{
		{"desconocida", "concurrencia:\n  inventada: 1\n", OrigenGlobal, "clave desconocida"},
		{"seguridad en versionada", "servicio:\n  permitir_remoto: true\n", OrigenProyecto, "clave de seguridad"},
		{"puntos de acceso en versionada", "puntos_acceso: []\n", OrigenProyecto, "clave de seguridad"},
		{"version no soportada", "version: 2\n", OrigenGlobal, "versión de formato"},
		{"tipo incorrecto", "concurrencia:\n  maximo_tamano_cola: muchos\n", OrigenGlobal, "tipo incorrecto"},
		{"yaml roto", "concurrencia: [\n", OrigenGlobal, "YAML no válido"},
	}
	for _, caso := range casos {
		ruta := escribir(t, caso.contenido)
		_, err := CargarCapa(ruta, caso.origen)
		if CodigoDe(err) != CodigoConfiguracionInvalida || !strings.Contains(err.Error(), caso.mensaje) || !strings.Contains(err.Error(), ruta) {
			t.Errorf("%s: error inesperado %v", caso.nombre, err)
		}
	}
	if _, err := CargarCapa(escribir(t, "servicio:\n  permitir_remoto: true\n"), OrigenLocal); err != nil {
		t.Errorf("la configuración local sí admite claves de seguridad: %v", err)
	}
	if capa, err := CargarCapa(filepath.Join(t.TempDir(), "no-existe.yaml"), OrigenGlobal); err != nil || len(capa.Valores) != 0 {
		t.Errorf("un archivo inexistente debe dar una capa vacía: %v", err)
	}
	if _, err := CapaEntorno([]string{"TRAMTAI_NO__EXISTE=1"}); CodigoDe(err) != CodigoConfiguracionInvalida {
		t.Errorf("una variable TRAMTAI_ desconocida debe rechazarse: %v", err)
	}
}

func TestAplicarPredeterminados(t *testing.T) {
	c := Resolver(cargar(t, "predeterminados:\n  agente: claude\n  perfil_modelo: equilibrado\n", OrigenGlobal))
	explicita := c.AplicarPredeterminados(SolicitudEjecucion{Agente: "codex", Modelo: "m1", Mensajes: []Mensaje{{Rol: "usuario", Contenido: "x"}}})
	if explicita.Agente != "codex" || explicita.PerfilModelo != "" || explicita.ModoContexto != "aislado" || explicita.Salida.Formato != "markdown" {
		t.Errorf("los campos explícitos deben prevalecer y los vacíos completarse: %+v", explicita)
	}
	vacia := c.AplicarPredeterminados(SolicitudEjecucion{})
	if vacia.Agente != "claude" || vacia.PerfilModelo != "equilibrado" {
		t.Errorf("los vacíos deben tomar los predeterminados: %+v", vacia)
	}
}
