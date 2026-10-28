// Package servidor instala y administra el servicio del sistema (modo servidor): la unidad systemd en Linux y el
// servicio de Windows, con su cuenta sin privilegios, su programa y su carpeta de datos. Lo propio de cada sistema va en
// linux.go, windows.go y otros.go.
package servidor

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"tramtai/internal/carpetas"
	"tramtai/internal/idioma"
)

// Cuenta es la cuenta sin privilegios con la que corre el servicio del sistema en Linux; CuentaWindows, la cuenta
// virtual del servicio de Windows.
const (
	Cuenta        = "tramtai"
	CuentaWindows = `NT SERVICE\Tramtai`
)

// Sitios son las rutas del servidor: la carpeta del programa (tramtai y uv), la de datos (la raíz del servidor), el
// envoltorio tramtai-servidor y la unidad systemd y, para las claves (solo Linux), la carpeta de las credenciales
// cifradas, el drop-in que las carga y el archivo de entorno de root sin systemd-creds. El tipo no se llama Rutas
// porque ese nombre es el de la función.
type Sitios struct {
	Programa, Datos, Envoltorio, Unidad string
	Cifradas, ClavesConf, ClavesEnv     string
}

// rutas las sustituyen las pruebas para no tocar /opt, /var/lib ni /etc.
var rutas = func() Sitios {
	if runtime.GOOS == "windows" {
		programa := filepath.Join(cmp.Or(os.Getenv("ProgramFiles"), `C:\Program Files`), "Tramtai Servidor")
		return Sitios{Programa: programa, Datos: carpetas.RaizServidor(), Envoltorio: filepath.Join(programa, "tramtai-servidor.cmd")}
	}
	return Sitios{Programa: "/opt/tramtai", Datos: carpetas.RaizServidor(), Envoltorio: "/usr/local/bin/tramtai-servidor",
		Unidad: "/etc/systemd/system/tramtai.service", Cifradas: "/etc/credstore.encrypted",
		ClavesConf: "/etc/systemd/system/tramtai.service.d/claves.conf", ClavesEnv: "/etc/tramtai/claves.env"}
}

// Rutas devuelve las rutas del servidor en este sistema.
func Rutas() Sitios { return rutas() }

// Opciones de Instalar.
type Opciones struct {
	Origen         string // carpeta del programa que se copia (la de este tramtai)
	AceptoTerminos bool   // sin aceptar los términos no se instala nada
	Como           string // cómo se aceptaron: terminos.ComoServidorInstalar (escribiendo «acepto») o terminos.ComoOpcion
	Conversor      string // paquete .zip del conversor que se prepara sin red
	LetsEncrypt    bool   // Let's Encrypt integrado (TLS-ALPN-01): solo escucha en el 443
	PuertoBajo     bool   // escucha en un puerto menor de 1024
}

// EstadoSistema es lo que dice el sistema del servicio.
type EstadoSistema struct {
	Instalado     bool   // la unidad (o el servicio de Windows) existe
	Activo        string // lo que dice el sistema: active, inactive, failed… (vacío sin instalar)
	ClavesEnClaro bool   // hay claves en el archivo de entorno de root, sin cifrar (Linux sin systemd-creds)
}

// Aviso es lo que Desinstalar no pudo terminar sin que sea un fallo (el perfil de la cuenta del servicio que Windows
// aún tiene cargado): la desinstalación se hizo y quien llama lo muestra.
type Aviso string

func (a Aviso) Error() string { return string(a) }

// ejecutar lanza una orden del sistema y devuelve su salida; ejecutarConEntrada, con algo por stdin (una clave, que así
// no aparece en la lista de procesos). Las pruebas los sustituyen.
var (
	ejecutar = func(nombre string, args ...string) ([]byte, error) {
		return exec.Command(nombre, args...).CombinedOutput()
	}
	ejecutarConEntrada = func(entrada []byte, nombre string, args ...string) ([]byte, error) {
		cmd := exec.Command(nombre, args...)
		cmd.Stdin = bytes.NewReader(entrada)
		return cmd.CombinedOutput()
	}
)

// ErrClaveEnClaro es el aviso de GuardarClave cuando el sistema no tiene systemd-creds 250 o posterior: la clave queda
// guardada, pero sin cifrar en el archivo de entorno de root.
var ErrClaveEnClaro = Aviso(idioma.T("este systemd no tiene systemd-creds 250 o posterior: las claves del servidor están sin cifrar en /etc/tramtai/claves.env (0600, solo las lee root)"))

// AvisoLetsEncrypt es lo que dicen el registro al arrancar, «servidor estado» y «servidor reiniciar» con
// servicio.tls.modo letsencrypt: ese modo acepta el acuerdo de suscriptor de Let's Encrypt (autocert.AcceptTOS).
func AvisoLetsEncrypt() string {
	return idioma.T("con servicio.tls.modo letsencrypt, Tramtai acepta en tu nombre el acuerdo de suscriptor de Let's Encrypt (https://letsencrypt.org/repository/) y le da tu correo al pedir el certificado; si no lo aceptas, usa archivos o un proxy")
}

