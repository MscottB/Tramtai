package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// hastaLaFoto lee los eventos de la foto inicial: id → etapa.
func hastaLaFoto(t *testing.T, eventos <-chan map[string]any) map[string]string {
	t.Helper()
	vistos := map[string]string{}
	for {
		e := esperarEvento(t, eventos, func(map[string]any) bool { return true })
		if e["foto"] == true {
			return vistos
		}
		vistos[e["id"].(string)] = e["etapa"].(string)
	}
}

// flujoVivo abre /api/v1/eventos y entrega cada evento trabajo por el canal hasta que se cierra la prueba; el fin de la
// foto llega como {"foto": true}.
func flujoVivo(t *testing.T, ts *httptest.Server) (<-chan map[string]any, *http.Response) {
	t.Helper()
	ctx, cancelar := context.WithCancel(context.Background())
	t.Cleanup(cancelar)
	req, err := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/eventos", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secreto")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	eventos := make(chan map[string]any, 100)
	if resp.StatusCode != 200 {
		close(eventos)
		return eventos, resp
	}
	go func() {
		defer resp.Body.Close()
		defer close(eventos)
		lector := bufio.NewScanner(resp.Body)
		tipo := ""
		for lector.Scan() {
			linea := lector.Text()
			switch {
			case strings.HasPrefix(linea, "event: "):
				tipo = strings.TrimPrefix(linea, "event: ")
			case strings.HasPrefix(linea, "data: ") && tipo == "foto":
				eventos <- map[string]any{"foto": true}
			case strings.HasPrefix(linea, "data: ") && tipo == "trabajo":
				var e map[string]any
				if json.Unmarshal([]byte(strings.TrimPrefix(linea, "data: ")), &e) == nil {
					eventos <- e
				}
			}
		}
	}()
	return eventos, resp
}

// esperarEvento lee eventos hasta que uno cumple cond (o falla a los 10 s).
func esperarEvento(t *testing.T, eventos <-chan map[string]any, cond func(map[string]any) bool) map[string]any {
	t.Helper()
	plazo := time.After(10 * time.Second)
	for {
		select {
		case e, ok := <-eventos:
			if !ok {
				t.Fatal("el flujo se cerró")
			}
			if cond(e) {
				return e
			}
		case <-plazo:
			t.Fatal("no llegó el evento esperado")
		}
	}
}

func crearHola(t *testing.T, ts *httptest.Server, cuerpo string) string {
	t.Helper()
	_, c := pedir(t, ts, "POST", "/api/v1/trabajos", cuerpo)
	return decodificar(t, c)["id"].(string)
}

func TestEtapaDe(t *testing.T) {
	casos := map[nucleo.Estado]string{
		nucleo.EnCola: "cola", nucleo.PreparandoEntrada: "preparacion", nucleo.EsperandoAgente: "preparacion",
		nucleo.Ejecutando: "ejecucion", nucleo.CancelacionSolicitada: "ejecucion", nucleo.ValidandoSalida: "validacion",
		nucleo.Completado: "llegada", nucleo.Fallido: "llegada", nucleo.Cancelado: "llegada", nucleo.Interrumpido: "llegada",
	}
	for estado, etapa := range casos {
		if got := EtapaDe(estado); got != etapa {
			t.Errorf("%s → %s, quería %s", estado, got, etapa)
		}
	}
}

func TestEventosFotoInicial(t *testing.T) {
	ts := servidor(t, map[string]adaptadores.Adaptador{"simulado": adaptadores.Simulado{}, "lento": adaptadores.Simulado{Demora: time.Minute}}, nil)
	hecho := crearHola(t, ts, solicitudHola)
	esperarEstado(t, ts, hecho, nucleo.Completado)
	enCurso := crearHola(t, ts, `{"agente":"lento","mensajes":[{"rol":"usuario","contenido":"hola"}]}`)
	esperarEstado(t, ts, enCurso, nucleo.Ejecutando)
	eventos, _ := flujoVivo(t, ts)
	if vistos := hastaLaFoto(t, eventos); len(vistos) != 2 || vistos[hecho] != "llegada" || vistos[enCurso] != "ejecucion" {
		t.Fatalf("foto = %v", vistos)
	}
}

func TestEventosFotoPorPaginas(t *testing.T) {
	antes := persistencia.LimiteCambios
	persistencia.LimiteCambios = 2
	t.Cleanup(func() { persistencia.LimiteCambios = antes })
	ts := servidor(t, map[string]adaptadores.Adaptador{"lento": adaptadores.Simulado{Demora: time.Minute}}, nil)
	for i := 0; i < 5; i++ {
		crearHola(t, ts, `{"agente":"lento","mensajes":[{"rol":"usuario","contenido":"hola"}]}`)
	}
	eventos, _ := flujoVivo(t, ts)
	if vistos := hastaLaFoto(t, eventos); len(vistos) != 5 {
		t.Fatalf("con páginas de 2, la foto debe traer los 5: %v", vistos)
	}
}

func TestEventosAvisosSeguidosUnaLectura(t *testing.T) {
	var lecturas atomic.Int32
	original := leerCambios
	leerCambios = func(s *Servidor, ctx context.Context, punto string, desde time.Time) ([]nucleo.Trabajo, error) {
		lecturas.Add(1)
		return original(s, ctx, punto, desde)
	}
	t.Cleanup(func() { leerCambios = original })
	s := nuevoServidor(t, simulado(adaptadores.Simulado{}), nil, nil)
	s.Sondeo = time.Hour // solo despiertan los avisos
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	eventos, _ := flujoVivo(t, ts)
	hastaLaFoto(t, eventos)
	inicio := time.Now()
	var ids []string
	for i := 0; i < 10; i++ { // cada trabajo cambia varias veces: decenas de avisos
		ids = append(ids, crearHola(t, ts, solicitudHola))
	}
	for _, id := range ids {
		esperarEstado(t, ts, id, nucleo.Completado)
	}
	time.Sleep(500 * time.Millisecond)
	if maximo := int32(time.Since(inicio)/esperaLecturas) + 2; lecturas.Load() > maximo {
		t.Fatalf("%d lecturas en %v: los avisos seguidos deben juntarse (máximo %d)", lecturas.Load(), time.Since(inicio), maximo)
	}
}

