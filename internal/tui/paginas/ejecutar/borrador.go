package ejecutar

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/comp"

	tea "charm.land/bubbletea/v2"
)

// El borrador es el mensaje que se está escribiendo (recordarBorrador de la Web UI): se guarda en la carpeta de datos un
// momento después de la última tecla, se recupera al abrir la terminal y se borra al enviar.

const archivoBorrador = "terminal-borrador.json"

type borrador struct {
	Mensaje string `json:"mensaje"`
}

// leerBorrador es el mensaje guardado; "" si no hay carpeta de datos o no hay borrador.
func leerBorrador(carpeta string) string {
	if carpeta == "" {
		return ""
	}
	var b borrador
	if datos, err := os.ReadFile(filepath.Join(carpeta, archivoBorrador)); err == nil {
		_ = json.Unmarshal(datos, &b)
	}
	return b.Mensaje
}

// escribirBorrador guarda el mensaje; vacío, borra el archivo. Lo hace fuera de Update, como cualquier Cmd. Si no puede,
// lo avisa: el mensaje no se recuperaría al volver a abrir la terminal.
func escribirBorrador(carpeta, mensaje string) tea.Cmd {
	if carpeta == "" {
		return nil
	}
	ruta := filepath.Join(carpeta, archivoBorrador)
	return func() tea.Msg {
		var err error
		if mensaje == "" {
			if err = os.Remove(ruta); errors.Is(err, fs.ErrNotExist) { // no había borrador
				err = nil
			}
		} else {
			datos, _ := json.Marshal(borrador{Mensaje: mensaje})
			if err = os.MkdirAll(carpeta, 0o700); err == nil {
				err = os.WriteFile(ruta, datos, 0o600)
			}
		}
		if err != nil {
			return comp.NuevoAviso{Texto: idioma.Tf("No se pudo guardar el borrador del mensaje: %v", err), Tipo: comp.Advertencia}
		}
		return nil
	}
}

// programarBorrador guarda el borrador pasada la espera; una tecla antes deja sin efecto la anterior.
func (p *Pagina) programarBorrador() tea.Cmd {
	if p.c.Datos == "" {
		return nil
	}
	p.nborrador++
	n, espera := p.nborrador, p.esperaPara
	return func() tea.Msg {
		<-espera(esperaBorrador)
		return msgBorrador{n: n}
	}
}