// ErrInstaladoPorPaquete es lo que dice Actualizar si el programa lo puso el paquete .deb o .rpm (que deja .paquete en
// su carpeta): lo actualizan apt o dnf.
var ErrInstaladoPorPaquete = errors.New(idioma.T("este servidor se instaló con el paquete tramtai-servidor: actualízalo con apt o dnf"))

// Actualizar pone la última versión publicada en la carpeta del programa: bajar deja en una carpeta temporal el paquete
// del servidor de este sistema con la firma de sus sumas y su suma comprobadas (actualizacion.PaqueteServidor, que pasa
// cmd/tramtai: este paquete no importa el actualizador, porque lo enlaza la app de escritorio por AvisoCruzado). Saca de
// él el programa (como .nuevo, junto al de ahora), detiene el servicio con drenaje, aparta el de ahora como .anterior,
// pone el nuevo y arranca. Si algo falla antes de arrancar, lo apartado vuelve a su sitio y el servicio arranca con él.
func Actualizar(ctx context.Context, bajar func(ctx context.Context, dir string) (string, error)) error {
	r := Rutas()
	if existe(filepath.Join(r.Programa, ".paquete")) {
		return ErrInstaladoPorPaquete
	}
	if !existe(filepath.Join(r.Programa, programas[0])) {
		return errors.New(idioma.T("el servicio del sistema no está instalado (tramtai servidor instalar)"))
	}
	dir, err := os.MkdirTemp("", "tramtai-actualizacion") // actualizacion.PrefijoCarpeta: LimpiarDescargas la borra si queda
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	paquete, err := bajar(ctx, dir)
	if err == nil {
		err = extraerPrograma(paquete, r.Programa)
	}
	if err == nil {
		err = detenerServicio()
	}
	if err == nil {
		if err = cambiarPrograma(r.Programa); err != nil {
			_ = arrancarServicio() // con el programa de antes, que volvió a su sitio
			return err
		}
		return arrancarServicio()
	}
	for _, n := range programas {
		_ = os.Remove(filepath.Join(r.Programa, n+".nuevo"))
	}
	return err
}

// extraerPrograma saca del paquete (el .tar.gz de Linux, con todo en la carpeta tramtai, o el .zip portable de Windows,
// con los nombres quizá con \) el programa del servidor, como <nombre>.nuevo en dir. El paquete viene con la firma
// comprobada; si le falta algo, no sirve.
func extraerPrograma(paquete, dir string) error {
	escribir := func(nombre string, contenido io.Reader) error {
		f, err := os.OpenFile(filepath.Join(dir, nombre+".nuevo"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, contenido)
		if cerrar := f.Close(); err == nil {
			err = cerrar
		}
		return err
	}
	faltan := slices.Clone(programas)
	if strings.HasSuffix(paquete, ".zip") {
		z, err := zip.OpenReader(paquete)
		if err != nil {
			return err
		}
		defer z.Close()
		for _, f := range z.File {
			if i := slices.Index(faltan, strings.ReplaceAll(f.Name, `\`, "/")); i >= 0 {
				contenido, err := f.Open()
				if err == nil {
					err = escribir(faltan[i], contenido)
					contenido.Close()
				}
				if err != nil {
					return err
				}
				faltan = slices.Delete(faltan, i, i+1)
			}
		}
	} else {
		archivo, err := os.Open(paquete)
		if err != nil {
			return err
		}
		defer archivo.Close()
		gz, err := gzip.NewReader(archivo)
		if err != nil {
			return err
		}
		t := tar.NewReader(gz)
		for {
			h, err := t.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if i := slices.Index(faltan, strings.TrimPrefix(path.Clean(h.Name), "tramtai/")); i >= 0 && h.Typeflag == tar.TypeReg {
				if err := escribir(faltan[i], t); err != nil {
					return err
				}
				faltan = slices.Delete(faltan, i, i+1)
			}
		}
	}
	if len(faltan) > 0 {
		return errors.New(idioma.Tf("el paquete %s no trae %s", filepath.Base(paquete), strings.Join(faltan, ", ")))
	}
	return nil
}

// cambiarPrograma aparta cada programa de dir como .anterior (el de una actualización anterior se reemplaza) y pone el
// .nuevo en su sitio. Si falla, lo apartado vuelve y los .nuevo se borran.
func cambiarPrograma(dir string) error {
	var apartados []string
	var err error
	for _, n := range programas {
		actual := filepath.Join(dir, n)
		if err = os.Rename(actual, actual+".anterior"); err != nil {
			break
		}
		apartados = append(apartados, actual)
		if err = os.Rename(actual+".nuevo", actual); err != nil {
			break
		}
	}
	if err == nil {
		return nil
	}
	for _, actual := range apartados {
		_ = os.Rename(actual+".anterior", actual)
	}
	for _, n := range programas {
		_ = os.Remove(filepath.Join(dir, n+".nuevo"))
	}
	return err
}

func noDisponible() error {
	return errors.New(idioma.T("el modo servidor está disponible en Linux con systemd y en Windows"))
}

func sinAceptar() error {
	return errors.New(idioma.T("para instalar el servidor hay que aceptar los términos de uso"))
}
