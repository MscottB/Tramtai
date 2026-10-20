package herramientas

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
	"tramtai/internal/salidas"
)

var parametrosTiempo = json.RawMessage(`{"type":"object","properties":{"ciudad":{"type":"string"}},"required":["ciudad"]}`)

func conTiempo() nucleo.SolicitudEjecucion {
	return nucleo.SolicitudEjecucion{
		Instrucciones: "Sé breve.",
		Mensajes:      []nucleo.Mensaje{{Rol: "usuario", Contenido: "tiempo en Lima"}},
		Herramientas:  []nucleo.Herramienta{{Nombre: "tiempo", Descripcion: "El tiempo de una ciudad", Parametros: parametrosTiempo}},
	}
}

func falso() *bool { f := false; return &f }

func TestValidar(t *testing.T) {
	if err := Validar(conTiempo()); err != nil {
		t.Fatalf("válida: %v", err)
	}
	lista := []nucleo.Herramienta{{Nombre: "a"}, {Nombre: "b", Parametros: json.RawMessage(`{"type":"object"}`)}}
	casos := map[string]struct {
		sol    nucleo.SolicitudEjecucion
		codigo nucleo.CodigoError
	}{
		"repetida":          {nucleo.SolicitudEjecucion{Herramientas: []nucleo.Herramienta{{Nombre: "a"}, {Nombre: "a"}}}, nucleo.CodigoSolicitudInvalida},
		"sin nombre":        {nucleo.SolicitudEjecucion{Herramientas: []nucleo.Herramienta{{}}}, nucleo.CodigoSolicitudInvalida},
		"una que no está":   {nucleo.SolicitudEjecucion{Herramientas: lista, EleccionHerramienta: nucleo.EleccionHerramienta{Modo: "una", Nombre: "c"}}, nucleo.CodigoSolicitudInvalida},
		"obligatoria vacía": {nucleo.SolicitudEjecucion{EleccionHerramienta: nucleo.EleccionHerramienta{Modo: "obligatoria"}}, nucleo.CodigoSolicitudInvalida},
		"esquema roto":      {nucleo.SolicitudEjecucion{Herramientas: []nucleo.Herramienta{{Nombre: "a", Parametros: json.RawMessage(`{"type":7}`)}}}, nucleo.CodigoSolicitudInvalida},
		"con json":          {nucleo.SolicitudEjecucion{Herramientas: lista, Salida: nucleo.ConfigSalida{Formato: "json"}}, nucleo.CodigoParametroNoSoportado},
	}
	for nombre, c := range casos {
		if codigo := nucleo.CodigoDe(Validar(c.sol)); codigo != c.codigo {
			t.Errorf("%s: %s, se esperaba %s", nombre, codigo, c.codigo)
		}
	}
}

func TestValidarJSONSoloSiActivas(t *testing.T) {
	s := conTiempo()
	s.Salida = nucleo.ConfigSalida{Formato: "json", Esquema: json.RawMessage(`{"type":"object"}`)}
	s.EleccionHerramienta.Modo = "ninguna"
	if err := Validar(s); err != nil {
		t.Fatalf("con «ninguna» el agente no pide herramientas: el JSON vale: %v", err)
	}
	s.EleccionHerramienta.Modo = "auto"
	if nucleo.CodigoDe(Validar(s)) != nucleo.CodigoParametroNoSoportado {
		t.Fatal("herramientas activas y JSON no se combinan")
	}
}

func TestValidarIDLlamadaDesconocido(t *testing.T) {
	s := conTiempo()
	s.Mensajes = append(s.Mensajes, nucleo.Mensaje{Rol: "herramienta", IDLlamada: "call_9", Contenido: "18"})
	if nucleo.CodigoDe(Validar(s)) != nucleo.CodigoSolicitudInvalida {
		t.Fatal("un resultado sin su llamada se rechaza")
	}
	s.Mensajes = []nucleo.Mensaje{s.Mensajes[0],
		{Rol: "asistente", Llamadas: []nucleo.Llamada{{ID: "call_9", Nombre: "tiempo", Argumentos: json.RawMessage(`{}`)}}}, s.Mensajes[1]}
	if err := Validar(s); err != nil {
		t.Fatalf("con su llamada antes, vale: %v", err)
	}
}

func TestPrepararUna(t *testing.T) {
	s := nucleo.SolicitudEjecucion{Instrucciones: "Sé breve.", LlamadasParalelas: falso(),
		Herramientas:        []nucleo.Herramienta{{Nombre: "a"}, {Nombre: "b", Parametros: json.RawMessage(`{"type":"object"}`)}},
		EleccionHerramienta: nucleo.EleccionHerramienta{Modo: "una", Nombre: "b"}}
	p := Preparar(s)
	if p.Salida.Formato != "json" || !strings.Contains(string(p.Salida.Esquema), `"enum":["b"]`) ||
		!strings.HasPrefix(p.Instrucciones, "Sé breve.\n\n") || !strings.Contains(p.Instrucciones, "You must call the tool b.") ||
		!strings.Contains(p.Instrucciones, "Call at most one tool per reply.") {
		t.Fatalf("una: %s %q", p.Salida.Esquema, p.Instrucciones)
	}
	s.EleccionHerramienta = nucleo.EleccionHerramienta{Modo: "ninguna"}
	if p := Preparar(s); p.Salida.Formato != "" || p.Instrucciones != "Sé breve." {
		t.Fatalf("con «ninguna» no se describen las herramientas: %q", p.Instrucciones)
	}
}

