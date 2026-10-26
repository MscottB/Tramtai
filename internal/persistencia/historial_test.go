package persistencia

import (
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

func TestOrigenSeGuarda(t *testing.T) {
	a := abrir(t)
	tr := trabajo("global", "hola")
	tr.Origen = nucleo.DesdeCLI
	id := aceptar(t, a, tr, "").ID
	if leido, err := a.Obtener(ctx, id); err != nil || leido.Origen != "cli" {
		t.Fatalf("el origen se guarda: %q %v", leido.Origen, err)
	}
}

func TestListarConFiltro(t *testing.T) {
	a := abrir(t)
	base := time.Now()
	var creados []time.Time
	for i, c := range []struct{ punto, origen string }{{"", "probar"}, {"", "cli"}, {"tienda", "api"}, {"soporte", "api"}} {
		cuando := base.Add(time.Duration(i) * time.Second)
		a.reloj = func() time.Time { return cuando }
		tr := trabajo("global", "hola")
		tr.PuntoAcceso, tr.Origen = c.punto, c.origen
		aceptar(t, a, tr, "")
		creados = append(creados, cuando)
	}
	if l, _ := a.Listar(ctx, "global", FiltroTrabajos{Origen: "cli"}, 10); len(l) != 1 || l[0].Origen != "cli" {
		t.Fatalf("por origen: %+v", l)
	}
	if l, _ := a.Listar(ctx, "global", FiltroTrabajos{Puntos: []string{"tienda", "soporte"}}, 10); len(l) != 2 {
		t.Fatalf("por lista de APIs: %d", len(l))
	}
	if l, _ := a.Listar(ctx, "global", FiltroTrabajos{Antes: creados[2]}, 10); len(l) != 2 || l[0].Origen != "cli" || l[1].Origen != "probar" {
		t.Fatalf("antes del tercero, los dos primeros: %+v", l)
	}
}

// El Historial filtra por estado y busca en el texto de la petición en la base, no solo en lo cargado.
func TestListarPorEstadoYTexto(t *testing.T) {
	a := abrir(t)
	hecho := aceptar(t, a, trabajo("global", "Resume la factura de marzo"), "").ID
	avanzar(t, a, hecho, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.ValidandoSalida, nucleo.Completado)
	fallo := aceptar(t, a, trabajo("global", "Traduce 100% del contrato_final"), "").ID
	avanzar(t, a, fallo, nucleo.Fallido)
	enCola := aceptar(t, a, trabajo("global", "otra FACTURA"), "").ID
	reunion := aceptar(t, a, trabajo("global", "Notas de la Reunión"), "").ID
	avanzar(t, a, reunion, nucleo.CancelacionSolicitada, nucleo.Cancelado)
	// Una corrección añade la respuesta y el pedido de corrección; la fila enseña el mensaje original.
	corregida := trabajo("global", "Clasifica el ticket de envío")
	corregida.Solicitud.Mensajes = append(corregida.Solicitud.Mensajes, nucleo.Mensaje{Rol: "asistente", Contenido: "{}"},
		nucleo.Mensaje{Rol: "usuario", Contenido: "Tu respuesta no cumple el formato pedido:\n- falta prioridad"})
	corregida.Huella = corregida.Solicitud.Huella()
	corregida = aceptar(t, a, corregida, "")
	avanzar(t, a, corregida.ID, nucleo.Fallido)
	// Si la conversación acaba en el asistente, la fila enseña el último mensaje del usuario.
	conRespuesta := trabajo("global", "Revisa el presupuesto anual")
	conRespuesta.Solicitud.Mensajes = append(conRespuesta.Solicitud.Mensajes, nucleo.Mensaje{Rol: "asistente", Contenido: "Revisado"})
	conRespuesta.Huella = conRespuesta.Solicitud.Huella()
	conRespuesta = aceptar(t, a, conRespuesta, "")
	avanzar(t, a, conRespuesta.ID, nucleo.Fallido)
	ids := func(f FiltroTrabajos) []string {
		l, err := a.Listar(ctx, "global", f, 10)
		if err != nil {
			t.Fatal(err)
		}
		var salida []string
		for _, tr := range l {
			salida = append(salida, tr.ID)
		}
		return salida
	}
	casos := []struct {
		nombre string
		f      FiltroTrabajos
		quiere []string
	}{
		{"completadas", FiltroTrabajos{Estados: nucleo.EstadosDe(nucleo.FiltroCompletado)}, []string{hecho}},
		{"en curso", FiltroTrabajos{Estados: nucleo.EstadosDe(nucleo.FiltroEnCurso)}, []string{enCola}},
		{"el texto, sin distinguir mayúsculas", FiltroTrabajos{Texto: "factura"}, []string{enCola, hecho}},
		{"texto y estado", FiltroTrabajos{Texto: "factura", Estados: nucleo.EstadosDe(nucleo.FiltroEnCurso)}, []string{enCola}},
		{"% y _ son letras", FiltroTrabajos{Texto: "100% del contrato_"}, []string{fallo}},
		{"% solo no es comodín", FiltroTrabajos{Texto: "%"}, []string{fallo}},
		{"por el id", FiltroTrabajos{Texto: hecho[:12]}, []string{hecho}},
		{"sin coincidencias", FiltroTrabajos{Texto: "nada"}, nil},
		{"sin tildes ni mayúsculas, como la búsqueda de la terminal", FiltroTrabajos{Texto: "TRADUCE 100"}, []string{fallo}},
		{"la tilde no cuenta", FiltroTrabajos{Texto: "reunión"}, []string{reunion}},
		{"tampoco al revés", FiltroTrabajos{Texto: "REUNION"}, []string{reunion}},
		{"el mensaje original de una corrección", FiltroTrabajos{Texto: "ticket"}, []string{corregida.ID}},
		{"el del usuario, no la respuesta del asistente", FiltroTrabajos{Texto: "presupuesto"}, []string{conRespuesta.ID}},
		{"no la respuesta del asistente", FiltroTrabajos{Texto: "Revisado"}, nil},
	}
	for _, c := range casos {
		if got := ids(c.f); !slices.Equal(got, c.quiere) {
			t.Errorf("%s: %v, quería %v", c.nombre, got, c.quiere)
		}
	}
}

func TestPurgarConTope(t *testing.T) {
	a := abrir(t)
	base := time.Now()
	var ids []string
	for i := range 5 {
		cuando := base.Add(time.Duration(i) * time.Minute)
		a.reloj = func() time.Time { return cuando }
		id := aceptar(t, a, trabajo("global", "hola"), "").ID
		a.AgregarEvento(ctx, id, "texto", "x")
		avanzar(t, a, id, nucleo.Fallido)
		ids = append(ids, id)
	}
	activo := aceptar(t, a, trabajo("global", "sigue"), "").ID
	a.reloj = func() time.Time { return base.Add(10 * time.Minute) }
	if _, eliminados, err := a.Purgar(ctx, "global", Retencion{Resultado: 24 * time.Hour, Metadatos: 168 * time.Hour, Maximo: 2}); err != nil || eliminados != 3 {
		t.Fatalf("el tope cuenta lo que borra: %d %v", eliminados, err)
	}
	quedan := map[string]bool{}
	l, _ := a.Listar(ctx, "global", FiltroTrabajos{}, 100)
	for _, tr := range l {
		quedan[tr.ID] = true
	}
	if len(l) != 3 || !quedan[ids[3]] || !quedan[ids[4]] || !quedan[activo] {
		t.Fatalf("quedan los 2 terminados más recientes y el activo: %v", quedan)
	}
	if ev, _ := a.Eventos(ctx, ids[0], 0); len(ev) != 0 {
		t.Fatalf("los eventos del borrado también se van: %d", len(ev))
	}
}

func TestAutoVacuumIncremental(t *testing.T) {
	modo := func(a *Almacen) (n int) {
		a.db.QueryRow("PRAGMA auto_vacuum").Scan(&n)
		return n
	}
	if nueva := abrir(t); modo(nueva) != 2 || nueva.Compactada {
		t.Fatalf("una base nueva nace incremental, sin VACUUM: %d %t", modo(nueva), nueva.Compactada)
	}
	ruta := filepath.Join(t.TempDir(), "vieja.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(ruta))
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("CREATE TABLE previa (x INTEGER)")
	db.Close()
	a, err := Abrir(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Cerrar()
	if n := modo(a); n != 2 || !a.Compactada {
		t.Fatalf("una base vieja pasa a incremental y lo indica: %d %t", n, a.Compactada)
	}
	a.Cerrar()
	if b, _ := Abrir(ruta); b == nil || b.Compactada {
		t.Fatal("al reabrirla ya no hay que compactar")
	} else {
		b.Cerrar()
	}
}

func TestTopeSinSobrantes(t *testing.T) {
	a := abrir(t)
	for range 2 {
		avanzar(t, a, aceptar(t, a, trabajo("global", "hola"), "").ID, nucleo.Fallido)
	}
	if _, eliminados, err := a.Purgar(ctx, "global", Retencion{Resultado: 24 * time.Hour, Metadatos: 168 * time.Hour, Maximo: 5}); err != nil || eliminados != 0 {
		t.Fatalf("con menos terminados que el tope no se borra nada: %d %v", eliminados, err)
	}
}

func TestUsoHistorial(t *testing.T) {
	a := abrir(t)
	for range 3 {
		aceptar(t, a, trabajo("global", "hola"), "")
	}
	aceptar(t, a, trabajo("otra", "hola"), "")
	if uso, err := a.UsoHistorial(ctx, "global"); err != nil || uso.Trabajos != 3 || uso.Bytes <= 0 {
		t.Fatalf("%+v %v", uso, err)
	}
}

func TestListarEmpatesEnElCorte(t *testing.T) {
	a := abrir(t)
	mismo := time.Now()
	a.reloj = func() time.Time { return mismo }
	ids := map[string]bool{}
	for range 3 { // tres trabajos con el mismo creado_en: el reloj de Windows avanza a saltos
		ids[aceptar(t, a, trabajo("global", "hola"), "").ID] = true
	}
	primera, _ := a.Listar(ctx, "global", FiltroTrabajos{}, 2)
	ultimo := primera[len(primera)-1]
	resto, _ := a.Listar(ctx, "global", FiltroTrabajos{Antes: ultimo.CreadoEn, AntesID: ultimo.ID}, 2)
	vistos := map[string]bool{}
	for _, tr := range append(primera, resto...) {
		vistos[tr.ID] = true
	}
	if len(vistos) != 3 || len(primera)+len(resto) != 3 {
		t.Fatalf("la página siguiente trae el empatado que faltaba, sin repetir: %d + %d, %d distintos", len(primera), len(resto), len(vistos))
	}
}

func TestTopeLiberaLaClaveDeIdempotencia(t *testing.T) {
	a := abrir(t)
	base := time.Now()
	a.reloj = func() time.Time { return base }
	primero := aceptar(t, a, trabajo("global", "hola"), "clave-1")
	avanzar(t, a, primero.ID, nucleo.Fallido)
	for i := 1; i <= 2; i++ {
		cuando := base.Add(time.Duration(i) * time.Minute)
		a.reloj = func() time.Time { return cuando }
		avanzar(t, a, aceptar(t, a, trabajo("global", "otro"), "").ID, nucleo.Fallido)
	}
	if _, _, err := a.Purgar(ctx, "global", Retencion{Resultado: 24 * time.Hour, Metadatos: 168 * time.Hour, Maximo: 1}); err != nil {
		t.Fatal(err)
	}
	otra, reutilizado, err := a.Aceptar(ctx, trabajo("global", "hola"), "clave-1", 0)
	if err != nil || reutilizado || otra.ID == primero.ID {
		t.Fatalf("la clave del trabajo borrado por el tope queda libre: %v reutilizado=%t", err, reutilizado)
	}
}

func TestPuntosConConsumo(t *testing.T) {
	a := abrir(t)
	for _, punto := range []string{"tienda", "vieja", ""} {
		tr := trabajo("global", "hola")
		tr.PuntoAcceso = punto
		aceptar(t, a, tr, "")
	}
	a.EliminarConsumo(ctx, "global", "vieja") // sus filas pasan a «~»
	if puntos, err := a.PuntosConConsumo(ctx, "global"); err != nil || len(puntos) != 1 || puntos[0] != "tienda" {
		t.Fatalf("solo las APIs con nombre, sin '', '*' ni '~': %q %v", puntos, err)
	}
}
