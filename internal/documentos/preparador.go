package documentos

import "context"

// Preparador resuelve las cargas de un trabajo y las convierte; lo usa el planificador en preparando_entrada.
type Preparador struct {
	Cargas    *Cargas
	Conversor *Conversor
}

// Verificar comprueba que las cargas de una solicitud nueva existan y no hayan vencido.
func (p Preparador) Verificar(cargas []string) error {
	for _, id := range cargas {
		if err := p.Cargas.Vigente(id); err != nil {
			return err
		}
	}
	return nil
}

// Preparar devuelve el texto ensamblado de las cargas, o entrada_no_disponible si alguna venció.
func (p Preparador) Preparar(ctx context.Context, ids []string) (string, error) {
	entradas := make([]Entrada, 0, len(ids))
	for _, id := range ids {
		carga, ruta, err := p.Cargas.Obtener(id)
		if err != nil {
			return "", err
		}
		entradas = append(entradas, Entrada{Ruta: ruta, Nombre: carga.Nombre, SHA256: carga.SHA256})
	}
	return p.Conversor.Convertir(ctx, entradas)
}
