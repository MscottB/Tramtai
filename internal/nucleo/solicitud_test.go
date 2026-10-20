package nucleo

import (
	"encoding/json"
	"strings"
	"testing"
)

func solicitudValida() SolicitudEjecucion {
	return SolicitudEjecucion{Agente: "claude", Modelo: "claude-sonnet-5-5", Mensajes: []Mensaje{{Rol: "usuario", Contenido: "Hola"}}}
}

func TestValidarSolicitud(t *testing.T) {
	if err := solicitudValida().Validar(); err != nil {
		t.Fatalf("la solicitud base debe ser válida: %v", err)
	}
	casos := map[string]func(*SolicitudEjecucion){
		"modelo y perfil":      func(s *SolicitudEjecucion) { s.PerfilModelo = "rapido" },
		"sin entrada":          func(s *SolicitudEjecucion) { s.Mensajes = nil },
		"rol desconocido":      func(s *SolicitudEjecucion) { s.Mensajes[0].Rol = "user" },
		"contenido vacío":      func(s *SolicitudEjecucion) { s.Mensajes[0].Contenido = "" },
		"modelo con espacios":  func(s *SolicitudEjecucion) { s.Modelo = "modelo raro" },
		"modo desconocido":     func(s *SolicitudEjecucion) { s.ModoContexto = "otro" },
		"formato desconocido":  func(s *SolicitudEjecucion) { s.Salida.Formato = "xml" },
		"tiempo negativo":      func(s *SolicitudEjecucion) { s.Ejecucion.TiempoLimiteSegundos = -1 },
		"reintentos de más":    func(s *SolicitudEjecucion) { s.Salida = ConfigSalida{Formato: "json", Reintentos: 4} },
		"reintentos negativos": func(s *SolicitudEjecucion) { s.Salida = ConfigSalida{Formato: "json", Reintentos: -1} },
		"reintentos sin json":  func(s *SolicitudEjecucion) { s.Salida.Reintentos = 1 },
	}
	for nombre, modificar := range casos {
		s := solicitudValida()
		modificar(&s)
		if CodigoDe(s.Validar()) != CodigoSolicitudInvalida {
			t.Errorf("%s: se esperaba solicitud_invalida", nombre)
		}
	}
}

func TestHuellaEstableYSensible(t *testing.T) {
	a, b := solicitudValida(), solicitudValida()
	if a.Huella() != b.Huella() || len(a.Huella()) != 64 {
		t.Fatal("dos solicitudes iguales deben tener la misma huella de 64 caracteres")
	}
	b.Modelo = "otro-modelo"
	if a.Huella() == b.Huella() {
		t.Fatal("cambiar el modelo debe cambiar la huella")
	}
}

func TestValidarRespaldo(t *testing.T) {
	for _, ok := range [][]string{nil, {"codex"}, {"codex/gpt-5", "gemini"}, {"a", "b", "c"}, {"codex+alto"}, {"codex/gpt-5+maximo", "claude"}, {"codex+agente"}} {
		if err := ValidarRespaldo(ok); err != nil {
			t.Fatalf("%v: %v", ok, err)
		}
	}
	for _, mal := range [][]string{{"a", "b", "c", "d"}, {"perfil/rapido"}, {""}, {"codex/"}, {"a b"}, {"codex", "codex"}, {"a/b/c"},
		{"codex+turbo"}, {"codex+"}, {"codex", "codex+alto"}, {"+alto"}} {
		if CodigoDe(ValidarRespaldo(mal)) != CodigoSolicitudInvalida {
			t.Fatalf("%v debía rechazarse", mal)
		}
	}
	s := solicitudValida()
	s.Respaldo = []string{"perfil/r"}
	if s.Validar() == nil {
		t.Fatal("Validar debe aplicar ValidarRespaldo")
	}
	if a, m, e := DividirRespaldo("codex/gpt-5+alto"); a != "codex" || m != "gpt-5" || e != "alto" {
		t.Fatalf("DividirRespaldo: %q %q %q", a, m, e)
	}
	if a, m, e := DividirRespaldo("codex"); a != "codex" || m != "" || e != "" {
		t.Fatalf("DividirRespaldo sin modelo ni esfuerzo: %q %q %q", a, m, e)
	}
}

func TestEsquemaConTope(t *testing.T) {
	s := solicitudValida()
	s.Salida.Esquema = json.RawMessage(`{"description":"` + strings.Repeat("x", 256<<10) + `"}`)
	if CodigoDe(s.Validar()) != CodigoSolicitudInvalida {
		t.Fatal("un esquema de más de 256 KB se rechaza")
	}
}

func TestMensajesConTope(t *testing.T) {
	s := solicitudValida()
	for range 1000 {
		s.Mensajes = append(s.Mensajes, Mensaje{Rol: "usuario", Contenido: "x"})
	}
	if CodigoDe(s.Validar()) != CodigoSolicitudInvalida {
		t.Fatal("más de 1.000 mensajes se rechazan")
	}
}

func TestEntornoSinSecretos(t *testing.T) {
	for _, par := range []string{"STRIPE_KEY=x", "DB_PASS=x", "X_PWD=x", "MI_CLAVE=x", "PRIVATE_KEY=x", "DATABASE_URL=postgres://u:p@h/db",
		"SSH_AUTH_SOCK=/tmp/s", "GITHUB_TOKEN=x"} {
		if !EntornoSecreto(par) {
			t.Fatalf("%s no debe pasar", par)
		}
	}
	for _, par := range []string{"PATH=/bin", "PWD=/home/u", "HTTPS_PROXY=http://proxy:8080", "LANG=es_ES.UTF-8", "KEYBOARD=x"} {
		if EntornoSecreto(par) {
			t.Fatalf("%s debe pasar", par)
		}
	}
}

