// Package inicio es la página Inicio de la terminal: la dirección del servicio, la red en vivo (las peticiones en curso),
// los contadores, los Primeros pasos y los avisos (espacios con problema, el conversor de archivos y la sugerencia de una
// API propia). Es la misma página que Panel.vue de la Web UI, con sus cuentas (vivo.go) y sus textos.
package inicio

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/adaptadores"
	"tramtai/internal/autoarranque"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/vivo"
)

const (
	// sinNovedades es lo que aguanta el flujo sin una sola línea, ni un latido (el servicio manda uno cada 15 s): pasado
	// ese tiempo se da por muerto y se reconecta.
	sinNovedades = 45 * time.Second
	// cortesSinFlujo son los cortes seguidos tras los que se consultan los trabajos cada 5 s, como la Web UI.
	cortesSinFlujo = 3
	cadaLimpieza   = 500 * time.Millisecond
	// peticionesRecientes son las filas de «Peticiones recientes».
	peticionesRecientes = 5
	cadaRefresco        = 5 * time.Second
)

// Las teclas de los contadores: cada una abre el Historial con ese estado (los filtros ok, curso y error de la Web UI).
const (
	teclaCompletados = "o"
	teclaEnCurso     = "e"
	teclaConError    = "f"
)

// origen es lo que cada mensaje propio lleva para saber si aún vale: el contexto y el espacio con que se pidió y la
// versión de la página (cada Iniciar, Ocultar o Mostrar la cambia).
type origen struct {
	ctx       context.Context
	instancia string
	gen       int
}

// adaptador y punto son lo poco que Inicio lee de /api/v1/capacidades y /api/v1/puntos.
type adaptador struct {
	Agente         string `json:"agente"`
	Desconectado   bool   `json:"desconectado"`
	Problema       string `json:"problema"`
	ProblemaCodigo string `json:"problema_codigo"`
}

// listo es agenteListo de la Web UI: un aviso (el AGENTS.md de Codex) no impide usar el agente.
func (a adaptador) listo() bool {
	return !a.Desconectado && (a.Problema == "" || a.ProblemaCodigo == adaptadores.ProblemaAviso)
}

type (
	msgInstancia struct {
		origen
		r   cliente.ResumenInstancia
		err error
	}
	msgAgentes struct {
		origen
		lista []adaptador
		err   error
	}
	msgPuntos struct {
		origen
		lista []contrato.EstadoPunto
		err   error
	}
	msgEspacios struct {
		origen
		lista []contrato.Espacio
		err   error
	}
	msgConversor struct {
		origen
		e   contrato.EstadoConversor
		err error
	}
	msgRecientes struct {
		origen
		lista []nucleo.Trabajo
		err   error
	}
	msgRetencion struct {
		origen
		horas int
	}
	msgAutoarranque struct {
		origen
		disponible, activo bool
	}
	msgTrabajos struct {
		origen
		lista []nucleo.Trabajo
		err   error
	}
	// msgFlujo es el flujo recién abierto; msgRed, lo que llega por él (abre, trabajo, foto o corte).
	msgFlujo struct {
		origen
		canal <-chan msgRed
	}
	msgRed struct {
		origen
		tipo   string
		ev     evento
		fallos int
	}
	msgTic      struct{ origen } // cada 500 ms: borra las que terminaron y repinta el tiempo
	msgRefresco struct{ origen } // cada 5 s: vuelve a leer los contadores
)

