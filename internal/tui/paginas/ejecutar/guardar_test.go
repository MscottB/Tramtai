package ejecutar_test

import (
	"strings"
	"testing"

	"tramtai/internal/contrato"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de «Guardar como API», de abrir una API en Probar y de «Ver como código».

// TestGuardarComoApi: g pide el nombre, crea la API con la receta y enseña su clave una vez; después g la actualiza.
func TestGuardarComoApi(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	escribir(t, a, tx("receta.instrucciones"), "Responde en una frase")
	a.Pulsar("g")
	if !a.P.EnCampo() || !strings.Contains(a.Vista(), tx("ejecutar.guardarApi")) {
		t.Fatalf("g abre «Guardar como API»:\n%s", a.Vista())
	}
	a.Escribir("Mal Nombre")
	if fuera := a.Pulsar("ctrl+s"); len(fuera) != 0 || !a.P.EnCampo() {
		t.Fatalf("un nombre que no vale no se guarda: %v", fuera)
	}
	if v := a.Vista(); !strings.Contains(v, "Minúsculas, números") {
		t.Fatalf("dice la regla del nombre:\n%s", v)
	}
	a.Pulsar("end", "ctrl+u")
	a.Escribir("mi-app")
	a.Pulsar("ctrl+s")
	a.Hasta(func() bool {
		v := a.Vista()
		return !a.P.EnCampo() && strings.Contains(v, tx("ejecutar.apiCreada", "nombre", "mi-app")) && strings.Contains(v, "/openai/v1")
	})
	if v := a.Vista(); a.P.EnCampo() || !strings.Contains(v, tx("ejecutar.apiCreada", "nombre", "mi-app")) || !strings.Contains(v, "/openai/v1") {
		t.Fatalf("crea la API y enseña su URL:\n%s", v)
	}
	clave, _ := copiado(a.Pulsar("y"))
	if len(clave) < 20 || !strings.Contains(a.Vista(), clave[:20]) {
		t.Fatalf("enseña la clave y y la copia: %q\n%s", clave, a.Vista())
	}
	l, _ := cli.Puntos(ctx, id)
	if len(l.Puntos) != 1 || l.Puntos[0].Instrucciones != "Responde en una frase" {
		t.Fatalf("la API lleva la receta: %+v", l.Puntos)
	}
	a.Pulsar("esc")
	if v := a.Vista(); strings.Contains(v, tx("ejecutar.apiCreada", "nombre", "mi-app")) || strings.Contains(v, "/openai/v1") || strings.Contains(v, clave) {
		t.Fatalf("Esc cierra y olvida la clave:\n%s", v)
	}
	if v := a.Vista(); !strings.Contains(v, "Editando la API mi-app") {
		t.Fatalf("dice qué API se edita:\n%s", v)
	}
	if pendiente(a) != "" {
		t.Fatalf("recién guardada no hay cambios: %q", pendiente(a))
	}
	poner(t, a, tx("receta.instrucciones"), "Responde en dos frases")
	if !strings.Contains(pendiente(a), "La API mi-app tiene cambios sin guardar") {
		t.Fatalf("salir avisa de los cambios sin guardar: %q", pendiente(a))
	}
	fuera := a.HastaSalir(func() { a.Pulsar("g") }, prueba.HayAviso)
	if av, ok := aviso(fuera); !ok || av.Texto != "Cambios guardados en mi-app" {
		t.Fatalf("g actualiza la API: %v", fuera)
	}
	l, _ = cli.Puntos(ctx, id)
	if l.Puntos[0].Instrucciones != "Responde en dos frases" || pendiente(a) != "" {
		t.Fatalf("con la receta nueva, y ya sin cambios: %q %q", l.Puntos[0].Instrucciones, pendiente(a))
	}
	a.Pulsar("d")
	if strings.Contains(a.Vista(), "Editando la API") {
		t.Fatal("d deja de editarla")
	}
}

// TestConPunto: abrir una API en Probar trae su receta y la edita.
func TestConPunto(t *testing.T) {
	cli, id := prueba.Servicio(t)
	if _, _, err := cli.CrearPunto(ctx, id, contratoPunto()); err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	a.Entregar(pagina.ConPunto{Nombre: "notas"})
	a.HastaVer("Editando la API notas")
	if !strings.Contains(a.Vista(), "Editando la API notas") || valor(t, a, tx("receta.instrucciones")) != "Resume" ||
		!marcado(t, a, tx("receta.salidaEstructurada")) || valor(t, a, tx("receta.modelo")) != "simulado" {
		t.Fatalf("trae la receta de la API:\n%s", a.Vista())
	}
}

// TestCodigoComoLaPrueba: k enseña la prueba de ahora como código, con la URL del servicio y el marcador de la clave.
func TestCodigoComoLaPrueba(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	poner(t, a, tx("ejecutar.mensaje"), "Hola desde la prueba")
	a.Pulsar("k")
	a.HastaVer("[Python", "curl", "JavaScript")
	v := a.Vista()
	if !strings.Contains(v, "[Python") || !strings.Contains(v, "curl") || !strings.Contains(v, "JavaScript") {
		t.Fatalf("k abre «Ver como código» con curl, Python y JavaScript, Python primero:\n%s", v)
	}
	a.Pulsar("left")
	curl, _ := copiado(a.Pulsar("y"))
	if !strings.Contains(a.Vista(), "[curl") || !strings.Contains(curl, cli.Base+"/api/v1/trabajos") ||
		!strings.Contains(curl, "Bearer [tu clave]") || !strings.Contains(curl, "Hola desde la prueba") {
		t.Fatalf("el código de la prueba:\n%s\n%s", a.Vista(), curl)
	}
	a.Pulsar("right", "right")
	if js, _ := copiado(a.Pulsar("y")); !strings.Contains(a.Vista(), "[JavaScript") || js == curl || !strings.Contains(js, "Hola desde la prueba") {
		t.Fatalf("→ pasa al siguiente y y copia el suyo: %q", js)
	}
	a.Pulsar("esc")
	if strings.Contains(a.Vista(), "[Python") || strings.Contains(a.Vista(), "[JavaScript") {
		t.Fatal("Esc lo cierra")
	}
}

func contratoPunto() contrato.ConfigPunto {
	return contrato.ConfigPunto{Nombre: "notas", Instrucciones: "Resume", Agente: "simulado",
		Esquema: map[string]any{"type": "object", "properties": map[string]any{"r": map[string]any{"type": "string"}}}}
}
