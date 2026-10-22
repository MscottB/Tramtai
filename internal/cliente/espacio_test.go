package cliente

import (
	"testing"

	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
)

func TestElegirEspacio(t *testing.T) {
	activos := []contrato.EstadoInstancia{{ID: "global", Ambito: "global"}, {ID: "otro_1234abcd", Ambito: "proyecto", Raiz: `/tmp/otro`}}
	if _, err := ElegirEspacio("/tmp/detenido", activos, "", false); nucleo.CodigoDe(err) != nucleo.CodigoEspacioDetenido {
		t.Fatalf("dentro de un proyecto detenido no debe usar la global: %v", err)
	}
	if id, err := ElegirEspacio("/tmp/detenido", activos, "", true); err != nil || id != "global" {
		t.Fatalf("con global: %q %v", id, err)
	}
	if id, _ := ElegirEspacio("/tmp/otro", activos, "", false); id != "otro_1234abcd" {
		t.Fatalf("el proyecto activo: %q", id)
	}
	if id, _ := ElegirEspacio("", activos, "", false); id != "global" {
		t.Fatalf("fuera de un proyecto, la global: %q", id)
	}
	if id, _ := ElegirEspacio("/tmp/detenido", activos, "elegido", false); id != "elegido" {
		t.Fatalf("la preferida manda: %q", id)
	}
}
