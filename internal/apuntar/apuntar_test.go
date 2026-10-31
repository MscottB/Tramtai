package apuntar

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// formatoPrueba escribe {"tramtai":1}; antes se ejecuta dentro de Aplicar, para simular a otro que toca el archivo.
type formatoPrueba struct {
	antes       func()
	antesQuitar func()
	escrito     map[string]any // por defecto {"tramtai":"1"}
	previos     map[string]any
	err         error
}

func (f *formatoPrueba) Aplicar(actual []byte, v Valores) ([]byte, map[string]any, map[string]any, error) {
	if f.antes != nil {
		f.antes()
	}
	if f.err != nil {
		return nil, nil, nil, f.err
	}
	escrito := f.escrito
	if escrito == nil {
		escrito = map[string]any{"tramtai": "1"}
	}
	return []byte(`{"tramtai":1}`), escrito, f.previos, nil
}

func (f *formatoPrueba) Quitar(actual []byte, a Anotacion) ([]byte, bool, error) {
	if f.antesQuitar != nil {
		f.antesQuitar()
	}
	return []byte(`{}`), true, nil
}

func (f *formatoPrueba) Contiene(actual []byte, a Anotacion) bool { return true }

func preparar(t *testing.T, f *formatoPrueba) (datos, ruta string, h Herramienta) {
	t.Helper()
	datos, ruta = t.TempDir(), filepath.Join(t.TempDir(), "config", "settings.json")
	h = Herramienta{ID: "prueba", Nombre: "Prueba", Formato: f,
		Ruta: func(Alcance, string) (string, error) { return ruta, nil }}
	return datos, ruta, h
}

func leerTexto(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func comprobarPermisos(t *testing.T, ruta string) {
	t.Helper()
	info, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("%s: permisos %v, quería 0600", ruta, info.Mode().Perm())
	}
}

