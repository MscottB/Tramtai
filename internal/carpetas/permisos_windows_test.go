package carpetas

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// cuentaPrueba hace de NT SERVICE\Tramtai: es una cuenta de servicio que existe en todo Windows.
const cuentaPrueba = `NT SERVICE\TrustedInstaller`

// descriptor resume la seguridad de ruta: el dueño, si la DACL está protegida y sus entradas (SID, máscara, herencia).
func descriptor(t *testing.T, ruta string) (dueno string, protegida bool, entradas []string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(ruta, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	o, _, err := sd.Owner()
	if err != nil {
		t.Fatal(err)
	}
	control, _, _ := sd.Control()
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		entradas = append(entradas, fmt.Sprintf("%s %x %x", (*windows.SID)(unsafePointer(&ace.SidStart)), ace.Mask, ace.Header.AceFlags))
	}
	return o.String(), control&windows.SE_DACL_PROTECTED != 0, entradas
}

func elevado(t *testing.T) {
	t.Helper()
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("necesita una consola de administrador (dueño Administradores)")
	}
}

// La carpeta del servidor nueva nace ya con su descriptor final: dueño Administradores, sin herencia, y solo SYSTEM,
// Administradores y la cuenta del servicio con control total heredable.
func TestProtegerServidorNueva(t *testing.T) {
	elevado(t)
	ruta := filepath.Join(t.TempDir(), "Tramtai")
	if err := ProtegerServidor(ruta, cuentaPrueba); err != nil {
		t.Fatal(err)
	}
	servicio, _, _, err := windows.LookupSID("", cuentaPrueba)
	if err != nil {
		t.Fatal(err)
	}
	dueno, protegida, entradas := descriptor(t, ruta)
	quiere := []string{"S-1-5-18 1f01ff 3", "S-1-5-32-544 1f01ff 3", servicio.String() + " 1f01ff 3"}
	if dueno != "S-1-5-32-544" || !protegida || !slices.Equal(entradas, quiere) {
		t.Errorf("descriptor: %s %v %q", dueno, protegida, entradas)
	}
	// Otra vez (una instalación nuestra anterior, dueño Administradores): vale y queda igual.
	if err := ProtegerServidor(ruta, cuentaPrueba); err != nil {
		t.Fatal(err)
	}
	if d, p, e := descriptor(t, ruta); d != dueno || !p || !slices.Equal(e, quiere) {
		t.Errorf("tras repetir: %s %v %q", d, p, e)
	}
}

// Una carpeta que ya existía con otro dueño (la pudo crear un usuario sin privilegios para quedarse con algo dentro)
// no se toma: es un error y no se cambian sus permisos.
func TestProtegerServidorOtroDueno(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "Tramtai")
	if err := os.Mkdir(ruta, 0o700); err != nil {
		t.Fatal(err)
	}
	usuarios, _ := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	dueno0 := duenoDe
	t.Cleanup(func() { duenoDe = dueno0 })
	duenoDe = func(windows.Handle) (*windows.SID, error) { return usuarios, nil }
	_, protegida0, entradas0 := descriptor(t, ruta)
	err := ProtegerServidor(ruta, cuentaPrueba)
	if err == nil || !strings.Contains(err.Error(), "otro usuario") {
		t.Fatalf("con otro dueño: %v", err)
	}
	if _, p, e := descriptor(t, ruta); p != protegida0 || !slices.Equal(e, entradas0) {
		t.Errorf("no se tocan sus permisos: %v %q", p, e)
	}
}

