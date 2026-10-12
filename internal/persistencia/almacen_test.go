package persistencia

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

var ctx = context.Background()

func abrir(t *testing.T) *Almacen {
	t.Helper()
	a, err := Abrir(filepath.Join(t.TempDir(), "instancia.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Cerrar() })
	return a
}

func trabajo(instancia, texto string) nucleo.Trabajo {
	s := nucleo.SolicitudEjecucion{Agente: "simulado", Mensajes: []nucleo.Mensaje{{Rol: "usuario", Contenido: texto}}}
	return nucleo.Trabajo{ID: nucleo.NuevoID(), Instancia: instancia, Solicitud: s, Huella: s.Huella()}
}

func aceptar(t *testing.T, a *Almacen, tr nucleo.Trabajo, clave string) nucleo.Trabajo {
	t.Helper()
	aceptado, _, err := a.Aceptar(ctx, tr, clave, 0)
	if err != nil {
		t.Fatal(err)
	}
	return aceptado
}

func avanzar(t *testing.T, a *Almacen, id string, estados ...nucleo.Estado) {
	t.Helper()
	actual, err := a.Obtener(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, siguiente := range estados {
		if err := a.CambiarEstado(ctx, id, actual.Estado, siguiente, Cambio{}); err != nil {
			t.Fatalf("%s → %s: %v", actual.Estado, siguiente, err)
		}
		actual.Estado = siguiente
	}
}

func TestAceptacionPersisteTrasReabrir(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "instancia.db")
	a, err := Abrir(ruta)
	if err != nil {
		t.Fatal(err)
	}
	original := aceptar(t, a, trabajo("global", "hola"), "")
	a.Cerrar()
	b, err := Abrir(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cerrar()
	leido, err := b.Obtener(ctx, original.ID)
	if err != nil || leido.Estado != nucleo.EnCola || leido.Solicitud.Mensajes[0].Contenido != "hola" || leido.Huella != original.Huella {
		t.Fatalf("trabajo releído incorrecto: %+v, %v", leido, err)
	}
	if eventos, _ := b.Eventos(ctx, original.ID, 0); len(eventos) != 1 || eventos[0].Datos != "en_cola" {
		t.Fatalf("se esperaba un evento en_cola: %+v", eventos)
	}
}

func TestIdempotencia(t *testing.T) {
	a := abrir(t)
	primero := aceptar(t, a, trabajo("global", "hola"), "clave-1")
	repetido, reutilizado, err := a.Aceptar(ctx, trabajo("global", "hola"), "clave-1", 0)
	if err != nil || !reutilizado || repetido.ID != primero.ID {
		t.Fatalf("misma clave y huella deben devolver el trabajo original: %v %v", reutilizado, err)
	}
	if _, _, err := a.Aceptar(ctx, trabajo("global", "otro texto"), "clave-1", 0); nucleo.CodigoDe(err) != nucleo.CodigoConflictoIdempotencia {
		t.Fatalf("otra huella con la misma clave debe dar conflicto: %v", err)
	}
	if _, reutilizado, err := a.Aceptar(ctx, trabajo("proyecto_x", "hola"), "clave-1", 0); err != nil || reutilizado {
		t.Fatalf("la clave se delimita por instancia: %v %v", reutilizado, err)
	}
}

func TestVentanaIdempotenciaTrasTerminar(t *testing.T) {
	a := abrir(t)
	primero := aceptar(t, a, trabajo("global", "hola"), "clave-1")
	avanzar(t, a, primero.ID, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.ValidandoSalida, nucleo.Completado)
	if _, reutilizado, _ := a.Aceptar(ctx, trabajo("global", "hola"), "clave-1", 0); !reutilizado {
		t.Fatal("dentro de la ventana la clave sigue vinculada al trabajo terminado")
	}
	a.reloj = func() time.Time { return time.Now().Add(25 * time.Hour) }
	nuevo, reutilizado, err := a.Aceptar(ctx, trabajo("global", "hola"), "clave-1", 0)
	if err != nil || reutilizado || nuevo.ID == primero.ID {
		t.Fatalf("vencida la ventana, la clave representa una ejecución nueva: %v %v", reutilizado, err)
	}
}

func TestColaLlenaNoReservaClave(t *testing.T) {
	a := abrir(t)
	for i := 0; i < 2; i++ {
		if _, _, err := a.Aceptar(ctx, trabajo("global", "t"), "", 2); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := a.Aceptar(ctx, trabajo("global", "t"), "clave-llena", 2); nucleo.CodigoDe(err) != nucleo.CodigoColaLlena {
		t.Fatalf("se esperaba cola_llena: %v", err)
	}
	if _, reutilizado, err := a.Aceptar(ctx, trabajo("global", "t"), "clave-llena", 0); err != nil || reutilizado {
		t.Fatalf("una solicitud rechazada no reserva la clave: %v %v", reutilizado, err)
	}
}

func TestTransicionesYEventos(t *testing.T) {
	a := abrir(t)
	tr := aceptar(t, a, trabajo("global", "hola"), "")
	if err := a.CambiarEstado(ctx, tr.ID, nucleo.EnCola, nucleo.Completado, Cambio{}); nucleo.CodigoDe(err) != nucleo.CodigoTransicionInvalida {
		t.Fatalf("en_cola → completado no está en la tabla: %v", err)
	}
	if err := a.CambiarEstado(ctx, tr.ID, nucleo.EsperandoAgente, nucleo.Ejecutando, Cambio{}); nucleo.CodigoDe(err) != nucleo.CodigoTransicionInvalida {
		t.Fatalf("un estado de origen desactualizado debe rechazarse: %v", err)
	}
	if err := a.CambiarEstado(ctx, "trabajo_inexistente", nucleo.EnCola, nucleo.EsperandoAgente, Cambio{}); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("se esperaba no_encontrado: %v", err)
	}
	avanzar(t, a, tr.ID, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.ValidandoSalida)
	salida := "resultado"
	if err := a.CambiarEstado(ctx, tr.ID, nucleo.ValidandoSalida, nucleo.Completado, Cambio{Salida: &salida}); err != nil {
		t.Fatal(err)
	}
	final, _ := a.Obtener(ctx, tr.ID)
	if final.Salida != "resultado" || final.TerminadoEn == nil {
		t.Fatalf("el trabajo completado debe guardar salida y fecha de fin: %+v", final)
	}
	eventos, _ := a.Eventos(ctx, tr.ID, 0)
	for i, e := range eventos {
		if e.Numero != i+1 {
			t.Fatalf("los eventos deben numerarse de forma consecutiva: %+v", eventos)
		}
	}
	if desde, _ := a.Eventos(ctx, tr.ID, 3); len(desde) != len(eventos)-3 {
		t.Fatal("Eventos debe devolver solo los posteriores al cursor")
	}
}

func TestRecuperarTrasReinicio(t *testing.T) {
	a := abrir(t)
	enCola := aceptar(t, a, trabajo("global", "1"), "")
	esperando := aceptar(t, a, trabajo("global", "2"), "")
	ejecutando := aceptar(t, a, trabajo("global", "3"), "")
	cancelando := aceptar(t, a, trabajo("global", "4"), "")
	completado := aceptar(t, a, trabajo("global", "5"), "")
	avanzar(t, a, esperando.ID, nucleo.EsperandoAgente)
	avanzar(t, a, ejecutando.ID, nucleo.EsperandoAgente, nucleo.Ejecutando)
	avanzar(t, a, cancelando.ID, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.CancelacionSolicitada)
	avanzar(t, a, completado.ID, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.ValidandoSalida, nucleo.Completado)

	interrumpidos, reencolados, err := a.Recuperar(ctx, "global")
	if err != nil || interrumpidos != 2 || reencolados != 1 {
		t.Fatalf("recuperar: %d interrumpidos, %d reencolados, %v", interrumpidos, reencolados, err)
	}
	esperado := map[string]nucleo.Estado{enCola.ID: nucleo.EnCola, esperando.ID: nucleo.EnCola,
		ejecutando.ID: nucleo.Interrumpido, cancelando.ID: nucleo.Interrumpido, completado.ID: nucleo.Completado}
	for id, estado := range esperado {
		if tr, _ := a.Obtener(ctx, id); tr.Estado != estado {
			t.Errorf("%s: %s, se esperaba %s", id, tr.Estado, estado)
		}
	}
}

func TestVersionDeBaseNoSoportada(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "instancia.db")
	a, err := Abrir(ruta)
	if err != nil {
		t.Fatal(err)
	}
	a.db.Exec("PRAGMA user_version = 99")
	a.Cerrar()
	if b, err := Abrir(ruta); err == nil {
		b.Cerrar()
		t.Fatal("una base de una versión futura debe rechazarse")
	}
}

func TestMigracionV1AV2ConservaTrabajos(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "instancia.db")
	a, err := Abrir(ruta)
	if err != nil {
		t.Fatal(err)
	}
	// Recrea una base v1: solo el esquema inicial con un trabajo.
	a.db.Exec("DROP TABLE trabajos")
	a.db.Exec("DROP TABLE eventos")
	a.db.Exec("DROP TABLE idempotencia")
	if _, err := a.db.Exec(migraciones[0]); err != nil {
		t.Fatal(err)
	}
	a.db.Exec("PRAGMA user_version = 1")
	if _, err := a.db.Exec(`INSERT INTO trabajos (id, instancia, estado, solicitud, huella, creado_en, actualizado_en)
		VALUES ('trabajo_v1', 'global', 'completado', '{}', 'h', '2026-10-12T00:00:00.000000000Z', '2026-10-12T00:00:00.000000000Z')`); err != nil {
		t.Fatal(err)
	}
	a.Cerrar()

	b, err := Abrir(ruta)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Cerrar()
	if tr, err := b.Obtener(ctx, "trabajo_v1"); err != nil || tr.Estado != nucleo.Completado || tr.ModeloEnviado != "" {
		t.Fatalf("el trabajo v1 debe leerse tras migrar: %+v %v", tr, err)
	}
}

func TestUsoYModeloSeGuardan(t *testing.T) {
	a := abrir(t)
	tr := trabajo("global", "x")
	tr.ModeloEnviado = "haiku"
	tr = aceptar(t, a, tr, "")
	avanzar(t, a, tr.ID, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.ValidandoSalida)
	salida := "ok"
	uso := nucleo.Uso{TokensEntrada: 650, TokensSalida: 12, CostoUSD: 0.0014}
	if err := a.CambiarEstado(ctx, tr.ID, nucleo.ValidandoSalida, nucleo.Completado,
		Cambio{Salida: &salida, Uso: &uso, ModeloEfectivo: "claude-haiku-4-5"}); err != nil {
		t.Fatal(err)
	}
	final, _ := a.Obtener(ctx, tr.ID)
	if final.ModeloEnviado != "haiku" || final.ModeloEfectivo != "claude-haiku-4-5" || final.Uso != uso {
		t.Fatalf("modelo o uso no guardados: %+v", final)
	}
}

func TestPurgaPorRetencion(t *testing.T) {
	a := abrir(t)
	r := Retencion{Resultado: 24 * time.Hour, Metadatos: 168 * time.Hour}
	terminado := trabajo("global", "secreto")
	terminado.Solicitud.Cargas = []string{"carga_a"}
	terminado = aceptar(t, a, terminado, "clave")
	avanzar(t, a, terminado.ID, nucleo.EsperandoAgente, nucleo.Ejecutando, nucleo.ValidandoSalida)
	salida := "respuesta"
	if err := a.CambiarEstado(ctx, terminado.ID, nucleo.ValidandoSalida, nucleo.Completado, Cambio{Salida: &salida}); err != nil {
		t.Fatal(err)
	}
	a.AgregarEvento(ctx, terminado.ID, "texto", "respuesta")
	activo := trabajo("global", "otro")
	activo.Solicitud.Cargas = []string{"carga_b"}
	activo = aceptar(t, a, activo, "")

	if enUso, _ := a.CargasEnUso(ctx, "global"); !enUso["carga_a"] || !enUso["carga_b"] {
		t.Fatalf("antes de vencer, las dos cargas están en uso: %v", enUso)
	}
	if v, e, err := a.Purgar(ctx, "global", r); err != nil || v != 0 || e != 0 {
		t.Fatalf("dentro de la ventana no se purga nada: %d %d %v", v, e, err)
	}
	if tr, _ := a.Obtener(ctx, terminado.ID); tr.Disponibilidad != nucleo.SalidaDisponible {
		t.Fatalf("disponibilidad %q", tr.Disponibilidad)
	}

	ahora := time.Now()
	a.reloj = func() time.Time { return ahora.Add(25 * time.Hour) }
	if v, e, err := a.Purgar(ctx, "global", r); err != nil || v != 1 || e != 0 {
		t.Fatalf("vence el resultado: %d %d %v", v, e, err)
	}
	tr, _ := a.Obtener(ctx, terminado.ID)
	if tr.Disponibilidad != nucleo.SalidaExpirada || tr.Salida != "" || !tr.EntradasEliminadas || len(tr.Solicitud.Mensajes) != 0 || tr.Estado != nucleo.Completado {
		t.Fatalf("tras vencer: %+v", tr)
	}
	eventos, _ := a.Eventos(ctx, terminado.ID, 0)
	for _, e := range eventos {
		if e.Tipo != "estado" {
			t.Fatalf("quedó un evento con contenido: %+v", e)
		}
	}
	if enUso, _ := a.CargasEnUso(ctx, "global"); enUso["carga_a"] || !enUso["carga_b"] {
		t.Fatalf("solo el trabajo activo protege su carga: %v", enUso)
	}
	if _, reutilizado, _ := a.Aceptar(ctx, trabajo("global", "secreto"), "clave", 0); reutilizado {
		t.Fatal("la clave de idempotencia debía vencer con el resultado")
	}

	a.reloj = func() time.Time { return ahora.Add(170 * time.Hour) }
	if _, e, err := a.Purgar(ctx, "global", r); err != nil || e != 1 {
		t.Fatalf("se eliminan los metadatos: %d %v", e, err)
	}
	if _, err := a.Obtener(ctx, terminado.ID); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("tras los metadatos el trabajo no existe: %v", err)
	}
	if tr, err := a.Obtener(ctx, activo.ID); err != nil || tr.Estado != nucleo.EnCola {
		t.Fatalf("un trabajo activo nunca se purga: %+v %v", tr, err)
	}
}

func TestEliminarEntradasAlTerminar(t *testing.T) {
	a := abrir(t)
	tr := aceptar(t, a, trabajo("global", "secreto"), "")
	avanzar(t, a, tr.ID, nucleo.Fallido)
	if _, _, err := a.Purgar(ctx, "global", Retencion{Resultado: time.Hour, Metadatos: time.Hour, EliminarEntradas: true}); err != nil {
		t.Fatal(err)
	}
	tr, _ = a.Obtener(ctx, tr.ID)
	if !tr.EntradasEliminadas || len(tr.Solicitud.Mensajes) != 0 || tr.Disponibilidad == nucleo.SalidaExpirada {
		t.Fatalf("entradas borradas sin vencer el resultado: %+v", tr)
	}
}
