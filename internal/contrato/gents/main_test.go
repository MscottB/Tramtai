package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"tramtai/internal/nucleo"
)

// TestAlDia: web/src/api/tipos.gen.ts es lo que sale hoy de los structs de Go. Si falla, un tipo del contrato cambió:
// go generate ./internal/contrato y sube el archivo.
func TestAlDia(t *testing.T) {
	guardado, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "api", "tipos.gen.ts"))
	if err != nil {
		t.Fatalf("%v: go generate ./internal/contrato", err)
	}
	if strings.ReplaceAll(string(guardado), "\r\n", "\n") != Generar() {
		t.Fatal("web/src/api/tipos.gen.ts no está al día: go generate ./internal/contrato")
	}
}

// TestUnionesYOpcionales: lo que el generador deduce de Go y de la etiqueta ts.
func TestUnionesYOpcionales(t *testing.T) {
	ts := Generar()
	for _, quiere := range []string{
		"  estado: 'listo' | 'desconectado' | 'no_instalado'", // EstadoAgente con sus valores
		"  etapa: 'cola' | 'preparacion'",                     // la etiqueta ts
		"  corregible: boolean",
		"  presupuesto?: Presupuesto", // un puntero es opcional
		"  agente?: string",           // Solicitud es parcial
		"  limites: LimitesPeticion",  // contrato.Limites con su nombre de la Web UI
		"  eleccion_herramienta?: 'auto' | 'ninguna' | 'obligatoria' | { nombre: string }",
		"  base: string", // ResumenInstancia incrusta EstadoInstancia
		"  trabajos: Record<string, number>",
	} {
		if !strings.Contains(ts, quiere) {
			t.Errorf("falta %q", quiere)
		}
	}
}

type conNulo struct {
	Nulo     *int `json:"nulo"`
	Opcional *int `json:"opcional,omitempty"`
}

type conMarshal struct{ A int }

func (conMarshal) MarshalJSON() ([]byte, error) { return []byte(`"a"`), nil }

// TestNulosYMarshalers: un puntero sin omitempty sale como null, y un tipo con MarshalJSON propio que el generador no
// conoce es un error, no una interfaz con sus campos.
func TestNulosYMarshalers(t *testing.T) {
	g := &generador{nombreDe: map[reflect.Type]string{}, tipoDe: map[string]reflect.Type{}, hechos: map[reflect.Type]bool{}}
	if campos := strings.Join(g.campos(reflect.TypeFor[conNulo](), false), "; "); campos != "nulo: number | null; opcional?: number" {
		t.Fatalf("campos: %s", campos)
	}
	if err := sinMarshalerPropio(reflect.TypeFor[conMarshal]()); err == nil {
		t.Fatal("un MarshalJSON propio sin literal tiene que dar error")
	}
	if err := sinMarshalerPropio(reflect.TypeFor[nucleo.EleccionHerramienta]()); err != nil {
		t.Fatalf("un literal conocido: %v", err)
	}
}
