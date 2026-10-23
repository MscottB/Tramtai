package ejecutar_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas del formulario: lo pegado, las plantillas y el esquema, el modelo y el esfuerzo, y los archivos.

// TestPegarVariasLineas: lo pegado (3 líneas, 5000 caracteres) entra entero en el mensaje, e Intro no lo envía.
func TestPegarVariasLineas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	linea := strings.Repeat("x", 1666)
	texto := linea + "\n" + linea + "\n" + linea
	if len([]rune(texto)) != 5000 {
		t.Fatalf("la prueba pega 5000 caracteres: %d", len(texto))
	}
	enfocar(t, a, tx("ejecutar.mensaje"))
	a.Pegar(texto)
	if !a.P.EnCampo() {
		t.Fatal("pegar empieza a editar el campo")
	}
	a.Pulsar("enter")
	if n := cuantos(t, cli, id); n != 0 || pestana(a, "ejecutar.respuesta") {
		t.Fatalf("Intro en el mensaje salta de línea, no envía: %d", n)
	}
	enviar(a)
	if e := terminada(t, a); e != nucleo.Completado || !strings.Contains(a.Vista(), linea[:50]) {
		t.Fatalf("Ctrl+S lo envía: %s", e)
	}
	if m := ultimo(t, cli, id).Solicitud.Mensajes[0].Contenido; m != texto+"\n" {
		t.Fatalf("lo pegado entra entero e Intro añade una línea: %d caracteres, %d líneas", len(m), strings.Count(m, "\n")+1)
	}
}

// TestPlantillaRellenaEsquema: con la salida estructurada, la plantilla pone su esquema y sus instrucciones; con un esquema
// propio pregunta antes de cambiarlo.
func TestPlantillaRellenaEsquema(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	poner(t, a, tx("ejecutar.mensaje"), "hola")
	abrirPliegue(t, a, "receta.opciones")
	editar(t, a, tx("receta.salidaEstructurada"))
	a.Pulsar("space")
	if !marcado(t, a, tx("receta.salidaEstructurada")) || valor(t, a, campo("1", "nombre")) != "resumen" {
		t.Fatalf("activar la salida estructurada pone el esquema de ejemplo:\n%s", a.Vista())
	}
	a.Pulsar("esc")
	editar(t, a, tx("receta.plantillaCorta"))
	if fuera := a.Pulsar("enter"); len(fuera) != 0 {
		t.Fatalf("sobre el esquema de ejemplo no pregunta: %v", fuera)
	}
	if !strings.Contains(a.Vista(), tx("campos.tipo.fecha")) || valor(t, a, campo("1", "nombre")) != "nombre" {
		t.Fatalf("pone el esquema de «Extraer datos», en campos:\n%s", a.Vista())
	}
	if v := valor(t, a, tx("receta.instrucciones")); !strings.HasPrefix(v, "Extrae del texto") {
		t.Fatalf("y sus instrucciones, que estaban vacías: %q", v)
	}
	// Otra plantilla sobre un esquema propio: pregunta; cancelar lo deja y confirmar lo cambia.
	editar(t, a, campo("1", "nombre"))
	a.Escribir("_propio")
	editar(t, a, tx("receta.plantillaCorta"))
	a.Pulsar("right")
	q, ok := pregunta(a.Pulsar("enter"))
	if !ok || !strings.Contains(q.Texto, "Cambiar el esquema") {
		t.Fatalf("pregunta antes de cambiar el esquema, con la pregunta de la Web UI: %+v", q)
	}
	a.Entregar(comp.Cancelado{})
	if valor(t, a, campo("1", "nombre")) != "nombre_propio" || !a.P.EnCampo() {
		t.Fatal("cancelar deja el esquema y sigue editando")
	}
	a.Pulsar("enter")
	a.Entregar(comp.Confirmado{})
	if valor(t, a, campo("1", "nombre")) == "nombre_propio" || !strings.Contains(a.Vista(), "categoria") {
		t.Fatalf("confirmar pone la otra plantilla:\n%s", a.Vista())
	}
	if v := valor(t, a, tx("receta.instrucciones")); !strings.HasPrefix(v, "Extrae del texto") {
		t.Fatalf("las instrucciones propias no se tocan: %q", v)
	}
	// En JSON, el esquema de la plantilla va con sangría.
	aJSON(t, a)
	if !strings.Contains(a.Vista(), `"type": "object"`) || !strings.Contains(esquemaEnviado(t, a, cli, id), `"categoria"`) {
		t.Fatalf("en JSON, con sangría:\n%s", a.Vista())
	}
}

