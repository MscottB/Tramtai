package nucleo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"slices"

	"tramtai/internal/idioma"
)

// Mensaje es un turno de la conversación con su rol original.
type Mensaje struct {
	Rol       string `json:"rol"`
	Contenido string `json:"contenido"`
}

// ConfigSalida describe el formato pedido por el consumidor.
type ConfigSalida struct {
	Formato            string `json:"formato,omitempty"`
	ValidacionEstricta bool   `json:"validacion_estricta,omitempty"`
	// Esquema es el JSON Schema que debe cumplir la salida (sección 11); exige formato json.
	Esquema json.RawMessage `json:"esquema,omitempty"`
	// ConservarSalidaInvalida guarda la salida que no cumple el esquema, para revisarla, con la misma retención.
	ConservarSalidaInvalida bool `json:"conservar_salida_invalida,omitempty"`
}

// ConfigEjecucion agrupa los límites de ejecución pedidos.
type ConfigEjecucion struct {
	TiempoLimiteSegundos int  `json:"tiempo_limite_segundos,omitempty"`
	PersistirSesion      bool `json:"persistir_sesion,omitempty"`
}

// SolicitudEjecucion es el contrato interno único (sección 4.4).
type SolicitudEjecucion struct {
	Agente       string `json:"agente,omitempty"`
	Modelo       string `json:"modelo,omitempty"`
	PerfilModelo string `json:"perfil_modelo,omitempty"`
	// Esfuerzo es cuánto razona el agente (bajo, medio, alto, muy_alto, maximo); vacío deja el de su CLI. Los
	// agentes que no lo admiten lo ignoran.
	Esfuerzo string `json:"esfuerzo,omitempty"`
	// Perfil es un perfil de ejecución explícito (profiles/<nombre>.yaml); se aplica al aceptar y queda en la fotografía.
	Perfil       string `json:"perfil,omitempty"`
	ModoContexto string `json:"modo_contexto,omitempty"`
	// RaizProyecto la fija el planificador en modo consciente_proyecto; no viene del cliente (sección 6.3).
	RaizProyecto  string          `json:"-"`
	Instrucciones string          `json:"instrucciones,omitempty"`
	Mensajes      []Mensaje       `json:"mensajes,omitempty"`
	Cargas        []string        `json:"cargas,omitempty"`
	Salida        ConfigSalida    `json:"salida"`
	Ejecucion     ConfigEjecucion `json:"ejecucion"`
}

const (
	maximoTiempoLimiteSegundos = 24 * 60 * 60
	maximoCargas               = 20
)

// ReCarga es el formato de un identificador de carga: impide rutas u otros valores en su lugar.
var ReCarga = regexp.MustCompile(`^carga_[0-9a-f]{32}$`)

var (
	rolesValidos    = map[string]bool{"sistema": true, "usuario": true, "asistente": true}
	formatosValidos = map[string]bool{"texto": true, "markdown": true, "json": true}
	modosContexto   = map[string]bool{"aislado": true, "consciente_proyecto": true}
	// NivelesEsfuerzo, de menor a mayor.
	NivelesEsfuerzo = []string{"bajo", "medio", "alto", "muy_alto", "maximo"}
	// El modelo es una cadena validada por tamaño y formato técnico, no un enum cerrado (sección 9.2).
	reIdentificador = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,127}$`)
)

// Validar rechaza una solicitud inválida antes de aceptarla; nunca se lanza un proceso por ella.
func (s SolicitudEjecucion) Validar() error {
	invalida := func(mensaje string) error {
		return NuevoError(CodigoSolicitudInvalida, mensaje)
	}
	if s.Modelo != "" && s.PerfilModelo != "" {
		return invalida(idioma.T("modelo y perfil_modelo son excluyentes"))
	}
	for campo, valor := range map[string]string{"agente": s.Agente, "modelo": s.Modelo, "perfil_modelo": s.PerfilModelo} {
		if valor != "" && !reIdentificador.MatchString(valor) {
			return invalida(idioma.Tf("%s con formato no válido", campo))
		}
	}
	for i, m := range s.Mensajes {
		if !rolesValidos[m.Rol] {
			return invalida(idioma.Tf("mensaje %d con rol desconocido %q", i, m.Rol))
		}
		if m.Contenido == "" {
			return invalida(idioma.Tf("mensaje %d sin contenido", i))
		}
	}
	if s.Esfuerzo != "" && !slices.Contains(NivelesEsfuerzo, s.Esfuerzo) {
		return invalida(idioma.Tf("esfuerzo desconocido %q (bajo, medio, alto, muy_alto, maximo)", s.Esfuerzo))
	}
	if s.ModoContexto != "" && !modosContexto[s.ModoContexto] {
		return invalida(idioma.Tf("modo_contexto desconocido %q", s.ModoContexto))
	}
	if s.Salida.Formato != "" && !formatosValidos[s.Salida.Formato] {
		return invalida(idioma.Tf("formato de salida desconocido %q", s.Salida.Formato))
	}
	if len(s.Salida.Esquema) > 0 {
		if s.Salida.Formato != "json" {
			return invalida(idioma.T("un esquema de salida exige formato json"))
		}
		if !json.Valid(s.Salida.Esquema) {
			return invalida(idioma.T("el esquema de salida no es JSON válido"))
		}
	}
	if len(s.Cargas) > maximoCargas {
		return invalida(idioma.Tf("demasiadas cargas (máximo %d)", maximoCargas))
	}
	for i, c := range s.Cargas {
		if !ReCarga.MatchString(c) {
			return invalida(idioma.Tf("carga %d con identificador no válido", i))
		}
	}
	if len(s.Mensajes) == 0 && s.Instrucciones == "" && len(s.Cargas) == 0 {
		return invalida(idioma.T("la solicitud no tiene mensajes, instrucciones ni cargas"))
	}
	if t := s.Ejecucion.TiempoLimiteSegundos; t < 0 || t > maximoTiempoLimiteSegundos {
		return invalida(idioma.T("tiempo_limite_segundos fuera de rango"))
	}
	return nil
}

// Huella resume la solicitud normalizada para la idempotencia sin guardar su texto (sección 7.2).
func (s SolicitudEjecucion) Huella() string {
	datos, _ := json.Marshal(s) // el orden fijo de los campos hace la serialización determinista
	suma := sha256.Sum256(datos)
	return hex.EncodeToString(suma[:])
}
