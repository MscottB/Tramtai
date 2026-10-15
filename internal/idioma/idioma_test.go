package idioma

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestElegir(t *testing.T) {
	casos := map[[2]string]string{
		{"", ""}:            "es",
		{"", "es-CL"}:       "es",
		{"", "es_ES.UTF-8"}: "es",
		{"", "en-US"}:       "en",
		{"", "fr_FR.UTF-8"}: "en",
		{"", "C.UTF-8"}:     "es",
		{"", "POSIX"}:       "es",
		{"en", "es-CL"}:     "en",
		{"es", "en-US"}:     "es",
	}
	for c, esperado := range casos {
		if r := elegir(c[0], c[1]); r != esperado {
			t.Errorf("elegir(%q, %q) = %q, se esperaba %q", c[0], c[1], r, esperado)
		}
	}
}

func TestTraducir(t *testing.T) {
	anterior := Activo
	defer func() { Activo = anterior }()
	agregar(map[string]string{"hola %s": "hello %s"})
	defer delete(en, "hola %s")
	Activo = "es"
	if r := Tf("hola %s", "Ana"); r != "hola Ana" {
		t.Errorf("en español: %q", r)
	}
	Activo = "en"
	if r := Tf("hola %s", "Ana"); r != "hello Ana" {
		t.Errorf("en inglés: %q", r)
	}
	if r := T("sin traducción"); r != "sin traducción" {
		t.Errorf("un texto sin traducción debe quedar en español: %q", r)
	}
}

var verbo = regexp.MustCompile(`%[-+# 0-9.*\[\]]*[a-zA-Z%]`)

// TestCobertura recorre el código de la CLI, el servicio y la app de escritorio: cada texto pasado a idioma.T o
// idioma.Tf debe ser un literal con traducción al inglés y los mismos verbos de fmt; y no puede sobrar ninguna.
func TestCobertura(t *testing.T) {
	usados := map[string]bool{}
	for _, raiz := range []string{"../../cmd", "../../internal", "../../escritorio"} {
		err := filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
				return err
			}
			if filepath.Base(filepath.Dir(ruta)) == "idioma" {
				return nil
			}
			conjunto := token.NewFileSet()
			archivo, err := parser.ParseFile(conjunto, ruta, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(archivo, func(n ast.Node) bool {
				llamada, ok := n.(*ast.CallExpr)
				if !ok || len(llamada.Args) == 0 {
					return true
				}
				sel, ok := llamada.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "T" && sel.Sel.Name != "Tf") {
					return true
				}
				if paquete, ok := sel.X.(*ast.Ident); !ok || paquete.Name != "idioma" {
					return true
				}
				posicion := conjunto.Position(llamada.Pos())
				texto, ok := literal(llamada.Args[0])
				if !ok {
					t.Errorf("%s: idioma.%s necesita un texto literal", posicion, sel.Sel.Name)
					return true
				}
				usados[texto] = true
				traduccion, ok := en[texto]
				switch {
				case !ok:
					t.Errorf("%s: falta la traducción de %q", posicion, texto)
				case strings.Join(verbo.FindAllString(texto, -1), " ") != strings.Join(verbo.FindAllString(traduccion, -1), " "):
					t.Errorf("%s: los verbos de %q y %q no coinciden", posicion, texto, traduccion)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for texto := range en {
		if !usados[texto] {
			t.Errorf("traducción sin uso: %q", texto)
		}
	}
	for _, texto := range conflictos {
		t.Errorf("dos traducciones distintas para %q", texto)
	}
}

// literal devuelve el valor de un texto literal o de una suma de literales.
func literal(e ast.Expr) (string, bool) {
	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(v.Value)
		return s, err == nil
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}
		a, ok1 := literal(v.X)
		b, ok2 := literal(v.Y)
		return a + b, ok1 && ok2
	case *ast.ParenExpr:
		return literal(v.X)
	}
	return "", false
}
