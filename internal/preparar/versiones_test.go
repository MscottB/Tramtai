package preparar

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
)

// fuentes es un servidor HTTPS que hace de npm, de GitHub, de las URL de los fabricantes y del release «compatibilidad».
type fuentes struct {
	srv      *httptest.Server
	pedidos  atomic.Int32
	mu       sync.Mutex
	rutas    map[string]string // ruta → cuerpo (200); "403" o "429" como cuerpo devuelven ese estado
	cabecera []http.Header
	privada  ed25519.PrivateKey
}

// nuevasFuentes arranca el servidor, apunta a él npm, GitHub y la lista firmada y hace que la firma se compruebe con una
// clave de la prueba.
func nuevasFuentes(t *testing.T) *fuentes {
	t.Helper()
	publica, privada, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fu := &fuentes{rutas: map[string]string{}, privada: privada}
	fu.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fu.pedidos.Add(1)
		fu.mu.Lock()
		cuerpo, ok := fu.rutas[r.URL.Path]
		fu.cabecera = append(fu.cabecera, r.Header.Clone())
		fu.mu.Unlock()
		switch {
		case !ok:
			http.NotFound(w, r)
		case strings.HasPrefix(cuerpo, "->"):
			http.Redirect(w, r, strings.TrimPrefix(cuerpo, "->"), http.StatusFound)
		case cuerpo == "403" || cuerpo == "429":
			w.WriteHeader(map[string]int{"403": http.StatusForbidden, "429": http.StatusTooManyRequests}[cuerpo])
		default:
			fmt.Fprint(w, cuerpo)
		}
	}))
	t.Cleanup(fu.srv.Close)
	antes := [3]string{baseNPM, baseGitHub, URLLista}
	baseNPM, baseGitHub, URLLista = fu.srv.URL+"/npm/", fu.srv.URL+"/gh/", fu.srv.URL+"/lista/compatibilidad.json"
	t.Cleanup(func() { baseNPM, baseGitHub, URLLista = antes[0], antes[1], antes[2] })
	actualizacion.ParaPruebas(t.Cleanup, fu.srv.URL+"/nada", fu.srv.URL+"/nada/", publica)
	return fu
}

func (fu *fuentes) poner(ruta, cuerpo string) {
	fu.mu.Lock()
	defer fu.mu.Unlock()
	fu.rutas[ruta] = cuerpo
}

// lista publica la lista firmada; con firmaMala, su firma no vale.
func (fu *fuentes) lista(t *testing.T, version int, agentes map[string]map[string][]string, firmaMala bool) {
	t.Helper()
	datos := listaJSON(t, version, agentes, "")
	firma := ed25519.Sign(fu.privada, datos)
	if firmaMala {
		firma[0] ^= 0xff
	}
	fu.publicar(datos, firma)
}

// publicar deja en el release una lista y una firma cualesquiera.
func (fu *fuentes) publicar(datos, firma []byte) {
	fu.poner("/lista/compatibilidad.json", string(datos))
	fu.poner("/lista/compatibilidad.json.sig", base64.StdEncoding.EncodeToString(firma))
}

func listaJSON(t *testing.T, version int, agentes map[string]map[string][]string, relleno string) []byte {
	t.Helper()
	datos, err := json.Marshal(map[string]any{"version": version, "agentes": agentes, "relleno": relleno})
	if err != nil {
		t.Fatal(err)
	}
	return datos
}

// conFuentes es una prueba cuyo preparador usa sus propios ganchos (Comprobada, UltimaVersion) y el servidor de fuentes.
func conFuentes(t *testing.T, fu *fuentes, f adaptadores.Ficha) *prueba {
	t.Helper()
	pr := nueva(t, f)
	pr.p.cliente.Transport = fu.srv.Client().Transport
	pr.p.o.Comprobada = pr.p.Comprobada
	pr.p.o.UltimaVersion = pr.p.ultimaPublicada
	pr.p.o.Agentes = []string{f.Nombre}
	return pr
}

