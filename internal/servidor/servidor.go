// Package servidor instala y administra el servicio del sistema (modo servidor): la unidad systemd en Linux y el
// servicio de Windows, con su cuenta sin privilegios, su programa y su carpeta de datos. Lo propio de cada sistema va en
// linux.go, windows.go y otros.go.
package servidor

import (
	"bytes"
	"cmp"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	LetsEncrypt    bool   // Let's Encrypt integrado: escucha en el 80 y el 443
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

func noDisponible() error {
	return errors.New(idioma.T("el modo servidor está disponible en Linux con systemd y en Windows"))
}

func sinAceptar() error {
	return errors.New(idioma.T("para instalar el servidor hay que aceptar los términos de uso"))
}
