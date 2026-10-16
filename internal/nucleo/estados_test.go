package nucleo

import "testing"

func TestTransicionesDeLaTabla(t *testing.T) {
	validas := [][2]Estado{
		{EnCola, EsperandoAgente}, {EsperandoAgente, Ejecutando}, {Ejecutando, ValidandoSalida},
		{ValidandoSalida, Completado}, {Ejecutando, CancelacionSolicitada}, {CancelacionSolicitada, Cancelado},
		{CancelacionSolicitada, Fallido}, {Ejecutando, Interrumpido}, {EsperandoAgente, EnCola}, {EnCola, Fallido}, {Ejecutando, EsperandoAgente},
	}
	for _, par := range validas {
		if err := ValidarTransicion(par[0], par[1]); err != nil {
			t.Errorf("%s → %s debería ser válida: %v", par[0], par[1], err)
		}
	}
	invalidas := [][2]Estado{
		{EnCola, Completado}, {EnCola, Ejecutando}, {Completado, EnCola},
		{Interrumpido, EnCola}, {Cancelado, Cancelado}, {EnCola, Interrumpido},
	}
	for _, par := range invalidas {
		if CodigoDe(ValidarTransicion(par[0], par[1])) != CodigoTransicionInvalida {
			t.Errorf("%s → %s debería ser inválida", par[0], par[1])
		}
	}
}

func TestEstadosTerminalesSinSalida(t *testing.T) {
	for _, e := range []Estado{Completado, Fallido, Cancelado, Interrumpido} {
		if !e.Terminal() || len(transiciones[e]) != 0 || !e.Valido() {
			t.Errorf("%s debe ser terminal, válido y sin transiciones", e)
		}
	}
	if Estado("otro").Valido() {
		t.Error("un estado desconocido no debe ser válido")
	}
}