func unaVersion(t *testing.T, p *Preparador, forzar bool) contrato.VersionesAgente {
	t.Helper()
	v := p.Versiones(context.Background(), forzar)
	if len(v) != 1 {
		t.Fatalf("Versiones: %+v", v)
	}
	return v[0]
}

func TestUltimaVersion(t *testing.T) {
	fu := nuevasFuentes(t)
	fu.poner("/npm/@falso/cli/latest", `{"name":"@falso/cli","version":"3.4.5"}`)
	fu.poner("/gh/falso/cli/releases/latest", `{"tag_name":"rust-v0.167.2","html_url":"x"}`)
	fu.poner("/texto", "2.1.323\n")
	fu.poner("/manifiesto.json", `{"version":"1.5.7","url":"https://x/y.zip","sha512":"ab"}`)
	fu.poner("/gh/limitado/cli/releases/latest", "403")
	fu.poner("/gh/frenado/cli/releases/latest", "429")
	fu.poner("/rara", "1.0;iex x")
	fu.poner("/a-http", "->"+strings.Replace(fu.srv.URL, "https", "http", 1)+"/texto")
	casos := []struct {
		nombre string
		fuente adaptadores.FuenteVersion
		ultima string
	}{
		{"npm", adaptadores.FuenteVersion{Tipo: "npm", Nombre: "@falso/cli"}, "3.4.5"},
		{"github", adaptadores.FuenteVersion{Tipo: "github", Nombre: "falso/cli", Prefijo: "rust-v"}, "0.167.2"},
		{"url de texto", adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/texto"}, "2.1.323"},
		{"url de JSON", adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/manifiesto.json"}, "1.5.7"},
		{"GitHub con el límite agotado", adaptadores.FuenteVersion{Tipo: "github", Nombre: "limitado/cli"}, ""},
		{"GitHub con demasiadas peticiones", adaptadores.FuenteVersion{Tipo: "github", Nombre: "frenado/cli"}, ""},
		{"no encontrada", adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/no-esta"}, ""},
		{"con otra forma", adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/rara"}, ""},
		{"por http", adaptadores.FuenteVersion{Tipo: "url", Nombre: strings.Replace(fu.srv.URL, "https", "http", 1) + "/texto"}, ""},
		{"redirigida a http", adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/a-http"}, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			f := fichaFalsa(t, "instalar")
			f.Receta.Ultima = c.fuente
			v := unaVersion(t, conFuentes(t, fu, f).p, true)
			if v.Agente != "falso" || v.Ultima != c.ultima {
				t.Fatalf("Ultima = %q, se esperaba %q: %+v", v.Ultima, c.ultima, v)
			}
			if sinRed := v.MotivoCodigo == CodigoSinRed; sinRed != (c.ultima == "") {
				t.Fatalf("MotivoCodigo = %q: %+v", v.MotivoCodigo, v)
			}
		})
	}

	// Sin red: un servidor cerrado.
	cerrado := httptest.NewTLSServer(http.NotFoundHandler())
	cerrado.Close()
	f := fichaFalsa(t, "instalar")
	f.Receta.Ultima = adaptadores.FuenteVersion{Tipo: "url", Nombre: cerrado.URL + "/texto"}
	if v := unaVersion(t, conFuentes(t, fu, f).p, true); v.Ultima != "" || v.MotivoCodigo != CodigoSinRed || v.Motivo == "" {
		t.Fatalf("sin red: %+v", v)
	}

	// Solo la cabecera User-Agent (más Accept-Encoding, que pone Go): nada que identifique al usuario.
	fu.mu.Lock()
	defer fu.mu.Unlock()
	for _, h := range fu.cabecera {
		if !strings.HasPrefix(h.Get("User-Agent"), "Tramtai/") {
			t.Errorf("User-Agent: %q", h.Get("User-Agent"))
		}
		for nombre := range h {
			if nombre != "User-Agent" && nombre != "Accept-Encoding" {
				t.Errorf("cabecera de más: %s: %q", nombre, h.Get(nombre))
			}
		}
	}
}

// Una consulta al día como mucho (también tras reiniciar el servicio), salvo con forzar.
func TestVersionesUnaVezAlDia(t *testing.T) {
	fu := nuevasFuentes(t)
	fu.poner("/texto", "2.1.323")
	f := fichaFalsa(t, "instalar")
	f.Receta.Ultima = adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/texto"}
	pr := conFuentes(t, fu, f)
	unaVersion(t, pr.p, false)
	tras := fu.pedidos.Load()
	if tras == 0 {
		t.Fatal("la primera vez se consulta")
	}
	if v := unaVersion(t, pr.p, false); v.Ultima != "2.1.323" || fu.pedidos.Load() != tras {
		t.Fatalf("la segunda vez del día no se consulta: %+v (%d pedidos)", v, fu.pedidos.Load()-tras)
	}
	otro := Nuevo(pr.p.o) // el servicio se reinició: lo leído sigue en compatibilidad.json
	otro.cliente.Transport = fu.srv.Client().Transport
	if v := unaVersion(t, otro, false); v.Ultima != "2.1.323" || fu.pedidos.Load() != tras {
		t.Fatalf("tras reiniciar: %+v (%d pedidos)", v, fu.pedidos.Load()-tras)
	}
	fu.poner("/texto", "2.1.330")
	if v := unaVersion(t, pr.p, true); v.Ultima != "2.1.330" || fu.pedidos.Load() == tras {
		t.Fatalf("con forzar se consulta: %+v", v)
	}
}

func TestComprobadaComodin(t *testing.T) {
	f := fichaFalsa(t, "instalar")
	f.Receta.Comprobadas = map[string][]string{"iniciar_sesion": {"2.1.*", "3.0.0"}}
	p := nueva(t, f).p
	for version, esperada := range map[string]bool{"2.1.300": true, "2.1.0": true, "3.0.0": true, "2.2.0": false,
		"2.10.1": false, "2.1": false, "": false, "3.0.1": false} {
		if p.Comprobada("falso", "iniciar_sesion", version) != esperada {
			t.Errorf("Comprobada(%q) != %v", version, esperada)
		}
	}
	if p.Comprobada("falso", "instalar", "2.1.300") || p.Comprobada("otro", "iniciar_sesion", "2.1.300") {
		t.Error("solo cuenta la acción y el agente de la lista")
	}
	// Sin los ganchos del servicio, el preparador usa los suyos.
	q := Nuevo(Opciones{Datos: t.TempDir(), Ficha: p.o.Ficha})
	if q.o.Comprobada == nil || q.o.UltimaVersion == nil || !q.o.Comprobada("falso", "iniciar_sesion", "2.1.300") || q.o.Comprobada("falso", "iniciar_sesion", "2.2.0") {
		t.Error("Nuevo sin Comprobada ni UltimaVersion usa las del preparador")
	}
}

func TestListaFirmada(t *testing.T) {
	fu := nuevasFuentes(t)
	pr := conFuentes(t, fu, fichaFalsa(t, "instalar"))
	p := pr.p
	ctx := context.Background()
	if p.Comprobada("falso", "instalar", "9.0.0") {
		t.Fatal("sin lista no está comprobada")
	}
	fu.lista(t, 2, map[string]map[string][]string{"falso": {"instalar": {"9.0.0"}}}, false)
	if err := p.LeerLista(ctx); err != nil || !p.Comprobada("falso", "instalar", "9.0.0") {
		t.Fatalf("con la firma buena se añade: %v", err)
	}
	fu.lista(t, 3, map[string]map[string][]string{"falso": {"instalar": {"9.1.0"}}}, true)
	if err := p.LeerLista(ctx); err == nil || p.Comprobada("falso", "instalar", "9.1.0") || !p.Comprobada("falso", "instalar", "9.0.0") {
		t.Fatalf("con la firma mala se ignora y se conserva la anterior: %v", err)
	}
	// Una lista vieja, aunque firmada, no sustituye a la guardada.
	fu.lista(t, 1, map[string]map[string][]string{"falso": {"instalar": {"9.2.0"}}}, false)
	if err := p.LeerLista(ctx); err != nil || p.Comprobada("falso", "instalar", "9.2.0") || !p.Comprobada("falso", "instalar", "9.0.0") {
		t.Fatalf("una lista con version menor no vale: %v", err)
	}
	// La misma version con otro contenido tampoco: no se puede cambiar una lista ya aceptada.
	fu.lista(t, 2, map[string]map[string][]string{"falso": {"instalar": {"9.2.1"}}}, false)
	if err := p.LeerLista(ctx); err != nil || p.Comprobada("falso", "instalar", "9.2.1") || !p.Comprobada("falso", "instalar", "9.0.0") {
		t.Fatalf("la misma version con otro contenido no vale: %v", err)
	}
	// La firma válida de otra lista no vale para esta.
	otra := listaJSON(t, 5, map[string]map[string][]string{"falso": {"instalar": {"9.4.0"}}}, "")
	fu.publicar(listaJSON(t, 5, map[string]map[string][]string{"falso": {"instalar": {"9.5.0"}}}, ""), ed25519.Sign(fu.privada, otra))
	if err := p.LeerLista(ctx); err == nil || !strings.Contains(err.Error(), "lista de versiones comprobadas") || p.Comprobada("falso", "instalar", "9.5.0") {
		t.Fatalf("una firma de otra lista no vale: %v", err)
	}
	// Una lista de más de 1 MiB no se acepta, aunque esté firmada.
	grande := listaJSON(t, 6, map[string]map[string][]string{"falso": {"instalar": {"9.6.0"}}}, strings.Repeat("x", 1<<20))
	fu.publicar(grande, ed25519.Sign(fu.privada, grande))
	if err := p.LeerLista(ctx); err == nil || p.Comprobada("falso", "instalar", "9.6.0") {
		t.Fatalf("una lista de más de 1 MiB no vale: %v", err)
	}
	datos, err := os.ReadFile(filepath.Join(pr.datos, "agentes", "compatibilidad.json"))
	if err != nil || !strings.Contains(string(datos), `"version": 2`) {
		t.Fatalf("compatibilidad.json: %s %v", datos, err)
	}
	// Sin firma publicada tampoco.
	fu.lista(t, 4, map[string]map[string][]string{"falso": {"instalar": {"9.3.0"}}}, false)
	fu.poner("/lista/compatibilidad.json.sig", "")
	if err := p.LeerLista(ctx); err == nil || p.Comprobada("falso", "instalar", "9.3.0") {
		t.Fatalf("sin firma no se acepta: %v", err)
	}
}

func TestMarcaLocal(t *testing.T) {
	fu := nuevasFuentes(t)
	f := fichaFalsa(t, "instalar")
	f.Receta.Comprobadas = map[string][]string{"iniciar_sesion": {"2.1.288"}}
	pr := conFuentes(t, fu, f)
	p := pr.p
	if !p.Comprobada("falso", "iniciar_sesion", "2.1.288") {
		t.Fatal("la receta la tiene")
	}
	if err := p.Marcar("falso", "2.1.288"); err != nil {
		t.Fatal(err)
	}
	if p.Comprobada("falso", "iniciar_sesion", "2.1.288") || Nuevo(p.o).Comprobada("falso", "iniciar_sesion", "2.1.288") {
		t.Fatal("marcada no está comprobada, aunque el servicio se reinicie")
	}
	fu.lista(t, 1, map[string]map[string][]string{"falso": {"iniciar_sesion": {"2.1.300"}}}, false)
	if err := p.LeerLista(context.Background()); err != nil || p.Comprobada("falso", "iniciar_sesion", "2.1.288") {
		t.Fatalf("una lista que no la trae no quita la marca: %v", err)
	}
	// La marca sale del inicio de sesión: traerla solo para otra acción no la quita.
	fu.lista(t, 2, map[string]map[string][]string{"falso": {"desinstalar": {"2.1.288"}}}, false)
	if err := p.LeerLista(context.Background()); err != nil || p.Comprobada("falso", "iniciar_sesion", "2.1.288") {
		t.Fatalf("una lista que la trae solo para desinstalar no quita la marca: %v", err)
	}
	fu.lista(t, 3, map[string]map[string][]string{"falso": {"iniciar_sesion": {"2.1.*"}}}, false)
	if err := p.LeerLista(context.Background()); err != nil || !p.Comprobada("falso", "iniciar_sesion", "2.1.288") {
		t.Fatalf("la lista con version mayor que la trae quita la marca: %v", err)
	}
}

// El inicio de sesión con una versión sin comprobar no se lanza nunca, ni con Confirmar.
func TestIniciarConVersionNoComprobada(t *testing.T) {
	latido := filepath.Join(t.TempDir(), "latido")
	fu := nuevasFuentes(t)
	f := fichaSesion(t, "codex", "sesion", "PREPARAR_SALIDA="+salidaDe(t, "codex", "sesion-windows.txt"), "PREPARAR_LATIDO="+latido)
	f.Receta.Comprobadas = map[string][]string{"iniciar_sesion": {"1.2.2"}}
	pr := conFuentes(t, fu, f)
	ponerFalso(t, pr.casa)
	for _, confirmar := range []bool{false, true} {
		if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "iniciar_sesion", Confirmar: confirmar}); err != nil {
			t.Fatal(err)
		}
		e := final(t, pr.p, "falso")
		if e.Estado != "error" || e.MotivoCodigo != CodigoVersionNoComprobada || len(e.Lineas) != 0 || e.Enlace != "" {
			t.Fatalf("Confirmar=%v: %+v", confirmar, e)
		}
	}
	if _, err := os.Stat(latido); !os.IsNotExist(err) {
		t.Fatalf("no se lanzó nada: %v", err)
	}
	v := unaVersion(t, pr.p, false)
	if slices.Contains(v.Acciones, "iniciar_sesion") || v.Comprobadas["iniciar_sesion"] || v.MotivoCodigo != CodigoVersionNoComprobada ||
		!strings.Contains(v.Motivo, "Falso CLI 1.2.3 aún no está comprobado") || !strings.HasSuffix(v.Motivo, ordenFalso(pr.casa)) ||
		v.OrdenConsola != ordenFalso(pr.casa) {
		t.Fatalf("Versiones lo dice: %+v", v)
	}
}

// Un instalador que no fija la versión instala la última publicada: si no está comprobada, pide Confirmar y, con él,
// instala sin seguir con el inicio de sesión.
func TestInstalarSinFijarVersionPideConfirmar(t *testing.T) {
	fu := nuevasFuentes(t)
	fu.poner("/texto", "9.9.9")
	f := fichaSesion(t, "codex", "sesion", "PREPARAR_SALIDA="+salidaDe(t, "codex", "sesion-windows.txt"))
	f.Receta.Ultima = adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/texto"}
	f.Receta.Comprobadas = map[string][]string{"instalar": {"1.0.0"}, "iniciar_sesion": {"1.0.0"}}
	pr := conFuentes(t, fu, f)
	v := unaVersion(t, pr.p, true)
	if v.Instalada != "" || v.Ultima != "9.9.9" || v.Comprobadas["instalar"] || !slices.Equal(v.Acciones, []string{"instalar"}) ||
		v.MotivoCodigo != CodigoVersionNoComprobada || !strings.Contains(v.Motivo, "Falso CLI 9.9.9") {
		t.Fatalf("Versiones: %+v", v)
	}
	e := iniciar(t, pr.p, "instalar")
	if e.Estado != "error" || e.MotivoCodigo != CodigoVersionNoComprobada || e.Mensaje != "Falso CLI 9.9.9 aún no está comprobado con Tramtai" {
		t.Fatalf("sin Confirmar: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(pr.casa, "bin", nombreFalso())); !os.IsNotExist(err) {
		t.Fatalf("sin Confirmar no se instala: %v", err)
	}
	if _, err := pr.p.Iniciar("falso", contrato.PedidoPreparacion{Accion: "instalar", Confirmar: true}); err != nil {
		t.Fatal(err)
	}
	e = final(t, pr.p, "falso")
	if lineas := strings.Join(e.Lineas, "\n"); e.Estado != "listo" || e.Enlace != "" || !strings.Contains(lineas, "inicio de sesión automático") {
		t.Fatalf("con Confirmar se instala y el inicio de sesión queda desactivado: %+v", e)
	}
}

// Con un instalador que fija la versión se instala la comprobada más reciente (por número, sin las marcadas).
func TestInstalarFijaLaComprobadaMasReciente(t *testing.T) {
	fu := nuevasFuentes(t)
	f := fichaFalsa(t, "instalar")
	f.Receta.FijaVersion = true
	f.Receta.Comprobadas = map[string][]string{"instalar": {"1.9.0", "1.10.0", "1.*"}}
	pr := conFuentes(t, fu, f)
	instalada := func() string {
		t.Helper()
		if e := iniciar(t, pr.p, "instalar"); e.Estado != "listo" {
			t.Fatalf("estado %+v", e)
		}
		registro, _ := os.ReadFile(filepath.Join(pr.datos, "agentes", "preparaciones", "falso.log"))
		for _, v := range []string{"1.10.0", "1.9.0"} {
			if strings.Contains(strings.ReplaceAll(string(registro), "\r", ""), " "+v+"\n") { // la orden lleva la versión al final
				return v
			}
		}
		t.Fatalf("no se fijó ninguna versión:\n%s", registro)
		return ""
	}
	if v := instalada(); v != "1.10.0" {
		t.Fatalf("se instaló %s", v)
	}
	if err := pr.p.Marcar("falso", "1.10.0"); err != nil {
		t.Fatal(err)
	}
	if v := instalada(); v != "1.9.0" {
		t.Fatalf("con 1.10.0 marcada se instaló %s", v)
	}
	if fu.pedidos.Load() != 0 {
		t.Fatalf("con la versión fijada no hace falta consultar la última: %d pedidos", fu.pedidos.Load())
	}
}

// Con servicio.buscar_versiones_agentes=false no se hace ninguna petición.
func TestSinBusqueda(t *testing.T) {
	fu := nuevasFuentes(t)
	fu.poner("/texto", "2.1.323")
	fu.lista(t, 1, map[string]map[string][]string{"falso": {"instalar": {"2.1.323"}}}, false)
	f := fichaFalsa(t, "instalar")
	f.Receta.Ultima = adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/texto"}
	pr := conFuentes(t, fu, f)
	pr.p.o.BuscarVersiones = func() bool { return false }
	if v := unaVersion(t, pr.p, true); v.Ultima != "" || v.MotivoCodigo == CodigoSinRed {
		t.Fatalf("Versiones: %+v", v)
	}
	if err := pr.p.LeerLista(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e := iniciar(t, pr.p, "instalar"); e.MotivoCodigo != CodigoVersionNoComprobada {
		t.Fatalf("sin la última versión no se sabe si está comprobada: %+v", e)
	}
	if n := fu.pedidos.Load(); n != 0 {
		t.Fatalf("%d peticiones con la búsqueda desactivada", n)
	}
}

func TestCompararVersiones(t *testing.T) {
	casos := []struct {
		a, b string
		c    int
	}{
		{"1.10.0", "1.9.0", 1}, {"2.0.0-beta", "2.0.0", -1}, {"2.0.0", "2.0.0-beta", 1}, {"1.10-rc", "1.9", 1},
		{"1.2", "1.2.0", 0}, {"1.2.0", "1.2", 0}, {"2.0.0-alpha", "2.0.0-beta", -1}, {"1.2.3", "1.2.3", 0}, {"0.167.2", "0.167.10", -1},
	}
	for _, c := range casos {
		if got := compararVersiones(c.a, c.b); got != c.c {
			t.Errorf("compararVersiones(%q, %q) = %d, se esperaba %d", c.a, c.b, got, c.c)
		}
	}
}

// Un fallo pasajero no borra la última versión que ya se sabía, y la próxima llamada lo vuelve a intentar.
func TestVersionesFalloPasajero(t *testing.T) {
	fu := nuevasFuentes(t)
	fu.poner("/texto2", "2.1.330")
	otro := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "2.1.323") }))
	f := fichaFalsa(t, "instalar")
	f.Receta.Ultima = adaptadores.FuenteVersion{Tipo: "url", Nombre: otro.URL + "/texto"}
	pr := conFuentes(t, fu, f)
	if v := unaVersion(t, pr.p, false); v.Ultima != "2.1.323" {
		t.Fatalf("la primera: %+v", v)
	}
	otro.Close()
	viejo := time.Now().Add(-25 * time.Hour)
	if err := pr.p.cambiar(func(c *compatibilidad) { c.Leida = viejo }); err != nil {
		t.Fatal(err)
	}
	if v := unaVersion(t, pr.p, false); v.Ultima != "2.1.323" || v.MotivoCodigo == CodigoSinRed {
		t.Fatalf("sin red se conserva la que se sabía: %+v", v)
	}
	if leida := pr.p.compatibilidad().Leida; !leida.Equal(viejo) {
		t.Fatalf("sin ninguna respuesta no se da por consultado: %v", leida)
	}
	f.Receta.Ultima.Nombre = fu.srv.URL + "/texto2" // la receta es la misma que ve el preparador
	if v := unaVersion(t, pr.p, false); v.Ultima != "2.1.330" {
		t.Fatalf("la siguiente llamada vuelve a intentarlo: %+v", v)
	}
}

