//go:build linux

package servidor

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"tramtai/internal/configuracion"
	"tramtai/internal/credenciales"
	"tramtai/internal/idioma"
	"tramtai/internal/terminos"
)

// envoltorio es tramtai-servidor: el programa del servidor con la raíz de datos del servidor.
const envoltorio = "#!/bin/sh\nexec /opt/tramtai/tramtai --servidor \"$@\"\n"

// sinBloquear abre config.yaml sin quedarse colgado si es un FIFO (leerConfiguracion).
const sinBloquear = syscall.O_NONBLOCK

// programas es lo que hay en la carpeta del programa; Actualizar lo cambia.
var programas = []string{"tramtai", "uv"}

// detenerServicio (systemctl stop deja terminar las peticiones en curso) y arrancarServicio, para Actualizar; las
// pruebas comunes los sustituyen.
var (
	detenerServicio  = func() error { return orden("systemctl", "stop", "tramtai") }
	arrancarServicio = func() error { return orden("systemctl", "start", "tramtai") }
)

// conSystemd dice si el sistema arrancó con systemd (como sd_booted); las pruebas lo sustituyen.
var conSystemd = func() bool {
	_, err := os.Stat("/run/systemd/system")
	return err == nil
}

// Cambiar el dueño a la cuenta del servicio; las pruebas lo sustituyen (no hay cuenta tramtai ni root). lchown no sigue
// enlaces y no sale de la raíz.
var (
	idsServicio = func() (uid, gid int, err error) { return idsDe(Cuenta) }
	lchown      = func(r *os.Root, nombre string, uid, gid int) error { return r.Lchown(nombre, uid, gid) }
)

// idsDe son el uid y el gid (el del grupo principal) de una cuenta.
func idsDe(cuenta string) (uid, gid int, err error) {
	u, err := user.Lookup(cuenta)
	if err != nil {
		return 0, 0, err
	}
	if uid, err = strconv.Atoi(u.Uid); err != nil {
		return 0, 0, err
	}
	gid, err = strconv.Atoi(u.Gid)
	return uid, gid, err
}

// AjustarDueno deja toda la raíz del servidor a nombre de la cuenta del servicio: lo que la CLI escribe como root
// (configuración, términos, administrador) el servicio, que corre como tramtai, no podría leerlo. La raíz es de esa
// cuenta, que podría dejar enlaces: el recorrido va por un os.Root, no sigue enlaces y no sale de ella. Sin la raíz no
// hace nada, y tampoco sin la cuenta (aún no se ha instalado).
func AjustarDueno(raiz string) error {
	if _, _, err := idsServicio(); errors.As(err, new(user.UnknownUserError)) {
		return nil
	}
	r, err := os.OpenRoot(raiz)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer r.Close()
	return ajustarDueno(r)
}

func ajustarDueno(r *os.Root) error {
	uid, gid, err := idsServicio()
	if err != nil {
		return err
	}
	return fs.WalkDir(r.FS(), ".", func(nombre string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			var info fs.FileInfo
			// Un enlace duro puede ser un archivo de fuera (/etc/shadow, con fs.protected_hardlinks=0): no se regala.
			if info, err = r.Lstat(nombre); err == nil && info.Sys().(*syscall.Stat_t).Nlink > 1 {
				return nil
			}
		}
		if err == nil {
			err = lchown(r, nombre, uid, gid)
		}
		if errors.Is(err, fs.ErrNotExist) { // el servicio, en marcha, renombró o borró un temporal: se sigue con el resto
			return nil
		}
		return err
	})
}

// EsAdministrador dice si el proceso corre como root.
func EsAdministrador() bool { return os.Geteuid() == 0 }

// orden lanza una orden del sistema; si falla, el error lleva lo que dijo.
func orden(nombre string, args ...string) error {
	if salida, err := ejecutar(nombre, args...); err != nil {
		return fmt.Errorf(idioma.T("falló «%s» (%v): %s"), strings.Join(append([]string{nombre}, args...), " "), err, strings.TrimSpace(string(salida)))
	}
	return nil
}

