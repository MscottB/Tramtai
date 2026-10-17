package nucleo

import "testing"

func solicitudValida() SolicitudEjecucion {
	return SolicitudEjecucion{Agente: "claude", Modelo: "claude-sonnet-5-5", Mensajes: []Mensaje{{Rol: "usuario", Contenido: "Hola"}}}
}

func TestValidarSolicitud(t *testing.T) {
	if err := solicitudValida().Validar(); err != nil {
		t.Fatalf("la solicitud base debe ser válida: %v", err)
	}
	casos := map[string]func(*SolicitudEjecucion){
		"modelo y perfil":     func(s *SolicitudEjecucion) { s.PerfilModelo = "rapido" },
		"sin entrada":         func(s *SolicitudEjecucion) { s.Mensajes = nil },
		"rol desconocido":     func(s *SolicitudEjecucion) { s.Mensajes[0].Rol = "user" },
		"contenido vacío":     func(s *SolicitudEjecucion) { s.Mensajes[0].Contenido = "" },
		"modelo con espacios": func(s *SolicitudEjecucion) { s.Modelo = "modelo raro" },
		"modo desconocido":    func(s *SolicitudEjecucion) { s.ModoContexto = "otro" },
		"formato desconocido": func(s *SolicitudEjecucion) { s.Salida.Formato = "xml" },
		"tiempo negativo":     func(s *SolicitudEjecucion) { s.Ejecucion.TiempoLimiteSegundos = -1 },
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
	for _, ok := range [][]string{nil, {"codex"}, {"codex/gpt-5", "gemini"}, {"a", "b", "c"}, {"codex+alto"}, {"codex/gpt-5+maximo", "claude"}} {
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