// Versiones, LeerLista y Marcar a la vez no pierden nada de compatibilidad.json.
func TestCompatibilidadALaVez(t *testing.T) {
	fu := nuevasFuentes(t)
	fu.poner("/texto", "2.1.323")
	fu.lista(t, 1, map[string]map[string][]string{"falso": {"instalar": {"3.0.0"}}}, false)
	f := fichaFalsa(t, "instalar")
	f.Receta.Ultima = adaptadores.FuenteVersion{Tipo: "url", Nombre: fu.srv.URL + "/texto"}
	pr := conFuentes(t, fu, f)
	var grupo sync.WaitGroup
	marcadas := []string{}
	for i := range 8 {
		marcadas = append(marcadas, fmt.Sprintf("1.0.%d", i))
		grupo.Go(func() { pr.p.Versiones(context.Background(), true) })
		grupo.Go(func() { _ = pr.p.LeerLista(context.Background()) })
		grupo.Go(func() {
			if err := pr.p.Marcar("falso", fmt.Sprintf("1.0.%d", i)); err != nil {
				t.Error(err)
			}
		})
	}
	grupo.Wait()
	c := pr.p.compatibilidad()
	slices.Sort(c.NoCompatibles["falso"])
	if c.Ultimas["falso"] != "2.1.323" || c.Lista.Version != 1 || !slices.Equal(c.NoCompatibles["falso"], marcadas) || c.Leida.IsZero() {
		t.Fatalf("compatibilidad.json: %+v", c)
	}
}

