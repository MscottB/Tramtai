package planificador

import (
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

func TestPresupuestoInstanciaSinAPI(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 1, opciones())
	p.FijarPresupuestos(map[string]nucleo.Presupuesto{persistencia.TodaLaInstancia: {Diario: nucleo.Limites{Peticiones: 1}}})
	primero := enviar(t, p, solicitud("uno"))
	if _, _, err := p.Enviar(ctx, solicitud("dos"), ""); nucleo.CodigoDe(err) != nucleo.CodigoPresupuestoAgotado {
		t.Fatalf("sin API: %v", err)
	}
	if _, _, err := p.EnviarPorPunto(ctx, solicitud("tres"), "", "facturas"); nucleo.CodigoDe(err) != nucleo.CodigoPresupuestoAgotado {
		t.Fatalf("por una API: %v", err)
	}
	conRespaldo := solicitud("cuatro")
	conRespaldo.Respaldo = []string{"simulado"}
	if _, _, err := p.Enviar(ctx, conRespaldo, ""); nucleo.CodigoDe(err) != nucleo.CodigoPresupuestoAgotado {
		t.Fatalf("el respaldo no evita el presupuesto: %v", err)
	}
	esperar(t, alm, primero, nucleo.Completado)
	if _, err := p.Reintentar(ctx, primero); nucleo.CodigoDe(err) != nucleo.CodigoPresupuestoAgotado {
		t.Fatalf("el reintento también consume: %v", err)
	}
}

func TestPresupuestoDeLaAPI(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 1, opciones())
	p.FijarPresupuestos(map[string]nucleo.Presupuesto{"facturas": {Diario: nucleo.Limites{Peticiones: 1}}, "vacia": {}})
	if _, _, err := p.EnviarPorPunto(ctx, solicitud("uno"), "", "facturas"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.EnviarPorPunto(ctx, solicitud("dos"), "", "facturas"); nucleo.CodigoDe(err) != nucleo.CodigoPresupuestoAgotado {
		t.Fatalf("API agotada: %v", err)
	}
	enviar(t, p, solicitud("sin API")) // el presupuesto de facturas no frena a los demás
	if _, ok, err := p.EstadoPresupuesto(ctx, "vacia"); ok || err != nil {
		t.Fatalf("un presupuesto vacío se omite: %v %v", ok, err)
	}
	estado, ok, err := p.EstadoPresupuesto(ctx, "facturas")
	if !ok || err != nil || estado.Diario.Consumo.Peticiones != 1 || estado.Diario.Limites.Peticiones != 1 || estado.Diario.Reinicio.IsZero() {
		t.Fatalf("estado %+v %v %v", estado, ok, err)
	}
	if err := p.ReiniciarPresupuesto(ctx, "facturas"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.EnviarPorPunto(ctx, solicitud("tres"), "", "facturas"); err != nil {
		t.Fatalf("tras reiniciar vuelve a admitir: %v", err)
	}
	if err := p.EliminarConsumo(ctx, "facturas"); err != nil {
		t.Fatal(err)
	}
	if estado, _, _ := p.EstadoPresupuesto(ctx, "facturas"); estado.Diario.Consumo.Peticiones != 0 {
		t.Fatalf("tras eliminar no queda consumo: %+v", estado)
	}
}

func TestPorcentajePresupuesto(t *testing.T) {
	alm := abrir(t)
	p := iniciar(t, alm, adaptadores.Simulado{}, 1, opciones())
	if _, ok := p.PorcentajePresupuesto(ctx, "facturas"); ok {
		t.Fatal("sin presupuestos no hay porcentaje")
	}
	p.FijarPresupuestos(map[string]nucleo.Presupuesto{
		"facturas":                   {Diario: nucleo.Limites{Peticiones: 4}},
		persistencia.TodaLaInstancia: {Mensual: nucleo.Limites{Peticiones: 10}},
	})
	for range 2 {
		if _, _, err := p.EnviarPorPunto(ctx, solicitud("api"), "", "facturas"); err != nil {
			t.Fatal(err)
		}
	}
	for range 7 {
		enviar(t, p, solicitud("sin API"))
	}
	if n, ok := p.PorcentajePresupuesto(ctx, "facturas"); n != 90 || !ok {
		t.Fatalf("API al 50 %% e instancia al 90 %%: %d %v", n, ok)
	}
	if n, ok := p.PorcentajePresupuesto(ctx, ""); n != 90 || !ok {
		t.Fatalf("sin API cuenta la instancia: %d %v", n, ok)
	}
}
