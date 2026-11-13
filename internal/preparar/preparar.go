// Package preparar instala, actualiza y desinstala los CLI de los agentes con la receta de su ficha, como una tarea por
// agente con estados que la pantalla sigue (la preparación vive en el servicio aunque se cierre la pantalla).
package preparar

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// Códigos de Error y de EstadoPreparacion.MotivoCodigo.
const (
	CodigoVersionNoComprobada = "version_no_comprobada"
	CodigoPeticionesEnCurso   = "peticiones_en_curso"
	CodigoYaEnCurso           = "ya_en_curso"
	CodigoInstaladorFallo     = "instalador_fallo"
	CodigoNoEncontrado        = "no_encontrado_tras_instalar"
	CodigoPlazoAgotado        = "plazo_agotado"
	CodigoSinRed              = "sin_red"
	CodigoInterrumpida        = "interrumpida"
)

// plazoPreparacion acota instalar, actualizar, desinstalar e iniciar sesión (cada una por su lado, también el inicio de
// sesión que sigue a instalar): un instalador colgado termina en plazo_agotado.
const plazoPreparacion = 15 * time.Minute

// maximoLineas son las que se enseñan y se guardan de la salida.
const maximoLineas = 20

// estadoDe es el estado con que empieza cada acción.
var estadoDe = map[string]string{"instalar": "instalando", "actualizar": "actualizando", "desinstalar": "desinstalando",
	"iniciar_sesion": "comprobando"}

// enMarcha dice si un estado es de una preparación que no ha terminado.
func enMarcha(estado string) bool {
	return slices.Contains([]string{"instalando", "actualizando", "desinstalando", "comprobando", "esperando_usuario"}, estado)
}

// Error es un fallo de la preparación con su código (las constantes Codigo*).
type Error struct{ Codigo, Mensaje string }

func (e *Error) Error() string { return e.Mensaje }

// Opciones son lo que la preparación necesita del servicio. Las funciones nunca se llaman con el cerrojo del Preparador
// tomado: pueden llamar a Estado, Iniciar o Cancelar sin bloquearse.
type Opciones struct {
	// Datos es la carpeta de datos de Tramtai: el estado va en datos/agentes/preparaciones.
	Datos string
	// Ficha da la ficha de un id del catálogo; un id que no está se rechaza.
	Ficha func(id string) (adaptadores.Ficha, bool)
	// EnCurso son las peticiones en curso del agente: con alguna no se instala, actualiza ni desinstala (sí se inicia
	// sesión).
	EnCurso func(agente string) int
	// Guardar fija (o quita, con "") comandos_agentes.<agente>.
	Guardar func(agente, ruta string) error
	// Comprobar es ComprobarSesion del agente: tras instalar, si ya hay sesión no se inicia otra; al terminar el inicio
	// de sesión, la confirma. nil: no se comprueba.
	Comprobar func(ctx context.Context, agente string) adaptadores.SesionCLI
	// UltimaVersion da la última versión publicada, para una Descarga sin versión fijada (Copilot en Windows).
	UltimaVersion func(ctx context.Context, agente string) (string, error)
	// Comprobada dice si la versión está comprobada para la acción (Tarea 5); nil deja pasar todas.
	Comprobada func(agente, accion, version string) bool
}

// Preparador lleva una preparación por agente a la vez.
type Preparador struct {
	o     Opciones
	plazo time.Duration
	// plazoLector es lo que se espera al enlace (y al código) del inicio de sesión antes de salida_no_reconocida.
	plazoLector time.Duration
	cliente     *http.Client

	mu     sync.Mutex
	tareas map[string]*tarea
}

type tarea struct {
	g        guardado
	enMarcha bool
	cancelar context.CancelFunc
	// entrada es la del inicio de sesión en marcha, donde Codigo escribe lo pegado; pegados, lo que se pegó, que el
	// registro y las líneas tapan.
	entrada *os.File
	pegados []string
}

