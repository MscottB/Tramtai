package servicio

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// abrirSesion entra en la Web UI con un código de un solo uso y deja el CSRF de la sesión.
func abrirSesion(t *testing.T, s *Servicio, base string) *navegador {
	t.Helper()
	n := nuevoNavegador(t, base)
	codigo, err := s.CodigoSesion("global")
	if err != nil {
		t.Fatal(err)
	}
	n.pedir("GET", "/ui/entrar?codigo="+codigo, "")
	_, cuerpo := n.pedir("GET", "/ui/sesion", "")
	var sesion struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &sesion); err != nil || sesion.CSRF == "" {
		t.Fatalf("sesión: %v %s", err, cuerpo)
	}
	n.csrf = sesion.CSRF
	return n
}

func TestPresupuestoPuntoUI(t *testing.T) {
	global := `puntos_acceso:
  - nombre: facturas
    contratos: [nativa]
    agente: simulado
    presupuesto:
      diario:
        peticiones: 1
`
	s, ts, _ := entorno(t, global, "", adaptadores.Simulado{})
	instancia, _ := s.ActivarGlobal()
	token, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "facturas"))
	s.Recargar()
	trabajo := func() int {
		r, _ := pedir(t, ts.URL, "POST", "/p/facturas/api/v1/trabajos", token, hola)
		return r.StatusCode
	}
	if c := trabajo(); c != 202 {
		t.Fatalf("primera petición: %d", c)
	}
	if c := trabajo(); c != 429 {
		t.Fatalf("la segunda agota el presupuesto diario: %d", c)
	}

	n := abrirSesion(t, s, ts.URL)
	_, cuerpo := n.pedir("GET", "/api/v1/puntos", "")
	var lista struct {
		Presupuestos map[string]nucleo.EstadoPresupuesto `json:"presupuestos"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &lista); err != nil || lista.Presupuestos["facturas"].Diario.Consumo.Peticiones != 1 ||
		lista.Presupuestos["facturas"].Diario.Limites.Peticiones != 1 {
		t.Fatalf("el listado muestra el consumo frente al límite: %v %s", err, cuerpo)
	}

	if r, _ := pedir(t, ts.URL, "POST", "/api/v1/puntos/facturas/presupuesto/reiniciar", instancia.token, ""); r.StatusCode != 401 {
		t.Fatalf("la clave de aplicaciones no reinicia presupuestos: %d", r.StatusCode)
	}
	if r, cuerpo := n.pedir("POST", "/api/v1/puntos/facturas/presupuesto/reiniciar", "", "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("reiniciar: %d %s", r.StatusCode, cuerpo)
	}
	if c := trabajo(); c != 202 {
		t.Fatalf("tras reiniciar vuelve a admitir: %d", c)
	}

	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/facturas", `{"presupuesto":{"diario":{"peticiones":-1}}}`, "X-CSRF-Token", n.csrf); r.StatusCode != 400 ||
		!strings.Contains(cuerpo, "configuracion_invalida") {
		t.Fatalf("un límite negativo se rechaza: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := n.pedir("PATCH", "/api/v1/puntos/facturas", `{"presupuesto":null}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 {
		t.Fatalf("quitar el presupuesto: %d %s", r.StatusCode, cuerpo)
	}
	if c := trabajo(); c != 202 {
		t.Fatalf("sin presupuesto no hay límite: %d", c)
	}
	if _, cuerpo := n.pedir("GET", "/api/v1/puntos", ""); strings.Contains(cuerpo, `"facturas":{"diario"`) {
		t.Fatalf("sin presupuesto no se lista: %s", cuerpo)
	}
	planificador := s.instancias["global"].Planificador
	planificador.FijarPresupuestos(map[string]nucleo.Presupuesto{"facturas": {Diario: nucleo.Limites{Peticiones: 9}}})
	if r, _ := n.pedir("DELETE", "/api/v1/puntos/facturas", "", "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("eliminar: %d", r.StatusCode)
	}
	planificador.FijarPresupuestos(map[string]nucleo.Presupuesto{"facturas": {Diario: nucleo.Limites{Peticiones: 9}}}) // para leer el consumo
	if estado, _, _ := planificador.EstadoPresupuesto(context.Background(), "facturas"); estado.Diario.Consumo.Peticiones != 0 {
		t.Fatalf("eliminar la API borra su consumo: %+v", estado)
	}
}

