package main

// tramtai agentes instalar|entrar|actualizar|desinstalar|versiones: preparar los CLI de los agentes desde la consola, con
// las mismas reglas y textos que la Web UI (web/src/preparacion.ts) y la terminal (internal/tui/paginas/agentes). El
// servicio hace el trabajo; la orden lo pide, sigue su estado y enseña lo que el usuario tiene que hacer (abrir el enlace,
// pegar un código). No abre ningún navegador.

import (
	"bufio"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"tramtai/internal/agentes"
	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/preparar"
)

const (
	accionInstalar    = "instalar"
	accionActualizar  = "actualizar"
	accionIniciar     = "iniciar_sesion"
	accionDesinstalar = "desinstalar"

	salidaNoPosible = 2   // versión sin comprobar sin confirmar, o acción que ahora no se puede
	salidaFalta     = 3   // hecho, pero falta un paso (queda sin sesión o sin clave)
	salidaCancelado = 130 // Ctrl+C (o cancelada desde otra pantalla)
)

// espacioPreparar es el espacio por el que se pide: preparar los CLI es de todo el servicio, no de un proyecto.
const espacioPreparar = "global"

// cadaConsulta es lo que espera la orden entre dos consultas del estado de una preparación (como el panel de la Web UI).
var cadaConsulta = 1500 * time.Millisecond

// errConCodigo es un fallo de una orden con su código de salida (codigoSalida): 1 es el de siempre.
type errConCodigo struct {
	codigo int
	err    error
}

func (e errConCodigo) Error() string { return e.err.Error() }
func (e errConCodigo) Unwrap() error { return e.err }

func conCodigo(codigo int, mensaje string) error { return errConCodigo{codigo, errors.New(mensaje)} }

// conSalidas añade a la ayuda de una orden de preparar sus códigos de salida.
func conSalidas(texto string) string {
	return texto + "\n\n" + idioma.T("Códigos de salida: 0 listo; 1 error; 2 versión sin comprobar sin confirmar o acción que ahora no se puede; 3 hecho, pero falta un paso (queda sin sesión o sin clave); 130 cancelado (Ctrl+C).")
}

// banderas define una opción booleana con sus alias (el primero es el que sale en la ayuda).
func banderas(cmd *cobra.Command, destino *bool, uso string, nombres ...string) {
	for i, n := range nombres {
		cmd.Flags().BoolVar(destino, n, false, uso)
		if i > 0 {
			_ = cmd.Flags().MarkHidden(n)
		}
	}
}

func banderasConfirmar(cmd *cobra.Command, destino *bool) {
	banderas(cmd, destino, idioma.T("no pregunta: sigue aunque la versión del CLI no esté comprobada"), "confirmar", "confirm", "yes")
}