func TestProxyConCredencialesPasa(t *testing.T) {
	for _, par := range []string{"HTTPS_PROXY=http://u:p@proxy:8080", "http_proxy=http://u:p@proxy:8080", "ALL_PROXY=socks5://u:p@proxy:1080"} {
		if EntornoSecreto(par) {
			t.Fatalf("%s debe pasar: los CLI lo necesitan para salir a su proveedor", par)
		}
	}
}

func TestEleccionHerramientaJSON(t *testing.T) {
	casos := map[string]EleccionHerramienta{
		`"auto"`: {Modo: "auto"}, `"ninguna"`: {Modo: "ninguna"}, `"obligatoria"`: {Modo: "obligatoria"},
		`{"nombre":"clima"}`: {Modo: "una", Nombre: "clima"},
	}
	for crudo, quiere := range casos {
		var e EleccionHerramienta
		if err := json.Unmarshal([]byte(crudo), &e); err != nil || e != quiere {
			t.Fatalf("%s: %+v %v", crudo, e, err)
		}
		if vuelta, _ := json.Marshal(e); string(vuelta) != crudo {
			t.Fatalf("ida y vuelta de %s: %s", crudo, vuelta)
		}
	}
	for _, crudo := range []string{`"otra"`, `{"nombre":""}`, `3`} {
		var e EleccionHerramienta
		if json.Unmarshal([]byte(crudo), &e) == nil {
			t.Fatalf("%s debía rechazarse", crudo)
		}
	}
}

func TestLlamadaArgumentosTextoUObjeto(t *testing.T) {
	for _, crudo := range []string{`{"id":"a","nombre":"clima","argumentos":"{\"ciudad\": \"Santiago\"}"}`,
		`{"id":"a","nombre":"clima","argumentos":{ "ciudad": "Santiago" }}`} {
		var l Llamada
		if err := json.Unmarshal([]byte(crudo), &l); err != nil || string(l.Argumentos) != `{"ciudad":"Santiago"}` {
			t.Fatalf("%s: %s %v", crudo, l.Argumentos, err)
		}
	}
	for _, crudo := range []string{`{"id":"a","nombre":"clima","argumentos":"[1]"}`, `{"id":"a","nombre":"clima","argumentos":"x"}`} {
		var l Llamada
		if json.Unmarshal([]byte(crudo), &l) == nil {
			t.Fatalf("%s debía rechazarse", crudo)
		}
	}
}

func TestValidarMensajesDeHerramientas(t *testing.T) {
	conversacion := func() SolicitudEjecucion {
		s := solicitudValida()
		s.Herramientas = []Herramienta{{Nombre: "clima"}}
		s.Mensajes = append(s.Mensajes,
			Mensaje{Rol: "asistente", Llamadas: []Llamada{{ID: "l0", Nombre: "clima", Argumentos: json.RawMessage(`{}`)}}},
			Mensaje{Rol: "herramienta", IDLlamada: "l0", Contenido: "18 °C"})
		return s
	}
	if err := conversacion().Validar(); err != nil {
		t.Fatalf("una conversación con llamadas y su resultado es válida: %v", err)
	}
	s := conversacion()
	s.Salida.Reintentos = 1
	if err := s.Validar(); err != nil {
		t.Fatalf("con herramientas, las correcciones no exigen formato json: %v", err)
	}
	casos := map[string]func(*SolicitudEjecucion){
		"herramienta sin id_llamada":   func(s *SolicitudEjecucion) { s.Mensajes[2].IDLlamada = "" },
		"llamadas en un usuario":       func(s *SolicitudEjecucion) { s.Mensajes[0].Llamadas = s.Mensajes[1].Llamadas },
		"asistente vacío sin llamadas": func(s *SolicitudEjecucion) { s.Mensajes[1].Llamadas = nil },
		"reintentos con ninguna":       func(s *SolicitudEjecucion) { s.EleccionHerramienta.Modo = "ninguna"; s.Salida.Reintentos = 1 },
	}
	for nombre, modificar := range casos {
		s := conversacion()
		modificar(&s)
		if CodigoDe(s.Validar()) != CodigoSolicitudInvalida {
			t.Errorf("%s: se esperaba solicitud_invalida", nombre)
		}
	}
	// Una solicitud sin herramientas se guarda como antes: sin claves nuevas.
	datos, _ := json.Marshal(solicitudValida())
	for _, clave := range []string{"herramientas", "eleccion_herramienta", "llamadas", "id_llamada", "es_error"} {
		if strings.Contains(string(datos), clave) {
			t.Fatalf("clave nueva en una solicitud sin herramientas: %s", datos)
		}
	}
}

func TestResultadoDeHerramientaVacio(t *testing.T) {
	s := solicitudValida()
	s.Mensajes = append(s.Mensajes,
		Mensaje{Rol: "asistente", Llamadas: []Llamada{{ID: "l0", Nombre: "clima", Argumentos: json.RawMessage(`{}`)}}},
		Mensaje{Rol: "herramienta", IDLlamada: "l0"})
	if err := s.Validar(); err != nil {
		t.Fatalf("un resultado de herramienta puede venir vacío (OpenAI lo permite): %v", err)
	}
}
