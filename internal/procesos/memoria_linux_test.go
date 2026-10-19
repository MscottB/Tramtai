package procesos

import (
	"context"
	"strings"
	"testing"
)

// El tope de memoria rige para el programa y para lo que este lance (aquí, un sh dentro de otro).
func TestTopeDeMemoriaHeredado(t *testing.T) {
	var lineas []string
	r, err := Ejecutar(context.Background(), Orden{Programa: "/bin/sh", Argumentos: []string{"-c", "ulimit -v; sh -c 'ulimit -v'"}, MemoriaMB: 64},
		func(l string) error { lineas = append(lineas, l); return nil })
	if err != nil || r.CodigoSalida != 0 {
		t.Fatalf("%v %+v", err, r)
	}
	if strings.Join(lineas, ",") != "65536,65536" {
		t.Fatalf("tope del programa y de su hijo: %v", lineas)
	}
}