// comandosPreparar son las órdenes de preparar los CLI de los agentes que cuelgan de «tramtai agentes».
func comandosPreparar() []*cobra.Command {
	var confirmaInstalar, confirmaActualizar, confirmaDesinstalar, todos, borrar bool
	instalar := &cobra.Command{Use: "instalar <agente>...", Aliases: []string{"install"}, Args: cobra.MinimumNArgs(1),
		Short: idioma.T("Instala el CLI de uno o varios agentes con su instalador oficial"),
		Long:  conSalidas(idioma.T("Instala el CLI de uno o varios agentes con su instalador oficial, a través del servicio. Si la versión no está comprobada con Tramtai, pregunta antes de seguir.")),
		RunE: func(cmd *cobra.Command, args []string) error {
			return prepararAgentes(cmd, args, accionInstalar, false, confirmaInstalar)
		}}
	banderasConfirmar(instalar, &confirmaInstalar)

	actualizar := &cobra.Command{Use: "actualizar <agente>... | --todos", Aliases: []string{"update"},
		Short: idioma.T("Actualiza el CLI de uno o varios agentes, o de todos los que tienen una versión nueva"),
		Long:  conSalidas(idioma.T("Actualiza el CLI de uno o varios agentes con su propia orden de actualizar. Con --todos, los instalados con una versión nueva publicada.")),
		RunE: func(cmd *cobra.Command, args []string) error {
			if todos == (len(args) > 0) {
				return errors.New(idioma.T("indica qué agentes actualizar, o --todos para los que tienen una versión nueva (no las dos cosas)"))
			}
			if todos {
				var err error
				if args, err = conVersionNueva(cmd); err != nil {
					return err
				}
				if len(args) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), idioma.T("no hay nada que actualizar: todos los CLI instalados están al día"))
					return nil
				}
			}
			return prepararAgentes(cmd, args, accionActualizar, false, confirmaActualizar)
		}}
	banderas(actualizar, &todos, idioma.T("actualiza todos los instalados con una versión nueva"), "todos", "all")
	banderasConfirmar(actualizar, &confirmaActualizar)

	desinstalar := &cobra.Command{Use: "desinstalar <agente>", Aliases: []string{"uninstall"}, Args: cobra.ExactArgs(1),
		Short: idioma.T("Desinstala el CLI de un agente con su orden oficial (pregunta antes)"),
		Long:  conSalidas(idioma.T("Desinstala el CLI de un agente con su orden oficial: deja de estar también fuera de Tramtai. Su sesión y sus ajustes se quedan, salvo con --borrar-datos.")),
		RunE: func(cmd *cobra.Command, args []string) error {
			return prepararAgentes(cmd, args, accionDesinstalar, borrar, confirmaDesinstalar)
		}}
	banderas(desinstalar, &borrar, idioma.T("borra también su sesión y sus ajustes"), "borrar-datos", "delete-data")
	banderasConfirmar(desinstalar, &confirmaDesinstalar)

	var comprobar bool
	versiones := &cobra.Command{Use: "versiones", Aliases: []string{"versions"}, Args: cobra.NoArgs,
		Short: idioma.T("Muestra la versión instalada, la última y si está comprobada con Tramtai, de cada CLI"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := adminAgentesServidor(); err != nil {
				return err
			}
			c, err := conectar()
			if err != nil {
				return err
			}
			ctx, cancelar := contexto()
			defer cancelar()
			lista, err := c.VersionesAgentes(ctx, espacioPreparar, comprobar)
			if err != nil {
				return err
			}
			return tablaVersiones(cmd, lista)
		}}
	banderas(versiones, &comprobar, idioma.T("vuelve a consultar las versiones ahora (puede tardar unos segundos)"), "comprobar", "check")
	return []*cobra.Command{instalar, actualizar, desinstalar, versiones}
}

// tablaVersiones enseña, por agente, la versión instalada, la última y si la que se usaría está comprobada; debajo, lo que
// el servicio avisa de cada uno.
func tablaVersiones(cmd *cobra.Command, lista []contrato.VersionesAgente) error {
	salida := cmd.OutOrStdout()
	t := tabwriter.NewWriter(salida, 0, 4, 2, ' ', 0)
	fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", idioma.T("AGENTE"), idioma.T("INSTALADA"), idioma.T("ÚLTIMA"), idioma.T("COMPROBADA"))
	for _, v := range lista {
		comprobada := "-"
		if len(v.Comprobadas) > 0 {
			comprobada = idioma.T("sí")
			if slices.Contains(slices.Collect(maps.Values(v.Comprobadas)), false) {
				comprobada = idioma.T("no")
			}
		}
		fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", v.Agente, cmp.Or(v.Instalada, "-"), cmp.Or(v.Ultima, "-"), comprobada)
	}
	if err := t.Flush(); err != nil {
		return err
	}
	for _, v := range lista {
		if v.Motivo != "" {
			fmt.Fprintf(salida, "%s: %s\n", v.Agente, limpio(v.Motivo))
		}
	}
	return nil
}

// conVersionNueva son los agentes instalados con una versión nueva publicada que el servicio deja actualizar ahora.
func conVersionNueva(cmd *cobra.Command) ([]string, error) {
	if err := adminAgentesServidor(); err != nil {
		return nil, err
	}
	c, err := conectar()
	if err != nil {
		return nil, err
	}
	ctx, cancelar := contexto()
	defer cancelar()
	lista, err := c.VersionesAgentes(ctx, espacioPreparar, false)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, v := range lista {
		if v.Instalada != "" && v.Ultima != "" && preparar.CompararVersiones(v.Ultima, v.Instalada) > 0 && slices.Contains(v.Acciones, accionActualizar) {
			ids = append(ids, v.Agente)
		}
	}
	return ids, nil
}

