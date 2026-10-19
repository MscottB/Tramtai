package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// archivoEnLinea es un archivo que llega dentro de un mensaje de las fachadas (base64), antes de guardarlo como carga.
type archivoEnLinea struct {
	Nombre string
	Datos  []byte
}

// parteArchivo reúne los campos de un archivo en las tres formas: file (chat de OpenAI), input_file (responses) y
// document (Anthropic).
type parteArchivo struct {
	File *struct {
		Filename string `json:"filename"`
		FileData string `json:"file_data"`
		FileID   string `json:"file_id"`
	} `json:"file"`
	Filename string `json:"filename"`
	FileData string `json:"file_data"`
	FileID   string `json:"file_id"`
	FileURL  string `json:"file_url"`
	Title    string `json:"title"`
	Source   *struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
	} `json:"source"`
}

// extensionesMIME dan la extensión de un archivo sin nombre (o con un nombre sin extensión admitida).
var extensionesMIME = map[string]string{
	"application/pdf": ".pdf",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"text/html": ".html", "text/plain": ".txt", "text/markdown": ".md", "text/csv": ".csv",
	"application/json": ".json", "application/xml": ".xml", "text/xml": ".xml", "application/zip": ".zip",
}

// extensionDeMIME es la extensión de un tipo MIME admitido, o vacía.
func extensionDeMIME(mime string) string {
	tipo, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(mime)), ";")
	return extensionesMIME[tipo]
}

// archivoDeParte lee el archivo de una parte file, input_file o document. mensaje y parte (desde 1) sitúan el error;
// n numera los archivos de la petición (documento-<n> si no trae nombre).
func archivoDeParte(tipo string, p parteArchivo, mensaje, parte int, n *int) (archivoEnLinea, error) {
	donde := idioma.Tf("mensaje %d, parte %d", mensaje, parte)
	*n++
	var nombre, mime, datos string
	esBase64 := true
	switch tipo {
	case "file", "input_file":
		datos, nombre = p.FileData, p.Filename
		id := p.FileID
		if p.File != nil {
			datos, nombre, id = p.File.FileData, p.File.Filename, p.File.FileID
		}
		if datos == "" && p.FileURL != "" {
			return archivoEnLinea{}, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("%s: Tramtai no descarga archivos por URL; envía el archivo en file_data", donde))
		}
		if datos == "" && id != "" {
			return archivoEnLinea{}, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("%s: Tramtai no tiene /files; envía el archivo en file_data", donde))
		}
		cabecera, contenido, ok := strings.Cut(datos, ",")
		if !ok || !strings.HasPrefix(cabecera, "data:") || !strings.HasSuffix(cabecera, ";base64") {
			return archivoEnLinea{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s: file_data debe ser data:<tipo>;base64,<contenido>", donde))
		}
		mime, datos = strings.TrimSuffix(strings.TrimPrefix(cabecera, "data:"), ";base64"), contenido
	case "document":
		if p.Source == nil || (p.Source.Type != "base64" && p.Source.Type != "text") {
			return archivoEnLinea{}, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("%s: Tramtai no descarga documentos por URL ni usa la API de archivos; envía el documento en base64", donde))
		}
		nombre, mime, datos, esBase64 = p.Title, p.Source.MediaType, p.Source.Data, p.Source.Type == "base64"
		if !esBase64 && mime == "" {
			mime = "text/plain"
		}
	}
	contenido := []byte(datos)
	if esBase64 {
		var err error
		if contenido, err = base64.StdEncoding.DecodeString(datos); err != nil {
			return archivoEnLinea{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s: el base64 del archivo no es válido", donde))
		}
	}
	// Sin ruta, como hará el almacén de cargas: la marca del turno y la carga dicen el mismo nombre.
	if nombre = filepath.Base(strings.ReplaceAll(nombre, "\\", "/")); nombre == "." || nombre == "/" {
		nombre = ""
	}
	if nombre == "" {
		nombre = fmt.Sprintf("documento-%d", *n)
	}
	if !documentos.ExtensionAdmitida(filepath.Ext(nombre)) {
		extension := extensionDeMIME(mime)
		if extension == "" {
			return archivoEnLinea{}, nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.Tf("%s: tipo de archivo no admitido: %s", donde, mime))
		}
		nombre += extension
	}
	return archivoEnLinea{Nombre: nombre, Datos: contenido}, nil
}

// guardarEnLinea guarda los archivos de los mensajes como cargas de esta API y los suma a la solicitud, sin repetir
// el mismo contenido (el mismo PDF en dos turnos es una sola carga).
func (s *Servidor) guardarEnLinea(archivos []archivoEnLinea, sol *nucleo.SolicitudEjecucion) error {
	if len(archivos) == 0 {
		return nil
	}
	if s.Cargas == nil {
		return nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("esta instancia no admite cargas"))
	}
	vistos := map[[32]byte]bool{}
	var unicos []archivoEnLinea
	for _, a := range archivos {
		if suma := sha256.Sum256(a.Datos); !vistos[suma] {
			vistos[suma] = true
			unicos = append(unicos, a)
		}
	}
	if len(unicos)+len(sol.Cargas) > nucleo.MaximoCargas {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("demasiadas cargas (máximo %d)", nucleo.MaximoCargas))
	}
	for _, a := range unicos {
		carga, err := s.Cargas.Guardar(a.Nombre, s.nombrePunto(), bytes.NewReader(a.Datos))
		if err != nil {
			return err
		}
		sol.Cargas = append(sol.Cargas, carga.ID)
	}
	return nil
}
