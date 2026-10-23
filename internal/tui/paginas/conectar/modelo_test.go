package conectar_test

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/cliente"
	"tramtai/internal/contrato"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/paginas/prueba"
)

// TestModeloEscrito: además de elegirlo del catálogo, el modelo se puede escribir a mano, como opcionEscrita de la Web UI,
// con un agente que el catálogo conoce o un perfil. La pista de los permitidos es la de la Web UI.
func TestModeloEscrito(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	a.Entregar(tea.WindowSizeMsg{Width: 90, Height: 50})
	a.Pulsar("n")
	a.Escribir("mi-app")
	a.Pulsar("tab", "tab", "tab", "tab") // grupo, instrucciones, modelo y escrito a mano
	a.Escribir("nadie/x")
	a.Pulsar("ctrl+s")
	if v := a.Vista(); !strings.Contains(v, "Escribe agente/modelo con un agente conocido: simulado") {
		t.Fatalf("un agente que el catálogo no conoce no vale:\n%s", v)
	}
	for range 7 {
		a.Pulsar("backspace")
	}
	a.Escribir("perfil/profundo")
	a.Pulsar("ctrl+s")
	a.HastaVer("API lista")
	if p, _ := punto(t, cli, id, "mi-app"); p.PerfilModelo != "profundo" {
		t.Fatalf("el escrito a mano gana a la opción: %+v", p)
	}
	a.Pulsar("esc")
	a.Entregar(comp.Confirmado{})
	a.Pulsar("e")
	if v := a.Vista(); !strings.Contains(v, "Vacío: cualquiera.") {
		t.Fatalf("los permitidos llevan la pista de la Web UI:\n%s", v)
	}
}

// sinCatalogo es el cliente con un proxy delante que deja pasar todo salvo el catálogo de modelos, que falla.
func sinCatalogo(t *testing.T, cli *cliente.Cliente) *cliente.Cliente {
	t.Helper()
	destino, err := url.Parse(cli.Base)
	if err != nil {
		t.Fatal(err)
	}
	pasar := httputil.NewSingleHostReverseProxy(destino)
	base := prueba.Servidor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/modelos" {
			pasar.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"codigo": "solicitud_invalida", "mensaje": "el catálogo no se pudo leer"}})
	}))
	copia := *cli
	copia.Base = base
	return &copia
}

// TestSinCatalogo: si el catálogo no llega, el modelo vuelve a ser texto libre y se avisa del error, sin tragárselo.
func TestSinCatalogo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, sinCatalogo(t, cli), id)
	av := hastaAviso(a, func() { a.Pulsar("n") })
	if av.Tipo != comp.Advertencia || !strings.Contains(av.Texto, "el catálogo no se pudo leer") {
		t.Fatalf("avisa del catálogo: %+v", av)
	}
	a.Escribir("mi-app")
	a.Pulsar("tab", "tab", "tab") // grupo, instrucciones y modelo
	if v := a.Vista(); strings.Contains(v, "‹ Predeterminado") {
		t.Fatalf("sin catálogo no hay opciones:\n%s", v)
	}
	a.Escribir("perfil/rapido")
	a.Pulsar("ctrl+s")
	a.HastaVer("API lista")
	if p, _ := punto(t, cli, id, "mi-app"); p.PerfilModelo != "rapido" {
		t.Fatalf("el modelo escrito se guarda: %+v", p)
	}
}

// TestParcialNoPisaLaCompleta: si una acción vuelve mientras la carga completa está en camino, su recarga también es
// completa: la completa vieja se descarta, pero la página no se queda sin catálogo ni clave del espacio.
func TestParcialNoPisaLaCompleta(t *testing.T) {
	cli, id := prueba.Servicio(t)
	crear(t, cli, id, contrato.ConfigPunto{Nombre: "mi-app"})
	a := abierta(t, cli, id)
	accion := a.Retener(func() { a.Pulsar("a") })
	completa := a.Retener(func() { a.Correr(a.P.Iniciar(contexto(t, cli, id))) })
	a.Soltar(accion) // la acción vuelve y recarga antes de que llegue la completa
	a.HastaVer("mi-app               ○ Detenida")
	a.Soltar(completa) // ya es vieja
	a.Entregar(tea.WindowSizeMsg{Width: 70, Height: 60})
	a.Pulsar("right")
	clave, err := cli.ClaveInstancia(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	a.HastaVer(clave[:6]+"•••", "perfil/rapido")
}
