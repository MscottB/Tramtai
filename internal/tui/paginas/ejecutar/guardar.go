package ejecutar

import (
	"encoding/json"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
)

// vistaCodigo es «Ver como código» abierto: los ejemplos y el elegido.
type vistaCodigo struct {
	ejemplos []Ejemplo
	i        int
}

// apiCreada es la API recién guardada, con su clave: se enseña una sola vez y se olvida al cerrar.
type apiCreada struct{ nombre, url, clave, modelo string }

var reNombreApi = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// verCodigo abre «Ver como código» con la prueba de ahora: la URL de la API que se edita o la
// del servicio, y la clave como marcador. Al editar una API su receta la aplica la propia API: el código solo lleva el
// mensaje, el perfil y el tiempo.
func (p *Pagina) verCodigo() tea.Cmd {
	if p.c.Cli == nil {
		return sinServicio
	}
	d := p.leer()
	esquema, err := esquemaDe(d.esquema)
	if err != nil {
		return comp.Aviso(p.tx("receta.esquemaInvalido"), comp.Fallo)
	}
	var nombres []string
	for _, r := range rutasDe(d.archivos) {
		nombres = append(nombres, nombreArchivo(r))
	}
	s := solicitudDe(d, nombres, esquema)
	if p.api != "" {
		s = nucleo.SolicitudEjecucion{Mensajes: s.Mensajes, Cargas: s.Cargas, Perfil: s.Perfil, Ejecucion: s.Ejecucion}
	} else if d.modelo == "" && d.esfuerzo == "" {
		s.Esfuerzo = p.esfuerzoEspacio()
	}
	if strings.TrimSpace(s.Mensajes[0].Contenido) == "" {
		s.Mensajes[0].Contenido = p.tx("conectar.hola")
		if len(nombres) > 0 {
			s.Mensajes[0].Contenido = p.tx("ejemplos.mensaje")
		}
	}
	c, o, api, punto, clave := p.c, p.origen(), p.api, p.punto, p.tx("codigo.tuClave")
	return func() tea.Msg {
		base := c.Cli.Base
		if r, err := c.Cli.Instancia(c.Ctx, c.Instancia); err == nil && r.Base != "" {
			base = r.Base
		}
		url := base
		if api != "" {
			url = base + "/p/" + api
			if punto != nil && punto.URL != "" {
				url = punto.URL
			}
		}
		return msgCodigo{origen: o, ejemplos: Ejemplos(url, clave, s)}
	}
}

