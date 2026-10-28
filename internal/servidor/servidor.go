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
// envoltorio tramtai-servidor, la unidad systemd y el drop-in que ajusta la del paquete (10-: antes que el override.conf
// de «systemctl edit», que así manda) y, para las claves (solo Linux),
// la carpeta de las credenciales cifradas, el drop-in que las carga y el archivo de entorno de root sin systemd-creds.
// El tipo no se llama Rutas porque ese nombre es el de la función.
type Sitios struct {
	Programa, Datos, Envoltorio, Unidad, UnidadConf string
	Cifradas, ClavesConf, ClavesEnv                 string
}

// rutas las sustituyen las pruebas para no tocar /opt, /var/lib ni /etc.
var rutas = func() Sitios {
	if runtime.GOOS == "windows" {
		programa := filepath.Join(cmp.Or(os.Getenv("ProgramFiles"), `C:\Program Files`), "Tramtai Servidor")
		return Sitios{Programa: programa, Datos: carpetas.RaizServidor(), Envoltorio: filepath.Join(programa, "tramtai-servidor.cmd")}
	}
	s := Sitios{Programa: "/opt/tramtai", Datos: carpetas.RaizServidor(), Envoltorio: "/usr/local/bin/tramtai-servidor",
		Unidad: "/etc/systemd/system/tramtai.service", UnidadConf: "/etc/systemd/system/tramtai.service.d/10-tramtai.conf",
		Cifradas: "/etc/credstore.encrypted", ClavesConf: "/etc/systemd/system/tramtai.service.d/claves.conf",
		ClavesEnv: "/etc/tramtai/claves.env"}
	if delPaquete(s) { // el paquete pone el envoltorio y la unidad donde van los de los paquetes
		s.Envoltorio, s.Unidad = "/usr/bin/tramtai-servidor", "/usr/lib/systemd/system/tramtai.service"
	}
	return s
}

// delPaquete dice si el programa lo puso el paquete .deb o .rpm tramtai-servidor, que deja .paquete en su carpeta.
func delPaquete(s Sitios) bool { return existe(filepath.Join(s.Programa, ".paquete")) }

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

// ErrQuitarPaquete es el aviso de Desinstalar con el paquete: el servicio queda detenido y deshabilitado, pero el
// programa y la unidad los quitan apt o dnf.
var ErrQuitarPaquete = Aviso(idioma.T("el programa y la unidad son del paquete tramtai-servidor: quítalo con «apt remove tramtai-servidor» o «dnf remove tramtai-servidor»"))

func sinInstalar() error {
	return errors.New(idioma.T("el servicio del sistema no está instalado (tramtai servidor instalar)"))
}

// ejecutableActual es la ruta real de este programa y lanzarInstalado ejecuta el programa instalado con la salida a la
// vista; las pruebas los sustituyen.
var (
	ejecutableActual = func() (string, error) {
		e, err := os.Executable()
		if err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(e)
	}
	lanzarInstalado = func(programa string, salida, errores io.Writer, args ...string) error {
		cmd := exec.Command(programa, args...)
		cmd.Stdout, cmd.Stderr = salida, errores
		if err := cmd.Run(); err != nil {
			return errors.New(idioma.Tf("la actualización con el programa instalado (%s) terminó con error: %v", programa, err))
		}
		return nil
	}
)

// ActualizarConElInstalado deja la actualización al programa instalado si este no lo es (sudo tramtai servidor
// actualizar desde la copia del escritorio, por ejemplo): así se compara con la versión del servidor y no con la de
// esta copia. Si delega, ejecuta «<programa> --servidor servidor actualizar» con su salida a la vista y devuelve true:
// quien llama no hace nada más. Sin servidor instalado, devuelve true y el error de siempre.
func ActualizarConElInstalado(salida, errores io.Writer) (bool, error) {
	instalado := filepath.Join(Rutas().Programa, programas[0])
	info, err := os.Stat(instalado)
	if err != nil {
		return true, sinInstalar()
	}
	actual, err := ejecutableActual()
	if err != nil {
		return true, err
	}
	a, err := os.Stat(actual)
	if err != nil { // sin saber qué programa es este, no se delega: podría ser el instalado delegando en sí mismo
		return true, err
	}
	if os.SameFile(a, info) {
		return false, nil
	}
	return true, lanzarInstalado(instalado, salida, errores, "--servidor", "servidor", "actualizar")
}

