package persistencia

import (
	"errors"
	"sync"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

// conPresupuesto abre un almacén con reloj y zona fijos y los topes indicados.
func conPresupuesto(t *testing.T, ahora *time.Time, topes ...Tope) *Almacen {
	t.Helper()
	a := abrir(t)
	a.Zona = time.UTC
	a.reloj = func() time.Time { return *ahora }
	a.Topes = func(instancia, punto string) []Tope {
		var aplican []Tope
		for _, tope := range topes {
			if tope.Punto == TodaLaInstancia || tope.Punto == punto {
				aplican = append(aplican, tope)
			}
		}
		return aplican
	}
	return a
}

func enPunto(punto string) nucleo.Trabajo {
	tr := trabajo("global", "hola")
	tr.PuntoAcceso = punto
	return tr
}

// terminar lleva un trabajo en cola a fallido con el uso indicado.
func terminar(t *testing.T, a *Almacen, id string, uso nucleo.Uso) {
	t.Helper()
	if err := a.CambiarEstado(ctx, id, nucleo.EnCola, nucleo.Fallido, Cambio{Uso: &uso}); err != nil {
		t.Fatal(err)
	}
}

func consumoDe(t *testing.T, a *Almacen, punto string) (nucleo.Consumo, nucleo.Consumo) {
	t.Helper()
	dia, mes, err := a.Consumo(ctx, "global", punto)
	if err != nil {
		t.Fatal(err)
	}
	return dia, mes
}

func contarAvisos(t *testing.T, a *Almacen) int {
	t.Helper()
	var n int
	if err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM avisos_presupuesto`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestConsumoAceptarYTerminar(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora)
	primero := aceptar(t, a, enPunto("facturas"), "")
	aceptar(t, a, enPunto("facturas"), "")
	terminar(t, a, primero.ID, nucleo.Uso{TokensEntrada: 100, TokensSalida: 50, CostoUSD: 0.2})
	dia, mes := consumoDe(t, a, "facturas")
	if esperado := (nucleo.Consumo{Peticiones: 2, Terminados: 1, Tokens: 150, USD: 0.2}); dia != esperado || mes != esperado {
		t.Fatalf("día %+v mes %+v", dia, mes)
	}
}

func TestConsumoDiaDeAceptacion(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 23, 59, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora)
	tr := aceptar(t, a, enPunto("facturas"), "")
	ahora = ahora.Add(2 * time.Minute)
	terminar(t, a, tr.ID, nucleo.Uso{TokensEntrada: 10})
	if dia, mes := consumoDe(t, a, "facturas"); dia != (nucleo.Consumo{}) || mes != (nucleo.Consumo{Peticiones: 1, Terminados: 1, Tokens: 10}) {
		t.Fatalf("el día siguiente no debe tener consumo: día %+v mes %+v", dia, mes)
	}
	ahora = ahora.Add(-2 * time.Minute)
	if dia, _ := consumoDe(t, a, "facturas"); dia != (nucleo.Consumo{Peticiones: 1, Terminados: 1, Tokens: 10}) {
		t.Fatalf("los tokens van al día de aceptación: %+v", dia)
	}
}

func TestPresupuestoAgotadoEnAceptar(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora, Tope{"facturas", nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 2}}})
	aceptar(t, a, enPunto("facturas"), "")
	aceptar(t, a, enPunto("facturas"), "")
	tercero := enPunto("facturas")
	_, _, err := a.Aceptar(ctx, tercero, "", 0)
	var agotado *nucleo.PresupuestoAgotado
	if !errors.As(err, &agotado) || !agotado.Reinicio.Equal(time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("se esperaba presupuesto_agotado: %v", err)
	}
	if _, err := a.Obtener(ctx, tercero.ID); nucleo.CodigoDe(err) != nucleo.CodigoNoEncontrado {
		t.Fatalf("el trabajo rechazado no debe guardarse: %v", err)
	}
	aceptar(t, a, enPunto("otra"), "") // el tope es solo de facturas
}

func TestPresupuestoIdempotencia(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora, Tope{"facturas", nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 1}}})
	tr := enPunto("facturas")
	original := aceptar(t, a, tr, "clave")
	repetido, reutilizado, err := a.Aceptar(ctx, tr, "clave", 0)
	if err != nil || !reutilizado || repetido.ID != original.ID {
		t.Fatalf("la repetición debe devolver el original: %v %v", err, reutilizado)
	}
	if dia, _ := consumoDe(t, a, "facturas"); dia.Peticiones != 1 {
		t.Fatalf("la repetición no consume: %+v", dia)
	}
}

func TestPresupuestoConcurrente(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora, Tope{TodaLaInstancia, nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 1}}})
	var grupo sync.WaitGroup
	var mu sync.Mutex
	aceptados := 0
	for range 5 {
		grupo.Go(func() {
			if _, _, err := a.Aceptar(ctx, enPunto(""), "", 0); err == nil {
				mu.Lock()
				aceptados++
				mu.Unlock()
			}
		})
	}
	grupo.Wait()
	if aceptados != 1 {
		t.Fatalf("con 1 petición restante debe pasar exactamente 1; pasaron %d", aceptados)
	}
}

func TestPresupuestoReserva(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora, Tope{"facturas", nucleo.Presupuesto{Mensual: nucleo.Limites{Tokens: 1000}}})
	terminado := aceptar(t, a, enPunto("facturas"), "")
	terminar(t, a, terminado.ID, nucleo.Uso{TokensEntrada: 400})
	aceptar(t, a, enPunto("facturas"), "") // sin nada en curso: 400 < 1000
	aceptar(t, a, enPunto("facturas"), "") // 1 en curso: 400 + 1×400 = 800 < 1000
	if _, _, err := a.Aceptar(ctx, enPunto("facturas"), "", 0); nucleo.CodigoDe(err) != nucleo.CodigoPresupuestoAgotado {
		t.Fatalf("2 en curso: 400 + 2×400 ≥ 1000 debe rechazarse: %v", err)
	}
}

func TestPresupuestoInstancia(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora, Tope{TodaLaInstancia, nucleo.Presupuesto{Mensual: nucleo.Limites{Peticiones: 2}}})
	aceptar(t, a, enPunto(""), "")
	aceptar(t, a, enPunto("facturas"), "")
	if dia, mes := consumoDe(t, a, TodaLaInstancia); dia.Peticiones != 2 || mes.Peticiones != 2 {
		t.Fatalf("la instancia suma con y sin API: %+v %+v", dia, mes)
	}
	if _, _, err := a.Aceptar(ctx, enPunto("otra"), "", 0); nucleo.CodigoDe(err) != nucleo.CodigoPresupuestoAgotado {
		t.Fatalf("el presupuesto de la instancia frena cualquier API: %v", err)
	}
}

func TestAvisosPresupuesto(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora, Tope{"facturas", nucleo.Presupuesto{Diario: nucleo.Limites{Tokens: 100}}})
	tr := aceptar(t, a, enPunto("facturas"), "")
	terminar(t, a, tr.ID, nucleo.Uso{TokensEntrada: 85})
	if n := contarAvisos(t, a); n != 1 {
		t.Fatalf("al pasar el 80 %% debe haber 1 aviso; hay %d", n)
	}
	tr = aceptar(t, a, enPunto("facturas"), "")
	terminar(t, a, tr.ID, nucleo.Uso{TokensEntrada: 20})
	if n := contarAvisos(t, a); n != 2 {
		t.Fatalf("al pasar el 100 %% debe haber 2 avisos; hay %d", n)
	}
	var nivel int
	_ = a.db.QueryRowContext(ctx, `SELECT MAX(nivel) FROM avisos_presupuesto WHERE punto = 'facturas' AND periodo = 'D:2026-10-16'`).Scan(&nivel)
	if nivel != 100 {
		t.Fatalf("nivel %d", nivel)
	}
	ahora = ahora.Add(time.Minute)
	tr = aceptar(t, a, enPunto("otra"), "") // otra API sin tope: no registra avisos
	terminar(t, a, tr.ID, nucleo.Uso{TokensEntrada: 500})
	if n := contarAvisos(t, a); n != 2 {
		t.Fatalf("los avisos no se repiten ni aparecen sin tope: %d", n)
	}
}

func TestReiniciarConsumo(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora, Tope{"facturas", nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 1}}})
	enCurso := aceptar(t, a, enPunto("facturas"), "")
	if n := contarAvisos(t, a); n != 2 {
		t.Fatalf("al 100 %% de peticiones se registran los avisos de 80 y 100: %d", n)
	}
	if err := a.ReiniciarConsumo(ctx, "global", "facturas"); err != nil {
		t.Fatal(err)
	}
	if dia, mes := consumoDe(t, a, "facturas"); dia != (nucleo.Consumo{}) || mes != (nucleo.Consumo{}) {
		t.Fatalf("tras reiniciar el consumo es cero: %+v %+v", dia, mes)
	}
	if n := contarAvisos(t, a); n != 0 {
		t.Fatalf("tras reiniciar no quedan avisos del periodo: %d", n)
	}
	terminar(t, a, enCurso.ID, nucleo.Uso{TokensEntrada: 5})
	if dia, _ := consumoDe(t, a, "facturas"); dia != (nucleo.Consumo{Terminados: 1, Tokens: 5}) {
		t.Fatalf("el trabajo que seguía en curso suma al terminar: %+v", dia)
	}
	aceptar(t, a, enPunto("facturas"), "") // tras reiniciar vuelve a admitir
}

func TestReiniciarConsumoInstancia(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora)
	aceptar(t, a, enPunto(""), "")
	aceptar(t, a, enPunto("facturas"), "")
	if err := a.ReiniciarConsumo(ctx, "global", TodaLaInstancia); err != nil {
		t.Fatal(err)
	}
	if dia, _ := consumoDe(t, a, TodaLaInstancia); dia != (nucleo.Consumo{}) {
		t.Fatalf("reiniciar la instancia borra todas sus filas: %+v", dia)
	}
}

func TestEliminarConsumo(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora)
	aceptar(t, a, enPunto("facturas"), "")
	aceptar(t, a, enPunto("otra"), "")
	if err := a.EliminarConsumo(ctx, "global", "facturas"); err != nil {
		t.Fatal(err)
	}
	if dia, _ := consumoDe(t, a, "facturas"); dia != (nucleo.Consumo{}) {
		t.Fatalf("%+v", dia)
	}
	if dia, _ := consumoDe(t, a, "otra"); dia.Peticiones != 1 {
		t.Fatalf("las demás API conservan su consumo: %+v", dia)
	}
}

func TestPurgarConsumo(t *testing.T) {
	ahora := time.Date(2025, 9, 30, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora)
	aceptar(t, a, enPunto("facturas"), "") // hace 13 meses
	ahora = time.Date(2025, 10, 1, 12, 0, 0, 0, time.UTC)
	aceptar(t, a, enPunto("facturas"), "") // hace 12 meses
	ahora = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	if _, _, err := a.Purgar(ctx, "global", Retencion{Resultado: time.Hour, Metadatos: 24 * 400 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	var filas int
	_ = a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM consumo`).Scan(&filas)
	if filas != 1 {
		t.Fatalf("se conservan el mes en curso y los 12 anteriores: quedan %d filas", filas)
	}
}

