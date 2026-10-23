package ejecutar

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
	p := abierta(t, cli, id)
	escribir(p, "instrucciones", "Responde en una frase")
	pulsar(p, "esc", "g")
	if !p.EnCampo() || !strings.Contains(p.Vista(), p.tx("ejecutar.guardarApi")) {
		t.Fatal("g abre «Guardar como API»")
	}
	escribir2 := func(texto string) {
		for _, r := range texto {
			_, cmd := p.Update(tecla(string(r)))
			correr(p, cmd)
		}
	}
	escribir2("Mal Nombre")
	if fuera := pulsar(p, "ctrl+s"); len(fuera) != 0 || !p.EnCampo() {
		t.Fatalf("un nombre que no vale no se guarda: %v", fuera)
	}
	if v := p.Vista(); !strings.Contains(v, "Minúsculas, números") {
		t.Fatalf("dice la regla del nombre:\n%s", v)
	}
	p.guardar.Poner("nombre", "")
	escribir2("mi-app")
	pulsar(p, "ctrl+s")
	if p.guardar != nil || p.creada == nil || p.api != "mi-app" {
		t.Fatalf("crea la API y pasa a editarla: %v %+v %q", p.guardar, p.creada, p.api)
	}
	if p.creada.clave == "" || !strings.Contains(p.caja.Vista(), p.creada.clave[:20]) || !strings.Contains(p.caja.Vista(), "/openai/v1") {
		t.Fatalf("enseña la URL y la clave:\n%s", p.caja.Vista())
	}
	l, _ := cli.Puntos(ctx, id)
	if len(l.Puntos) != 1 || l.Puntos[0].Instrucciones != "Responde en una frase" {
		t.Fatalf("la API lleva la receta: %+v", l.Puntos)
	}
	pulsar(p, "esc")
	if strings.Contains(p.Vista(), "API mi-app lista") || strings.Contains(p.Vista(), "/openai/v1") {
		t.Fatal("Esc cierra y olvida la clave")
	}
	if v := p.Vista(); !strings.Contains(v, "Editando la API mi-app") {
		t.Fatalf("dice qué API se edita:\n%s", v)
	}
	if p.Pendiente() != "" {
		t.Fatalf("recién guardada no hay cambios: %q", p.Pendiente())
	}
	p.form.Poner("instrucciones", "Responde en dos frases")
	if !strings.Contains(p.Pendiente(), "La API mi-app tiene cambios sin guardar") {
		t.Fatalf("salir avisa de los cambios sin guardar: %q", p.Pendiente())
	}
	fuera := pulsar(p, "g")
	if a, ok := aviso(fuera); !ok || a.Texto != "API mi-app actualizada" {
		t.Fatalf("g actualiza la API: %v", fuera)
	}
	l, _ = cli.Puntos(ctx, id)
	if l.Puntos[0].Instrucciones != "Responde en dos frases" || p.Pendiente() != "" {
		t.Fatalf("con la receta nueva, y ya sin cambios: %q %q", l.Puntos[0].Instrucciones, p.Pendiente())
	}
	pulsar(p, "d")
	if p.api != "" {
		t.Fatal("d deja de editarla")
	}
}

// TestConPunto: abrir una API en Probar trae su receta y la edita.
func TestConPunto(t *testing.T) {
	cli, id := prueba.Servicio(t)
	if _, _, err := cli.CrearPunto(ctx, id, contratoPunto()); err != nil {
		t.Fatal(err)
	}
	p := abierta(t, cli, id)
	_, cmd := p.Update(pagina.ConPunto{Nombre: "notas"})
	correr(p, cmd)
	if p.api != "notas" || p.form.Valor("instrucciones") != "Resume" || !p.form.Marcado("estructurada") || p.valorOpcion("modelo") != "simulado" {
		t.Fatalf("trae la receta de la API: %q %+v", p.api, p.leer())
	}
}

// TestCodigoComoLaPrueba: k enseña la prueba de ahora como código, con la URL del servicio y el marcador de la clave.
func TestCodigoComoLaPrueba(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	p.form.Poner("mensaje", "Hola desde la prueba")
	pulsar(p, "k")
	if p.codigo == nil || len(p.codigo.ejemplos) != 3 {
		t.Fatal("k abre «Ver como código» con curl, Python y JavaScript")
	}
	v := p.Vista()
	if !strings.Contains(v, "[Python") || !strings.Contains(p.codigo.ejemplos[0].Codigo, cli.Base+"/api/v1/trabajos") ||
		!strings.Contains(p.codigo.ejemplos[0].Codigo, "Bearer [tu clave]") || !strings.Contains(p.codigo.ejemplos[0].Codigo, "Hola desde la prueba") {
		t.Fatalf("el código de la prueba:\n%s\n%s", v, p.codigo.ejemplos[0].Codigo)
	}
	pulsar(p, "right")
	if p.codigo.i != 2 {
		t.Fatal("→ pasa al siguiente")
	}
	if fuera := pulsar(p, "y"); len(fuera) == 0 || fuera[0] != portapapeles(p.codigo.ejemplos[2].Codigo) {
		t.Fatalf("y copia el código: %v", fuera)
	}
	pulsar(p, "esc")
	if strings.Contains(p.Vista(), "[Python") {
		t.Fatal("Esc lo cierra")
	}
}

func contratoPunto() contrato.ConfigPunto {
	return contrato.ConfigPunto{Nombre: "notas", Instrucciones: "Resume", Agente: "simulado",
		Esquema: map[string]any{"type": "object", "properties": map[string]any{"r": map[string]any{"type": "string"}}}}
}

func punto(nombre string) contrato.EstadoPunto { return contrato.EstadoPunto{Nombre: nombre} }
