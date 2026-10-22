package ejecutar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"tramtai/internal/nucleo"
	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"
)

// Las pruebas de la primera ronda de arreglos: errores que no se tragan, atajos sin servicio, rutas con tildes, el esquema
// al salir con Esc, navegar mientras se envía y las pistas de «Guardar como API».

func errorServicio(fuera []tea.Msg) bool {
	for _, m := range fuera {
		if _, ok := m.(pagina.ErrorServicio); ok {
			return true
		}
	}
	return false
}

// TestErroresNoSeTragan: si el servicio no responde al validar el esquema o al mirar el conversor, se avisa; si el borrador
// no se puede guardar, también.
func TestErroresNoSeTragan(t *testing.T) {
	cli, id, cerrar := prueba.Arrancar(t)
	p := abierta(t, cli, id)
	pdf := filepath.Join(t.TempDir(), "informe.pdf")
	os.WriteFile(pdf, []byte("%PDF"), 0o600)
	cerrar()
	if fuera := correr(p, p.validarEsquema(datosForm{estructurada: true, esquema: `{"type":"object"}`})); !errorServicio(fuera) {
		t.Fatalf("validar el esquema sin servicio avisa: %v", fuera)
	}
	if fuera := correr(p, p.comprobarConversor([]string{pdf})); !errorServicio(fuera) {
		t.Fatalf("mirar el conversor sin servicio avisa: %v", fuera)
	}
	archivo := filepath.Join(t.TempDir(), "no-es-carpeta")
	os.WriteFile(archivo, nil, 0o600)
	m := escribirBorrador(archivo, "a medias")()
	if a, ok := m.(comp.NuevoAviso); !ok || a.Tipo != comp.Advertencia || !strings.Contains(a.Texto, "borrador") {
		t.Fatalf("si el borrador no se guarda, avisa: %#v", m)
	}
}

// TestAtajosSinServicio: sin cliente, cada atajo que habla con el servicio avisa a la app, como enviar.
func TestAtajosSinServicio(t *testing.T) {
	enEspanol(t)
	p := nueva()
	correr(p, p.Iniciar(contexto(nil, "global", "")))
	for _, k := range []string{"k", "g", "ctrl+p"} {
		if fuera := pulsar(p, k); !errorServicio(fuera) {
			t.Errorf("%s sin servicio: %v", k, fuera)
		}
	}
	_, cmd := p.Update(pagina.ConPunto{Nombre: "mi-app"})
	if fuera := correr(p, cmd); !errorServicio(fuera) {
		t.Errorf("abrir una API sin servicio: %v", fuera)
	}
	p.final = &nucleo.Trabajo{ID: "a", Estado: nucleo.Fallido, CodigoError: nucleo.CodigoSalidaNoCumpleEsquema, Salida: "x", Corregible: true}
	if fuera := pulsar(p, "o"); !errorServicio(fuera) {
		t.Errorf("corregir sin servicio: %v", fuera)
	}
	p.final, p.trabajo, p.estado = nil, &nucleo.Trabajo{ID: "a", Estado: nucleo.Ejecutando}, nucleo.Ejecutando
	pulsar(p, "c")
	_, cmd = p.Update(comp.Confirmado{})
	if fuera := correr(p, cmd); !errorServicio(fuera) {
		t.Errorf("cancelar sin servicio: %v", fuera)
	}
}

// TestCompletarConTildes: lo común de «más.txt» y «mís.txt» es «m», sin partir la tilde.
func TestCompletarConTildes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "más.txt"), nil, 0o600)
	os.WriteFile(filepath.Join(dir, "mís.txt"), nil, 0o600)
	base := dir + string(filepath.Separator)
	got, candidatos := completar(base + "m")
	if got != base+"m" || len(candidatos) != 2 || !utf8.ValidString(got) {
		t.Fatalf("lo común sin partir caracteres: %q %v", got, candidatos)
	}
}

// aJSON pasa el esquema a JSON, si la receta tiene el modo campos.
func aJSON(p *Pagina) {
	p.form.Enfocar("modo")
	if p.form.Foco() == "modo" {
		p.editando = true
		pulsar(p, "right", "esc")
	}
}

// conEstructurada activa la salida estructurada.
func conEstructurada(p *Pagina) {
	p.form.Enfocar("estructurada")
	p.editando = true
	pulsar(p, "space", "esc")
}

// TestEsquemaAlSalirConEsc: dejar el esquema con Esc también lo comprueba con el servicio.
func TestEsquemaAlSalirConEsc(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	conEstructurada(p)
	aJSON(p)
	p.form.Poner("esquema", "")
	escribir(p, "esquema", `{"type":"object","properties":{"x":{"type":"nada"}}}`)
	pulsar(p, "esc")
	if p.EnCampo() || p.errEsquema == "" {
		t.Fatalf("Esc deja el campo y el servicio dice qué está mal: %q", p.errEsquema)
	}
}

// TestNavegarMientrasSeEnvia: si se deja la página con el envío en camino, el trabajo creado no se pierde, no se puede
// mandar otra vez y el borrador se borra; con la página oculta no se abre el seguimiento en vivo.
func TestNavegarMientrasSeEnvia(t *testing.T) {
	enEspanol(t)
	cli, id := prueba.Servicio(t)
	carpeta := t.TempDir()
	p := nueva()
	correr(p, p.Iniciar(contexto(cli, id, carpeta)))
	t.Cleanup(p.Ocultar)
	escribir(p, "mensaje", "hola")
	pulsar(p, "esc")
	envio := p.enviar()
	p.Ocultar()
	correr(p, p.Mostrar())
	pulsar(p, "ctrl+s")
	correr(p, envio)
	if lista, _ := cli.Trabajos(ctx, id, 10); len(lista) != 1 {
		t.Fatalf("una sola petición: %d", len(lista))
	}
	if p.trabajo == nil {
		t.Fatal("la petición creada se enseña")
	}
	if _, err := os.Stat(filepath.Join(carpeta, archivoBorrador)); !os.IsNotExist(err) {
		t.Fatalf("el borrador se borra: %v", err)
	}
	hasta(t, p, func() bool { return p.final != nil })

	// Con la página oculta cuando llega, se recuerda sin seguirla; al volver se sigue.
	pulsar(p, "n")
	escribir(p, "mensaje", "otra")
	pulsar(p, "esc")
	envio = p.enviar()
	p.Ocultar()
	correr(p, envio)
	if p.trabajo == nil || p.cancelar != nil {
		t.Fatalf("oculta, la recuerda sin abrir el seguimiento: %v %v", p.trabajo, p.cancelar != nil)
	}
	correr(p, p.Mostrar())
	if p.activo() && p.cancelar == nil {
		t.Fatal("al volver se sigue en vivo")
	}
}

// TestGuardarConPistas: «Guardar como API» explica el nombre y el grupo, como GuardarComoApi.vue.
func TestGuardarConPistas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	p := abierta(t, cli, id)
	pulsar(p, "g")
	if v := p.Vista(); !strings.Contains(v, "Por ejemplo: contabilidad") {
		t.Fatalf("la pista del nombre:\n%s", v)
	}
	pulsar(p, "tab")
	if v := p.Vista(); !strings.Contains(v, "Para ordenarlas") {
		t.Fatalf("la pista del grupo:\n%s", v)
	}
}
