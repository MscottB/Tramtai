package nucleo

import (
	"testing"

	"golang.org/x/sys/windows"
)

// La carpeta portable queda solo para el usuario actual y SYSTEM, sin herencia.
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
}
