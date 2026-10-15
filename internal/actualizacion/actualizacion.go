// Package actualizacion consulta la última versión publicada de Tramtai y, en Windows, descarga su instalador,
// comprueba la suma SHA-256 y lo ejecuta en silencio. Solo habla con GitHub y solo para pedir la ficha pública de la
// última versión: no envía ningún dato del usuario.
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
	prefijoDescarga = "https://github.com/" + repositorio + "/releases/download/"
	clienteHTTP     = &http.Client{Timeout: 10 * time.Minute}
)

const (
	sufijoInstalador = "-instalador-windows-x64.exe"
	archivoSumas     = "SHA256SUMS.txt"
	maximoInstalador = 500 << 20
)

// Version es una versión publicada.
type Version struct {
	Etiqueta         string `json:"version"`
	Pagina           string `json:"url"`
	Notas            string `json:"notas,omitempty"` // las novedades, para leerlas dentro de la app
	nombreInstalador string
	urlInstalador    string
	urlSumas         string
}

// TieneInstalador indica si la versión publicó el instalador de Windows y sus sumas.
func (v Version) TieneInstalador() bool { return v.urlInstalador != "" && v.urlSumas != "" }

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
	v := Version{Etiqueta: ficha.Etiqueta, Pagina: ficha.Pagina, Notas: recortar(ficha.Notas, 6000)}
	for _, a := range ficha.Archivos {
		switch {
		case strings.HasSuffix(a.Nombre, sufijoInstalador):
			v.nombreInstalador, v.urlInstalador = a.Nombre, a.URL
		case a.Nombre == archivoSumas:
			v.urlSumas = a.URL
		}
	}
	return v, nil
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

// Comprobador recuerda la última consulta y la repite como mucho una vez al día (o al cabo de una hora si falló).
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
	vencida := c.hecha.IsZero() || time.Since(c.hecha) > 24*time.Hour || (c.fallo && time.Since(c.hecha) > time.Hour)
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
// versión. Solo acepta direcciones de descarga del repositorio de Tramtai; si la suma no coincide, borra el archivo.
func Descargar(ctx context.Context, v Version, dir string) (string, error) {
	if !v.TieneInstalador() {
		return "", errors.New(idioma.T("esta versión no publicó instalador para Windows"))
	}
	for _, u := range []string{v.urlInstalador, v.urlSumas} {
		if !strings.HasPrefix(u, prefijoDescarga) {
			return "", fmt.Errorf(idioma.T("dirección de descarga inesperada: %s"), u)
		}
	}
	sumas, err := bajar(ctx, v.urlSumas, 1<<16, "")
	if err != nil {
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
	suma, err := guardar(ctx, v.urlInstalador, destino)
	if err == nil && suma != esperada {
		err = fmt.Errorf(idioma.T("la suma SHA-256 de %s no coincide: la descarga está dañada o fue alterada"), v.nombreInstalador)
	}
	if err != nil {
		os.Remove(destino)
		return "", err
	}
	return destino, nil
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

// guardar escribe la descarga en destino y devuelve su SHA-256 en hexadecimal.
func guardar(ctx context.Context, url, destino string) (string, error) {
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
	_, err = io.Copy(io.MultiWriter(archivo, suma), io.LimitReader(respuesta.Body, maximoInstalador))
	if cerrar := archivo.Close(); err == nil {
		err = cerrar
	}
	return hex.EncodeToString(suma.Sum(nil)), err
}
