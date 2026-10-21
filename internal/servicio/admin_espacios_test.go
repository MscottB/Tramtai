package servicio

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/nucleo"
)

// conModo es un agente simulado que admite elegir entre suscripción y clave de API, como claude.
type conModo struct {
	adaptadores.Simulado
	modo *adaptadores.Modo
}

func (c conModo) ModoAutenticacion() *adaptadores.Modo { return c.modo }

// admin llama a la administración con el token de servicio.
func admin(t *testing.T, s *Servicio, base, metodo, ruta, cuerpo string) (*http.Response, string) {
	t.Helper()
	return pedir(t, base, metodo, ruta, s.tokenAdmin, cuerpo)
}

func espacioRespondido(t *testing.T, cuerpo string) contrato.Espacio {
	t.Helper()
	var e contrato.Espacio
	if err := json.Unmarshal([]byte(cuerpo), &e); err != nil {
		t.Fatalf("no es un espacio: %s", cuerpo)
	}
	return e
}

func TestAdminConfiarProyecto(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	no := false
	inst, err := s.AbrirProyecto(dir, AbrirOpciones{Confiar: &no})
	if err != nil {
		t.Fatal(err)
	}
	ruta := "/api/v1/servicio/espacios/" + inst.ID + "/confianza"
	if r, cuerpo := admin(t, s, ts.URL, "POST", ruta, `{"confiar":true}`); r.StatusCode != 204 || !confiable(t, inst.Raiz) {
		t.Fatalf("confiar: %d %s", r.StatusCode, cuerpo)
	}
	if r, cuerpo := admin(t, s, ts.URL, "POST", ruta, `{"confiar":false}`); r.StatusCode != 204 || confiable(t, inst.Raiz) {
		t.Fatalf("dejar de confiar: %d %s", r.StatusCode, cuerpo)
	}
	// Sin decir si se confía, o sobre la global o un espacio que no existe: es una solicitud no válida.
	for _, c := range []struct{ id, cuerpo string }{{inst.ID, `{}`}, {"global", `{"confiar":true}`}, {"no-existe", `{"confiar":true}`}} {
		r, cuerpo := admin(t, s, ts.URL, "POST", "/api/v1/servicio/espacios/"+c.id+"/confianza", c.cuerpo)
		if r.StatusCode != 400 || !strings.Contains(cuerpo, string(nucleo.CodigoSolicitudInvalida)) {
			t.Fatalf("%s %s: %d %s", c.id, c.cuerpo, r.StatusCode, cuerpo)
		}
	}
}

func TestAdminUbicar(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	s.DetenerEspacio(inst.ID)
	nueva := filepath.Join(t.TempDir(), "Nueva")
	copiarCarpeta(t, proyecto, nueva)
	os.Remove(filepath.Join(nueva, ".tramtai", "espacio"))
	os.RemoveAll(proyecto)
	ruta := "/api/v1/servicio/espacios/" + inst.ID + "/ubicacion"
	if r, _ := admin(t, s, ts.URL, "POST", ruta, `{"ruta":`+strconv.Quote(t.TempDir())+`}`); r.StatusCode != 400 {
		t.Fatalf("una carpeta sin .tramtai no vale: %d", r.StatusCode)
	}
	r, cuerpo := admin(t, s, ts.URL, "POST", ruta, `{"ruta":`+strconv.Quote(nueva)+`}`)
	real, _ := filepath.EvalSymlinks(nueva) // el servicio guarda la ruta real
	e := espacioRespondido(t, cuerpo)
	if r.StatusCode != 200 || e.ID != inst.ID || e.Estado != "activo" || !nucleo.MismaRuta(e.Ruta, real) {
		t.Fatalf("ubicar: %d %s", r.StatusCode, cuerpo)
	}
	if LeerMarca(nueva) != inst.ID || !registroEnDisco(t, s)[inst.ID].Activo {
		t.Fatal("ubicar guarda la marca y el registro")
	}
}

func TestAdminAbrirProyecto409Confianza(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	dir := t.TempDir()
	conTramtai(t, dir, "comandos_agentes:\n  claude: otro.exe\n")
	cuerpo := `{"raiz":` + strconv.Quote(dir) + `}`
	r, texto := admin(t, s, ts.URL, "POST", "/api/v1/servicio/proyectos", cuerpo)
	var conflicto struct {
		Error  struct{ Codigo, Mensaje string }
		Claves []string
		Movido string
	}
	if err := json.Unmarshal([]byte(texto), &conflicto); err != nil || r.StatusCode != 409 || conflicto.Error.Codigo != string(nucleo.CodigoConfianzaRequerida) ||
		conflicto.Error.Mensaje == "" || !slices.Equal(conflicto.Claves, []string{"comandos_agentes"}) {
		t.Fatalf("debía pedir confianza: %d %s", r.StatusCode, texto)
	}
	if confiable(t, dir) {
		t.Fatal("preguntar no debe confiar")
	}
	r, texto = admin(t, s, ts.URL, "POST", "/api/v1/servicio/proyectos", `{"raiz":`+strconv.Quote(dir)+`,"confiar":true}`)
	if e := espacioRespondido(t, texto); r.StatusCode != 200 || e.Estado != "activo" || !e.Confiado || !registroEnDisco(t, s)[e.ID].Activo {
		t.Fatalf("abrir confiando: %d %s", r.StatusCode, texto)
	}

	// Dentro de otro proyecto: pregunta cuál; con crear_aqui lo crea en la subcarpeta.
	sub := filepath.Join(proyecto, "sub")
	os.MkdirAll(sub, 0o700)
	r, texto = admin(t, s, ts.URL, "POST", "/api/v1/servicio/proyectos", `{"raiz":`+strconv.Quote(sub)+`}`)
	var dentro struct {
		Error    struct{ Codigo string }
		Proyecto string
	}
	if err := json.Unmarshal([]byte(texto), &dentro); err != nil || r.StatusCode != 409 || dentro.Error.Codigo != string(nucleo.CodigoDentroDeProyecto) || dentro.Proyecto == "" {
		t.Fatalf("debía avisar de que está dentro de otro proyecto: %d %s", r.StatusCode, texto)
	}
	if r, texto = admin(t, s, ts.URL, "POST", "/api/v1/servicio/proyectos", `{"raiz":`+strconv.Quote(sub)+`,"crear_aqui":true}`); r.StatusCode != 200 {
		t.Fatalf("crear aquí: %d %s", r.StatusCode, texto)
	}
	if r, _ := admin(t, s, ts.URL, "POST", "/api/v1/servicio/proyectos", `{"raiz":"relativa"}`); r.StatusCode != 400 {
		t.Fatalf("una ruta que no es completa es una solicitud no válida: %d", r.StatusCode)
	}
}