// Instalar crea la cuenta del servicio, copia el programa, prepara la carpeta de datos, escribe la unidad y la habilita
// y arranca. Repetirla es seguro: con la misma versión no cambia nada; con otra, actualiza y reinicia. Con el programa
// del paquete .deb/.rpm no copia nada ni escribe la unidad o el envoltorio, que son del paquete: lo que la unidad toma
// de la configuración va en un drop-in.
func Instalar(o Opciones) error {
	if !o.AceptoTerminos {
		return sinAceptar()
	}
	if !conSystemd() {
		return noDisponible()
	}
	r := Rutas()
	if _, err := ejecutar("id", Cuenta); err != nil {
		// --user-group: el grupo tramtai también donde USERGROUPS_ENAB es no.
		if err := orden("useradd", "--system", "--user-group", "--home", r.Datos, "--shell", "/usr/sbin/nologin", Cuenta); err != nil {
			return err
		}
	}
	antes := instalado(r)
	cambiado, reiniciar := false, false
	var avisos []string
	if !delPaquete(r) {
		// La cuenta del servicio solo lee el programa: es de root.
		if err := os.MkdirAll(r.Programa, 0o755); err != nil {
			return err
		}
		for _, nombre := range []string{"tramtai", "uv"} {
			contenido, err := os.ReadFile(filepath.Join(o.Origen, nombre))
			if err != nil {
				return fmt.Errorf(idioma.T("falta %s junto a este tramtai: instala el servidor desde el .tar.gz de Linux (%v)"), nombre, err)
			}
			c, err := escribirSiCambia(filepath.Join(r.Programa, nombre), contenido, 0o755)
			if err != nil {
				return err
			}
			cambiado = cambiado || c
		}
		if _, err := escribirSiCambia(r.Envoltorio, []byte(envoltorio), 0o755); err != nil {
			return err
		}
	}
	if err := prepararDatos(r.Datos, cmp.Or(o.Como, terminos.ComoServidorInstalar), o.Conversor); err != nil {
		return err
	}
	if delPaquete(r) { // tras lo que puede fallar: la unidad vieja no se deshabilita si no va a quedar la del paquete
		var err error
		if reiniciar, avisos, err = limpiarTarGz(r); err != nil {
			return err
		}
		cambiado = cambiado || reiniciar
	}
	nueva, err := actualizarUnidad(r, UnidadOpciones{LetsEncrypt: o.LetsEncrypt, PuertoBajo: o.PuertoBajo})
	if err != nil {
		return err
	}
	cambiado = cambiado || nueva
	if cambiado {
		if err := orden("systemctl", "daemon-reload"); err != nil {
			return err
		}
	}
	if err := orden("systemctl", "enable", "--now", "tramtai"); err != nil {
		return err
	}
	if cambiado && antes || reiniciar { // ya estaba en marcha con el programa o la unidad de antes
		if err := orden("systemctl", "restart", "tramtai"); err != nil {
			return err
		}
	}
	if len(avisos) > 0 {
		return Aviso(strings.Join(avisos, "; "))
	}
	return nil
}

// restosTarGz son la unidad y el envoltorio que deja instalar desde el .tar.gz: con el paquete, Rutas() da los suyos.
// Las pruebas los sustituyen.
var restosTarGz = Sitios{Unidad: "/etc/systemd/system/tramtai.service", Envoltorio: "/usr/local/bin/tramtai-servidor"}

// limpiarTarGz quita, al instalar con el paquete, lo que dejó una instalación anterior desde el .tar.gz (R34): su
// unidad en /etc, que taparía la del paquete (antes la deshabilita, para que no quede su enlace en
// multi-user.target.wants, y pide reiniciar con la del paquete), su envoltorio y los .anterior de actualizar. Solo lo
// que es de Tramtai: una unidad en /etc con otra cabecera, o otro envoltorio, no se toca. Devuelve los avisos.
func limpiarTarGz(r Sitios) (reiniciar bool, avisos []string, err error) {
	var quitados []string
	unidad, err := os.ReadFile(restosTarGz.Unidad)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return false, nil, err
	case bytes.HasPrefix(unidad, []byte(cabeceraUnidad)) || bytes.HasPrefix(unidad, []byte(cabeceraUnidadAntigua)):
		if err := orden("systemctl", "disable", "tramtai"); err != nil {
			return false, nil, err
		}
		if err := os.Remove(restosTarGz.Unidad); err != nil {
			return false, nil, err
		}
		quitados, reiniciar = append(quitados, restosTarGz.Unidad), true
	default:
		avisos = append(avisos, idioma.Tf("%s no la generó Tramtai y tapa la unidad del paquete: no se toca; quítala si sobra", restosTarGz.Unidad))
	}
	if b, err := os.ReadFile(restosTarGz.Envoltorio); err == nil && string(b) == envoltorio {
		if err := os.Remove(restosTarGz.Envoltorio); err != nil {
			return false, nil, err
		}
		quitados = append(quitados, restosTarGz.Envoltorio)
	}
	for _, n := range programas {
		anterior := filepath.Join(r.Programa, n+".anterior")
		if err := os.Remove(anterior); err == nil {
			quitados = append(quitados, anterior)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return false, nil, err
		}
	}
	if len(quitados) > 0 {
		avisos = append([]string{idioma.Tf("quitado lo que dejó la instalación desde el .tar.gz: %s", strings.Join(quitados, ", "))}, avisos...)
	}
	return reiniciar, avisos, nil
}

