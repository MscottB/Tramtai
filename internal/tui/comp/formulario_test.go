package comp

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func envia(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(Enviado)
	return ok
}

func TestFormularioValida(t *testing.T) {
	f := NuevoFormulario(estilos(), Texto("nombre", "Nombre", ""), Numero("puerto", "Puerto", ""), Interruptor("activa", "Activa", false), Opcion("modelo", "Modelo", []string{"rápido", "lento"}, "lento"))
	f.Validar = func(f *Formulario) map[string]string {
		if f.Valor("nombre") == "" {
			return map[string]string{"nombre": "Falta el nombre"}
		}
		return nil
	}
	if !f.EnCampo() {
		t.Fatal("el primer campo de texto tiene el foco")
	}
	if envia(f.Update(tecla("ctrl+enter"))) {
		t.Fatal("no se envía con errores")
	}
	if f.Errores()["nombre"] != "Falta el nombre" || !strings.Contains(f.Vista(), "Falta el nombre") {
		t.Fatalf("enseña el error: %v\n%s", f.Errores(), f.Vista())
	}
	escribir(f.Update, "mi-app")
	if len(f.Errores()) != 0 || f.Valor("nombre") != "mi-app" {
		t.Fatalf("valida al escribir: %v %q", f.Errores(), f.Valor("nombre"))
	}
	f.Update(tecla("tab"))
	escribir(f.Update, "8a0")
	if f.Valor("puerto") != "80" || f.Entero("puerto") != 80 {
		t.Fatalf("un número solo admite cifras: %q", f.Valor("puerto"))
	}
	f.Update(tecla("enter"))
	if !f.EnCampo() {
		t.Fatal("en el interruptor el formulario sigue con el foco: Tab, q y 1…7 no se escapan")
	}
	f.Update(tecla("space"))
	if !f.Marcado("activa") {
		t.Fatal("espacio marca el interruptor")
	}
	f.Update(tecla("down"))
	f.Update(tecla("left"))
	if f.Valor("modelo") != "rápido" || !f.EnCampo() {
		t.Fatalf("← cambia la opción, que también tiene el foco: %q", f.Valor("modelo"))
	}
	if !envia(f.Update(tecla("enter"))) {
		t.Fatal("Intro en el último campo envía")
	}
	if f.Valor("modelo") != "rápido" || f.Valor("nombre") != "mi-app" {
		t.Fatal("enviar no borra los valores")
	}
	if c := f.Update(tecla("esc")); c == nil {
		t.Fatal("Esc cancela")
	} else if _, ok := c().(Cancelado); !ok {
		t.Fatal("Esc responde Cancelado")
	}
}

func TestAreaCtrlEnter(t *testing.T) {
	f := NuevoFormulario(estilos(), Area("mensaje", "Mensaje", ""))
	largo := strings.Repeat("x", 5000)
	f.Update(tea.PasteMsg{Content: "uno\ndos\n" + largo})
	if envia(f.Update(tecla("enter"))) {
		t.Fatal("Intro en un área de texto no envía")
	}
	if v := f.Valor("mensaje"); v != "uno\ndos\n"+largo+"\n" {
		t.Fatalf("el área guarda lo pegado entero y el salto de Intro: %d caracteres, %d saltos", len(v), strings.Count(v, "\n"))
	}
	if !f.EnCampo() {
		t.Fatal("un área de texto es un campo")
	}
	if !envia(f.Update(tecla("ctrl+enter"))) {
		t.Fatal("Ctrl+Enter envía")
	}
}

// TestFormularioDesdeFuera: la página mueve el foco, pone valores (una plantilla, una receta) y sabe dónde está el campo.
func TestFormularioDesdeFuera(t *testing.T) {
	f := NuevoFormulario(estilos(), Area("notas", "Notas", ""), Opcion("modelo", "Modelo", []string{"a", "b"}, "a"), Interruptor("si", "Sí", false), Texto("nombre", "Nombre", ""))
	f.Enfocar("nombre")
	if f.Foco() != "nombre" {
		t.Fatalf("Enfocar mueve el foco: %s", f.Foco())
	}
	f.Enfocar("no-existe")
	if f.Foco() != "nombre" {
		t.Fatal("un campo que no existe no lo mueve")
	}
	f.Poner("notas", "uno\ndos")
	f.Poner("modelo", "b")
	f.Poner("si", "1")
	f.Poner("no-existe", "x")
	if f.Valor("notas") != "uno\ndos" || f.Valor("modelo") != "b" || !f.Marcado("si") {
		t.Fatalf("Poner cambia los valores: %q %q %v", f.Valor("notas"), f.Valor("modelo"), f.Marcado("si"))
	}
	f.Vista()
	if f.LineaFoco() == 0 {
		t.Fatal("el último campo no está en la primera línea")
	}
	errorDeFuera := ""
	f.Validar = func(*Formulario) map[string]string { return map[string]string{"nombre": errorDeFuera} }
	errorDeFuera = "lo dice el servicio"
	f.Update(tecla("x"))
	f.Revalidar()
	if !strings.Contains(f.Vista(), "lo dice el servicio") {
		t.Fatal("Revalidar enseña un error que llega después")
	}
}

// TestPliegue: los campos de un pliegue cerrado no se ven ni reciben el foco; Intro lo abre. Un error escondido o
// Enfocar abren el pliegue.
func TestPliegue(t *testing.T) {
	campos := append([]Campo{Texto("nombre", "Nombre", ""), Pliegue("mas", "Opciones", false)},
		Plegados("mas", Texto("grupo", "Grupo", ""), Numero("tiempo", "Tiempo", ""))...)
	f := NuevoFormulario(estilos(), append(campos, Texto("fin", "Fin", ""))...)
	if v := f.Vista(); strings.Contains(v, "Grupo") || !strings.Contains(v, "▸ Opciones") {
		t.Fatalf("cerrado no enseña sus campos:\n%s", v)
	}
	f.Update(tecla("tab"))
	f.Update(tecla("tab"))
	if f.Foco() != "fin" {
		t.Fatalf("Tab salta lo plegado: %s", f.Foco())
	}
	f.Update(tecla("up"))
	f.Update(tecla("enter"))
	if v := f.Vista(); !strings.Contains(v, "Grupo") || !strings.Contains(v, "▾ Opciones") || f.Foco() != "mas" {
		t.Fatalf("Intro lo abre:\n%s", v)
	}
	if a, n := f.Posicion(); a != 2 || n != 5 {
		t.Fatalf("la posición cuenta lo que se ve: %d/%d", a, n)
	}
	f.Update(tecla("enter"))
	f.Validar = func(f *Formulario) map[string]string { return map[string]string{"tiempo": "Falta"} }
	f.Update(tecla("ctrl+enter"))
	if f.Foco() != "tiempo" || !f.Marcado("mas") {
		t.Fatalf("un error escondido abre el pliegue: %s", f.Foco())
	}
	f.Update(tecla("up"))
	f.Update(tecla("up"))
	f.Update(tecla("space"))
	f.Enfocar("grupo")
	if f.Foco() != "grupo" || !f.Marcado("mas") {
		t.Fatal("Enfocar abre el pliegue")
	}
}