func copias(t *testing.T, datos string) []string {
	t.Helper()
	l, err := filepath.Glob(filepath.Join(datos, "herramientas", "copias", "*"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestAplicarHaceCopiaUnaVez(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(`{"original":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 { // la segunda, ya activa, no hace otra copia
		if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); err != nil {
			t.Fatal(err)
		}
	}
	l := copias(t, datos)
	if len(l) != 1 {
		t.Fatalf("copias = %v, quería exactamente una", l)
	}
	if got := leerTexto(t, l[0]); got != `{"original":true}` {
		t.Errorf("la copia tiene %q, quería el original", got)
	}
	comprobarPermisos(t, l[0])
	comprobarPermisos(t, ruta)
	comprobarPermisos(t, filepath.Join(datos, "herramientas", "prueba.json"))
	if got := leerTexto(t, ruta); got != `{"tramtai":1}` {
		t.Errorf("archivo = %q", got)
	}
}

func TestAplicarCreaSiNoExiste(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	if !a.NoExistia {
		t.Error("NoExistia debería ser true")
	}
	comprobarPermisos(t, ruta)
	if len(copias(t, datos)) != 0 {
		t.Error("sin archivo previo no hay nada que copiar")
	}
}

func TestQuitarBorraSiQuedaVacio(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ruta); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("el archivo debería haberse borrado: %v", err)
	}
	if _, hay, _ := Leer(datos, h.ID); hay {
		t.Error("la anotación debería haberse olvidado")
	}
}

func TestQuitarConservaElArchivoQueYaExistia(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	os.MkdirAll(filepath.Dir(ruta), 0o755)
	os.WriteFile(ruta, []byte(`{"original":true}`), 0o644)
	a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if got := leerTexto(t, ruta); got != `{}` {
		t.Errorf("archivo = %q, quería lo que devuelve el formato", got)
	}
}

func TestAnotacionIdaYVuelta(t *testing.T) {
	datos, _, h := preparar(t, &formatoPrueba{previos: map[string]any{"model": "gpt-5"}})
	v := Valores{Modelo: "claude/sonnet", Puerto: 17373, Orden: []string{`C:\Tramtai\tramtai.exe`, "clave"}}
	esperada, err := Aplicar(datos, h, AlcanceGlobal, "", v)
	if err != nil {
		t.Fatal(err)
	}
	if esperada.Orden != `C:\Tramtai\tramtai.exe` || esperada.Modelo != "claude/sonnet" || esperada.Puerto != 17373 {
		t.Errorf("anotación = %+v", esperada)
	}
	leida, hay, err := Leer(datos, h.ID)
	if err != nil || !hay {
		t.Fatalf("Leer: %v %v", hay, err)
	}
	if !reflect.DeepEqual(leida, esperada) {
		t.Errorf("Leer = %+v\nAplicar = %+v", leida, esperada)
	}
	if _, hay, err := Leer(datos, "otra"); hay || err != nil {
		t.Errorf("una herramienta sin activar: %v %v", hay, err)
	}
	if err := Borrar(datos, h.ID); err != nil {
		t.Fatal(err)
	}
	if err := Borrar(datos, h.ID); err != nil {
		t.Errorf("borrar lo que no está no es un error: %v", err)
	}
}

func TestAplicarOtraVezConservaLosPrevios(t *testing.T) {
	// "extra" no existía antes de Tramtai: no tiene previo, y en la segunda vez ya vale lo de Tramtai
	f := &formatoPrueba{escrito: map[string]any{"model": "x", "extra": "y"}, previos: map[string]any{"model": "gpt-5"}}
	datos, _, h := preparar(t, f)
	primera, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	f.previos = map[string]any{"model": "x", "extra": "y"} // lo que ve la segunda vez: ya es de Tramtai
	segunda, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{Modelo: "otro"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(segunda.Previos, map[string]any{"model": "gpt-5"}) || !segunda.Fecha.Equal(primera.Fecha) || segunda.Modelo != "otro" {
		t.Errorf("segunda = %+v", segunda)
	}
	leida, _, _ := Leer(datos, h.ID)
	if !reflect.DeepEqual(leida.Previos, map[string]any{"model": "gpt-5"}) {
		t.Errorf("anotación guardada = %+v", leida.Previos)
	}
}

func TestAplicarEnOtraRutaEsError(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	primera, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	otra := filepath.Join(t.TempDir(), "proyecto", "settings.json")
	h.Ruta = func(Alcance, string) (string, error) { return otra, nil }
	if _, err := Aplicar(datos, h, AlcanceProyecto, "p", Valores{}); !errors.Is(err, ErrOtraActivacion) {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(otra); !errors.Is(err, os.ErrNotExist) {
		t.Error("no debe haber escrito en la otra ruta")
	}
	if leida, _, _ := Leer(datos, h.ID); !reflect.DeepEqual(leida, primera) || leida.Ruta != ruta {
		t.Errorf("la anotación de la primera activación debe seguir intacta: %+v", leida)
	}
}

func TestLeerConservaLosEnteros(t *testing.T) {
	datos, _, h := preparar(t, &formatoPrueba{escrito: map[string]any{"puerto": 9007199254740993}})
	if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); err != nil {
		t.Fatal(err)
	}
	leida, _, err := Leer(datos, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := leida.Escrito["puerto"].(json.Number); !ok || n.String() != "9007199254740993" {
		t.Errorf("puerto = %#v", leida.Escrito["puerto"])
	}
}

func TestCopiasSeLimitanATres(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	os.MkdirAll(filepath.Dir(ruta), 0o755)
	os.WriteFile(ruta, []byte(`{"original":true}`), 0o644)
	carpeta := filepath.Join(datos, "herramientas", "copias")
	os.MkdirAll(carpeta, 0o700)
	for _, n := range []string{"prueba-20200101-000001.json", "prueba-20200101-000002.json", "prueba-20200101-000003.json",
		"prueba-20200101-000004.json", "prueba-2-20200101-000001.json", "otra-20200101-000001.json"} {
		os.WriteFile(filepath.Join(carpeta, n), []byte("x"), 0o600)
	}
	if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); err != nil {
		t.Fatal(err)
	}
	var quedan []string
	for _, c := range copias(t, datos) {
		quedan = append(quedan, filepath.Base(c))
	}
	// las 2 más recientes de las anteriores y la nueva; las de otras herramientas, aunque su id empiece igual, no se tocan
	if len(quedan) != 5 || quedan[0] != "otra-20200101-000001.json" || quedan[1] != "prueba-2-20200101-000001.json" ||
		quedan[2] != "prueba-20200101-000003.json" || quedan[3] != "prueba-20200101-000004.json" {
		t.Errorf("copias = %v", quedan)
	}
	if err := Borrar(datos, h.ID); err != nil || len(copias(t, datos)) != 5 {
		t.Errorf("Borrar no toca las copias: %v", err)
	}
}

func TestQuitarCambioMientrasEscribe(t *testing.T) {
	f := &formatoPrueba{}
	datos, ruta, h := preparar(t, f)
	a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	llamadas := 0
	f.antesQuitar = func() {
		llamadas++
		os.WriteFile(ruta, []byte(`{"otro":`+string(rune('0'+llamadas))+`}`), 0o644)
	}
	if err := Quitar(datos, h, a); !errors.Is(err, ErrCambioMientras) {
		t.Fatalf("err = %v", err)
	}
	if got := leerTexto(t, ruta); got != `{"otro":2}` {
		t.Errorf("no se debe tocar el archivo: %q", got)
	}
	if _, hay, _ := Leer(datos, h.ID); !hay {
		t.Error("la anotación debe seguir: no se quitó nada")
	}
}

func TestAplicarArchivoCreadoOBorradoEntreLecturas(t *testing.T) {
	t.Run("lo crean mientras tanto", func(t *testing.T) {
		f := &formatoPrueba{}
		datos, ruta, h := preparar(t, f)
		llamadas := 0
		f.antes = func() {
			if llamadas++; llamadas == 1 {
				os.MkdirAll(filepath.Dir(ruta), 0o755)
				os.WriteFile(ruta, []byte(`{"nuevo":1}`), 0o644)
			}
		}
		a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
		if err != nil {
			t.Fatal(err)
		}
		l := copias(t, datos)
		if a.NoExistia || len(l) != 1 || leerTexto(t, l[0]) != `{"nuevo":1}` {
			t.Errorf("NoExistia = %v, copias = %v", a.NoExistia, l)
		}
	})
	t.Run("lo borran mientras tanto", func(t *testing.T) {
		f := &formatoPrueba{}
		datos, ruta, h := preparar(t, f)
		os.MkdirAll(filepath.Dir(ruta), 0o755)
		os.WriteFile(ruta, []byte(`{"a":1}`), 0o644)
		llamadas := 0
		f.antes = func() {
			if llamadas++; llamadas == 1 {
				os.Remove(ruta)
			}
		}
		a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
		if err != nil {
			t.Fatal(err)
		}
		if !a.NoExistia || len(copias(t, datos)) != 0 || leerTexto(t, ruta) != `{"tramtai":1}` {
			t.Errorf("NoExistia = %v, copias = %v", a.NoExistia, copias(t, datos))
		}
	})
}

func TestQuitarConElArchivoAusente(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(ruta)
	if err := Quitar(datos, h, a); err != nil {
		t.Fatal(err)
	}
	if _, hay, _ := Leer(datos, h.ID); hay {
		t.Error("la anotación debería haberse olvidado")
	}
}

// La comprobación final, justo antes de pisar, deja el archivo como lo dejó el otro y no deja copia ni anotación nuevas.
func TestCambioDespuesDeLaCopiaDeshaceTodo(t *testing.T) {
	t.Cleanup(func() { antesDeEscribir = func() {} })
	t.Run("primera activación", func(t *testing.T) {
		datos, ruta, h := preparar(t, &formatoPrueba{})
		os.MkdirAll(filepath.Dir(ruta), 0o755)
		os.WriteFile(ruta, []byte(`{"a":1}`), 0o644)
		antesDeEscribir = func() { os.WriteFile(ruta, []byte(`{"a":2}`), 0o644) }
		if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); !errors.Is(err, ErrCambioMientras) {
			t.Fatalf("err = %v", err)
		}
		if got := leerTexto(t, ruta); got != `{"a":2}` || len(copias(t, datos)) != 0 {
			t.Errorf("archivo = %q, copias = %v", got, copias(t, datos))
		}
		if _, hay, _ := Leer(datos, h.ID); hay {
			t.Error("no debe quedar anotación")
		}
	})
	t.Run("la activación en curso recupera su anotación", func(t *testing.T) {
		datos, ruta, h := preparar(t, &formatoPrueba{})
		antesDeEscribir = func() {}
		primera, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{Modelo: "uno"})
		if err != nil {
			t.Fatal(err)
		}
		antesDeEscribir = func() { os.WriteFile(ruta, []byte(`{"otro":1}`), 0o644) }
		if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{Modelo: "dos"}); !errors.Is(err, ErrCambioMientras) {
			t.Fatalf("err = %v", err)
		}
		if leida, _, _ := Leer(datos, h.ID); leida.Modelo != "uno" || !leida.Fecha.Equal(primera.Fecha) {
			t.Errorf("la anotación debe ser la anterior: %+v", leida)
		}
	})
	t.Run("el archivo no se puede escribir", func(t *testing.T) {
		datos, ruta, h := preparar(t, &formatoPrueba{})
		os.MkdirAll(filepath.Dir(ruta), 0o755)
		os.WriteFile(ruta, []byte(`{"a":1}`), 0o644)
		antesDeEscribir = func() { // una carpeta donde estaba el archivo: ni se lee ni se escribe
			os.Remove(ruta)
			os.Mkdir(ruta, 0o755)
		}
		if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); err == nil {
			t.Fatal("debería fallar")
		}
		if len(copias(t, datos)) != 0 {
			t.Error("la copia debe borrarse si no se pudo escribir")
		}
		if _, hay, _ := Leer(datos, h.ID); hay {
			t.Error("no debe quedar anotación")
		}
	})
}

func TestOperacionesDeUnaHerramientaNoSeSolapan(t *testing.T) {
	var dentro, maximo atomic.Int32
	f := &formatoPrueba{}
	f.antes = func() {
		n := dentro.Add(1)
		for m := maximo.Load(); n > m && !maximo.CompareAndSwap(m, n); m = maximo.Load() {
		}
		time.Sleep(5 * time.Millisecond)
		dentro.Add(-1)
	}
	datos, _, h := preparar(t, f)
	var espera sync.WaitGroup
	for range 8 {
		espera.Add(1)
		go func() {
			defer espera.Done()
			if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); err != nil {
				t.Error(err)
			}
		}()
	}
	espera.Wait()
	if maximo.Load() != 1 {
		t.Errorf("hubo %d Aplicar a la vez", maximo.Load())
	}
}

func TestReintentar(t *testing.T) {
	llamadas := 0
	err := reintentar(func() error {
		if llamadas++; llamadas < 3 {
			return errors.New("bloqueado")
		}
		return nil
	})
	if runtime.GOOS == "windows" && (err != nil || llamadas != 3) {
		t.Errorf("en Windows reintenta: err = %v, llamadas = %d", err, llamadas)
	}
	if runtime.GOOS != "windows" && (err == nil || llamadas != 1) {
		t.Errorf("fuera de Windows no reintenta: err = %v, llamadas = %d", err, llamadas)
	}
}

func TestCambioMientrasEscribe(t *testing.T) {
	t.Run("un cambio se reintenta", func(t *testing.T) {
		f := &formatoPrueba{}
		datos, ruta, h := preparar(t, f)
		os.MkdirAll(filepath.Dir(ruta), 0o755)
		os.WriteFile(ruta, []byte(`{"a":1}`), 0o644)
		llamadas := 0
		f.antes = func() {
			if llamadas++; llamadas == 1 {
				os.WriteFile(ruta, []byte(`{"a":2}`), 0o644)
			}
		}
		if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); err != nil {
			t.Fatal(err)
		}
		if llamadas != 2 {
			t.Errorf("Formato.Aplicar se llamó %d veces, quería 2", llamadas)
		}
		l := copias(t, datos)
		if len(l) != 1 || leerTexto(t, l[0]) != `{"a":2}` {
			t.Errorf("la copia debe ser del archivo que se leyó en el reintento: %v", l)
		}
		if got := leerTexto(t, ruta); got != `{"tramtai":1}` {
			t.Errorf("archivo final = %q", got)
		}
	})
	t.Run("dos cambios son un error y no se toca", func(t *testing.T) {
		f := &formatoPrueba{}
		datos, ruta, h := preparar(t, f)
		os.MkdirAll(filepath.Dir(ruta), 0o755)
		os.WriteFile(ruta, []byte(`{"a":1}`), 0o644)
		llamadas := 0
		f.antes = func() {
			llamadas++
			os.WriteFile(ruta, []byte(`{"a":`+string(rune('1'+llamadas))+`}`), 0o644)
		}
		_, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
		if !errors.Is(err, ErrCambioMientras) {
			t.Fatalf("err = %v", err)
		}
		if got := leerTexto(t, ruta); got != `{"a":3}` {
			t.Errorf("el archivo debería ser lo último que escribió el otro, no lo de Tramtai: %q", got)
		}
		if len(copias(t, datos)) != 0 {
			t.Error("no debe haber copia")
		}
		if _, hay, _ := Leer(datos, h.ID); hay {
			t.Error("no debe haber anotación")
		}
	})
}

func TestArchivoIlegible(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{err: ErrArchivoIlegible})
	os.MkdirAll(filepath.Dir(ruta), 0o755)
	os.WriteFile(ruta, []byte(`{rota`), 0o644)
	if _, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{}); !errors.Is(err, ErrArchivoIlegible) {
		t.Fatalf("err = %v", err)
	}
	if got := leerTexto(t, ruta); got != `{rota` {
		t.Errorf("el archivo cambió: %q", got)
	}
	if len(copias(t, datos)) != 0 {
		t.Error("no debe haber copia")
	}
	if _, hay, _ := Leer(datos, h.ID); hay {
		t.Error("no debe haber anotación")
	}
}

func TestRestaurar(t *testing.T) {
	datos, ruta, h := preparar(t, &formatoPrueba{})
	os.MkdirAll(filepath.Dir(ruta), 0o755)
	os.WriteFile(ruta, []byte(`{"original":true}`), 0o644)
	a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	copia, err := Restaurar(datos, a)
	if err != nil {
		t.Fatal(err)
	}
	if got := leerTexto(t, ruta); got != `{"original":true}` || copia == "" {
		t.Errorf("archivo = %q, copia = %q", got, copia)
	}
	if _, hay, _ := Leer(datos, h.ID); hay {
		t.Error("restaurar olvida la anotación")
	}
}

func TestRestaurarSinCopia(t *testing.T) {
	datos, _, h := preparar(t, &formatoPrueba{})
	a, err := Aplicar(datos, h, AlcanceGlobal, "", Valores{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Restaurar(datos, a); !errors.Is(err, ErrSinCopia) {
		t.Errorf("err = %v", err)
	}
}

func TestDetectada(t *testing.T) {
	carpeta := t.TempDir()
	h := Herramienta{Ruta: func(Alcance, string) (string, error) { return filepath.Join(carpeta, "config.toml"), nil }}
	if !Detectada(h) {
		t.Error("la carpeta de configuración existe")
	}
	h.Ruta = func(Alcance, string) (string, error) { return filepath.Join(carpeta, "no", "config.toml"), nil }
	if Detectada(h) {
		t.Error("sin programa ni carpeta no está instalada")
	}
}