// guardado es lo que va en <id>.json: el estado y, para Recuperar, el proceso en marcha y la ruta que puso Tramtai.
type guardado struct {
	contrato.EstadoPreparacion
	PID          int    `json:"pid,omitempty"`
	Firma        string `json:"firma,omitempty"`
	RutaGuardada string `json:"ruta_guardada,omitempty"`
}

// Nuevo crea el preparador; Recuperar recoge lo que dejó un servicio anterior.
func Nuevo(o Opciones) *Preparador {
	return &Preparador{o: o, plazo: plazoPreparacion, plazoLector: 30 * time.Second, cliente: clienteHTTPS(), tareas: map[string]*tarea{}}
}

func (p *Preparador) carpeta() string { return filepath.Join(p.o.Datos, "agentes", "preparaciones") }

// Estado es el de la preparación en marcha o la última del agente; false si no hay ninguna.
func (p *Preparador) Estado(agente string) (contrato.EstadoPreparacion, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, ok := p.tareas[agente]
	if !ok {
		return contrato.EstadoPreparacion{}, false
	}
	return copia(t.g.EstadoPreparacion), true
}

func copia(e contrato.EstadoPreparacion) contrato.EstadoPreparacion {
	e.Lineas = slices.Clone(e.Lineas)
	return e
}

// Iniciar lanza la acción en segundo plano y devuelve su estado. Si ya hay una del agente en marcha, devuelve el estado
// de esa con ya_en_curso.
func (p *Preparador) Iniciar(agente string, pedido contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error) {
	f, ok := p.o.Ficha(agente)
	if !ok || f.Receta == nil {
		return contrato.EstadoPreparacion{}, errors.New(idioma.Tf("agente desconocido o sin receta: %s", agente))
	}
	if pedido.Accion == "iniciar_sesion" && f.Receta.Sesion == nil {
		return contrato.EstadoPreparacion{}, errors.New(idioma.Tf("%s no inicia sesión desde Tramtai: usa su clave de API", f.Programa))
	}
	estado, ok := estadoDe[pedido.Accion]
	if !ok || !disponible(f.Receta, pedido.Accion) {
		return contrato.EstadoPreparacion{}, errors.New(idioma.Tf("acción no disponible para %s en este sistema: %s", agente, pedido.Accion))
	}
	ruta := ""
	if g, err := p.leer(agente); err == nil {
		ruta = g.RutaGuardada
	}
	// Se reserva el agente con el cerrojo y se pregunta por las peticiones sin él (EnCurso puede llamar a Estado).
	p.mu.Lock()
	anterior := p.tareas[agente]
	if anterior != nil && anterior.enMarcha {
		defer p.mu.Unlock()
		return copia(anterior.g.EstadoPreparacion), &Error{CodigoYaEnCurso, idioma.Tf("ya hay una preparación de %s en marcha", f.Programa)}
	}
	if anterior != nil {
		ruta = anterior.g.RutaGuardada
	}
	ahora := time.Now()
	// El plazo va por fases (corrida.nuevoPlazo); cancelar corta todas.
	ctx, cancelar := context.WithCancel(context.Background())
	t := &tarea{enMarcha: true, cancelar: cancelar, g: guardado{RutaGuardada: ruta,
		EstadoPreparacion: contrato.EstadoPreparacion{Agente: agente, Accion: pedido.Accion, Estado: estado, Inicio: &ahora}}}
	p.tareas[agente] = t
	estadoInicial := copia(t.g.EstadoPreparacion)
	p.mu.Unlock()
	if pedido.Accion != "iniciar_sesion" && p.o.EnCurso != nil && p.o.EnCurso(agente) > 0 {
		cancelar()
		p.mu.Lock()
		if anterior != nil {
			p.tareas[agente] = anterior
		} else {
			delete(p.tareas, agente)
		}
		p.mu.Unlock()
		return contrato.EstadoPreparacion{}, &Error{CodigoPeticionesEnCurso, idioma.Tf("%s tiene peticiones en curso: espera a que terminen", f.Programa)}
	}
	c := &corrida{p: p, t: t, agente: agente, f: f, r: f.Receta, base: ctx, pedido: pedido}
	go c.correr(cancelar)
	return estadoInicial, nil
}