// Pagina es la página Inicio.
type Pagina struct {
	c       pagina.Contexto
	lengua  string
	estilos *pagina.Estilos
	gen     int
	visible bool
	desp    int // la primera línea que se ve

	inst       cliente.ResumenInstancia
	cargado    bool
	sinServ    bool // el servicio no responde
	err        string
	agentes    []adaptador
	hayAgentes bool
	puntos     []contrato.EstadoPunto
	problemas  []contrato.Espacio
	recientes  []nucleo.Trabajo // las últimas peticiones del espacio
	conv       *contrato.EstadoConversor
	dias       float64
	autoDisp   bool // este sistema admite el arranque automático
	autoActivo bool

	// La red en vivo.
	red       tranvias
	ticActivo bool // el temporizador de limpieza está armado
	borrados  map[string]bool
	enFoto    map[string]bool // los ids de la foto del flujo, hasta que llega el evento foto
	enVivo    bool
	fallos    int // cortes seguidos
	canal     <-chan msgRed
	cancelar  context.CancelFunc

	// Lo que cambian las pruebas: el reloj, las esperas, el flujo y el arranque automático.
	ahora        func() time.Time
	esperaPara   func(time.Duration) <-chan time.Time
	abrir        func(ctx context.Context, instancia string, plazo time.Duration, alEvento func(tipo string, datos []byte) error) error
	autoarranque func() (disponible, activo bool)
}

// Nueva crea la página; su título es el del menú de la Web UI.
func Nueva(lengua string) pagina.Pagina {
	return &Pagina{lengua: lengua, dias: 7, ahora: time.Now, esperaPara: time.After, autoarranque: func() (bool, bool) {
		if !autoarranque.Disponible {
			return false, false
		}
		activo, _ := autoarranque.Activo()
		return true, activo
	}}
}

func (p *Pagina) Titulo() string { return datos.Texto(p.lengua, "menu.inicio") }
func (p *Pagina) Ayuda() string  { return pagina.PaginaInicio }
func (p *Pagina) EnCampo() bool  { return false }

func (p *Pagina) Teclas() []pagina.Tecla {
	teclas := []pagina.Tecla{{Tecla: p.flechas(), Accion: idioma.T("desplaza")}}
	if sugerirApiPropia(p.inst.AppConClave24h) {
		teclas = append(teclas, pagina.Tecla{Tecla: "c", Accion: p.tx("claves.crearApi")})
	}
	if len(p.problemas) > 0 {
		teclas = append(teclas, pagina.Tecla{Tecla: "v", Accion: p.tx("inicio.verEspacios")})
	}
	if p.avisoConversor() {
		teclas = append(teclas, pagina.Tecla{Tecla: "p", Accion: p.tx("conversor.preparar")})
	}
	return append(teclas, pagina.Tecla{Tecla: "g", Accion: p.tx("inicio.detalles")}, pagina.Tecla{Tecla: "h", Accion: p.tx("inicio.verHistorial")},
		pagina.Tecla{Tecla: "a", Accion: p.tx("menu.grupo.empezar")})
}

// flechas es el dibujo de las teclas de desplazar; en ASCII (o antes de conocer los estilos, que es cuando la app puede
// estar en ASCII) son ^v.
func (p *Pagina) flechas() string {
	if p.estilos == nil || p.estilos.ASCII {
		return "^v"
	}
	return "↑↓"
}

// tx es un texto de la Web UI (datos.Texto) en el idioma de la sesión, con sus {marcadores} rellenos.
func (p *Pagina) tx(clave string, pares ...string) string {
	return p.a(datos.Textof(p.lengua, clave, pares...))
}

var textoAscii = strings.NewReplacer("·", "-", "…", "...", "’", "'", "“", "\"", "”", "\"", "—", "-")

// a deja un texto sin los caracteres de dibujo que usan los textos de la Web UI (· … ’) cuando la consola es ASCII.
func (p *Pagina) a(texto string) string {
	if p.estilos != nil && p.estilos.ASCII {
		return textoAscii.Replace(texto)
	}
	return texto
}

// sistema es el nombre del sistema operativo para el paso de arrancar con él, como `sistema` de la Web UI.
func sistema() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	}
	return "Linux"
}

// sugerirApiPropia es sugerirApiPropia de la Web UI (web/src/claves.ts): con más de 20 peticiones de apps con la clave del
// espacio en 24 h conviene darles su propia API.
func sugerirApiPropia(n int) bool { return n > 20 }