// instalado dice si el servicio está instalado: con su unidad y, si es la del paquete, con el drop-in de Instalar (el
// paquete no habilita nada hasta que se aceptan los términos con «tramtai-servidor instalar»).
func instalado(r Sitios) bool { return existe(r.Unidad) && (!delPaquete(r) || existe(r.UnidadConf)) }

// actualizarUnidad escribe la unidad con la configuración actual del servidor, si cambió, y dice si la cambió; con la
// del paquete, que es de apt o dnf (y otra en /etc la taparía), escribe su drop-in. extra suma lo que pida quien llama;
// Let's Encrypt (servicio.tls.modo) y el puerto bajo se calculan aquí.
func actualizarUnidad(s Sitios, extra UnidadOpciones) (bool, error) {
	r, err := os.OpenRoot(s.Datos)
	if err != nil {
		return false, err
	}
	defer r.Close()
	c, err := leerConfiguracion(r)
	if err != nil {
		return false, err
	}
	o := UnidadOpciones{PlazoDrenaje: c.Entero("servicio.plazo_drenaje_segundos"), LetsEncrypt: extra.LetsEncrypt || c.Texto("servicio.tls.modo") == "letsencrypt",
		PuertoBajo: extra.PuertoBajo || c.Entero("servicio.puerto") < 1024}
	if delPaquete(s) {
		return escribirSiCambia(s.UnidadConf, []byte(ajustesPaquete(o)), 0o644)
	}
	// Sin el paquete, un drop-in suyo que quedara cambiaría el plazo y las capacidades de esta unidad.
	quitado := true
	if err := os.Remove(s.UnidadConf); errors.Is(err, fs.ErrNotExist) {
		quitado = false
	} else if err != nil {
		return false, err
	}
	cambiada, err := escribirSiCambia(s.Unidad, []byte(Unidad(o)), 0o644)
	return cambiada || quitado, err
}