// preparables son los ids del catálogo con receta para instalar y desinstalar sus CLI.
func preparables() []string {
	var ids []string
	for _, f := range agentes.Todos() {
		if f.Receta != nil {
			ids = append(ids, f.Nombre)
		}
	}
	return ids
}

// prepararAgentes pide la acción para cada agente, en orden, y sigue cada preparación hasta el final. Con un id que no se
// puede preparar no toca nada. Devuelve el primer fallo; con Ctrl+C se para.
func prepararAgentes(cmd *cobra.Command, ids []string, accion string, borrar, confirmar bool) error {
	for _, id := range ids {
		if f, ok := agentes.Buscar(id); !ok || f.Receta == nil {
			return fmt.Errorf(idioma.T("%s no es un agente que Tramtai pueda preparar (los hay: %s)"), id, strings.Join(preparables(), ", "))
		}
	}
	if err := adminAgentesServidor(); err != nil {
		return err
	}
	c, err := conectar()
	if err != nil {
		return err
	}
	base := cmd.Context()
	if base == nil {
		base = context.Background()
	}
	if fd := os.Stdin.Fd(); term.IsTerminal(fd) { // el código se lee sin eco: tras Ctrl+C, la terminal vuelve como estaba
		if estado, err := term.GetState(fd); err == nil {
			defer term.Restore(fd, estado)
		}
	}
	ctx, parar := contextoDeOrden(base)
	defer parar()
	p := &preparador{c: c, ctx: ctx, parar: parar, out: cmd.OutOrStdout(), aviso: cmd.ErrOrStderr(), entrada: bufio.NewReader(cmd.InOrStdin()), confirmar: confirmar}
	var primero error
	for _, id := range ids {
		err := p.correr(id, accion, borrar)
		var con errConCodigo
		if err != nil && (!errors.As(err, &con) || con.codigo == salidaCancelado) { // Ctrl+C o un fallo del servicio: no se sigue
			return err
		}
		primero = cmp.Or(primero, err)
	}
	return primero
}

// preparador sigue las preparaciones de una orden.
type preparador struct {
	c          *cliente.Cliente
	ctx        context.Context
	out, aviso io.Writer
	entrada    *bufio.Reader
	confirmar  bool
	parar      context.CancelFunc // suelta las señales: un segundo Ctrl+C mata la orden
	adoptada   bool               // se sigue una preparación que lanzó otra pantalla: esta orden no la cancela
}

// leerLinea lee una línea de la entrada; lo secreto, sin eco si es una terminal.
func (p *preparador) leerLinea(secreto bool) (string, error) {
	if secreto && term.IsTerminal(os.Stdin.Fd()) {
		b, err := term.ReadPassword(os.Stdin.Fd())
		return string(b), err
	}
	s, err := p.entrada.ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}

// preguntar hace una pregunta de sí o no (la «s» o la «y»; cualquier otra cosa es un no); Ctrl+C la deja sin responder.
func (p *preparador) preguntar(pregunta string) bool {
	fmt.Fprint(p.aviso, pregunta+idioma.T(" [s/N] "))
	type respuesta struct {
		linea string
		err   error
	}
	r := make(chan respuesta, 1)
	go func() { l, err := p.leerLinea(false); r <- respuesta{l, err} }()
	select {
	case x := <-r:
		l := strings.ToLower(strings.TrimSpace(x.linea))
		return strings.HasPrefix(l, "s") || strings.HasPrefix(l, "y")
	case <-p.ctx.Done():
		return false
	}
}