func TestReiniciarAPIConservaLaInstancia(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora)
	aceptar(t, a, enPunto("facturas"), "")
	aceptar(t, a, enPunto("facturas"), "")
	aceptar(t, a, enPunto("otra"), "")
	if err := a.ReiniciarConsumo(ctx, "global", "facturas"); err != nil {
		t.Fatal(err)
	}
	if err := a.ReiniciarConsumo(ctx, "global", "facturas"); err != nil { // dos veces el mismo día
		t.Fatal(err)
	}
	if err := a.EliminarConsumo(ctx, "global", "otra"); err != nil {
		t.Fatal(err)
	}
	if dia, _ := consumoDe(t, a, TodaLaInstancia); dia.Peticiones != 3 {
		t.Fatalf("reiniciar o eliminar una API no devuelve consumo al presupuesto de la instancia: %+v", dia)
	}
	if dia, _ := consumoDe(t, a, "facturas"); dia.Peticiones != 0 {
		t.Fatalf("la API sí vuelve a cero: %+v", dia)
	}
}

func TestPresupuestoAgotadoReinicioMasTardio(t *testing.T) {
	ahora := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	a := conPresupuesto(t, &ahora,
		Tope{"facturas", nucleo.Presupuesto{Diario: nucleo.Limites{Peticiones: 1}}},
		Tope{TodaLaInstancia, nucleo.Presupuesto{Mensual: nucleo.Limites{Peticiones: 1}}})
	aceptar(t, a, enPunto("facturas"), "")
	_, _, err := a.Aceptar(ctx, enPunto("facturas"), "", 0)
	var agotado *nucleo.PresupuestoAgotado
	if !errors.As(err, &agotado) || !agotado.Reinicio.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("la API agotada hoy y la instancia este mes: Retry-After hasta el mes siguiente: %v", err)
	}
}
