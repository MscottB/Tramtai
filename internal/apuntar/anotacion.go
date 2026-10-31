package apuntar

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"tramtai/internal/configuracion"
)

// Alcance es dónde se apunta la herramienta: su archivo global o el de un proyecto.
type Alcance string

const (
	AlcanceGlobal   Alcance = "global"
	AlcanceProyecto Alcance = "proyecto"
)

// Anotacion es el registro de una activación: qué se escribió y qué había antes, para poder deshacerlo aunque el usuario
// haya tocado el archivo después. Es el único estado que guarda Tramtai de una herramienta.
type Anotacion struct {
	ID        string         `json:"id"`
	Ruta      string         `json:"ruta"`
	Alcance   Alcance        `json:"alcance"`
	Modelo    string         `json:"modelo"`
	API       string         `json:"api"`
	Orden     string         `json:"orden"` // el ejecutable de apiKeyHelper / auth.command: si ya no existe, la activación está desactualizada
	Puerto    int            `json:"puerto"`
	Escrito   map[string]any `json:"escrito"`
	Previos   map[string]any `json:"previos"`
	NoExistia bool           `json:"noExistia"` // el archivo lo creó Tramtai: al desactivar se borra si solo queda lo suyo
	Fecha     time.Time      `json:"fecha"`
}

func carpetaTramtai(datos string) string { return filepath.Join(datos, "herramientas") }

func rutaAnotacion(datos, id string) string { return filepath.Join(carpetaTramtai(datos), id+".json") }

// rutaCopia se deduce de la anotación (id, fecha y extensión del archivo), sin buscar en la carpeta.
func rutaCopia(datos string, a Anotacion) string {
	nombre := a.ID + "-" + a.Fecha.UTC().Format("20060102-150405") + filepath.Ext(a.Ruta)
	return filepath.Join(carpetaTramtai(datos), "copias", nombre)
}

// Leer devuelve la anotación de la herramienta; el segundo valor es false si no está activada.
func Leer(datos, id string) (Anotacion, bool, error) {
	var a Anotacion
	crudo, err := os.ReadFile(rutaAnotacion(datos, id))
	if errors.Is(err, fs.ErrNotExist) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	d := json.NewDecoder(bytes.NewReader(crudo))
	d.UseNumber() // un número de Escrito/Previos vuelve como json.Number, no como un float64 que pierde enteros grandes
	if err := d.Decode(&a); err != nil {
		return a, false, err
	}
	return a, true, nil
}

func guardar(datos string, a Anotacion) error {
	crudo, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return configuracion.EscribirAtomico(rutaAnotacion(datos, a.ID), crudo, 0o600)
}

// Borrar olvida la anotación (no toca el archivo de la herramienta ni la copia); que no exista no es un error.
func Borrar(datos, id string) error {
	err := os.Remove(rutaAnotacion(datos, id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
