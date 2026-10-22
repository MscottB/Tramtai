package servicio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
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
	if LeerMarca(proyecto) != antes.ID {
		t.Fatalf("marca %q", LeerMarca(proyecto))
	}
}

// El selector de espacios (la sesión) y la página Espacios dicen el mismo nombre: el de su configuración.
func TestMismoNombreEnLaSesionYEnEspacios(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	os.WriteFile(filepath.Join(proyecto, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Otro Nombre\n"), 0o600)
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	e, _ := s.registro().buscar(inst.ID)
	if got := resumir(inst).Nombre; got != "Otro Nombre" || e.Nombre != got {
		t.Fatalf("sesión %q, Espacios %q", got, e.Nombre)
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
	if e, _ := s.registro().buscar(antes.ID); !carpetas.MismaRuta(e.Ruta, nueva) {
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
	if LeerMarca(copia) != otra.ID || LeerMarca(proyecto) != original.ID {
		t.Fatalf("marcas: copia %q original %q", LeerMarca(copia), LeerMarca(proyecto))
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
		c, _ := configuracion.CargarConfiguracion(dir, agentes.Internos())
		if inst.ID == marca || inst.ID != IDProyecto(dir, c) || LeerMarca(dir) != inst.ID {
			t.Fatalf("marca %q adoptada: id %s, marca ahora %q", marca, inst.ID, LeerMarca(dir))
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
	if apartados, _ := filepath.Glob(ruta + ".dañado-*"); len(apartados) != 1 {
		t.Fatalf("no se apartó el registro dañado con su fecha: %v", apartados)
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

// espacioDe busca un espacio en la lista que verá la Web UI.
func espacioDe(t *testing.T, s *Servicio, id string) EspacioUI {
	t.Helper()
	for _, e := range s.Espacios(context.Background()) {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no está el espacio %s", id)
	return EspacioUI{}
}

// proyectoConDatos abre un proyecto confiado con una petición en su historial y una API con su clave.
func proyectoConDatos(t *testing.T, s *Servicio, base string) *Instancia {
	t.Helper()
	inst, err := s.AbrirProyecto(t.TempDir(), AbrirOpciones{})
	if err != nil {
		t.Fatal(err)
	}
	if r, cuerpo := pedir(t, base, "POST", "/api/v1/ejecutar", inst.token, hola); r.StatusCode != 200 {
		t.Fatalf("petición: %d %s", r.StatusCode, cuerpo)
	}
	err = configuracion.EditarPuntos(filepath.Join(inst.Raiz, ".tramtai", "local.yaml"), configuracion.OrigenLocal, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		return append(lista, ConfigPunto{Nombre: "asistente"}), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LeerOCrearToken(RutaTokenPunto(s.opc.Datos, "asistente")); err != nil {
		t.Fatal(err)
	}
	return inst
}

func TestEspaciosListaEstados(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	detenido := proyectoConDatos(t, s, ts.URL)
	if err := s.DetenerEspacio(detenido.ID); err != nil {
		t.Fatal(err)
	}
	borrado, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	s.DetenerEspacio(borrado.ID)
	os.RemoveAll(proyecto)
	if e := espacioDe(t, s, "global"); e.Estado != "activo" {
		t.Fatalf("global: %+v", e)
	}
	if e := espacioDe(t, s, detenido.ID); e.Estado != "detenido" || e.APIs != 1 || e.Peticiones != 1 || !e.Confiado {
		t.Fatalf("detenido: %+v", e)
	}
	if e := espacioDe(t, s, borrado.ID); e.Estado != "no_encontrado" {
		t.Fatalf("borrado: %+v", e)
	}
}

func TestDetenerYActivarPorId(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	token := inst.token
	if err := s.DetenerEspacio(inst.ID); err != nil {
		t.Fatal(err)
	}
	if e, _ := s.registro().buscar(inst.ID); e.Activo || s.instancia(inst.ID) != nil {
		t.Fatalf("debía quedar detenido: %+v", e)
	}
	otra, err := s.ActivarEspacio(inst.ID)
	if err != nil || otra.token != token {
		t.Fatalf("activar por id: %v", err)
	}
}

func TestUbicarReasocia(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	token := inst.token
	s.DetenerEspacio(inst.ID)
	nueva := filepath.Join(t.TempDir(), "Nueva")
	copiarCarpeta(t, proyecto, nueva)
	os.Remove(filepath.Join(nueva, ".tramtai", "espacio"))
	os.RemoveAll(proyecto)
	if _, err := s.Ubicar(inst.ID, t.TempDir()); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("una carpeta sin .tramtai no vale: %v", err)
	}
	otra, err := s.Ubicar(inst.ID, nueva)
	if err != nil {
		t.Fatal(err)
	}
	if otra.ID != inst.ID || otra.token != token || LeerMarca(nueva) != inst.ID {
		t.Fatalf("debía conservar id y token: %s", otra.ID)
	}
}

func TestOlvidarNoTocaLaCarpeta(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	inst := proyectoConDatos(t, s, ts.URL)
	antes := archivosDe(t, inst.Raiz)
	nombre := espacioDe(t, s, inst.ID).Nombre
	if err := s.Olvidar(inst.ID, " "+strings.ToUpper(nombre)+" "); err != nil {
		t.Fatal(err)
	}
	for _, ruta := range []string{filepath.Join(s.opc.Datos, "instancias", inst.ID+".db"), filepath.Join(s.opc.Datos, "cargas", inst.ID),
		RutaTokenInstancia(s.opc.Datos, inst.ID), RutaTokenPunto(s.opc.Datos, "asistente")} {
		if _, err := os.Stat(ruta); !os.IsNotExist(err) {
			t.Errorf("debía borrarse %s", ruta)
		}
	}
	if _, ok := s.registro().buscar(inst.ID); ok {
		t.Error("debía salir del registro")
	}
	if despues := archivosDe(t, inst.Raiz); despues != antes {
		t.Fatalf("la carpeta del proyecto cambió:\nantes %s\ndespués %s", antes, despues)
	}
}

func TestOlvidarPideNombre(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	inst := proyectoConDatos(t, s, ts.URL)
	if err := s.Olvidar(inst.ID, "otro"); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("nombre equivocado: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.opc.Datos, "instancias", inst.ID+".db")); err != nil {
		t.Fatal("con el nombre equivocado no se borra nada")
	}
	if err := s.Olvidar("global", "Global"); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("la global no se olvida: %v", err)
	}
}

// Un espacios.json que existe pero no se puede leer (bloqueado, o una carpeta) no se toma por «no hay registro»: no se
// migra instancias.json ni se reescribe el registro, y se avisa.
func TestRegistroIlegibleNoSeReescribe(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	os.MkdirAll(filepath.Join(s.opc.Datos, "espacios.json"), 0o700)
	viejo := filepath.Join(s.opc.Datos, "instancias.json")
	os.WriteFile(viejo, []byte(`{"global":true,"proyectos":[`+strconvQuote(proyecto)+`]}`), 0o600)
	if errores := s.Restaurar(); len(errores) == 0 {
		t.Fatal("debía avisar de que no pudo leer el registro")
	}
	if _, err := os.Stat(viejo); err != nil {
		t.Fatal("no debía migrar (ni borrar) instancias.json")
	}
	if info, err := os.Stat(filepath.Join(s.opc.Datos, "espacios.json")); err != nil || !info.IsDir() {
		t.Fatal("no debía reescribir el registro")
	}
}

func strconvQuote(s string) string { b, _ := json.Marshal(s); return string(b) }

// Confiar desde la página Espacios enseña antes las claves de seguridad que trae su local.yaml.
func TestEspaciosMuestraLasClavesDeUnoNoConfiado(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	no := false
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{Confiar: &no})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range s.Espacios(context.Background()) {
		if e.ID == inst.ID && strings.Join(e.Claves, ",") != "el programa de cada agente" {
			t.Fatalf("claves: %v", e.Claves)
		}
	}
}

// «Buscar» no asocia a un espacio una carpeta que ya es de otro.
func TestUbicarRechazaLaCarpetaDeOtroEspacio(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	os.MkdirAll(filepath.Join(s.opc.Datos, "instancias"), 0o700)
	os.WriteFile(filepath.Join(s.opc.Datos, "instancias", "viejo_1234abcd.db"), nil, 0o600)
	if _, err := s.ActivarProyecto(proyecto); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ubicar("viejo_1234abcd", proyecto); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("debía rechazar una carpeta que ya es de otro espacio: %v", err)
	}
}

// Un CSRF que no coincide tiene su propio código: la Web UI renueva la sesión solo por eso, no por un Origin ajeno.
func TestCSRFInvalidoTieneSuCodigo(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	n := abrirSesion(t, s, ts.URL)
	n.csrf = "malo"
	if r, cuerpo := n.pedir("POST", "/api/v1/instancia/heredar", `{"clave":"predeterminados.modelo"}`); r.StatusCode != 403 || !strings.Contains(cuerpo, "csrf_invalido") {
		t.Fatalf("%d %s", r.StatusCode, cuerpo)
	}
}

// Una API de un local.yaml con un nombre que no es válido (con barras) no lleva a Olvidar fuera de datos/tokens.
func TestOlvidarIgnoraNombresDeApiConBarras(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FijarConfianza(inst.Raiz, true); err != nil {
		t.Fatal(err)
	}
	otra := filepath.Join(s.opc.Datos, "instancias", "otro.db")
	os.WriteFile(otra, []byte("x"), 0o600)
	os.WriteFile(filepath.Join(proyecto, ".tramtai", "local.yaml"), []byte("puntos_acceso:\n  - nombre: x/../../instancias/otro.db\n"), 0o600)
	if err := s.Olvidar(inst.ID, espacioDe(t, s, inst.ID).Nombre); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(otra); err != nil {
		t.Fatalf("borró la base de otro espacio: %v", err)
	}
}

// La CLI resuelve el espacio de una carpeta con la misma regla que el servicio: una marca copiada en otra carpeta
// (una copia o un repositorio que la versiona) no es la de su proyecto mientras el original siga en su sitio.
func TestIDDeCarpetaNoAdoptaLaMarcaDeOtro(t *testing.T) {
	s, _, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.registro().guardar(); err != nil {
		t.Fatal(err)
	}
	copia := filepath.Join(t.TempDir(), "copia")
	os.MkdirAll(filepath.Join(copia, ".tramtai"), 0o700)
	os.WriteFile(filepath.Join(copia, ".tramtai", "config.yaml"), []byte("version: 1\nproyecto:\n  nombre: Demo App\n"), 0o600)
	os.WriteFile(filepath.Join(copia, ".tramtai", "espacio"), []byte(inst.ID), 0o600)
	c, _ := configuracion.CargarConfiguracion(copia, agentes.Internos())
	if id := IDDeCarpeta(s.opc.Datos, copia, c); id == inst.ID {
		t.Fatalf("la copia tomó el espacio del original %s", id)
	}
	if id := IDDeCarpeta(s.opc.Datos, inst.Raiz, inst.Config); id != inst.ID {
		t.Fatalf("el original: %s, era %s", id, inst.ID)
	}
}

func TestOlvidarSoloDentroDeDatos(t *testing.T) {
	s, _, _ := entorno(t, "", "", adaptadores.Simulado{})
	for _, id := range []string{"../x", `C:\x`, "/x", "", "x/../../y"} {
		if err := s.Olvidar(id, id); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
			t.Errorf("id %q: %v", id, err)
		}
	}
	if dentroDe(s.opc.Datos, filepath.Join(s.opc.Datos, "..", "fuera")) || !dentroDe(s.opc.Datos, filepath.Join(s.opc.Datos, "tokens", "x")) {
		t.Fatal("dentroDe")
	}
}

func TestRutasEspaciosUI(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	inst := proyectoConDatos(t, s, ts.URL)
	n := abrirSesion(t, s, ts.URL)
	if r, cuerpo := n.pedir("GET", "/ui/espacios", ""); r.StatusCode != 200 || !strings.Contains(cuerpo, inst.ID) {
		t.Fatalf("lista: %d %s", r.StatusCode, cuerpo)
	}
	cuerpo := `{"id":"` + inst.ID + `"}`
	if r, _ := n.pedir("POST", "/ui/espacios/detener", cuerpo); r.StatusCode != 401 {
		t.Fatalf("sin CSRF: %d", r.StatusCode)
	}
	if r, c := n.pedir("POST", "/ui/espacios/detener", cuerpo, "X-CSRF-Token", n.csrf); r.StatusCode != 200 || !strings.Contains(c, `"detenido"`) {
		t.Fatalf("detener: %d %s", r.StatusCode, c)
	}
	nombre := espacioDe(t, s, inst.ID).Nombre
	if r, c := n.pedir("POST", "/ui/espacios/olvidar", `{"id":"`+inst.ID+`","nombre":`+jsonTexto(nombre)+`}`, "X-CSRF-Token", n.csrf); r.StatusCode != 200 {
		t.Fatalf("olvidar: %d %s", r.StatusCode, c)
	}
}

func TestDescriptorTrasActivarDesdeUI(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	s.descriptor = true
	if _, err := s.ActivarGlobal(); err != nil {
		t.Fatal(err)
	}
	n := abrirSesion(t, s, ts.URL)
	s.Desactivar("global")
	if r, c := n.pedir("POST", "/ui/instancias", "", "X-CSRF-Token", n.csrf); r.StatusCode != 200 {
		t.Fatalf("activar: %d %s", r.StatusCode, c)
	}
	datos, err := os.ReadFile(filepath.Join(s.opc.Datos, "servicio.json"))
	if err != nil || !strings.Contains(string(datos), `"global"`) {
		t.Fatalf("descriptor: %v %s", err, datos)
	}
}

// archivosDe resume un árbol (rutas y tamaños) para ver si cambió.
func archivosDe(t *testing.T, raiz string) string {
	t.Helper()
	var b strings.Builder
	filepath.Walk(raiz, func(ruta string, info os.FileInfo, err error) error {
		if err == nil {
			rel, _ := filepath.Rel(raiz, ruta)
			fmt.Fprintf(&b, "%s:%d;", rel, info.Size())
		}
		return nil
	})
	return b.String()
}
