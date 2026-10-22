package nucleo

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"

	"tramtai/internal/idioma"
)

// Mensaje es un turno de la conversación con su rol original.
type Mensaje struct {
	Rol       string `json:"rol"`
	Contenido string `json:"contenido"`
	// Llamadas son las herramientas que pidió el agente en un turno «asistente» anterior.
	Llamadas []Llamada `json:"llamadas,omitempty"`
	// IDLlamada y EsError son de un turno «herramienta»: el resultado de esa llamada, y si falló.
	IDLlamada string `json:"id_llamada,omitempty"`
	EsError   bool   `json:"es_error,omitempty"`
}

// Herramienta es una función que ofrece la aplicación; Parametros es su JSON Schema (opcional).
type Herramienta struct {
	Nombre      string          `json:"nombre"`
	Descripcion string          `json:"descripcion,omitempty"`
	Parametros  json.RawMessage `json:"parametros,omitempty"`
}

// Llamada es una herramienta que pidió el agente, con sus argumentos como objeto JSON compacto.
type Llamada struct {
	ID         string          `json:"id"`
	Nombre     string          `json:"nombre"`
	Argumentos json.RawMessage `json:"argumentos" ts:"Record<string, unknown>"`
}

// UnmarshalJSON acepta los argumentos como objeto o, como los manda OpenAI, como texto con un objeto JSON.
func (l *Llamada) UnmarshalJSON(datos []byte) error {
	var crudo struct {
		ID         string          `json:"id"`
		Nombre     string          `json:"nombre"`
		Argumentos json.RawMessage `json:"argumentos"`
	}
	if err := json.Unmarshal(datos, &crudo); err != nil {
		return err
	}
	argumentos, err := ObjetoJSON(crudo.Argumentos)
	if err != nil {
		return NuevoError(CodigoSolicitudInvalida, idioma.Tf("los argumentos de %s no son un objeto JSON", crudo.Nombre))
	}
	*l = Llamada{ID: crudo.ID, Nombre: crudo.Nombre, Argumentos: json.RawMessage(argumentos)}
	return nil
}

// ObjetoJSON acepta un objeto JSON o un texto que lo contiene (vacío equivale a {}) y lo devuelve compacto.
func ObjetoJSON(crudo json.RawMessage) (string, error) {
	var texto string
	if json.Unmarshal(crudo, &texto) == nil {
		crudo = json.RawMessage(texto)
	}
	if strings.TrimSpace(string(crudo)) == "" {
		crudo = json.RawMessage("{}")
	}
	var objeto map[string]json.RawMessage
	if err := json.Unmarshal(crudo, &objeto); err != nil || objeto == nil {
		return "", errors.New("no es un objeto JSON")
	}
	var compacto bytes.Buffer
	if err := json.Compact(&compacto, crudo); err != nil {
		return "", err
	}
	return compacto.String(), nil
}

// EleccionHerramienta es qué puede hacer el agente con las herramientas: Modo auto (o vacío), ninguna, obligatoria o
// una (la de Nombre). En JSON es "auto", "ninguna", "obligatoria" o {"nombre": "…"}.
type EleccionHerramienta struct {
	Modo   string
	Nombre string
}

// MarshalJSON escribe la forma de la API.
func (e EleccionHerramienta) MarshalJSON() ([]byte, error) {
	if e.Modo == "una" {
		return json.Marshal(map[string]string{"nombre": e.Nombre})
	}
	return json.Marshal(e.Modo)
}

// UnmarshalJSON lee la forma de la API.
func (e *EleccionHerramienta) UnmarshalJSON(datos []byte) error {
	var modo string
	if json.Unmarshal(datos, &modo) == nil {
		if modo != "auto" && modo != "ninguna" && modo != "obligatoria" {
			return NuevoError(CodigoSolicitudInvalida, idioma.Tf("eleccion_herramienta desconocida %q (auto, ninguna, obligatoria o {\"nombre\": …})", modo))
		}
		*e = EleccionHerramienta{Modo: modo}
		return nil
	}
	var una struct {
		Nombre string `json:"nombre"`
	}
	if err := json.Unmarshal(datos, &una); err != nil || una.Nombre == "" {
		return NuevoError(CodigoSolicitudInvalida, idioma.T("eleccion_herramienta debe ser auto, ninguna, obligatoria o {\"nombre\": …}"))
	}
	*e = EleccionHerramienta{Modo: "una", Nombre: una.Nombre}
	return nil
}

