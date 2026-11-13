package preparar

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/procesos"
)

// CodigoSalidaNoReconocida: el lector no encontró el enlace a tiempo, o el CLI terminó sin dejar la sesión iniciada.
const CodigoSalidaNoReconocida = "salida_no_reconocida"

// maximoPegado acota lo que se pega (Claude: «código#estado», unos 100 caracteres).
const maximoPegado = 1024

// maximoLector es lo que el lector guarda de la salida: el enlace y el código salen en las primeras líneas.
const maximoLector = 64 << 10

// escapes son las secuencias que la salida puede traer y el lector quita: OSC (enlaces OSC 8, portapapeles OSC 52), CSI
// (color), las demás de escape de dos o tres bytes (ESC ( B…) y el CR de las líneas con CRLF.
var escapes = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-?]*[ -/]*[@-~]|\x1b[ -/]*[0-Z\\^-~]|\r`)

// reNumero saca el número de versión de lo que imprime --version («2.1.311 (Claude Code)» → 2.1.311).
var reNumero = regexp.MustCompile(`\d+\.\d+\.\d+[0-9A-Za-z.-]*`)

func numeroVersion(texto string) string { return reNumero.FindString(texto) }

var errNoReconocida = errors.New("salida no reconocida")

// lector busca el enlace, el código y la petición de pegar en la salida acumulada del inicio de sesión, sin escapes
// (llega en trozos: un enlace puede venir partido). Avisa al tenerlos y otra vez si después pide pegar.
type lector struct {
	l      *adaptadores.LectorSesion
	avisar func(enlace, codigo string, pegar bool)

	mu         sync.Mutex
	bruto      []byte
	hay, pegar bool
}

func (x *lector) Write(p []byte) (int, error) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if len(x.bruto)+len(p) > maximoLector {
		return len(p), nil // shortcut: lo que pasa de 64 KB no se mira; el plazo del lector da salida_no_reconocida
	}
	x.bruto = append(x.bruto, p...)
	texto := escapes.ReplaceAllString(string(x.bruto), "")
	if i := strings.IndexByte(texto, 0x1b); i >= 0 {
		texto = texto[:i] // una secuencia a medias: lo que sigue llega en el próximo trozo
	}
	enlace, hayEnlace := valor(x.l.Enlace, texto)
	codigo, hayCodigo := "", true
	if x.l.Codigo != nil {
		codigo, hayCodigo = valor(x.l.Codigo, texto)
	}
	pegar := x.l.PidePegar != nil && x.l.PidePegar.MatchString(texto)
	if hayEnlace && hayCodigo && (!x.hay || pegar != x.pegar) {
		x.hay, x.pegar = true, pegar
		x.avisar(enlace, codigo, pegar)
	}
	return len(p), nil
}

func (x *lector) visto() bool {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.hay
}

// valor es el grupo 1 de e en texto si ya está entero: algo lo sigue (el trozo siguiente podría alargarlo) y no lleva
// caracteres de control.
func valor(e *regexp.Regexp, texto string) (string, bool) {
	m := e.FindStringSubmatchIndex(texto)
	if len(m) < 4 || m[2] < 0 || m[3] >= len(texto) {
		return "", false
	}
	v := texto[m[2]:m[3]]
	return v, !strings.ContainsFunc(v, unicode.IsControl)
}

// sesionTrasInstalar sigue con el inicio de sesión tras instalar, si el CLI lo tiene y aún no hay sesión. Con la versión
// instalada sin comprobar para iniciar sesión, la instalación queda lista y se dice por qué no sigue.
func (c *corrida) sesionTrasInstalar() error {
	if c.r.Sesion == nil || c.pedido.Accion != "instalar" {
		return nil
	}
	if c.p.o.Comprobar != nil && c.p.o.Comprobar(c.ctx, c.agente).Estado == adaptadores.SesionOK {
		return nil
	}
	version := numeroVersion(c.t.g.VersionDespues) // solo la escribe esta corrida
	if c.p.o.Comprobada != nil && !c.p.o.Comprobada(c.agente, "iniciar_sesion", version) {
		aviso := idioma.Tf("%s %s aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté", c.f.Programa, version)
		c.linea(aviso)
		c.anotar(aviso)
		return nil
	}
	c.nuevoPlazo()
	return c.sesion(version)
}

// sesion lanza el inicio de sesión del CLI con tuberías (el M0 no pidió pty a ninguno) y la entrada abierta para lo
// pegado, lee su salida con el lector de la receta y, cuando termina, confirma la sesión con Comprobar.
func (c *corrida) sesion(version string) error {
	l := c.r.Sesion
	ruta := buscar(c.f)
	if ruta == "" {
		return &Error{CodigoNoEncontrado, idioma.Tf("no se encuentra %s", c.f.Programa)}
	}
	c.fijar(func(g *guardado) { g.Estado, g.Etapa = "comprobando", "" })
	lectura, escritura, err := os.Pipe()
	if err != nil {
		return &Error{CodigoInstaladorFallo, err.Error()}
	}
	defer lectura.Close()
	c.p.mu.Lock()
	c.t.entrada = escritura
	c.p.mu.Unlock()
	defer func() {
		c.p.mu.Lock()
		c.t.entrada = nil
		c.p.mu.Unlock()
		escritura.Close()
	}()
	x := &lector{l: l, avisar: func(enlace, codigo string, pegar bool) {
		c.fijar(func(g *guardado) {
			g.Estado, g.Enlace, g.Codigo, g.PidePegar = "esperando_usuario", enlace, codigo, pegar
		})
	}}
	// El plazo del lector cuenta desde que arranca el proceso (Copilot tarda 11 s la primera vez).
	ctx, cortar := context.WithCancelCause(c.ctx)
	defer cortar(nil)
	reloj := time.AfterFunc(c.p.plazoLector, func() {
		if !x.visto() {
			cortar(errNoReconocida)
		}
	})
	defer reloj.Stop()
	// El entorno de la receta (Copilot: COPILOT_DEBUG_BROWSER) evita que el CLI abra un navegador.
	err = c.lanzar(ctx, procesos.Orden{Programa: ruta, Argumentos: l.Argumentos, Entorno: entornoDe(l.Entorno),
		EntradaAbierta: lectura, Bruto: x})
	if c.ctx.Err() == nil && errors.Is(context.Cause(ctx), errNoReconocida) {
		c.anotar("marcar " + version)
		if err := c.p.Marcar(c.agente, version); err != nil {
			c.anotar(err.Error())
		}
		orden := strings.Join(append([]string{c.f.Comando}, l.Argumentos...), " ")
		return &Error{CodigoSalidaNoReconocida, idioma.Tf("no se reconoció lo que escribió %s en %d s: inicia sesión en una consola con %s",
			c.f.Programa, int(math.Ceil(c.p.plazoLector.Seconds())), orden)}
	}
	if err != nil {
		return err
	}
	c.fijar(func(g *guardado) { g.Estado, g.PidePegar = "comprobando", false })
	if c.p.o.Comprobar != nil && c.p.o.Comprobar(c.ctx, c.agente).Estado == adaptadores.SesionSin {
		return &Error{CodigoSalidaNoReconocida, idioma.Tf("%s terminó sin dejar la sesión iniciada", c.f.Programa)}
	}
	return nil
}

// Codigo pasa lo pegado al inicio de sesión en marcha que lo pide, por su entrada y con salto de línea. Lo pegado nunca
// entra en el estado, las líneas, el registro ni los errores.
func (p *Preparador) Codigo(agente, codigo string) error {
	f, ok := p.o.Ficha(agente)
	if !ok {
		return errors.New(idioma.Tf("agente desconocido o sin receta: %s", agente))
	}
	codigo = strings.TrimSpace(codigo)
	if codigo == "" || len(codigo) > maximoPegado || !utf8.ValidString(codigo) ||
		strings.ContainsFunc(codigo, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return errors.New(idioma.Tf("el código pegado no es válido: una sola línea sin espacios, de hasta %d caracteres", maximoPegado))
	}
	p.mu.Lock()
	t := p.tareas[agente]
	if t == nil || t.entrada == nil || t.g.Estado != "esperando_usuario" || !t.g.PidePegar {
		p.mu.Unlock()
		return errors.New(idioma.Tf("%s no está esperando ningún código", f.Programa))
	}
	t.pegados = append(t.pegados, codigo)
	if antes, _, ok := strings.Cut(codigo, "#"); ok && len(antes) >= 8 {
		t.pegados = append(t.pegados, antes) // por si el CLI repite solo el código, sin el estado
	}
	entrada := t.entrada
	p.mu.Unlock()
	if _, err := io.WriteString(entrada, codigo+"\n"); err != nil {
		return errors.New(idioma.Tf("no se pudo pasar el código a %s", f.Programa))
	}
	return nil
}

// Marcar anota en este equipo que esa versión del agente no es compatible (su inicio de sesión no se reconoció) hasta que
// la lista firmada la vuelva a confirmar.
// shortcut: no guarda nada; la Tarea 5 lo escribe en datos/agentes/compatibilidad.json.
func (p *Preparador) Marcar(agente, version string) error { return nil }
