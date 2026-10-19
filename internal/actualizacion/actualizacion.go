// Package actualizacion consulta la última versión publicada de Tramtai y, en Windows, descarga su instalador,
// comprueba la suma SHA-256 y lo ejecuta en silencio. Solo habla con GitHub y solo para pedir la ficha pública de la
// última versión y la lista de versiones publicadas (sus notas): no envía ningún dato del usuario.
package actualizacion

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/version"
)

const repositorio = "MscottB/Tramtai"

var (
	// Variables para que las pruebas apunten a un servidor local.
	urlUltima       = "https://api.github.com/repos/" + repositorio + "/releases/latest"
	urlVersiones    = "https://api.github.com/repos/" + repositorio + "/releases?per_page=30"
	prefijoDescarga = "https://github.com/" + repositorio + "/releases/download/"
	clienteHTTP     = &http.Client{Timeout: 10 * time.Minute}
)

const (
	sufijoInstalador = "-instalador-windows-x64.exe"
	archivoSumas     = "SHA256SUMS.txt"
	maximoInstalador = 500 << 20
	// PrefijoCarpeta nombra las carpetas temporales de las descargas, para encontrarlas y borrarlas después.
	PrefijoCarpeta = "tramtai-actualizacion"
)

// Version es una versión publicada.
type Version struct {
	Etiqueta         string `json:"version"`
	Pagina           string `json:"url"`
	Notas            string `json:"notas,omitempty"` // las novedades, para leerlas dentro de la app
	nombreInstalador string
	urlInstalador    string
	urlSumas         string
	urlFirma         string
}

// TieneInstalador indica si la versión publicó el instalador de Windows, sus sumas y la firma de las sumas.
func (v Version) TieneInstalador() bool { return v.urlInstalador != "" && v.urlSumas != "" && v.urlFirma != "" }

