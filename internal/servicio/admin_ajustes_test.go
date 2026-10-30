package servicio

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"tramtai/internal/actualizacion"
	"tramtai/internal/adaptadores"
	"tramtai/internal/autoarranque"
	"tramtai/internal/contrato"
	"tramtai/internal/version"
)

// La sesión local de la Web UI enseña el puerto y el arranque con el sistema que ve Ajustes en la terminal; el arranque
// va aparte, fuera de servicio.
func TestSesionWebYAjustesCoinciden(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	s.opc.Autoarranque = func() (bool, autoarranque.Modo) { return true, autoarranque.ModoServicio }
	s.opc.PuertoConfigurado, s.opc.PuertoAutomatico = 4321, true
	n := nuevoNavegador(t, ts.URL)
	codigo, _ := s.CodigoSesion("global")
	n.pedir("GET", "/ui/entrar?codigo="+codigo, "")
	_, cuerpo := n.pedir("GET", "/ui/sesion", "")
	var sesion struct {
		Servicio     json.RawMessage `json:"servicio"`
		Autoarranque *bool           `json:"autoarranque"`
		ModoArranque string          `json:"modo_arranque"`
	}
	var web, terminal contrato.AjustesServicio
	if err := json.Unmarshal([]byte(cuerpo), &sesion); err != nil || json.Unmarshal(sesion.Servicio, &web) != nil || strings.Contains(string(sesion.Servicio), "autoarranque") || strings.Contains(string(sesion.Servicio), "modo_arranque") || sesion.ModoArranque != "servicio" {
		t.Fatalf("sesión: %v %s", err, cuerpo)
	}
	web.Autoarranque, web.ModoArranque = sesion.Autoarranque, sesion.ModoArranque
	_, texto := admin(t, s, ts.URL, "GET", "/api/v1/servicio/ajustes", "")
	if err := json.Unmarshal([]byte(texto), &terminal); err != nil || !reflect.DeepEqual(web, terminal) {
		t.Fatalf("la Web UI ve %s y la terminal %s", cuerpo, texto)
	}
}