func TestPrepararEscribeLaConversacion(t *testing.T) {
	s := conTiempo()
	s.Mensajes = append(s.Mensajes,
		nucleo.Mensaje{Rol: "asistente", Contenido: "Lo miro.", Llamadas: []nucleo.Llamada{{ID: "call_1", Nombre: "tiempo", Argumentos: json.RawMessage(`{"ciudad":"Lima"}`)}}},
		nucleo.Mensaje{Rol: "herramienta", IDLlamada: "call_1", Contenido: "18 grados"},
		nucleo.Mensaje{Rol: "herramienta", IDLlamada: "call_1", Contenido: "sin datos", EsError: true})
	m := Preparar(s).Mensajes
	if len(m) != 4 || m[1].Rol != "asistente" || len(m[1].Llamadas) != 0 ||
		m[1].Contenido != `{"content":"Lo miro.","tool_calls":[{"id":"call_1","name":"tiempo","arguments":"{\"ciudad\":\"Lima\"}"}]}` {
		t.Fatalf("asistente: %#v", m)
	}
	if m[2].Rol != "usuario" || m[2].Contenido != "Tool result for tiempo (call call_1):\n18 grados" ||
		m[3].Contenido != "Tool result (error) for tiempo (call call_1):\nsin datos" {
		t.Fatalf("resultados: %#v", m)
	}
}

func TestPrepararSinHerramientasNoCambiaNada(t *testing.T) {
	s := nucleo.SolicitudEjecucion{Instrucciones: "x", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: "hola"}}}
	if p := Preparar(s); !reflect.DeepEqual(p, s) {
		t.Fatalf("%#v", p)
	}
}

func TestPrepararNoModificaLaEntrada(t *testing.T) {
	s := conTiempo()
	s.Mensajes = append(s.Mensajes, nucleo.Mensaje{Rol: "asistente", Llamadas: []nucleo.Llamada{{ID: "c", Nombre: "tiempo", Argumentos: json.RawMessage(`{}`)}}})
	copia, _ := json.Marshal(s)
	Preparar(s)
	if despues, _ := json.Marshal(s); string(despues) != string(copia) {
		t.Fatalf("la entrada cambió:\n%s\n%s", copia, despues)
	}
}

func TestInterpretar(t *testing.T) {
	s := conTiempo()
	texto, llamadas, err := Interpretar(s, `{"content":"","tool_calls":[{"name":"tiempo","arguments":"{\"ciudad\": \"Lima\"}"}]}`, "call_x")
	if err != nil || texto != "" || len(llamadas) != 1 || llamadas[0].ID != "call_x_0" || string(llamadas[0].Argumentos) != `{"ciudad":"Lima"}` {
		t.Fatalf("%q %v %v", texto, llamadas, err)
	}
	// Con llamadas_paralelas false solo pasa la primera; los argumentos pueden llegar como objeto.
	s.Herramientas = append(s.Herramientas, nucleo.Herramienta{Nombre: "otra"})
	s.LlamadasParalelas = falso()
	_, llamadas, err = Interpretar(s, `{"content":"","tool_calls":[{"name":"otra","arguments":"{}"},{"name":"tiempo","arguments":{"ciudad":"Lima"}}]}`, "call_x")
	if err != nil || len(llamadas) != 1 || llamadas[0].Nombre != "otra" {
		t.Fatalf("solo una: %v %v", llamadas, err)
	}
	// Sin herramientas activas, la salida pasa tal cual.
	if texto, llamadas, err := Interpretar(nucleo.SolicitudEjecucion{}, "hola", "x"); texto != "hola" || llamadas != nil || err != nil {
		t.Fatalf("sin herramientas: %q %v %v", texto, llamadas, err)
	}
}

func TestInterpretarDevuelveProblemas(t *testing.T) {
	obligatoria := conTiempo()
	obligatoria.EleccionHerramienta.Modo = "obligatoria"
	casos := map[string]struct {
		sol    nucleo.SolicitudEjecucion
		salida string
	}{
		"no es el sobre":     {conTiempo(), "hola"},
		"no existe":          {conTiempo(), `{"content":"","tool_calls":[{"name":"otra","arguments":"{}"}]}`},
		"argumentos malos":   {conTiempo(), `{"content":"","tool_calls":[{"name":"tiempo","arguments":"{\"ciudad\":7}"}]}`},
		"argumentos no JSON": {conTiempo(), `{"content":"","tool_calls":[{"name":"tiempo","arguments":"x"}]}`},
		"faltó la llamada":   {obligatoria, `{"content":"no sé","tool_calls":[]}`},
		"sin el sobre":       {conTiempo(), `{"respuesta":"Hace 18 grados"}`},
		"null":               {conTiempo(), `null`},
		"name no es texto":   {conTiempo(), `{"content":"","tool_calls":[{"name":7,"arguments":"{}"}]}`},
	}
	for nombre, c := range casos {
		_, _, err := Interpretar(c.sol, c.salida, "x")
		if nucleo.CodigoDe(err) != nucleo.CodigoSalidaNoCumpleEsquema || len(salidas.ProblemasDe(err)) == 0 {
			t.Errorf("%s: %v", nombre, err)
		}
	}
}
