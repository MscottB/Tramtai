package agentes_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/cliente"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/paginas/prueba"
)

// grabadora pone un servidor delante del servicio de prueba que anota el cuerpo de cada PUT de autenticación y lo
// contesta sin pasarlo (el servicio de pruebas no tiene Claude); lo demás lo deja pasar. Devuelve el cliente que le habla.
func grabadora(t *testing.T, cli *cliente.Cliente) (*cliente.Cliente, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var cuerpos []string
	real, err := http.NewRequest("GET", cli.Base, nil)
	if err != nil {
		t.Fatal(err)
	}
	url := prueba.Servidor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && r.URL.Path == "/api/v1/servicio/agentes/claude/autenticacion" {
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			cuerpos = append(cuerpos, strings.TrimSpace(string(b)))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{}"))
			return
		}
		r.URL.Scheme, r.URL.Host, r.Host = real.URL.Scheme, real.URL.Host, real.URL.Host
		r.RequestURI = ""
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	copia := *cli
	copia.Base = url
	return &copia, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), cuerpos...)
	}
}

// TestAgentesEnviaElModoCorrecto: pasar a la suscripción manda «suscripcion» solo tras confirmar, y pasar a la clave manda
// «clave_api» directo.
func TestAgentesEnviaElModoCorrecto(t *testing.T) {
	// conGrabadora abre la página con Claude, en el modo dado, detrás de una grabadora de lo que se envía.
	conGrabadora := func(modo string) (*prueba.App, func() []string) {
		cli, id := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: modo})})
		proxy, cuerpos := grabadora(t, cli)
		return abrir(t, contexto(t, proxy, id)), cuerpos
	}
	a, cuerpos := conGrabadora("clave_api")
	if f := a.Pulsar("a"); len(f) != 1 {
		t.Fatalf("pregunta antes: %+v", f)
	}
	if n := len(cuerpos()); n != 0 {
		t.Fatalf("sin confirmar no se manda nada: %v", cuerpos())
	}
	f := a.HastaSalir(func() { a.Entregar(comp.Confirmado{}) }, prueba.HayAviso)
	if got := cuerpos(); len(got) != 1 || got[0] != `{"modo":"suscripcion"}` {
		t.Fatalf("tras confirmar manda suscripcion: %v", got)
	}
	if av, ok := aviso(f); !ok || av.Texto != "Claude usa ahora tu suscripción" {
		t.Fatalf("avisa: %+v", f)
	}

	a, cuerpos = conGrabadora("suscripcion")
	f = a.HastaSalir(func() { a.Pulsar("a") }, prueba.HayAviso)
	if got := cuerpos(); len(got) != 1 || got[0] != `{"modo":"clave_api"}` {
		t.Fatalf("pasar a la clave manda clave_api: %v", got)
	}
	if av, ok := aviso(f); !ok || av.Texto != "Claude usa ahora la clave de API" {
		t.Fatalf("avisa: %+v", f)
	}
}

// TestAgentesDesconectadoSinCLIEsNoInstalado: como en la Web UI, va al final, cuenta como no instalado, explica cómo
// instalarlo y no se puede desconectar ni conectar.
func TestAgentesDesconectadoSinCLIEsNoInstalado(t *testing.T) {
	a, cli, _ := conAgentes(t, map[string]adaptadores.Adaptador{"codex": sinCLI})
	if err := cli.ConectarAgente(context.Background(), "codex", false); err != nil {
		t.Fatal(err)
	}
	a.Pulsar("c") // vuelve a comprobar
	a.HastaVer("Listos (1) Falta conectar (0) No instalados (1)")
	v := a.Vista()
	if strings.Index(v, "simulado") > strings.Index(v, "Codex") {
		t.Fatalf("va al final:\n%s", v)
	}
	if !strings.Contains(v, "Listos (1) Falta conectar (0) No instalados (1)") {
		t.Fatalf("cuenta como no instalado:\n%s", v)
	}
	a.Pulsar("down")
	f := a.Pulsar("d")
	if _, ok := aviso(f); !ok || len(f) != 1 {
		t.Fatalf("d no desconecta ni conecta un no instalado: %+v", f)
	}
	if f := a.Entregar(comp.Confirmado{}); len(f) != 0 {
		t.Fatalf("no queda nada pendiente: %v", f)
	}
	a.Pulsar("enter")
	if v := a.Vista(); !strings.Contains(v, "comandos_agentes.codex") || strings.Contains(v, "Desconectado:") {
		t.Fatalf("el detalle explica cómo instalarlo:\n%s", v)
	}
}

// TestAgentesCambioDeEspacioVaciaElCatalogo: el catálogo de modelos de un espacio no se ve en el siguiente.
func TestAgentesCambioDeEspacioVaciaElCatalogo(t *testing.T) {
	a, cli, id := abierta(t)
	a.Pulsar("enter")
	a.HastaVer("Modelos: predeterminado")
	if v := a.Vista(); !strings.Contains(v, "Modelos: predeterminado") {
		t.Fatalf("antes se ve el catálogo:\n%s", v)
	}
	a.Correr(a.P.Iniciar(contexto(t, cli, id))) // la app vuelve a iniciar la página (otro espacio o el mismo)
	a.EsperarCarga()                            // la lista, para que Intro tenga un agente que abrir
	a.Retener(func() { a.Pulsar("enter") })     // el catálogo se pide de nuevo; la petición aún no ha vuelto
	if v := a.Vista(); !strings.Contains(v, "Modelos: Cargando…") {
		t.Fatalf("Iniciar vacía el catálogo: no se ve el de antes:\n%s", v)
	}
}

// TestAgentesTextosComoLaWeb: los nombres de estado, el resumen y los detalles salen de los textos de la Web UI, en el
// idioma de la sesión (aquí inglés, con el idioma del proceso en español).
func TestAgentesTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	c := contexto(t, cli, id)
	c.Idioma = "en"
	a := abrir(t, c)
	v := a.Vista()
	for _, quiere := range []string{"Ready", "Ready (1)", "To connect (0)", "Not installed (0)", "API key"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	a.Pulsar("enter")
	d := a.Vista()
	for _, quiere := range []string{"Ready · uses your API key", "Version: not detected", "Authentication: API key", "Live response: When finished",
		"JSON response with schema: No", "Reports the model used: No", "Adjustable effort: No"} {
		if !strings.Contains(d, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, d)
		}
	}
}
