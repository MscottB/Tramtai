package historial

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"tramtai/internal/nucleo"
	"tramtai/internal/tui/pagina"
)

// Las pruebas del formato del detalle: los números, las duraciones y las líneas del registro, que se comprueban con
// registros hechos a mano (un servicio de prueba no da intentos con corrección, documentos ni validaciones así).

func trabajo(id, texto string, estado nucleo.Estado) nucleo.Trabajo {
	return nucleo.Trabajo{ID: id, Estado: estado, CreadoEn: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		Solicitud: nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}}
}

func TestNumeroYTiempo(t *testing.T) {
	es, en := Nueva("es").(*Pagina), Nueva("en").(*Pagina)
	for n, quiere := range map[int][2]string{999: {"999", "999"}, 1000: {"1000", "1,000"}, 12345: {"12.345", "12,345"}, 1234567: {"1.234.567", "1,234,567"}} {
		if es.numero(n) != quiere[0] || en.numero(n) != quiere[1] {
			t.Errorf("numero(%d) = %q y %q", n, es.numero(n), en.numero(n))
		}
	}
	if es.tiempo(250*time.Millisecond) != "250 ms" || es.tiempo(1500*time.Millisecond) != "1,5 s" || en.tiempo(1500*time.Millisecond) != "1.5 s" || es.tiempo(2*time.Second) != "2 s" {
		t.Errorf("tiempo: %q %q %q", es.tiempo(250*time.Millisecond), es.tiempo(1500*time.Millisecond), en.tiempo(1500*time.Millisecond))
	}
}

// TestCosteYDuracionDelIdioma: la ficha escribe el coste y la duración con la coma decimal en español, como el registro.
func TestCosteYDuracionDelIdioma(t *testing.T) {
	for lengua, quiere := range map[string][2]string{"es": {"US$ 0,0069", "13,3 s"}, "en": {"US$ 0.0069", "13.3 s"}} {
		p := Nueva(lengua).(*Pagina)
		p.estilos = &pagina.Estilos{Iconos: pagina.IconosUnicode}
		tr := trabajo("a", "uno", nucleo.Completado)
		tr.Uso.CostoUSD = 0.0069
		fin := tr.CreadoEn.Add(13300 * time.Millisecond)
		tr.TerminadoEn = &fin
		if texto := p.textoMensaje(tr); !strings.Contains(texto, quiere[0]) || !strings.Contains(texto, quiere[1]) {
			t.Errorf("%s: falta %q o %q:\n%s", lengua, quiere[0], quiere[1], texto)
		}
	}
}

// TestRegistroComoLaWeb: las líneas del registro dicen lo mismo que RegistroTrabajo.vue: el intento junto a su estado, las
// correcciones, el esfuerzo del agente, el contexto, la entrada y la validación.
func TestRegistroComoLaWeb(t *testing.T) {
	p := Nueva("es").(*Pagina)
	p.estilos = &pagina.Estilos{Iconos: pagina.IconosUnicode}
	inicio := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	ev := func(n int, tipo, datos string, ms int) nucleo.EventoRegistro {
		e := nucleo.EventoRegistro{Numero: n, Tipo: tipo, Datos: datos, CreadoEn: inicio.Add(time.Duration(ms) * time.Millisecond)}
		bruto, _ := json.Marshal(e) // como llega del servicio: con la etapa de su estado
		_ = json.Unmarshal(bruto, &e)
		return e
	}
	p.registro = []nucleo.EventoRegistro{
		ev(1, "estado", "en_cola", 0), ev(2, "entrada", `{"tokens_estimados":200000,"documentos":[{"nombre":"notas.pdf","tokens_estimados":150000}]}`, 10),
		ev(3, "estado", "ejecutando", 1500), ev(4, "intento", `{"agente":"simulado","modelo":"rapido","esfuerzo":"alto","del_agente":true,"contexto_tokens":200000,"correccion":1,"correcciones":2}`, 1501),
		ev(5, "aviso", "el agente llegó a su límite", 2000), ev(6, "estado", "validando_salida", 3000),
		ev(7, "validacion", `{"problemas":[{"texto":"falta total"},{"texto":"otro"},{"texto":"y otro"}]}`, 3100), ev(8, "estado", "completado", 4000),
	}
	texto := p.textoRegistro(trabajo("a", "uno", nucleo.Completado))
	for _, quiere := range []string{
		"En cola 1,5 s · Ejecución 1,5 s · Validación 1 s", // las etapas arriba
		"+0 ms  En cola", "+10 ms  Entrada: ~200.000 tokens (notas.pdf ~150.000)",
		"+1,5 s  Ejecutando · corrección 1 de 2 · simulado/rapido · alto (del agente) · contexto 200.000",
		"+2 s  Aviso: el agente llegó a su límite", "+3 s  Validando", "No cumple el esquema: falta total (y 2 más)", "+4 s  Completado",
	} {
		if !strings.Contains(texto, quiere) {
			t.Fatalf("falta %q:\n%s", quiere, texto)
		}
	}
	if got := p.esfuerzoUsado(trabajo("a", "uno", nucleo.Completado)); got != "Alto (del agente)" {
		t.Fatalf("esfuerzo del último intento: %q", got)
	}
	p.registro = nil
	t2 := trabajo("a", "uno", nucleo.Completado)
	t2.Solicitud.Esfuerzo = "medio"
	if got := p.esfuerzoUsado(t2); got != "Medio" {
		t.Fatalf("sin intentos, el de la solicitud: %q", got)
	}
	p.errRegistro = true
	if got := p.textoRegistro(t2); got != "No se pudo cargar el registro." {
		t.Fatalf("si falla: %q", got)
	}
}
