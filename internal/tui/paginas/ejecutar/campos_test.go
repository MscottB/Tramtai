package ejecutar_test

import (
	"encoding/json"
	"strings"
	"testing"

	"tramtai/internal/cliente"
	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

// campo es la etiqueta de una fila de un campo del esquema: «1 · Nombre», «2.1 · Tipo».
func campo(numero, clave string) string { return numero + " · " + tx("campos."+clave) }

// esquemaEnviado manda la prueba y devuelve, compacto, el esquema que recibió el servicio; vuelve a la receta.
func esquemaEnviado(t *testing.T, a *prueba.App, cli *cliente.Cliente, id string) string {
	t.Helper()
	enviar(a)
	terminada(t, a)
	a.Pulsar("left")
	return string(ultimo(t, cli, id).Solicitud.Salida.Esquema)
}

// TestModoCampos: la salida estructurada se edita como campos (como la Web UI) y pasa a JSON con la forma de esquemaCampos;
// cambiar de modo no pierde lo escrito y, si el esquema no se representa en campos, se queda en JSON y lo avisa.
func TestModoCampos(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	poner(t, a, tx("ejecutar.mensaje"), "hola")
	conEstructurada(t, a)
	if valor(t, a, tx("receta.modo")) != tx("receta.modoCampos") || valor(t, a, campo("1", "nombre")) != "resumen" ||
		valor(t, a, campo("1", "tipo")) != tx("campos.tipo.texto") {
		t.Fatalf("empieza en campos, con el campo de ejemplo:\n%s", a.Vista())
	}
	for _, l := range lineas(a) {
		if sinMarcas(l) == tx("receta.esquema") {
			t.Fatalf("en campos no se ve el JSON:\n%s", a.Vista())
		}
	}
	// El nombre no lleva espacios; un tipo nuevo trae sus filas.
	editar(t, a, campo("1", "nombre"))
	vaciar(a)
	a.Escribir("mi titulo")
	a.Pulsar("esc")
	if v := valor(t, a, campo("1", "nombre")); v != "mi_titulo" {
		t.Fatalf("los espacios pasan a _: %q", v)
	}
	elegir(t, a, campo("1", "tipo"), tx("campos.tipo.opcion"))
	escribir(t, a, campo("1", "opciones"), "alta, baja")
	quiere := `{"type":"object","properties":{"mi_titulo":{"type":"string","enum":["alta","baja"]}},"required":["mi_titulo"],"additionalProperties":false}`
	if got := esquemaEnviado(t, a, cli, id); got != quiere {
		t.Fatalf("el esquema es el de esquemaCampos:\n%s\n%s", got, quiere)
	}
	// n añade un campo detrás y lo edita; un objeto trae un subcampo; x quita con confirmación.
	enfocar(t, a, campo("1", "nombre"))
	a.Pulsar("n")
	if foco(a) != campo("2", "nombre") || !a.P.EnCampo() {
		t.Fatalf("n añade y edita el campo nuevo: %s", foco(a))
	}
	a.Escribir("datos")
	a.Pulsar("esc")
	elegir(t, a, campo("2", "tipo"), tx("campos.tipo.objeto"))
	if v := a.Vista(); !strings.Contains(v, campo("2.1", "nombre")) {
		t.Fatalf("un objeto trae un subcampo, que se numera bajo su objeto:\n%s", v)
	}
	if !strings.Contains(a.Vista(), "Un campo no tiene nombre") {
		t.Fatalf("el problema de los campos se ve:\n%s", a.Vista())
	}
	antes := cuantos(t, cli, id)
	if fuera := enviar(a); len(fuera) != 0 || cuantos(t, cli, id) != antes || foco(a) != tx("receta.modo") {
		t.Fatalf("con un campo sin nombre no se envía: %v %s", fuera, foco(a))
	}
	enfocar(t, a, campo("2", "nombre"))
	q, ok := pregunta(a.Pulsar("x"))
	if !ok || !strings.Contains(q.Texto, "datos") {
		t.Fatalf("x pregunta antes de quitar: %+v", q)
	}
	a.Entregar(comp.Confirmado{})
	if strings.Contains(a.Vista(), campo("2", "nombre")) {
		t.Fatalf("confirmar lo quita:\n%s", a.Vista())
	}
	// A JSON: el mismo esquema, con sangría.
	aJSON(t, a)
	if !strings.Contains(a.Vista(), `"type": "object"`) || esquemaEnviado(t, a, cli, id) != quiere {
		t.Fatalf("pasar a JSON no pierde nada:\n%s", a.Vista())
	}
	// Un esquema avanzado se queda en JSON y lo avisa.
	avanzado := `{"type":"object","additionalProperties":false,"properties":{"a":{"type":"string","pattern":"^x"}}}`
	poner(t, a, tx("receta.esquema"), avanzado)
	editar(t, a, tx("receta.modo"))
	if av, ok := aviso(a.Pulsar("right")); !ok || !strings.Contains(av.Texto, "opciones avanzadas") || valor(t, a, tx("receta.modo")) != tx("receta.modoJson") {
		t.Fatalf("se queda en JSON con su aviso: %+v\n%s", av, a.Vista())
	}
	a.Pulsar("esc")
	if got := esquemaEnviado(t, a, cli, id); got != avanzado {
		t.Fatalf("y el esquema no cambia: %s", got)
	}
	// Uno que sí se representa vuelve a campos; y lo enviado es lo que generan.
	total := `{"type":"object","properties":{"total":{"type":"number"}},"required":["total"],"additionalProperties":false}`
	poner(t, a, tx("receta.esquema"), total)
	elegir(t, a, tx("receta.modo"), tx("receta.modoCampos"))
	if valor(t, a, campo("1", "nombre")) != "total" || valor(t, a, campo("1", "tipo")) != tx("campos.tipo.numero") {
		t.Fatalf("vuelve a campos con los del JSON:\n%s", a.Vista())
	}
	poner(t, a, tx("ejecutar.mensaje"), `{"total":3}`)
	enviar(a)
	if e := terminada(t, a); e != nucleo.Completado || string(ultimo(t, cli, id).Solicitud.Salida.Esquema) != total {
		t.Fatalf("se envía el esquema de los campos: %s %s", e, ultimo(t, cli, id).Solicitud.Salida.Esquema)
	}
}

// TestSubirBajarYEjemplo: como los botones de CamposEsquema.vue en Probar, Alt+↑ y Alt+↓ mueven el campo en su lista (y
// el esquema sigue ese orden); v enseña el ejemplo de respuesta de EditorReceta.
func TestSubirBajarYEjemplo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	poner(t, a, tx("ejecutar.mensaje"), "hola")
	conEstructurada(t, a)
	enfocar(t, a, campo("1", "nombre"))
	a.Pulsar("n")
	a.Escribir("total")
	a.Pulsar("esc")
	elegir(t, a, campo("2", "tipo"), tx("campos.tipo.numero"))
	enfocar(t, a, campo("2", "tipo"))
	a.Pulsar("alt+up")
	if valor(t, a, campo("1", "nombre")) != "total" || valor(t, a, campo("2", "nombre")) != "resumen" || foco(a) != campo("1", "tipo") {
		t.Fatalf("Alt+↑ sube el campo y el foco lo sigue:\n%s", a.Vista())
	}
	if e := esquemaEnviado(t, a, cli, id); !strings.HasPrefix(e, `{"type":"object","properties":{"total":`) {
		t.Fatalf("el esquema sigue el orden de los campos: %s", e)
	}
	enfocar(t, a, campo("1", "tipo"))
	if a.Pulsar("alt+up"); valor(t, a, campo("1", "nombre")) != "total" {
		t.Fatal("el primero no sube más")
	}
	a.Pulsar("alt+down")
	if valor(t, a, campo("2", "nombre")) != "total" || foco(a) != campo("2", "tipo") {
		t.Fatalf("Alt+↓ lo baja:\n%s", a.Vista())
	}
	a.Pulsar("v")
	if v := a.Vista(); !strings.Contains(v, "Ver un ejemplo de respuesta") || !strings.Contains(v, `"resumen": "…"`) || !strings.Contains(v, `"total": 0`) {
		t.Fatalf("v enseña el ejemplo de respuesta:\n%s", v)
	}
	a.Pulsar("esc")
	if strings.Contains(a.Vista(), "Ver un ejemplo de respuesta") {
		t.Fatal("Esc lo cierra")
	}
}

// TestRecetaAvanzadaEnJSON: una receta que llega con un esquema que los campos no representan se abre en JSON.
func TestRecetaAvanzadaEnJSON(t *testing.T) {
	cli, id := prueba.Servicio(t)
	tr, err := cli.Crear(ctx, id, nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}},
		Salida: nucleo.ConfigSalida{Formato: "json", Esquema: json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`)}})
	if err != nil {
		t.Fatal(err)
	}
	a := abierta(t, cli, id)
	a.Entregar(pagina.SeguirTrabajo{ID: tr.ID})
	a.Pulsar("left")
	if valor(t, a, tx("receta.modo")) != tx("receta.modoJson") || !strings.Contains(valor(t, a, tx("receta.esquema")), `"a"`) {
		t.Fatalf("se abre en JSON:\n%s", a.Vista())
	}
	if enfocar(t, a, tx("receta.esquema")); !strings.Contains(a.Vista(), "opciones avanzadas") {
		t.Fatalf("y lo explica:\n%s", a.Vista())
	}
}
