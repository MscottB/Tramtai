package nucleo

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestFraccion(t *testing.T) {
	casos := []struct {
		nombre   string
		limites  Limites
		consumo  Consumo
		reservar bool
		fraccion float64
		unidad   string
	}{
		{"peticiones", Limites{Peticiones: 10}, Consumo{Peticiones: 8}, false, 0.8, "peticiones"},
		{"tokens sin reservar", Limites{Tokens: 1000}, Consumo{Peticiones: 3, Terminados: 1, Tokens: 300}, false, 0.3, "tokens"},
		{"tokens con reserva", Limites{Tokens: 1000}, Consumo{Peticiones: 3, Terminados: 1, Tokens: 300}, true, 0.9, "tokens"},
		{"sin historial no reserva", Limites{Tokens: 1000}, Consumo{Peticiones: 2}, true, 0, "tokens"},
		{"en curso acotado a 0", Limites{Tokens: 1000}, Consumo{Peticiones: 1, Terminados: 2, Tokens: 500}, true, 0.5, "tokens"},
		{"usd con reserva", Limites{USD: 10}, Consumo{Peticiones: 2, Terminados: 1, USD: 4}, true, 0.8, "usd"},
		{"la mayor", Limites{Peticiones: 10, Tokens: 100}, Consumo{Peticiones: 2, Terminados: 2, Tokens: 90}, false, 0.9, "tokens"},
		{"sin límites", Limites{}, Consumo{Peticiones: 5}, true, 0, ""},
	}
	for _, c := range casos {
		f, u := c.limites.Fraccion(c.consumo, c.reservar)
		if diferencia := f - c.fraccion; diferencia > 1e-9 || diferencia < -1e-9 || u != c.unidad {
			t.Errorf("%s: Fraccion = %v %q; se esperaba %v %q", c.nombre, f, u, c.fraccion, c.unidad)
		}
	}
}

func TestPeriodos(t *testing.T) {
	zona := time.FixedZone("x", -3*3600)
	dia, mes, finDia, finMes := Periodos(time.Date(2026, 10, 31, 2, 30, 0, 0, time.UTC), zona)
	if dia != "2026-10-30" || mes != "2026-10" {
		t.Errorf("claves %q %q", dia, mes)
	}
	if !finDia.Equal(time.Date(2026, 10, 31, 0, 0, 0, 0, zona)) || !finMes.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, zona)) {
		t.Errorf("reinicios %v %v", finDia, finMes)
	}
}

func TestComprobar(t *testing.T) {
	zona := time.UTC
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, zona)
	_, _, finDia, finMes := Periodos(ahora, zona)
	p := Presupuesto{Diario: Limites{Peticiones: 2}, Mensual: Limites{Tokens: 1000}}

	err := p.Comprobar(Consumo{Peticiones: 2, Terminados: 2}, Consumo{Peticiones: 2, Terminados: 2}, ahora, zona, "de la API facturas")
	var agotado *PresupuestoAgotado
	if CodigoDe(err) != CodigoPresupuestoAgotado || !errors.As(err, &agotado) || !agotado.Reinicio.Equal(finDia) {
		t.Fatalf("agotado por peticiones diarias: %v", err)
	}
	if m := MensajeDe(err); !strings.Contains(m, "diario") || !strings.Contains(m, "peticiones") || !strings.Contains(m, "de la API facturas") {
		t.Errorf("mensaje %q", m)
	}

	err = p.Comprobar(Consumo{Peticiones: 1, Terminados: 1, Tokens: 10}, Consumo{Peticiones: 9, Terminados: 9, Tokens: 1000}, ahora, zona, "de la instancia")
	if !errors.As(err, &agotado) || !agotado.Reinicio.Equal(finMes) || !strings.Contains(MensajeDe(err), "mensual") {
		t.Errorf("agotado en el mes: %v", err)
	}

	if err := p.Comprobar(Consumo{Peticiones: 1, Terminados: 1}, Consumo{Peticiones: 1, Terminados: 1, Tokens: 100}, ahora, zona, "de la instancia"); err != nil {
		t.Errorf("sin agotar: %v", err)
	}
}

func TestPorcentaje(t *testing.T) {
	p := Presupuesto{Diario: Limites{Peticiones: 10}, Mensual: Limites{Tokens: 1000}}
	if n, ok := p.Porcentaje(Consumo{Peticiones: 1}, Consumo{Peticiones: 5, Terminados: 4, Tokens: 850}); n != 85 || !ok {
		t.Errorf("Porcentaje = %d %v", n, ok)
	}
	if _, ok := (Presupuesto{}).Porcentaje(Consumo{}, Consumo{}); ok {
		t.Error("un presupuesto vacío no tiene porcentaje")
	}
}

func TestValidarPresupuesto(t *testing.T) {
	for _, usd := range []float64{-1, math.NaN(), math.Inf(1)} {
		if err := (Presupuesto{Mensual: Limites{USD: usd}}).Validar(); CodigoDe(err) != CodigoConfiguracionInvalida {
			t.Errorf("USD %v: %v", usd, err)
		}
	}
	if err := (Presupuesto{Diario: Limites{Peticiones: 5, Tokens: 10, USD: 1.5}}).Validar(); err != nil {
		t.Errorf("válido: %v", err)
	}
	if !(Presupuesto{}).Vacio() || (Presupuesto{Diario: Limites{Tokens: 1}}).Vacio() {
		t.Error("Vacio")
	}
}

func TestPresupuestoConfiguracion(t *testing.T) {
	c := Resolver(cargar(t, "presupuesto:\n  diario:\n    peticiones: 500\n  mensual:\n    tokens: 2000\n    usd: 50\n", OrigenGlobal))
	if p := PresupuestoDe(c); p != (Presupuesto{Diario: Limites{Peticiones: 500}, Mensual: Limites{Tokens: 2000, USD: 50}}) {
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
		if _, err := CargarCapa(escribir(t, caso.contenido), caso.origen); CodigoDe(err) != CodigoConfiguracionInvalida {
			t.Errorf("%s: se esperaba configuracion_invalida, fue %v", caso.nombre, err)
		}
	}
}
