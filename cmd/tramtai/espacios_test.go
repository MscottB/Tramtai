package main

import (
	"strings"
	"testing"

	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
)

func TestCLINoCaeEnLaGlobal(t *testing.T) {
	activos := []contrato.EstadoInstancia{{ID: "global", Ambito: "global"}, {ID: "otro_1234abcd", Ambito: "proyecto", Raiz: `/tmp/otro`}}
	if _, err := elegirEspacio("/tmp/detenido", activos, "", false); nucleo.CodigoDe(err) != nucleo.CodigoEspacioDetenido {
		t.Fatalf("dentro de un proyecto detenido no debe usar la global: %v", err)
	}
	if id, err := elegirEspacio("/tmp/detenido", activos, "", true); err != nil || id != "global" {
		t.Fatalf("con --global: %q %v", id, err)
	}
	if id, _ := elegirEspacio("/tmp/otro", activos, "", false); id != "otro_1234abcd" {
		t.Fatalf("el proyecto activo: %q", id)
	}
	if id, _ := elegirEspacio("", activos, "", false); id != "global" {
		t.Fatalf("fuera de un proyecto, la global: %q", id)
	}
	if id, _ := elegirEspacio("/tmp/detenido", activos, "elegido", false); id != "elegido" {
		t.Fatalf("-i manda: %q", id)
	}
}

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