// Una carpeta de Administradores sin proteger (la creó una orden --servidor antes de instalar, con los permisos de
// %ProgramData%) se toma solo si todo lo de dentro es de Administradores, SYSTEM o la cuenta del servicio.
func TestProtegerServidorSinProteger(t *testing.T) {
	elevado(t)
	usuarios, _ := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	admins, _ := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	dueno0 := duenoDe
	t.Cleanup(func() { duenoDe = dueno0 })
	// Los nombres con «#deotro» son de otro usuario; el resto, de Administradores (el nombre de la prueba no lo lleva).
	duenoDe = func(h windows.Handle) (*windows.SID, error) {
		nombre := make([]uint16, windows.MAX_LONG_PATH)
		n, err := windows.GetFinalPathNameByHandle(h, &nombre[0], uint32(len(nombre)), 0)
		if err != nil {
			return nil, err
		}
		if strings.Contains(windows.UTF16ToString(nombre[:n]), "#deotro") {
			return usuarios, nil
		}
		return admins, nil
	}
	preparar := func(t *testing.T, archivos ...string) string {
		ruta := filepath.Join(t.TempDir(), "Tramtai")
		if err := os.MkdirAll(filepath.Join(ruta, "datos"), 0o700); err != nil {
			t.Fatal(err)
		}
		for _, a := range archivos {
			if err := os.WriteFile(filepath.Join(ruta, a), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if _, protegida, _ := descriptor(t, ruta); protegida {
			t.Fatal("la carpeta de partida no debe estar protegida")
		}
		return ruta
	}
	t.Run("con algo de otro dentro", func(t *testing.T) {
		ruta := preparar(t, "config.yaml", filepath.Join("datos", "administrador#deotro"))
		if err := ProtegerServidor(ruta, cuentaPrueba); err == nil || !strings.Contains(err.Error(), "otro usuario") {
			t.Fatalf("con un hijo ajeno: %v", err)
		}
		// El primer intento dejó la raíz protegida; uno segundo tampoco lo toma.
		if err := ProtegerServidor(ruta, cuentaPrueba); err == nil || !strings.Contains(err.Error(), "otro usuario") {
			t.Fatalf("otra vez, ya protegida: %v", err)
		}
	})
	t.Run("con un punto de unión dentro", func(t *testing.T) {
		ruta := preparar(t, "config.yaml")
		fuera := t.TempDir()
		if salida, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(ruta, "datos", "union"), fuera).CombinedOutput(); err != nil {
			t.Fatalf("mklink /J: %v %s", err, salida)
		}
		if err := ProtegerServidor(ruta, cuentaPrueba); err == nil || !strings.Contains(err.Error(), "otro usuario") {
			t.Fatalf("con una unión dentro: %v", err)
		}
	})
	t.Run("todo propio", func(t *testing.T) {
		ruta := preparar(t, "config.yaml", filepath.Join("datos", "terminos.json"))
		if err := ProtegerServidor(ruta, cuentaPrueba); err != nil {
			t.Fatal(err)
		}
		if dueno, protegida, _ := descriptor(t, ruta); dueno != "S-1-5-32-544" || !protegida {
			t.Errorf("se protege: %s %v", dueno, protegida)
		}
	})
}

// Un punto de unión en el lugar de la carpeta (mklink /J, sin privilegios) es un error, y la carpeta a la que apunta
// no cambia.
func TestProtegerServidorUnion(t *testing.T) {
	base := t.TempDir()
	destino, union := filepath.Join(base, "destino"), filepath.Join(base, "Tramtai")
	if err := os.Mkdir(destino, 0o700); err != nil {
		t.Fatal(err)
	}
	if salida, err := exec.Command("cmd", "/c", "mklink", "/J", union, destino).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v %s", err, salida)
	}
	_, protegida0, entradas0 := descriptor(t, destino)
	if err := ProtegerServidor(union, cuentaPrueba); err == nil || !strings.Contains(err.Error(), "no es una carpeta") {
		t.Fatalf("con un punto de unión: %v", err)
	}
	if _, p, e := descriptor(t, destino); p != protegida0 || !slices.Equal(e, entradas0) {
		t.Errorf("la carpeta de destino no cambia: %v %q", p, e)
	}
}

// La carpeta portable queda solo para el usuario actual y SYSTEM, sin herencia; si ya lo estaba, no se toca.
func TestProtegerCarpeta(t *testing.T) {
	dir := t.TempDir()
	if err := protegerCarpeta(dir); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("la DACL debe estar protegida (sin herencia): %v %x", err, control)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl.AceCount != 2 {
		t.Fatalf("se esperaban 2 entradas (usuario y SYSTEM): %v %+v", err, dacl)
	}
	usuario, _ := windows.GetCurrentProcessToken().GetTokenUser()
	sistema, _ := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	var sids []string
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		sids = append(sids, (*windows.SID)(unsafePointer(&ace.SidStart)).String())
	}
	if !slices.Contains(sids, usuario.User.Sid.String()) || !slices.Contains(sids, sistema.String()) {
		t.Fatalf("las entradas deben ser el usuario actual y SYSTEM: %v", sids)
	}
	if err := protegerCarpeta(dir); err != nil {
		t.Fatalf("protegida ya, no debe fallar: %v", err)
	}
}
