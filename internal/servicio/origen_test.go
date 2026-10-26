package servicio

import (
	"context"
	"encoding/json"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
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
	clave := claveDe(t, s, global)
	tienda, _ := NuevaClave(RutaTokenPunto(s.opc.Datos, "tienda"))
	s.Recargar()
	n := abrirSesion(t, s, ts.URL)

	_, cuerpo := n.pedir("POST", "/api/v1/trabajos", hola, "X-CSRF-Token", n.csrf, "Content-Type", "application/json")
	if o := origenDe(t, cuerpo); o != "probar" {
		t.Errorf("con la sesión de la Web UI: %q", o)
	}
	for cliente, esperado := range map[string]string{"cli": "cli", "terminal": "terminal", "": "app", "otra": "app"} {
		_, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/trabajos", clave, hola, "X-Tramtai-Cliente", cliente)
		if o := origenDe(t, cuerpo); o != esperado {
			t.Errorf("con la clave y X-Tramtai-Cliente %q: %q, esperado %q", cliente, o, esperado)
		}
	}
	// Desde otro equipo (o tras un proxy) la etiqueta no vale: es una aplicación, también para el bloqueo de Antigravity.
	if !procesos.MismoUsuario(netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:2")) && runtime.GOOS != "windows" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Log("sin identificación del dueño de la conexión: no se comprueba la etiqueta remota")
	} else if _, cuerpo := pedir(t, ts.URL, "POST", "/api/v1/trabajos", clave, hola, "X-Tramtai-Cliente", "cli", "X-Forwarded-For", "203.0.113.5"); origenDe(t, cuerpo) != "app" {
		t.Errorf("la etiqueta cli tras un proxy: %q", origenDe(t, cuerpo))
	}
	_, cuerpo = pedir(t, ts.URL, "POST", "/p/tienda/api/v1/trabajos", tienda, hola, "X-Tramtai-Cliente", "cli")
	if o := origenDe(t, cuerpo); o != "api" {
		t.Errorf("por una API extra el encabezado no cuenta: %q", o)
	}
}

func TestListarPorGrupoYOrigen(t *testing.T) {
	s, ts, _ := entorno(t, puntosOrigen, "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	clave := claveDe(t, s, global)
	tokens := map[string]string{}
	for _, p := range []string{"tienda", "soporte", "suelta"} {
		tokens[p], _ = NuevaClave(RutaTokenPunto(s.opc.Datos, p))
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
		if c, total := contar(clave, ruta); c != 200 || total != esperado {
			t.Errorf("%s: %d %d, esperado %d", ruta, c, total, esperado)
		}
	}
	if c, _ := contar(clave, "/api/v1/trabajos?antes=ayer-no"); c != 400 {
		t.Errorf("antes inválido: %d", c)
	}
	if c, _ := contar(clave, "/api/v1/trabajos?origen=nadie"); c != 400 {
		t.Errorf("origen desconocido: %d", c)
	}
	_, cuerpo := pedir(t, ts.URL, "GET", "/api/v1/trabajos?limite=1", clave, "")
	mas := cuerpo[strings.Index(cuerpo, `"creado_en":"`)+13:]
	mas = mas[:strings.Index(mas, `"`)]
	if c, total := contar(clave, "/api/v1/trabajos?antes="+mas); c != 200 || total != 3 {
		t.Errorf("antes del más nuevo: %d %d", c, total)
	}
	if c, total := contar(tokens["tienda"], "/p/tienda/api/v1/trabajos?grupo=web&origen=probar"); c != 200 || total != 1 {
		t.Errorf("una API extra solo ve las suyas, sin grupo ni origen: %d %d", c, total)
	}
}

// Solo eliminar una API borra su consumo: una que falta en la configuración y cuya clave se borró a mano (tokens/)
// conserva el suyo, porque perderlo dejaría superar su presupuesto.
func TestConsumoDeAPIAusenteSeConserva(t *testing.T) {
	s, _, _ := entorno(t, puntosOrigen, "", adaptadores.Simulado{})
	global, _ := s.ActivarGlobal()
	for _, punto := range []string{"tienda", "vieja"} { // vieja ya no está en la configuración ni tiene clave
		tr := nucleo.Trabajo{ID: nucleo.NuevoID(), Instancia: global.ID, PuntoAcceso: punto,
			Solicitud: nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}}
		if _, _, err := global.almacen.Aceptar(context.Background(), tr, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	os.Remove(RutaTokenPunto(s.opc.Datos, "tienda")) // tokens/ tocado a mano: tienda sigue configurada
	s.Recargar()
	vieja, _, _ := global.almacen.Consumo(context.Background(), global.ID, "vieja")
	tienda, _, _ := global.almacen.Consumo(context.Background(), global.ID, "tienda")
	if vieja.Peticiones != 1 || tienda.Peticiones != 1 {
		t.Fatalf("recargar no borra consumo: vieja %+v, tienda %+v", vieja, tienda)
	}
}

func TestPuntoConAntigravity(t *testing.T) {
	entorno(t, "", "", adaptadores.Simulado{}) // la configuración de la prueba (sin permitir_apps)
	for _, p := range []ConfigPunto{{Nombre: "a", Agente: "antigravity"}, {Nombre: "b", Respaldo: []string{"antigravity/gemini-x"}}} {
		if err := validarPunto(p, enCatalogo); nucleo.CodigoDe(err) != nucleo.CodigoConfiguracionInvalida || !strings.Contains(nucleo.MensajeDe(err), "Antigravity") {
			t.Fatalf("%+v: %v", p, err)
		}
	}
	if err := validarPunto(ConfigPunto{Nombre: "c", Agente: "claude", Respaldo: []string{"codex"}}, enCatalogo); err != nil {
		t.Fatal(err)
	}
}

func enCatalogo(agente string) bool {
	_, ok := agentes.Buscar(agente)
	return ok
}
