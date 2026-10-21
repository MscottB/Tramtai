// Package pagina es lo que comparten la terminal y sus páginas: la interfaz de una página, lo que recibe al iniciarse,
// los mensajes con los que habla con la app y los estilos. Así las páginas no importan internal/tui.
package pagina

import (
	"context"
	"errors"
	"net/url"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
)

// Contexto es lo que una página recibe en Iniciar. La app lo vuelve a dar al entrar en la página, al cambiar de espacio
// y al reintentar con el servicio caído.
type Contexto struct {
	Cli         *cliente.Cliente // nil si el servicio no está en marcha: la página devuelve ErrorServicio
	Instancia   string           // id del espacio activo
	Idioma      string           // "es" | "en"
	Estilos     *Estilos         // los mismos durante toda la sesión: cambian por dentro si cambia el fondo
	Ancho, Alto int              // el área de la página, sin cabecera, menú ni pie
	Proyecto    string           // raíz del proyecto desde el que se abrió la terminal; vacía fuera de uno
}

// Pagina es una página de la terminal. La app le pasa las teclas solo cuando es la actual; el resto de mensajes (las
// respuestas de sus Cmd, por ejemplo) le llegan aunque no lo sea. Al cambiar el tamaño recibe un tea.WindowSizeMsg con
// el de su área, no el de la ventana.
type Pagina interface {
	Titulo() string                   // el nombre del menú; se puede llamar antes de Iniciar
	Iniciar(Contexto) tea.Cmd         // carga sus datos
	Update(tea.Msg) (Pagina, tea.Cmd) // tea.Model sin Init ni View
	Vista() string                    // como mucho Ancho×Alto; lo que sobre se corta
	Teclas() []Tecla                  // para el pie, además de las comunes
	EnCampo() bool                    // un campo de texto tiene el foco: las teclas comunes no se aplican
	Ayuda() string                    // la página de la Ayuda (el campo pagina de su tema): inicio, ejecutar…
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
	ConAyuda    struct{ Pagina string } // la Ayuda en el tema de una página (lo que devuelve su Ayuda())
	// CambioEspacio cambia el espacio activo; la app vuelve a iniciar la página actual con él.
	CambioEspacio struct{ ID string }
	// ErrorServicio dice a la app que el servicio no responde: avisa y, con r, reintenta. La página no se cierra.
	ErrorServicio struct{ Err error }
)

// NoResponde dice si err es de conexión (el servicio no está en marcha o no contesta) y no una respuesta de su API,
// que trae su propio mensaje. Con NoResponde, la página devuelve ErrorServicio.
func NoResponde(err error) bool {
	var u *url.Error
	return errors.Is(err, cliente.ErrServicioInactivo) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &u)
}
