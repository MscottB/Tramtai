package ejecutar

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// enEspanol pone el idioma activo en español mientras dura la prueba: los ejemplos guardados están en español.
func enEspanol(t *testing.T) {
	antes := idioma.Activo
	idioma.Activo = "es"
	t.Cleanup(func() { idioma.Activo = antes })
}

// TestEjemplosComoLaWeb: con la prueba fija, el código de curl, Python y JavaScript es el de ejemplos.ts, byte a byte.
func TestEjemplosComoLaWeb(t *testing.T) {
	enEspanol(t)
	datos, err := os.ReadFile("../../datos/testdata/ejemplos.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Entrada struct {
			URL, Clave string
			Solicitud  nucleo.SolicitudEjecucion
		}
		Ejemplos []struct{ ID, Codigo string }
	}
	if err := json.Unmarshal(datos, &f); err != nil {
		t.Fatal(err)
	}
	lista := Ejemplos(f.Entrada.URL, f.Entrada.Clave, f.Entrada.Solicitud)
	if len(lista) != len(f.Ejemplos) {
		t.Fatalf("%d ejemplos, la Web UI da %d", len(lista), len(f.Ejemplos))
	}
	for i, quiere := range f.Ejemplos {
		if lista[i].ID != quiere.ID {
			t.Fatalf("el ejemplo %d es %s, no %s", i, lista[i].ID, quiere.ID)
		}
		if lista[i].Codigo != quiere.Codigo {
			a, b := strings.Split(lista[i].Codigo, "\n"), strings.Split(quiere.Codigo, "\n")
			for j := range min(len(a), len(b)) {
				if a[j] != b[j] {
					t.Fatalf("%s, línea %d:\n  Go:  %q\n  Web: %q", quiere.ID, j+1, a[j], b[j])
				}
			}
			t.Fatalf("%s: %d líneas, la Web UI da %d", quiere.ID, len(a), len(b))
		}
	}
}

// TestEjemplosConArchivos: con archivos, cada código los sube antes y pone sus cargas en la solicitud.
func TestEjemplosConArchivos(t *testing.T) {
	enEspanol(t)
	s := nucleo.SolicitudEjecucion{Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "Resume 'esto'"}}, Cargas: []string{"informe.pdf"},
		Ejecucion: nucleo.ConfigEjecucion{TiempoLimiteSegundos: 60}}
	lista := Ejemplos("http://127.0.0.1:1", "[tu clave]", s)
	for _, e := range lista {
		if strings.Contains(e.Codigo, marcaCargas) || !strings.Contains(e.Codigo, "1. Sube cada archivo") || !strings.Contains(e.Codigo, "informe.pdf") {
			t.Fatalf("%s sin el paso de subida o con la marca:\n%s", e.ID, e.Codigo)
		}
	}
	if !strings.Contains(lista[0].Codigo, `"cargas": ['"$CARGAS"']`) || !strings.Contains(lista[0].Codigo, `'\''esto'\''`) {
		t.Fatalf("curl expande la variable y escapa las comillas:\n%s", lista[0].Codigo)
	}
	if !strings.Contains(lista[1].Codigo, `"cargas": cargas,`) || !strings.Contains(lista[1].Codigo, `"tiempo_limite_segundos": 60,`) {
		t.Fatalf("Python usa la lista de cargas:\n%s", lista[1].Codigo)
	}
	if !strings.Contains(lista[2].Codigo, `"cargas": cargas,`) || !strings.HasPrefix(lista[2].Codigo, "import { readFile }") {
		t.Fatalf("JavaScript lee los archivos:\n%s", lista[2].Codigo)
	}
}
