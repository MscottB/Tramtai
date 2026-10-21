package servicio

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"tramtai/internal/nucleo"
)

func TestFijarPuerto(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "config.yaml")
	if err := FijarPuerto(ruta, "127.0.0.1", 80, false); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("el 80 está fuera de rango: %v", err)
	}
	if err := FijarPuerto(ruta, "127.0.0.1", 70000, true); err == nil {
		t.Fatal("el 70000 está fuera de rango aunque sea automático")
	}
	ocupado, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ocupado.Close()
	puerto := ocupado.Addr().(*net.TCPAddr).Port
	if puerto < 1024 {
		t.Skip("el sistema dio un puerto bajo")
	}
	if err := FijarPuerto(ruta, "127.0.0.1", puerto, false); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("un puerto ocupado sin automático da error: %v", err)
	}
	if _, err := os.Stat(ruta); err == nil {
		t.Fatal("con error no se escribe nada")
	}
	if err := FijarPuertoPropio(ruta, "127.0.0.1", puerto, puerto, false); err != nil {
		t.Fatalf("el puerto que ya usa Tramtai cuenta como libre: %v", err)
	}
	if err := FijarPuerto(ruta, "127.0.0.1", puerto, true); nucleo.CodigoDe(err) != nucleo.CodigoSolicitudInvalida {
		t.Fatalf("un puerto ocupado da error también en automático: %v", err)
	}
	libre, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	puerto = libre.Addr().(*net.TCPAddr).Port
	libre.Close()
	if err := FijarPuerto(ruta, "127.0.0.1", puerto, true); err != nil {
		t.Fatalf("un puerto libre en automático: %v", err)
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(datos), "puerto_automatico: true") || !strings.Contains(string(datos), "puerto: "+strconv.Itoa(puerto)) {
		t.Fatalf("no se escribió el puerto y el automático:\n%s", datos)
	}
}
