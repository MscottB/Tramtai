package nucleo

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Uso es el consumo que informa el agente; ceros si no lo informa.
type Uso struct {
	TokensEntrada int     `json:"tokens_entrada"`
	TokensSalida  int     `json:"tokens_salida"`
	CostoUSD      float64 `json:"costo_usd,omitempty"`
}

// EventoRegistro es un paso del registro de un trabajo (GET /api/v1/trabajos/{id}/registro): estados, intentos y
// avisos, sin el texto de la respuesta.
type EventoRegistro struct {
	Numero   int       `json:"numero"`
	Tipo     string    `json:"tipo"`
	Datos    string    `json:"datos"`
	CreadoEn time.Time `json:"creado_en"`
	// Etapa, en un evento de estado, es la de ese estado: solo viaja en el JSON (la calcula MarshalJSON).
	Etapa string `json:"etapa,omitempty" ts:"'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'"`
}

// MarshalJSON escribe el evento con la etapa de su estado, calculada aquí.
func (e EventoRegistro) MarshalJSON() ([]byte, error) {
	type plano EventoRegistro // sin métodos: no vuelve a llamar a MarshalJSON
	p := plano(e)
	if p.Etapa = ""; e.Tipo == "estado" {
		p.Etapa = Estado(e.Datos).Etapa()
	}
	return json.Marshal(p)
}

// Trabajo es la unidad de ejecución con su estado, tiempos y ámbito.
type Trabajo struct {
	ID           string             `json:"id"`
	Instancia    string             `json:"instancia"`
	Estado       Estado             `json:"estado"`
	CodigoError  CodigoError        `json:"codigo_error,omitempty"`
	MensajeError string             `json:"mensaje_error,omitempty"`
	Solicitud    SolicitudEjecucion `json:"solicitud"`
	Huella       string             `json:"-"`
	Salida       string             `json:"salida,omitempty"`
	IntentoDe    string             `json:"intento_de,omitempty"`
	// PuntoAcceso es el punto de acceso que creó el trabajo (uso por aplicación).
	PuntoAcceso string `json:"punto_acceso,omitempty"`
	// Origen es de dónde llegó la petición (DesdeProbar, DesdeCLI…); vacío en los trabajos anteriores.
	Origen string `json:"origen,omitempty"`
	// ModeloEnviado es el identificador pasado al CLI tras resolver el perfil; vacío = predeterminado del CLI.
	ModeloEnviado string `json:"modelo_enviado,omitempty"`
	// ModeloEfectivo es el que informó el agente; vacío = no verificable.
	ModeloEfectivo string `json:"modelo_efectivo,omitempty"`
	Uso            Uso    `json:"uso"`
	// Disponibilidad de la salida, aparte del estado: disponible o expirada; vacía si no hubo salida.
	Disponibilidad string `json:"disponibilidad,omitempty"`
	// PID es el proceso del agente; se conserva para diagnosticar una cancelación no confirmada.
	PID int `json:"pid,omitempty"`
	// FirmaProceso distingue ese proceso de otro que reutilice el PID.
	FirmaProceso string `json:"-"`
	// EntradasEliminadas indica que la retención ya borró las entradas: el reintento exige reenviarlas.
	EntradasEliminadas bool       `json:"entradas_eliminadas,omitempty"`
	CreadoEn           time.Time  `json:"creado_en"`
	ActualizadoEn      time.Time  `json:"actualizado_en"`
	TerminadoEn        *time.Time `json:"terminado_en,omitempty"`
	// Etapa y Corregible solo viajan en el JSON: MarshalJSON los calcula (Estado.Etapa y SePuedeCorregir) para que
	// la Web UI y la terminal los pinten sin calcularlos. Dentro del servicio se usan los métodos.
	Etapa      string `json:"etapa" ts:"'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'"`
	Corregible bool   `json:"corregible"`
}

// SePuedeCorregir dice si «Corregir» vale: el trabajo terminó porque la respuesta no cumplía el esquema, conservó esa
// respuesta y sus entradas siguen ahí.
func (t Trabajo) SePuedeCorregir() bool {
	return t.Estado.Terminal() && t.CodigoError == CodigoSalidaNoCumpleEsquema && t.Salida != "" && !t.EntradasEliminadas
}

// MarshalJSON escribe el trabajo con su etapa y si se puede corregir, calculados aquí.
func (t Trabajo) MarshalJSON() ([]byte, error) {
	type plano Trabajo // sin métodos: no vuelve a llamar a MarshalJSON
	p := plano(t)
	p.Etapa, p.Corregible = t.Estado.Etapa(), t.SePuedeCorregir()
	return json.Marshal(p)
}

// Disponibilidad de la salida de un trabajo terminado.
const (
	SalidaDisponible = "disponible"
	SalidaExpirada   = "expirada"
)

// NuevoID genera un identificador de trabajo aleatorio.
func NuevoID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b) // crypto/rand.Read no falla en las plataformas soportadas
	return "trabajo_" + hex.EncodeToString(b)
}
