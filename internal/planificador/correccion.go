package planificador

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/salidas"
)

// Corregir es un reintento que pide al mismo agente que corrija la respuesta anterior: la conversación sigue con esa
// respuesta y los problemas que tenía. Solo vale para un trabajo que falló por el esquema y conservó su salida.
func (p *Planificador) Corregir(ctx context.Context, id string) (nucleo.Trabajo, error) {
	anterior, err := p.Obtener(ctx, id)
	if err != nil {
		return anterior, err
	}
	switch {
	case anterior.SePuedeCorregir():
	case !anterior.Estado.Terminal():
		return anterior, nucleo.NuevoError(nucleo.CodigoTrabajoNoTerminado, idioma.T("solo se reintenta una petición terminada"))
	case anterior.CodigoError != nucleo.CodigoSalidaNoCumpleEsquema || anterior.Salida == "":
		return anterior, nucleo.NuevoError(nucleo.CodigoCorreccionNoDisponible,
			idioma.T("solo se corrige una petición que falló porque la respuesta no cumplía el formato y que conservó esa respuesta (conservar_salida_invalida)"))
	default: // la regla de SePuedeCorregir que queda: las entradas ya no están
		return anterior, nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("los archivos de la petición ya vencieron o se borraron al terminar: vuelve a enviarlos"))
	}
	problemas, err := p.ProblemasSalida(ctx, id)
	if err != nil {
		return anterior, err
	}
	solicitud := conCorreccion(anterior.Solicitud, anterior.Salida, problemas, anterior.MensajeError)
	solicitud.Salida.Reintentos = 0 // la corrección manual es una sola
	t, _, err := p.aceptar(nucleo.ConOrigenPeticion(ctx, anterior.Origen), solicitud, "", anterior, anterior.PuntoAcceso)
	return t, err
}

// ProblemasSalida son los problemas de la última validación fallida de un trabajo (nil si no la hubo).
func (p *Planificador) ProblemasSalida(ctx context.Context, id string) ([]salidas.Problema, error) {
	eventos, err := p.almacen.EventosDeRegistro(ctx, id) // sin el texto de la respuesta
	if err != nil {
		return nil, err
	}
	for i := len(eventos) - 1; i >= 0; i-- {
		if eventos[i].Tipo == "validacion" {
			var v validacion
			if err := json.Unmarshal([]byte(eventos[i].Datos), &v); err != nil {
				return nil, err
			}
			return v.Problemas, nil
		}
	}
	return nil, nil
}

// validacion es lo que guarda el evento validacion cuando la salida no cumple: correccion es cuántas se habían pedido.
type validacion struct {
	Problemas  []salidas.Problema `json:"problemas"`
	Correccion int                `json:"correccion"`
}

// inicioCorreccion empieza el mensaje de una corrección. La Web UI (web/src/recetas.ts) y la terminal (paginas/ejecutar)
// lo reconocen para mostrar el mensaje original; la prueba de recetas.test.ts compara la copia de la Web con esta.
const inicioCorreccion = "Tu respuesta no cumple el formato pedido:"

// errCorreccionDetenida: el servicio se detuvo mientras una corrección esperaba al agente. No se reencola (repetiría
// la petición entera y perdería el uso de los intentos): se cierra con los problemas que tenía.
var errCorreccionDetenida = errors.New("corrección detenida")

// conCorreccion sigue la conversación con la respuesta que no cumplía y un mensaje con sus problemas. El mensaje es
// para el modelo: no se traduce. mensaje sustituye a los problemas si no se conocen.
func conCorreccion(s nucleo.SolicitudEjecucion, salida string, problemas []salidas.Problema, mensaje string) nucleo.SolicitudEjecucion {
	var b strings.Builder
	b.WriteString(inicioCorreccion + "\n")
	for _, pr := range problemas {
		b.WriteString("- " + pr.Texto + "\n")
	}
	if len(problemas) == 0 && mensaje != "" {
		b.WriteString("- " + mensaje + "\n")
	}
	b.WriteString("Devuelve solo el JSON corregido, sin texto adicional.")
	s.Mensajes = append(slices.Clone(s.Mensajes), nucleo.Mensaje{Rol: "asistente", Contenido: salida}, nucleo.Mensaje{Rol: "usuario", Contenido: b.String()})
	return s
}
