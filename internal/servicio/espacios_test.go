package servicio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
)

// registroEnDisco lee datos/espacios.json tal como quedó guardado.
func registroEnDisco(t *testing.T, s *Servicio) map[string]Espacio {
	t.Helper()
	datos, err := os.ReadFile(filepath.Join(s.opc.Datos, "espacios.json"))
	if err != nil {
		t.Fatal(err)
	}
	var archivo struct{ Espacios []Espacio }
	if err := json.Unmarshal(datos, &archivo); err != nil {
		t.Fatal(err)
	}
	m := map[string]Espacio{}
	for _, e := range archivo.Espacios {
		m[e.ID] = e
	}
	return m
}

func TestMigrarInstanciasJSON(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	os.MkdirAll(s.opc.Datos, 0o700)
	viejo, _ := json.Marshal(map[string]any{"global": true, "proyectos": []string{proyecto}})
	os.WriteFile(filepath.Join(s.opc.Datos, "instancias.json"), viejo, 0o600)
	if errores := s.Restaurar(); len(errores) != 0 {
		t.Fatal(errores)
	}
	reg := registroEnDisco(t, s)
	if !reg["global"].Activo {
		t.Fatalf("la global debía seguir activa: %+v", reg)
	}
	var pry Espacio
	for _, e := range reg {
		if e.Ambito == "proyecto" {
			pry = e
		}
	}
	if !pry.Activo || pry.Nombre != "Demo App" || pry.Ruta == "" {
		t.Fatalf("proyecto migrado: %+v", pry)
	}
	if _, err := os.Stat(filepath.Join(s.opc.Datos, "instancias.json")); !os.IsNotExist(err) {
		t.Fatal("instancias.json debía desaparecer tras migrar")
	}
}

func TestIdFijoAlRenombrar(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	antes, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	s.Desactivar(antes.ID)
	os.WriteFile(filepath.Join(proyecto, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Otro Nombre\n"), 0o600)
	despues, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	if despues.ID != antes.ID {
		t.Fatalf("renombrar cambió el id: %s → %s", antes.ID, despues.ID)
	}
	if leerMarca(proyecto) != antes.ID {
		t.Fatalf("marca %q", leerMarca(proyecto))
	}
}

func TestMoverConMarca(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	antes, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	token := antes.token
	s.Desactivar(antes.ID)
	nueva := filepath.Join(filepath.Dir(proyecto), "Movido")
	if err := os.Rename(proyecto, nueva); err != nil {
		t.Fatal(err)
	}
	despues, err := s.ActivarProyecto(nueva)
	if err != nil {
		t.Fatal(err)
	}
	if despues.ID != antes.ID || despues.token != token {
		t.Fatalf("al mover debía conservar id y token: %s %s", antes.ID, despues.ID)
	}
	if e, _ := s.registro().buscar(antes.ID); !mismaRuta(e.Ruta, nueva) {
		t.Fatalf("ruta del registro: %q", e.Ruta)
	}
}

func TestCopiaEsNueva(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	original, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	copia := filepath.Join(filepath.Dir(proyecto), "Copia")
	copiarCarpeta(t, proyecto, copia)
	otra, err := s.ActivarProyecto(copia)
	if err != nil {
		t.Fatal(err)
	}
	if otra.ID == original.ID {
		t.Fatal("una copia debía ser un espacio nuevo")
	}
	if leerMarca(copia) != otra.ID || leerMarca(proyecto) != original.ID {
		t.Fatalf("marcas: copia %q original %q", leerMarca(copia), leerMarca(proyecto))
	}
}

func TestMarcaAjenaNoAdopta(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	for _, marca := range []string{"global", "desconocido_12345678"} {
		dir := filepath.Join(filepath.Dir(proyecto), "Clonado "+marca)
		copiarCarpeta(t, proyecto, dir)
		os.WriteFile(filepath.Join(dir, ".tramtai", "espacio"), []byte(marca+"\n"), 0o600)
		inst, err := s.ActivarProyecto(dir)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := nucleo.CargarConfiguracion(dir)
		if inst.ID == marca || inst.ID != IDProyecto(dir, c) || leerMarca(dir) != inst.ID {
			t.Fatalf("marca %q adoptada: id %s, marca ahora %q", marca, inst.ID, leerMarca(dir))
		}
	}
}

func TestDatosSueltosNoEncontrado(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	os.MkdirAll(filepath.Join(s.opc.Datos, "instancias"), 0o700)
	os.WriteFile(filepath.Join(s.opc.Datos, "instancias", "viejo_1234abcd.db"), nil, 0o600)
	e, ok := s.registro().buscar("viejo_1234abcd")
	if !ok || e.Ambito != "proyecto" || e.Ruta != "" || e.Nombre != "viejo_1234abcd" || e.Activo {
		t.Fatalf("datos sueltos: %+v %v", e, ok)
	}
}

func TestRegistroDanado(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	os.MkdirAll(s.opc.Datos, 0o700)
	ruta := filepath.Join(s.opc.Datos, "espacios.json")
	os.WriteFile(ruta, []byte("{"), 0o600)
	errores := s.Restaurar()
	if len(errores) != 1 || !strings.Contains(errores[0].Error(), "dañado") {
		t.Fatalf("errores: %v", errores)
	}
	if _, err := os.Stat(ruta + ".dañado"); err != nil {
		t.Fatal("no se apartó el registro dañado")
	}
	if _, activa := s.instancias["global"]; !activa {
		t.Fatal("la global debía abrirse")
	}
}

func TestRestaurarCarpetaBorradaDejaError(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	s.guardarActivas()
	s.Cerrar()
	os.RemoveAll(proyecto)
	s2, err := Nuevo(s.opc)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Cerrar()
	if errores := s2.Restaurar(); len(errores) == 0 {
		t.Fatal("debía avisar del proyecto que no pudo abrir")
	}
	e, _ := s2.registro().buscar(inst.ID)
	if e.Activo || e.UltimoError == "" {
		t.Fatalf("espacio tras restaurar: %+v", e)
	}
	if reg := registroEnDisco(t, s2); reg[inst.ID].UltimoError == "" {
		t.Fatal("el error debía quedar guardado")
	}
}

func TestGitignoreMarca(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	os.WriteFile(filepath.Join(proyecto, ".tramtai", ".gitignore"), []byte("# configuración propia de cada máquina\nlocal.yaml\n"), 0o600)
	for range 2 {
		inst, err := s.ActivarProyecto(proyecto)
		if err != nil {
			t.Fatal(err)
		}
		s.Desactivar(inst.ID)
	}
	datos, _ := os.ReadFile(filepath.Join(proyecto, ".tramtai", ".gitignore"))
	if strings.Count(string(datos), "\nespacio\n") != 1 || !strings.Contains(string(datos), "local.yaml") {
		t.Fatalf(".gitignore:\n%s", datos)
	}
}

// copiarCarpeta copia un árbol pequeño de archivos (los proyectos de las pruebas).
func copiarCarpeta(t *testing.T, origen, destino string) {
	t.Helper()
	err := filepath.Walk(origen, func(ruta string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(origen, ruta)
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(destino, rel), 0o700)
		}
		datos, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(destino, rel), datos, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}
