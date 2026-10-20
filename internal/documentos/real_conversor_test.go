//go:build conversor

// Preparación real del conversor con uv y conversión de las muestras: go test -tags conversor -run TestConversorReal
// -v ./internal/documentos (con TRAMTAI_UV o uv en el PATH). La corre el CI en Windows, Linux y macOS.
package documentos

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConversorReal(t *testing.T) {
	uv := os.Getenv("TRAMTAI_UV")
	if uv == "" {
		var err error
		if uv, err = BuscarUV(); err != nil {
			t.Fatal(err)
		}
	}
	raiz := t.TempDir()
	dir := filepath.Join(raiz, "conversor")
	ctx, cancelar := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancelar()
	if err := PrepararEn(ctx, dir, uv, func(etapa string, progreso int) { t.Logf("%s %d%%", etapa, progreso) }); err != nil {
		t.Fatal(err)
	}
	// Como en el servicio: el conversor preparado en la raíz del usuario gana al markitdown del PATH.
	c := &Conversor{Comando: "markitdown", Raiz: raiz, TiempoLimite: 2 * time.Minute, MaximoSalida: 1 << 20,
		Limites: Limites{MaximoBytes: 1 << 20}}
	for _, nombre := range []string{"muestra.pdf", "muestra.docx", "muestra.xlsx"} {
		ruta, _ := filepath.Abs(filepath.Join("testdata", nombre))
		d, err := c.Convertir(ctx, []Entrada{{Ruta: ruta, Nombre: nombre}})
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		if !strings.Contains(d.Texto, "Tramtai muestra") {
			t.Fatalf("%s no trae su texto: %q", nombre, d.Texto)
		}
	}
}
