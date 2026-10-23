package ejecutar

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/carpetas"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
)

// Los archivos que se adjuntan a la prueba: la ruta con autocompletado y lo que se comprueba antes de subirlos.

// rutasDe separa el campo de archivos (por comas o punto y coma) y quita espacios y comillas de cada ruta.
func rutasDe(campo string) []string {
	var rutas []string
	for _, ruta := range strings.FieldsFunc(campo, func(r rune) bool { return r == ',' || r == ';' }) {
		if ruta = strings.Trim(strings.TrimSpace(ruta), `"`); ruta != "" {
			rutas = append(rutas, ruta)
		}
	}
	return rutas
}

// revisarArchivos dice, antes de subir nada, si un archivo no se admite o necesita el conversor y no está listo.
func revisarArchivos(c pagina.Contexto, rutas []string) error {
	if err := documentos.AdmitirArchivos(rutas); err != nil {
		return err
	}
	if !slices.ContainsFunc(rutas, documentos.NecesitaConversor) {
		return nil
	}
	e, err := c.Cli.Conversor(c.Ctx, c.Instancia)
	switch {
	case err != nil:
		return err
	case e.Estado != "listo":
		return errors.New(idioma.Tf("el conversor de archivos no está listo (%s): Ctrl+P lo prepara", e.Estado))
	}
	return nil
}

// comprobarConversor avisa, al dejar el campo de archivos, de lo que impedirá enviarlos.
func (p *Pagina) comprobarConversor(rutas []string) tea.Cmd {
	if len(rutas) == 0 || p.c.Cli == nil {
		return nil
	}
	c, o := p.c, p.origen()
	return func() tea.Msg {
		m := msgConversor{origen: o, err: revisarArchivos(c, rutas)}
		if m.err != nil && !pagina.NoResponde(m.err) {
			m.aviso, m.err = nucleo.MensajeDe(m.err), nil
		}
		return m
	}
}

// prepararConversor lanza la preparación del conversor de archivos en el servicio.
func (p *Pagina) prepararConversor() tea.Cmd {
	if p.c.Cli == nil {
		return sinServicio
	}
	c := p.c
	return func() tea.Msg {
		e, err := c.Cli.PrepararConversor(c.Ctx, c.Instancia)
		if err != nil {
			return comp.NuevoAviso{Texto: nucleo.MensajeDe(err), Tipo: comp.Fallo}
		}
		return comp.NuevoAviso{Texto: idioma.Tf("Preparando el conversor de archivos (%s)", e.Estado), Tipo: comp.Info}
	}
}

// completar completa la última ruta del campo (tras la última coma) con lo que hay en disco: el nombre entero si solo hay
// uno que empieza así (una carpeta, con su separador) o lo común a todos. Devuelve también los que encajan.
func completar(campo string) (string, []string) {
	corte := strings.LastIndexAny(campo, ",;") + 1
	resto := campo[corte:]
	ruta := strings.TrimLeft(resto, " ")
	antes := campo[:corte] + resto[:len(resto)-len(ruta)]
	dir, base := filepath.Split(ruta)
	leer := dir
	if leer == "" {
		leer = "."
	}
	entradas, err := os.ReadDir(carpetas.ExpandirCasa(leer))
	if err != nil {
		return campo, nil
	}
	var candidatos []string
	for _, e := range entradas {
		if strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(base)) {
			nombre := e.Name()
			if e.IsDir() {
				nombre += string(filepath.Separator)
			}
			candidatos = append(candidatos, nombre)
		}
	}
	if len(candidatos) == 0 {
		return campo, nil
	}
	comun := candidatos[0]
	for _, c := range candidatos[1:] {
		for !strings.HasPrefix(c, comun) { // por runas: una tilde no se parte por la mitad
			r := []rune(comun)
			comun = string(r[:len(r)-1])
		}
	}
	if len(comun) < len(base) {
		return campo, candidatos
	}
	return antes + dir + comun, candidatos
}
