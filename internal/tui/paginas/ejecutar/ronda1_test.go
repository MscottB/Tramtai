package ejecutar_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"tramtai/internal/tui/comp"
	"tramtai/internal/tui/pagina"
	"tramtai/internal/tui/paginas/prueba"

	tea "charm.land/bubbletea/v2"
)

// Las pruebas de la primera ronda de arreglos: errores que no se tragan, atajos sin servicio, rutas con tildes, el esquema
// al salir con Esc, navegar mientras se envía y las pistas de «Guardar como API».

// TestErroresNoSeTragan: si el servicio no responde al validar el esquema o al mirar el conversor, se avisa; si el borrador
// no se puede guardar, también.
func TestErroresNoSeTragan(t *testing.T) {
	cli, id, cerrar := prueba.Arrancar(t)
	a := abierta(t, cli, id)
	pdf := filepath.Join(t.TempDir(), "informe.pdf")
	os.WriteFile(pdf, []byte("%PDF"), 0o600)
	conEstructurada(t, a)
	aJSON(t, a)
	poner(t, a, tx("receta.esquema"), `{"type":"object"}`)
	editar(t, a, tx("receta.esquema"))
	cerrar()
	if fuera := a.HastaSalir(func() { a.Pulsar("esc") }, errorServicio); !errorServicio(fuera) {
		t.Fatalf("validar el esquema sin servicio avisa: %v", fuera)
	}
	editar(t, a, tx("ejecutar.adjuntar"))
	a.Pegar(pdf)
	if fuera := a.HastaSalir(func() { a.Pulsar("tab") }, errorServicio); !errorServicio(fuera) {
		t.Fatalf("mirar el conversor sin servicio avisa: %v", fuera)
	}
	// Una carpeta de datos que es un archivo: el borrador no se puede guardar.
	archivo := filepath.Join(t.TempDir(), "no-es-carpeta")
	os.WriteFile(archivo, nil, 0o600)
	cli2, id2 := prueba.Servicio(t)
	b, r := abrirConReloj(t, contexto(t, cli2, id2, archivo))
	escribir(t, b, tx("ejecutar.mensaje"), "a medias")
	guardarBorrador(b, r)
	b.Hasta(func() bool { return prueba.HayAviso(b.Fuera) })
	if av, ok := aviso(b.Fuera); !ok || av.Tipo != comp.Advertencia || !strings.Contains(av.Texto, "borrador") {
		t.Fatalf("avisa de que el borrador no se pudo guardar: %v", b.Fuera)
	}
}

// TestAtajosSinServicio: sin cliente, cada atajo que habla con el servicio avisa a la app, como enviar.
func TestAtajosSinServicio(t *testing.T) {
	a := abrir(t, contexto(t, nil, "global", ""))
	for _, k := range []string{"k", "g", "ctrl+p"} {
		if fuera := a.Pulsar(k); !errorServicio(fuera) {
			t.Errorf("%s sin servicio: %v", k, fuera)
		}
	}
	if fuera := a.Entregar(pagina.ConPunto{Nombre: "mi-app"}); !errorServicio(fuera) {
		t.Errorf("abrir una API sin servicio: %v", fuera)
	}
	// Una respuesta que se puede corregir y, después, el servicio que deja de estar: corregir avisa.
	cli, id := prueba.Servicio(t)
	b := abierta(t, cli, id)
	conEstructurada(t, b)
	aJSON(t, b)
	poner(t, b, tx("receta.esquema"), `{"type":"object","required":["total"],"properties":{"total":{"type":"number"}}}`)
	poner(t, b, tx("ejecutar.mensaje"), "no es JSON")
	enviar(b)
	terminada(t, b)
	b.Correr(b.P.Iniciar(contexto(t, nil, id, "")))
	if fuera := b.Pulsar("o"); !errorServicio(fuera) {
		t.Errorf("corregir sin servicio: %v", fuera)
	}
	// Una petición en marcha y, después, sin servicio: cancelarla avisa.
	cli3, id3, bloq := conBloqueado(t)
	c := abierta(t, cli3, id3)
	poner(t, c, tx("ejecutar.mensaje"), "esperar")
	enviar(c)
	<-bloq.Entro
	c.HastaVer("primera parte")
	c.Correr(c.P.Iniciar(contexto(t, nil, id3, "")))
	c.Pulsar("c")
	if fuera := c.Entregar(comp.Confirmado{}); !errorServicio(fuera) {
		t.Errorf("cancelar sin servicio: %v", fuera)
	}
}

