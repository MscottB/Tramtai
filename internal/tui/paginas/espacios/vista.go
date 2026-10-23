package espacios

import (
	"runtime"
	"strings"

	"tramtai/internal/contrato"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

// Vista es el diálogo abierto o la lista, con los detalles del espacio elegido debajo, como su tarjeta en la Web UI.
func (p *Pagina) Vista() string {
	e, ancho, alto := p.estilos, p.c.Ancho, p.c.Alto
	if e == nil {
		return ""
	}
	if p.dlg != nil {
		return p.vistaDialogo()
	}
	if v, ok := e.Preambulo(p.lengua, ancho, p.sinServ, !p.cargado && p.err == ""); ok {
		return v
	}
	if !p.cargado { // la carga falló: el mensaje del servicio tal cual y cómo reintentar
		return "\n" + e.Error.Width(ancho).Render(p.tx("comun.errorCarga")+": "+p.err) + "\n" + e.Tenue.Render(e.Recortar("r "+p.tx("comun.reintentar"), ancho))
	}
	var lineas []string // el título ya está en la cabecera de la app
	if alto >= 12 {
		lineas = append(lineas, e.Tenue.Render(e.Recortar(p.tx("espacios.subtitulo"), ancho)))
	}
	if len(p.espacios) == 0 {
		lineas = append(lineas, "")
		lineas = append(lineas, e.Envolver(p.tx("espacios.vacio"), ancho, 0)...)
		lineas = append(lineas, "", e.Tenue.Render(e.Recortar("n "+p.tx("proyecto.abrir"), ancho)))
		return cortar(lineas, alto)
	}
	detalle := p.detalle()
	libre := alto - len(lineas) - len(detalle) - 1
	if libre < min(len(p.espacios), 3) { // con poco sitio, la lista va antes que los detalles
		detalle, libre = nil, alto-len(lineas)
	}
	lineas = append(lineas, p.lista.Vista(ancho, max(libre, 1)))
	if len(detalle) > 0 {
		lineas = append(lineas, "")
		lineas = append(lineas, detalle...)
	}
	return cortar(lineas, alto)
}

// cortar junta las líneas y deja como mucho alto.
func cortar(lineas []string, alto int) string {
	l := strings.Split(strings.Join(lineas, "\n"), "\n")
	return strings.Join(l[:min(len(l), max(alto, 1))], "\n")
}

// fila es un espacio en la lista: su estado, si se confía en él (solo los proyectos) y si es en el que estás.
func (p *Pagina) fila(x contrato.Espacio) string {
	e := p.estilos
	icono, estilo, estado := e.Iconos.Activo, e.Exito, p.tx("espacios.estadoActivo")
	switch x.Estado {
	case "detenido":
		icono, estilo, estado = e.Iconos.Inactivo, e.Tenue, p.tx("espacios.estadoDetenido")
	case "no_encontrado":
		icono, estilo, estado = e.Iconos.Fallo, e.Error, p.tx("espacios.estadoNoEncontrado")
	}
	partes := []string{estilo.Render(estado)}
	if x.Ambito == "proyecto" && x.Estado != "no_encontrado" {
		if x.Confiado {
			partes = append(partes, e.Exito.Render(p.tx("espacios.confiado")))
		} else {
			partes = append(partes, e.Advertencia.Render(p.tx("espacios.sinConfiar")))
		}
	}
	if x.ID == p.c.Instancia {
		partes = append(partes, e.Acento.Render(p.tx("espacios.actual")))
	}
	return estilo.Render(icono) + " " + pagina.Rellenar(e.Recortar(x.Nombre, 20), 20) + " " + strings.Join(partes, e.Tenue.Render(" "+e.Iconos.Separador+" "))
}

// detalle es lo de la tarjeta del espacio elegido que no cabe en su fila: la carpeta, las APIs, las peticiones, cuándo
// se usó y su último error.
func (p *Pagina) detalle() []string {
	x, hay := p.elegido()
	if !hay {
		return nil
	}
	e, ancho := p.estilos, p.c.Ancho
	ruta := x.Ruta
	if ruta == "" {
		ruta = p.tx("app.sinProyecto")
	}
	apis := p.tx("espacios.apis", "n", pagina.Numero(p.lengua, x.APIs))
	if x.APIs == 1 {
		apis = p.tx("espacios.apisUno")
	}
	peticiones := p.tx("espacios.peticiones", "n", pagina.Numero(p.lengua, x.Peticiones))
	if x.Peticiones == 1 {
		peticiones = p.tx("espacios.peticionesUno")
	}
	meta := []string{apis, peticiones}
	if !x.UltimoUso.IsZero() {
		meta = append(meta, p.tx("espacios.ultimoUso", "cuando", vivo.Hace(p.ahora(), x.UltimoUso)))
	}
	l := []string{e.Tenue.Render(e.Recortar(ruta, ancho)), e.Recortar(strings.Join(meta, " "+e.Iconos.Separador+" "), ancho)}
	if x.UltimoError != "" {
		for _, linea := range e.Envolver(x.UltimoError, ancho, 2) {
			l = append(l, e.Advertencia.Render(linea))
		}
	}
	return l
}

// ejemplo es la ruta de ejemplo de la Web UI para este sistema.
func ejemplo() string {
	if runtime.GOOS == "windows" {
		return `C:\Proyectos\mi-app`
	}
	return "~/proyectos/mi-app"
}

// vistaDialogo es el título y el campo con su explicación o, si el servicio preguntó, la pregunta con lo que hay que
// saber para decidir. Lo que no cabe se corta por abajo: el campo y la pregunta van primero.
func (p *Pagina) vistaDialogo() string {
	e, ancho, d := p.estilos, p.c.Ancho, p.dlg
	var titulo, texto string
	switch d.accion {
	case accProyecto:
		titulo, texto = p.tx("proyecto.titulo"), p.tx("proyecto.texto")
	case accBuscar:
		titulo, texto = p.tx("espacios.buscarTitulo", "nombre", d.e.Nombre), p.tx("espacios.buscarTexto")
	case accOlvidar:
		titulo, texto = p.tx("espacios.olvidarTitulo", "nombre", d.e.Nombre), p.tx("espacios.olvidarTexto", "nombre", d.e.Nombre)
	}
	lineas := []string{e.Titulo.Render(e.Recortar(titulo, ancho))}
	envolver := func(estilo func(...string) string, s string) {
		for _, l := range e.Envolver(s, ancho, 0) {
			lineas = append(lineas, estilo(l))
		}
	}
	switch {
	case d.confianza:
		lineas = append(lineas, e.Tenue.Render(e.Recortar(d.f.Valor("ruta"), ancho)), "")
		envolver(e.Advertencia.Render, p.tx("proyecto.confiarTitulo"))
		if d.movido != "" {
			envolver(e.Texto.Render, p.tx("proyecto.movido", "ruta", d.movido))
		}
		if len(d.claves) > 0 {
			envolver(e.Texto.Render, p.tx("proyecto.confiarTexto"))
			for _, c := range d.claves {
				lineas = append(lineas, e.Recortar("  "+e.Iconos.Separador+" "+c, ancho))
			}
		} else {
			envolver(e.Texto.Render, p.tx("proyecto.sinClaves"))
		}
		lineas = append(lineas, "", e.Tenue.Render(e.Recortar("c "+p.tx("proyecto.confiarYAbrir")+"  "+e.Iconos.Separador+"  a "+p.tx("proyecto.abrirSinConfiar"), ancho)))
	case d.dentro != "":
		lineas = append(lineas, e.Tenue.Render(e.Recortar(d.f.Valor("ruta"), ancho)), "")
		envolver(e.Advertencia.Render, p.tx("proyecto.dentro", "proyecto", d.dentro))
		lineas = append(lineas, "", e.Tenue.Render(e.Recortar("a "+p.tx("proyecto.abrirEse")+"  "+e.Iconos.Separador+"  c "+p.tx("proyecto.crearAqui"), ancho)))
	default:
		lineas = append(lineas, "", d.f.Vista())
		if d.accion != accOlvidar {
			lineas = append(lineas, e.Tenue.Render(e.Recortar("  "+p.tx("proyecto.rutaHint", "ejemplo", ejemplo()), ancho)))
		}
		lineas = append(lineas, "")
		envolver(e.Tenue.Render, texto)
	}
	return cortar(lineas, p.c.Alto)
}
