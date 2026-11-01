package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
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