// TestCompletarConTildes: lo común de «más.txt» y «mís.txt» es «m», sin partir la tilde.
func TestCompletarConTildes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "más.txt"), nil, 0o600)
	os.WriteFile(filepath.Join(dir, "mís.txt"), nil, 0o600)
	base := dir + string(filepath.Separator)
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	editar(t, a, tx("ejecutar.adjuntar"))
	a.Pegar(base + "m")
	av, ok := aviso(a.Pulsar("tab"))
	if got := valor(t, a, tx("ejecutar.adjuntar")); !ok || got != base+"m" || len(strings.Fields(av.Texto)) != 2 || !utf8.ValidString(got) {
		t.Fatalf("lo común sin partir caracteres: %q %+v", got, av)
	}
}

// TestEsquemaAlSalirConEsc: dejar el esquema con Esc también lo comprueba con el servicio.
func TestEsquemaAlSalirConEsc(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	conEstructurada(t, a)
	aJSON(t, a)
	editar(t, a, tx("receta.esquema"))
	vaciar(a)
	a.Escribir(`{"type":"object","properties":{"x":{"type":"nada"}}}`)
	a.Pulsar("esc")
	a.HastaVer("properties.x.type")
	if enfocar(t, a, tx("receta.esquema")); a.P.EnCampo() || !strings.Contains(a.Vista(), "properties.x.type") {
		t.Fatalf("Esc deja el campo y el servicio dice qué está mal:\n%s", a.Vista())
	}
}

// TestNavegarMientrasSeEnvia: si se deja la página con el envío en camino, el trabajo creado no se pierde, no se puede
// mandar otra vez y el borrador se borra; con la página oculta no se abre el seguimiento en vivo.
func TestNavegarMientrasSeEnvia(t *testing.T) {
	cli, id := prueba.Servicio(t)
	carpeta := t.TempDir()
	a, r := abrirConReloj(t, contexto(t, cli, id, carpeta))
	escribir(t, a, tx("ejecutar.mensaje"), "hola")
	guardarBorrador(a, r)
	if _, hay := hayBorrador(carpeta); !hay {
		t.Fatal("el borrador se guarda")
	}
	// enCamino deja el envío pedido al servicio sin que haya respondido: devuelve la petición retenida.
	enCamino := func() []tea.Cmd {
		enviado := a.Recoger(a.Retener(func() { enviar(a) })) // el formulario da el Enviado
		return a.Retener(func() { a.Entregar(enviado[0]) })   // y la página pide crear el trabajo
	}
	envio := enCamino()
	ocultar(a)
	mostrar(a)
	if fuera := enviar(a); len(fuera) != 0 || cuantos(t, cli, id) != 0 {
		t.Fatalf("con el envío en camino no se manda otro: %v", fuera)
	}
	a.Soltar(envio)
	a.Hasta(func() bool { return cuantos(t, cli, id) == 1 && pestana(a, "ejecutar.respuesta") })
	if n := cuantos(t, cli, id); n != 1 || !pestana(a, "ejecutar.respuesta") {
		t.Fatalf("la petición creada se enseña, una sola: %d", n)
	}
	a.Hasta(func() bool { _, hay := hayBorrador(carpeta); return !hay })
	if _, hay := hayBorrador(carpeta); hay {
		t.Fatal("la petición creada borra el borrador")
	}
	terminada(t, a)

	// Con la página oculta cuando llega, se recuerda sin seguirla; al volver se sigue.
	cli2, id2, b := conBloqueado(t)
	c := abierta(t, cli2, id2)
	poner(t, c, tx("ejecutar.mensaje"), "esperar")
	envio = func() []tea.Cmd {
		enviado := c.Recoger(c.Retener(func() { enviar(c) }))
		return c.Retener(func() { c.Entregar(enviado[0]) })
	}()
	ocultar(c)
	c.Soltar(envio)
	<-b.Entro
	c.Hasta(func() bool { return pestana(c, "ejecutar.respuesta") && !strings.Contains(c.Vista(), "en vivo") })
	if v := c.Vista(); !pestana(c, "ejecutar.respuesta") || strings.Contains(v, "en vivo") {
		t.Fatalf("oculta, la recuerda sin abrir el seguimiento:\n%s", v)
	}
	mostrar(c)
	c.HastaVer("en vivo")
	b.SoltarFin()
	terminada(t, c)
}

// TestGuardarConPistas: «Guardar como API» explica el nombre y el grupo, como GuardarComoApi.vue.
func TestGuardarConPistas(t *testing.T) {
	cli, id := prueba.Servicio(t)
	a := abierta(t, cli, id)
	a.Pulsar("g")
	if v := a.Vista(); !strings.Contains(v, "Por ejemplo: contabilidad") {
		t.Fatalf("la pista del nombre:\n%s", v)
	}
	a.Pulsar("tab")
	if v := a.Vista(); !strings.Contains(v, "Para ordenarlas") {
		t.Fatalf("la pista del grupo:\n%s", v)
	}
}
