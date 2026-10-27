//go:build windows

package servidor

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/instalacion"
	"tramtai/internal/terminos"
)

// envoltorio es tramtai-servidor.cmd, junto al programa: el programa del servidor con la raíz de datos del servidor.
// (goto) termina el .cmd antes de lanzar tramtai (la línea ya está leída y expandida): cmd no vuelve a leerlo, así que
// «tramtai-servidor desinstalar», que lo borra, no acaba con «no se encuentra la ruta» ni con código 1; tampoco
// pregunta «¿Desea terminar el trabajo por lotes?» con Ctrl+C. El código de salida es el de tramtai.
const envoltorio = "@(goto) 2>nul & \"%~dp0tramtai.exe\" --servidor %*\r\n"

// sinBloquear: en Windows no hay FIFO en una carpeta (leerConfiguracion).
const sinBloquear = 0

// El nombre del servicio y el PATH del sistema; las pruebas los sustituyen (TramtaiPrueba<pid>, sin tocar el PATH).
var (
	nombreServicio    = "Tramtai"
	editarRutaSistema = instalacion.EditarRutaSistema
)

// cuentaServicio es la cuenta virtual del servicio, NT SERVICE\Tramtai: Windows la crea con el servicio.
func cuentaServicio() string { return `NT SERVICE\` + nombreServicio }

// ajustarDueno no hace nada: en Windows lo que escribe el administrador lo lee el servicio por la DACL heredable.
func ajustarDueno(*os.Root) error { return nil }

// AjustarDueno no hace nada en Windows (ver ajustarDueno).
func AjustarDueno(string) error { return nil }

// EsAdministrador dice si el proceso corre elevado (una consola de administrador).
func EsAdministrador() bool { return windows.GetCurrentProcessToken().IsElevated() }

// abrirServicio conecta con el administrador de servicios y abre el servicio; s es nil si no está instalado.
func abrirServicio() (m *mgr.Mgr, s *mgr.Service, err error) {
	if m, err = mgr.Connect(); err != nil {
		return nil, nil, err
	}
	s, err = m.OpenService(nombreServicio)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return m, nil, nil
	}
	if err != nil {
		m.Disconnect()
		return nil, nil, err
	}
	return m, s, nil
}

// Instalar copia el programa, crea el servicio con su cuenta virtual, prepara la carpeta de datos con su DACL, añade
// el programa al PATH del sistema y arranca el servicio. Repetirla es seguro: con el mismo programa no cambia nada; con
// otro, detiene el servicio (con drenaje), lo sustituye y lo vuelve a arrancar.
func Instalar(o Opciones) error {
	if !o.AceptoTerminos {
		return sinAceptar()
	}
	r := Rutas()
	m, s, err := abrirServicio()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s != nil {
		defer s.Close()
	}
	// La cuenta del servicio solo lee el programa: Program Files deja leer a los usuarios y escribir a los administradores.
	if err := os.MkdirAll(r.Programa, 0o755); err != nil {
		return err
	}
	programa := map[string][]byte{}
	distinto := false
	for _, nombre := range []string{"tramtai.exe", "uv.exe"} {
		contenido, err := os.ReadFile(filepath.Join(o.Origen, nombre))
		if err != nil {
			return fmt.Errorf(idioma.T("falta %s junto a este tramtai: instala el servidor desde el instalador o el .zip de Windows (%v)"), nombre, err)
		}
		actual, err := os.ReadFile(filepath.Join(r.Programa, nombre))
		distinto = distinto || err != nil || !bytes.Equal(actual, contenido)
		programa[nombre] = contenido
	}
	if distinto && s != nil { // el programa en marcha no se puede sustituir
		if err := detener(s); err != nil {
			return err
		}
	}
	for nombre, contenido := range programa {
		if _, err := escribirSiCambia(filepath.Join(r.Programa, nombre), contenido, 0o755); err != nil {
			return err
		}
	}
	if _, err := escribirSiCambia(r.Envoltorio, []byte(envoltorio), 0o755); err != nil {
		return err
	}
	if s == nil {
		s, err = m.CreateService(nombreServicio, filepath.Join(r.Programa, "tramtai.exe"), mgr.Config{DisplayName: "Tramtai (servidor)",
			StartType: mgr.StartAutomatic, DelayedAutoStart: true, ServiceStartName: cuentaServicio()},
			"--servidor", "servicio", "iniciar", "--registro", filepath.Join(r.Datos, "datos", "servicio.log"))
		if err != nil {
			return err
		}
		defer s.Close()
	}
	// Tres reinicios a los 10 s, también si sale con error y no solo si se cae (como Restart=on-failure).
	reinicio := mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: 10 * time.Second}
	if err := s.SetRecoveryActions([]mgr.RecoveryAction{reinicio, reinicio, reinicio}, 24*60*60); err != nil {
		return err
	}
	if err := s.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		return err
	}
	// Después de crear el servicio: antes, NT SERVICE\Tramtai no existe para la DACL.
	if err := os.MkdirAll(r.Datos, 0o700); err != nil {
		return err
	}
	if err := carpetas.ProtegerServidor(r.Datos, cuentaServicio()); err != nil {
		return err
	}
	if err := prepararDatos(r.Datos, cmp.Or(o.Como, terminos.ComoServidorInstalar), o.Conversor); err != nil {
		return err
	}
	if err := editarRutaSistema(r.Programa, true); err != nil {
		return err
	}
	if err := s.Start(); err != nil && !errors.Is(err, windows.ERROR_SERVICE_ALREADY_RUNNING) {
		return err
	}
	return nil
}

// detener para el servicio y espera a que termine: drena como «tramtai servicio detener», hasta el plazo de drenaje
// de la configuración más 30 s (como TimeoutStopSec en Linux). El servicio informa que se detuvo antes de salir: espera
// también a que termine su proceso, que si no retiene el programa y no se puede sustituir ni borrar.
func detener(s *mgr.Service) error {
	antes, err := s.Query()
	if err != nil {
		return err
	}
	_, err = s.Control(svc.Stop)
	if err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) && !errors.Is(err, windows.ERROR_SERVICE_CANNOT_ACCEPT_CTRL) {
		return err
	}
	plazo := plazoParada()
	for limite := time.Now().Add(plazo); ; time.Sleep(300 * time.Millisecond) {
		e, err := s.Query()
		if err == nil && e.State == svc.Stopped && antes.ProcessId != 0 {
			if h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, antes.ProcessId); err == nil {
				_, _ = windows.WaitForSingleObject(h, 10_000)
				windows.CloseHandle(h)
			}
		}
		if err != nil || e.State == svc.Stopped {
			return err
		}
		if time.Now().After(limite) {
			return errors.New(idioma.Tf("el servicio %s no se detuvo en %s", nombreServicio, plazo.String()))
		}
	}
}

func plazoParada() time.Duration {
	plazo := configuracion.Resolver().Entero("servicio.plazo_drenaje_segundos")
	if r, err := os.OpenRoot(Rutas().Datos); err == nil {
		if c, err := leerConfiguracion(r); err == nil {
			plazo = c.Entero("servicio.plazo_drenaje_segundos")
		}
		r.Close()
	}
	return time.Duration(plazo+30) * time.Second
}

// Desinstalar detiene el servicio (con drenaje), lo borra y quita el programa y su entrada del PATH del sistema. Con
// borrarDatos borra también la carpeta de datos y el perfil de la cuenta virtual, que sobrevive al servicio y guarda
// las sesiones de los agentes; si Windows aún lo tiene cargado, devuelve un Aviso con la orden para quitarlo después.
func Desinstalar(borrarDatos bool) error {
	r := Rutas()
	m, s, err := abrirServicio()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s != nil {
		err := detener(s)
		if err == nil {
			err = s.Delete()
		}
		s.Close()
		if err != nil {
			return err
		}
	}
	if err := editarRutaSistema(r.Programa, false); err != nil {
		return err
	}
	if err := quitarPrograma(r.Programa); err != nil {
		return err
	}
	if !borrarDatos {
		return nil
	}
	if err := os.RemoveAll(r.Datos); err != nil {
		return err
	}
	return borrarPerfil(sidServicio(nombreServicio))
}

// quitarPrograma borra la carpeta del programa. Si este proceso es el tramtai de esa carpeta (tramtai-servidor
// desinstalar), Windows no deja borrarlo en marcha pero sí moverlo: se aparta a la temporal y se borra al reiniciar.
func quitarPrograma(dir string) error {
	if ejecutable, err := os.Executable(); err == nil && carpetas.Dentro(dir, ejecutable) {
		apartado := filepath.Join(os.TempDir(), "tramtai-desinstalado-"+rand.Text()+".exe")
		if err := os.Rename(ejecutable, apartado); err != nil {
			return err
		}
		if p, err := windows.UTF16PtrFromString(apartado); err == nil {
			_ = windows.MoveFileEx(p, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
		}
	}
	return os.RemoveAll(dir)
}

// sidServicio es el SID de NT SERVICE\<nombre>: S-1-5-80 y el SHA-1 del nombre en mayúsculas (UTF-16LE) en cinco
// partes. Se calcula porque sin el servicio la cuenta ya no se resuelve, y el perfil sigue ahí.
func sidServicio(nombre string) string {
	var b []byte
	for _, c := range utf16.Encode([]rune(strings.ToUpper(nombre))) {
		b = binary.LittleEndian.AppendUint16(b, c)
	}
	h := sha1.Sum(b)
	sid := "S-1-5-80"
	for i := 0; i < len(h); i += 4 {
		sid += fmt.Sprintf("-%d", binary.LittleEndian.Uint32(h[i:]))
	}
	return sid
}

var borrarPerfilW = windows.NewLazySystemDLL("userenv.dll").NewProc("DeleteProfileW")

// borrarPerfil borra el perfil de la cuenta del servicio (C:\Windows\ServiceProfiles\<servicio>). Windows tarda en
// descargarlo tras detener el servicio: lo intenta unos segundos y, si sigue cargado, devuelve un Aviso.
func borrarPerfil(sid string) error {
	p, err := windows.UTF16PtrFromString(sid)
	if err != nil {
		return err
	}
	for intento := 0; ; intento++ {
		ok, _, e := borrarPerfilW.Call(uintptr(unsafe.Pointer(p)), 0, 0)
		if ok != 0 || errors.Is(e, windows.ERROR_FILE_NOT_FOUND) || errors.Is(e, windows.ERROR_PATH_NOT_FOUND) {
			return nil
		}
		if intento == 10 {
			perfil := filepath.Join(cmp.Or(os.Getenv("SystemRoot"), `C:\Windows`), "ServiceProfiles", nombreServicio)
			orden := fmt.Sprintf(`Get-CimInstance Win32_UserProfile -Filter "SID='%s'" | Remove-CimInstance`, sid)
			return Aviso(idioma.Tf("Windows aún tiene cargado el perfil de la cuenta del servicio (%s), con las sesiones de los agentes: se podrá quitar tras reiniciar, en PowerShell como administrador: %s (%v)", perfil, orden, e))
		}
		time.Sleep(time.Second)
	}
}

// estados son los nombres del sistema (los de sc query).
var estados = map[svc.State]string{svc.Stopped: "STOPPED", svc.StartPending: "START_PENDING", svc.StopPending: "STOP_PENDING",
	svc.Running: "RUNNING", svc.ContinuePending: "CONTINUE_PENDING", svc.PausePending: "PAUSE_PENDING", svc.Paused: "PAUSED"}

// Estado dice si el servicio está instalado y en qué estado lo tiene el sistema.
func Estado(_ context.Context) (EstadoSistema, error) {
	m, s, err := abrirServicio()
	if err != nil {
		return EstadoSistema{}, err
	}
	defer m.Disconnect()
	if s == nil {
		return EstadoSistema{}, nil
	}
	defer s.Close()
	e, err := s.Query()
	if err != nil {
		return EstadoSistema{Instalado: true}, err
	}
	return EstadoSistema{Instalado: true, Activo: estados[e.State]}, nil
}

// Reiniciar detiene el servicio (deja terminar las peticiones en curso) y lo vuelve a arrancar.
func Reiniciar() error {
	m, s, err := abrirServicio()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	if s == nil {
		return errors.New(idioma.T("el servicio del sistema no está instalado (tramtai servidor instalar)"))
	}
	defer s.Close()
	if err := detener(s); err != nil {
		return err
	}
	return s.Start()
}
