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

// TestTeclaWServidorConAdministrador (spec §8): con navegador en el equipo, W hace lo de siempre (pide el código y abre
// la Web UI ya dentro). Sin navegador, si el servidor tiene administrador y se puede entrar con contraseña (TLS propio o
// un proxy de confianza), basta la dirección pública, sin código; si no hay HTTPS, la dirección con el código de siempre.
func TestTeclaWServidorConAdministrador(t *testing.T) {
	for _, c := range []struct {
		nombre    string
		navegador bool
		estado    contrato.EstadoServidor
		sinCodigo bool
	}{
		{"con navegador", true, contrato.EstadoServidor{Administrador: true, TLS: "archivos"}, false},
		{"sin navegador y con TLS propio", false, contrato.EstadoServidor{Administrador: true, TLS: "archivos"}, true},
		{"sin navegador y tras un proxy", false, contrato.EstadoServidor{Administrador: true, TLS: "ninguno", Confiables: []string{"10.0.0.0/8"}}, true},
		{"sin navegador y sin HTTPS", false, contrato.EstadoServidor{Administrador: true, TLS: "ninguno"}, false},
	} {
		m, abiertas := conWeb(t, c.navegador)
		m.o.Entorno = "servidor"
		m.servidor = &c.estado
		a := conducir(t, m)
		_, cmd := m.Update(tecla("W"))
		a.Correr(cmd)
		if c.navegador {
			if len(*abiertas) != 1 || !strings.Contains((*abiertas)[0], "/ui/entrar?codigo=") || m.pregunta != nil {
				t.Errorf("%s: abre la Web UI con el código: %v", c.nombre, *abiertas)
			}
			continue
		}
		if len(*abiertas) != 0 || m.pregunta == nil || !m.pregunta.Leer {
			t.Fatalf("%s: enseña la dirección: %v %+v", c.nombre, *abiertas, m.pregunta)
		}
		conContrasena := idioma.T("Abre esta dirección en un navegador y entra con la contraseña del administrador:") + "\n\n" + m.cli.BasePublica()
		if c.sinCodigo && m.pregunta.Texto != conContrasena {
			t.Errorf("%s: solo la dirección pública: %q", c.nombre, m.pregunta.Texto)
		}
		if !c.sinCodigo && (!strings.Contains(m.pregunta.Texto, "/ui/entrar?codigo=") || !strings.Contains(m.pregunta.Texto, idioma.T("Código"))) {
			t.Errorf("%s: la dirección con el código: %q", c.nombre, m.pregunta.Texto)
		}
	}
}
