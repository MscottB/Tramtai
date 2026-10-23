package agentes

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	t.Cleanup(proxy.Close)
	copia := *cli
	copia.Base = proxy.URL
	return &copia, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), cuerpos...)
	}
}

// TestAgentesEnviaElModoCorrecto: pasar a la suscripción manda «suscripcion» solo tras confirmar, y pasar a la clave manda
// «clave_api» directo.
func TestAgentesEnviaElModoCorrecto(t *testing.T) {
	// abrir abre la página con Claude, en el modo dado, detrás de una grabadora de lo que se envía.
	abrir := func(modo string) (*Pagina, func() []string) {
		cli, id := prueba.ConAgentes(t, map[string]adaptadores.Adaptador{"claude": detectado(adaptadores.Estado{Autenticacion: modo})})
		proxy, cuerpos := grabadora(t, cli)
		p := Nueva("es").(*Pagina)
		correr(p, p.Iniciar(contexto(proxy, id)))
		return p, cuerpos
	}
	p, cuerpos := abrir("clave_api")
	_, cmd := p.Update(tecla("a"))
	if f := correr(p, cmd); len(f) != 1 {
		t.Fatalf("pregunta antes: %+v", f)
	}
	if n := len(cuerpos()); n != 0 {
		t.Fatalf("sin confirmar no se manda nada: %v", cuerpos())
	}
	_, cmd = p.Update(comp.Confirmado{})
	f := correr(p, cmd)
	if got := cuerpos(); len(got) != 1 || got[0] != `{"modo":"suscripcion"}` {
		t.Fatalf("tras confirmar manda suscripcion: %v", got)
	}
	if a, ok := aviso(f); !ok || a.Texto != "Claude usa ahora tu suscripción" {
		t.Fatalf("avisa: %+v", f)
	}

	p, cuerpos = abrir("suscripcion")
	_, cmd = p.Update(tecla("a"))
	f = correr(p, cmd)
	if got := cuerpos(); len(got) != 1 || got[0] != `{"modo":"clave_api"}` {
		t.Fatalf("pasar a la clave manda clave_api: %v", got)
	}
	if a, ok := aviso(f); !ok || a.Texto != "Claude usa ahora la clave de API" {
		t.Fatalf("avisa: %+v", f)
	}
}

// TestAgentesDesconectadoSinCLIEsNoInstalado: como en la Web UI, va al final, cuenta como no instalado, explica cómo
// instalarlo y no se puede desconectar ni conectar.
func TestAgentesDesconectadoSinCLIEsNoInstalado(t *testing.T) {
	p, cli, id := conAgentes(t, map[string]adaptadores.Adaptador{"codex": sinCLI})
	if err := cli.ConectarAgente(context.Background(), "codex", false); err != nil {
		t.Fatal(err)
	}
	correr(p, p.Iniciar(contexto(cli, id)))
	v := p.Vista()
	if strings.Index(v, "simulado") > strings.Index(v, "Codex") {
		t.Fatalf("va al final:\n%s", v)
	}
	if !strings.Contains(v, "Listos (1) Falta conectar (0) No instalados (1)") {
		t.Fatalf("cuenta como no instalado:\n%s", v)
	}
	p.Update(tecla("down"))
	_, cmd := p.Update(tecla("d"))
	f := correr(p, cmd)
	if _, ok := aviso(f); !ok || len(f) != 1 {
		t.Fatalf("d no desconecta ni conecta un no instalado: %+v", f)
	}
	if _, cmd := p.Update(comp.Confirmado{}); cmd != nil {
		t.Fatal("no queda nada pendiente")
	}
	p.Update(tecla("enter"))
	if v := p.Vista(); !strings.Contains(v, "comandos_agentes.codex") || strings.Contains(v, "Desconectado:") {
		t.Fatalf("el detalle explica cómo instalarlo:\n%s", v)
	}
}

// TestAgentesCambioDeEspacioVaciaElCatalogo: el catálogo de modelos de un espacio no se ve en el siguiente.
func TestAgentesCambioDeEspacioVaciaElCatalogo(t *testing.T) {
	p, cli, id := abierta(t)
	_, cmd := p.Update(tecla("enter"))
	correr(p, cmd)
	if v := p.Vista(); !strings.Contains(v, "Modelos: predeterminado") {
		t.Fatalf("antes se ve el catálogo:\n%s", v)
	}
	correr(p, p.Iniciar(contexto(cli, id))) // la app vuelve a iniciar la página (otro espacio o el mismo)
	p.Update(tecla("enter"))                // el catálogo se pide de nuevo; la petición aún no ha vuelto
	if v := p.Vista(); !strings.Contains(v, "Modelos: Cargando…") {
		t.Fatalf("Iniciar vacía el catálogo: no se ve el de antes:\n%s", v)
	}
}

// TestAgentesTextosComoLaWeb: los nombres de estado, el resumen y los detalles salen de los textos de la Web UI, en el
// idioma de la sesión (aquí inglés, con el idioma del proceso en español).
func TestAgentesTextosComoLaWeb(t *testing.T) {
	cli, id := prueba.Servicio(t)
	c := contexto(cli, id)
	c.Idioma = "en"
	p := Nueva("en").(*Pagina)
	correr(p, p.Iniciar(c))
	v := p.Vista()
	for _, quiere := range []string{"Ready", "Ready (1)", "To connect (0)", "Not installed (0)", "API key"} {
		if !strings.Contains(v, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, v)
		}
	}
	p.Update(tecla("enter"))
	d := p.Vista()
	for _, quiere := range []string{"Ready · uses your API key", "Version: not detected", "Authentication: API key", "Live response: When finished",
		"JSON response with schema: No", "Reports the model used: No", "Adjustable effort: No"} {
		if !strings.Contains(d, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, d)
		}
	}
}
