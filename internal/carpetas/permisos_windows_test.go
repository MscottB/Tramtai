package carpetas

import (
	"slices"
	"testing"

	"golang.org/x/sys/windows"
)

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
