//go:build windows

package servidor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// entradasDACL lista las entradas de la DACL de ruta («SID máscara banderas») y dice si está protegida (sin herencia).
func entradasDACL(t *testing.T, ruta string) (entradas []string, protegida bool) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(ruta, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		entradas = append(entradas, fmt.Sprintf("%s %x %x", sid, ace.Mask, ace.Header.AceFlags&^windows.INHERITED_ACE))
	}
	return entradas, control&windows.SE_DACL_PROTECTED != 0
}

// lecturaPublica es la DACL de la carpeta publico: control total para SYSTEM, los administradores y la cuenta del
// servicio, y solo lectura (FILE_GENERIC_READ) para Usuarios, sin heredar nada de la raíz del servidor. En la carpeta
// van con las banderas de herencia; en el archivo, heredadas, sin ellas.
func lecturaPublica(sidServicio string, banderas uint8) []string {
	const total, lectura = 0x1f01ff, 0x120089
	return []string{fmt.Sprintf("S-1-5-18 %x %x", total, banderas), fmt.Sprintf("S-1-5-32-544 %x %x", total, banderas),
		fmt.Sprintf("%s %x %x", sidServicio, total, banderas), fmt.Sprintf("S-1-5-32-545 %x %x", lectura, banderas)}
}

// TestCarpetaPublicaWindows: publicar el puerto crea %ProgramData%\Tramtai\publico con su propia DACL, protegida: los
// usuarios leen el archivo puerto y no pueden escribirlo; lo demás de la raíz sigue cerrado (la DACL de la raíz no se
// toca). Un punto de unión en el lugar de la carpeta no se sigue.
func TestCarpetaPublicaWindows(t *testing.T) {
	ruta := conRutaPublica(t)
	if err := PublicarPuerto(17374); err != nil {
		t.Fatal(err)
	}
	carpeta := filepath.Dir(ruta)
	quiere := lecturaPublica(sidServicio(nombreServicio), windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE)
	if entradas, protegida := entradasDACL(t, carpeta); !protegida || !slices.Equal(entradas, quiere) {
		t.Errorf("DACL de publico (protegida %v):\n %q\n quiere %q", protegida, entradas, quiere)
	}
	if entradas, _ := entradasDACL(t, ruta); !slices.Equal(entradas, lecturaPublica(sidServicio(nombreServicio), 0)) {
		t.Errorf("el archivo hereda la DACL de publico (Usuarios solo lee): %q", entradas)
	}

	// Una unión a otra carpeta en el lugar de publico: no se escribe a través de ella.
	otra := t.TempDir()
	if err := os.RemoveAll(carpeta); err != nil {
		t.Fatal(err)
	}
	if salida, err := exec.Command("cmd", "/c", "mklink", "/J", carpeta, otra).CombinedOutput(); err != nil {
		t.Skipf("no se pudo crear la unión: %v %s", err, salida)
	}
	if err := PublicarPuerto(17374); err == nil {
		t.Error("con una unión en lugar de la carpeta no se publica")
	}
	if _, err := os.Stat(filepath.Join(otra, "puerto")); err == nil {
		t.Error("se escribió a través de la unión")
	}
}
