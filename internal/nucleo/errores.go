// Package nucleo define los contratos internos de Tramtai: estados, trabajos, solicitudes y configuración.
package nucleo

import "errors"

// CodigoError identifica la causa de un error de forma estable para API, CLI e interfaces.
type CodigoError string

const (
	CodigoSolicitudInvalida       CodigoError = "solicitud_invalida"
	CodigoConfiguracionInvalida   CodigoError = "configuracion_invalida"
	CodigoColaLlena               CodigoError = "cola_llena"
	CodigoLimiteFlujos            CodigoError = "limite_flujos"
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
	CodigoModeloNoPermitido       CodigoError = "modelo_no_permitido"
	CodigoResultadoExpirado       CodigoError = "resultado_expirado"
	CodigoContextoNoPermitido     CodigoError = "contexto_no_permitido"
	CodigoRutaNoPermitida         CodigoError = "ruta_no_permitida"
	CodigoPresupuestoAgotado      CodigoError = "presupuesto_agotado"
	// CodigoEntradaDemasiadoGrande: tras convertir los documentos, la entrada no cabe en el contexto del modelo; no
	// llegó al CLI (ni gasta cuota) y se prueba con el respaldo.
	CodigoEntradaDemasiadoGrande CodigoError = "entrada_demasiado_grande"
	// CodigoCuotaCargasAgotada: las cargas guardadas de la instancia llegan a conversion.cuota_cargas_mb.
	CodigoCuotaCargasAgotada CodigoError = "cuota_cargas_agotada"
	// CodigoCorreccionNoDisponible: se pidió corregir un trabajo que no falló por el esquema o no conservó su salida.
	CodigoCorreccionNoDisponible CodigoError = "correccion_no_disponible"
	// CodigoConversorNoDisponible: un archivo necesita el conversor (markitdown) y no hay ninguno listo.
	CodigoConversorNoDisponible CodigoError = "conversor_no_disponible"
	// CodigoConfianzaRequerida: la carpeta ya traía .tramtai/ y hay que decidir si se confía en ella antes de abrirla.
	CodigoConfianzaRequerida CodigoError = "confianza_requerida"
	// CodigoDentroDeProyecto: la carpeta elegida está dentro de otro proyecto; hay que elegir cuál abrir.
	CodigoDentroDeProyecto CodigoError = "dentro_de_proyecto"
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

// ConversorPreparando es conversor_no_disponible mientras el conversor se prepara: la API añade Retry-After.
type ConversorPreparando struct {
	Err error
}

func (e *ConversorPreparando) Error() string { return e.Err.Error() }
func (e *ConversorPreparando) Unwrap() error { return e.Err }
