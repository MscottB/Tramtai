package nucleo

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// soloDeLaWeb son los códigos que inventa la propia Web UI (api/cliente.ts) y no salen del servicio.
var soloDeLaWeb = map[string]bool{"sesion_caducada": true}

// TestCodigosConTextoEnLaWeb: cada código de error del servicio tiene su texto error.<código> en los dos idiomas de la
// Web UI (si no, tituloError enseña el identificador en bruto), y la Web UI no tiene textos de códigos que ya no existen.
func TestCodigosConTextoEnLaWeb(t *testing.T) {
	codigos := codigosDeError(t)
	if len(codigos) < 20 {
		t.Fatalf("solo se encontraron %d códigos en errores.go", len(codigos))
	}
	clave := regexp.MustCompile(`'error\.([a-z_]+)':`)
	for _, lengua := range []string{"es", "en"} {
		fuente, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "idiomas", lengua+".ts"))
		if err != nil {
			t.Fatal(err)
		}
		enLaWeb := map[string]bool{}
		for _, m := range clave.FindAllStringSubmatch(string(fuente), -1) {
			enLaWeb[m[1]] = true
		}
		for c := range codigos {
			if !enLaWeb[c] {
				t.Errorf("%s.ts no tiene error.%s", lengua, c)
			}
		}
		for c := range enLaWeb {
			if !codigos[c] && !soloDeLaWeb[c] {
				t.Errorf("%s.ts tiene error.%s, que el servicio no usa", lengua, c)
			}
		}
	}
}

// codigosDeError son los valores de las constantes CodigoError de errores.go.
func codigosDeError(t *testing.T) map[string]bool {
	t.Helper()
	archivo, err := parser.ParseFile(token.NewFileSet(), "errores.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	codigos := map[string]bool{}
	ast.Inspect(archivo, func(n ast.Node) bool {
		if v, ok := n.(*ast.ValueSpec); ok {
			if tipo, ok := v.Type.(*ast.Ident); ok && tipo.Name == "CodigoError" {
				for _, valor := range v.Values {
					if lit, ok := valor.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						texto, _ := strconv.Unquote(lit.Value)
						codigos[texto] = true
					}
				}
			}
		}
		return true
	})
	return codigos
}
