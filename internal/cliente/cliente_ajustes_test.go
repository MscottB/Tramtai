package cliente

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"tramtai/internal/nucleo"
	"tramtai/internal/version"
)

// Los ajustes del servicio que usa la terminal, contra el servicio real: el puerto, las actualizaciones (sin ir a GitHub:
// la búsqueda está apagada), el avance, la decisión de instalar fuera de la cuenta atrás y Acerca de.
func TestClienteAjustesDelServicio(t *testing.T) {
	cli, _ := servicioDePrueba(t, "")
	ctx := context.Background()
	a, err := cli.AjustesServicio(ctx)
	if err != nil || a.Puerto == 0 || a.PuertoConfigurado != a.Puerto {
		t.Fatalf("ajustes: %+v %v", a, err)
	}
	e, err := cli.Actualizacion(ctx, false)
	if err != nil || e.Actual != version.Version || e.Buscar || e.Disponible {
		t.Fatalf("actualización: %+v %v", e, err)
	}
	if e, err = cli.BuscarActualizaciones(ctx, true); err != nil || !e.Buscar {
		t.Fatalf("activar la búsqueda: %+v %v", e, err)
	}
	if e, err = cli.BuscarActualizaciones(ctx, false); err != nil || e.Buscar {
		t.Fatalf("desactivar la búsqueda: %+v %v", e, err)
	}
	if p, err := cli.ProgresoActualizacion(ctx); err != nil || p.Fase != "" {
		t.Fatalf("progreso: %+v %v", p, err)
	}
	if err := cli.DecidirInstalacion(ctx, true); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("instalar ya sin cuenta atrás: %v", err)
	}
	if v, err := cli.Novedades(ctx, "v0.1.0", ""); err != nil || len(v) != 0 {
		t.Fatalf("novedades sin búsqueda: %+v %v", v, err)
	}
	if ac, err := cli.Acerca(ctx); err != nil || ac.Version != version.Texto() || ac.Rutas == nil {
		t.Fatalf("acerca: %+v %v", ac, err)
	}
}

// Lo que cambia el equipo o descarga se comprueba contra un servidor que solo apunta lo que recibe: el arranque con el
// sistema, actualizar, buscar ahora y las novedades hasta una versión.
func TestClienteActualizarYArranque(t *testing.T) {
	var recibido []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cuerpo, _ := io.ReadAll(r.Body)
		recibido = append(recibido, r.Method+" "+r.URL.RequestURI()+" "+string(cuerpo))
		switch r.URL.Path {
		case "/api/v1/servicio/actualizar":
			json.NewEncoder(w).Encode(map[string]string{"version": "v9.9.9"})
		case "/api/v1/servicio/actualizacion":
			json.NewEncoder(w).Encode(map[string]any{"disponible": true, "version": "v9.9.9"})
		case "/api/v1/servicio/actualizacion/novedades":
			json.NewEncoder(w).Encode(map[string]any{"versiones": []map[string]string{{"version": "v9.9.9", "notas": "x"}}})
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	c := &Cliente{Base: srv.URL, token: "x", http: srv.Client()}
	ctx := context.Background()
	if err := c.Autoarranque(ctx, true); err != nil {
		t.Fatal(err)
	}
	if v, err := c.Actualizar(ctx); err != nil || v != "v9.9.9" {
		t.Fatalf("actualizar: %q %v", v, err)
	}
	if e, err := c.Actualizacion(ctx, true); err != nil || !e.Disponible {
		t.Fatalf("buscar ahora: %+v %v", e, err)
	}
	if v, err := c.Novedades(ctx, "v1.0.0", "v9.9.9"); err != nil || len(v) != 1 || v[0].Version != "v9.9.9" {
		t.Fatalf("novedades: %+v %v", v, err)
	}
	if err := c.DecidirInstalacion(ctx, false); err != nil {
		t.Fatal(err)
	}
	esperado := []string{
		`PUT /api/v1/servicio/autoarranque {"activo":true}`,
		"POST /api/v1/servicio/actualizar ",
		"GET /api/v1/servicio/actualizacion?ahora=1 ",
		"GET /api/v1/servicio/actualizacion/novedades?desde=v1.0.0&hasta=v9.9.9 ",
		"POST /api/v1/servicio/actualizar/cancelar ",
	}
	if len(recibido) != len(esperado) {
		t.Fatalf("recibido: %q", recibido)
	}
	for i := range esperado {
		if recibido[i] != esperado[i] {
			t.Fatalf("petición %d: %q, esperada %q", i, recibido[i], esperado[i])
		}
	}
}
