package nucleo

import (
	"math"
	"time"

	"tramtai/internal/idioma"
)

// Limites son los topes de un periodo; 0 = sin límite en esa unidad.
type Limites struct {
	Peticiones int     `json:"peticiones,omitempty" yaml:"peticiones,omitempty"`
	Tokens     int     `json:"tokens,omitempty" yaml:"tokens,omitempty"`
	USD        float64 `json:"usd,omitempty" yaml:"usd,omitempty"`
}

// Presupuesto son los límites diarios y mensuales de una API o de una instancia.
type Presupuesto struct {
	Diario  Limites `json:"diario,omitzero" yaml:"diario,omitempty"`
	Mensual Limites `json:"mensual,omitzero" yaml:"mensual,omitempty"`
}

// Vacio indica que no hay ningún límite.
func (p Presupuesto) Vacio() bool { return p == Presupuesto{} }

// Validar rechaza los valores negativos.
func (p Presupuesto) Validar() error {
	for _, l := range []Limites{p.Diario, p.Mensual} {
		if l.Peticiones < 0 || l.Tokens < 0 || l.USD < 0 {
			return NuevoError(CodigoConfiguracionInvalida, idioma.T("los límites del presupuesto no pueden ser negativos"))
		}
	}
	return nil
}

// Consumo es lo gastado en un periodo: peticiones aceptadas, trabajos terminados y lo que informaron.
type Consumo struct {
	Peticiones int     `json:"peticiones"`
	Terminados int     `json:"terminados"`
	Tokens     int     `json:"tokens"`
	USD        float64 `json:"usd"`
}

// Fraccion es la mayor fracción usada entre los límites definidos y su unidad ("peticiones", "tokens" o "usd");
// 0 y "" sin límites. Con reservar, tokens y usd suman lo que gastarían los trabajos en curso al promedio de los
// terminados: así varios trabajos lanzados a la vez cerca del límite no lo pasan de largo.
func (l Limites) Fraccion(c Consumo, reservar bool) (float64, string) {
	reserva := 0.0 // trabajos en curso por cada terminado: usado × reserva = en curso × promedio
	if reservar && c.Terminados > 0 {
		reserva = float64(max(0, c.Peticiones-c.Terminados)) / float64(c.Terminados)
	}
	mayor, unidad := 0.0, ""
	medir := func(nombre string, usado, limite, reserva float64) {
		if limite <= 0 {
			return
		}
		if f := usado * (1 + reserva) / limite; f > mayor || unidad == "" {
			mayor, unidad = f, nombre
		}
	}
	medir("peticiones", float64(c.Peticiones), float64(l.Peticiones), 0)
	medir("tokens", float64(c.Tokens), float64(l.Tokens), reserva)
	medir("usd", c.USD, l.USD, reserva)
	return mayor, unidad
}

// Periodos da las claves del día ("2006-01-02") y del mes ("2006-01") de ahora en zona, y cuándo reinician.
func Periodos(ahora time.Time, zona *time.Location) (dia, mes string, finDia, finMes time.Time) {
	l := ahora.In(zona)
	a, m, d := l.Date()
	return l.Format("2006-01-02"), l.Format("2006-01"), time.Date(a, m, d+1, 0, 0, 0, 0, zona), time.Date(a, m+1, 1, 0, 0, 0, 0, zona)
}

// PresupuestoAgotado es el rechazo por presupuesto: lleva cuándo reinicia el periodo agotado (Retry-After).
type PresupuestoAgotado struct {
	Err      *Error
	Reinicio time.Time
}

func (e *PresupuestoAgotado) Error() string { return e.Err.Error() }
func (e *PresupuestoAgotado) Unwrap() error { return e.Err }

// Comprobar devuelve *PresupuestoAgotado si un límite del día o del mes está agotado contando la reserva de los
// trabajos en curso. ambito completa el mensaje ("de la API facturas", "de la instancia").
func (p Presupuesto) Comprobar(dia, mes Consumo, ahora time.Time, zona *time.Location, ambito string) error {
	_, _, finDia, finMes := Periodos(ahora, zona)
	periodos := []struct {
		nombre   string
		limites  Limites
		consumo  Consumo
		reinicio time.Time
	}{{idioma.T("diario"), p.Diario, dia, finDia}, {idioma.T("mensual"), p.Mensual, mes, finMes}}
	for _, per := range periodos {
		if f, unidad := per.limites.Fraccion(per.consumo, true); unidad != "" && f >= 1 {
			nombres := map[string]string{"peticiones": idioma.T("peticiones"), "tokens": "tokens", "usd": "USD"}
			return &PresupuestoAgotado{Reinicio: per.reinicio, Err: NuevoError(CodigoPresupuestoAgotado,
				idioma.Tf("presupuesto %s de %s %s agotado", per.nombre, nombres[unidad], ambito))}
		}
	}
	return nil
}

// Porcentaje es el porcentaje usado (sin reserva) del límite más cercano a agotarse; false si no hay límites.
func (p Presupuesto) Porcentaje(dia, mes Consumo) (int, bool) {
	if p.Vacio() {
		return 0, false
	}
	fd, _ := p.Diario.Fraccion(dia, false)
	fm, _ := p.Mensual.Fraccion(mes, false)
	return int(math.Floor(100*max(fd, fm) + 1e-9)), true
}

// EstadoPeriodo es el consumo de un periodo frente a sus límites, para mostrarlo.
type EstadoPeriodo struct {
	Limites  Limites   `json:"limites"`
	Consumo  Consumo   `json:"consumo"`
	Reinicio time.Time `json:"reinicio"`
}

// EstadoPresupuesto es el consumo del día y del mes de una API o de la instancia.
type EstadoPresupuesto struct {
	Diario  EstadoPeriodo `json:"diario"`
	Mensual EstadoPeriodo `json:"mensual"`
}

// PresupuestoDe lee el presupuesto de la instancia de la configuración efectiva.
func PresupuestoDe(c *Configuracion) Presupuesto {
	limites := func(periodo string) Limites {
		return Limites{Peticiones: c.Entero("presupuesto." + periodo + ".peticiones"), Tokens: c.Entero("presupuesto." + periodo + ".tokens"),
			USD: c.Decimal("presupuesto." + periodo + ".usd")}
	}
	return Presupuesto{Diario: limites("diario"), Mensual: limites("mensual")}
}
