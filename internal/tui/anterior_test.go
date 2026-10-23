package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/tui/pagina"
)

// servicioAnterior simula un servicio 0.55 con la forma exacta de su /healthz (sin reto ni prueba), como el de
// internal/cliente, y devuelve su carpeta de datos.
func servicioAnterior(t *testing.T) string {
	datos := t.TempDir()
	const id = "servicio_anterior"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"estado":"ok","servicio":%q,"version":"0.55.0+21102026"}`+"\n", id)
	}))
	t.Cleanup(ts.Close)
	d, _ := json.Marshal(contrato.Descriptor{Identificador: id, PID: 4242, Base: ts.URL})
	if err := os.WriteFile(contrato.RutaDescriptor(datos), d, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(datos, "token-servicio"), []byte(strings.Repeat("s", 43)), 0o600); err != nil {
		t.Fatal(err)
	}
	return datos
}

// Con el servicio de la v0.55 en marcha, la terminal no dice solo «no responde»: la línea de estado dice que r lo
// detiene y, si no se puede (no es de este usuario), que ! dice cómo detenerlo a mano; ! enseña el mensaje entero.
func TestServicioAnteriorSeExplica(t *testing.T) {
	datos := servicioAnterior(t)
	aMano := fmt.Errorf("%w (PID 4242): %s", cliente.ErrServicioAnterior,
		"detenlo con «tramtai servicio detener» de esa versión o termina ese proceso, y vuelve a intentarlo")
	m := nuevo(Opciones{Datos: datos, Iniciar: func() (*cliente.Cliente, error) { return nil, aMano }})
	m.paginas["inicio"] = &paginaFalsa{}
	a := conducir(t, m)
	m.conectando, m.comprobando = true, true
	a.Correr(m.conectar(false, true, false))

	vista := m.View().Content
	if !strings.Contains(vista, idioma.T("Sigue en marcha el servicio de una versión anterior · r lo detiene y arranca el nuevo")) ||
		strings.Contains(vista, idioma.T("El servicio no responde · r reintenta")) {
		t.Fatalf("la línea de estado dice que es el servicio anterior y que r lo detiene:\n%s", vista)
	}
	m.Update(tecla("!"))
	if m.pregunta == nil || !strings.Contains(m.pregunta.Texto, cliente.ErrServicioAnterior.Error()) {
		t.Fatalf("! enseña el error entero: %+v", m.pregunta)
	}
	m.Update(tecla("esc"))

	_, cmd := m.Update(tecla("r"))
	if cmd == nil {
		t.Fatal("r intenta detenerlo")
	}
	a.Correr(cmd)
	// Una página que pide sin cliente no tapa la explicación con un «no responde» genérico.
	m.Update(pagina.ErrorServicio{Err: cliente.ErrServicioInactivo})
	if vista := m.View().Content; !strings.Contains(vista, idioma.T("No se pudo detener el servicio de la versión anterior · ! dice cómo")) {
		t.Fatalf("la línea de estado dice que ! explica cómo detenerlo:\n%s", vista)
	}
	if pie := m.vistaPie(); !strings.Contains(pie, "!") {
		t.Fatalf("el pie ofrece !: %q", pie)
	}
	m.Update(tecla("!"))
	if m.pregunta == nil || !strings.Contains(m.pregunta.Texto, "tramtai servicio detener") || !strings.Contains(m.pregunta.Texto, "4242") {
		t.Fatalf("! dice cómo detenerlo a mano: %+v", m.pregunta)
	}
}