// correr hace la acción de un agente: la pide, la sigue y, si se paró por una versión sin comprobar, pregunta y la repite
// con confirmar.
func (p *preparador) correr(id, accion string, borrar bool) error {
	nombre := agentes.NombreVisible(id)
	if accion == accionDesinstalar && !p.confirmar {
		fmt.Fprintln(p.aviso, idioma.Tf("Se desinstala %s con su orden oficial y deja de estar también fuera de Tramtai.", nombre))
		if borrar {
			fmt.Fprintln(p.aviso, idioma.T("Se borran también su sesión y sus ajustes."))
		}
		if !p.preguntar(idioma.Tf("¿Desinstalar %s?", nombre)) {
			if p.ctx.Err() != nil {
				return p.salirCancelado()
			}
			return p.sinCambios(id)
		}
	}
	confirmado := false
	for {
		e, err := p.lanzar(id, contrato.PedidoPreparacion{Accion: accion, BorrarDatos: borrar, Confirmar: confirmado})
		if err != nil {
			return err
		}
		if e, err = p.seguir(id, e); err != nil {
			return err
		}
		e.Accion = cmp.Or(e.Accion, accion)
		if e.Estado != "error" || e.MotivoCodigo != "version_no_comprobada" || e.Accion == accionIniciar || confirmado {
			return p.terminar(id, e)
		}
		fmt.Fprintln(p.aviso, avisoNoComprobada(e.Accion, id))
		if !p.confirmar && !p.preguntar(idioma.T("¿Seguir igualmente?")) {
			if p.ctx.Err() != nil {
				return p.salirCancelado()
			}
			return p.sinCambios(id)
		}
		confirmado = true
	}
}

func (p *preparador) sinCambios(id string) error {
	return conCodigo(salidaNoPosible, idioma.Tf("no se ha cambiado nada en %s", agentes.NombreVisible(id)))
}

// lanzar pide la acción al servicio. Con otra ya en marcha para el agente (esta orden y la Web UI a la vez), sigue esa si
// es la misma acción; si es otra, no la adopta. Con Ctrl+C antes de que el servicio la acepte, no cancela nada: no es suya.
func (p *preparador) lanzar(id string, pedido contrato.PedidoPreparacion) (contrato.EstadoPreparacion, error) {
	p.adoptada = false
	if p.ctx.Err() != nil {
		return contrato.EstadoPreparacion{}, p.salirCancelado()
	}
	e, err := p.c.Preparar(p.ctx, espacioPreparar, id, pedido)
	switch nucleo.CodigoDe(err) {
	case nucleo.CodigoYaEnCurso:
		nombre := agentes.NombreVisible(id)
		if e, err = p.c.Preparacion(p.ctx, espacioPreparar, id); err != nil {
			if p.ctx.Err() != nil {
				return e, p.salirCancelado()
			}
			return e, err
		}
		if e.Accion != "" && e.Accion != pedido.Accion {
			fmt.Fprintln(p.aviso, idioma.Tf("ya hay otra preparación de %s en marcha (%s): espera a que termine o cancélala desde donde la lanzaste", nombre, e.Accion))
			return e, conCodigo(salidaNoPosible, idioma.Tf("no se ha cambiado nada en %s", nombre))
		}
		fmt.Fprintln(p.out, idioma.Tf("ya hay una preparación de %s en marcha: se sigue esa", nombre))
		p.adoptada = true
		return e, nil
	case nucleo.CodigoPeticionesEnCurso, nucleo.CodigoNoDisponibleEnServidor:
		return e, errConCodigo{salidaNoPosible, err}
	}
	if err != nil && p.ctx.Err() != nil {
		return e, p.salirCancelado()
	}
	return e, err
}

// enMarcha: la preparación aún no terminó.
func enMarcha(e contrato.EstadoPreparacion) bool {
	return e.Estado != "" && !slices.Contains([]string{"listo", "error", "cancelado"}, e.Estado)
}

// codigoPegado es lo que el usuario escribió cuando el CLI esperaba un código.
type codigoPegado struct {
	codigo string
	err    error
}

// visto es lo último que la orden ha dicho de una preparación, para no repetirlo.
type visto struct{ estado, etapa, enlace, codigo string }

