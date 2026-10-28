package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/servicio"
)

// TestSalidaServidor: con --servidor, la salida de una orden que crea o cambia algo empieza por «Servidor: »; la de una
// que solo consulta queda como estaba, y sin --servidor no se marca nada.
func TestSalidaServidor(t *testing.T) {
	simularServidor(t, true)
	t.Setenv("TRAMTAI_SERVIDOR", "1")
	salida, err := ejecutarServidor(t, "", "terminos", "aceptar")
	if err != nil || !strings.HasPrefix(salida, idioma.T("Servidor: ")) || strings.Count(salida, idioma.T("Servidor: ")) != 1 {
		t.Errorf("aceptar en el servidor: %v %q", err, salida)
	}
	if salida, err := ejecutarServidor(t, "", "terminos"); err != nil || strings.HasPrefix(salida, idioma.T("Servidor: ")) {
		t.Errorf("consultar no se marca: %v %q", err, salida[:min(len(salida), 80)])
	}
	t.Setenv("TRAMTAI_SERVIDOR", "")
	if salida, err := ejecutarServidor(t, "", "terminos", "aceptar"); err != nil || strings.Contains(salida, idioma.T("Servidor: ")) {
		t.Errorf("sin --servidor no se marca: %v %q", err, salida)
	}

	// Lo que ya empieza por «Servidor: » no se marca dos veces.
	var o bytes.Buffer
	w := &conPrefijo{w: &o, prefijo: "Servidor: "}
	w.Write([]byte("Servidor: hecho\n"))
	w.Write([]byte("otra línea\n"))
	if o.String() != "Servidor: hecho\notra línea\n" {
		t.Errorf("sin marcar dos veces: %q", o.String())
	}
}

// TestEstadoConAvisoCruzado: «tramtai servicio estado» del usuario avisa del otro Tramtai, el del sistema, si lo hay.
func TestEstadoConAvisoCruzado(t *testing.T) {
	anterior := avisoCruzado
	t.Cleanup(func() { avisoCruzado = anterior })
	otro := idioma.Tf("Hay otro Tramtai, del sistema, en el puerto %d: es otro entorno, con sus propias APIs y claves", 17374)
	avisoCruzado = func() string { return otro }
	var o bytes.Buffer
	imprimirEstado(&o, servicio.Estado{Base: "http://127.0.0.1:17373", Version: "v0.58.0", PID: 1, IniciadoEn: time.Now()})
	if !strings.Contains(o.String(), otro) {
		t.Errorf("estado con el aviso:\n%s", o.String())
	}
	avisoCruzado = func() string { return "" }
	o.Reset()
	imprimirEstado(&o, servicio.Estado{Base: "http://127.0.0.1:17373"})
	if strings.Contains(o.String(), "otro Tramtai") {
		t.Errorf("sin servidor no avisa:\n%s", o.String())
	}
}
