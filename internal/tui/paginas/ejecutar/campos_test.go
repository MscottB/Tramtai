package ejecutar

import (
	"encoding/json"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/paginas/prueba"
)

// elegir cambia un campo de opción con → hasta que vale lo que se quiere.
func elegir(t *testing.T, p *Pagina, id, valor string) {
	t.Helper()
	p.form.Enfocar(id)
	p.editando = true
	for i := 0; i < 10 && p.valorOpcion(id) != valor; i++ {
		pulsar(p, "right")
	}
	if p.valorOpcion(id) != valor {
		t.Fatalf("%s no llega a %q: %q", id, valor, p.valorOpcion(id))
	}
	pulsar(p, "esc")
}

// TestModoCampos: la salida estructurada se edita como campos (como la Web UI) y pasa a JSON con la forma de esquemaCampos;
// cambiar de modo no pierde lo escrito y, si el esquema no se representa en campos, se queda en JSON y lo avisa.
func TestModoCampos(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	conEstructurada(p)
	if p.modo != modoCampos || p.form.Valor("c0.nombre") != "resumen" || p.valorOpcion("c0.tipo") != "texto" {
		t.Fatalf("empieza en campos, con el campo de ejemplo: %q %q", p.modo, p.form.Valor("c0.nombre"))
	}
	if p.form.Enfocar("esquema"); p.form.Foco() == "esquema" {
		t.Fatal("en campos no se ve el JSON")
	}
	// El nombre no lleva espacios; un tipo nuevo trae sus filas.
	p.form.Poner("c0.nombre", "")
	escribir(p, "c0.nombre", "mi titulo")
	pulsar(p, "esc")
	if p.form.Valor("c0.nombre") != "mi_titulo" {
		t.Fatalf("los espacios pasan a _: %q", p.form.Valor("c0.nombre"))
	}
	elegir(t, p, "c0.tipo", "opcion")
	escribir(p, "c0.opciones", "alta, baja")
	pulsar(p, "esc")
	quiere := aEsquema([]campoEsquema{{Nombre: "mi_titulo", Tipo: "opcion", Obligatorio: true, Opciones: []string{"alta", "baja"}}})
	if got := compacto(t, p.leer().esquema); got != quiere {
		t.Fatalf("el esquema es el de esquemaCampos:\n%s\n%s", got, quiere)
	}
	// n añade un campo detrás y lo edita; un objeto trae un subcampo; x quita con confirmación.
	p.form.Enfocar("c0.nombre")
	pulsar(p, "n")
	if p.form.Foco() != "c1.nombre" || !p.EnCampo() {
		t.Fatalf("n añade y edita el campo nuevo: %s", p.form.Foco())
	}
	escribir(p, "c1.nombre", "datos")
	pulsar(p, "esc")
	elegir(t, p, "c1.tipo", "objeto")
	if p.form.Enfocar("c1.0.nombre"); p.form.Foco() != "c1.0.nombre" {
		t.Fatal("un objeto trae un subcampo")
	}
	if v := p.Vista(); !strings.Contains(v, "2.1 · Nombre") {
		t.Fatalf("el subcampo se numera bajo su objeto:\n%s", v)
	}
	if !strings.Contains(p.Vista(), "Un campo no tiene nombre") {
		t.Fatalf("el problema de los campos se ve:\n%s", p.Vista())
	}
	if fuera := pulsar(p, "ctrl+s"); len(fuera) != 0 || p.trabajo != nil || p.form.Foco() != "modo" {
		t.Fatalf("con un campo sin nombre no se envía: %v %s", fuera, p.form.Foco())
	}
	p.form.Enfocar("c1.nombre")
	fuera := pulsar(p, "x")
	if len(fuera) != 1 || !strings.Contains(fuera[0].(comp.Pregunta).Texto, "datos") {
		t.Fatalf("x pregunta antes de quitar: %v", fuera)
	}
	p.Update(comp.Confirmado{})
	if p.form.Enfocar("c1.nombre"); p.form.Foco() == "c1.nombre" || len(p.campos) != 1 {
		t.Fatalf("confirmar lo quita: %+v", p.campos)
	}
	// A JSON: el mismo esquema, con sangría.
	elegir(t, p, "modo", modoJSON)
	if p.form.Valor("esquema") != sangrar([]byte(quiere)) {
		t.Fatalf("pasar a JSON no pierde nada:\n%s", p.form.Valor("esquema"))
	}
	// Un esquema avanzado se queda en JSON y lo avisa.
	avanzado := `{"type":"object","additionalProperties":false,"properties":{"a":{"type":"string","pattern":"^x"}}}`
	p.form.Poner("esquema", avanzado)
	p.form.Enfocar("modo")
	p.editando = true
	if a, ok := aviso(pulsar(p, "right")); !ok || !strings.Contains(a.Texto, "opciones avanzadas") || p.modo != modoJSON || p.form.Valor("esquema") != avanzado {
		t.Fatalf("se queda en JSON con su aviso: %+v %q", a, p.modo)
	}
	pulsar(p, "esc")
	// Uno que sí se representa vuelve a campos; y lo enviado es lo que generan.
	p.form.Poner("esquema", `{"type":"object","properties":{"total":{"type":"number"}},"required":["total"],"additionalProperties":false}`)
	elegir(t, p, "modo", modoCampos)
	if p.form.Valor("c0.nombre") != "total" || p.valorOpcion("c0.tipo") != "numero" {
		t.Fatalf("vuelve a campos con los del JSON: %+v", p.campos)
	}
	p.form.Poner("mensaje", `{"total":3}`)
	pulsar(p, "ctrl+s")
	hasta(t, p, func() bool { return p.final != nil })
	tr, _ := cli.Trabajo(ctx, id, p.final.ID)
	if tr.Estado != nucleo.Completado || string(tr.Solicitud.Salida.Esquema) != compacto(t, p.leer().esquema) {
		t.Fatalf("se envía el esquema de los campos: %s %s", tr.Estado, tr.Solicitud.Salida.Esquema)
	}
}

// TestRecetaAvanzadaEnJSON: una receta que llega con un esquema que los campos no representan se abre en JSON.
func TestRecetaAvanzadaEnJSON(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	s := nucleo.SolicitudEjecucion{Salida: nucleo.ConfigSalida{Formato: "json", Esquema: json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`)}}
	p.cargarReceta(recetaDeSolicitud(s), "hola")
	if p.modo != modoJSON || !strings.Contains(p.form.Valor("esquema"), `"a"`) {
		t.Fatalf("se abre en JSON: %q %q", p.modo, p.form.Valor("esquema"))
	}
	p.form.Enfocar("esquema")
	if v := p.Vista(); !strings.Contains(v, "opciones avanzadas") {
		t.Fatalf("y lo explica:\n%s", v)
	}
}

func compacto(t *testing.T, texto string) string {
	t.Helper()
	var v json.RawMessage
	if err := json.Unmarshal([]byte(texto), &v); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	return string(b)
}