// Desinstalar detiene el servicio (con drenaje) y quita la unidad, el programa y el envoltorio. Con borrarDatos borra
// también la carpeta de datos y la cuenta. Con el paquete, el programa, la unidad y el envoltorio se quedan (los quitan
// apt o dnf) y devuelve el aviso ErrQuitarPaquete.
func Desinstalar(borrarDatos bool) error {
	r := Rutas()
	paquete := delPaquete(r)
	if instalado(r) {
		if err := orden("systemctl", "disable", "--now", "tramtai"); err != nil {
			return err
		}
		quitar := []string{r.UnidadConf}
		if !paquete {
			quitar = append(quitar, r.Unidad)
		}
		for _, ruta := range quitar {
			if err := os.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
		_ = os.Remove(filepath.Dir(r.UnidadConf)) // solo si quedó vacía (sin claves.conf)
		if err := orden("systemctl", "daemon-reload"); err != nil {
			return err
		}
	}
	if !paquete {
		if err := os.Remove(r.Envoltorio); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.RemoveAll(r.Programa); err != nil {
			return err
		}
	}
	if err := borrarDatosYCuenta(r, borrarDatos); err != nil {
		return err
	}
	if paquete {
		return ErrQuitarPaquete
	}
	return nil
}

// borrarDatosYCuenta, con borrar, borra la carpeta de datos, las claves y la cuenta del servicio.
func borrarDatosYCuenta(r Sitios, borrar bool) error {
	if !borrar {
		return nil
	}
	if err := os.RemoveAll(r.Datos); err != nil {
		return err
	}
	cifradas, _ := filepath.Glob(filepath.Join(r.Cifradas, "tramtai.*"))
	for _, ruta := range append(cifradas, r.ClavesConf, r.ClavesEnv) {
		if err := os.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	for _, d := range []string{filepath.Dir(r.ClavesConf), filepath.Dir(r.ClavesEnv)} {
		_ = os.Remove(d) // solo si quedaron vacías
	}
	if _, err := ejecutar("id", Cuenta); err == nil {
		return orden("userdel", Cuenta)
	}
	return nil
}

// Estado dice si el servicio está instalado y lo que dice systemctl is-active (que sale con error si no está activa).
func Estado(_ context.Context) (EstadoSistema, error) {
	r := Rutas()
	if !instalado(r) {
		return EstadoSistema{}, nil
	}
	salida, err := ejecutar("systemctl", "is-active", "tramtai")
	activo := strings.TrimSpace(string(salida))
	if activo == "" && err != nil {
		return EstadoSistema{Instalado: true, ClavesEnClaro: existe(r.ClavesEnv)}, err
	}
	return EstadoSistema{Instalado: true, Activo: activo, ClavesEnClaro: existe(r.ClavesEnv)}, nil
}

// cabeceraClaves encabeza claves.conf, que se reescribe entero con cada clave que se guarda o se borra.
const cabeceraClaves = "# Generado por tramtai-servidor claves guardar/borrar: no lo edites a mano.\n"

// GuardarClave guarda una clave del servidor para que systemd se la entregue al servicio al arrancar: la cifra con
// systemd-creds en credstore.encrypted (el valor por stdin), la carga en claves.conf con LoadCredentialEncrypted= y
// reinicia el servicio, que deja terminar las peticiones en curso. Sin systemd-creds 250 o posterior la escribe en
// claves.env (0600, de root), que claves.conf carga con EnvironmentFile=, y devuelve el aviso ErrClaveEnClaro.
func GuardarClave(nombre, valor string) error {
	if err := credenciales.Comprobar(nombre, valor); err != nil {
		return err
	}
	if !conSystemd() {
		return noDisponible()
	}
	r := Rutas()
	cifrada := filepath.Join(r.Cifradas, "tramtai."+nombre)
	enClaro := valor
	if conSystemdCreds() {
		if err := os.MkdirAll(r.Cifradas, 0o700); err != nil {
			return err
		}
		if salida, err := ejecutarConEntrada([]byte(valor), "systemd-creds", "encrypt", "--name=tramtai."+nombre, "-", cifrada); err != nil {
			return fmt.Errorf(idioma.T("falló «%s» (%v): %s"), "systemd-creds encrypt --name=tramtai."+nombre, err, strings.TrimSpace(string(salida)))
		}
		if err := os.Chmod(cifrada, 0o600); err != nil { // systemd-creds la deja con la umask de root (0644)
			return err
		}
		enClaro = ""
	} else if err := os.Remove(cifrada); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	// La clave queda en un solo sitio: la de antes, en el otro, se quita.
	if err := cambiarClaves(r, nombre, enClaro); err != nil {
		return err
	}
	if enClaro != "" {
		return ErrClaveEnClaro
	}
	return nil
}

// BorrarClave quita una clave del servidor (cifrada o en claro) y reinicia el servicio sin ella. No falla si no estaba.
func BorrarClave(nombre string) error {
	if !credenciales.Admitido(nombre) && !credenciales.AdmitidoInterno(nombre) {
		return errors.New(idioma.T("clave no admitida"))
	}
	r := Rutas()
	if err := os.Remove(filepath.Join(r.Cifradas, "tramtai."+nombre)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return cambiarClaves(r, nombre, "")
}

// GuardarTOTP guarda el secreto TOTP del administrador como una clave más (TRAMTAI_TOTP): el servicio no puede escribir
// en systemd-creds, así que solo lo hace la CLI como root, y reinicia el servicio.
func GuardarTOTP(secreto string) error { return GuardarClave("TRAMTAI_TOTP", secreto) }

// BorrarTOTP quita el secreto TOTP y reinicia el servicio.
func BorrarTOTP() error { return BorrarClave("TRAMTAI_TOTP") }

// conSystemdCreds dice si hay systemd-creds de la versión 250 o posterior (antes no cifra credenciales).
func conSystemdCreds() bool {
	salida, err := ejecutar("systemd-creds", "--version")
	var version int
	_, errVersion := fmt.Sscanf(string(salida), "systemd %d", &version)
	return err == nil && errVersion == nil && version >= 250
}

// cambiarClaves pone (o quita, con valor vacío) la clave en claves.env, reescribe claves.conf con las claves que hay y
// reinicia el servicio para que las reciba. Sin claves no quedan ni claves.env ni claves.conf.
func cambiarClaves(r Sitios, nombre, valor string) error {
	enClaro := map[string]string{}
	if datos, err := os.ReadFile(r.ClavesEnv); err == nil {
		for linea := range strings.Lines(string(datos)) {
			if n, v, ok := strings.Cut(strings.TrimSpace(linea), "="); ok {
				enClaro[n] = v
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	enClaro[nombre] = valor
	var env, conf strings.Builder
	for _, n := range append(slices.Clone(credenciales.Nombres), credenciales.Internos...) {
		if enClaro[n] != "" {
			env.WriteString(n + "=" + enClaro[n] + "\n")
		}
		if cifrada := filepath.Join(r.Cifradas, "tramtai."+n); existe(cifrada) {
			conf.WriteString("LoadCredentialEncrypted=tramtai." + n + ":" + cifrada + "\n")
		}
	}
	if env.Len() > 0 {
		conf.WriteString("EnvironmentFile=-" + r.ClavesEnv + "\n")
	}
	for _, f := range []struct {
		ruta, contenido string
		permisos        os.FileMode
	}{{r.ClavesEnv, env.String(), 0o600}, {r.ClavesConf, conf.String(), 0o644}} {
		var err error
		if f.contenido == "" {
			if err = os.Remove(f.ruta); errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
			_ = os.Remove(filepath.Dir(f.ruta)) // tramtai.service.d o /etc/tramtai, solo si quedó vacía
		} else {
			if f.ruta == r.ClavesConf {
				f.contenido = cabeceraClaves + "[Service]\n" + f.contenido
			}
			err = configuracion.EscribirAtomico(f.ruta, []byte(f.contenido), f.permisos)
		}
		if err != nil {
			return err
		}
	}
	if !instalado(r) { // aún sin instalar: la recibirá al arrancar
		return nil
	}
	if err := orden("systemctl", "daemon-reload"); err != nil {
		return err
	}
	return orden("systemctl", "restart", "tramtai")
}

// Reiniciar reinicia el servicio con la unidad al día con la configuración (plazo de drenaje, puerto bajo): si cambió,
// la reescribe (o, con la del paquete, su drop-in) y hace daemon-reload antes. El que se detiene deja terminar las
// peticiones en curso. Sin instalar no arranca nada: con el paquete aún no se han aceptado los términos.
func Reiniciar() error {
	r := Rutas()
	if !instalado(r) {
		return sinInstalar()
	}
	if err := AjustarDueno(r.Datos); err != nil { // lo que root haya dejado ahí, legible para el servicio que arranca
		return err
	}
	cambiada, err := actualizarUnidad(r, UnidadOpciones{})
	if err != nil {
		return err
	}
	if cambiada {
		if err := orden("systemctl", "daemon-reload"); err != nil {
			return err
		}
	}
	return orden("systemctl", "restart", "tramtai")
}

// pathServicio es el PATH que systemd da al servicio (la unidad no pone otro): el inicio de sesión busca el CLI donde lo
// buscará el servicio.
const pathServicio = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin"

// buscarPrograma localiza un programa en el PATH; las pruebas lo sustituyen.
var buscarPrograma = exec.LookPath

// OrdenEntrar es el inicio de sesión del CLI de un agente (comando y args de su ficha) como la cuenta del servicio, con
// su carpeta como HOME y un entorno limpio (env -i) que no depende de la política de sudo: el CLI guarda la sesión donde
// la busca el servicio. En Codex con suscripción, el adaptador no pone CODEX_HOME, así que es ~/.codex de esa carpeta.
// Tramtai no lee ni copia lo que guarde el CLI.
func OrdenEntrar(_, comando string, args []string) (*exec.Cmd, error) {
	sudo, err := buscarPrograma("sudo")
	if err != nil {
		return nil, errors.New(idioma.T("agentes entrar necesita sudo para iniciar sesión con la cuenta del servicio: instálalo y vuelve a probar"))
	}
	orden := []string{"-u", Cuenta, "-H", "env", "-i", "HOME=" + Rutas().Datos, "PATH=" + pathServicio}
	for _, nombre := range []string{"TERM", "LANG"} { // el código de dispositivo se lee en la terminal, en su idioma
		if valor := os.Getenv(nombre); valor != "" {
			orden = append(orden, nombre+"="+valor)
		}
	}
	return exec.Command(sudo, append(append(orden, comando), args...)...), nil
}
