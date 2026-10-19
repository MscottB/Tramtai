package nucleo

import (
	"os"
	"path/filepath"
	"sync"

	"tramtai/internal/idioma"
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
	if real, err := filepath.EvalSymlinks(ejecutable); err == nil {
		ejecutable = real
	}
	carpeta := filepath.Dir(ejecutable)
	if info, err := os.Stat(filepath.Join(carpeta, ArchivoPortable)); err != nil || !info.Mode().IsRegular() {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", false, err
		}
		return filepath.Join(dir, "tramtai"), false, nil
	}
	raiz = filepath.Join(carpeta, CarpetaPortable)
	temporal := carpetaTemporal()
	if real, err := filepath.EvalSymlinks(temporal); err == nil {
		temporal = real // en Windows, la temporal suele venir con nombres cortos (ADMINI~1)
	}
	if dentroDe(temporal, carpeta) {
		return raiz, true, NuevoError(CodigoConfiguracionInvalida, idioma.Tf(
			"Tramtai está en una carpeta temporal (%s), que se borrará con sus datos: descomprime Tramtai en una carpeta donde puedas escribir", carpeta))
	}
	if err := os.MkdirAll(raiz, 0o700); err != nil {
		return raiz, true, NuevoError(CodigoConfiguracionInvalida, idioma.Tf(
			"no se pudo crear %s (%s): descomprime Tramtai en una carpeta donde puedas escribir", raiz, err.Error()))
	}
	if err := protegerCarpeta(raiz); err != nil {
		return raiz, true, NuevoError(CodigoConfiguracionInvalida, idioma.Tf(
			"no se pudieron poner los permisos de %s (%s): descomprime Tramtai en una carpeta tuya", raiz, err.Error()))
	}
	return raiz, true, nil
}

// raiz se calcula una vez por proceso: un proceso nunca cambia de carpeta de datos a mitad de su vida.
var raiz = sync.OnceValues(func() (resultado, error) {
	ejecutable, err := os.Executable()
	if err != nil {
		return resultado{}, err
	}
	r, portable, err := raizDe(ejecutable)
	return resultado{r, portable}, err
})

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
	return datos, os.MkdirAll(datos, 0o700)
}