// ConfigSalida describe el formato pedido por el consumidor.
type ConfigSalida struct {
	Formato            string `json:"formato,omitempty"`
	ValidacionEstricta bool   `json:"validacion_estricta,omitempty"`
	// Esquema es el JSON Schema que debe cumplir la salida; exige formato json.
	Esquema json.RawMessage `json:"esquema,omitempty"`
	// ConservarSalidaInvalida guarda la salida que no cumple el esquema, para revisarla, con la misma retención.
	ConservarSalidaInvalida bool `json:"conservar_salida_invalida,omitempty"`
	// Reintentos es cuántas veces se pide al mismo agente que corrija una salida que no cumple (0 a 3; 0, ninguna).
	Reintentos int `json:"reintentos,omitempty"`
}

// MaximoReintentosSalida es el tope de correcciones de una salida: cada una es otra petición al agente.
const MaximoReintentosSalida = 3

// ConfigEjecucion agrupa los límites de ejecución pedidos.
type ConfigEjecucion struct {
	TiempoLimiteSegundos int  `json:"tiempo_limite_segundos,omitempty"`
	PersistirSesion      bool `json:"persistir_sesion,omitempty"`
}

// SolicitudEjecucion es el contrato interno único.
type SolicitudEjecucion struct {
	Agente       string `json:"agente,omitempty"`
	Modelo       string `json:"modelo,omitempty"`
	PerfilModelo string `json:"perfil_modelo,omitempty"`
	// Esfuerzo es cuánto razona el agente (bajo, medio, alto, muy_alto, maximo); vacío deja el de su CLI. Los
	// agentes que no lo admiten lo ignoran.
	Esfuerzo string `json:"esfuerzo,omitempty"`
	// Respaldo son los agentes (agente o agente/modelo) que siguen, en orden, si el principal falla antes de
	// producir texto.
	Respaldo []string `json:"respaldo,omitempty"`
	// Perfil es un perfil de ejecución explícito (profiles/<nombre>.yaml); se aplica al aceptar y queda en la fotografía.
	Perfil       string `json:"perfil,omitempty"`
	ModoContexto string `json:"modo_contexto,omitempty"`
	// RaizProyecto la fija el planificador en modo consciente_proyecto; no viene del cliente.
	RaizProyecto  string          `json:"-"`
	Instrucciones string          `json:"instrucciones,omitempty"`
	Mensajes      []Mensaje       `json:"mensajes,omitempty"`
	Cargas        []string        `json:"cargas,omitempty"`
	Salida        ConfigSalida    `json:"salida"`
	Ejecucion     ConfigEjecucion `json:"ejecucion"`
	// Herramientas son las funciones que ofrece la aplicación (internal/herramientas las emula para el agente).
	Herramientas        []Herramienta       `json:"herramientas,omitempty"`
	EleccionHerramienta EleccionHerramienta `json:"eleccion_herramienta,omitzero"`
	// LlamadasParalelas en false limita a una llamada por respuesta; nil equivale a true.
	LlamadasParalelas *bool `json:"llamadas_paralelas,omitempty"`
}

// conHerramientas indica si el agente puede pedir herramientas (las hay y la elección no es «ninguna»).
func (s SolicitudEjecucion) conHerramientas() bool {
	return len(s.Herramientas) > 0 && s.EleccionHerramienta.Modo != "ninguna"
}

const (
	maximoTiempoLimiteSegundos = 24 * 60 * 60
	// MaximoCargas, MaximoMensajes y MaximoEsquemaPeticion son límites de una petición (/api/v1/limites).
	MaximoCargas          = 20
	maximoRespaldo        = 3
	MaximoMensajes        = 1000
	MaximoEsquemaPeticion = 256 << 10
)

// ReCarga es el formato de un identificador de carga: impide rutas u otros valores en su lugar.
var ReCarga = regexp.MustCompile(`^carga_[0-9a-f]{32}$`)