// seguir consulta el estado hasta que termine, enseñando el progreso, el enlace y el código; con «pide_pegar» lee el
// código de la entrada y lo envía (nunca lo imprime). Ctrl+C cancela la preparación en el servicio.
func (p *preparador) seguir(id string, e contrato.EstadoPreparacion) (contrato.EstadoPreparacion, error) {
	var dicho visto
	var pegado chan codigoPegado
	enviado, fallos := false, 0
	turno := time.NewTicker(cadaConsulta)
	defer turno.Stop()
	for enMarcha(e) {
		p.mostrar(id, e, &dicho)
		quierePegar := e.Estado == "esperando_usuario" && e.PidePegar
		if !quierePegar {
			enviado = false
		}
		if quierePegar && pegado == nil && !enviado {
			fmt.Fprint(p.aviso, idioma.T("Pega aquí el código (no se muestra): "))
			pegado = make(chan codigoPegado, 1)
			go func(c chan<- codigoPegado) {
				l, err := p.leerLinea(true)
				c <- codigoPegado{strings.TrimSpace(l), err}
			}(pegado)
		}
		select {
		case <-p.ctx.Done():
			return e, p.cancelar(id)
		case r := <-pegado:
			pegado = nil
			fmt.Fprintln(p.aviso)
			switch {
			case r.err != nil:
				p.cancelar(id)
				return e, fmt.Errorf(idioma.T("no se pudo leer el código: %w"), r.err)
			case r.codigo == "":
			default:
				if err := p.c.CodigoPreparacion(p.ctx, espacioPreparar, id, r.codigo); err != nil {
					if p.ctx.Err() != nil {
						return e, p.cancelar(id)
					}
					fmt.Fprintln(p.aviso, nucleo.MensajeDe(err)) // no llegó: se vuelve a pedir
				} else {
					enviado = true
				}
			}
		case <-turno.C:
			n, err := p.c.Preparacion(p.ctx, espacioPreparar, id)
			switch {
			case p.ctx.Err() != nil:
				return e, p.cancelar(id)
			case err != nil:
				if fallos++; fallos >= 3 { // como la Web UI: tres intentos y, si sigue fallando, el error
					return e, err
				}
			default:
				fallos, e = 0, n
			}
		}
	}
	return e, nil
}

// cancelar corta la preparación en el servicio (aunque el contexto de la orden ya esté cancelado) y sale con 130. Una que
// lanzó otra pantalla no se corta.
func (p *preparador) cancelar(id string) error {
	if p.adoptada {
		fmt.Fprintln(p.aviso, "\n"+idioma.Tf("La preparación de %s sigue en marcha: la lanzó otra pantalla; cancélala desde allí", agentes.NombreVisible(id)))
		return p.salirCancelado()
	}
	p.parar()
	ctx, parar := context.WithTimeout(context.WithoutCancel(p.ctx), 10*time.Second)
	defer parar()
	if err := p.c.CancelarPreparacion(ctx, espacioPreparar, id); err != nil {
		fmt.Fprintln(p.aviso, "\n"+nucleo.MensajeDe(err))
	}
	fmt.Fprintln(p.aviso, "\n"+idioma.Tf("Se canceló la preparación de %s.", agentes.NombreVisible(id)))
	return conCodigo(salidaCancelado, idioma.T("cancelado"))
}

// salirCancelado sale con 130 sin tocar el servicio: lo que se cancela no lo lanzó esta orden (o aún no existe).
func (p *preparador) salirCancelado() error {
	p.parar()
	fmt.Fprintln(p.aviso)
	return conCodigo(salidaCancelado, idioma.T("cancelado"))
}

// mostrar dice qué pasa ahora: el progreso por fases y, al esperar al usuario, el enlace y el código como texto plano.
func (p *preparador) mostrar(id string, e contrato.EstadoPreparacion, dicho *visto) {
	nombre := agentes.NombreVisible(id)
	if e.Estado != dicho.estado || e.Etapa != dicho.etapa {
		dicho.estado, dicho.etapa = e.Estado, e.Etapa
		fmt.Fprintln(p.out, progreso(e, nombre))
		if clave, enlace := agentes.AvisoSesion(id); e.Estado == "esperando_usuario" && clave != "" { // el de la receta (Codex: código de dispositivo)
			if aviso := textoAviso(clave); aviso != "" {
				fmt.Fprintln(p.out, aviso)
				fmt.Fprintln(p.out, idioma.T("Abrir los ajustes de seguridad de ChatGPT")+": "+enlace)
			}
		}
	}
	if enlace := enlaceSeguro(e.Enlace); enlace != "" && enlace != dicho.enlace {
		dicho.enlace = enlace
		fmt.Fprintln(p.out, enlace)
	}
	if codigo := limpio(e.Codigo); codigo != "" && codigo != dicho.codigo {
		dicho.codigo = codigo
		fmt.Fprintln(p.out, idioma.T("Código")+": "+codigo)
	}
}