// TestEsquemaInvalidoAvisa: al salir del esquema lo comprueba el servicio y su error se ve bajo el campo; sin arreglarlo
// no se envía.
func TestEsquemaInvalidoAvisa(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	poner(t, a, tx("ejecutar.mensaje"), "hola")
	conEstructurada(t, a)
	aJSON(t, a)
	editar(t, a, tx("receta.esquema"))
	vaciar(a)
	a.Escribir(`{"type":"object","properties":{"x":{"type":"nada"}}}`)
	if strings.Contains(a.Vista(), "properties.x.type") {
		t.Fatal("mientras se escribe no se pregunta al servicio")
	}
	a.Pulsar("tab")
	a.HastaVer("properties.x.type")
	if enfocar(t, a, tx("receta.esquema")); !strings.Contains(a.Vista(), "properties.x.type") {
		t.Fatalf("al salir del campo, el servicio dice qué está mal, bajo el campo:\n%s", a.Vista())
	}
	if enviar(a); cuantos(t, cli, id) != 0 || foco(a) != tx("receta.esquema") {
		t.Fatalf("con el esquema mal no se envía y se enseña el campo con el error: %s", foco(a))
	}
	// Un JSON roto también: el servicio da la línea; escribir otra cosa quita el error viejo.
	poner(t, a, tx("receta.esquema"), "{")
	a.HastaVer("línea")
	if enfocar(t, a, tx("receta.esquema")); !strings.Contains(a.Vista(), "línea") {
		t.Fatalf("un JSON roto también se avisa:\n%s", a.Vista())
	}
	editar(t, a, tx("receta.esquema"))
	a.Escribir("}")
	if strings.Contains(a.Vista(), "línea") {
		t.Fatalf("al cambiar el esquema, el error del servicio se olvida:\n%s", a.Vista())
	}
}

// TestModeloYEsfuerzo: el modelo viene del catálogo y el esfuerzo ofrece los niveles de su agente; ConAgente lo elige.
func TestModeloYEsfuerzo(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	if v := valor(t, a, tx("receta.modelo")); !strings.Contains(v, "Predeterminado") {
		t.Fatalf("el catálogo da los modelos: %q", v)
	}
	a.Entregar(pagina.ConAgente{Agente: "simulado"})
	if valor(t, a, tx("receta.modelo")) != "simulado" || foco(a) != tx("ejecutar.mensaje") {
		t.Fatalf("Probar con un agente lo deja elegido:\n%s", a.Vista())
	}
	if abrirPliegue(t, a, "receta.opciones"); !strings.Contains(a.Vista(), tx("receta.forzar")) {
		t.Fatal("con modelo se ofrece forzar")
	}
	editar(t, a, tx("receta.modelo"))
	for i := 0; i < 20 && !strings.Contains(valor(t, a, tx("receta.modelo")), "Predeterminado"); i++ {
		a.Pulsar("left")
	}
	if !strings.Contains(valor(t, a, tx("receta.modelo")), "Predeterminado") || foco(a) != tx("receta.modelo") {
		t.Fatalf("← vuelve al predeterminado y el foco sigue en el modelo:\n%s", a.Vista())
	}
	if a.Pulsar("esc"); strings.Contains(a.Vista(), tx("receta.forzar")) {
		t.Fatal("sin modelo no hay forzar")
	}
}