func TestPresupuestoInstanciaUI(t *testing.T) {
	s, ts, _ := entorno(t, "presupuesto:\n  mensual:\n    peticiones: 4\n", "", adaptadores.Simulado{})
	instancia, _ := s.ActivarGlobal()
	n := abrirSesion(t, s, ts.URL)
	_, cuerpo := n.pedir("GET", "/api/v1/instancia/presupuesto", "")
	var respuesta struct {
		Presupuesto *nucleo.EstadoPresupuesto `json:"presupuesto"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &respuesta); err != nil || respuesta.Presupuesto == nil || respuesta.Presupuesto.Mensual.Limites.Peticiones != 4 {
		t.Fatalf("presupuesto de la instancia: %v %s", err, cuerpo)
	}
	for range 2 {
		pedir(t, ts.URL, "POST", "/api/v1/trabajos", instancia.token, hola)
	}
	if r, _ := pedir(t, ts.URL, "POST", "/api/v1/trabajos", instancia.token, hola); r.Header.Get("X-Tramtai-Presupuesto") != "50" {
		t.Fatalf("cabecera con 2 de 4: %q", r.Header.Get("X-Tramtai-Presupuesto"))
	}
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/presupuesto/reiniciar", "", "X-CSRF-Token", n.csrf); r.StatusCode != 204 {
		t.Fatalf("reiniciar la instancia: %d %s", r.StatusCode, cuerpo)
	}
	if r, _ := pedir(t, ts.URL, "POST", "/api/v1/trabajos", instancia.token, hola); r.Header.Get("X-Tramtai-Presupuesto") != "0" {
		t.Fatalf("tras reiniciar: %q", r.Header.Get("X-Tramtai-Presupuesto"))
	}
}

func TestNormalizarPresupuestoNegativo(t *testing.T) {
	p := ConfigPunto{Nombre: "x", Presupuesto: &nucleo.Presupuesto{Diario: nucleo.Limites{Tokens: -1}}}
	if _, err := p.Normalizar("global", 17373); nucleo.CodigoDe(err) != nucleo.CodigoConfiguracionInvalida {
		t.Fatalf("límite negativo: %v", err)
	}
	vacio := ConfigPunto{Nombre: "x", Presupuesto: &nucleo.Presupuesto{}}
	if n, err := vacio.Normalizar("global", 17373); err != nil || n.Presupuesto != nil {
		t.Fatalf("un presupuesto vacío se quita: %v %+v", err, n.Presupuesto)
	}
}

func TestRecrearAPINoHeredaConsumo(t *testing.T) {
	global := `puntos_acceso:
  - nombre: facturas
    contratos: [nativa]
    agente: simulado
    presupuesto:
      diario:
        peticiones: 1
`
	s, ts, _ := entorno(t, global, "", adaptadores.Simulado{})
	s.ActivarGlobal()
	pedirCon := func(token string) int {
		r, _ := pedir(t, ts.URL, "POST", "/p/facturas/api/v1/trabajos", token, hola)
		return r.StatusCode
	}
	token, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "facturas"))
	s.Recargar()
	if c := pedirCon(token); c != 202 {
		t.Fatalf("primera petición: %d", c)
	}
	// Como tramtai acceso eliminar: quita el punto del archivo, borra su clave y recarga.
	rutaGlobal, _ := nucleo.RutaGlobal()
	var anterior []ConfigPunto
	if err := EditarPuntos(rutaGlobal, nucleo.OrigenGlobal, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		anterior = lista
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	EliminarTokenPunto(s.opc.Datos, "facturas")
	s.Recargar()
	// Otra API con el mismo nombre empieza sin consumo.
	if err := EditarPuntos(rutaGlobal, nucleo.OrigenGlobal, func([]ConfigPunto) ([]ConfigPunto, error) { return anterior, nil }); err != nil {
		t.Fatal(err)
	}
	token, _ = RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "facturas"))
	s.Recargar()
	if c := pedirCon(token); c != 202 {
		t.Fatalf("la API recreada no hereda el consumo de la eliminada: %d", c)
	}
}

func TestPresupuestoInstanciaSeAplicaAlRecargar(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	if _, ok, _ := s.instancias["global"].Planificador.EstadoPresupuesto(context.Background(), persistencia.TodaLaInstancia); ok {
		t.Fatal("sin presupuesto todavía")
	}
	rutaGlobal, _ := nucleo.RutaGlobal()
	actual, _ := os.ReadFile(rutaGlobal)
	if err := os.WriteFile(rutaGlobal, append(actual, []byte("presupuesto:\n  diario:\n    peticiones: 3\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	s.Recargar()
	if estado, ok, _ := s.instancias["global"].Planificador.EstadoPresupuesto(context.Background(), persistencia.TodaLaInstancia); !ok || estado.Diario.Limites.Peticiones != 3 {
		t.Fatalf("el presupuesto de la instancia se aplica al recargar, sin reiniciar: %v %+v", ok, estado)
	}
}
