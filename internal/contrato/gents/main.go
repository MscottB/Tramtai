// gents escribe web/src/api/tipos.gen.ts: los tipos del contrato entre el servicio y la Web UI, en TypeScript, sacados
// de los structs de Go por reflexión. Se ejecuta con go generate ./internal/contrato y una prueba comprueba que el
// archivo guardado está al día.
//
// Lo que no se deduce de Go se dice con la etiqueta ts:"…" del campo (por ejemplo, una unión de literales).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"time"

	"tramtai/internal/api"
	"tramtai/internal/contrato"
	"tramtai/internal/documentos"
	"tramtai/internal/nucleo"
	"tramtai/internal/salidas"
)

// raiz es un tipo que la Web UI usa por su nombre; parcial: todos sus campos son opcionales (se envía a medias).
type raiz struct {
	nombre  string
	valor   any
	parcial bool
}

var raices = []raiz{
	{nombre: "Trabajo", valor: api.VistaTrabajo{}},
	{nombre: "Solicitud", valor: nucleo.SolicitudEjecucion{}, parcial: true},
	{nombre: "EventoRegistro", valor: nucleo.EventoRegistro{}},
	{nombre: "EventoTrabajo", valor: contrato.EventoTrabajo{}},
	{nombre: "Capacidades", valor: contrato.Capacidades{}},
	{nombre: "PrimerosPasos", valor: contrato.PrimerosPasos{}},
	{nombre: "ResumenInstancia", valor: contrato.ResumenInstancia{}},
	{nombre: "EspacioUI", valor: contrato.Espacio{}},
	{nombre: "ListaPuntos", valor: contrato.ListaPuntos{}},
	{nombre: "ConfigPunto", valor: contrato.ConfigPunto{}},
	{nombre: "Retencion", valor: contrato.Retencion{}},
	{nombre: "EntradaConfiguracion", valor: contrato.EntradaConfiguracion{}},
	{nombre: "EstadoClave", valor: contrato.EstadoClave{}},
	{nombre: "Modelo", valor: contrato.Modelo{}},
	{nombre: "AjustesServicio", valor: contrato.AjustesServicio{}},
	{nombre: "EstadoActualizacion", valor: contrato.EstadoActualizacion{}},
	{nombre: "ProgresoActualizacion", valor: contrato.ProgresoActualizacion{}},
	{nombre: "Acerca", valor: contrato.Acerca{}},
	{nombre: "EstadoTerminos", valor: contrato.EstadoTerminos{}},
	{nombre: "EntradaRemota", valor: contrato.EntradaRemota{}},
	{nombre: "SecretoTOTP", valor: contrato.SecretoTOTP{}},
	{nombre: "CodigosRecuperacion", valor: contrato.CodigosRecuperacion{}},
	{nombre: "EstadoServidor", valor: contrato.EstadoServidor{}},
}

// nombres son los de TypeScript que no son el de Go: la Web UI ya los usaba, o el de Go choca con otro.
var nombres = map[reflect.Type]string{
	reflect.TypeFor[contrato.Limites](): "LimitesPeticion",
	reflect.TypeFor[salidas.Problema](): "ProblemaSalida",
}

// literales son tipos de Go cuyo JSON no sale de sus campos (tienen MarshalJSON propio).
var literales = map[reflect.Type]string{
	reflect.TypeFor[nucleo.EleccionHerramienta](): "'auto' | 'ninguna' | 'obligatoria' | { nombre: string }",
}

// uniones son los tipos de texto con sus valores posibles.
var uniones = map[reflect.Type][]string{
	reflect.TypeFor[contrato.EstadoAgente](): textos(contrato.EstadosAgente),
}

func textos[T ~string](lista []T) []string {
	salida := make([]string, len(lista))
	for i, v := range lista {
		salida[i] = string(v)
	}
	return salida
}

// generador junta las interfaces en el orden en que aparecen: primero las raíces y después lo que usan.
type generador struct {
	nombreDe  map[reflect.Type]string
	tipoDe    map[string]reflect.Type
	parciales map[reflect.Type]bool
	pendiente []reflect.Type
	hechos    map[reflect.Type]bool
	salida    strings.Builder
}

// Generar devuelve el contenido de tipos.gen.ts.
func Generar() string {
	g := &generador{nombreDe: map[reflect.Type]string{}, tipoDe: map[string]reflect.Type{}, parciales: map[reflect.Type]bool{}, hechos: map[reflect.Type]bool{}}
	g.salida.WriteString("// Generado por internal/contrato/gents (go generate ./internal/contrato): no lo edites a mano.\n" +
		"// Son los tipos que el servicio envía y recibe, sacados de los structs de Go.\n")
	for _, r := range raices {
		t := reflect.TypeOf(r.valor)
		g.nombrar(t, r.nombre)
		g.parciales[t] = r.parcial
		g.pendiente = append(g.pendiente, t)
	}
	for len(g.pendiente) > 0 {
		t := g.pendiente[0]
		g.pendiente = g.pendiente[1:]
		if !g.hechos[t] {
			g.hechos[t] = true
			g.interfaz(t)
		}
	}
	for _, c := range constantes {
		fmt.Fprintf(&g.salida, "\n// %s\nexport const %s: readonly string[] = ['%s']\n", c.doc, c.nombre, strings.Join(c.valores, "', '"))
	}
	return g.salida.String()
}

