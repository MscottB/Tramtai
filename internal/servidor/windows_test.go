//go:build windows

package servidor

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"tramtai/internal/terminos"
)

// TestMain: el servicio de prueba que instala TestInstalarServicioWindows es este mismo binario de pruebas, que arrancado
// por el SCM hace de servicio mínimo (en marcha hasta que lo detienen) en lugar de pasar las pruebas.
func TestMain(m *testing.M) {
	if es, _ := svc.IsWindowsService(); es {
		_ = svc.Run("", servicioMinimo{})
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type servicioMinimo struct{}

func (servicioMinimo) Execute(_ []string, peticiones <-chan svc.ChangeRequest, estado chan<- svc.Status) (bool, uint32) {
	estado <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for p := range peticiones {
		if p.Cmd == svc.Stop || p.Cmd == svc.Shutdown {
			return false, 0
		}
		estado <- p.CurrentStatus
	}
	return false, 0
}

// TestInstalarServicioWindows instala de verdad, con el SCM, un servicio de prueba (TramtaiPrueba<pid>) con su cuenta
// virtual: la cuenta, el arranque automático retrasado, las acciones de recuperación, la DACL de la carpeta de datos y
// el servicio en marcha. Después lo desinstala, también si la prueba falla. Necesita una consola de administrador.
func TestInstalarServicioWindows(t *testing.T) {
	if !EsAdministrador() {
		t.Skip("necesita una consola de administrador (el SCM y la DACL)")
	}
	nombre := fmt.Sprintf("TramtaiPrueba%d", os.Getpid())
	// En %ProgramData% y no en la temporal del administrador: la cuenta virtual tiene que poder leer el programa.
	base := filepath.Join(os.Getenv("ProgramData"), nombre)
	s := Sitios{Programa: filepath.Join(base, "programa"), Datos: filepath.Join(base, "datos")}
	s.Envoltorio = filepath.Join(s.Programa, "tramtai-servidor.cmd")
	origen := t.TempDir()
	ejecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copiar(t, ejecutable, filepath.Join(origen, "tramtai.exe"))
	if err := os.WriteFile(filepath.Join(origen, "uv.exe"), []byte("uv"), 0o755); err != nil {
		t.Fatal(err)
	}
	var rutasPath []string
	nombre0, rutas0, path0 := nombreServicio, rutas, editarRutaSistema
	nombreServicio, rutas = nombre, func() Sitios { return s }
	editarRutaSistema = func(dir string, agregar bool) error { // el PATH del equipo no se toca en las pruebas
		rutasPath = append(rutasPath, fmt.Sprint(agregar, " ", dir))
		return nil
	}
	t.Cleanup(func() {
		if err := Desinstalar(true); err != nil && !errors.As(err, new(Aviso)) {
			t.Errorf("desinstalar al terminar: %v", err)
		}
		_ = os.RemoveAll(base)
		nombreServicio, rutas, editarRutaSistema = nombre0, rutas0, path0
	})

	if err := Instalar(Opciones{Origen: origen, AceptoTerminos: true, Como: terminos.ComoOpcion}); err != nil {
		t.Fatal(err)
	}
	cuenta := `NT SERVICE\` + nombre
	m, err := mgr.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Disconnect()
	servicio, err := m.OpenService(nombre)
	if err != nil {
		t.Fatal(err)
	}
	defer servicio.Close()
	c, err := servicio.Config()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(c.ServiceStartName, cuenta) || c.StartType != mgr.StartAutomatic || !c.DelayedAutoStart || c.DisplayName != "Tramtai (servidor)" {
		t.Errorf("configuración del servicio: %+v", c)
	}
	registro := filepath.Join(s.Datos, "datos", "servicio.log")
	if !strings.Contains(c.BinaryPathName, filepath.Join(s.Programa, "tramtai.exe")) ||
		!strings.Contains(c.BinaryPathName, "--servidor servicio iniciar --registro "+registro) {
		t.Errorf("orden del servicio: %s", c.BinaryPathName)
	}
	acciones, err := servicio.RecoveryActions()
	if err != nil {
		t.Fatal(err)
	}
	reinicio := mgr.RecoveryAction{Type: mgr.ServiceRestart, Delay: 10 * time.Second}
	if !slices.Equal(acciones, []mgr.RecoveryAction{reinicio, reinicio, reinicio}) {
		t.Errorf("acciones de recuperación: %+v", acciones)
	}
	if siempre, err := servicio.RecoveryActionsOnNonCrashFailures(); err != nil || !siempre {
		t.Errorf("también si sale con error: %v %v", siempre, err)
	}

	sid, _, _, err := windows.LookupSID("", cuenta)
	if err != nil {
		t.Fatal(err)
	}
	if calculado := sidServicio(nombre); calculado != sid.String() {
		t.Errorf("SID calculado %s, el del sistema %s", calculado, sid)
	}
	comprobarDACL(t, s.Datos, sid.String())
	// publico, donde el servicio deja su puerto para el aviso de los dos entornos: los usuarios solo leen.
	if entradas, protegida := entradasDACL(t, filepath.Join(s.Datos, "publico")); !protegida ||
		!slices.Equal(entradas, lecturaPublica(sid.String(), windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE)) {
		t.Errorf("DACL de publico (protegida %v): %q", protegida, entradas)
	}
	for _, f := range []string{"config.yaml", filepath.Join("datos", "terminos.json")} {
		if _, err := os.Stat(filepath.Join(s.Datos, f)); err != nil {
			t.Errorf("falta %s: %v", f, err)
		}
	}
	if env, err := os.ReadFile(s.Envoltorio); err != nil || string(env) != envoltorio {
		t.Errorf("envoltorio: %q %v", env, err)
	}
	if !slices.Equal(rutasPath, []string{"true " + s.Programa}) {
		t.Errorf("PATH del sistema: %q", rutasPath)
	}
	esperarEstado(t, servicio, svc.Running)
	if e, err := Estado(t.Context()); err != nil || !e.Instalado || e.Activo != "RUNNING" {
		t.Errorf("estado en marcha: %+v %v", e, err)
	}

	// Repetirla con el mismo programa lo deja en marcha y devuelve su configuración a la debida si alguien la cambió.
	cambiada := c
	cambiada.DisplayName, cambiada.StartType, cambiada.DelayedAutoStart = "Otro", mgr.StartManual, false
	if err := servicio.UpdateConfig(cambiada); err != nil {
		t.Fatal(err)
	}
	if err := Instalar(Opciones{Origen: origen, AceptoTerminos: true, Como: terminos.ComoOpcion}); err != nil {
		t.Fatalf("instalar otra vez: %v", err)
	}
	esperarEstado(t, servicio, svc.Running)
	if c2, err := servicio.Config(); err != nil || c2.DisplayName != c.DisplayName || c2.StartType != mgr.StartAutomatic ||
		!c2.DelayedAutoStart || c2.BinaryPathName != c.BinaryPathName || !strings.EqualFold(c2.ServiceStartName, cuenta) {
		t.Errorf("configuración tras reinstalar: %+v %v", c2, err)
	}

	servicio.Close()
	if err := Desinstalar(true); err != nil {
		var aviso Aviso
		if !errors.As(err, &aviso) {
			t.Fatal(err)
		}
		t.Logf("aviso del perfil: %v", aviso) // Windows puede tardar en descargarlo
	}
	if _, err := m.OpenService(nombre); err == nil {
		t.Error("el servicio sigue registrado")
	}
	for _, d := range []string{s.Programa, s.Datos} {
		if _, err := os.Stat(d); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s sigue ahí: %v", d, err)
		}
	}
	if !slices.Equal(rutasPath, []string{"true " + s.Programa, "true " + s.Programa, "false " + s.Programa}) {
		t.Errorf("PATH del sistema al desinstalar: %q", rutasPath)
	}
	if e, err := Estado(t.Context()); err != nil || e.Instalado {
		t.Errorf("estado sin instalar: %+v %v", e, err)
	}
}

// TestConAviso: si quitar el programa falla después de un aviso (el borrado al reiniciar sin programar), el fallo
// manda y no se toma por un aviso, pero lleva la ruta del aviso.
func TestConAviso(t *testing.T) {
	fallo := errors.New("acceso denegado")
	err := conAviso(fallo, Aviso(`bórralo a mano: C:\Program Files\.tramtai-desinstalado-X.exe`))
	if !errors.Is(err, fallo) || errors.As(err, new(Aviso)) || !strings.Contains(err.Error(), ".tramtai-desinstalado-X.exe") {
		t.Errorf("fallo con aviso: %v", err)
	}
	if err := conAviso(fallo, nil); err != fallo {
		t.Errorf("sin aviso: %v", err)
	}
}

// comprobarDACL: protegida (sin herencia de ProgramData), dueño Administradores y solo SYSTEM, Administradores y el
// servicio, con control total heredable por carpetas y archivos.
func comprobarDACL(t *testing.T, ruta, sidServicio string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(ruta, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if control, _, err := sd.Control(); err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Errorf("la DACL debe estar protegida: %x %v", control, err)
	}
	admins, _ := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if dueno, _, err := sd.Owner(); err != nil || !dueno.Equals(admins) {
		t.Errorf("dueño: %v %v", dueno, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	var entradas []string
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		entradas = append(entradas, fmt.Sprintf("%s %x %x", sid, ace.Mask, ace.Header.AceFlags))
	}
	const total, oici = 0x1f01ff, windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE
	quiere := []string{fmt.Sprintf("S-1-5-18 %x %x", total, oici), fmt.Sprintf("S-1-5-32-544 %x %x", total, oici),
		fmt.Sprintf("%s %x %x", sidServicio, total, oici)}
	if !slices.Equal(entradas, quiere) {
		t.Errorf("DACL de %s:\n %q\n quiere %q", ruta, entradas, quiere)
	}
}

func esperarEstado(t *testing.T, s *mgr.Service, quiere svc.State) {
	t.Helper()
	for limite := time.Now().Add(30 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		e, err := s.Query()
		if err == nil && e.State == quiere {
			return
		}
		if time.Now().After(limite) {
			t.Fatalf("el servicio no llegó al estado %d: %+v %v", quiere, e, err)
		}
	}
}

func copiar(t *testing.T, origen, destino string) {
	t.Helper()
	o, err := os.Open(origen)
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	d, err := os.Create(destino)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := io.Copy(d, o); err != nil {
		t.Fatal(err)
	}
}

// TestBorrarCredencialesSinSalirDelPerfil: borra Credentials (Local y Roaming) del perfil de la cuenta del servicio, pero
// a través de un os.Root: si la cuenta cambió un tramo (AppData) por una unión a otra carpeta, como la del
// administrador, lo de esa carpeta sigue ahí y queda un Aviso. Sin perfil no hay nada que borrar.
func TestBorrarCredencialesSinSalirDelPerfil(t *testing.T) {
	base := t.TempDir()
	perfil, fuera := filepath.Join(base, "perfil"), filepath.Join(base, "admin", "AppData")
	anterior := perfilServicio
	t.Cleanup(func() { perfilServicio = anterior })
	perfilServicio = func() string { return perfil }
	crear := func(carpeta string) string {
		t.Helper()
		archivo := filepath.Join(carpeta, "Local", "Microsoft", "Credentials", "36858EAF")
		if err := os.MkdirAll(filepath.Dir(archivo), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(archivo, []byte("clave"), 0o600); err != nil {
			t.Fatal(err)
		}
		return archivo
	}

	if err := borrarCredenciales(); err != nil {
		t.Fatalf("sin perfil: %v", err)
	}
	propia := crear(filepath.Join(perfil, "AppData"))
	if err := borrarCredenciales(); err != nil || existe(propia) {
		t.Fatalf("las del perfil se borran: %v, sigue %v", err, existe(propia))
	}

	ajena := crear(fuera)
	if err := os.RemoveAll(filepath.Join(perfil, "AppData")); err != nil {
		t.Fatal(err)
	}
	if salida, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(perfil, "AppData"), fuera).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v %s", err, salida)
	}
	err := borrarCredenciales()
	if !existe(ajena) {
		t.Fatal("se borró lo de fuera del perfil a través de la unión")
	}
	if !errors.As(err, new(Aviso)) {
		t.Errorf("una unión que sale del perfil deja un Aviso: %v", err)
	}
}