// Los avisos de Versiones al instalar o actualizar una versión sin comprobar.
func TestAvisosDeVersiones(t *testing.T) {
	f := fichaSesion(t, "codex", "sesion")
	p := nueva(t, f).p
	p.o.Comprobada = func(_, accion, _ string) bool { return accion != "actualizar" && accion != "instalar" }
	if v := p.versionesDe("falso", f, "1.2.3", "9.9.9", true); v.MotivoCodigo != CodigoVersionNoComprobada ||
		v.Motivo != "La versión nueva aún no está comprobada: si actualizas, el inicio de sesión automático quedará desactivado hasta que lo esté" ||
		!slices.Contains(v.Acciones, "actualizar") {
		t.Fatalf("actualizar a una sin comprobar: %+v", v)
	}
	if v := p.versionesDe("falso", f, "1.2.3", "", false); v.MotivoCodigo != CodigoVersionNoComprobada || !strings.HasPrefix(v.Motivo, "No se sabe qué versión se instalará") {
		t.Fatalf("sin saber la nueva: %+v", v)
	}
	if v := p.versionesDe("falso", f, "", "9.9.9", true); !strings.HasPrefix(v.Motivo, "Falso CLI 9.9.9 aún no está comprobado con Tramtai: si sigues") {
		t.Fatalf("instalar una sin comprobar: %+v", v)
	}
}
