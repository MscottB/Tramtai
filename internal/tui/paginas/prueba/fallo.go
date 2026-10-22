package prueba

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"testing"

	"tramtai/internal/cliente"
)

// ConFallo pone delante del servicio un servidor que lo deja pasar todo hasta que la prueba le da un mensaje: desde
// entonces contesta cada petición con ese error de la API (400, solicitud_invalida). Así una página enseña un error del
// servicio tal cual, con el texto que haga falta. Devuelve el cliente que le habla y la función que lo hace fallar.
func ConFallo(t *testing.T, cli *cliente.Cliente) (*cliente.Cliente, func(mensaje string)) {
	t.Helper()
	destino, err := url.Parse(cli.Base)
	if err != nil {
		t.Fatal(err)
	}
	pasar := httputil.NewSingleHostReverseProxy(destino)
	var mu sync.Mutex
	var fallo string
	base := Servidor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		mensaje := fallo
		mu.Unlock()
		if mensaje == "" {
			pasar.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"codigo": "solicitud_invalida", "mensaje": mensaje}})
	}))
	copia := *cli
	copia.Base = base
	return &copia, func(mensaje string) {
		mu.Lock()
		defer mu.Unlock()
		fallo = mensaje
	}
}
