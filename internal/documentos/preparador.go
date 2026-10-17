package documentos

import (
	"context"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Preparador resuelve las cargas de un trabajo y las convierte; lo usa el planificador en preparando_entrada.
type Preparador struct {
	Cargas    *Cargas
	Conversor *Conversor
}

// Verificar comprueba que las cargas de una solicitud nueva existan, no hayan vencido y sean de la misma API (punto,
// vacío para la instancia): una API no cita las cargas de otra aunque conozca su identificador.
func (p Preparador) Verificar(cargas []string, punto string) error {
	for _, id := range cargas {
		if err := p.Cargas.Vigente(id); err != nil {
			return err
		}
		if carga, _, _ := p.Cargas.Obtener(id); carga.Punto != punto {
			return nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.Tf("la carga %s no existe o venció; vuelve a enviarla", id))
		}
	}
	return nil
}

// Preparar devuelve el texto ensamblado de las cargas, o entrada_no_disponible si alguna venció.
func (p Preparador) Preparar(ctx context.Context, ids []string) (nucleo.Documentos, error) {
	entradas := make([]Entrada, 0, len(ids))
	for _, id := range ids {
		carga, ruta, err := p.Cargas.Obtener(id)
		if err != nil {
			return nucleo.Documentos{}, err
		}
		entradas = append(entradas, Entrada{Ruta: ruta, Nombre: carga.Nombre, SHA256: carga.SHA256})
	}
	return p.Conversor.Convertir(ctx, entradas)
}
