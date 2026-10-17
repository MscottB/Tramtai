package servicio

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

const puntosOrigen = `puntos_acceso:
  - nombre: tienda
    grupo: web
    contratos: [nativa]
    agente: simulado
  - nombre: soporte
    grupo: web
    contratos: [nativa]
    agente: simulado
  - nombre: suelta
    contratos: [nativa]
    agente: simulado
`

func origenDe(t *testing.T, cuerpo string) string {
	t.Helper()
	var tr struct {
		Origen string `json:"origen"`
	}
	if err := json.Unmarshal([]byte(cuerpo), &tr); err != nil {
		t.Fatalf("%v: %s", err, cuerpo)
	}
	return tr.Origen
}

func TestOrigenSegunAutenticacion(t *testing.T) {
	s, ts, _ := entorno(t, puntosOrigen, "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	tienda, _ := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, "tienda"))
	s.Recargar()
	n := abrirSesion(t, s, ts.URL)

	_, cuerpo := n.pedir("POST", "/api/v1/trabajos", hola, "X-CSRF-Token", n.csrf, "Content-Type", "application/json")
	if o := origenDe(t, cuerpo); o != "probar" {
		t.Errorf("con la sesión de la Web UI: %q", o)
	}
	for cliente, esperado := range map[string]string{"cli": "cli", "terminal": "terminal", "": "app", "otra": "app"} {
		_, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/trabajos", global.token, hola, "X-Tramtai-Cliente", cliente)
		if o := origenDe(t, cuerpo); o != esperado {
			t.Errorf("con la clave y X-Tramtai-Cliente %q: %q, esperado %q", cliente, o, esperado)
		}
	}
	_, cuerpo = pedir(t, ts.URL, "POST", "/p/tienda/api/v1/trabajos", tienda, hola, "X-Tramtai-Cliente", "cli")
	if o := origenDe(t, cuerpo); o != "api" {
		t.Errorf("por una API extra el encabezado no cuenta: %q", o)
	}
}

func TestListarPorGrupoYOrigen(t *testing.T) {
	s, ts, _ := entorno(t, puntosOrigen, "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	tokens := map[string]string{}
	for _, p := range []string{"tienda", "soporte", "suelta"} {
		tokens[p], _ = RotarTokenPunto(RutaTokenPunto(s.opc.Datos, p))
	}
	s.Recargar()
	for _, p := range []string{"tienda", "soporte", "suelta"} {
		pedir(t, ts.URL, "POST", "/p/"+p+"/api/v1/trabajos", tokens[p], hola)
	}
	n := abrirSesion(t, s, ts.URL)
	n.pedir("POST", "/api/v1/trabajos", hola, "X-CSRF-Token", n.csrf, "Content-Type", "application/json")

	contar := func(token, ruta string) (int, int) {
		r, cuerpo := pedir(t, ts.URL, "GET", ruta, token, "")
		var lista struct {
			Trabajos []struct {
				CreadoEn string `json:"creado_en"`
			} `json:"trabajos"`
		}
		json.Unmarshal([]byte(cuerpo), &lista)
		return r.StatusCode, len(lista.Trabajos)
	}
	for ruta, esperado := range map[string]int{
		"/api/v1/trabajos?grupo=web":     2,
		"/api/v1/trabajos?grupo=nadie":   0, // un grupo sin APIs no lista todo
		"/api/v1/trabajos?origen=probar": 1,
		"/api/v1/trabajos":               4,
	} {
		if c, total := contar(global.token, ruta); c != 200 || total != esperado {
			t.Errorf("%s: %d %d, esperado %d", ruta, c, total, esperado)
		}
	}
	if c, _ := contar(global.token, "/api/v1/trabajos?antes=ayer-no"); c != 400 {
		t.Errorf("antes inválido: %d", c)
	}
	if c, _ := contar(global.token, "/api/v1/trabajos?origen=nadie"); c != 400 {
		t.Errorf("origen desconocido: %d", c)
	}
	_, cuerpo := pedir(t, ts.URL, "GET", "/api/v1/trabajos?limite=1", global.token, "")
	mas := cuerpo[strings.Index(cuerpo, `"creado_en":"`)+13:]
	mas = mas[:strings.Index(mas, `"`)]
	if c, total := contar(global.token, "/api/v1/trabajos?antes="+mas); c != 200 || total != 3 {
		t.Errorf("antes del más nuevo: %d %d", c, total)
	}
	if c, total := contar(tokens["tienda"], "/p/tienda/api/v1/trabajos?grupo=web&origen=probar"); c != 200 || total != 1 {
		t.Errorf("una API extra solo ve las suyas, sin grupo ni origen: %d %d", c, total)
	}
}

func TestConsumoDeAPIAusenteSeBorra(t *testing.T) {
	s, _, _ := entorno(t, puntosOrigen, "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	for _, punto := range []string{"tienda", "vieja"} { // vieja ya no está en la configuración
		tr := nucleo.Trabajo{ID: nucleo.NuevoID(), Instancia: global.ID, PuntoAcceso: punto,
			Solicitud: nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}}
		if _, _, err := global.almacen.Aceptar(context.Background(), tr, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	s.Recargar()
	dia, _, _ := global.almacen.Consumo(context.Background(), global.ID, "vieja")
	tienda, _, _ := global.almacen.Consumo(context.Background(), global.ID, "tienda")
	if dia.Peticiones != 0 || tienda.Peticiones != 1 {
		t.Fatalf("se borra el consumo de la API ausente y no el de las demás: vieja %+v, tienda %+v", dia, tienda)
	}
}