// Iniciar empieza de cero: «Cargando…» hasta que llegue lo del espacio, y abre el flujo en vivo.
func (p *Pagina) Iniciar(c pagina.Contexto) tea.Cmd {
	p.c, p.lengua, p.estilos = c, c.Idioma, c.Estilos
	p.gen++
	p.visible = true
	p.inst, p.cargado, p.sinServ, p.err = cliente.ResumenInstancia{}, false, false, ""
	p.agentes, p.hayAgentes, p.puntos, p.problemas, p.conv, p.recientes = nil, false, nil, nil, nil, nil
	p.autoDisp, p.autoActivo, p.desp = false, false, 0
	p.red, p.borrados, p.enFoto, p.enVivo, p.fallos, p.ticActivo = tranvias{}, map[string]bool{}, nil, false, 0, false
	if c.Cli == nil {
		p.cerrarRed()
		p.sinServ = true
		return func() tea.Msg { return pagina.ErrorServicio{Err: cliente.ErrServicioInactivo} }
	}
	return tea.Batch(p.cargar(), p.abrirRed(), p.esperarA(cadaRefresco, func(o origen) tea.Msg { return msgRefresco{o} }))
}

// Ocultar suelta el flujo y los temporizadores: Inicio no gasta nada mientras no se ve (la Web UI lo cierra al salir).
func (p *Pagina) Ocultar() {
	p.visible = false
	p.gen++
	p.cerrarRed()
}

// Mostrar los recupera al volver a Inicio sin iniciarla de nuevo: lo cargado se conserva y se pone al día.
func (p *Pagina) Mostrar() tea.Cmd {
	p.gen++
	p.visible = true
	if p.c.Cli == nil {
		return nil
	}
	return tea.Batch(p.cargar(), p.abrirRed(), p.esperarA(cadaRefresco, func(o origen) tea.Msg { return msgRefresco{o} }))
}

// armarTic arranca el temporizador que borra las filas que terminaron y repinta su tiempo; solo corre mientras haya filas, así
// una página sin peticiones no repinta cada 500 ms.
func (p *Pagina) armarTic() tea.Cmd {
	if p.ticActivo || len(p.red) == 0 {
		return nil
	}
	p.ticActivo = true
	return p.esperarA(cadaLimpieza, func(o origen) tea.Msg { return msgTic{o} })
}

func (p *Pagina) origen() origen { return origen{ctx: p.c.Ctx, instancia: p.c.Instancia, gen: p.gen} }

// vigente dice si una respuesta sigue valiendo: su contexto no se canceló, es del mismo espacio y de esta versión de la
// página.
func (p *Pagina) vigente(o origen) bool {
	return o.ctx != nil && o.ctx.Err() == nil && o.instancia == p.c.Instancia && o.gen == p.gen && p.visible
}

// esperarA es un temporizador: pasado d manda lo que haga mensaje, salvo que el contexto de la página se cancele antes.
func (p *Pagina) esperarA(d time.Duration, mensaje func(origen) tea.Msg) tea.Cmd {
	o, espera := p.origen(), p.esperaPara
	return func() tea.Msg {
		select {
		case <-o.ctx.Done():
			return nil
		case <-espera(d):
			return mensaje(o)
		}
	}
}

// cargar pide todo lo que enseña la página, cada cosa por su lado: si una falla, las demás se ven.
func (p *Pagina) cargar() tea.Cmd {
	c, o, auto := p.c, p.origen(), p.autoarranque
	return tea.Batch(
		func() tea.Msg { r, err := c.Cli.Instancia(c.Ctx, c.Instancia); return msgInstancia{o, r, err} },
		func() tea.Msg {
			r, err := c.Cli.Capacidades(c.Ctx, c.Instancia)
			var lista []adaptador
			if bruto, e := json.Marshal(r["adaptadores"]); e == nil {
				_ = json.Unmarshal(bruto, &lista)
			}
			return msgAgentes{o, lista, err}
		},
		func() tea.Msg { r, err := c.Cli.Puntos(c.Ctx, c.Instancia); return msgPuntos{o, r.Puntos, err} },
		func() tea.Msg { l, err := c.Cli.Espacios(c.Ctx); return msgEspacios{o, l, err} },
		func() tea.Msg {
			l, err := c.Cli.Trabajos(c.Ctx, c.Instancia, peticionesRecientes)
			return msgRecientes{o, l, err}
		},
		func() tea.Msg { e, err := c.Cli.Conversor(c.Ctx, c.Instancia); return msgConversor{o, e, err} },
		func() tea.Msg {
			r, err := c.Cli.Retencion(c.Ctx, c.Instancia)
			if err != nil {
				return nil // sin plazo se sigue contando «los últimos 7 días»
			}
			return msgRetencion{o, r.HorasMetadatos}
		},
		func() tea.Msg { disp, activo := auto(); return msgAutoarranque{o, disp, activo} },
	)
}