// disponible dice si la receta tiene la acción en este sistema.
func disponible(r *adaptadores.Receta, accion string) bool {
	_, instalar := r.Instalar[runtime.GOOS]
	_, descarga := r.Descarga[runtime.GOOS]
	switch accion {
	case "desinstalar":
		return len(r.Borrar[runtime.GOOS]) > 0
	case "iniciar_sesion":
		return r.Sesion != nil
	}
	return instalar || descarga // actualizar sin orden propia (o sin el CLI) es volver a instalar
}

// Cancelar corta la preparación en marcha del agente (cierra el árbol de procesos); sin ninguna, no hace nada.
func (p *Preparador) Cancelar(agente string) error {
	if _, ok := p.o.Ficha(agente); !ok {
		return errors.New(idioma.Tf("agente desconocido o sin receta: %s", agente))
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if t := p.tareas[agente]; t != nil && t.enMarcha {
		t.cancelar()
	}
	return nil
}

// Recuperar, al arrancar, carga el estado de cada agente. Un proceso que quedó anotado (una preparación que el servicio
// dejó a medias, o una cancelación sin confirmar) se termina si sigue vivo y su firma coincide; la preparación a medias
// queda en error con interrumpida. Un agente con una preparación ya en marcha en este servicio no se toca.
func (p *Preparador) Recuperar() {
	archivos, _ := filepath.Glob(filepath.Join(p.carpeta(), "*.json"))
	for _, archivo := range archivos {
		agente := strings.TrimSuffix(filepath.Base(archivo), ".json")
		if _, ok := p.o.Ficha(agente); !ok {
			continue
		}
		p.mu.Lock()
		cargado := p.tareas[agente] != nil
		p.mu.Unlock()
		if cargado {
			continue
		}
		g, err := p.leer(agente)
		if err != nil {
			continue
		}
		if g.PID != 0 || enMarcha(g.Estado) {
			procesos.TerminarHuerfano(g.PID, g.Firma)
			if enMarcha(g.Estado) {
				ahora := time.Now()
				g.Estado, g.MotivoCodigo, g.Mensaje, g.Fin = "error", CodigoInterrumpida,
					idioma.T("el servicio se reinició a mitad de la preparación"), &ahora
			}
			g.PID, g.Firma = 0, ""
		}
		p.mu.Lock()
		if p.tareas[agente] == nil {
			p.tareas[agente] = &tarea{g: g}
			_ = p.guardar(agente, g)
		}
		p.mu.Unlock()
	}
}

func (p *Preparador) leer(agente string) (guardado, error) {
	var g guardado
	datos, err := os.ReadFile(filepath.Join(p.carpeta(), agente+".json"))
	if err == nil {
		err = json.Unmarshal(datos, &g)
	}
	return g, err
}

func (p *Preparador) guardar(agente string, g guardado) error {
	datos, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return configuracion.EscribirAtomico(filepath.Join(p.carpeta(), agente+".json"), datos, 0o600)
}

// corrida es una preparación en marcha.
type corrida struct {
	p *Preparador
	t *tarea
	// agente es el id con que se pidió: la única clave del estado, los archivos y las opciones.
	agente string
	f      adaptadores.Ficha
	r      *adaptadores.Receta
	// base se corta al cancelar; ctx es la fase en curso (base con su plazo), finPlazo la cierra.
	base     context.Context
	ctx      context.Context
	finPlazo context.CancelFunc
	pedido   contrato.PedidoPreparacion
	secretos []string
	registro []string // órdenes y resultado, para <id>.log
	// sinConfirmar es el aviso de un proceso que no confirmó su terminación: su PID y su firma se quedan para Recuperar.
	sinConfirmar string
}

// fijar cambia el estado y lo guarda en disco, con el cerrojo tomado: la última preparación es la última que se escribe.
func (c *corrida) fijar(cambiar func(g *guardado)) {
	c.p.mu.Lock()
	defer c.p.mu.Unlock()
	cambiar(&c.t.g)
	_ = c.p.guardar(c.agente, c.t.g)
}

// formaSecreto son textos con forma de clave o token (Anthropic, OpenAI, GitHub, GitLab, Google, AWS, npm, Slack, JWT,
// Bearer) o una URL
// con usuario y contraseña: el registro y las líneas los tapan aunque no sean claves conocidas.
var formaSecreto = regexp.MustCompile(`\b(sk-[A-Za-z0-9_-]{16,}|gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AIza[0-9A-Za-z_-]{30,}|xox[abprs]-[A-Za-z0-9-]{10,}|npm_[A-Za-z0-9]{30,}|AKIA[0-9A-Z]{16}|ya29\.[0-9A-Za-z_-]{10,}|glpat-[0-9A-Za-z_-]{20,}|eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]+)|(?i:bearer\s+)[A-Za-z0-9._~+/-]{16,}=*|://[^/@\s:]+:[^/@\s]+@`)

// redactar tapa los secretos y lo pegado al inicio de sesión. No se llama con el cerrojo tomado.
func (c *corrida) redactar(texto string) string {
	c.p.mu.Lock()
	pegados := c.t.pegados
	c.p.mu.Unlock()
	return formaSecreto.ReplaceAllString(adaptadores.RedactarCon(texto, append(slices.Clone(c.secretos), pegados...)...), "[REDACTADO]")
}

// nuevoPlazo empieza una fase con su propio plazo (instalar y el inicio de sesión que le sigue tienen 15 min cada uno).
func (c *corrida) nuevoPlazo() {
	if c.finPlazo != nil {
		c.finPlazo()
	}
	c.ctx, c.finPlazo = context.WithTimeout(c.base, c.p.plazo)
}

// linea añade una línea de la salida, sin escapes ANSI ni secretos, a las últimas.
func (c *corrida) linea(texto string) {
	if texto = strings.TrimSpace(c.redactar(escapes.ReplaceAllString(texto, ""))); texto == "" {
		return
	}
	c.p.mu.Lock()
	defer c.p.mu.Unlock()
	c.t.g.Lineas = append(c.t.g.Lineas, texto)
	if exceso := len(c.t.g.Lineas) - maximoLineas; exceso > 0 {
		c.t.g.Lineas = c.t.g.Lineas[exceso:]
	}
}

func (c *corrida) anotar(texto string) {
	c.registro = append(c.registro, time.Now().Format(time.RFC3339)+" "+c.redactar(texto))
}

func (c *corrida) correr(cancelar context.CancelFunc) {
	defer cancelar()
	c.nuevoPlazo()
	defer func() { c.finPlazo() }()
	c.secretos = adaptadores.Secretos() // fuera del cerrojo: en macOS y Linux pregunta al almacén con un proceso por clave
	c.fijar(func(*guardado) {})
	c.anotar(c.pedido.Accion + " " + c.agente)
	err := c.accion()
	codigo, mensaje := "", ""
	estado := "listo"
	var e *Error
	switch {
	case err == nil:
	case errors.Is(c.ctx.Err(), context.DeadlineExceeded):
		estado, codigo, mensaje = "error", CodigoPlazoAgotado, idioma.Tf("la preparación superó su plazo de %d min y se cerró", int(math.Ceil(c.p.plazo.Minutes())))
	case c.ctx.Err() != nil:
		estado = "cancelado"
	case errors.As(err, &e):
		estado, codigo, mensaje = "error", e.Codigo, e.Mensaje
	default:
		estado, codigo, mensaje = "error", CodigoInstaladorFallo, err.Error()
	}
	if c.sinConfirmar != "" {
		mensaje = strings.TrimPrefix(mensaje+". "+c.sinConfirmar, ". ")
	}
	c.p.mu.Lock()
	c.registro = append(c.registro, c.t.g.Lineas...)
	c.p.mu.Unlock()
	c.anotar(strings.TrimSpace(estado + " " + codigo + " " + mensaje))
	_ = configuracion.EscribirAtomico(filepath.Join(c.p.carpeta(), c.agente+".log"), []byte(strings.Join(c.registro, "\n")+"\n"), 0o600)
	ahora := time.Now()
	mensaje = c.redactar(mensaje)
	c.fijar(func(g *guardado) {
		g.Estado, g.Etapa, g.MotivoCodigo, g.Mensaje, g.Fin, g.PidePegar = estado, "", codigo, mensaje, &ahora, false
		c.t.enMarcha = false // con el estado final: quien lo ve terminado ya puede lanzar otra
	})
}

// accion ejecuta lo pedido.
func (c *corrida) accion() error {
	instalado := buscar(c.f)
	antes := ""
	if instalado != "" {
		antes = c.version(instalado)
	}
	c.fijar(func(g *guardado) { g.VersionAntes = antes })
	if c.pedido.Accion == "iniciar_sesion" {
		version := numeroVersion(antes)
		if c.p.o.Comprobada != nil && !c.pedido.Confirmar && !c.p.o.Comprobada(c.agente, c.pedido.Accion, version) {
			return &Error{CodigoVersionNoComprobada, idioma.Tf("%s %s aún no está comprobado con Tramtai", c.f.Programa, version)}
		}
		return c.sesion(version)
	}
	// Actualizar sin orden propia, o sin el CLI instalado, es volver a instalar.
	orden, hayOrden := c.r.Actualizar[runtime.GOOS]
	actualizar := c.pedido.Accion == "actualizar" && hayOrden && instalado != ""
	descarga, hayDescarga := c.r.Descarga[runtime.GOOS]
	// version es la que se instala si se sabe (la última publicada, para una descarga sin versión fijada; la Tarea 5 pone
	// aquí la comprobada) o la que se desinstala.
	version := ""
	switch {
	case c.pedido.Accion == "desinstalar":
		version = antes
	case !actualizar && hayDescarga:
		var err error
		if version, err = c.ultimaVersion(); err != nil {
			return err
		}
	}
	// Tarea 5: la versión debe estar comprobada para esta acción, salvo que el usuario confirme.
	if c.p.o.Comprobada != nil && !c.pedido.Confirmar && !c.p.o.Comprobada(c.agente, c.pedido.Accion, version) {
		return &Error{CodigoVersionNoComprobada, idioma.Tf("%s %s aún no está comprobado con Tramtai", c.f.Programa, version)}
	}
	switch {
	case c.pedido.Accion == "desinstalar":
		return c.desinstalar()
	case actualizar:
		if err := c.ejecutar(orden, ""); err != nil {
			return err
		}
		return c.encontrar()
	}
	if hayDescarga {
		rutas := c.r.Rutas[runtime.GOOS]
		if len(rutas) == 0 {
			return &Error{CodigoInstaladorFallo, idioma.Tf("acción no disponible para %s en este sistema: %s", c.agente, c.pedido.Accion)}
		}
		destino := expandir(rutas[0])
		c.fijar(func(g *guardado) { g.Etapa = "descarga" })
		if err := c.descargar(descarga, version, destino); err != nil {
			return err
		}
	} else if err := c.ejecutar(c.r.Instalar[runtime.GOOS], version); err != nil {
		return err
	}
	if err := c.encontrar(); err != nil {
		return err
	}
	return c.sesionTrasInstalar()
}

func (c *corrida) ultimaVersion() (string, error) {
	if c.p.o.UltimaVersion == nil {
		return "", &Error{CodigoSinRed, idioma.Tf("no se pudo consultar la última versión de %s", c.f.Programa)}
	}
	ctx, cancelar := context.WithTimeout(c.ctx, 10*time.Second)
	defer cancelar()
	version, err := c.p.o.UltimaVersion(ctx, c.agente)
	if err != nil || version == "" {
		return "", &Error{CodigoSinRed, idioma.Tf("no se pudo consultar la última versión de %s", c.f.Programa)}
	}
	return version, nil
}

func (c *corrida) version(ruta string) string {
	ctx, cancelar := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancelar()
	return adaptadores.VersionDe(ctx, ruta)
}

// encontrar busca el CLI tras instalar o actualizar, lee su versión y, si comandos_agentes con su valor predeterminado no
// llega a él (está fuera del PATH del servicio), guarda su ruta.
func (c *corrida) encontrar() error {
	c.fijar(func(g *guardado) { g.Estado, g.Etapa = "comprobando", "" })
	ruta := buscar(c.f)
	if ruta == "" {
		return &Error{CodigoNoEncontrado, idioma.Tf("el instalador terminó, pero no se encuentra %s", c.f.Programa)}
	}
	despues := c.version(ruta)
	guardada := c.t.g.RutaGuardada // solo la escribe esta corrida
	if !mismoArchivo(ruta, enPath(c.f.Comando)) && c.p.o.Guardar != nil {
		if err := c.p.o.Guardar(c.agente, ruta); err != nil {
			return &Error{CodigoInstaladorFallo, idioma.Tf("no se pudo guardar la ruta de %s: %s", c.f.Programa, err.Error())}
		}
		guardada = ruta
	}
	c.fijar(func(g *guardado) { g.VersionDespues, g.RutaGuardada = despues, guardada })
	return nil
}

// desinstalar borra lo de Borrar y, con BorrarDatos, después las carpetas del CLI; y quita de comandos_agentes la ruta que
// puso Tramtai. Solo se borra dentro de la casa de la cuenta del servicio.
func (c *corrida) desinstalar() error {
	c.fijar(func(g *guardado) { g.Etapa = "borrado" })
	lista := c.r.Borrar[runtime.GOOS]
	programa := len(lista) // las de después son DatosCLI
	if c.pedido.BorrarDatos {
		lista = append(slices.Clone(lista), c.r.DatosCLI...)
	}
	casa, err := os.UserHomeDir()
	if err == nil && (!filepath.IsAbs(casa) || filepath.Dir(casa) == casa) {
		err = errors.New(idioma.Tf("la carpeta del usuario (%s) no es válida para borrar nada", casa))
	}
	if err != nil {
		return &Error{CodigoInstaladorFallo, err.Error()}
	}
	// Todas se comprueban antes de borrar ninguna.
	rutas := make([]string, len(lista))
	for i, r := range lista {
		if rutas[i], err = dentroDeCasa(casa, r); err != nil {
			return &Error{CodigoInstaladorFallo, err.Error()}
		}
	}
	for i, ruta := range rutas {
		c.anotar("borrar " + ruta)
		enlace, err := borrar(ruta)
		if err != nil {
			return &Error{CodigoInstaladorFallo, idioma.Tf("no se pudo borrar %s: %s", ruta, err.Error())}
		}
		if enlace && i >= programa {
			aviso := idioma.Tf("%s es un enlace: se quitó el enlace, no lo que hay detrás", ruta)
			c.linea(aviso)
			c.anotar(aviso)
		}
	}
	if c.t.g.RutaGuardada != "" && c.p.o.Guardar != nil {
		if err := c.p.o.Guardar(c.agente, ""); err != nil {
			return &Error{CodigoInstaladorFallo, idioma.Tf("no se pudo guardar la ruta de %s: %s", c.f.Programa, err.Error())}
		}
		c.fijar(func(g *guardado) { g.RutaGuardada = "" })
	}
	return nil
}

// ejecutar lanza una orden de la receta con la entrada cerrada (un instalador que pregunta no se queda esperando) y el
// árbol de procesos atado; anota su PID y su firma para Recuperar.
func (c *corrida) ejecutar(o adaptadores.OrdenReceta, version string) error {
	programa, err := c.resolver(o.Programa)
	if err != nil {
		return err
	}
	args, extra, err := conVersion(o, version)
	if err != nil {
		return err
	}
	c.fijar(func(g *guardado) { g.Etapa = "orden" })
	return c.lanzar(c.ctx, procesos.Orden{Programa: programa, Argumentos: args, Entorno: entornoDe(extra)})
}

// lanzar ejecuta la orden en la casa del usuario con el árbol de procesos atado, anota su PID y su firma para Recuperar y
// pasa su salida a Lineas.
func (c *corrida) lanzar(ctx context.Context, o procesos.Orden) error {
	c.anotar(strings.Join(append([]string{o.Programa}, o.Argumentos...), " "))
	o.Directorio, _ = os.UserHomeDir()
	ctx = procesos.ConAlIniciar(ctx, func(pid int, firma string) {
		c.fijar(func(g *guardado) { g.PID, g.Firma = pid, firma })
	})
	r, err := procesos.Ejecutar(ctx, o, func(l string) error { c.linea(l); return nil })
	if nucleo.CodigoDe(err) == nucleo.CodigoCancelacionNoConfirmada {
		c.sinConfirmar = nucleo.MensajeDe(err) // su PID y su firma se quedan: Recuperar lo terminará
	} else {
		c.fijar(func(g *guardado) { g.PID, g.Firma = 0, "" })
	}
	// stderr llega entero al terminar (procesos solo guarda su cola): sus líneas van detrás de las de stdout.
	for _, l := range strings.Split(r.Errores, "\n") {
		c.linea(l)
	}
	switch {
	case c.ctx.Err() != nil:
		return c.ctx.Err()
	case err != nil:
		return &Error{CodigoInstaladorFallo, nucleo.MensajeDe(err)}
	case r.CodigoSalida != 0:
		return &Error{CodigoInstaladorFallo, idioma.Tf("%s terminó con el código %d", filepath.Base(o.Programa), r.CodigoSalida)}
	}
	return nil
}

// resolver localiza el programa de una orden: el CLI del agente (su Comando) por buscar, no por el PATH del servicio, que
// es de antes de instalarlo; una ruta absoluta tal cual; lo demás (powershell, sh) por el PATH.
func (c *corrida) resolver(programa string) (string, error) {
	switch {
	case programa == c.f.Comando:
		if ruta := buscar(c.f); ruta != "" {
			return ruta, nil
		}
		return "", &Error{CodigoNoEncontrado, idioma.Tf("no se encuentra %s", c.f.Programa)}
	case filepath.IsAbs(programa):
		return programa, nil
	}
	ruta, err := adaptadores.ResolverPrograma(programa)
	if err != nil {
		return "", &Error{CodigoInstaladorFallo, nucleo.MensajeDe(err)}
	}
	return ruta, nil
}

// reVersion es la forma de una versión que puede entrar en una orden o una URL (2.1.311, 0.167.2, 1.0.0-beta).
var reVersion = regexp.MustCompile(`^[0-9]+(\.[0-9A-Za-z-]+)*$`)

func validarVersion(version string) error {
	if !reVersion.MatchString(version) {
		return &Error{CodigoInstaladorFallo, idioma.Tf("versión no válida: %q", version)}
	}
	return nil
}

// conVersion sustituye "{version}" en una orden. Sin versión se omite el argumento que la lleva y la variable queda como
// «NOMBRE=», que entornoDe quita también del entorno heredado. Una versión con otra forma no se usa.
func conVersion(o adaptadores.OrdenReceta, version string) (args, entorno []string, err error) {
	if version != "" {
		if err := validarVersion(version); err != nil {
			return nil, nil, err
		}
	}
	for _, a := range o.Argumentos {
		if strings.Contains(a, "{version}") {
			if version == "" {
				continue
			}
			a = strings.ReplaceAll(a, "{version}", version)
		}
		args = append(args, a)
	}
	for _, e := range o.Entorno {
		if strings.Contains(e, "{version}") {
			if version == "" {
				nombre, _, _ := strings.Cut(e, "=")
				e = nombre + "="
			} else {
				e = strings.ReplaceAll(e, "{version}", version)
			}
		}
		entorno = append(entorno, e)
	}
	return args, entorno, nil
}

// entornoDe es el entorno de una orden: el heredado sin credenciales, con las variables de la receta; una «NOMBRE=»
// vacía no se pasa ni se hereda.
func entornoDe(extra []string) []string {
	return slices.DeleteFunc(adaptadores.EntornoLimpio(nil, extra...), func(par string) bool {
		return strings.HasSuffix(par, "=") && slices.Contains(extra, par)
	})
}
