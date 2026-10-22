package persistencia

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// compactable solo dice que compactó si la base quedó de verdad en auto_vacuum incremental: si el VACUUM falla (disco
// lleno; aquí, una base de solo lectura), el registro no puede decir «se compactó la base».
func TestCompactableSinVacuum(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "vieja.db")
	vieja, err := sql.Open("sqlite", "file:"+rutaURI(ruta))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vieja.Exec("CREATE TABLE t (x)"); err != nil {
		t.Fatal(err)
	}
	vieja.Close()
	soloLectura, err := sql.Open("sqlite", "file:"+rutaURI(ruta)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer soloLectura.Close()
	if compactable(soloLectura) {
		t.Fatal("dijo que compactó sin poder hacer el VACUUM")
	}
}
