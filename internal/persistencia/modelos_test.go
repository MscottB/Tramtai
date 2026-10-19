package persistencia

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
)

// BenchmarkModelosEfectivos mide la consulta con un historial largo: sin el índice parcial crece con la tabla
// (unos 44 ms con 20 000 trabajos); con él, ~1 ms.
func BenchmarkModelosEfectivos(b *testing.B) {
	a, err := Abrir(b.TempDir() + "/b.db")
	if err != nil {
		b.Fatal(err)
	}
	defer a.Cerrar()
	ctx := context.Background()
	for i := 0; i < 20000; i++ {
		tr := nucleo.Trabajo{ID: fmt.Sprintf("t%d", i), Instancia: "global", Estado: nucleo.EnCola, ModeloEnviado: "haiku",
			Solicitud: nucleo.SolicitudEjecucion{Agente: "claude", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}}
		if _, _, err := a.Aceptar(ctx, tr, "", 0); err != nil {
			b.Fatal(err)
		}
	}
	a.db.Exec(`UPDATE trabajos SET modelo_efectivo = 'claude-haiku-4-5'`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.ModelosEfectivos(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

// La consulta de los modelos efectivos usa el índice parcial: así no recorre todo el historial.
func TestModelosEfectivosUsaElIndice(t *testing.T) {
	a := abrir(t)
	filas, err := a.db.Query("EXPLAIN QUERY PLAN " + consultaEfectivos)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()
	var plan strings.Builder
	for filas.Next() {
		var id, padre, libre int
		var detalle string
		if err := filas.Scan(&id, &padre, &libre, &detalle); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detalle + "\n")
	}
	if !strings.Contains(plan.String(), "trabajos_efectivo") || strings.Contains(plan.String(), "TEMP B-TREE") {
		t.Fatalf("la consulta debía usar trabajos_efectivo sin ordenar aparte:\n%s", plan.String())
	}
}
