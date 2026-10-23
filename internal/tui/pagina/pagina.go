// Package pagina es lo que comparten la terminal y sus páginas: la interfaz de una página, lo que recibe al iniciarse,
// los mensajes con los que habla con la app y los estilos. Así las páginas no importan internal/tui.
package pagina

import (
	"context"
	"errors"
	"net/url"
	"time"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
)

// Contexto es lo que una página recibe en Iniciar. La app lo vuelve a dar al entrar en la página, al cambiar de espacio
// y al reintentar con el servicio caído.
type Contexto struct {
	// Ctx se cancela cuando la app vuelve a iniciar la página, al cambiar de espacio y al salir. Las peticiones de la
	// página lo usan; una respuesta con Ctx ya cancelado (o de otra Instancia) es vieja y se descarta.
	Ctx         context.Context
	Cli         *cliente.Cliente // nil si el servicio no está en marcha: la página devuelve ErrorServicio
	Instancia   string           // id del espacio activo
	Idioma      string           // "es" | "en"
	Estilos     *Estilos         // los mismos durante toda la sesión: cambian por dentro si cambia el fondo
	Ancho, Alto int              // el área de la página, sin cabecera, menú ni pie
	Proyecto    string           // raíz del proyecto desde el que se abrió la terminal; vacía fuera de uno
	Datos       string           // la carpeta de datos de Tramtai (Opciones.Datos): ahí guarda Probar su borrador
}

// Claves de las páginas: las de la Web UI. IrA con otra clave no hace nada.
const (
	PaginaInicio        = "inicio"
	PaginaAgentes       = "agentes"
	PaginaEjecutar      = "ejecutar"
	PaginaConectar      = "conectar"
	PaginaHistorial     = "historial"
	PaginaEspacios      = "espacios"
	PaginaConfiguracion = "configuracion"
	PaginaAyuda         = "ayuda"
)

// Pagina es una página de la terminal. La app le pasa las teclas solo cuando es la actual; el resto de mensajes (las
// respuestas de sus Cmd, CambioEspacio…) le llegan aunque no lo sea. Al cambiar el tamaño recibe un tea.WindowSizeMsg
// con el de su área, no el de la ventana. Esc es de la página: si no tiene nada que cerrar, devuelve Atras.
type Pagina interface {
	Titulo() string                   // el nombre del menú; se puede llamar antes de Iniciar
	Iniciar(Contexto) tea.Cmd         // carga sus datos
	Update(tea.Msg) (Pagina, tea.Cmd) // tea.Model sin Init ni View
	Vista() string                    // como mucho Ancho×Alto; lo que sobre se corta
	Teclas() []Tecla                  // para el pie, además de las comunes
	EnCampo() bool                    // un campo de texto tiene el foco: las teclas comunes no se aplican
	Ayuda() string                    // la página de la Ayuda (el campo pagina de su tema): inicio, ejecutar…
}

// Ocultable la implementa una página con algo en vivo que solo debe correr mientras se ve (el flujo de Inicio). La app
// llama a Ocultar al dejar la página (menú, atajos, IrA, ?) y a Mostrar al volver a ella con Volver sin iniciarla de nuevo.
// Con el menú o IrA ya se llama a Iniciar, y con CambioEspacio también: entonces no hace falta Mostrar.
type Ocultable interface {
	Ocultar()
	Mostrar() tea.Cmd
}

// Pendiente la implementa una página que puede tener algo que el usuario perdería de vista al salir de la terminal (una
// petición en marcha, una API con cambios sin guardar). Pendiente lo dice en una frase; "" si no hay nada. Con alguna
// frase, q pregunta antes de salir.
type Pendiente interface {
	Pendiente() string
}

// Tecla es una entrada del pie: la tecla y lo que hace.
type Tecla struct{ Tecla, Accion string }

// IrA lleva a otra página; Datos, si no es nil, le llega a su Update después de Iniciar.
// Por ejemplo IrA{"ejecutar", ConPunto{"mi-app"}} o IrA{"historial", FiltroPunto{"mi-app"}}.
type IrA struct {
	Pagina string
	Datos  any
}