// refrescar vuelve a leer lo que cambia solo (los contadores), el conversor mientras se prepara y, sin flujo, los trabajos
// recientes para poner la red al día.
func (p *Pagina) refrescar() tea.Cmd {
	c, o := p.c, p.origen()
	if c.Cli == nil {
		return nil
	}
	cmds := []tea.Cmd{func() tea.Msg { r, err := c.Cli.Instancia(c.Ctx, c.Instancia); return msgInstancia{o, r, err} },
		func() tea.Msg {
			l, err := c.Cli.Trabajos(c.Ctx, c.Instancia, peticionesRecientes)
			return msgRecientes{o, l, err}
		}}
	if p.conv != nil && (p.conv.Estado == "preparando" || p.conv.Etapa != "") {
		cmds = append(cmds, func() tea.Msg { e, err := c.Cli.Conversor(c.Ctx, c.Instancia); return msgConversor{o, e, err} })
	}
	if p.fallos >= cortesSinFlujo && !p.enVivo {
		cmds = append(cmds, func() tea.Msg { l, err := c.Cli.Trabajos(c.Ctx, c.Instancia, 20); return msgTrabajos{o, l, err} })
	}
	return tea.Batch(cmds...)
}

// cerrarRed cancela el flujo en vivo (el goroutine termina y cierra su canal).
func (p *Pagina) cerrarRed() {
	if p.cancelar != nil {
		p.cancelar()
		p.cancelar = nil
	}
	p.canal = nil
}

// abrirRed abre el flujo de GET /api/v1/eventos en un goroutine que manda sus mensajes por un canal; la página lo lee con
// esperar, de uno en uno. Se cancela con el contexto de la página, al salir de Inicio o al volver a abrirlo.
func (p *Pagina) abrirRed() tea.Cmd {
	p.cerrarRed()
	p.red, p.borrados, p.enFoto, p.enVivo, p.fallos, p.ticActivo = tranvias{}, map[string]bool{}, nil, false, 0, false
	ctx, cancelar := context.WithCancel(p.c.Ctx)
	p.cancelar = cancelar
	o, instancia, espera := p.origen(), p.c.Instancia, p.esperaPara
	abrir := p.abrir
	if abrir == nil {
		cli := p.c.Cli
		abrir = func(ctx context.Context, instancia string, plazo time.Duration, alEvento func(string, []byte) error) error {
			return cli.EventosInstanciaVigilado(ctx, instancia, plazo, alEvento)
		}
	}
	return func() tea.Msg {
		canal := make(chan msgRed, 64)
		go flujo(ctx, o, instancia, abrir, espera, canal)
		return msgFlujo{o, canal}
	}
}

// flujo mantiene abierto el flujo en vivo: si se corta (o calla más de sinNovedades) espera 1, 2, 4… hasta 30 s y
// reconecta. Cuenta los cortes seguidos; la foto de un flujo nuevo los pone a cero. Cierra el canal al terminar.
func flujo(ctx context.Context, o origen, instancia string,
	abrir func(context.Context, string, time.Duration, func(string, []byte) error) error,
	espera func(time.Duration) <-chan time.Time, canal chan<- msgRed) {
	defer close(canal)
	enviar := func(m msgRed) bool {
		m.origen = o
		select {
		case canal <- m:
			return true
		case <-ctx.Done():
			return false
		}
	}
	fallos := 0
	for {
		if !enviar(msgRed{tipo: "abre"}) {
			return
		}
		abrir(ctx, instancia, sinNovedades, func(tipo string, datos []byte) error {
			switch tipo {
			case "trabajo":
				var e evento
				if json.Unmarshal(datos, &e) != nil {
					return nil
				}
				if !enviar(msgRed{tipo: tipo, ev: e}) {
					return ctx.Err()
				}
			case "foto":
				fallos = 0
				if !enviar(msgRed{tipo: tipo}) {
					return ctx.Err()
				}
			}
			return nil
		})
		if ctx.Err() != nil {
			return
		}
		fallos++
		if !enviar(msgRed{tipo: "corte", fallos: fallos}) {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-espera(esperaReconexion(fallos - 1)):
		}
	}
}

