package agentes

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSinNombresFuera falla si un archivo Go fuera del catálogo nombra a un agente: lo que depende de un agente se
// pregunta al catálogo (su ficha). Quedan fuera las pruebas, los textos traducidos (idioma) y pruebas/.
func TestSinNombresFuera(t *testing.T) {
	raiz := filepath.Join("..", "..")
	err := filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativa := filepath.ToSlash(strings.TrimPrefix(ruta, raiz+string(filepath.Separator)))
		if d.IsDir() {
			switch relativa {
			case "internal/agentes", "internal/idioma", "internal/apuntar", "pruebas", "web", "node_modules", ".git", "graphify-out":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
			return nil
		}
		datos, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		for i, linea := range strings.Split(string(datos), "\n") {
			if strings.Contains(linea, "predeterminados.agente") || strings.HasPrefix(strings.TrimSpace(linea), "//") {
				continue
			}
			for _, f := range Todos() {
				if strings.Contains(linea, `"`+f.Nombre+`"`) {
					t.Errorf("%s:%d nombra a %s: pregúntale al catálogo de agentes", relativa, i+1, f.Nombre)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
