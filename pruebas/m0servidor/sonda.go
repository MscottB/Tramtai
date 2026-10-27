//go:build m0servidor

// Package main es la sonda de la M0 del modo servidor (v0.58, anexo A de la spec). Se registra como servicio de
// Windows (cuenta virtual NT SERVICE\TramtaiM0) o como unidad de systemd, y escribe en un JSON dónde cree la cuenta
// del servicio que está su perfil y si el almacén de claves funciona sin sesión. Solo guarda la clave de prueba
// «prueba-m0» y la borra al terminar.
//
// Compilar y registrar (Windows, administrador):
//
//	go build -tags m0servidor -o C:\ProgramData\TramtaiM0\sonda.exe ./pruebas/m0servidor
//	sc.exe create TramtaiM0 binPath= "C:\ProgramData\TramtaiM0\sonda.exe C:\ProgramData\TramtaiM0\sonda.json" obj= "NT SERVICE\TramtaiM0"
//
// En Linux, una unidad con User=, StateDirectory= y ExecStart=<sonda> <ruta del JSON>.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"

	"tramtai/internal/credenciales"
)

const (
	nombreClave = "ANTHROPIC_API_KEY"
	valorClave  = "prueba-m0"
)

// informe es lo que la sonda deja en el JSON. Los errores van como texto vacío cuando no hubo.
type informe struct {
	Sistema       string `json:"sistema"`
	Cuenta        string `json:"cuenta"`
	Home          string `json:"home"`
	UserProfile   string `json:"userprofile"`
	AppData       string `json:"appdata"`
	LocalAppData  string `json:"localappdata"`
	UserHomeDir   string `json:"userHomeDir"`
	UserConfigDir string `json:"userConfigDir"`
	ConfigError   string `json:"userConfigDirError"`
	PerfilExiste  bool   `json:"perfilExiste"`
	AlmacenOK     bool   `json:"almacenDisponible"`
	Guardar       string `json:"guardar"`
	Leer          string `json:"leer"`
	LeerCoincide  bool   `json:"leerCoincide"`
	Borrar        string `json:"borrar"`
	Systemd       string `json:"systemdCreds,omitempty"`
}

func texto(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// modo (segundo argumento): vacío = guardar, leer y borrar; «guardar» = solo guardar; «leer» = leer sin guardar y borrar.
// Los dos últimos, en dos arranques seguidos del servicio, prueban que la clave sobrevive a un reinicio.
func medir(modo string) informe {
	r := informe{Sistema: runtime.GOOS + "/" + runtime.GOARCH, Home: os.Getenv("HOME"), UserProfile: os.Getenv("USERPROFILE"),
		AppData: os.Getenv("APPDATA"), LocalAppData: os.Getenv("LOCALAPPDATA")}
	if u, err := user.Current(); err == nil {
		r.Cuenta = u.Username + " (" + u.Uid + ")"
	}
	r.UserHomeDir, _ = os.UserHomeDir()
	var err error
	r.UserConfigDir, err = os.UserConfigDir()
	r.ConfigError = texto(err)
	if r.UserHomeDir != "" {
		_, err = os.Stat(r.UserHomeDir)
		r.PerfilExiste = err == nil
	}
	r.AlmacenOK = credenciales.Disponible()
	if modo != "leer" {
		r.Guardar = texto(credenciales.Guardar(nombreClave, valorClave))
	}
	if modo != "guardar" {
		valor, err := credenciales.Leer(nombreClave)
		r.Leer, r.LeerCoincide = texto(err), valor == valorClave
		r.Borrar = texto(credenciales.Borrar(nombreClave))
	}
	if runtime.GOOS == "linux" {
		salida, err := exec.Command("systemd-creds", "--version").CombinedOutput()
		r.Systemd = strings.TrimSpace(string(salida))
		if err != nil {
			r.Systemd += " [" + err.Error() + "]"
		}
	}
	return r
}

// escribir mide y deja el JSON en la ruta. Guardar toca el almacén real de quien ejecuta: por eso main no lo lanza
// fuera de un servicio.
func escribir(ruta, modo string) error {
	datos, _ := json.MarshalIndent(medir(modo), "", "  ")
	return os.WriteFile(ruta, datos, 0o644)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "uso: sonda <ruta del JSON> [guardar|leer]")
		os.Exit(2)
	}
	ruta, modo := os.Args[1], ""
	if len(os.Args) > 2 {
		modo = os.Args[2]
	}
	if enServicio() {
		if err := ejecutarServicio(ruta, modo); err != nil {
			os.Exit(1)
		}
		return
	}
	// Fuera de un servicio (a mano) sobrescribiría la clave real del usuario que la lanza. Ojo: INVOCATION_ID también
	// existe en las unidades de usuario (systemd --user), donde la sonda sí tocaría el almacén real; usa una unidad del
	// sistema con User=.
	if os.Getenv("INVOCATION_ID") == "" && os.Getenv("M0_FORZAR") != "1" {
		fmt.Fprintln(os.Stderr, "la sonda toca el almacén de claves de quien la ejecuta: lánzala como servicio o con M0_FORZAR=1")
		os.Exit(2)
	}
	if err := escribir(ruta, modo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
