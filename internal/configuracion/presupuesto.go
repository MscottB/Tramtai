package configuracion

import "tramtai/internal/nucleo"

// PresupuestoDe lee el presupuesto de la instancia de la configuración efectiva.
func PresupuestoDe(c *Configuracion) nucleo.Presupuesto {
	limites := func(periodo string) nucleo.Limites {
		return nucleo.Limites{Peticiones: c.Entero("presupuesto." + periodo + ".peticiones"), Tokens: c.Entero("presupuesto." + periodo + ".tokens"),
			USD: c.Decimal("presupuesto." + periodo + ".usd")}
	}
	return nucleo.Presupuesto{Diario: limites("diario"), Mensual: limites("mensual")}
}