// fallo reparte un error: de conexión es del servicio (lo avisa la app); el de la API, el mensaje tal cual.
func (p *Pagina) fallo(err error, propio bool) tea.Cmd {
	if pagina.NoResponde(err) {
		p.sinServ = true
		return func() tea.Msg { return pagina.ErrorServicio{Err: err} }
	}
	if propio {
		p.err = nucleo.MensajeDe(err)
	}
	return nil
}

func (p *Pagina) Update(msg tea.Msg) (pagina.Pagina, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		p.c.Ancho, p.c.Alto = m.Width, m.Height
	case msgFlujo:
		if p.vigente(m.origen) {
			p.canal = m.canal
			return p, vivo.Esperar(m.canal)
		}
	case msgRed:
		if !p.vigente(m.origen) {
			return p, nil
		}
		p.alFlujo(m)
		if p.canal == nil {
			return p, nil
		}
		return p, tea.Batch(vivo.Esperar(p.canal), p.armarTic())
	case msgTic:
		if !p.vigente(m.origen) {
			return p, nil
		}
		limpiar(p.red, p.ahora(), p.borrados)
		p.ticActivo = false
		return p, p.armarTic()
	case msgRefresco:
		if !p.vigente(m.origen) {
			return p, nil
		}
		siguiente := p.esperarA(cadaRefresco, func(o origen) tea.Msg { return msgRefresco{o} })
		if p.sinServ { // la app reinicia la página cuando el servicio vuelve
			return p, siguiente
		}
		return p, tea.Batch(p.refrescar(), siguiente)
	case msgInstancia:
		if !p.vigente(m.origen) {
			return p, nil
		}
		if m.err != nil {
			return p, p.fallo(m.err, true)
		}
		p.sinServ, p.cargado, p.err, p.inst = false, true, "", m.r
	case msgAgentes:
		if !p.vigente(m.origen) {
			return p, nil
		}
		if m.err != nil {
			p.agentes, p.hayAgentes = nil, true // como la Web UI: sin respuesta, la lista queda vacía
			return p, p.fallo(m.err, false)
		}
		p.agentes, p.hayAgentes = m.lista, true
	case msgPuntos:
		if p.vigente(m.origen) {
			if m.err != nil {
				return p, p.fallo(m.err, false)
			}
			p.puntos = m.lista
		}
	case msgEspacios:
		if p.vigente(m.origen) {
			if m.err != nil {
				return p, p.fallo(m.err, false)
			}
			p.problemas = nil
			for _, e := range m.lista {
				if e.UltimoError != "" {
					p.problemas = append(p.problemas, e)
				}
			}
		}
	case msgConversor:
		if p.vigente(m.origen) {
			if m.err != nil {
				return p, p.fallo(m.err, false)
			}
			p.conv = &m.e
		}
	case msgRecientes:
		if p.vigente(m.origen) {
			if m.err != nil {
				return p, p.fallo(m.err, false)
			}
			p.recientes = m.lista[:min(len(m.lista), peticionesRecientes)]
		}
	case msgRetencion:
		if p.vigente(m.origen) {
			p.dias = float64(int(float64(m.horas)/24*10+0.5)) / 10
		}
	case msgAutoarranque:
		if p.vigente(m.origen) {
			p.autoDisp, p.autoActivo = m.disponible, m.activo
		}
	case msgTrabajos:
		if p.vigente(m.origen) && m.err == nil {
			eventos := make([]evento, len(m.lista))
			for i, j := range m.lista {
				eventos[i] = eventoDe(j)
			}
			reconciliar(p.red, eventos, p.ahora(), p.borrados)
			return p, p.armarTic()
		}
	case tea.KeyPressMsg:
		return p, p.tecla(m)
	}
	return p, nil
}

