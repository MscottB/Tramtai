package ejecutar

import (
	"encoding/json"
	"os"
	"path/filepath"

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

// escribirBorrador guarda el mensaje; vacío, borra el archivo. Lo hace fuera de Update, como cualquier Cmd.
func escribirBorrador(carpeta, mensaje string) tea.Cmd {
	if carpeta == "" {
		return nil
	}
	ruta := filepath.Join(carpeta, archivoBorrador)
	return func() tea.Msg {
		if mensaje == "" {
			_ = os.Remove(ruta)
			return nil
		}
		datos, _ := json.Marshal(borrador{Mensaje: mensaje})
		if os.MkdirAll(carpeta, 0o700) == nil {
			_ = os.WriteFile(ruta, datos, 0o600)
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