var (
	rolesValidos    = map[string]bool{"sistema": true, "usuario": true, "asistente": true, "herramienta": true}
	formatosValidos = map[string]bool{"texto": true, "markdown": true, "json": true}
	modosContexto   = map[string]bool{"aislado": true, "consciente_proyecto": true}
	// NivelesEsfuerzo, de menor a mayor.
	NivelesEsfuerzo = []string{"bajo", "medio", "alto", "muy_alto", "maximo"}
	// El modelo es una cadena validada por tamaño y formato técnico, no un enum cerrado.
	reIdentificador = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,127}$`)
	// reParte es un agente o un modelo sin barra: cada mitad de una entrada de respaldo.
	reParte = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@-]{0,127}$`)
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
	if len(s.Mensajes) > MaximoMensajes {
		return invalida(idioma.Tf("demasiados mensajes (máximo %d)", MaximoMensajes))
	}
	if len(s.Salida.Esquema) > MaximoEsquemaPeticion {
		return invalida(idioma.T("el esquema supera 256 KB"))
	}
	for i, m := range s.Mensajes {
		if !rolesValidos[m.Rol] {
			return invalida(idioma.Tf("mensaje %d con rol desconocido %q", i, m.Rol))
		}
		if len(m.Llamadas) > 0 && m.Rol != "asistente" {
			return invalida(idioma.Tf("mensaje %d: solo un mensaje del asistente lleva llamadas", i))
		}
		if m.Rol == "herramienta" && m.IDLlamada == "" {
			return invalida(idioma.Tf("mensaje %d: un resultado de herramienta necesita id_llamada", i))
		}
		// Un turno del asistente que solo pidió herramientas no tiene texto.
		if m.Contenido == "" && len(m.Llamadas) == 0 && m.Rol != "herramienta" {
			return invalida(idioma.Tf("mensaje %d sin contenido", i))
		}
	}
	if err := ValidarRespaldo(s.Respaldo); err != nil {
		return err
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
	if s.Salida.Reintentos < 0 || s.Salida.Reintentos > MaximoReintentosSalida {
		return invalida(idioma.Tf("salida.reintentos va de 0 a %d", MaximoReintentosSalida))
	}
	if s.Salida.Reintentos > 0 && s.Salida.Formato != "json" && !s.conHerramientas() {
		return invalida(idioma.T("salida.reintentos exige formato json o un esquema"))
	}
	if len(s.Salida.Esquema) > 0 {
		if s.Salida.Formato != "json" {
			return invalida(idioma.T("un esquema de salida exige formato json"))
		}
		if !json.Valid(s.Salida.Esquema) {
			return invalida(idioma.T("el esquema de salida no es JSON válido"))
		}
	}
	if len(s.Cargas) > MaximoCargas {
		return invalida(idioma.Tf("demasiadas cargas (máximo %d)", MaximoCargas))
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

// ValidarRespaldo comprueba la lista de respaldo: hasta 3 entradas agente o agente/modelo, con un esfuerzo propio
// opcional (+nivel), sin repetir agente y modelo. Un perfil no vale: da un modelo por agente y, sin agente, sería
// ambiguo.
func ValidarRespaldo(lista []string) error {
	if len(lista) > maximoRespaldo {
		return NuevoError(CodigoSolicitudInvalida, idioma.T("respaldo: máximo 3 agentes"))
	}
	var vistas []string
	for _, e := range lista {
		base, nivel, conNivel := strings.Cut(e, "+")
		agente, modelo, conModelo := strings.Cut(base, "/")
		if !reParte.MatchString(agente) || agente == "perfil" || (conModelo && !reParte.MatchString(modelo)) {
			return NuevoError(CodigoSolicitudInvalida, idioma.Tf("respaldo %q con formato no válido (agente o agente/modelo)", e))
		}
		if conNivel && nivel != EsfuerzoDelAgente && !slices.Contains(NivelesEsfuerzo, nivel) {
			return NuevoError(CodigoSolicitudInvalida, idioma.Tf("respaldo %q: esfuerzo desconocido %q", e, nivel))
		}
		if slices.Contains(vistas, base) {
			return NuevoError(CodigoSolicitudInvalida, idioma.Tf("respaldo repetido: %s", base))
		}
		vistas = append(vistas, base)
	}
	return nil
}

// EsfuerzoDelAgente, como sufijo de respaldo (codex+agente), deja el esfuerzo que use su CLI en lugar de heredar el del
// principal. Lo escribe el planificador al saltar si el principal no tenía: la solicitud guardada ya lleva el esfuerzo
// del respaldo que respondió y el resto no debe heredarlo.
const EsfuerzoDelAgente = "agente"

// DividirRespaldo separa una entrada de respaldo en agente, modelo (vacío: el predeterminado del agente) y esfuerzo
// (vacío: el del principal).
func DividirRespaldo(entrada string) (agente, modelo, esfuerzo string) {
	base, esfuerzo, _ := strings.Cut(entrada, "+")
	agente, modelo, _ = strings.Cut(base, "/")
	return agente, modelo, esfuerzo
}

// Huella resume la solicitud normalizada para la idempotencia sin guardar su texto.
func (s SolicitudEjecucion) Huella() string {
	datos, _ := json.Marshal(s) // el orden fijo de los campos hace la serialización determinista
	suma := sha256.Sum256(datos)
	return hex.EncodeToString(suma[:])
}

// Documentos son las cargas ya convertidas a texto: el texto ensamblado (un bloque delimitado por archivo) y el tamaño
// de cada archivo, para medir la entrada y decir cuál pesa más.
type Documentos struct {
	Texto  string
	Partes []ParteDocumento
}

// ParteDocumento es un archivo convertido (los de un ZIP, como «paquete.zip/interno.txt») y sus caracteres.
type ParteDocumento struct {
	Nombre     string
	Caracteres int
}