// alFlujo aplica a la red lo que llega por el flujo, como useEventosInstancia de la Web UI.
func (p *Pagina) alFlujo(m msgRed) {
	switch m.tipo {
	case "abre":
		p.enFoto = map[string]bool{}
	case "trabajo":
		if p.enFoto != nil {
			p.enFoto[m.ev.ID] = true
		}
		aplicar(p.red, m.ev, p.ahora(), p.borrados)
	case "foto":
		podar(p.red, p.enFoto) // con enFoto nil (ya cerrada) no queda nada que conservar: como en la Web UI, solo la primera
		p.enFoto = nil
		p.fallos, p.enVivo = 0, true
	case "corte":
		p.enVivo, p.fallos = false, m.fallos
	}
}

// tecla atiende una tecla. Esc, si no hay nada que cerrar, vuelve.
func (p *Pagina) tecla(k tea.KeyPressMsg) tea.Cmd {
	ir := func(destino string, datos any) tea.Cmd {
		return func() tea.Msg { return pagina.IrA{Pagina: destino, Datos: datos} }
	}
	switch k.String() {
	case "esc":
		return pagina.Atras
	case "r":
		return p.cargar()
	case "up", "k":
		p.desp = max(p.desp-1, 0)
	case "down", "j":
		p.desp = min(p.desp+1, p.maxDesp())
	case "pgup":
		p.desp = max(p.desp-max(p.c.Alto-1, 1), 0)
	case "pgdown":
		p.desp = min(p.desp+max(p.c.Alto-1, 1), p.maxDesp())
	case "home":
		p.desp = 0
	case "end":
		p.desp = p.maxDesp()
	case "h":
		return ir(pagina.PaginaHistorial, nil)
	case teclaCompletados:
		return ir(pagina.PaginaHistorial, pagina.FiltroEstado{Estado: "ok"})
	case teclaEnCurso:
		return ir(pagina.PaginaHistorial, pagina.FiltroEstado{Estado: "curso"})
	case teclaConError:
		return ir(pagina.PaginaHistorial, pagina.FiltroEstado{Estado: "error"})
	case "g":
		return ir(pagina.PaginaAgentes, nil)
	case "a":
		return ir(pagina.PaginaAyuda, pagina.ConAyuda{Pagina: pagina.PaginaInicio})
	case "c":
		if sugerirApiPropia(p.inst.AppConClave24h) {
			return ir(pagina.PaginaConectar, nil)
		}
	case "v":
		if len(p.problemas) > 0 {
			return ir(pagina.PaginaEspacios, nil)
		}
	case "s":
		if p.avisoConversor() {
			return ir(pagina.PaginaConfiguracion, nil)
		}
	case "p":
		if p.avisoConversor() && p.c.Cli != nil {
			c, o := p.c, p.origen()
			return func() tea.Msg {
				e, err := c.Cli.PrepararConversor(c.Ctx, c.Instancia)
				return msgConversor{o, e, err}
			}
		}
	}
	return nil
}

// avisoConversor dice si el conversor de archivos falta o falló. Preparándose o listo no se avisa, como Panel.vue.
func (p *Pagina) avisoConversor() bool {
	return p.conv != nil && p.conv.Estado != "listo" && p.conv.Estado != "preparando"
}

// textoConversor es la frase del estado del conversor (textoConversor de la Web UI para los estados que se avisan).
func (p *Pagina) textoConversor() string {
	if p.conv.Estado == "error" {
		return p.tx("conversor.error", "mensaje", p.conv.Mensaje) + "\n" + p.tx("conversor.redEmpresa")
	}
	return p.tx("conversor.falta")
}

// maxDesp es hasta dónde se puede desplazar: la última línea se ve en la última fila.
func (p *Pagina) maxDesp() int {
	n := len(p.lineas())
	return max(n-p.ventana(n), 0)
}
