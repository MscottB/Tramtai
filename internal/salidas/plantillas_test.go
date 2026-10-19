package salidas

import (
	"encoding/json"
	"os"
	"testing"
)

// TestPlantillasDeLaWebUI compila el esquema de cada plantilla de «Respuesta con forma» con la misma biblioteca que
// valida, y valida su ejemplo. La copia la escribe la prueba de web/src/esquemaCampos.test.ts.
func TestPlantillasDeLaWebUI(t *testing.T) {
	datos, err := os.ReadFile("testdata/plantillas.json")
	if err != nil {
		t.Fatal(err)
	}
	var plantillas []struct {
		ID, Idioma string
		Esquema    json.RawMessage
		Ejemplo    json.RawMessage
	}
	if err := json.Unmarshal(datos, &plantillas); err != nil {
		t.Fatal(err)
	}
	if len(plantillas) != 14 {
		t.Fatalf("se esperaban 7 plantillas en 2 idiomas: %d", len(plantillas))
	}
	for _, p := range plantillas {
		if _, err := Compilar(p.Esquema); err != nil {
			t.Errorf("%s (%s): %v", p.ID, p.Idioma, err)
		}
		if err := Validar(p.Esquema, string(p.Ejemplo)); err != nil {
			t.Errorf("%s (%s): el ejemplo no cumple: %v", p.ID, p.Idioma, err)
		}
	}
}