// Actualizar pone la última versión publicada en la carpeta del programa: bajar deja en una carpeta temporal el paquete
// del servidor de este sistema con la firma de sus sumas y su suma comprobadas (actualizacion.PaqueteServidor, que pasa
// cmd/tramtai: este paquete no importa el actualizador, porque lo enlaza la app de escritorio por AvisoCruzado). Saca de
// él el programa (como .nuevo, junto al de ahora), detiene el servicio con drenaje, aparta el de ahora como .anterior,
// pone el nuevo y lo arranca el programa nuevo con «servidor reiniciar», que deja la unidad al día. Si algo falla antes
// de que el nuevo se pueda ejecutar, lo apartado vuelve a su sitio y el servicio arranca con él.
func Actualizar(ctx context.Context, bajar func(ctx context.Context, dir string) (string, error)) error {
	r := Rutas()
	if delPaquete(r) {
		return ErrInstaladoPorPaquete
	}
	if !existe(filepath.Join(r.Programa, programas[0])) {
		return sinInstalar()
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
	if err != nil {
		for _, n := range programas {
			_ = os.Remove(filepath.Join(r.Programa, n+".nuevo"))
		}
		return err
	}
	if err := cambiarPrograma(r.Programa); err != nil {
		return errors.Join(err, arrancarServicio()) // con el programa de antes, que volvió a su sitio
	}
	nuevo := filepath.Join(r.Programa, programas[0])
	salida, err := ejecutar(nuevo, "--servidor", "servidor", "reiniciar")
	if err == nil {
		return nil
	}
	if !errors.As(err, new(*exec.ExitError)) { // el nuevo ni siquiera se ejecuta: vuelve el de antes
		return errors.Join(err, devolverPrograma(r.Programa, programas), arrancarServicio())
	}
	return errors.New(idioma.Tf("el programa nuevo ya está en su sitio y el de antes quedó como %s.anterior, pero no arrancó (%s). "+
		"Para volver al de antes, renombra cada .anterior de %s a su nombre y ejecuta tramtai-servidor reiniciar",
		nuevo, cmp.Or(strings.TrimSpace(string(salida)), err.Error()), r.Programa))
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
		// 0755 aunque root tenga otra umask (027, 077) o quedara un .nuevo de antes: la cuenta del servicio lo ejecuta.
		err = f.Chmod(0o755)
		if err == nil {
			_, err = io.Copy(f, contenido)
		}
		if err == nil {
			err = f.Sync()
		}
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
			if i := slices.Index(faltan, path.Clean(strings.ReplaceAll(f.Name, `\`, "/"))); i >= 0 && f.Mode().IsRegular() {
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
	for _, n := range programas {
		actual := filepath.Join(dir, n)
		err := os.Rename(actual, actual+".anterior")
		if err == nil {
			apartados = append(apartados, n)
			err = os.Rename(actual+".nuevo", actual)
		}
		if err != nil {
			return errors.Join(err, devolverPrograma(dir, apartados))
		}
	}
	return nil
}

// devolverPrograma pone en su sitio los .anterior de nombres y borra los .nuevo que queden. Si alguno no vuelve, el
// error lo dice: falta ese programa.
func devolverPrograma(dir string, nombres []string) error {
	var errs []error
	for _, n := range nombres {
		actual := filepath.Join(dir, n)
		if err := os.Rename(actual+".anterior", actual); err != nil {
			errs = append(errs, errors.New(idioma.Tf("no se pudo devolver %s a su sitio: %v", actual, err)))
		}
	}
	for _, n := range programas {
		_ = os.Remove(filepath.Join(dir, n+".nuevo"))
	}
	return errors.Join(errs...)
}

func noDisponible() error {
	return errors.New(idioma.T("el modo servidor está disponible en Linux con systemd y en Windows"))
}

func sinAceptar() error {
	return errors.New(idioma.T("para instalar el servidor hay que aceptar los términos de uso"))
}
