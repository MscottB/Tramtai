package nucleo

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"tramtai/internal/idioma"
)

// Perfil es un perfil de ejecución explícito (sección 22.3): un archivo <nombre>.yaml en .tramtai/profiles/ del
// proyecto o en profiles/ junto a la configuración global. Solo se aplica si la solicitud lo pide.
type Perfil struct {
	Nombre        string       `yaml:"nombre"`
	Modo          string       `yaml:"modo"`
	Agente        string       `yaml:"agente"`
	PerfilModelo  string       `yaml:"perfil_modelo"`
	ModoContexto  string       `yaml:"modo_contexto"`
	Instrucciones string       `yaml:"instrucciones"`
	Salida        SalidaPerfil `yaml:"salida"`
}

// SalidaPerfil es la salida que fija el perfil; el esquema es una ruta relativa al archivo del perfil.
type SalidaPerfil struct {
	Formato                 string `yaml:"formato"`
	ValidacionEstricta      bool   `yaml:"validacion_estricta"`
	Esquema                 string `yaml:"esquema"`
	RepararAutomaticamente  bool   `yaml:"reparar_automaticamente"`
	ConservarSalidaInvalida bool   `yaml:"conservar_salida_invalida"`
}

// carpetaPerfiles es dónde se buscan perfiles y el límite del que no pueden salir sus esquemas.
type carpetaPerfiles struct{ dir, limite string }

// maximoEsquema acota el JSON Schema que se lee de disco.
const maximoEsquema = 1 << 20

// AplicarPerfil completa la solicitud con el perfil que pide; los campos explícitos de la solicitud prevalecen
// (sección 12.2). El perfil del proyecto tiene prioridad sobre uno global con el mismo nombre.
func (c *Configuracion) AplicarPerfil(s SolicitudEjecucion) (SolicitudEjecucion, error) {
	if s.Perfil == "" {
		return s, nil
	}
	if !reIdentificador.MatchString(s.Perfil) || strings.ContainsAny(s.Perfil, "/\\:") || strings.Contains(s.Perfil, "..") {
		return s, NuevoError(CodigoSolicitudInvalida, idioma.T("perfil con formato no válido"))
	}
	p, carpeta, err := c.buscarPerfil(s.Perfil)
	if err != nil {
		return s, err
	}
	if p.Salida.RepararAutomaticamente {
		return s, errorConfiguracion(s.Perfil, "salida.reparar_automaticamente", idioma.T("no soportado: no se repite la llamada sin autorización (sección 11)"))
	}
	if s.Agente == "" {
		s.Agente = p.Agente
	}
	if s.Modelo == "" && s.PerfilModelo == "" {
		s.PerfilModelo = p.PerfilModelo
	}
	if s.ModoContexto == "" {
		s.ModoContexto = p.ModoContexto
	}
	if s.Instrucciones == "" {
		s.Instrucciones = strings.TrimSpace(p.Instrucciones)
	}
	if s.Salida.Formato == "" {
		s.Salida.Formato = p.Salida.Formato
	}
	s.Salida.ValidacionEstricta = s.Salida.ValidacionEstricta || p.Salida.ValidacionEstricta
	s.Salida.ConservarSalidaInvalida = s.Salida.ConservarSalidaInvalida || p.Salida.ConservarSalidaInvalida
	if len(s.Salida.Esquema) == 0 && p.Salida.Esquema != "" {
		if s.Salida.Esquema, err = leerEsquema(carpeta, p.Salida.Esquema); err != nil {
			return s, err
		}
	}
	return s, nil
}

func (c *Configuracion) buscarPerfil(nombre string) (Perfil, carpetaPerfiles, error) {
	for _, carpeta := range c.perfiles {
		datos, err := os.ReadFile(filepath.Join(carpeta.dir, nombre+".yaml"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return Perfil{}, carpeta, err
		}
		var p Perfil
		d := yaml.NewDecoder(bytes.NewReader(datos))
		d.KnownFields(true)
		if err := d.Decode(&p); err != nil {
			return p, carpeta, errorConfiguracion(filepath.Join(carpeta.dir, nombre+".yaml"), "", idioma.Tf("perfil no válido: %s", err.Error()))
		}
		return p, carpeta, nil
	}
	return Perfil{}, carpetaPerfiles{}, NuevoError(CodigoSolicitudInvalida, idioma.Tf("no existe el perfil %s (profiles/%s.yaml)", nombre, nombre))
}

// leerEsquema lee el esquema del perfil sin salir de su carpeta de configuración (ni con ../ ni con enlaces).
func leerEsquema(carpeta carpetaPerfiles, relativa string) (json.RawMessage, error) {
	fuera := NuevoError(CodigoConfiguracionInvalida, idioma.Tf("el esquema %s queda fuera de la carpeta de configuración", relativa))
	ruta, err := filepath.EvalSymlinks(filepath.Join(carpeta.dir, relativa))
	if err != nil {
		return nil, NuevoError(CodigoConfiguracionInvalida, idioma.Tf("no se encontró el esquema %s", relativa))
	}
	limite, err := filepath.EvalSymlinks(carpeta.limite)
	if err != nil {
		return nil, fuera
	}
	if rel, err := filepath.Rel(limite, ruta); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fuera
	}
	info, err := os.Stat(ruta)
	if err != nil || info.Size() > maximoEsquema {
		return nil, NuevoError(CodigoConfiguracionInvalida, idioma.Tf("el esquema %s no se puede leer o supera 1 MB", relativa))
	}
	datos, err := os.ReadFile(ruta)
	if err != nil || !json.Valid(datos) {
		return nil, NuevoError(CodigoConfiguracionInvalida, idioma.Tf("el esquema %s no es JSON válido", relativa))
	}
	return datos, nil
}