// nombreArchivo es el nombre de un archivo sin su carpeta, también con separadores de Windows.
func nombreArchivo(ruta string) string {
	if i := strings.LastIndexAny(ruta, `/\`); i >= 0 {
		return ruta[i+1:]
	}
	return ruta
}

// ponerCodigo pone en la caja el código del ejemplo elegido.
func (p *Pagina) ponerCodigo() {
	if p.codigo != nil && len(p.codigo.ejemplos) > 0 {
		p.caja.Poner(p.codigo.ejemplos[p.codigo.i].Codigo)
	}
}

func (p *Pagina) teclaCodigo(t string) tea.Cmd {
	v := p.codigo
	switch t {
	case "esc":
		p.codigo = nil
	case "left", "right":
		v.i = (v.i + map[string]int{"left": len(v.ejemplos) - 1, "right": 1}[t]) % len(v.ejemplos)
		p.ponerCodigo()
	case "y":
		return copiar(v.ejemplos[v.i].Codigo)
	default:
		if t != "k" && t != "j" {
			p.caja.Update(teclaDe(t))
		}
	}
	return nil
}

// guardarApi (g) actualiza la API que se edita con la receta o, si no se edita ninguna, abre «Guardar como API».
func (p *Pagina) guardarApi() tea.Cmd {
	if p.c.Cli == nil {
		return sinServicio
	}
	d := p.leer()
	esquema, err := esquemaDe(d.esquema)
	if err != nil {
		return comp.Aviso(p.tx("receta.esquemaInvalido"), comp.Fallo)
	}
	if p.api != "" {
		return p.actualizarApi(d, esquema)
	}
	f := comp.NuevoFormulario(p.estilos, comp.Texto("nombre", p.tx("puntos.nombre"), ""), comp.Texto("grupo", p.tx("puntos.grupo"), ""))
	f.Validar = func(f *comp.Formulario) map[string]string {
		if !reNombreApi.MatchString(strings.TrimSpace(f.Valor("nombre"))) {
			return map[string]string{"nombre": p.tx("puntos.nombreRegla")}
		}
		return nil
	}
	f.Update(tea.WindowSizeMsg{Width: max(p.c.Ancho, 10)})
	p.guardar = &f
	return nil
}

// crearApi crea la API con la receta tal como está; al crearla, Probar pasa a editarla.
func (p *Pagina) crearApi() tea.Cmd {
	d := p.leer()
	esquema, _ := esquemaDe(d.esquema)
	m := aplicarModelo(solicitudDe(datosForm{}, nil, nil), d.modelo)
	cfg := contrato.ConfigPunto{Nombre: strings.TrimSpace(p.guardar.Valor("nombre")), Grupo: strings.TrimSpace(p.guardar.Valor("grupo")),
		Instrucciones: strings.TrimSpace(d.instrucciones), Agente: m.Agente, Modelo: m.Modelo, PerfilModelo: m.PerfilModelo,
		ForzarModelo: d.forzar && d.modelo != "", Esfuerzo: d.esfuerzo, Respaldo: d.respaldo}
	if esquema != nil {
		_ = json.Unmarshal(esquema, &cfg.Esquema)
		cfg.ReintentosSalida = d.reintentos
	}
	modelo := d.modelo
	for _, x := range p.catalogo {
		if modelo == "" && x.Predeterminado {
			modelo = x.ID
		}
	}
	if modelo == "" && len(p.catalogo) > 0 {
		modelo = p.catalogo[0].ID
	}
	c, o := p.c, p.origen()
	return func() tea.Msg {
		e, clave, err := c.Cli.CrearPunto(c.Ctx, c.Instancia, cfg)
		if e.Nombre == "" {
			e.Nombre = cfg.Nombre
		}
		return msgGuardado{origen: o, p: e, clave: clave, err: err, modeloCreada: modelo}
	}
}

// actualizarApi guarda la receta en la API que se edita.
func (p *Pagina) actualizarApi(d datosForm, esquema json.RawMessage) tea.Cmd {
	cambios := map[string]any{"instrucciones": d.instrucciones, "esfuerzo": d.esfuerzo, "modelo": d.modelo,
		"forzar_modelo": d.forzar && d.modelo != "", "respaldo": append([]string{}, d.respaldo...), "esquema": nil, "reintentos_salida": 0}
	if esquema != nil {
		cambios["esquema"], cambios["reintentos_salida"] = esquema, d.reintentos
	}
	c, o, nombre := p.c, p.origen(), p.api
	return func() tea.Msg {
		e, err := c.Cli.CambiarPunto(c.Ctx, c.Instancia, nombre, cambios)
		return msgGuardado{origen: o, p: e, actualizada: true, err: err}
	}
}

func (p *Pagina) alGuardado(m msgGuardado) tea.Cmd {
	if !p.vigente(m.origen) {
		return nil
	}
	if m.err != nil {
		return p.fallo(m.err)
	}
	e := m.p
	if m.actualizada {
		p.punto = &e
		return comp.Aviso(p.tx("ejecutar.apiActualizada", "nombre", e.Nombre), comp.Exito)
	}
	p.guardar = nil
	p.api, p.punto = e.Nombre, &e
	p.creada = &apiCreada{nombre: e.Nombre, url: e.URL, clave: m.clave, modelo: m.modeloCreada}
	p.caja.Poner(p.textoCreada())
	return nil
}

// textoCreada es lo que se enseña de la API nueva: la URL para OpenAI, la clave y un curl para probarla.
func (p *Pagina) textoCreada() string {
	c, e := p.creada, p.estilos
	curl := "curl " + c.url + "/openai/v1/chat/completions \\\n  -H \"Authorization: Bearer " + c.clave + "\" \\\n  -H \"Content-Type: application/json\" \\\n" +
		`  -d '{"model": "` + c.modelo + `", "messages": [{"role": "user", "content": "` + p.tx("conectar.hola") + `"}]}'`
	return strings.Join([]string{p.tx("ejecutar.apiCreadaTexto"), "", e.Advertencia.Render(p.tx("puntos.claveUnaVez")), "",
		e.Tenue.Render(p.tx("conectar.urlOpenai")), c.url + "/openai/v1", "", e.Tenue.Render(p.tx("conectar.clave")), c.clave, "",
		e.Tenue.Render("curl"), curl}, "\n")
}

func (p *Pagina) teclaCreada(t string) tea.Cmd {
	switch t {
	case "esc", "enter":
		p.creada = nil // la clave no se queda en la memoria de la página
		p.caja.Poner("")
	case "y":
		return copiar(p.creada.clave)
	default:
		p.caja.Update(teclaDe(t))
	}
	return nil
}

// teclaDe es la tecla de un nombre, para llevarla a un visor.
func teclaDe(t string) tea.KeyPressMsg {
	switch t {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	}
	if r := []rune(t); len(r) == 1 {
		return tea.KeyPressMsg{Code: r[0], Text: t}
	}
	return tea.KeyPressMsg{}
}
