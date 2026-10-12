package nucleo

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Trabajo es la unidad de ejecución con su estado, tiempos y ámbito.
type Trabajo struct {
	ID            string             `json:"id"`
	Instancia     string             `json:"instancia"`
	Estado        Estado             `json:"estado"`
	CodigoError   CodigoError        `json:"codigo_error,omitempty"`
	MensajeError  string             `json:"mensaje_error,omitempty"`
	Solicitud     SolicitudEjecucion `json:"solicitud"`
	Huella        string             `json:"-"`
	Salida        string             `json:"salida,omitempty"`
	IntentoDe     string             `json:"intento_de,omitempty"`
	CreadoEn      time.Time          `json:"creado_en"`
	ActualizadoEn time.Time          `json:"actualizado_en"`
	TerminadoEn   *time.Time         `json:"terminado_en,omitempty"`
}

// NuevoID genera un identificador de trabajo aleatorio.
func NuevoID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b) // crypto/rand.Read no falla en las plataformas soportadas
	return "trabajo_" + hex.EncodeToString(b)
}