// La terminal llega a Ajustes por la administración, con el token de servicio: el puerto y el arranque con el sistema,
// las actualizaciones (consultar, buscar ahora, activar la búsqueda, actualizar y decidir la instalación) y Acerca de.
// Nada va a GitHub ni cambia el arranque del equipo: la versión y el arranque son los de la prueba.
func TestAdminAjustesYActualizaciones(t *testing.T) {
	antes := version.Version
	version.Version = "v0.56.0" // una compilación «dev» nunca se da por desactualizada
	t.Cleanup(func() { version.Version = antes })
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	modo := autoarranque.Modo("")
	s.opc.Autoarranque = func() (bool, autoarranque.Modo) { return true, modo }
	s.opc.CambiarAutoarranque = func(m autoarranque.Modo) error { modo = m; return nil }
	consultas := 0
	nueva := func(context.Context) (actualizacion.Version, error) {
		consultas++
		return actualizacion.Version{Etiqueta: "v99.0.0", Pagina: "https://github.com/MscottB/Tramtai/releases/tag/v99.0.0", Notas: "Novedades:\n- Algo"}, nil
	}
	s.opc.ConsultarVersion, s.actualizaciones.Consultar = nueva, nueva
	leer := func(metodo, ruta, cuerpo string, destino any) int {
		t.Helper()
		r, texto := admin(t, s, ts.URL, metodo, ruta, cuerpo)
		if destino != nil && r.StatusCode < 300 {
			if err := json.Unmarshal([]byte(texto), destino); err != nil {
				t.Fatalf("%s %s: %s", metodo, ruta, texto)
			}
		}
		return r.StatusCode
	}

	var a contrato.AjustesServicio
	if c := leer("GET", "/api/v1/servicio/ajustes", "", &a); c != 200 || a.Puerto != s.opc.Puerto || a.PuertoConfigurado != s.opc.Puerto || a.Autoarranque == nil || *a.Autoarranque {
		t.Fatalf("ajustes: %d %+v", c, a)
	}
	if c := leer("PUT", "/api/v1/servicio/autoarranque", `{"activo":true}`, nil); c != 204 || modo != autoarranque.ModoApp {
		t.Fatalf("activar el arranque (sin modo es la app): %d %q", c, modo)
	}
	if leer("GET", "/api/v1/servicio/ajustes", "", &a); a.Autoarranque == nil || !*a.Autoarranque || a.ModoArranque != "app" {
		t.Fatalf("el arranque se ve activo: %+v", a)
	}
	if c := leer("PUT", "/api/v1/servicio/autoarranque", `{"activo":true,"modo":"service"}`, nil); c != 204 || modo != autoarranque.ModoServicio {
		t.Fatalf("«service» es el servicio: %d %q", c, modo)
	}
	if c := leer("PUT", "/api/v1/servicio/autoarranque", `{"activo":true,"modo":"otro"}`, nil); c != 400 || modo != autoarranque.ModoServicio {
		t.Fatalf("un modo desconocido no cambia nada: %d %q", c, modo)
	}
	if c := leer("PUT", "/api/v1/servicio/autoarranque", `{"activo":false,"modo":"servicio"}`, nil); c != 204 || modo != "" {
		t.Fatalf("desactivar: %d %q", c, modo)
	}
	a = contrato.AjustesServicio{} // decodificar sobre lo anterior dejaría el modo que ya no viene
	if leer("GET", "/api/v1/servicio/ajustes", "", &a); a.Autoarranque == nil || *a.Autoarranque || a.ModoArranque != "" {
		t.Fatalf("el arranque se ve desactivado: %+v", a)
	}

	var e contrato.EstadoActualizacion
	if c := leer("GET", "/api/v1/servicio/actualizacion", "", &e); c != 200 || e.Actual != version.Version || e.Buscar || e.Disponible || consultas != 0 {
		t.Fatalf("sin búsqueda no se consulta: %d %+v (%d)", c, e, consultas)
	}
	if leer("GET", "/api/v1/servicio/actualizacion?ahora=1", "", &e); !e.Disponible || e.Version != "v99.0.0" || e.Notas == "" || e.Comprobado.IsZero() || consultas != 1 {
		t.Fatalf("buscar ahora consulta aunque esté desactivada: %+v (%d)", e, consultas)
	}
	if leer("PUT", "/api/v1/servicio/actualizacion", `{"buscar":true}`, &e); !e.Buscar || !s.buscar.Load() {
		t.Fatalf("activar la búsqueda: %+v", e)
	}

	// Esta copia no se instaló con el instalador: actualizar responde por qué, sin descargar nada.
	r, texto := admin(t, s, ts.URL, "POST", "/api/v1/servicio/actualizar", "")
	if r.StatusCode != 400 || !strings.Contains(texto, "v99.0.0") {
		t.Fatalf("actualizar sin instalador: %d %s", r.StatusCode, texto)
	}
	var p contrato.ProgresoActualizacion
	if c := leer("GET", "/api/v1/servicio/actualizacion/progreso", "", &p); c != 200 || p.Fase != "" {
		t.Fatalf("sin actualización en curso: %d %+v", c, p)
	}
	for _, ruta := range []string{"/api/v1/servicio/actualizar/instalar", "/api/v1/servicio/actualizar/cancelar"} {
		if r, texto := admin(t, s, ts.URL, "POST", ruta, ""); r.StatusCode != 409 || !strings.Contains(texto, "solicitud_invalida") {
			t.Fatalf("%s fuera de la cuenta atrás: %d %s", ruta, r.StatusCode, texto)
		}
	}

	var acerca contrato.Acerca
	if c := leer("GET", "/api/v1/servicio/acerca", "", &acerca); c != 200 || acerca.Version != version.Texto() || acerca.Servicio.PID == 0 || acerca.Rutas == nil || acerca.Rutas.Datos == "" {
		t.Fatalf("acerca: %d %+v", c, acerca)
	}

	// Sin búsqueda, las novedades no hablan con GitHub.
	s.buscar.Store(false)
	var n struct {
		Versiones []contrato.VersionNueva `json:"versiones"`
	}
	if c := leer("GET", "/api/v1/servicio/actualizacion/novedades?desde=v0.1.0", "", &n); c != 200 || n.Versiones == nil || len(n.Versiones) != 0 {
		t.Fatalf("novedades sin búsqueda: %d %+v", c, n)
	}
}

// rutasAjustes son las de administración que usa Ajustes de la terminal, con su método y un cuerpo válido.
var rutasAjustes = []struct{ metodo, ruta, cuerpo string }{
	{"GET", "/api/v1/servicio/ajustes", ""}, {"PUT", "/api/v1/servicio/autoarranque", `{"activo":true}`},
	{"GET", "/api/v1/servicio/actualizacion", ""}, {"GET", "/api/v1/servicio/actualizacion?ahora=1", ""},
	{"PUT", "/api/v1/servicio/actualizacion", `{"buscar":true}`}, {"POST", "/api/v1/servicio/actualizar", ""},
	{"POST", "/api/v1/servicio/actualizar/instalar", ""}, {"POST", "/api/v1/servicio/actualizar/cancelar", ""},
	{"GET", "/api/v1/servicio/actualizacion/progreso", ""}, {"GET", "/api/v1/servicio/actualizacion/novedades?desde=v0.1.0", ""},
	{"GET", "/api/v1/servicio/acerca", ""}, {"POST", "/api/v1/servicio/reiniciar", ""},
}