func progreso(e contrato.EstadoPreparacion, nombre string) string {
	var t string
	switch e.Estado {
	case "instalando":
		t = idioma.Tf("Instalando %s…", nombre)
	case "actualizando":
		t = idioma.Tf("Actualizando %s…", nombre)
	case "desinstalando":
		t = idioma.Tf("Desinstalando %s…", nombre)
	case "comprobando":
		t = idioma.Tf("Comprobando %s…", nombre)
	case "esperando_usuario":
		if e.PidePegar {
			return idioma.Tf("Abre el enlace, inicia sesión en %s y pega aquí el código que te den.", nombre)
		}
		return idioma.Tf("Abre el enlace e introduce este código para iniciar sesión en %s. Tramtai sigue en cuanto lo apruebes.", nombre)
	default:
		t = e.Estado
	}
	switch e.Etapa {
	case "descarga":
		t += " · " + idioma.T("descargando")
	case "orden":
		t += " · " + idioma.T("con su instalador oficial")
	case "borrado":
		t += " · " + idioma.T("borrando sus archivos")
	}
	return t
}

// avisoNoComprobada es lo que pasa si se sigue con una versión sin comprobar (el CLI no la ha probado Tramtai).
func avisoNoComprobada(accion, id string) string {
	titulo := idioma.Tf("Esta versión de %s aún no está comprobada con Tramtai.", agentes.NombreVisible(id))
	switch {
	case accion == accionActualizar:
		return idioma.T("La versión nueva aún no está comprobada: si actualizas, el inicio de sesión automático quedará desactivado hasta que lo esté.")
	case accion == accionInstalar && agentes.AdmiteSuscripcion(id):
		return titulo + " " + idioma.T("Si lo instalas, el inicio de sesión automático quedará desactivado hasta que lo esté.")
	case accion == accionDesinstalar:
		return titulo + " " + idioma.T("Si sigues, se desinstala igualmente.")
	case accion == accionIniciar:
		return titulo + " " + idioma.T("El inicio de sesión automático vuelve cuando lo esté. Mientras, inicia sesión en una consola con la orden de abajo.")
	}
	return titulo
}

// terminar dice cómo acabó la preparación y da su código de salida: 0 solo si quedó listo de verdad.
func (p *preparador) terminar(id string, e contrato.EstadoPreparacion) error {
	nombre := agentes.NombreVisible(id)
	sinDato := idioma.T("Sin dato")
	antes, despues := cmp.Or(e.VersionAntes, sinDato), cmp.Or(e.VersionDespues, sinDato)
	switch e.Estado {
	case "listo":
		if e.MotivoCodigo == "" {
			fmt.Fprintln(p.out, map[string]string{
				accionInstalar:    idioma.Tf("%s está instalado (%s).", nombre, despues),
				accionActualizar:  idioma.Tf("%s está actualizado: antes %s, ahora %s.", nombre, antes, despues),
				accionIniciar:     idioma.Tf("Sesión iniciada en %s: ya puedes usarlo.", nombre),
				accionDesinstalar: idioma.Tf("%s está desinstalado.", nombre),
			}[e.Accion])
			return nil
		}
		// Listo con motivo: la acción se hizo, pero no queda listo para usar.
		quedaSinListo := idioma.Tf("%s no quedó listo para usar.", nombre)
		if e.Accion == accionInstalar && agentes.AdmiteSuscripcion(id) {
			quedaSinListo = idioma.Tf("%s está instalado (%s), pero sin iniciar sesión: inicia sesión con «tramtai agentes entrar %s».", nombre, despues, id)
		} else if e.Accion == accionInstalar {
			quedaSinListo = idioma.Tf("%s está instalado (%s), pero le falta su clave de API (tramtai-servidor claves guardar).", nombre, despues)
		}
		if e.Mensaje != "" {
			fmt.Fprintln(p.aviso, limpio(e.Mensaje))
		}
		if e.Accion == accionIniciar {
			fmt.Fprintln(p.aviso, quedaSinListo)
			return conCodigo(1, idioma.Tf("No se pudo iniciar sesión en %s", nombre))
		}
		return conCodigo(salidaFalta, quedaSinListo) // hecho, pero falta un paso
	case "cancelado":
		fmt.Fprintln(p.aviso, idioma.Tf("Se canceló la preparación de %s.", nombre))
		return conCodigo(salidaCancelado, idioma.T("cancelado"))
	case "error":
		return p.fallo(id, e)
	}
	return conCodigo(1, idioma.T("el servicio no dio el estado de la preparación"))
}

