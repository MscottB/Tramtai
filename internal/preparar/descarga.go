package preparar

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
)

// arquitecturas da el {arq} de una descarga según runtime.GOARCH.
var arquitecturas = map[string]string{"amd64": "x64", "arm64": "arm64"}

// maximoDescarga acota lo que se baja: el zip de Copilot ronda los 100 MB.
const maximoDescarga = 1 << 30

// errorSinHTTPS es una redirección que deja HTTPS: es la descarga la que falla, no la red.
type errorSinHTTPS string

func (e errorSinHTTPS) Error() string { return idioma.Tf("solo se descarga por HTTPS: %s", string(e)) }

// clienteHTTPS no sigue una redirección que deje HTTPS.
func clienteHTTPS() *http.Client {
	return &http.Client{CheckRedirect: func(r *http.Request, previas []*http.Request) error {
		if r.URL.Scheme != "https" {
			return errorSinHTTPS(r.URL.Redacted())
		}
		if len(previas) >= 10 {
			return errors.New(idioma.T("demasiadas redirecciones"))
		}
		return nil
	}}
}

// descargar baja el zip de la versión, comprueba su SHA-256 con la lista de sumas publicada y solo entonces deja el
// ejecutable en destino (la primera ruta de la receta), con el nombre de esa ruta.
func (c *corrida) descargar(d adaptadores.DescargaVerificada, version, destino string) error {
	arq := arquitecturas[runtime.GOARCH]
	if arq == "" {
		return &Error{CodigoInstaladorFallo, idioma.Tf("no hay descarga para la arquitectura %s", runtime.GOARCH)}
	}
	if err := validarVersion(version); err != nil {
		return err
	}
	url := strings.NewReplacer("{version}", version, "{arq}", arq).Replace(d.URL)
	urlSumas := strings.ReplaceAll(d.URLSumas, "{version}", version)
	for _, u := range []string{url, urlSumas} {
		if !strings.HasPrefix(u, "https://") {
			return &Error{CodigoInstaladorFallo, errorSinHTTPS(u).Error()}
		}
	}
	// Lo que dejó una descarga cortada (el servicio cayó a mitad) no se queda.
	sobrantes, _ := filepath.Glob(filepath.Join(filepath.Dir(destino), ".descarga-*.zip"))
	for _, s := range append(sobrantes, destino+".nuevo") {
		_ = os.Remove(s)
	}
	c.anotar("GET " + url)
	var sumas strings.Builder
	if err := c.bajar(urlSumas, &sumas, 1<<20); err != nil {
		return err
	}
	esperada := sumaDe(sumas.String(), path.Base(url))
	if esperada == "" {
		return &Error{CodigoInstaladorFallo, idioma.Tf("%s no está en la lista de sumas de la versión", path.Base(url))}
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return &Error{CodigoInstaladorFallo, err.Error()}
	}
	temporal, err := os.CreateTemp(filepath.Dir(destino), ".descarga-*.zip")
	if err != nil {
		return &Error{CodigoInstaladorFallo, err.Error()}
	}
	defer os.Remove(temporal.Name())
	defer temporal.Close()
	hash := sha256.New()
	if err := c.bajar(url, io.MultiWriter(temporal, hash), maximoDescarga); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != esperada {
		return &Error{CodigoInstaladorFallo, idioma.Tf("la suma SHA-256 de %s no coincide con la publicada", path.Base(url))}
	}
	return extraer(temporal, filepath.Base(destino), destino)
}

// bajar copia el cuerpo de url en w (como mucho limite bytes). Sin conexión es sin_red; una respuesta mala, instalador_fallo.
func (c *corrida) bajar(url string, w io.Writer, limite int64) error {
	pedido, err := http.NewRequestWithContext(c.ctx, http.MethodGet, url, nil)
	if err != nil {
		return &Error{CodigoInstaladorFallo, err.Error()}
	}
	r, err := c.p.cliente.Do(pedido)
	var sinHTTPS errorSinHTTPS
	switch {
	case err == nil:
	case c.ctx.Err() != nil:
		return c.ctx.Err()
	case errors.As(err, &sinHTTPS):
		return &Error{CodigoInstaladorFallo, sinHTTPS.Error()}
	default:
		return &Error{CodigoSinRed, idioma.Tf("no se pudo descargar %s: %s", url, err.Error())}
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return &Error{CodigoInstaladorFallo, idioma.Tf("%s respondió %d", url, r.StatusCode)}
	}
	n, err := io.Copy(w, io.LimitReader(r.Body, limite+1))
	switch {
	case c.ctx.Err() != nil:
		return c.ctx.Err()
	case err != nil:
		return &Error{CodigoSinRed, idioma.Tf("no se pudo descargar %s: %s", url, err.Error())}
	case n > limite:
		return &Error{CodigoInstaladorFallo, idioma.Tf("%s es más grande de lo esperado", url)}
	}
	return nil
}

// sumaDe busca en un SHA256SUMS («<sha256 en minúsculas>  <archivo>» por línea) la suma de nombre.
func sumaDe(sumas, nombre string) string {
	escaner := bufio.NewScanner(strings.NewReader(sumas))
	for escaner.Scan() {
		if campos := strings.Fields(escaner.Text()); len(campos) == 2 && strings.TrimPrefix(campos[1], "*") == nombre {
			return strings.ToLower(campos[0])
		}
	}
	return ""
}

// extraer saca del zip ya verificado el archivo nombre y lo deja en destino de una vez (por un temporal y un renombrado).
func extraer(f *os.File, nombre, destino string) error {
	info, err := f.Stat()
	if err != nil {
		return &Error{CodigoInstaladorFallo, err.Error()}
	}
	z, err := zip.NewReader(f, info.Size())
	if err != nil {
		return &Error{CodigoInstaladorFallo, err.Error()}
	}
	for _, entrada := range z.File {
		if entrada.Name != nombre {
			continue
		}
		origen, err := entrada.Open()
		if err != nil {
			return &Error{CodigoInstaladorFallo, err.Error()}
		}
		defer origen.Close()
		nuevo := destino + ".nuevo"
		salida, err := os.OpenFile(nuevo, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
		if err != nil {
			return &Error{CodigoInstaladorFallo, err.Error()}
		}
		n, err := io.Copy(salida, io.LimitReader(origen, maximoDescarga+1))
		if cerrar := salida.Close(); err == nil {
			err = cerrar
		}
		if err == nil && n > maximoDescarga { // nunca un binario cortado en silencio
			err = errors.New(idioma.Tf("%s es más grande de lo esperado", nombre))
		}
		if err == nil {
			err = os.Rename(nuevo, destino)
		}
		if err != nil {
			os.Remove(nuevo)
			return &Error{CodigoInstaladorFallo, err.Error()}
		}
		return nil
	}
	return &Error{CodigoInstaladorFallo, idioma.Tf("la descarga no trae %s", nombre)}
}
