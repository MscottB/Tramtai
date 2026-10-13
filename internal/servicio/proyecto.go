package servicio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tramtai/internal/nucleo"
)

const plantillaProyecto = `# Configuración de Tramtai para este proyecto (versionada). Las claves de seguridad, los puertos y los
# puntos de acceso van en .tramtai/local.yaml (no versionado) o en la configuración global.
version: 1
proyecto:
  nombre: %q
predeterminados:
  agente: claude
  perfil_modelo: equilibrado
  modo_contexto: aislado
  formato_salida: markdown
`

// PrepararProyecto crea .tramtai/ en dir: la configuración versionada, su .gitignore (local.yaml no se versiona)
// y un perfil de ejemplo con su esquema. Lo usan tramtai iniciar y la Web UI.
func PrepararProyecto(dir string) error {
	ruta := filepath.Join(dir, ".tramtai", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(ruta, []byte(fmt.Sprintf(plantillaProyecto, filepath.Base(dir))), 0o644); err != nil {
		return err
	}
	ignorar := filepath.Join(dir, ".tramtai", ".gitignore")
	if err := os.WriteFile(ignorar, []byte("# configuración propia de cada máquina\nlocal.yaml\n"), 0o644); err != nil {
		return err
	}
	// Un perfil de ejemplo con su esquema (sección 22.3): tramtai ejecutar --perfil-trabajo resumen "…"
	for nombre, contenido := range map[string]string{
		filepath.Join("profiles", "resumen.yaml"):       perfilEjemplo,
		filepath.Join("schemas", "resumen.schema.json"): esquemaEjemplo,
	} {
		destino := filepath.Join(dir, ".tramtai", nombre)
		if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(destino, []byte(contenido), 0o644); err != nil {
			return err
		}
	}
	_, err := nucleo.CargarCapa(ruta, nucleo.OrigenProyecto)
	return err
}

// expandirCasa cambia un ~ inicial por la carpeta personal, como hace la terminal en macOS y Linux.
func expandirCasa(ruta string) string {
	if ruta != "~" && !strings.HasPrefix(ruta, "~/") && !strings.HasPrefix(ruta, `~\`) {
		return ruta
	}
	casa, err := os.UserHomeDir()
	if err != nil {
		return ruta
	}
	return filepath.Join(casa, ruta[1:])
}

// AbrirProyecto activa la instancia del proyecto en raiz, una carpeta existente con ruta completa. Si la carpeta (o
// una superior) ya es un proyecto, usa ese; si no, la prepara antes.
func (s *Servicio) AbrirProyecto(raiz string) (*Instancia, error) {
	raiz = expandirCasa(strings.TrimSpace(raiz))
	if !filepath.IsAbs(raiz) {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "indica la ruta completa de la carpeta del proyecto")
	}
	info, err := os.Stat(raiz)
	if err != nil || !info.IsDir() {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "no existe la carpeta "+raiz)
	}
	// La ruta real (sin nombres cortos como ADMINI~1 ni enlaces): la misma carpeta da siempre el mismo espacio.
	if real, err := filepath.EvalSymlinks(raiz); err == nil {
		raiz = real
	}
	if existente, ok := nucleo.BuscarProyecto(raiz); ok {
		raiz = existente
	} else if err := PrepararProyecto(raiz); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "no se pudo preparar el proyecto: "+err.Error())
	}
	return s.ActivarProyecto(raiz)
}

const perfilEjemplo = `nombre: resumen
modo: perfil
instrucciones: |
  Resume el contenido en tres puntos y lista los pendientes. No inventes información.
salida:
  formato: json
  validacion_estricta: true
  esquema: ../schemas/resumen.schema.json
  conservar_salida_invalida: false
`

const esquemaEjemplo = `{
  "type": "object",
  "required": ["puntos", "pendientes"],
  "properties": {
    "puntos": {"type": "array", "items": {"type": "string"}, "maxItems": 3},
    "pendientes": {"type": "array", "items": {"type": "string"}}
  },
  "additionalProperties": false
}
`