// fallo explica un error: qué pasó y qué hacer, el detalle del servicio y las últimas líneas de la salida del CLI.
func (p *preparador) fallo(id string, e contrato.EstadoPreparacion) error {
	nombre := agentes.NombreVisible(id)
	codigo := 1
	if e.MotivoCodigo == "version_no_comprobada" { // al iniciar sesión nunca se confirma: da la orden de la consola
		codigo = salidaNoPosible
		fmt.Fprintln(p.aviso, avisoNoComprobada(e.Accion, id))
	} else if x := explicacion(e.MotivoCodigo); x != "" {
		fmt.Fprintln(p.aviso, x)
	}
	if e.MotivoCodigo == "no_disponible_en_servidor" {
		codigo = salidaNoPosible
	}
	if e.Mensaje != "" {
		fmt.Fprintln(p.aviso, limpio(e.Mensaje))
	}
	if e.MotivoCodigo != "version_no_comprobada" {
		for _, linea := range e.Lineas[max(len(e.Lineas)-5, 0):] {
			fmt.Fprintln(p.aviso, "  "+limpio(linea))
		}
	}
	titulo := map[string]string{
		accionInstalar:    idioma.Tf("No se pudo instalar %s", nombre),
		accionActualizar:  idioma.Tf("No se pudo actualizar %s", nombre),
		accionIniciar:     idioma.Tf("No se pudo iniciar sesión en %s", nombre),
		accionDesinstalar: idioma.Tf("No se pudo desinstalar %s", nombre),
	}[e.Accion]
	return conCodigo(codigo, cmp.Or(titulo, idioma.Tf("No se pudo preparar %s", nombre)))
}

// explicacion dice qué pasó y qué hacer con un código de error de preparar.
func explicacion(codigo string) string {
	switch codigo {
	case "instalador_fallo":
		return idioma.T("El instalador terminó con un error: mira las últimas líneas y vuelve a intentarlo.")
	case "no_encontrado_tras_instalar":
		return idioma.T("Tramtai no encuentra el CLI: escribe su ruta en comandos_agentes (configuración de Tramtai).")
	case "plazo_agotado":
		return idioma.T("Tardó demasiado y se cerró: vuelve a intentarlo.")
	case "salida_no_reconocida":
		return idioma.T("Tramtai no reconoció lo que escribió el CLI: inicia sesión en una consola con la orden de abajo.")
	case "inicio_sesion_fallo":
		return idioma.T("El inicio de sesión del CLI terminó con un error: vuelve a intentarlo o hazlo en una consola.")
	case "sin_red":
		return idioma.T("No hay conexión con el sitio del CLI: revisa tu red y vuelve a intentarlo.")
	case "no_disponible_en_servidor":
		return idioma.T("Esto no se puede hacer con la cuenta del servicio: usa una clave de API.")
	case "interrumpida":
		return idioma.T("El servicio se reinició a mitad: vuelve a intentarlo.")
	}
	return ""
}

// enlaceSeguro devuelve el enlace solo si es https y no lleva caracteres de control: lo que llega del servicio nunca se
// enseña con otro esquema.
func enlaceSeguro(url string) string {
	if !strings.HasPrefix(url, "https://") || strings.IndexFunc(url, unicode.IsControl) >= 0 || strings.ContainsAny(url, " \t") {
		return ""
	}
	return url
}

// limpio quita de una línea de la salida del CLI lo que no es texto (secuencias de escape, controles).
func limpio(linea string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, ansi.Strip(linea))
}

// textoAviso es el texto de la CLI de un aviso de sesión de la receta (su clave de texto en la Web UI); vacío si no lo conoce.
func textoAviso(clave string) string {
	if clave == "preparar.avisoCodex" {
		return idioma.T("Codex inicia sesión con un código de dispositivo, y ChatGPT pide activarlo una vez en sus ajustes de seguridad.")
	}
	return ""
}
