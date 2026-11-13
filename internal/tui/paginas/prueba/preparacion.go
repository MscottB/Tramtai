package prueba

import (
	"context"
	"slices"
	"sync"

	"tramtai/internal/contrato"
	"tramtai/internal/preparar"
)

// Pedido es una acción de preparar que llegó al servicio.
type Pedido struct {
	Agente string
	contrato.PedidoPreparacion
}

// Preparacion hace de preparar.Preparador en el servicio de una prueba (servicio.Opciones.Preparacion): no instala nada.
// Iniciar apunta el pedido y deja al agente en el estado de su acción (o en el que dé Regla); la prueba lo mueve con Fijar.
type Preparacion struct {
	// Regla, si no es nil, decide qué pasa con un pedido en lugar del estado inicial de siempre.
	Regla func(Pedido) (contrato.EstadoPreparacion, error)
	// ErrCodigo, si no es nil, es lo que contesta Codigo.
	ErrCodigo error

	mu         sync.Mutex
	estados    map[string]contrato.EstadoPreparacion
	pedidos    []Pedido
	codigos    []Pedido // el Agente y, en PedidoPreparacion.Accion, el código que llegó
	canceladas []string
	versiones  []contrato.VersionesAgente
	forzados   []bool
	consultas  map[string]int
}

// NuevaPreparacion es una preparación sin nada en marcha y sin versiones.
func NuevaPreparacion() *Preparacion {
	return &Preparacion{estados: map[string]contrato.EstadoPreparacion{}, consultas: map[string]int{}}
}

var enCurso = map[string]string{"instalar": "instalando", "actualizar": "actualizando", "desinstalar": "desinstalando", "iniciar_sesion": "comprobando"}

func final(e string) bool { return e == "listo" || e == "error" || e == "cancelado" }

func (f *Preparacion) Estado(agente string) (contrato.EstadoPreparacion, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consultas[agente]++
	e, ok := f.estados[agente]
	return e, ok
}

func (f *Preparacion) Iniciar(agente string, p contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.estados[agente]; ok && !final(e.Estado) {
		return e, &preparar.Error{Codigo: preparar.CodigoYaEnCurso, Mensaje: "ya hay una preparación en marcha"}
	}
	pedido := Pedido{agente, p}
	f.pedidos = append(f.pedidos, pedido)
	e := contrato.EstadoPreparacion{Agente: agente, Accion: p.Accion, Estado: enCurso[p.Accion]}
	if f.Regla != nil {
		var err error
		if e, err = f.Regla(pedido); err != nil {
			return e, err
		}
	}
	f.estados[agente] = e
	return e, nil
}

func (f *Preparacion) Cancelar(agente string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.canceladas = append(f.canceladas, agente)
	if e, ok := f.estados[agente]; ok && !final(e.Estado) {
		e.Estado = "cancelado"
		f.estados[agente] = e
	}
	return nil
}

func (f *Preparacion) Codigo(agente, codigo string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codigos = append(f.codigos, Pedido{Agente: agente, PedidoPreparacion: contrato.PedidoPreparacion{Accion: codigo}})
	return f.ErrCodigo
}

// Versiones apunta si se pidió forzando y da las que fijó la prueba.
func (f *Preparacion) Versiones(_ context.Context, forzar bool) []contrato.VersionesAgente {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.forzados = append(f.forzados, forzar)
	return slices.Clone(f.versiones)
}

func (f *Preparacion) Cerrar() {}

// Fijar deja a un agente en ese estado.
func (f *Preparacion) Fijar(agente string, e contrato.EstadoPreparacion) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e.Agente = agente
	f.estados[agente] = e
}

// FijarVersiones son las versiones que dará GET /api/v1/agentes/versiones.
func (f *Preparacion) FijarVersiones(v ...contrato.VersionesAgente) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.versiones = v
}

// Pedidos son las acciones que llegaron, en orden.
func (f *Preparacion) Pedidos() []Pedido {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.pedidos)
}

// Codigos son los códigos pegados que llegaron, en orden.
func (f *Preparacion) Codigos() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var cs []string
	for _, c := range f.codigos {
		cs = append(cs, c.Accion)
	}
	return cs
}

// Canceladas son los agentes cuya preparación se canceló, en orden.
func (f *Preparacion) Canceladas() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.canceladas)
}

// Consultas son las veces que se pidió el estado de ese agente.
func (f *Preparacion) Consultas(agente string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.consultas[agente]
}

// Forzados dice, de cada llamada a Versiones, si se pidió forzando.
func (f *Preparacion) Forzados() []bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.forzados)
}
