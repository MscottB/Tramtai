package configuracion

import (
	"testing"
	"tramtai/internal/nucleo"
)

func TestPresupuestoConfiguracion(t *testing.T) {
	c := Resolver(cargar(t, "presupuesto:\n  diario:\n    peticiones: 500\n  mensual:\n    tokens: 2000\n    usd: 50\n", OrigenGlobal))
	if p := PresupuestoDe(c); p != (nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 500}, Mensual: nucleo.Limites{Tokens: 2000, USD: 50}}) {
		t.Errorf("PresupuestoDe = %+v", p)
	}
	c = Resolver(cargar(t, "presupuesto:\n  mensual:\n    usd: 12.5\n", OrigenGlobal))
	if usd := c.Decimal("presupuesto.mensual.usd"); usd != 12.5 {
		t.Errorf("Decimal = %v", usd)
	}
	casos := []struct {
		nombre, contenido string
		origen            Origen
	}{
		{"administrada", "presupuesto:\n  mensual:\n    tokens: 1\n", OrigenProyecto},
		{"negativo", "presupuesto:\n  diario:\n    peticiones: -5\n", OrigenGlobal},
		{"usd negativo", "presupuesto:\n  diario:\n    usd: -0.5\n", OrigenGlobal},
		{"tipo incorrecto", "presupuesto:\n  diario:\n    usd: mucho\n", OrigenGlobal},
		{"usd no es un número", "presupuesto:\n  diario:\n    usd: .nan\n", OrigenGlobal},
		{"usd infinito", "presupuesto:\n  mensual:\n    usd: .inf\n", OrigenGlobal},
	}
	for _, caso := range casos {
		if _, err := CargarCapa(escribir(t, caso.contenido), caso.origen); nucleo.CodigoDe(err) != nucleo.CodigoConfiguracionInvalida {
			t.Errorf("%s: se esperaba configuracion_invalida, fue %v", caso.nombre, err)
		}
	}
}
