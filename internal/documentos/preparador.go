package documentos

import (
	"archive/zip"
	"context"
	"path/filepath"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Preparador resuelve las cargas de un trabajo y las convierte; lo usa el planificador en preparando_entrada.
type Preparador struct {
	Cargas    *Cargas
	Conversor *Conversor
	// Preparacion es la del servicio (nil en la CLI directa): dice si el conversor se está preparando.
	Preparacion *Preparacion
}

// ConversorListo rechaza, al aceptar un trabajo, las cargas que necesitan el conversor cuando no hay ninguno: así no
// se acepta un trabajo que fallaría al convertir. Mientras se prepara, el error es ConversorPreparando (Retry-After).
func (p Preparador) ConversorListo(cargas []string) error {
	necesita := false
	for _, id := range cargas {
		if carga, ruta, err := p.Cargas.Obtener(id); err == nil && NecesitaConversor(carga.Nombre, ruta) {
			necesita = true
			break
		}
	}
	if !necesita {
		return nil
	}
	// Uno que ya sirve basta, aunque se esté preparando otro (una versión nueva o «Preparar de nuevo»).
	if _, ok := Localizar(p.Conversor.Comando, p.Conversor.Raiz); ok {
		return nil
	}
	if p.Preparacion != nil && p.Preparacion.Estado().Estado == "preparando" {
		return &nucleo.ConversorPreparando{Err: nucleo.NuevoError(nucleo.CodigoConversorNoDisponible,
			idioma.T("el conversor de archivos se está preparando: vuelve a intentarlo en un momento"))}
	}
	return ErrSinConversor()
}

// NecesitaConversor indica si un archivo pasa por markitdown: HTML, PDF y Office, o un ZIP que los traiga (se mira su
// índice, sin extraer; un ZIP dentro de otro cuenta, porque no se ve su interior sin extraerlo).
func NecesitaConversor(nombre, ruta string) bool {
	extension := strings.ToLower(filepath.Ext(nombre))
	if extension != ".zip" {
		return extensionesConvertibles[extension]
	}
	lector, err := zip.OpenReader(ruta)
	if err != nil {
		return false // un ZIP que no se abre lo rechaza la conversión con su propio error
	}
	defer lector.Close()
	for _, f := range lector.File {
		interna := strings.ToLower(filepath.Ext(f.Name))
		if extensionesConvertibles[interna] || interna == ".zip" {
			return true
		}
	}
	return false
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
	if p.Preparacion != nil { // mientras convierte, una preparación nueva no reemplaza la carpeta del conversor
		p.Preparacion.uso.RLock()
		defer p.Preparacion.uso.RUnlock()
	}
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