type (
	ConPunto    struct{ Nombre string } // Probar con una API
	ConAgente   struct{ Agente string } // Probar con un agente
	FiltroPunto struct{ Nombre string } // el historial de una API
	// FiltroEstado abre el Historial con un filtro de estado del servicio (nucleo.Filtros: en_curso, completado o
	// con_error); "" o cualquier otro valor deja todas.
	FiltroEstado struct{ Estado string }
	// ConAyuda abre la Ayuda en el tema de una página (lo que devuelve su Ayuda()), con todas las teclas que valen en ella:
	// las suyas y las comunes, también las que no cupieron en el pie.
	ConAyuda struct {
		Pagina string
		Teclas []Tecla
	}
	// ConNovedades abre Ajustes en las novedades de la versión nueva, para instalarla: la u del aviso de la cabecera.
	ConNovedades struct{}
	// SeguirTrabajo abre Probar con ese trabajo (su receta y su mensaje) y lo sigue en vivo, como trabajoSeguido de la
	// Web UI: así llegan a Probar el reintento y la corrección del Historial.
	SeguirTrabajo struct{ ID string }
	// CambioEspacio cambia el espacio activo. La app cancela los Ctx de todas las páginas, se lo pasa a todas y vuelve
	// a iniciar la actual; las demás se inician con el espacio nuevo al visitarlas.
	CambioEspacio struct{ ID string }
	// EspaciosCambiados dice a la app que una página cambió espacios en el servicio (activó, detuvo, confió…): la app
	// relee la lista al momento, para la cabecera y Ctrl+E, como api.sesion() en la Web UI.
	EspaciosCambiados struct{}
	// ErrorServicio dice a la app que el servicio no responde: avisa y, con r, reintenta. La página no se cierra.
	ErrorServicio struct{ Err error }
	// Volver lleva a la página anterior: de la Ayuda a la de antes, o tras un IrA a la de origen. Sin anterior, nada.
	Volver struct{}
)

// Origen es lo que lleva cada respuesta de una página para saber si aún vale (las respuestas viejas se descartan): el Ctx y
// el espacio con que se pidió y la versión de la página (cada Iniciar, Ocultar o Mostrar la cambia). Cada página lo envuelve
// en su propio tipo, type origen struct{ pagina.Origen }, para que sus mensajes no se confundan con los de otra.
type Origen struct {
	Ctx       context.Context
	Instancia string
	Gen       int
}

// Origen es el de una petición que se hace ahora, en la versión gen de la página.
func (c Contexto) Origen(gen int) Origen { return Origen{Ctx: c.Ctx, Instancia: c.Instancia, Gen: gen} }

// DelEspacio dice si la respuesta es del espacio de c y su Ctx sigue vivo, aunque la página haya cambiado de versión:
// vale para lo que ya pasó en el servicio (una petición creada o cancelada).
func (o Origen) DelEspacio(c Contexto) bool {
	return o.Ctx != nil && o.Ctx.Err() == nil && o.Instancia == c.Instancia
}

// Vigente dice si la respuesta sigue valiendo: es DelEspacio y de la versión gen de la página.
func (o Origen) Vigente(c Contexto, gen int) bool { return o.DelEspacio(c) && o.Gen == gen }

// Fin se cierra cuando se cancela el Ctx con que se pidió.
func (o Origen) Fin() <-chan struct{} { return o.Ctx.Done() }

// Tras es un temporizador de una página: pasado d (con espera, que las pruebas sustituyen) responde mensaje(o), con el
// origen de la página, salvo que su Ctx se cancele antes.
func Tras[O interface{ Fin() <-chan struct{} }](o O, d time.Duration, espera func(time.Duration) <-chan time.Time, mensaje func(O) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-o.Fin():
			return nil
		case <-espera(d):
			return mensaje(o)
		}
	}
}

// SinCliente es el Cmd de lo que necesita el servicio cuando la página no tiene cliente: la app avisa y r reintenta.
func SinCliente() tea.Msg { return ErrorServicio{Err: cliente.ErrServicioInactivo} }

// SinServicio es el Cmd de ErrorServicio si err es de conexión (NoResponde); nil si es otro error, que la página enseña
// con su mensaje.
func SinServicio(err error) tea.Cmd {
	if !NoResponde(err) {
		return nil
	}
	return func() tea.Msg { return ErrorServicio{Err: err} }
}

// Atras es el Cmd de Volver, para el Esc de una página que no tiene nada que cerrar.
func Atras() tea.Msg { return Volver{} }

// NoResponde dice si err es de conexión (el servicio no está en marcha o no contesta) y no una respuesta de su API,
// que trae su propio mensaje, ni una cancelación (la app canceló el Ctx: la respuesta ya no interesa). Con NoResponde,
// la página devuelve ErrorServicio.
func NoResponde(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var u *url.Error
	return errors.Is(err, cliente.ErrServicioInactivo) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &u)
}
