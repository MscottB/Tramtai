package datos

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAyudaCargada(t *testing.T) {
	for _, idioma := range []string{"es", "en"} {
		temas := Ayuda(idioma)
		if len(temas) != 16 {
			t.Fatalf("Ayuda(%q): %d temas, esperaba 16", idioma, len(temas))
		}
		for _, tema := range temas {
			if tema.ID == "" || tema.Titulo == "" || tema.Resumen == "" || len(tema.Bloques) == 0 {
				t.Errorf("Ayuda(%q): tema incompleto %+v", idioma, tema)
			}
		}
	}
	if Ayuda("fr")[0].Titulo != Ayuda("es")[0].Titulo {
		t.Error("un idioma que no es en usa es")
	}
	if Ayuda("en")[0].Titulo == Ayuda("es")[0].Titulo {
		t.Error("en y es deberían diferir")
	}
}

func TestPlantillas(t *testing.T) {
	for _, idioma := range []string{"es", "en"} {
		lista := Plantillas(idioma)
		if len(lista) != 7 {
			t.Fatalf("Plantillas(%q): %d, esperaba 7", idioma, len(lista))
		}
		for _, p := range lista {
			var esquema map[string]any
			if err := json.Unmarshal(p.Esquema, &esquema); err != nil || esquema["type"] != "object" {
				t.Errorf("%s/%s: esquema no válido: %v", idioma, p.ID, err)
			}
			if p.ID == "" || p.Nombre == "" || p.Instrucciones == "" {
				t.Errorf("%s: plantilla incompleta %+v", idioma, p)
			}
		}
	}
}

func TestTextosMenu(t *testing.T) {
	esperado := map[string]string{
		"menu.inicio": "Inicio", "menu.agentes": "Agentes", "menu.ejecutar": "Probar", "menu.conectar": "Mis APIs",
		"menu.historial": "Historial", "menu.espacios": "Espacios", "menu.configuracion": "Ajustes", "menu.ayuda": "Ayuda",
		"menu.grupo.empezar": "Empezar", "menu.grupo.conectar": "Usar", "menu.grupo.organizar": "Organizar",
	}
	for clave, texto := range esperado {
		if got := Texto("es", clave); got != texto {
			t.Errorf("Texto(es, %s) = %q, esperaba %q", clave, got, texto)
		}
		if Texto("en", clave) == "" || Texto("en", clave) == clave {
			t.Errorf("Texto(en, %s) sin traducir", clave)
		}
	}
	if Texto("es", "no.existe") != "no.existe" {
		t.Error("una clave desconocida vuelve tal cual")
	}
}

func TestTextofRellenaLosMarcadores(t *testing.T) {
	if got := Textof("es", "agentes.usaClave", "cli", "Claude Code"); strings.Contains(got, "{") || strings.Count(got, "Claude Code") != 2 {
		t.Errorf("Textof no rellena todos los marcadores: %q", got)
	}
	if got := Textof("en", "agentes.filtroListos", "n", "3"); got != "Ready (3)" {
		t.Errorf("Textof en: %q", got)
	}
}
