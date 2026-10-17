package planificador

import (
	"context"

	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// FijarPresupuestos fija los presupuestos vigentes: por nombre de API y, con la clave persistencia.TodaLaInstancia,
// el de la instancia. Los vacíos se omiten. Se guardan en un puntero atómico porque el almacén los consulta dentro de
// sus transacciones y no debe esperar a p.mu.
func (p *Planificador) FijarPresupuestos(m map[string]nucleo.Presupuesto) {
	vigentes := map[string]nucleo.Presupuesto{}
	for nombre, pres := range m {
		if !pres.Vacio() {
			vigentes[nombre] = pres
		}
	}
	p.presupuestos.Store(&vigentes)
}

func (p *Planificador) presupuesto(ambito string) (nucleo.Presupuesto, bool) {
	vigentes := p.presupuestos.Load()
	if vigentes == nil {
		return nucleo.Presupuesto{}, false
	}
	pres, ok := (*vigentes)[ambito]
	return pres, ok
}

// topes son los presupuestos que se aplican a un trabajo de la API punto ("" = sin API): el suyo y el de la instancia.
func (p *Planificador) topes(_, punto string) []persistencia.Tope {
	var topes []persistencia.Tope
	if pres, ok := p.presupuesto(punto); ok && punto != "" {
		topes = append(topes, persistencia.Tope{Punto: punto, Presupuesto: pres})
	}
	if pres, ok := p.presupuesto(persistencia.TodaLaInstancia); ok {
		topes = append(topes, persistencia.Tope{Punto: persistencia.TodaLaInstancia, Presupuesto: pres})
	}
	return topes
}

// EstadoPresupuesto es el consumo frente a los límites de una API o, con persistencia.TodaLaInstancia, de la
// instancia; false si no tiene presupuesto.
func (p *Planificador) EstadoPresupuesto(ctx context.Context, punto string) (nucleo.EstadoPresupuesto, bool, error) {
	pres, ok := p.presupuesto(punto)
	if !ok {
		return nucleo.EstadoPresupuesto{}, false, nil
	}
	dia, mes, err := p.almacen.Consumo(ctx, p.opc.Instancia, punto)
	if err != nil {
		return nucleo.EstadoPresupuesto{}, true, err
	}
	_, _, finDia, finMes := nucleo.Periodos(p.almacen.Ahora(), p.almacen.Zona)
	return nucleo.EstadoPresupuesto{
		Diario:  nucleo.EstadoPeriodo{Limites: pres.Diario, Consumo: dia, Reinicio: finDia},
		Mensual: nucleo.EstadoPeriodo{Limites: pres.Mensual, Consumo: mes, Reinicio: finMes},
	}, true, nil
}

// PorcentajePresupuesto es el mayor porcentaje usado entre el presupuesto de la API punto y el de la instancia;
// false si ninguno tiene presupuesto o no se pudo leer el consumo.
func (p *Planificador) PorcentajePresupuesto(ctx context.Context, punto string) (int, bool) {
	mayor, alguno := 0, false
	for _, tope := range p.topes("", punto) {
		dia, mes, err := p.almacen.Consumo(ctx, p.opc.Instancia, tope.Punto)
		if err != nil {
			return 0, false
		}
		if n, ok := tope.Presupuesto.Porcentaje(dia, mes); ok {
			mayor, alguno = max(mayor, n), true
		}
	}
	return mayor, alguno
}

// ReiniciarPresupuesto pone a cero el consumo del día y del mes en curso de una API o de la instancia.
func (p *Planificador) ReiniciarPresupuesto(ctx context.Context, punto string) error {
	return p.almacen.ReiniciarConsumo(ctx, p.opc.Instancia, punto)
}

// EliminarConsumo borra todo el consumo de una API (al eliminarla).
// PuntosConConsumo son las APIs con consumo guardado en la instancia (para borrar el de las que ya no existen).
func (p *Planificador) PuntosConConsumo(ctx context.Context) ([]string, error) {
	return p.almacen.PuntosConConsumo(ctx, p.opc.Instancia)
}

func (p *Planificador) EliminarConsumo(ctx context.Context, punto string) error {
	return p.almacen.EliminarConsumo(ctx, p.opc.Instancia, punto)
}
