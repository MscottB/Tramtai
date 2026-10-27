// Package carpetas sabe dónde guarda Tramtai lo del usuario (la carpeta del sistema o, en modo portable, junto al
// ejecutable), protege esas carpetas para que solo las use el usuario y compara rutas como lo hace cada sistema.
package carpetas

import (
	"cmp"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// ArchivoPortable, junto al ejecutable, pone a Tramtai en modo portable: todo lo del usuario va a CarpetaPortable, al
// lado, en lugar de a la carpeta de configuración del sistema (%APPDATA% en Windows). Borrarlo con Tramtai cerrado lo
// devuelve al modo normal.
const (
	ArchivoPortable = "portable.txt"
	CarpetaPortable = "datos-tramtai"
)

// carpetaTemporal es la carpeta temporal del sistema; las pruebas la sustituyen para poder usar t.TempDir().
var carpetaTemporal = os.TempDir

// raizDe calcula la raíz de usuario para un ejecutable: sin portable.txt a su lado, <UserConfigDir>/tramtai; con él,
// <carpeta>/datos-tramtai, creada y solo para este usuario. Si la portable no puede usar su carpeta (es de solo lectura,
// o es la temporal donde el Explorador abre un .zip sin descomprimir), da un error y nunca cae en la carpeta del
// sistema: mezclaría los datos de dos copias.
func raizDe(ejecutable string) (raiz string, portable bool, err error) {
	if Servidor() { // el servicio del sistema no mira portable.txt, y su carpeta no se crea aquí: lo hace la instalación
		return RaizServidor(), false, nil
	}
	if real, err := filepath.EvalSymlinks(ejecutable); err == nil {
		ejecutable = real
	}
	carpeta := filepath.Dir(ejecutable)
	temporal := carpetaTemporal()
	if real, err := filepath.EvalSymlinks(temporal); err == nil {
		temporal = real // en Windows, la temporal suele venir con nombres cortos (ADMINI~1)
	}
	enTemporal := Dentro(temporal, carpeta)
	// El Explorador, al abrir un .exe de dentro de un .zip sin descomprimirlo, lo extrae solo a %TEMP%\Temp1_….zip\, a
	// veces sin portable.txt: esa copia no debe ponerse a escribir en ninguna parte.
	if enTemporal && strings.HasSuffix(strings.ToLower(filepath.Base(carpeta)), ".zip") {
		return "", false, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.T(
			"Tramtai se está ejecutando desde dentro del .zip: descomprímelo en una carpeta donde puedas escribir"))
	}
	info, err := os.Stat(filepath.Join(carpeta, ArchivoPortable))
	switch {
	case errors.Is(err, fs.ErrNotExist) || err == nil && !info.Mode().IsRegular():
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", false, err
		}
		return filepath.Join(dir, "tramtai"), false, nil
	case err != nil: // portable.txt existe pero no se puede leer: no se cae en la carpeta del sistema
		return "", true, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
			"no se pudo leer %s (%s): descomprime Tramtai en una carpeta donde puedas escribir", ArchivoPortable, err.Error()))
	}
	raiz = filepath.Join(carpeta, CarpetaPortable)
	if enTemporal {
		return raiz, true, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
			"Tramtai está en una carpeta temporal (%s), que se borrará con sus datos: descomprime Tramtai en una carpeta donde puedas escribir", carpeta))
	}
	if err := os.MkdirAll(raiz, 0o700); err != nil {
		return raiz, true, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
			"no se pudo crear %s (%s): descomprime Tramtai en una carpeta donde puedas escribir", raiz, err.Error()))
	}
	if err := protegerCarpeta(raiz); err != nil {
		return raiz, true, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
			"no se pudieron poner los permisos de %s (%s): descomprime Tramtai en una carpeta tuya", raiz, err.Error()))
	}
	return raiz, true, nil
}

// raizServidor es la raíz de datos del servicio del sistema; la crea y la protege la instalación. Las pruebas la
// sustituyen para no tocar /var/lib ni %ProgramData%.
var raizServidor = func() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(cmp.Or(os.Getenv("ProgramData"), `C:\ProgramData`), "Tramtai")
	}
	return "/var/lib/tramtai"
}

// RaizServidor es la carpeta de datos del servicio del sistema (modo servidor): /var/lib/tramtai en Linux y
// %ProgramData%\Tramtai en Windows.
func RaizServidor() string { return raizServidor() }

// modoServidor lo pone UsarServidor (la opción --servidor); calculadaComo recuerda con qué modo se calculó la raíz.
var (
	modoServidor  atomic.Bool
	calculadaComo atomic.Int32
)

const (
	sinCalcular = iota
	comoUsuario
	comoServidor
)

// Servidor dice si este proceso usa la raíz del servidor: tras UsarServidor o con TRAMTAI_SERVIDOR=1 (lo pone la
// unidad del servicio del sistema).
func Servidor() bool { return modoServidor.Load() || os.Getenv("TRAMTAI_SERVIDOR") == "1" }

// UsarServidor hace que la raíz sea la del servidor. Va antes del primer RaizUsuario: si la raíz ya se calculó para el
// usuario entra en pánico, porque sería un error de programación y dejaría al proceso a medias en cada carpeta.
func UsarServidor() {
	if calculadaComo.Load() == comoUsuario {
		panic("carpetas.UsarServidor se llamó después de calcular la raíz del usuario: debe ir antes que cualquier RaizUsuario o DirDatos")
	}
	modoServidor.Store(true)
}

// Entorno es «servidor», «portable» o «escritorio» según de dónde salen los datos de este proceso.
func Entorno() string {
	switch {
	case Servidor():
		return "servidor"
	case Portable():
		return "portable"
	}
	return "escritorio"
}

// raiz se calcula una vez por proceso: un proceso nunca cambia de carpeta de datos a mitad de su vida.
var raiz = sync.OnceValues(calcularRaiz)

func calcularRaiz() (resultado, error) {
	if Servidor() {
		calculadaComo.Store(comoServidor)
	} else {
		calculadaComo.Store(comoUsuario)
	}
	ejecutable, err := os.Executable()
	if err != nil {
		return resultado{}, err
	}
	r, portable, err := raizDe(ejecutable)
	return resultado{r, portable}, err
}

type resultado struct {
	ruta     string
	portable bool
}

// RaizUsuario es la carpeta de Tramtai del usuario: config.yaml, datos/, escritorio.json y, en la portable, el perfil
// del navegador de la app.
func RaizUsuario() (string, error) {
	r, err := raiz()
	return r.ruta, err
}

// Portable indica si esta copia está en modo portable (portable.txt junto al ejecutable).
func Portable() bool {
	r, _ := raiz()
	return r.portable
}

// DirDatos es la carpeta de datos (descriptor del servicio, tokens, instancias, registros y cargas), creada si falta.
func DirDatos() (string, error) {
	r, err := RaizUsuario()
	if err != nil {
		return "", err
	}
	datos := filepath.Join(r, "datos")
	if Servidor() { // la raíz la crea y la protege la instalación: MkdirAll la crearía con los permisos heredados
		if err := os.Mkdir(datos, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return "", nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf(
				"no se pudo crear %s (%s): la carpeta del servidor la crea la instalación del servidor", datos, err.Error()))
		}
		return datos, nil
	}
	return datos, os.MkdirAll(datos, 0o700)
}
