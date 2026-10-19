package servicio

import "tramtai/internal/contrato"

// Los tipos que viajan entre el servicio y sus clientes viven en contrato; aquí conservan su nombre de siempre.
type (
	Estado               = contrato.Estado
	EstadoInstancia      = contrato.EstadoInstancia
	EstadoPunto          = contrato.EstadoPunto
	ActivacionInstancia  = contrato.ActivacionInstancia
	EntradaConfiguracion = contrato.EntradaConfiguracion
	ConfigPunto          = contrato.ConfigPunto
	EjecucionPunto       = contrato.EjecucionPunto
	Descriptor           = contrato.Descriptor
)

var (
	RutaDescriptor    = contrato.RutaDescriptor
	LeerDescriptor    = contrato.LeerDescriptor
	NombrePuntoValido = contrato.NombrePuntoValido
)
