package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/datos"
	"tramtai/internal/tui/paginas/prueba"
)

// tardon es un agente que tarda en decir cómo está: mientras tanto Inicio dice «Comprobando…».
type tardon struct {
	adaptadores.Simulado
	tarda time.Duration
}

func (a tardon) Detectar(ctx context.Context) adaptadores.Estado {
	select {
	case <-time.After(a.tarda):
	case <-ctx.Done():
	}
	return adaptadores.Estado{}
}

// TestRedEnMovimiento: Captura no da por asentada la red de Inicio con peticiones en curso ni con filas ya terminadas que
// se van, en los dos idiomas, pero no confunde el contador «En curso», las filas en ejecución ni el Historial.
func TestRedEnMovimiento(t *testing.T) {
	for lengua, c := range map[string]struct{ si, no []string }{
		"es": {[]string{"Tu red · 2 peticiones en curso.", "Tu red · 15 peticiones en curso.", "Tu red · 1 petición en curso.",
			"✓ Clave del espacio  Claude         terminada     9 s", "✗ api-a  Claude  falló  3 s", "○ api-a  Claude  cancelada  1 s"},
			[]string{"● e En curso     en cola o ejecutándose ahora   0", "● api-a  Claude  en ejecución  0 s", "Tu red",
				"▸ ✓ Completado   Di «hola» en tres idiomas.   hace 25 s", "terminada la prueba"}},
		"en": {[]string{"Your network · 2 requests in progress.", "Your network · 1 request in progress.",
			"✓ Workspace key  Claude  finished  9 s", "✗ api-a  Claude  failed  3 s", "○ api-a  Claude  canceled  1 s"},
			[]string{"● e In progress   queued or running now   0", "Your network", "▸ ✓ Completed    Di «hola»   34 s ago"}},
	} {
		re := redEnMovimiento(lengua)
		for _, s := range c.si {
			if !re.MatchString(s) {
				t.Errorf("%s: debería reconocer %q", lengua, s)
			}
		}
		for _, s := range c.no {
			if re.MatchString(s) {
				t.Errorf("%s: no debería reconocer %q", lengua, s)
			}
		}
	}
}

// TestCapturaEsperaALaRedQuieta: al abrir Inicio, el flujo en vivo trae las peticiones recientes como filas que se van a
// los 2 s; Captura espera a que se vayan y devuelve la red vacía, igual en los dos idiomas.
func TestCapturaEsperaALaRedQuieta(t *testing.T) {
	cli, id := prueba.Servicio(t)
	j, err := cli.Crear(context.Background(), id, nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}})
	if err == nil {
		err = cli.Eventos(context.Background(), id, j.ID, 0, func(int, string, string) error { return nil })
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, lengua := range []string{"es", "en"} {
		salida, err := Captura(Opciones{Datos: t.TempDir(), Entorno: "escritorio"}, cli, lengua, true, 100, 28, nil, 15*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		plano := secuenciaANSI.ReplaceAllString(salida, "")
		if !strings.Contains(plano, datos.Texto(lengua, "inicio.red")) {
			t.Fatalf("%s: no es Inicio:\n%s", lengua, plano)
		}
		if redEnMovimiento(lengua).MatchString(plano) {
			t.Errorf("%s: la captura enseña la red en movimiento:\n%s", lengua, plano)
		}
		if !strings.Contains(plano, string([]rune(datos.Texto(lengua, "inicio.redTexto"))[:20])) {
			t.Errorf("%s: la red no está vacía:\n%s", lengua, plano)
		}
	}
}

// TestCapturaEsperaComprobando: Inicio dice «Comprobando…» mientras el servicio comprueba los agentes, y esa vista no
// cambia durante más de medio segundo; Captura no la da por asentada, espera a que llegue la comprobación.
func TestCapturaEsperaComprobando(t *testing.T) {
	cli, _ := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"tardon": tardon{tarda: 2 * time.Second}})
	salida, err := Captura(Opciones{Datos: t.TempDir(), Entorno: "escritorio"}, cli, "es", true, 100, 28, nil, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(salida, datos.Texto("es", "inicio.primerosPasos")) {
		t.Fatalf("no es Inicio:\n%s", salida)
	}
	if strings.Contains(salida, datos.Texto("es", "inicio.comprobando")) {
		t.Errorf("la captura se hizo antes de que acabara la comprobación:\n%s", salida)
	}
}

// TestCapturaPintaLaPaginaConColor: Captura (las capturas de la terminal de la portada) pinta la terminal entera con el
// servicio, en el tema pedido y con sus colores, y lleva a la página de las teclas.
func TestCapturaPintaLaPaginaConColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	cli, _ := prueba.Servicio(t)
	salida, err := Captura(Opciones{Datos: t.TempDir(), Entorno: "escritorio"}, cli, "es", false, 100, 30, []string{"4"}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(salida, "\x1b[") {
		t.Errorf("sin estilos ANSI:\n%s", salida)
	}
	// El subtítulo de Mis APIs sale recortado a lo ancho: basta con su principio.
	for _, quiero := range []string{datos.Texto("es", "entorno.escritorio"), string([]rune(datos.Texto("es", "conectar.subtitulo"))[:40])} {
		if !strings.Contains(salida, quiero) {
			t.Errorf("falta %q:\n%s", quiero, salida)
		}
	}
	if strings.Contains(salida, datos.Texto("es", "comun.cargando")) {
		t.Errorf("la captura se hizo antes de que llegaran los datos:\n%s", salida)
	}
}