// constantes son listas de valores que la Web UI necesita además de los tipos, con un dueño en Go.
var constantes = []struct {
	nombre, doc string
	valores     []string
}{
	{"estadosTerminales", "estadosTerminales son los estados de un trabajo que ya acabó (nucleo.Terminales).", textos(nucleo.Terminales)},
	{"extensionesConversor", "extensionesConversor son las que pasan por el conversor de archivos (documentos.Formatos).", conversor()},
}

func conversor() []string {
	_, c := documentos.Formatos()
	return c
}

// nombrar da a un tipo su nombre de TypeScript; dos tipos distintos con el mismo nombre son un error del generador.
func (g *generador) nombrar(t reflect.Type, nombre string) string {
	if n, ok := g.nombreDe[t]; ok {
		return n
	}
	if otro, ok := g.tipoDe[nombre]; ok && otro != t {
		log.Fatalf("%s y %s se llamarían %s en TypeScript: dale otro nombre en nombres", otro, t, nombre)
	}
	g.nombreDe[t], g.tipoDe[nombre] = nombre, t
	return nombre
}

func (g *generador) interfaz(t reflect.Type) {
	fmt.Fprintf(&g.salida, "\nexport interface %s {\n", g.nombreDe[t])
	for _, c := range g.campos(t, g.parciales[t]) {
		fmt.Fprintf(&g.salida, "  %s\n", c)
	}
	g.salida.WriteString("}\n")
}

// campos son las líneas de los campos de un struct, con los de sus structs incrustados en su sitio.
func (g *generador) campos(t reflect.Type, parcial bool) []string {
	var lineas []string
	for i := range t.NumField() {
		f := t.Field(i)
		etiqueta := f.Tag.Get("json")
		nombre, opciones, _ := strings.Cut(etiqueta, ",")
		if !f.IsExported() || nombre == "-" {
			continue
		}
		if f.Anonymous && nombre == "" {
			lineas = append(lineas, g.campos(f.Type, parcial)...)
			continue
		}
		if nombre == "" {
			nombre = f.Name
		}
		opcional := parcial || strings.Contains(opciones, "omitempty") || strings.Contains(opciones, "omitzero")
		tipo := f.Tag.Get("ts")
		if tipo == "" {
			tipo = g.tipo(f.Type)
		}
		if opcional {
			nombre += "?"
		} else if f.Type.Kind() == reflect.Pointer {
			tipo += " | null" // sin omitempty, un puntero nil sale como null
		}
		lineas = append(lineas, nombre+": "+tipo)
	}
	return lineas
}

// tipo es la expresión de TypeScript de un tipo de Go.
func (g *generador) tipo(t reflect.Type) string {
	if l, ok := literales[t]; ok {
		return l
	}
	if valores, ok := uniones[t]; ok {
		return "'" + strings.Join(valores, "' | '") + "'"
	}
	switch t {
	case reflect.TypeFor[time.Time]():
		return "string"
	case reflect.TypeFor[json.RawMessage]():
		return "unknown"
	}
	if err := sinMarshalerPropio(t); t.Kind() != reflect.Pointer && err != nil {
		log.Fatal(err)
	}
	switch t.Kind() {
	case reflect.Pointer:
		return g.tipo(t.Elem())
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Interface:
		return "unknown"
	case reflect.Slice, reflect.Array:
		elemento := g.tipo(t.Elem())
		if strings.Contains(elemento, "|") {
			elemento = "(" + elemento + ")"
		}
		return elemento + "[]"
	case reflect.Map:
		return "Record<string, " + g.tipo(t.Elem()) + ">"
	case reflect.Struct:
		if t.Name() == "" { // un struct anónimo va en línea
			return "{ " + strings.Join(g.campos(t, false), "; ") + " }"
		}
		nombre, ok := nombres[t]
		if !ok {
			nombre = t.Name()
		}
		nombre = g.nombrar(t, nombre)
		if !g.hechos[t] {
			g.pendiente = append(g.pendiente, t)
		}
		return nombre
	}
	log.Fatalf("gents no sabe escribir %s", t)
	return ""
}

// sinMarshalerPropio da error si el JSON de t no sale de sus campos (tiene MarshalJSON propio) y el generador no lo
// sabe: ni está en literales ni es una raíz, cuyos campos describen ya lo que envía su MarshalJSON.
func sinMarshalerPropio(t reflect.Type) error {
	marshaler := reflect.TypeFor[json.Marshaler]()
	if !t.Implements(marshaler) && !reflect.PointerTo(t).Implements(marshaler) {
		return nil
	}
	if _, ok := literales[t]; ok {
		return nil
	}
	for _, r := range raices {
		if reflect.TypeOf(r.valor) == t {
			return nil
		}
	}
	return fmt.Errorf("%s tiene MarshalJSON propio: añádelo a literales con su tipo de TypeScript", t)
}

func main() {
	salida := flag.String("o", "../../web/src/api/tipos.gen.ts", "el archivo que escribe")
	flag.Parse()
	if err := os.WriteFile(*salida, []byte(Generar()), 0o644); err != nil {
		log.Fatal(err)
	}
}
