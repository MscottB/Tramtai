//go:build linux

package servidor

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
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
	simularDueno(t, &p.duenos)
	rutas0, ejecutar0, conSystemd0 := rutas, ejecutar, conSystemd
	t.Cleanup(func() { rutas, ejecutar, conSystemd = rutas0, ejecutar0, conSystemd0 })
	rutas = func() Sitios { return p.r }
	conSystemd = func() bool { return true }
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
		case "useradd --system --user-group --home " + p.r.Datos + " --shell /usr/sbin/nologin tramtai":
			p.cuenta = true
		case "userdel tramtai":
			p.cuenta = false
		}
		return nil, nil
	}
	return p
}

// simularDueno anota, en lugar de cambiarlo, el dueño de cada nombre de dentro de la raíz (no hay cuenta tramtai ni root).
func simularDueno(t *testing.T, duenos *[]string) {
	t.Helper()
	lchown0, ids0 := lchown, idsServicio
	t.Cleanup(func() { lchown, idsServicio = lchown0, ids0 })
	idsServicio = func() (int, int, error) { return 990, 990, nil }
	lchown = func(r *os.Root, nombre string, uid, gid int) error {
		if uid != 990 || gid != 990 {
			t.Errorf("dueño de %s: %d:%d", nombre, uid, gid)
		}
		*duenos = append(*duenos, nombre)
		return nil
	}
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
		"useradd --system --user-group --home "+p.r.Datos+" --shell /usr/sbin/nologin tramtai",
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
	mismas(t, "dueño tramtai", p.duenos, ".", "config.yaml", "datos", "datos/terminos.json")
}

