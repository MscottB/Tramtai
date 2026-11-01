package datos

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestAyudaCargada(t *testing.T) {
	for _, idioma := range []string{"es", "en"} {
		temas := Ayuda(idioma)
		if len(temas) != 19 { // v0.59: más «Conectar tus herramientas»
			t.Fatalf("Ayuda(%q): %d temas, esperaba 19", idioma, len(temas))
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

// frasesSoloWeb son las que solo valen en la Web UI (hallazgo 69): el recorrido guiado, los tranvías del mapa de Inicio,
// el menú lateral, el botón ? y su panel, el selector de arriba y no abrir ninguna terminal. La misma lista que
// web/scripts/terminal-datos.test.ts.
var frasesSoloWeb = []string{`Haz el recorrido`, `tranvía`, `mapa de Inicio`, `menú lateral`, `botón \?`, `\b(el|un) panel\b`,
	`sin abrir ninguna terminal`, `selector de arriba`, `arriba a la derecha`,
	`Take the guided tour`, `\btrams?\b`, `Home map`, `side menu`, `\? button`, `\b(the|a) panel\b`, `without opening a terminal`,
	`selector at the top`, `top right`}

// TestSinFrasesSoloWeb: ni la Ayuda ni los textos que la terminal toma de la Web UI dicen lo que en la terminal no se
// puede hacer: llevan su variante .terminal o se quedan fuera.
func TestSinFrasesSoloWeb(t *testing.T) {
	for _, f := range frasesSoloWeb {
		re := regexp.MustCompile(`(?i)` + f)
		for nombre, datos := range map[string][]byte{"ayuda.json": ayudaJSON, "textos.json": textosJSON} {
			if m := re.Find(datos); m != nil {
				t.Errorf("%s dice %q, que solo vale en la Web UI", nombre, m)
			}
		}
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
