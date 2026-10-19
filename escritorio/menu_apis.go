package main

import (
	"cmp"
	"slices"

	"tramtai/internal/servicio"
)

// maximoApisMenu es cuántas APIs caben en el submenú «Mis APIs» de la bandeja; el resto se ve en Mis APIs.
const maximoApisMenu = 15

// menuApis ordena las APIs para la bandeja y devuelve las primeras maximo y cuántas quedan fuera: primero las que
// tienen error (necesitan atención), después las activas y al final las desactivadas; dentro de cada grupo, por nombre
// y por instancia. Con decenas de APIs el menú no pasa de la pantalla.
func menuApis(puntos []servicio.EstadoPunto, maximo int) (mostrar []servicio.EstadoPunto, resto int) {
	grupo := func(p servicio.EstadoPunto) int {
		switch {
		case p.Error != "desactivado" && !p.Activo:
			return 0
		case p.Activo:
			return 1
		}
		return 2
	}
	orden := slices.Clone(puntos)
	slices.SortStableFunc(orden, func(a, b servicio.EstadoPunto) int {
		return cmp.Or(cmp.Compare(grupo(a), grupo(b)), cmp.Compare(a.Nombre, b.Nombre), cmp.Compare(a.Instancia, b.Instancia))
	})
	if len(orden) <= maximo {
		return orden, 0
	}
	return orden[:maximo], len(orden) - maximo
}
