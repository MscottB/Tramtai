package nucleo

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Uso es el consumo que informa el agente; ceros si no lo informa.
type Uso struct {
	TokensEntrada int     `json:"tokens_entrada"`
	TokensSalida  int     `json:"tokens_salida"`
	CostoUSD      float64 `json:"costo_usd,omitempty"`
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
	// ModeloEnviado es el identificador pasado al CLI tras resolver el perfil; vacío = predeterminado del CLI.
	ModeloEnviado string `json:"modelo_enviado,omitempty"`
	// ModeloEfectivo es el que informó el agente; vacío = no verificable (sección 9.2).
	ModeloEfectivo string     `json:"modelo_efectivo,omitempty"`
	Uso            Uso        `json:"uso"`
	CreadoEn       time.Time  `json:"creado_en"`
	ActualizadoEn  time.Time  `json:"actualizado_en"`
	TerminadoEn    *time.Time `json:"terminado_en,omitempty"`
}

// NuevoID genera un identificador de trabajo aleatorio.
func NuevoID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b) // crypto/rand.Read no falla en las plataformas soportadas
	return "trabajo_" + hex.EncodeToString(b)
}
