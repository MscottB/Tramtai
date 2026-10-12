// Package nucleo define los contratos internos de Tramtai: estados, trabajos, solicitudes y configuración.
package nucleo

import "errors"

// CodigoError identifica la causa de un error de forma estable para API, CLI e interfaces.
type CodigoError string

const (
	CodigoSolicitudInvalida       CodigoError = "solicitud_invalida"
	CodigoConfiguracionInvalida   CodigoError = "configuracion_invalida"
	CodigoColaLlena               CodigoError = "cola_llena"
	CodigoConflictoIdempotencia   CodigoError = "conflicto_idempotencia"
	CodigoNoEncontrado            CodigoError = "no_encontrado"
	CodigoServicioDetenido        CodigoError = "servicio_detenido"
	CodigoTransicionInvalida      CodigoError = "transicion_invalida"
	CodigoTiempoColaAgotado       CodigoError = "tiempo_cola_agotado"
	CodigoTiempoEjecucionAgotado  CodigoError = "tiempo_ejecucion_agotado"
	CodigoSalidaNoCumpleEsquema   CodigoError = "salida_no_cumple_esquema"
	CodigoCancelacionNoConfirmada CodigoError = "cancelacion_no_confirmada"
	CodigoEntradaNoDisponible     CodigoError = "entrada_no_disponible"
	CodigoAgenteNoDisponible      CodigoError = "agente_no_disponible"
	CodigoErrorAgente             CodigoError = "error_agente"
	CodigoLimiteUsoProveedor      CodigoError = "limite_uso_proveedor"
	CodigoAutenticacionAgente     CodigoError = "autenticacion_agente"
	CodigoModeloNoDisponible      CodigoError = "modelo_no_disponible"
	CodigoModeloNoConfigurado     CodigoError = "modelo_no_configurado"
	CodigoErrorProtocoloAgente    CodigoError = "error_protocolo_agente"
	CodigoCargaInvalida           CodigoError = "carga_invalida"
	CodigoConversionFallida       CodigoError = "conversion_fallida"
	CodigoNoAutorizado            CodigoError = "no_autorizado"
	CodigoOrigenNoPermitido       CodigoError = "origen_no_permitido"
	CodigoModeloAmbiguo           CodigoError = "modelo_ambiguo"
	CodigoParametroNoSoportado    CodigoError = "parametro_no_soportado"
	CodigoTrabajoNoTerminado      CodigoError = "trabajo_no_terminado"
	CodigoTrabajoSinSalida        CodigoError = "trabajo_sin_salida"
	CodigoErrorInterno            CodigoError = "error_interno"
)

// Error es el error propio de Tramtai: un código estable y un mensaje legible sin secretos.
type Error struct {
	Codigo  CodigoError
	Mensaje string
}

func (e *Error) Error() string { return string(e.Codigo) + ": " + e.Mensaje }

// NuevoError construye un *Error.
func NuevoError(codigo CodigoError, mensaje string) *Error {
	return &Error{Codigo: codigo, Mensaje: mensaje}
}

// CodigoDe devuelve el código de un error de Tramtai, o "" si el error es de otro tipo.
func CodigoDe(err error) CodigoError {
	var e *Error
	if errors.As(err, &e) {
		return e.Codigo
	}
	return ""
}

// MensajeDe devuelve el mensaje sin el prefijo del código para los errores de Tramtai.
func MensajeDe(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Mensaje
	}
	return err.Error()
}
