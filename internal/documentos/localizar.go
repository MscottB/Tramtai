package documentos

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// LockConversor fija markitdown y todas sus dependencias con sus hashes (conversor/conversor.in; ver
// docs/equipo/conversor.md para regenerarlo): lo que se prepara es siempre lo mismo.
//
//go:embed conversor/conversor.lock
var LockConversor []byte

// HuellaLock identifica el lock con el que se preparó un conversor (listo.json): si cambia, toca prepararlo de nuevo.
func HuellaLock() string {
	suma := sha256.Sum256(LockConversor)
	return hex.EncodeToString(suma[:])
}

// Origen dice de dónde sale el conversor.
type Origen string

const (
	OrigenConfigurado Origen = "configurado" // conversion.comando cambiado a mano
	OrigenPaquete     Origen = "paquete"     // el paquete del conversor descomprimido junto al ejecutable
	OrigenPreparado   Origen = "preparado"   // preparado por Tramtai en la carpeta del usuario
	OrigenSistema     Origen = "sistema"     // markitdown en el PATH
)

// Ubicacion es el conversor encontrado y cómo se ejecuta.
type Ubicacion struct {
	Origen     Origen
	Version    string // la de listo.json; vacía si es del sistema o configurado
	Programa   string
	Argumentos []string // antes del archivo
	Lock       string   // lock_sha256 de listo.json
}

// Listo se escribe al terminar una preparación: sin él, una carpeta a medias no cuenta como conversor.
type Listo struct {
	Markitdown  string    `json:"markitdown"`
	LockSHA256  string    `json:"lock_sha256"`
	Python      string    `json:"python"`
	PreparadoEn time.Time `json:"preparado_en"`
}

// interprete es el Python independiente de un conversor.
func interprete(dir string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "python", "python.exe")
	}
	return filepath.Join(dir, "python", "bin", "python3")
}

// Localizar busca el conversor: el configurado a mano, el paquete junto al ejecutable, el preparado en la raíz del
// usuario y, por último, markitdown en el PATH.
func Localizar(comando, raiz string) (Ubicacion, bool) {
	return localizarEn(comando, dirEjecutable(), raiz, exec.LookPath)
}

func localizarEn(comando, dirEjecutable, raiz string, buscar func(string) (string, error)) (Ubicacion, bool) {
	if comando != "" && comando != "markitdown" {
		return Ubicacion{Origen: OrigenConfigurado, Programa: comando}, true
	}
	if dirEjecutable != "" {
		if u, ok := leerConversor(filepath.Join(dirEjecutable, "conversor"), OrigenPaquete); ok {
			return u, true
		}
	}
	if raiz != "" {
		if u, ok := leerConversor(filepath.Join(raiz, "conversor"), OrigenPreparado); ok {
			return u, true
		}
	}
	if ruta, err := buscar("markitdown"); err == nil {
		return Ubicacion{Origen: OrigenSistema, Programa: ruta}, true
	}
	return Ubicacion{}, false
}

// leerConversor da el conversor de dir si tiene listo.json y su intérprete.
func leerConversor(dir string, origen Origen) (Ubicacion, bool) {
	datos, err := os.ReadFile(filepath.Join(dir, "listo.json"))
	if err != nil {
		return Ubicacion{}, false
	}
	var l Listo
	if json.Unmarshal(datos, &l) != nil {
		return Ubicacion{}, false
	}
	python := interprete(dir)
	if info, err := os.Stat(python); err != nil || !info.Mode().IsRegular() {
		return Ubicacion{}, false
	}
	return Ubicacion{Origen: origen, Version: l.Markitdown, Programa: python, Argumentos: []string{"-m", "markitdown"}, Lock: l.LockSHA256}, true
}

// dirEjecutable es la carpeta del ejecutable con los enlaces resueltos (como actualizacion.Instalada).
func dirEjecutable() string {
	ruta, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(ruta); err == nil {
		ruta = real
	}
	return filepath.Dir(ruta)
}