// Todas las rutas de Ajustes son solo de la administración: ni sin clave, ni con la clave de una app, ni con una sesión de
// la Web UI (aunque lleve su CSRF) entran, y nada cambia.
func TestAdminAjustesSoloConLaClaveDelServicio(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	cambios := 0
	s.opc.CambiarAutoarranque = func(autoarranque.Modo) error { cambios++; return nil }
	s.opc.Reiniciar = func() error { cambios++; return nil }
	s.opc.ConsultarVersion = func(context.Context) (actualizacion.Version, error) { cambios++; return actualizacion.Version{}, nil }
	s.actualizaciones.Consultar = s.opc.ConsultarVersion
	inst, _ := s.ActivarGlobal()
	clave := claveDe(t, s, inst)
	web := abrirSesion(t, s, ts.URL)
	for _, c := range rutasAjustes {
		for quien, token := range map[string]string{"sin clave": "", "con la clave de la app": clave} {
			if r, _ := pedir(t, ts.URL, c.metodo, c.ruta, token, c.cuerpo); r.StatusCode != 401 {
				t.Errorf("%s %s %s: %d", c.metodo, c.ruta, quien, r.StatusCode)
			}
		}
		if r, _ := web.pedir(c.metodo, c.ruta, c.cuerpo, "X-CSRF-Token", web.csrf); r.StatusCode != 401 {
			t.Errorf("%s %s con la sesión de la Web UI: %d", c.metodo, c.ruta, r.StatusCode)
		}
	}
	if cambios != 0 || s.buscar.Load() {
		t.Fatalf("una petición rechazada no cambia nada: %d %v", cambios, s.buscar.Load())
	}
}

// «Reiniciar ahora» tras cambiar el puerto, como en la Web UI: la administración pide el servicio nuevo una vez (un segundo
// reinicio responde lo mismo) y dice dónde escuchará; sin reinicio posible, 400.
func TestAdminReiniciar(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if r, texto := admin(t, s, ts.URL, "POST", "/api/v1/servicio/reiniciar", ""); r.StatusCode != 400 {
		t.Fatalf("sin reinicio posible: %d %s", r.StatusCode, texto)
	}
	pedidos := 0
	s.opc.Reiniciar = func() error { pedidos++; return nil }
	for range 2 {
		r, texto := admin(t, s, ts.URL, "POST", "/api/v1/servicio/reiniciar", "")
		if r.StatusCode != 202 || !strings.Contains(texto, `"url":"http://127.0.0.1:`) {
			t.Fatalf("reiniciar: %d %s", r.StatusCode, texto)
		}
	}
	if pedidos != 1 {
		t.Fatalf("el servicio nuevo se pide una vez: %d", pedidos)
	}
	select {
	case <-s.Detenido():
	default:
		t.Fatal("este servicio se detiene para dejar sitio al nuevo")
	}
}

// TestAutoarranqueUIModo: /ui/autoarranque acepta {activo, modo} (sin modo, la app; «service» vale por «servicio») y la
// sesión dice el modo en que queda. Un modo desconocido es un 400 y no cambia nada.
func TestAutoarranqueUIModo(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.ActivarGlobal()
	modo := autoarranque.Modo("")
	s.opc.Autoarranque = func() (bool, autoarranque.Modo) { return true, modo }
	s.opc.CambiarAutoarranque = func(m autoarranque.Modo) error { modo = m; return nil }
	n := abrirSesion(t, s, ts.URL)
	pedir := func(cuerpo string) (int, string) {
		r, texto := n.pedir("POST", "/ui/autoarranque", cuerpo, "X-CSRF-Token", n.csrf)
		return r.StatusCode, texto
	}
	for _, c := range []struct {
		cuerpo string
		modo   autoarranque.Modo
	}{
		{`{"activo":true}`, autoarranque.ModoApp}, {`{"activo":true,"modo":"servicio"}`, autoarranque.ModoServicio},
		{`{"activo":true,"modo":"app"}`, autoarranque.ModoApp}, {`{"activo":true,"modo":"service"}`, autoarranque.ModoServicio},
		{`{"activo":false,"modo":"servicio"}`, ""},
	} {
		codigo, texto := pedir(c.cuerpo)
		var sesion struct {
			Autoarranque *bool  `json:"autoarranque"`
			ModoArranque string `json:"modo_arranque"`
		}
		if err := json.Unmarshal([]byte(texto), &sesion); codigo != 200 || err != nil || modo != c.modo || sesion.Autoarranque == nil || *sesion.Autoarranque != (c.modo != "") || sesion.ModoArranque != string(c.modo) {
			t.Fatalf("%s: %d %q %s (%v)", c.cuerpo, codigo, modo, texto, err)
		}
	}
	modo = autoarranque.ModoApp
	if codigo, texto := pedir(`{"activo":true,"modo":"otro"}`); codigo != 400 || !strings.Contains(texto, "solicitud_invalida") || modo != autoarranque.ModoApp {
		t.Fatalf("un modo desconocido: %d %s %q", codigo, texto, modo)
	}
	// En el servidor no hay arranque al iniciar sesión: el servicio del sistema arranca con el equipo.
	s.opc.Entorno = "servidor"
	if codigo, texto := pedir(`{"activo":true,"modo":"servicio"}`); codigo != 400 || !strings.Contains(texto, "arranca con el equipo") || modo != autoarranque.ModoApp {
		t.Fatalf("en el servidor: %d %s %q", codigo, texto, modo)
	}
}
