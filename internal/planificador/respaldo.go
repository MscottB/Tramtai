package planificador

import (
	"context"
	"slices"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// resolverModelo fija al aceptar el identificador que recibirá el CLI (fotografía del trabajo).
// Un perfil sin modelo para el agente se rechaza: nunca se sustituye el modelo en silencio.
func (p *Planificador) resolverModelo(s nucleo.SolicitudEjecucion) (string, error) {
	if s.PerfilModelo == "" {
		return s.Modelo, nil
	}
	if modelo := p.opc.Perfiles[s.PerfilModelo][s.Agente]; modelo != "" {
		return modelo, nil
	}
	return "", nucleo.NuevoError(nucleo.CodigoModeloNoConfigurado,
		idioma.Tf("el perfil %q no define modelo para el agente %q (modelos.perfiles.%s.%s)", s.PerfilModelo, s.Agente, s.PerfilModelo, s.Agente))
}

// versionVista es el último modelo efectivo que informó el agente para ese modelo enviado ("" si no hay).
func (p *Planificador) versionVista(agente, modelo string) string {
	efectivos, _ := p.almacen.ModelosEfectivos(context.Background())
	return efectivos[agente+"/"+modelo]
}

// candidato es un agente que puede ejecutar el trabajo: el principal o uno del respaldo. esfuerzo vacío usa el de la
// solicitud.
type candidato struct{ agente, modelo, esfuerzo string }

func (c candidato) String() string {
	texto := c.agente
	if c.modelo != "" {
		texto += "/" + c.modelo
	}
	if c.esfuerzo != "" {
		texto += "+" + c.esfuerzo
	}
	return texto
}

// candidatos son el agente principal (con el modelo ya resuelto) y después el respaldo. Una entrada que repite un
// candidato anterior se omite; una sin modelo repite cualquier intento anterior de su agente.
func candidatos(t nucleo.Trabajo) []candidato {
	lista := []candidato{{t.Solicitud.Agente, t.ModeloEnviado, ""}}
	for _, e := range t.Solicitud.Respaldo {
		agente, modelo, esfuerzo := nucleo.DividirRespaldo(e)
		if !slices.ContainsFunc(lista, func(c candidato) bool { return c.agente == agente && (modelo == "" || c.modelo == modelo) }) {
			lista = append(lista, candidato{agente, modelo, esfuerzo})
		}
	}
	return lista
}

// desconectado indica si el agente está desconectado (Opciones.Desconectado).
func (p *Planificador) desconectado(agente string) bool {
	return p.opc.Desconectado != nil && p.opc.Desconectado(agente)
}

// conectados quita del respaldo los agentes desconectados (el principal se comprobó al aceptar) y devuelve cuáles quitó.
func (p *Planificador) conectados(lista []candidato) ([]candidato, []string) {
	salida := lista[:1:1]
	var saltados []string
	for _, c := range lista[1:] {
		if p.desconectado(c.agente) {
			saltados = append(saltados, c.agente)
			continue
		}
		salida = append(salida, c)
	}
	return salida, saltados
}

// saltable indica los errores tras los que otro agente puede tener éxito: límite, credenciales, disponibilidad,
// fallo del agente o tiempo agotado. La cancelación y un esquema no cumplido nunca saltan.
func saltable(codigo nucleo.CodigoError) bool {
	switch codigo {
	case nucleo.CodigoLimiteUsoProveedor, nucleo.CodigoAutenticacionAgente, nucleo.CodigoEntradaDemasiadoGrande, nucleo.CodigoAgenteNoDisponible, nucleo.CodigoModeloNoDisponible,
		nucleo.CodigoErrorAgente, nucleo.CodigoErrorProtocoloAgente, nucleo.CodigoTiempoEjecucionAgotado:
		return true
	}
	return false
}