func TestAdminAutenticacion(t *testing.T) {
	s, ts, _ := entorno(t, "", "", adaptadores.Simulado{})
	modo := adaptadores.NuevoModo(adaptadores.AutenticacionClaveAPI)
	s.opc.Registro["claude"] = conModo{modo: modo}
	ruta := "/api/v1/servicio/agentes/claude/autenticacion"
	if r, cuerpo := admin(t, s, ts.URL, "PUT", ruta, `{"modo":"suscripcion"}`); r.StatusCode != 204 || modo.Valor() != adaptadores.AutenticacionSuscripcion {
		t.Fatalf("suscripción: %d %s", r.StatusCode, cuerpo)
	}
	g, _ := nucleo.CargarConfiguracion("")
	if g.Texto("adaptadores.claude.autenticacion") != adaptadores.AutenticacionSuscripcion {
		t.Fatal("el modo se guarda en la configuración global")
	}
	if r, _ := admin(t, s, ts.URL, "PUT", ruta, `{"modo":"clave_api"}`); r.StatusCode != 204 || modo.Valor() != adaptadores.AutenticacionClaveAPI {
		t.Fatalf("clave de API: %d", r.StatusCode)
	}
	if r, _ := admin(t, s, ts.URL, "PUT", ruta, `{"modo":"otro"}`); r.StatusCode != 400 {
		t.Fatalf("un modo desconocido es una solicitud no válida: %d", r.StatusCode)
	}
	if r, _ := admin(t, s, ts.URL, "PUT", "/api/v1/servicio/agentes/gemini/autenticacion", `{"modo":"suscripcion"}`); r.StatusCode != 400 {
		t.Fatalf("gemini no admite la suscripción: %d", r.StatusCode)
	}
}

func TestAdminActivarDetener(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/v1/servicio/espacios/" + inst.ID
	if r, cuerpo := admin(t, s, ts.URL, "POST", base+"/detener", ""); r.StatusCode != 204 || s.instancia(inst.ID) != nil || registroEnDisco(t, s)[inst.ID].Activo {
		t.Fatalf("detener: %d %s", r.StatusCode, cuerpo)
	}
	r, cuerpo := admin(t, s, ts.URL, "POST", base+"/activar", "")
	if e := espacioRespondido(t, cuerpo); r.StatusCode != 200 || e.ID != inst.ID || e.Estado != "activo" || !e.Activo || s.instancia(inst.ID) == nil || !registroEnDisco(t, s)[inst.ID].Activo {
		t.Fatalf("activar: %d %s", r.StatusCode, cuerpo)
	}
	for _, accion := range []string{"activar", "detener", "ubicacion"} {
		r, cuerpo := admin(t, s, ts.URL, "POST", "/api/v1/servicio/espacios/no-existe/"+accion, `{"ruta":"x"}`)
		if r.StatusCode != 404 {
			t.Fatalf("%s de un espacio que no existe: %d %s", accion, r.StatusCode, cuerpo)
		}
	}
}

func TestRutasAdminRechazanClaveDeApp(t *testing.T) {
	s, ts, proyecto := entorno(t, "", "", adaptadores.Simulado{})
	inst, err := s.ActivarProyecto(proyecto)
	if err != nil {
		t.Fatal(err)
	}
	e := "/api/v1/servicio/espacios/" + inst.ID
	rutas := []struct{ metodo, ruta string }{
		{"POST", e + "/activar"}, {"POST", e + "/detener"}, {"POST", e + "/confianza"}, {"POST", e + "/ubicacion"},
		{"POST", "/api/v1/servicio/proyectos"}, {"PUT", "/api/v1/servicio/agentes/claude/autenticacion"},
	}
	for _, c := range rutas {
		for _, token := range []string{inst.token, ""} {
			if r, _ := pedir(t, ts.URL, c.metodo, c.ruta, token, `{"modo":"suscripcion","confiar":true,"ruta":"x","raiz":"x"}`); r.StatusCode != 401 {
				t.Fatalf("%s %s con la clave de la app (%t): %d", c.metodo, c.ruta, token != "", r.StatusCode)
			}
		}
	}
	if s.instancia(inst.ID) == nil {
		t.Fatal("una petición rechazada no cambia nada")
	}
}
