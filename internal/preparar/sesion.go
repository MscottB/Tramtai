package preparar

import (
	"cmp"
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

const (
	// CodigoSalidaNoReconocida: el lector no encontró el enlace a tiempo, o el CLI terminó sin dejar la sesión iniciada.
	CodigoSalidaNoReconocida = "salida_no_reconocida"
	// CodigoInicioSesionFallo: el inicio de sesión del CLI terminó con un código distinto de 0 (un código rechazado…).
	CodigoInicioSesionFallo = "inicio_sesion_fallo"
)

// minimoPegado y maximoPegado acotan lo que se pega (Claude: «código#estado», unos 100 caracteres). El mínimo es el de
// adaptadores.Secretos: lo pegado se tapa en el registro, y algo más corto taparía texto corriente.
const (
	minimoPegado = 8
	maximoPegado = 1024
)

// maximoLector es lo que el lector guarda de la salida: el enlace y el código salen en las primeras líneas.
const maximoLector = 64 << 10

// escapes son las secuencias que la salida puede traer y el lector quita: OSC (enlaces OSC 8, portapapeles OSC 52), CSI
// (color), las demás de escape de dos o tres bytes (ESC ( B…) y el CR de las líneas con CRLF.
var escapes = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-?]*[ -/]*[@-~]|\x1b[ -/]*[0-Z\\^-~]|\r`)

// reNumero saca el número de versión de lo que imprime --version («2.1.311 (Claude Code)» → 2.1.311), sin el punto que
// cierra una frase («GitHub Copilot CLI 1.0.111.» → 1.0.111).
var reNumero = regexp.MustCompile(`\d+\.\d+\.\d+(?:[.-][0-9A-Za-z]+)*`)

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
	n := len(p)
	if x.hay && (x.l.PidePegar == nil || x.pegar) {
		return n, nil // ya está todo
	}
	// shortcut: lo que pasa de 64 KB no se mira; el plazo del lector da salida_no_reconocida.
	if resto := maximoLector - len(x.bruto); len(p) > resto {
		p = p[:resto]
	}
	if len(p) == 0 {
		return n, nil
	}
	x.bruto = append(x.bruto, p...)
	texto := sinEscapes(string(x.bruto))
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
	return n, nil
}

// sinEscapes quita las secuencias de escape. Un ESC que queda en la última línea es una secuencia a medias: se corta ahí
// y lo demás llega en el próximo trozo. Uno con un salto de línea detrás no se reconoce y se quita solo él.
func sinEscapes(bruto string) string {
	texto := escapes.ReplaceAllString(bruto, "")
	for {
		i := strings.IndexByte(texto, 0x1b)
		if i < 0 {
			return texto
		}
		if !strings.Contains(texto[i:], "\n") {
			return texto[:i]
		}
		texto = texto[:i] + texto[i+1:]
	}
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
	if !c.p.o.Comprobada(c.agente, "iniciar_sesion", version) {
		aviso := idioma.Tf("%s aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, %s",
			conVersionSi(c.f.Programa, version), c.p.aMano(c.f))
		c.linea(aviso)
		c.anotar(aviso)
		c.avisoListo = &Error{CodigoVersionNoComprobada, aviso}
		return nil
	}
	c.nuevoPlazo()
	return c.sesion(version)
}

// aMano dice cómo iniciar sesión a mano: en el escritorio, la orden del CLI en una consola («claude auth login»); en el
// servidor, la de Opciones.OrdenConsola con la ruta del CLI instalado (fuera del PATH del servicio), que lo hace como la
// cuenta del servicio. Sin orden (la cuenta virtual de Windows Server no tiene consola), queda la clave de API.
func (p *Preparador) aMano(f adaptadores.Ficha) string {
	args := f.Receta.Sesion.Argumentos
	orden := strings.Join(append([]string{f.Comando}, args...), " ")
	if p.o.OrdenConsola != nil {
		orden = p.o.OrdenConsola(cmp.Or(buscar(f), f.Comando), args)
	}
	if orden == "" {
		return idioma.T("con la cuenta del servicio no se puede iniciar sesión en una consola: usa su clave de API (tramtai-servidor claves guardar)")
	}
	return idioma.Tf("inicia sesión en una consola con %s", orden)
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
	// Ejecutar cierra la lectura en este proceso al lanzarlo: un Codigo tardío falla.
	salida, err := c.lanzar(ctx, procesos.Orden{Programa: ruta, Argumentos: l.Argumentos, Entorno: entornoDe(l.Entorno),
		EntradaAbierta: lectura, Bruto: x})
	if c.ctx.Err() == nil && errors.Is(context.Cause(ctx), errNoReconocida) {
		c.anotar("marcar " + version)
		if err := c.p.Marcar(c.agente, version); err != nil {
			c.anotar(err.Error())
		}
		return &Error{CodigoSalidaNoReconocida, idioma.Tf("no se reconoció lo que escribió %s en %d s: %s",
			c.f.Programa, int(math.Ceil(c.p.plazoLector.Seconds())), c.p.aMano(c.f))}
	}
	if err != nil {
		return err
	}
	if salida != 0 {
		return &Error{CodigoInicioSesionFallo, idioma.Tf("el inicio de sesión de %s falló (terminó con el código %d): vuelve a intentarlo; si no, %s",
			c.f.Programa, salida, c.p.aMano(c.f))}
	}
	c.fijar(func(g *guardado) { g.Estado, g.PidePegar = "comprobando", false })
	if c.p.o.Comprobar != nil && c.p.o.Comprobar(c.ctx, c.agente).Estado == adaptadores.SesionSin {
		return &Error{CodigoSalidaNoReconocida, idioma.Tf("%s terminó sin dejar la sesión iniciada: %s", c.f.Programa, c.p.aMano(c.f))}
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
	if len(codigo) < minimoPegado || len(codigo) > maximoPegado || !utf8.ValidString(codigo) ||
		strings.ContainsFunc(codigo, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return errors.New(idioma.Tf("el código pegado no es válido: una sola línea sin espacios, de %d a %d caracteres", minimoPegado, maximoPegado))
	}
	p.mu.Lock()
	t := p.tareas[agente]
	if t == nil || t.entrada == nil || t.g.Estado != "esperando_usuario" || !t.g.PidePegar {
		p.mu.Unlock()
		return errors.New(idioma.Tf("%s no está esperando ningún código", f.Programa))
	}
	t.pegados = append(t.pegados, codigo)
	if antes, _, ok := strings.Cut(codigo, "#"); ok && len(antes) >= minimoPegado {
		t.pegados = append(t.pegados, antes) // por si el CLI repite solo el código, sin el estado
	}
	entrada := t.entrada
	p.mu.Unlock()
	if _, err := io.WriteString(entrada, codigo+"\n"); err != nil {
		return errors.New(idioma.Tf("no se pudo pasar el código a %s", f.Programa))
	}
	return nil
}