// TestCompletarRuta: Tab en el campo de archivos completa la última ruta con lo que hay en disco, en lugar de pasar al
// siguiente campo: el único que encaja, lo común a varios (y los enseña) o una carpeta con su separador.
func TestCompletarRuta(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "informe.txt"), nil, 0o600)
	os.WriteFile(filepath.Join(dir, "indice.md"), nil, 0o600)
	os.Mkdir(filepath.Join(dir, "notas"), 0o700)
	base := dir + string(filepath.Separator)
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	archivos := tx("ejecutar.adjuntar")
	completa := func(escrito string) ([]string, string) {
		t.Helper()
		editar(t, a, archivos)
		vaciar(a)
		a.Pegar(escrito)
		var avisos []string
		if av, ok := aviso(a.Pulsar("tab")); ok {
			avisos = strings.Fields(av.Texto)
		}
		return avisos, valor(t, a, archivos)
	}
	if c, got := completa("a.txt, " + base + "inf"); got != "a.txt, "+base+"informe.txt" || len(c) != 0 || foco(a) != archivos {
		t.Fatalf("completa el único que encaja y sigue en el campo: %q %v %s", got, c, foco(a))
	}
	a.Pulsar("tab")
	if foco(a) != tx("comun.opcionesAvanzadas") {
		t.Fatalf("ya completa, Tab pasa al siguiente: %s", foco(a))
	}
	if c, got := completa(base + "i"); got != base+"in" || len(c) != 2 {
		t.Fatalf("con varios, lo común, y los enseña: %q %v", got, c)
	}
	if _, got := completa(base + "no"); got != base+"notas"+string(filepath.Separator) {
		t.Fatalf("una carpeta lleva su separador: %q", got)
	}
	if _, got := completa(base + "zz"); got != base+"zz" || foco(a) != tx("comun.opcionesAvanzadas") {
		t.Fatalf("sin coincidencias no cambia y Tab pasa al siguiente: %q %s", got, foco(a))
	}
}

// TestAvisoConversorAlAdjuntar: al dejar el campo de archivos con un PDF y sin conversor listo se avisa, con la tecla
// que lo prepara, y no se envía.
func TestAvisoConversorAlAdjuntar(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cli, id := prueba.SinConversor(t)
	a := abierta(t, cli, id)
	pdf := filepath.Join(t.TempDir(), "informe.pdf")
	os.WriteFile(pdf, []byte("%PDF"), 0o600)
	poner(t, a, tx("ejecutar.mensaje"), "resume")
	editar(t, a, tx("ejecutar.adjuntar"))
	a.Pegar(pdf)
	fuera := a.HastaSalir(func() { a.Pulsar("tab") }, prueba.HayAviso)
	if av, ok := aviso(fuera); !ok || !strings.Contains(av.Texto, "Ctrl+P") {
		t.Fatalf("al dejar el campo avisa con la tecla que lo prepara: %v", fuera)
	}
	if enfocar(t, a, tx("ejecutar.adjuntar")); !strings.Contains(a.Vista(), "Ctrl+P") {
		t.Fatalf("el aviso se ve junto al campo:\n%s", a.Vista())
	}
	if av, ok := aviso(a.HastaSalir(func() { enviar(a) }, prueba.HayAviso)); !ok || !strings.Contains(av.Texto, "Ctrl+P") || cuantos(t, cli, id) != 0 {
		t.Fatalf("no se envía sin conversor: %+v", av)
	}
	editar(t, a, tx("ejecutar.adjuntar"))
	a.Pulsar("end")
	a.Escribir(".zip")
	if av, ok := aviso(a.HastaSalir(func() { a.Pulsar("tab") }, prueba.HayAviso)); !ok || !strings.Contains(av.Texto, "ZIP") {
		t.Fatalf("un formato que no se admite se avisa: %+v", av)
	}
}

// TestRutaNoPermitida: un archivo fuera de las raíces permitidas no se sube, y el aviso dice por qué.
func TestRutaNoPermitida(t *testing.T) {
	cli, id := prueba.Servicio(t)
	global := os.Getenv("TRAMTAI_CONFIG_GLOBAL")
	permitida := t.TempDir()
	os.WriteFile(global, []byte("predeterminados:\n  agente: simulado\ninstancia_global:\n  raices_permitidas: ["+jsonTexto(permitida)+"]\n"), 0o600)
	fuera := filepath.Join(t.TempDir(), "nota.txt")
	os.WriteFile(fuera, []byte("hola"), 0o600)
	a := abierta(t, cli, id)
	poner(t, a, tx("ejecutar.mensaje"), "lee")
	poner(t, a, tx("ejecutar.adjuntar"), fuera)
	if av, ok := aviso(a.HastaSalir(func() { enviar(a) }, prueba.HayAviso)); !ok || cuantos(t, cli, id) != 0 || !strings.Contains(av.Texto, "fuera de las carpetas desde las que se pueden enviar archivos") {
		t.Fatalf("la ruta se comprueba antes de subir: %+v", av)
	}
}

func jsonTexto(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
