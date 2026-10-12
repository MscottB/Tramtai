// Package documentos prepara las entradas de un trabajo: guarda las cargas, las convierte a Markdown con
// markitdown y las ensambla con delimitadores explícitos (sección 10).
package documentos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"tramtai/internal/nucleo"
)

// Carga es un archivo recibido, identificado por su contenido (sección 7.1).
type Carga struct {
	ID       string    `json:"id"`
	Nombre   string    `json:"nombre"`
	SHA256   string    `json:"sha256"`
	Bytes    int64     `json:"tamano_bytes"`
	CreadoEn time.Time `json:"creado_en"`
	VenceEn  time.Time `json:"vence_en"`
}

// Cargas guarda las cargas de una instancia en un directorio privado: el contenido en <id> y los metadatos en <id>.json.
// shortcut: el vencimiento se comprueba al usarla; la purga periódica y la protección mientras un trabajo la usa llegan con la retención (M7).
type Cargas struct {
	dir         string
	vigencia    time.Duration
	maximoBytes int64
}

// AbrirCargas crea el directorio si no existe.
func AbrirCargas(dir string, vigencia time.Duration, maximoBytes int64) (*Cargas, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Cargas{dir: dir, vigencia: vigencia, maximoBytes: maximoBytes}, nil
}

// Guardar copia el contenido calculando su hash; el mismo contenido produce el mismo identificador.
func (c *Cargas) Guardar(nombre string, r io.Reader) (Carga, error) {
	temporal, err := os.CreateTemp(c.dir, "subida-*")
	if err != nil {
		return Carga{}, err
	}
	defer os.Remove(temporal.Name()) // sin efecto si ya se renombró
	resumen := sha256.New()
	n, err := io.Copy(io.MultiWriter(temporal, resumen), io.LimitReader(r, c.maximoBytes+1))
	if cerrar := temporal.Close(); err == nil {
		err = cerrar
	}
	if err != nil {
		return Carga{}, err
	}
	if n > c.maximoBytes {
		return Carga{}, nucleo.NuevoError(nucleo.CodigoCargaInvalida, "el archivo supera el tamaño máximo de carga")
	}
	suma := hex.EncodeToString(resumen.Sum(nil))
	ahora := time.Now().UTC()
	carga := Carga{ID: "carga_" + suma[:32], Nombre: nombreSeguro(nombre), SHA256: suma, Bytes: n, CreadoEn: ahora, VenceEn: ahora.Add(c.vigencia)}
	if err := os.Rename(temporal.Name(), filepath.Join(c.dir, carga.ID)); err != nil {
		return Carga{}, err
	}
	metadatos, _ := json.Marshal(carga)
	return carga, os.WriteFile(filepath.Join(c.dir, carga.ID+".json"), metadatos, 0o600)
}

// Obtener devuelve la carga y la ruta de su contenido, o entrada_no_disponible si no existe o venció.
func (c *Cargas) Obtener(id string) (Carga, string, error) {
	noDisponible := nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, "la carga "+id+" no existe o venció; vuelve a enviarla")
	if !nucleo.ReCarga.MatchString(id) {
		return Carga{}, "", noDisponible
	}
	datos, err := os.ReadFile(filepath.Join(c.dir, id+".json"))
	if err != nil {
		return Carga{}, "", noDisponible
	}
	var carga Carga
	if json.Unmarshal(datos, &carga) != nil || time.Now().After(carga.VenceEn) {
		return Carga{}, "", noDisponible
	}
	ruta := filepath.Join(c.dir, id)
	if _, err := os.Stat(ruta); err != nil {
		return Carga{}, "", noDisponible
	}
	return carga, ruta, nil
}

// nombreSeguro conserva solo el nombre base, sin caracteres de control y con longitud acotada.
func nombreSeguro(nombre string) string {
	nombre = filepath.Base(strings.ReplaceAll(nombre, "\\", "/"))
	nombre = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, nombre)
	if runas := []rune(nombre); len(runas) > 200 {
		nombre = string(runas[len(runas)-200:])
	}
	if nombre == "" || nombre == "." || nombre == "/" {
		return "archivo"
	}
	return nombre
}