// Ultima pide a GitHub la última versión publicada.
func Ultima(ctx context.Context) (Version, error) {
	datos, err := bajar(ctx, urlUltima, 1<<20, "application/vnd.github+json")
	if err != nil {
		return Version{}, err
	}
	var ficha struct {
		Etiqueta string `json:"tag_name"`
		Pagina   string `json:"html_url"`
		Notas    string `json:"body"`
		Archivos []struct {
			Nombre string `json:"name"`
			URL    string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(datos, &ficha); err != nil {
		return Version{}, err
	}
	if _, ok := numeros(ficha.Etiqueta); !ok {
		return Version{}, fmt.Errorf(idioma.T("la última versión publicada no tiene un número válido: %q"), ficha.Etiqueta)
	}
	v := armar(ficha.Etiqueta, ficha.Pagina, ficha.Notas)
	for _, a := range ficha.Archivos {
		switch {
		case strings.HasSuffix(a.Nombre, sufijoInstalador):
			v.nombreInstalador, v.urlInstalador = a.Nombre, a.URL
		case a.Nombre == archivoSumas:
			v.urlSumas = a.URL
		case a.Nombre == archivoFirma:
			v.urlFirma = a.URL
		}
	}
	return v, nil
}

// armar arma una versión con lo que la Web UI puede mostrar: las notas recortadas y la página como enlace, solo si
// es del repositorio (nunca javascript: ni otros sitios).
func armar(etiqueta, pagina, notas string) Version {
	v := Version{Etiqueta: etiqueta, Notas: recortar(notas, 12000)}
	if strings.HasPrefix(pagina, "https://github.com/"+repositorio+"/") {
		v.Pagina = pagina
	}
	return v
}

// Novedades devuelve las versiones publicadas después de desde y hasta hasta, de la más nueva a la más vieja y como
// mucho 10, para leer tras actualizar lo que trajo cada una. Sin desde válido, solo la de hasta.
func Novedades(ctx context.Context, desde, hasta string) ([]Version, error) {
	fichas, err := listaVersiones(ctx)
	if err != nil {
		return nil, err
	}
	_, conDesde := numeros(desde)
	h, conHasta := numeros(hasta)
	if !conHasta {
		return []Version{}, nil // una compilación sin versión (dev) no tiene novedades
	}
	lista := []Version{}
	for _, f := range fichas {
		n, ok := numeros(f.Etiqueta)
		if !ok || f.Borrador || f.Previa {
			continue
		}
		if (conDesde && MasNueva(f.Etiqueta, desde) && !MasNueva(f.Etiqueta, hasta)) || (!conDesde && slices.Equal(n, h)) {
			lista = append(lista, armar(f.Etiqueta, f.Pagina, f.Notas))
		}
	}
	slices.SortFunc(lista, func(a, b Version) int {
		x, _ := numeros(a.Etiqueta)
		y, _ := numeros(b.Etiqueta)
		return slices.Compare(y, x)
	})
	return lista[:min(len(lista), 10)], nil
}

// fichaVersion es lo que se lee de cada versión de la lista de GitHub.
type fichaVersion struct {
	Etiqueta string `json:"tag_name"`
	Pagina   string `json:"html_url"`
	Notas    string `json:"body"`
	Borrador bool   `json:"draft"`
	Previa   bool   `json:"prerelease"`
}

// La lista de versiones se guarda 10 minutos: GitHub admite 60 consultas por hora sin autenticar y cada navegador
// que se actualiza la pide al abrirse.
var cacheVersiones struct {
	sync.Mutex
	fichas []fichaVersion
	cuando time.Time
}

func listaVersiones(ctx context.Context) ([]fichaVersion, error) {
	cacheVersiones.Lock()
	defer cacheVersiones.Unlock()
	if cacheVersiones.fichas != nil && time.Since(cacheVersiones.cuando) < 10*time.Minute {
		return cacheVersiones.fichas, nil
	}
	datos, err := bajar(ctx, urlVersiones, 4<<20, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	fichas := []fichaVersion{}
	if err := json.Unmarshal(datos, &fichas); err != nil {
		return nil, err
	}
	cacheVersiones.fichas, cacheVersiones.cuando = fichas, time.Now()
	return fichas, nil
}

// reiniciarCache vacía la lista guardada (para las pruebas).
func reiniciarCache() {
	cacheVersiones.Lock()
	defer cacheVersiones.Unlock()
	cacheVersiones.fichas = nil
}

// recortar deja texto en un máximo de runas, sin cortar un carácter por la mitad.
func recortar(texto string, maximo int) string {
	runas := []rune(strings.TrimSpace(texto))
	if len(runas) <= maximo {
		return string(runas)
	}
	return string(runas[:maximo]) + "…"
}

// MasNueva indica si la versión publicada es posterior a la actual. Compara vX.Y.Z, no la fecha (+DDMMAAAA); una
// compilación sin versión (dev) nunca se da por desactualizada.
func MasNueva(publicada, actual string) bool {
	p, ok1 := numeros(publicada)
	a, ok2 := numeros(actual)
	return ok1 && ok2 && slices.Compare(p, a) > 0
}

func numeros(v string) ([]int, bool) {
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "+")
	partes := strings.Split(v, ".")
	if len(partes) != 3 {
		return nil, false
	}
	n := make([]int, 3)
	for i, p := range partes {
		x, err := strconv.Atoi(p)
		if err != nil || x < 0 {
			return nil, false
		}
		n[i] = x
	}
	return n, true
}

// Comprobador recuerda la última consulta y la repite como mucho una vez por hora (o a los 15 minutos si falló), así
// una versión publicada aparece sin tener que reiniciar la app.
type Comprobador struct {
	Consultar func(context.Context) (Version, error) // Ultima si es nil
	Actual    string                                 // version.Version si está vacía

	mu      sync.Mutex
	ultima  Version
	hecha   time.Time
	fallo   bool
	enCurso chan struct{}
}

// Disponible devuelve la última versión publicada e indica si es posterior a la actual. Si la consulta anterior
// venció, lanza otra en segundo plano; con esperar, aguarda su resultado mientras ctx lo permita.
func (c *Comprobador) Disponible(ctx context.Context, esperar bool) (Version, bool) {
	c.mu.Lock()
	vencida := c.hecha.IsZero() || time.Since(c.hecha) > time.Hour || (c.fallo && time.Since(c.hecha) > 15*time.Minute)
	if vencida && c.enCurso == nil {
		c.enCurso = make(chan struct{})
		go c.consultar(c.enCurso)
	}
	listo := c.enCurso
	c.mu.Unlock()
	if esperar && listo != nil {
		select {
		case <-listo:
		case <-ctx.Done():
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	actual := c.Actual
	if actual == "" {
		actual = version.Version
	}
	return c.ultima, MasNueva(c.ultima.Etiqueta, actual)
}

// Renovar hace que la próxima consulta vaya a GitHub aunque la anterior no haya vencido («Buscar ahora»). Si la
// última es de hace menos de 10 segundos no cambia nada: GitHub admite 60 consultas por hora sin autenticar.
func (c *Comprobador) Renovar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.enCurso == nil && time.Since(c.hecha) > 10*time.Second {
		c.hecha = time.Time{}
	}
}

// Comprobado indica cuándo terminó la última consulta (cero si aún no hubo ninguna) y si falló.
func (c *Comprobador) Comprobado() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hecha, c.fallo
}

func (c *Comprobador) consultar(listo chan struct{}) {
	ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelar()
	consultar := c.Consultar
	if consultar == nil {
		consultar = Ultima
	}
	v, err := consultar(ctx)
	c.mu.Lock()
	if err == nil {
		c.ultima = v
	}
	c.hecha, c.fallo, c.enCurso = time.Now(), err != nil, nil
	c.mu.Unlock()
	close(listo)
}

// Descargar baja a dir el instalador de v y comprueba que su SHA-256 coincide con la de SHA256SUMS.txt de la misma
// versión, y antes que esas sumas llevan la firma de Tramtai. Solo acepta direcciones de descarga del repositorio de
// Tramtai; si la suma no coincide, borra el archivo.
// Avance recibe los bytes descargados y el total (0 si GitHub no lo informa), como mucho cada 100 ms y al terminar.
type Avance func(recibidos, total int64)

func Descargar(ctx context.Context, v Version, dir string, avance Avance) (string, error) {
	LimpiarDescargas(time.Now().Add(-10 * time.Minute))
	if !v.TieneInstalador() {
		return "", errors.New(idioma.T("esta versión no publicó instalador para Windows"))
	}
	for _, u := range []string{v.urlInstalador, v.urlSumas, v.urlFirma} {
		if !strings.HasPrefix(u, prefijoDescarga) || strings.Contains(u, "..") || strings.Contains(strings.ToLower(u), "%2e") {
			return "", fmt.Errorf(idioma.T("dirección de descarga inesperada: %s"), u)
		}
	}
	sumas, err := bajar(ctx, v.urlSumas, 1<<16, "")
	if err != nil {
		return "", err
	}
	firma, err := bajar(ctx, v.urlFirma, 1<<10, "")
	if err != nil {
		return "", err
	}
	if err := verificarFirma(sumas, firma); err != nil {
		return "", err
	}
	esperada := ""
	lineas := bufio.NewScanner(bytes.NewReader(sumas))
	for lineas.Scan() {
		if campos := strings.Fields(lineas.Text()); len(campos) == 2 && strings.TrimPrefix(campos[1], "*") == v.nombreInstalador {
			esperada = strings.ToLower(campos[0])
		}
	}
	if esperada == "" {
		return "", fmt.Errorf(idioma.T("SHA256SUMS.txt no incluye %s"), v.nombreInstalador)
	}
	destino := filepath.Join(dir, filepath.Base(v.nombreInstalador))
	suma, err := guardar(ctx, v.urlInstalador, destino, avance)
	if err == nil && suma != esperada {
		err = fmt.Errorf(idioma.T("la suma SHA-256 de %s no coincide: la descarga está dañada o fue alterada"), v.nombreInstalador)
	}
	if err != nil {
		os.Remove(destino)
		return "", err
	}
	return destino, nil
}

// LimpiarDescargas borra las carpetas de descargas anteriores a antes: el instalador no puede borrarse a sí mismo
// mientras se ejecuta, así que lo hace la siguiente descarga o el servicio al arrancar. Lo que siga en uso se deja
// para la próxima vez.
func LimpiarDescargas(antes time.Time) {
	entradas, _ := os.ReadDir(os.TempDir())
	for _, e := range entradas {
		if info, err := e.Info(); err == nil && e.IsDir() && strings.HasPrefix(e.Name(), PrefijoCarpeta) && info.ModTime().Before(antes) {
			os.RemoveAll(filepath.Join(os.TempDir(), e.Name()))
		}
	}
}

func pedir(ctx context.Context, url, aceptar string) (*http.Response, error) {
	peticion, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	peticion.Header.Set("User-Agent", "Tramtai/"+version.Texto())
	if aceptar != "" {
		peticion.Header.Set("Accept", aceptar)
	}
	respuesta, err := clienteHTTP.Do(peticion)
	if err != nil {
		return nil, err
	}
	if respuesta.StatusCode != http.StatusOK {
		respuesta.Body.Close()
		return nil, fmt.Errorf(idioma.T("GitHub respondió %s"), respuesta.Status)
	}
	return respuesta, nil
}

func bajar(ctx context.Context, url string, limite int64, aceptar string) ([]byte, error) {
	respuesta, err := pedir(ctx, url, aceptar)
	if err != nil {
		return nil, err
	}
	defer respuesta.Body.Close()
	return io.ReadAll(io.LimitReader(respuesta.Body, limite))
}

// contador avisa a avance de lo que se va escribiendo.
type contador struct {
	recibidos, total int64
	avance           Avance
	ultimo           time.Time
}

func (c *contador) Write(p []byte) (int, error) {
	c.recibidos += int64(len(p))
	if c.avance != nil && (time.Since(c.ultimo) >= 100*time.Millisecond || c.recibidos == c.total) {
		c.ultimo = time.Now()
		c.avance(c.recibidos, c.total)
	}
	return len(p), nil
}

// guardar escribe la descarga en destino y devuelve su SHA-256 en hexadecimal.
func guardar(ctx context.Context, url, destino string, avance Avance) (string, error) {
	respuesta, err := pedir(ctx, url, "")
	if err != nil {
		return "", err
	}
	defer respuesta.Body.Close()
	archivo, err := os.Create(destino)
	if err != nil {
		return "", err
	}
	suma := sha256.New()
	cuenta := &contador{total: max(respuesta.ContentLength, 0), avance: avance}
	_, err = io.Copy(io.MultiWriter(archivo, suma, cuenta), io.LimitReader(respuesta.Body, maximoInstalador))
	if err == nil && avance != nil {
		avance(cuenta.recibidos, cuenta.total) // el último aviso, aunque no haya pasado el intervalo
	}
	if cerrar := archivo.Close(); err == nil {
		err = cerrar
	}
	return hex.EncodeToString(suma.Sum(nil)), err
}
