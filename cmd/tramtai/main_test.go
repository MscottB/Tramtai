package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tramtai/internal/carpetas"
)

// TestMain aísla todo el paquete del equipo donde corre: la CLI encuentra el servicio por el descriptor y el token de su
// carpeta de datos, así que mientras esa carpeta, la raíz del usuario, el HOME y la raíz del servidor sean temporales, ninguna
// prueba llega a un Tramtai instalado ni a su puerto. Las que necesitan un servicio arrancan el suyo (servicioFalso,
// servicioEnMarcha) y apuntan dirDatos a su carpeta.
func TestMain(m *testing.M) { os.Exit(correrAislado(m)) }

func correrAislado(m *testing.M) int {
	raiz, err := os.MkdirTemp("", "tramtai-cmd-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo crear la carpeta temporal de las pruebas:", err)
		return 1
	}
	defer os.RemoveAll(raiz)
	casa := filepath.Join(raiz, "casa")
	datos := filepath.Join(raiz, "datos")
	if err := os.MkdirAll(datos, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo crear la carpeta de datos de las pruebas:", err)
		return 1
	}
	// Lo que dejan las variables del equipo (TRAMTAI_SERVIDOR, claves, TOTP…) no entra; los marcadores de los procesos hijo
	// de las pruebas (TRAMTAI_PRUEBA_*) sí, porque la prueba que relanza este binario los necesita.
	for _, e := range os.Environ() {
		if nombre, _, _ := strings.Cut(e, "="); strings.HasPrefix(nombre, "TRAMTAI_") && !strings.HasPrefix(nombre, "TRAMTAI_PRUEBA_") {
			os.Unsetenv(nombre)
		}
	}
	for nombre, valor := range map[string]string{
		"HOME": casa, "USERPROFILE": casa,
		"APPDATA": filepath.Join(casa, "AppData", "Roaming"), "LOCALAPPDATA": filepath.Join(casa, "AppData", "Local"),
		"XDG_CONFIG_HOME": filepath.Join(casa, ".config"), "XDG_DATA_HOME": filepath.Join(casa, ".local", "share"),
		"XDG_CACHE_HOME":        filepath.Join(casa, ".cache"),
		"ProgramData":           filepath.Join(raiz, "ProgramData"),
		"TRAMTAI_CONFIG_GLOBAL": filepath.Join(raiz, "config.yaml"),
	} {
		os.Setenv(nombre, valor)
	}
	// Sin descriptor ni token en ella, conectar() no tiene a quién llamar. Las pruebas la sustituyen y la restauran.
	dirDatos = func() (string, error) { return datos, nil }
	// Ninguna prueba lee el almacén de claves del equipo: las que necesitan una clave ponen la suya y la restauran.
	leerClave = func(nombre string) (string, error) {
		return "", errors.New("la prueba leyó el almacén real de claves (" + nombre + "): sustituye leerClave")
	}
	// La raíz se calcula una vez por proceso: se fija aquí, como la del usuario y en la carpeta temporal, para que no la
	// fije la primera prueba que pase por ella (una con TRAMTAI_SERVIDOR=1 la dejaría como la del servidor para todas).
	if _, err := carpetas.RaizUsuario(); err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo calcular la raíz de las pruebas:", err)
		return 1
	}
	return m.Run()
}