func TestOlvidarVistos(t *testing.T) {
	ahora := time.Now()
	vistos := map[string]visto{
		"viejo":    {estado: "completado", terminado: true, cambio: ahora.Add(-time.Minute)},
		"reciente": {estado: "completado", terminado: true, cambio: ahora},
		"vivo":     {estado: "ejecutando", cambio: ahora.Add(-time.Hour)},
	}
	olvidar(vistos, ahora.Add(-time.Second))
	if _, ok := vistos["viejo"]; ok || len(vistos) != 2 {
		t.Fatalf("solo se olvidan los terminados de antes del límite: %v", vistos)
	}
}

func TestEventosCambioDeEstado(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{Demora: 50 * time.Millisecond}), nil)
	eventos, _ := flujoVivo(t, ts)
	id := crearHola(t, ts, solicitudHola)
	e := esperarEvento(t, eventos, func(e map[string]any) bool { return e["id"] == id && e["etapa"] == "llegada" })
	if e["estado"] != "completado" || e["agente"] != "simulado" || e["intento"] != float64(1) {
		t.Fatalf("evento final = %v", e)
	}
}

func TestEventosTrabajoRapido(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	eventos, _ := flujoVivo(t, ts)
	id := crearHola(t, ts, solicitudHola)
	// Aunque pase de la cola al final entre dos lecturas, el estado final llega.
	esperarEvento(t, eventos, func(e map[string]any) bool { return e["id"] == id && e["estado"] == "completado" })
}

func TestEventosRespaldoSubeIntento(t *testing.T) {
	registro := map[string]adaptadores.Adaptador{
		"simulado": adaptadores.Simulado{Demora: 200 * time.Millisecond, Error: nucleo.NuevoError(nucleo.CodigoLimiteUsoProveedor, "límite")},
		"otro":     adaptadores.Simulado{Demora: 200 * time.Millisecond, Respuesta: "ok"},
	}
	ts := servidor(t, registro, nil)
	eventos, _ := flujoVivo(t, ts)
	id := crearHola(t, ts, `{"agente":"simulado","respaldo":["otro"],"mensajes":[{"rol":"usuario","contenido":"hola"}]}`)
	e := esperarEvento(t, eventos, func(e map[string]any) bool { return e["id"] == id && e["agente"] == "otro" })
	if e["intento"] != float64(2) {
		t.Fatalf("tras el respaldo, intento debe ser 2: %v", e)
	}
}

func TestEventosSoloSusTrabajos(t *testing.T) {
	s := nuevoServidor(t, simulado(adaptadores.Simulado{}), nil, nil)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	ajeno := crearHola(t, ts, solicitudHola)
	esperarEstado(t, ts, ajeno, nucleo.Completado)
	s.Punto = &Punto{Nombre: "otra"} // desde aquí la API es la de un punto de acceso
	eventos, _ := flujoVivo(t, ts)
	propio := crearHola(t, ts, solicitudHola)
	esperarEvento(t, eventos, func(e map[string]any) bool {
		if e["id"] == ajeno {
			t.Fatalf("un punto de acceso no ve los trabajos de otro: %v", e)
		}
		return e["id"] == propio && e["etapa"] == "llegada" && e["api"] == "otra"
	})
}

func TestEventosSinContenido(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	eventos, _ := flujoVivo(t, ts)
	id := crearHola(t, ts, solicitudHola)
	e := esperarEvento(t, eventos, func(e map[string]any) bool { return e["id"] == id && e["etapa"] == "llegada" })
	var claves []string
	for k := range e {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	if strings.Join(claves, ",") != "actualizado,agente,api,codigo_error,creado,estado,etapa,id,intento,modelo,origen" {
		t.Fatalf("claves del evento: %v", claves)
	}
	if creado, _ := time.Parse(time.RFC3339Nano, e["creado"].(string)); creado.IsZero() || creado.After(time.Now()) {
		t.Fatalf("creado = %v", e["creado"])
	}
}

func TestEventosTope(t *testing.T) {
	ts := servidor(t, simulado(adaptadores.Simulado{}), nil)
	var cierres []func()
	for i := 0; i < 8; i++ {
		ctx, cancelar := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL+"/api/v1/eventos", nil)
		req.Header.Set("Authorization", "Bearer secreto")
		resp, err := ts.Client().Do(req)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("flujo %d: %v %v", i+1, err, resp)
		}
		cierres = append(cierres, func() { cancelar(); resp.Body.Close() })
	}
	t.Cleanup(func() {
		for _, c := range cierres {
			c()
		}
	})
	r, cuerpo := pedir(t, ts, "GET", "/api/v1/eventos", "")
	if r.StatusCode != 429 || r.Header.Get("Retry-After") != "5" || !strings.Contains(cuerpo, "limite_flujos") {
		t.Fatalf("el noveno: %d %q %s", r.StatusCode, r.Header.Get("Retry-After"), cuerpo)
	}
	cierres[0]()
	limite := time.Now().Add(5 * time.Second)
	for {
		_, resp := flujoVivo(t, ts)
		if resp.StatusCode == 200 {
			return
		}
		if time.Now().After(limite) {
			t.Fatal("al cerrar uno no se liberó el cupo")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
