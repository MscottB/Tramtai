package main

import (
	"strings"
	"testing"

	"tramtai/internal/contrato"
)

func TestComandoEspacios(t *testing.T) {
	var b strings.Builder
	escribirEspacios(&b, []contrato.Espacio{
		{ID: "global", Ambito: "global", Nombre: "Global", Estado: "activo", Confiado: true},
		{ID: "demo_1234abcd", Ambito: "proyecto", Nombre: "Demo", Ruta: "/tmp/demo", Estado: "no_encontrado", APIs: 2, Peticiones: 7},
	})
	salida := b.String()
	if !strings.Contains(salida, "Global") || !strings.Contains(salida, "demo_1234abcd") || !strings.Contains(salida, "/tmp/demo") || strings.Count(salida, "\n") < 3 {
		t.Fatalf("salida:\n%s", salida)
	}
}