// TestInstalarNoSigueEnlaces: la raíz es de la cuenta del servicio, que puede dejar enlaces; root, al instalar, no los
// sigue: ni datos/terminos.json, ni datos, ni config.yaml enlazados le hacen escribir o leer fuera.
func TestInstalarNoSigueEnlaces(t *testing.T) {
	t.Run("terminos.json", func(t *testing.T) {
		p := nuevaPrueba(t)
		p.instalar(t)
		secreto := filepath.Join(t.TempDir(), "shadow")
		if err := os.WriteFile(secreto, []byte("no tocar"), 0o640); err != nil {
			t.Fatal(err)
		}
		enlace := filepath.Join(p.r.Datos, "datos", "terminos.json")
		if err := os.Remove(enlace); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(secreto, enlace); err != nil {
			t.Fatal(err)
		}
		p.instalar(t)
		if b, _ := os.ReadFile(secreto); string(b) != "no tocar" {
			t.Errorf("se escribió a través del enlace: %q", b)
		}
		if info, err := os.Lstat(enlace); err != nil || !info.Mode().IsRegular() {
			t.Errorf("terminos.json queda como archivo propio: %v %v", info.Mode(), err)
		}
	})
	t.Run("datos", func(t *testing.T) {
		p := nuevaPrueba(t)
		otra := t.TempDir()
		if err := os.Chmod(otra, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(p.r.Datos, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(otra, filepath.Join(p.r.Datos, "datos")); err != nil {
			t.Fatal(err)
		}
		if err := Instalar(Opciones{Origen: p.origen, AceptoTerminos: true}); err == nil {
			t.Error("con datos enlazado, instalar falla")
		}
		if info, _ := os.Stat(otra); info.Mode().Perm() != 0o755 || existe(filepath.Join(otra, "terminos.json")) {
			t.Errorf("la carpeta enlazada no se toca: %v", info.Mode())
		}
		if len(p.duenos) > 0 {
			t.Errorf("se falla antes de cambiar dueños: %q", p.duenos)
		}
	})
	t.Run("config.yaml", func(t *testing.T) {
		p := nuevaPrueba(t)
		fuera := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(fuera, []byte("servicio:\n  host: 0.0.0.0\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(p.r.Datos, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(fuera, filepath.Join(p.r.Datos, "config.yaml")); err != nil {
			t.Fatal(err)
		}
		if err := Instalar(Opciones{Origen: p.origen, AceptoTerminos: true}); !errors.As(err, new(tipoInesperado)) {
			t.Errorf("con config.yaml enlazado, instalar falla por el tipo: %v", err)
		}
	})
	t.Run("config.yaml FIFO", func(t *testing.T) {
		p := nuevaPrueba(t)
		if err := os.MkdirAll(p.r.Datos, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(filepath.Join(p.r.Datos, "config.yaml"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Instalar(Opciones{Origen: p.origen, AceptoTerminos: true}); !errors.As(err, new(tipoInesperado)) {
			t.Errorf("con un FIFO por config.yaml, instalar falla sin quedarse leyéndolo: %v", err)
		}
	})
}

// TestReiniciarUnidadAlDia: reiniciar regenera la unidad con la configuración actual; si cambió, la reescribe y hace
// daemon-reload antes del restart.
func TestReiniciarUnidadAlDia(t *testing.T) {
	p := nuevaPrueba(t)
	p.instalar(t)
	p.ordenes = nil
	if err := Reiniciar(); err != nil {
		t.Fatal(err)
	}
	mismas(t, "sin cambios", p.ordenes, "systemctl restart tramtai")

	config := filepath.Join(p.r.Datos, "config.yaml")
	if err := configuracion.FijarValor(config, configuracion.OrigenGlobal, "servicio.plazo_drenaje_segundos", 90); err != nil {
		t.Fatal(err)
	}
	p.alOrdenar = func(orden string) {
		if b, _ := os.ReadFile(p.r.Unidad); orden == "systemctl daemon-reload" && string(b) != Unidad(UnidadOpciones{PlazoDrenaje: 90}) {
			t.Error("la unidad se reescribe antes del daemon-reload")
		}
	}
	p.ordenes = nil
	if err := Reiniciar(); err != nil {
		t.Fatal(err)
	}
	mismas(t, "otro plazo", p.ordenes, "systemctl daemon-reload", "systemctl restart tramtai")
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

// TestAjustarDueno: recorre la raíz y deja cada carpeta y archivo a nombre de la cuenta del servicio, sin seguir enlaces
// (el enlace mismo sí cambia de dueño; lo que hay detrás, no). Sin la raíz no hace nada.
func TestAjustarDueno(t *testing.T) {
	var duenos []string
	simularDueno(t, &duenos)
	raiz, fuera := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(fuera, "ajeno"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(raiz, "datos"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, nombre := range []string{"config.yaml", "datos/administrador"} {
		if err := os.WriteFile(filepath.Join(raiz, nombre), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(fuera, filepath.Join(raiz, "datos", "enlace")); err != nil {
		t.Fatal(err)
	}
	// Un enlace duro a un archivo de fuera (como /etc/shadow) no cambia de dueño.
	if err := os.Link(filepath.Join(fuera, "ajeno"), filepath.Join(raiz, "datos", "duro")); err != nil {
		t.Fatal(err)
	}
	if err := AjustarDueno(raiz); err != nil {
		t.Fatal(err)
	}
	mismas(t, "dueños", duenos, ".", "config.yaml", "datos", "datos/administrador", "datos/enlace")

	// Lo que desaparece a mitad del recorrido (un temporal del servicio en marcha) no lo para.
	duenos = nil
	anotar := lchown
	lchown = func(r *os.Root, nombre string, uid, gid int) error {
		if nombre == "config.yaml" {
			return &fs.PathError{Op: "lchown", Path: nombre, Err: fs.ErrNotExist}
		}
		return anotar(r, nombre, uid, gid)
	}
	if err := AjustarDueno(raiz); err != nil {
		t.Fatalf("un archivo que ya no está no es un error: %v", err)
	}
	mismas(t, "dueños sin config.yaml", duenos, ".", "datos", "datos/administrador", "datos/enlace")
	lchown = anotar

	duenos = nil
	if err := AjustarDueno(filepath.Join(raiz, "no-existe")); err != nil || len(duenos) > 0 {
		t.Errorf("sin la raíz no hace nada: %v %q", err, duenos)
	}
	// Antes de instalar no hay cuenta: no hay nada que ajustar ni de qué avisar.
	idsServicio = func() (int, int, error) { return 0, 0, user.UnknownUserError("tramtai") }
	if err := AjustarDueno(raiz); err != nil || len(duenos) > 0 {
		t.Errorf("sin la cuenta no hace nada: %v %q", err, duenos)
	}
}

// TestIdsDe: el uid y el gid de una cuenta, y error si no existe.
func TestIdsDe(t *testing.T) {
	yo, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	if uid, _, err := idsDe(yo.Username); err != nil || uid != os.Getuid() {
		t.Errorf("la propia cuenta: %d %v", uid, err)
	}
	if _, _, err := idsDe("tramtai-cuenta-que-no-existe"); err == nil {
		t.Error("sin la cuenta, falla")
	}
}
