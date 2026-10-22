package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
