package cliente

import (
	"path/filepath"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// ElegirEspacio decide el espacio con el estado del servicio ya leído: el preferido, la global si se pide o si no hay
// proyecto, o el del proyecto actual si está activo. Dentro de un proyecto detenido es un error: no se usa la global
// sin decirlo. Lo comparten la CLI y la terminal.
func ElegirEspacio(raiz string, activos []contrato.EstadoInstancia, preferida string, global bool) (string, error) {
	switch {
	case preferida != "":
		return preferida, nil
	case global || raiz == "":
		return "global", nil
	}
	for _, i := range activos {
		if i.Ambito == "proyecto" && nucleo.MismaRuta(i.Raiz, raiz) {
			return i.ID, nil
		}
	}
	return "", nucleo.NuevoError(nucleo.CodigoEspacioDetenido, idioma.Tf("el espacio %s está detenido: actívalo con tramtai espacio iniciar, o usa --global", filepath.Base(raiz)))
}
