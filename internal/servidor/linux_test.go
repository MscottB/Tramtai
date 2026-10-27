//go:build linux

package servidor

import (
	"context"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tramtai/internal/configuracion"
	"tramtai/internal/terminos"
)

// prueba es un sistema simulado: las rutas en una carpeta temporal, las órdenes y los cambios de dueño anotados, y la
// cuenta tramtai que existe o no.
type prueba struct {
	r       Sitios
	origen  string
	ordenes []string
	duenos  []string
	cuenta  bool
	// alOrdenar se llama con cada orden antes de anotarla (para mirar los archivos en ese momento).
	alOrdenar func(orden string)
}

func nuevaPrueba(t *testing.T) *prueba {
	t.Helper()
	base := t.TempDir()
	p := &prueba{r: Sitios{
		Programa:   filepath.Join(base, "opt", "tramtai"),
		Datos:      filepath.Join(base, "var", "lib", "tramtai"),
		Envoltorio: filepath.Join(base, "usr", "local", "bin", "tramtai-servidor"),
		Unidad:     filepath.Join(base, "etc", "systemd", "system", "tramtai.service"),
	}, origen: filepath.Join(base, "origen")}
	for _, d := range []string{p.origen, filepath.Dir(p.r.Envoltorio), filepath.Dir(p.r.Unidad)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p.programa(t, "v1")
	rutas0, ejecutar0, chown0, conSystemd0 := rutas, ejecutar, chown, conSystemd
	t.Cleanup(func() { rutas, ejecutar, chown, conSystemd = rutas0, ejecutar0, chown0, conSystemd0 })
	rutas = func() Sitios { return p.r }
	conSystemd = func() bool { return true }
	chown = func(ruta string) error {
		p.duenos = append(p.duenos, ruta)
		return nil
	}
	ejecutar = func(nombre string, args ...string) ([]byte, error) {
		orden := strings.Join(append([]string{nombre}, args...), " ")
		if p.alOrdenar != nil {
			p.alOrdenar(orden)
		}
		p.ordenes = append(p.ordenes, orden)
		switch orden {
		case "id tramtai":
			if !p.cuenta {
				return []byte("id: «tramtai»: no existe ese usuario\n"), errors.New("exit status 1")
			}
		case "useradd --system --home " + p.r.Datos + " --shell /usr/sbin/nologin tramtai":
			p.cuenta = true
		case "userdel tramtai":
			p.cuenta = false
		}
		return nil, nil
	}
	return p
}

// programa pone en el origen un tramtai y un uv de la versión dada.
func (p *prueba) programa(t *testing.T, version string) {
	t.Helper()
	for _, nombre := range []string{"tramtai", "uv"} {
		if err := os.WriteFile(filepath.Join(p.origen, nombre), []byte(nombre+" "+version), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func (p *prueba) instalar(t *testing.T) {
	t.Helper()
	p.ordenes, p.duenos = nil, nil
	if err := Instalar(Opciones{Origen: p.origen, AceptoTerminos: true, Como: terminos.ComoOpcion}); err != nil {
		t.Fatal(err)
	}
}

func mismas(t *testing.T, que string, obtenidas []string, esperadas ...string) {
	t.Helper()
	if !slices.Equal(obtenidas, esperadas) {
		t.Errorf("%s:\n obtenidas %q\n esperadas %q", que, obtenidas, esperadas)
	}
}

// TestInstalarLinuxOrdenes: la cuenta (solo si falta), el programa, la unidad, daemon-reload y enable --now, en ese
// orden; la carpeta de datos 0700 de tramtai, con la configuración inicial solo en loopback y los términos aceptados.
func TestInstalarLinuxOrdenes(t *testing.T) {
	p := nuevaPrueba(t)
	tramtai, uv := filepath.Join(p.r.Programa, "tramtai"), filepath.Join(p.r.Programa, "uv")
	p.alOrdenar = func(orden string) {
		switch {
		case strings.HasPrefix(orden, "useradd") && existe(tramtai):
			t.Error("el programa se copia después de crear la cuenta")
		case orden == "systemctl daemon-reload" && !(existe(tramtai) && existe(uv) && existe(p.r.Unidad) && existe(p.r.Envoltorio)):
			t.Error("daemon-reload va después de copiar el programa y escribir la unidad")
		}
	}
	p.instalar(t)
	mismas(t, "órdenes", p.ordenes,
		"id tramtai",
		"useradd --system --home "+p.r.Datos+" --shell /usr/sbin/nologin tramtai",
		"systemctl daemon-reload",
		"systemctl enable --now tramtai")

	if b, _ := os.ReadFile(tramtai); string(b) != "tramtai v1" {
		t.Errorf("tramtai copiado: %q", b)
	}
	if info, err := os.Stat(p.r.Programa); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("la carpeta del programa la lee la cuenta del servicio (0755): %v %v", info.Mode(), err)
	}
	if b, _ := os.ReadFile(p.r.Unidad); string(b) != Unidad(UnidadOpciones{}) {
		t.Errorf("unidad escrita:\n%s", b)
	}
	if b, _ := os.ReadFile(p.r.Envoltorio); string(b) != "#!/bin/sh\nexec /opt/tramtai/tramtai --servidor \"$@\"\n" {
		t.Errorf("envoltorio: %q", b)
	}
	if info, _ := os.Stat(p.r.Envoltorio); info.Mode().Perm() != 0o755 {
		t.Errorf("el envoltorio se ejecuta: %v", info.Mode())
	}
	datos := filepath.Join(p.r.Datos, "datos")
	for _, d := range []string{p.r.Datos, datos} {
		if info, err := os.Stat(d); err != nil || info.Mode().Perm() != 0o700 {
			t.Errorf("%s debe ser 0700: %v", d, err)
		}
	}
	c, err := configuracion.CargarCapa(filepath.Join(p.r.Datos, "config.yaml"), configuracion.OrigenGlobal)
	if err != nil || c.Valores["servicio.host"] != "127.0.0.1" {
		t.Errorf("configuración inicial en loopback: %v %v", c.Valores, err)
	}
	if a, _ := terminos.Leer(datos); a.Como != terminos.ComoOpcion || a.Version != terminos.Version {
		t.Errorf("términos aceptados: %+v", a)
	}
	// Lo que escribe la instalación es de la cuenta del servicio, que corre como tramtai (ruling R12).
	mismas(t, "dueño tramtai", p.duenos, p.r.Datos, datos, filepath.Join(p.r.Datos, "config.yaml"), filepath.Join(datos, "terminos.json"))
}

// TestInstalarSinAceptar: sin aceptar los términos no se hace nada.
func TestInstalarSinAceptar(t *testing.T) {
	p := nuevaPrueba(t)
	if err := Instalar(Opciones{Origen: p.origen}); err == nil {
		t.Fatal("sin aceptar los términos, instalar falla")
	}
	mismas(t, "órdenes", p.ordenes)
	if existe(p.r.Programa) || existe(p.r.Datos) || existe(p.r.Unidad) {
		t.Error("sin aceptar no se escribe nada")
	}
}

// TestInstalarSinSystemd: en un Linux sin systemd lo dice y no hace nada.
func TestInstalarSinSystemd(t *testing.T) {
	p := nuevaPrueba(t)
	conSystemd = func() bool { return false }
	if err := Instalar(Opciones{Origen: p.origen, AceptoTerminos: true}); err == nil || !strings.Contains(err.Error(), "systemd") {
		t.Fatalf("sin systemd: %v", err)
	}
	mismas(t, "órdenes", p.ordenes)
}

// TestInstalarRepetido: con la misma versión no copia ni reinicia (ni pisa la configuración); con otra, actualiza y
// reinicia.
func TestInstalarRepetido(t *testing.T) {
	p := nuevaPrueba(t)
	p.instalar(t)
	p.instalar(t)
	mismas(t, "misma versión", p.ordenes, "id tramtai", "systemctl enable --now tramtai")

	config := filepath.Join(p.r.Datos, "config.yaml")
	if err := configuracion.FijarValor(config, configuracion.OrigenGlobal, "servicio.plazo_drenaje_segundos", 90); err != nil {
		t.Fatal(err)
	}
	p.programa(t, "v2")
	p.instalar(t)
	mismas(t, "otra versión", p.ordenes, "id tramtai", "systemctl daemon-reload", "systemctl enable --now tramtai", "systemctl restart tramtai")
	if b, _ := os.ReadFile(filepath.Join(p.r.Programa, "uv")); string(b) != "uv v2" {
		t.Errorf("uv actualizado: %q", b)
	}
	if c, _ := configuracion.CargarCapa(config, configuracion.OrigenGlobal); c.Valores["servicio.plazo_drenaje_segundos"] != 90 {
		t.Errorf("la configuración existente no se pisa: %v", c.Valores)
	}
	// La unidad toma el plazo de drenaje de la configuración del servidor.
	if b, _ := os.ReadFile(p.r.Unidad); string(b) != Unidad(UnidadOpciones{PlazoDrenaje: 90}) {
		t.Errorf("unidad con el plazo de la configuración:\n%s", b)
	}
}

// TestDesinstalarConservaDatos: quita el servicio, el programa, el envoltorio y la unidad, y conserva los datos; con
// borrarDatos, borra también la carpeta y la cuenta.
func TestDesinstalarConservaDatos(t *testing.T) {
	p := nuevaPrueba(t)
	p.instalar(t)
	p.ordenes = nil
	if err := Desinstalar(false); err != nil {
		t.Fatal(err)
	}
	mismas(t, "desinstalar", p.ordenes, "systemctl disable --now tramtai", "systemctl daemon-reload")
	if existe(p.r.Programa) || existe(p.r.Envoltorio) || existe(p.r.Unidad) {
		t.Error("el programa, el envoltorio y la unidad se quitan")
	}
	if !existe(filepath.Join(p.r.Datos, "datos", "terminos.json")) || !p.cuenta {
		t.Error("los datos y la cuenta se conservan")
	}

	p.instalar(t)
	p.ordenes = nil
	if err := Desinstalar(true); err != nil {
		t.Fatal(err)
	}
	mismas(t, "desinstalar --borrar-datos", p.ordenes, "systemctl disable --now tramtai", "systemctl daemon-reload", "id tramtai", "userdel tramtai")
	if existe(p.r.Datos) || p.cuenta {
		t.Error("con borrarDatos se borran la carpeta y la cuenta")
	}
	// Repetirlo no falla: ya no hay unidad ni cuenta.
	p.ordenes = nil
	if err := Desinstalar(true); err != nil {
		t.Fatal(err)
	}
	mismas(t, "desinstalar otra vez", p.ordenes, "id tramtai")
}

// TestEstadoLinux: sin unidad, no instalado y sin órdenes; con ella, lo que dice systemctl is-active (que sale con
// error cuando el servicio no está activo).
func TestEstadoLinux(t *testing.T) {
	p := nuevaPrueba(t)
	if e, err := Estado(context.Background()); err != nil || e.Instalado || len(p.ordenes) > 0 {
		t.Fatalf("sin instalar: %+v %v %q", e, err, p.ordenes)
	}
	p.instalar(t)
	ejecutar = func(string, ...string) ([]byte, error) { return []byte("inactive\n"), errors.New("exit status 3") }
	if e, err := Estado(context.Background()); err != nil || !e.Instalado || e.Activo != "inactive" {
		t.Errorf("instalado y detenido: %+v %v", e, err)
	}
}

// TestAjustarDueno: cambia el dueño a la cuenta con ese nombre, y falla si la cuenta no existe.
func TestAjustarDueno(t *testing.T) {
	yo, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	ruta := filepath.Join(t.TempDir(), "terminos.json")
	if err := os.WriteFile(ruta, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := cambiarDueno(ruta, yo.Username); err != nil {
		t.Errorf("a la propia cuenta: %v", err)
	}
	if err := cambiarDueno(ruta, "tramtai-cuenta-que-no-existe"); err == nil {
		t.Error("sin la cuenta, falla")
	}
}
