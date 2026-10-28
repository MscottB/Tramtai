package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/servicio"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/paginas/prueba"
)

// lineaEstado es la línea encima del pie, sin colores.
func lineaEstado(m *modelo) string {
	lineas := strings.Split(ansi.Strip(m.View().Content), "\n")
	return lineas[len(lineas)-2]
}

// TestMarcaEntorno: la línea de estado dice siempre en qué Tramtai se está («Servidor», «Este equipo» o «Portable»),
// también al ancho mínimo; debajo de los avisos que pasan quedan los fijos: en el servidor, la suscripción personal, y en
// el del usuario, el otro Tramtai del sistema.
func TestMarcaEntorno(t *testing.T) {
	for entorno, clave := range map[string]string{"servidor": "entorno.servidor", "escritorio": "entorno.escritorio", "portable": "entorno.portable", "": "entorno.escritorio"} {
		marca := datos.Texto("es", clave)
		if marca == clave {
			t.Fatal("faltan los textos en internal/tui/datos: cd web; npm run ayuda-terminal")
		}
		m := nuevoEn(Opciones{Datos: t.TempDir(), Entorno: entorno}, "es")
		m.Update(tea.WindowSizeMsg{Width: anchoMinimo, Height: altoMinimo})
		if l := lineaEstado(m); !strings.HasSuffix(strings.TrimRight(l, " "), marca) {
			t.Errorf("%q: la línea de estado acaba con %q: %q", entorno, marca, l)
		}
	}

	otro := idioma.Tf("Hay otro Tramtai, del sistema, en el puerto %d: es otro entorno, con sus propias APIs y claves", 17374)
	m := nuevoEn(Opciones{Datos: t.TempDir(), Entorno: "escritorio", AvisoCruzado: otro}, "es")
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	if l := lineaEstado(m); !strings.Contains(l, otro) || !strings.Contains(l, datos.Texto("es", "entorno.escritorio")) {
		t.Errorf("el aviso cruzado queda fijo en la línea de estado: %q", l)
	}

	s := nuevoEn(Opciones{Datos: t.TempDir(), Entorno: "servidor"}, "es")
	s.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	s.servidor = &contrato.EstadoServidor{Suscripciones: []contrato.SuscripcionServidor{{Agente: "claude", Personal: true, Simultaneas: 1},
		{Agente: "codex", Simultaneas: 1}}}
	personal := datos.Textof("es", "servidor.avisoSuscripcion", "agente", "Claude")
	if l := lineaEstado(s); !strings.Contains(l, personal) || strings.Contains(l, "Codex") {
		t.Errorf("la suscripción personal (y solo ella) queda fija en la línea de estado: %q", l)
	}
}

// TestEstadoServidorAlConectar: la terminal del servidor lee el estado del servidor al conectar (el administrador, las
// suscripciones); la del usuario no lo pide.
func TestEstadoServidorAlConectar(t *testing.T) {
	cli, global := prueba.Ajustado(t, func(o *servicio.Opciones) {
		o.Entorno = "servidor"
		if err := servicio.FijarContrasena(o.Datos, "una contraseña de prueba"); err != nil {
			t.Fatal(err)
		}
	})
	m := nuevo(Opciones{Entorno: "servidor"})
	m.cli, m.instancia = cli, global
	correr(t, m, m.conectar(false, false, false))
	if m.servidor == nil || !m.servidor.Administrador {
		t.Fatalf("estado del servidor: %+v", m.servidor)
	}
	u := nuevo(Opciones{Entorno: "escritorio"})
	u.cli, u.instancia = cli, global
	correr(t, u, u.conectar(false, false, false))
	if u.servidor != nil {
		t.Fatalf("el Tramtai del usuario no lo pide: %+v", u.servidor)
	}
}

// TestTeclaWServidorConAdministrador: en el servidor con administrador, W no pide un código de un solo uso: basta la
// dirección pública, donde se entra con la contraseña. Sin navegador la enseña, sin código; con él, la abre.
func TestTeclaWServidorConAdministrador(t *testing.T) {
	for _, navegador := range []bool{false, true} {
		m, abiertas := conWeb(t, navegador)
		m.o.Entorno = "servidor"
		m.servidor = &contrato.EstadoServidor{Administrador: true}
		a := conducir(t, m)
		_, cmd := m.Update(tecla("W"))
		a.Correr(cmd)
		if navegador {
			if len(*abiertas) != 1 || (*abiertas)[0] != m.cli.BasePublica() {
				t.Fatalf("abre la dirección pública, sin código: %v", *abiertas)
			}
			continue
		}
		if len(*abiertas) != 0 || m.pregunta == nil || !m.pregunta.Leer {
			t.Fatalf("sin navegador enseña la dirección: %v %+v", *abiertas, m.pregunta)
		}
		v := ansi.Strip(m.View().Content)
		if !strings.Contains(v, m.cli.BasePublica()) || strings.Contains(v, "codigo=") || strings.Contains(v, idioma.T("Código")) ||
			m.pregunta.Texto != idioma.T("Abre esta dirección en un navegador y entra con la contraseña del administrador:")+"\n\n"+m.cli.BasePublica() {
			t.Fatalf("la ventana trae la dirección, sin código:\n%s", v)
		}
	}
}
