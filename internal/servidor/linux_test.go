//go:build linux

package servidor

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"tramtai/internal/actualizacion"
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
	// creds es lo que dice «systemd-creds --version» (vacío: no está); entradas, lo que recibió cada orden por stdin.
	creds    string
	entradas []string
}

func nuevaPrueba(t *testing.T) *prueba {
	t.Helper()
	base := t.TempDir()
	p := &prueba{r: Sitios{
		Programa:   filepath.Join(base, "opt", "tramtai"),
		Datos:      filepath.Join(base, "var", "lib", "tramtai"),
		Envoltorio: filepath.Join(base, "usr", "local", "bin", "tramtai-servidor"),
		Unidad:     filepath.Join(base, "etc", "systemd", "system", "tramtai.service"),
		UnidadConf: filepath.Join(base, "etc", "systemd", "system", "tramtai.service.d", "unidad.conf"),
		Cifradas:   filepath.Join(base, "etc", "credstore.encrypted"),
		ClavesConf: filepath.Join(base, "etc", "systemd", "system", "tramtai.service.d", "claves.conf"),
		ClavesEnv:  filepath.Join(base, "etc", "tramtai", "claves.env"),
	}, origen: filepath.Join(base, "origen")}
	for _, d := range []string{p.origen, filepath.Dir(p.r.Envoltorio), filepath.Dir(p.r.Unidad)} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p.programa(t, "v1")
	simularDueno(t, &p.duenos)
	rutas0, ejecutar0, entrada0, conSystemd0 := rutas, ejecutar, ejecutarConEntrada, conSystemd
	t.Cleanup(func() { rutas, ejecutar, ejecutarConEntrada, conSystemd = rutas0, ejecutar0, entrada0, conSystemd0 })
	rutas = func() Sitios { return p.r }
	conSystemd = func() bool { return true }
	ejecutarConEntrada = func(entrada []byte, nombre string, args ...string) ([]byte, error) {
		p.entradas = append(p.entradas, string(entrada))
		salida, err := ejecutar(nombre, args...)
		if err == nil && nombre == "systemd-creds" && args[0] == "encrypt" { // cifrado de mentira, en el archivo de salida
			err = os.WriteFile(args[len(args)-1], append([]byte("cifrado:"), entrada...), 0o644) // con la umask, como el de verdad
		}
		return salida, err
	}
	ejecutar = func(nombre string, args ...string) ([]byte, error) {
		orden := strings.Join(append([]string{nombre}, args...), " ")
		if p.alOrdenar != nil {
			p.alOrdenar(orden)
		}
		p.ordenes = append(p.ordenes, orden)
		switch orden {
		case "systemd-creds --version":
			if p.creds == "" {
				return nil, &exec.Error{Name: "systemd-creds", Err: exec.ErrNotFound}
			}
			return []byte(p.creds), nil
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
		if err := Instalar(Opciones{Origen: p.origen, AceptoTerminos: true}); !errors.As(err, new(configuracion.ErrTipoInesperado)) {
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
		if err := Instalar(Opciones{Origen: p.origen, AceptoTerminos: true}); !errors.As(err, new(configuracion.ErrTipoInesperado)) {
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

// TestActualizarServidor: descarga el .tar.gz firmado de la versión nueva y, con el nuevo ya preparado, detiene el
// servicio (systemctl stop drena), aparta el programa como .anterior, pone el nuevo y lo arranca el programa nuevo con
// «servidor reiniciar» (que deja la unidad al día), en ese orden. El programa nuevo es 0755 aunque root tenga umask
// 077 y aunque quedara un .nuevo de antes con otros permisos.
func TestActualizarServidor(t *testing.T) {
	p := nuevaPrueba(t)
	p.instalar(t)
	versionPublicada(t, map[string]string{"tramtai": "tramtai nuevo", "uv": "uv nuevo", "LICENCIA.md": "licencia"}, false)
	if err := os.WriteFile(filepath.Join(p.r.Programa, "tramtai.nuevo"), []byte("resto"), 0o600); err != nil {
		t.Fatal(err)
	}
	umask := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(umask) })
	leer := func(nombre string) string {
		b, _ := os.ReadFile(filepath.Join(p.r.Programa, nombre))
		return string(b)
	}
	reiniciar := filepath.Join(p.r.Programa, "tramtai") + " --servidor servidor reiniciar"
	p.alOrdenar = func(orden string) {
		switch orden {
		case "systemctl stop tramtai":
			if leer("tramtai") != "tramtai v1" || leer("tramtai.nuevo") != "tramtai nuevo" || leer("uv.nuevo") != "uv nuevo" {
				t.Error("se detiene con el programa nuevo ya preparado y el de antes en su sitio")
			}
		case reiniciar:
			if leer("tramtai") != "tramtai nuevo" || leer("uv") != "uv nuevo" {
				t.Error("se arranca con el programa nuevo en su sitio")
			}
		}
	}
	p.ordenes = nil
	if err := Actualizar(t.Context(), actualizacion.PaqueteServidor); err != nil {
		t.Fatal(err)
	}
	mismas(t, "órdenes", p.ordenes, "systemctl stop tramtai", reiniciar)
	for nombre, quiere := range map[string]string{"tramtai": "tramtai nuevo", "uv": "uv nuevo", "tramtai.anterior": "tramtai v1",
		"uv.anterior": "uv v1", "tramtai.nuevo": "", "LICENCIA.md": ""} {
		if leer(nombre) != quiere {
			t.Errorf("%s: %q, se esperaba %q", nombre, leer(nombre), quiere)
		}
	}
	for _, nombre := range []string{"tramtai", "uv"} {
		if info, err := os.Stat(filepath.Join(p.r.Programa, nombre)); err != nil || info.Mode().Perm() != 0o755 {
			t.Errorf("la cuenta del servicio ejecuta el %s nuevo (0755): %v %v", nombre, info.Mode(), err)
		}
	}
}

// TestUnidadSigueAlModoTLS: con servicio.tls.modo letsencrypt, reiniciar regenera la unidad con CAP_NET_BIND_SERVICE
// (en AmbientCapabilities y en CapabilityBoundingSet, para escuchar en el 443); al dejarlo, la quita.
func TestUnidadSigueAlModoTLS(t *testing.T) {
	p := nuevaPrueba(t)
	p.instalar(t)
	config := filepath.Join(p.r.Datos, "config.yaml")
	for _, c := range []struct {
		modo string
		con  bool
	}{{"letsencrypt", true}, {"ninguno", false}} {
		if err := configuracion.FijarValor(config, configuracion.OrigenGlobal, "servicio.tls.modo", c.modo); err != nil {
			t.Fatal(err)
		}
		if err := Reiniciar(); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(p.r.Unidad)
		for _, linea := range []string{"\nAmbientCapabilities=CAP_NET_BIND_SERVICE\n", "\nCapabilityBoundingSet=CAP_NET_BIND_SERVICE\n"} {
			if strings.Contains(string(b), linea) != c.con {
				t.Errorf("modo %s: la unidad debería llevar %q: %v\n%s", c.modo, linea, c.con, b)
			}
		}
	}
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

// paquete simula el paquete tramtai-servidor recién instalado: el programa con .paquete, la unidad y el envoltorio en
// sus rutas (con contenido reconocible, para ver que no se tocan) y la cuenta, que crea systemd-sysusers.
func (p *prueba) paquete(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(p.r.Programa, 0o755); err != nil {
		t.Fatal(err)
	}
	for ruta, contenido := range map[string]string{filepath.Join(p.r.Programa, "tramtai"): "tramtai del paquete",
		filepath.Join(p.r.Programa, "uv"): "uv del paquete", filepath.Join(p.r.Programa, ".paquete"): "",
		p.r.Unidad: "unidad del paquete", p.r.Envoltorio: "envoltorio del paquete"} {
		if err := os.WriteFile(ruta, []byte(contenido), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p.cuenta = true
}

// sinTocarPaquete comprueba que lo que puso el paquete sigue igual: es de apt o dnf.
func (p *prueba) sinTocarPaquete(t *testing.T) {
	t.Helper()
	for ruta, contenido := range map[string]string{filepath.Join(p.r.Programa, "tramtai"): "tramtai del paquete",
		filepath.Join(p.r.Programa, "uv"): "uv del paquete", p.r.Unidad: "unidad del paquete", p.r.Envoltorio: "envoltorio del paquete"} {
		if b, err := os.ReadFile(ruta); err != nil || string(b) != contenido {
			t.Errorf("%s es del paquete y no se toca: %q %v", ruta, b, err)
		}
	}
}

// TestInstalarConPaquete: con el programa del paquete (.paquete), instalar no copia el programa ni escribe la unidad o
// el envoltorio: crea la configuración, acepta los términos, deja en un drop-in lo que la unidad toma de la
// configuración, y habilita y arranca. Repetirla no cambia nada. El envoltorio del paquete es el de instalar.
func TestInstalarConPaquete(t *testing.T) {
	p := nuevaPrueba(t)
	p.paquete(t)
	p.instalar(t)
	mismas(t, "órdenes", p.ordenes, "id tramtai", "systemctl daemon-reload", "systemctl enable --now tramtai")
	p.sinTocarPaquete(t)
	datos := filepath.Join(p.r.Datos, "datos")
	if a, _ := terminos.Leer(datos); a.Como != terminos.ComoOpcion {
		t.Errorf("términos aceptados: %+v", a)
	}
	if c, err := configuracion.CargarCapa(filepath.Join(p.r.Datos, "config.yaml"), configuracion.OrigenGlobal); err != nil || c.Valores["servicio.host"] != "127.0.0.1" {
		t.Errorf("configuración inicial en loopback: %v %v", c, err)
	}
	if conf := leido(t, p.r.UnidadConf); conf != ajustesPaquete(UnidadOpciones{}) {
		t.Errorf("drop-in de la unidad del paquete:\n%s", conf)
	}

	p.instalar(t)
	mismas(t, "otra vez", p.ordenes, "id tramtai", "systemctl enable --now tramtai")

	if b, err := os.ReadFile("../../scripts/linux/servidor/tramtai-servidor"); err != nil || string(b) != envoltorio {
		t.Errorf("el envoltorio del paquete es el de instalar: %q %v", b, err)
	}
}

// TestReiniciarConPaquete: con la unidad del paquete, reiniciar (el último paso de actualizar, R32) no escribe una
// unidad propia, que taparía la del paquete: el plazo de parada y las capacidades van en el drop-in. Antes de
// «tramtai-servidor instalar» no arranca nada: no hay términos aceptados.
func TestReiniciarConPaquete(t *testing.T) {
	p := nuevaPrueba(t)
	p.paquete(t)
	if err := Reiniciar(); err == nil {
		t.Error("sin instalar, reiniciar falla")
	}
	mismas(t, "sin instalar", p.ordenes)

	p.instalar(t)
	if err := configuracion.FijarValor(filepath.Join(p.r.Datos, "config.yaml"), configuracion.OrigenGlobal, "servicio.tls.modo", "letsencrypt"); err != nil {
		t.Fatal(err)
	}
	p.ordenes = nil
	if err := Reiniciar(); err != nil {
		t.Fatal(err)
	}
	mismas(t, "órdenes", p.ordenes, "systemctl daemon-reload", "systemctl restart tramtai")
	p.sinTocarPaquete(t)
	conf := leido(t, p.r.UnidadConf)
	if conf != ajustesPaquete(UnidadOpciones{LetsEncrypt: true}) || !strings.Contains(conf, "\nAmbientCapabilities=CAP_NET_BIND_SERVICE\n") {
		t.Errorf("drop-in con Let's Encrypt:\n%s", conf)
	}
}

// TestDesinstalarConPaquete: con el paquete, desinstalar detiene y deshabilita el servicio y quita el drop-in, pero deja
// el programa, la unidad y el envoltorio (los quita apt o dnf, y lo dice); con borrarDatos borra los datos y la cuenta.
func TestDesinstalarConPaquete(t *testing.T) {
	p := nuevaPrueba(t)
	p.paquete(t)
	p.instalar(t)
	p.ordenes = nil
	if err := Desinstalar(false); !errors.Is(err, ErrQuitarPaquete) {
		t.Fatalf("avisa de que el paquete lo quitan apt o dnf: %v", err)
	}
	mismas(t, "desinstalar", p.ordenes, "systemctl disable --now tramtai", "systemctl daemon-reload")
	p.sinTocarPaquete(t)
	if existe(p.r.UnidadConf) || !existe(filepath.Join(p.r.Datos, "datos", "terminos.json")) {
		t.Error("el drop-in se quita y los datos se conservan")
	}

	p.instalar(t)
	if err := Desinstalar(true); !errors.Is(err, ErrQuitarPaquete) {
		t.Fatal(err)
	}
	p.sinTocarPaquete(t)
	if existe(p.r.Datos) || p.cuenta {
		t.Error("con borrarDatos se borran la carpeta y la cuenta")
	}
}

// conUnidad deja la unidad escrita, como tras instalar.
func (p *prueba) conUnidad(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(p.r.Unidad, []byte(Unidad(UnidadOpciones{})), 0o644); err != nil {
		t.Fatal(err)
	}
}

func leido(t *testing.T, ruta string) string {
	t.Helper()
	datos, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return string(datos)
}

// TestGuardarClaveSystemd: cada clave se cifra con systemd-creds (el valor por stdin, nunca en la orden) en
// credstore.encrypted, claves.conf carga todas con LoadCredentialEncrypted= y el servicio se reinicia para recibirlas.
// Borrar es simétrico; sin claves no queda claves.conf.
func TestGuardarClaveSystemd(t *testing.T) {
	p := nuevaPrueba(t)
	p.creds = "systemd 252 (252.39-1~deb12u2)\n+PAM +AUDIT\n"
	p.conUnidad(t)
	cifrada := func(nombre string) string { return filepath.Join(p.r.Cifradas, "tramtai."+nombre) }
	for _, c := range [][2]string{{"OPENAI_API_KEY", "sk-otra-456"}, {"ANTHROPIC_API_KEY", "sk-prueba-123"}} {
		p.ordenes, p.entradas = nil, nil
		if err := GuardarClave(c[0], c[1]); err != nil {
			t.Fatal(err)
		}
		mismas(t, "órdenes de "+c[0], p.ordenes, "systemd-creds --version",
			"systemd-creds encrypt --name=tramtai."+c[0]+" - "+cifrada(c[0]), "systemctl daemon-reload", "systemctl restart tramtai")
		mismas(t, "stdin de "+c[0], p.entradas, c[1])
		if info, err := os.Stat(cifrada(c[0])); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("la clave cifrada queda 0600: %v %v", info, err)
		}
	}
	conf := leido(t, p.r.ClavesConf)
	esperado := cabeceraClaves + "[Service]\n" +
		"LoadCredentialEncrypted=tramtai.ANTHROPIC_API_KEY:" + cifrada("ANTHROPIC_API_KEY") + "\n" +
		"LoadCredentialEncrypted=tramtai.OPENAI_API_KEY:" + cifrada("OPENAI_API_KEY") + "\n"
	if conf != esperado {
		t.Errorf("claves.conf:\n%s\nesperado:\n%s", conf, esperado)
	}
	if existe(p.r.ClavesEnv) {
		t.Error("con systemd-creds no se escribe claves.env")
	}

	p.ordenes = nil
	if err := BorrarClave("ANTHROPIC_API_KEY"); err != nil {
		t.Fatal(err)
	}
	mismas(t, "borrar", p.ordenes, "systemctl daemon-reload", "systemctl restart tramtai")
	if existe(cifrada("ANTHROPIC_API_KEY")) || strings.Contains(leido(t, p.r.ClavesConf), "ANTHROPIC") {
		t.Error("la clave borrada no queda ni cifrada ni en claves.conf")
	}
	if err := BorrarClave("OPENAI_API_KEY"); err != nil {
		t.Fatal(err)
	}
	if existe(p.r.ClavesConf) || existe(filepath.Dir(p.r.ClavesConf)) {
		t.Error("sin claves no quedan claves.conf ni su carpeta")
	}

	p.ordenes = nil
	for _, mal := range [][2]string{{"CLAUDE_CODE_OAUTH_TOKEN", "sk-prueba-123"}, {"ANTHROPIC_API_KEY", "con espacio"}} {
		if err := GuardarClave(mal[0], mal[1]); err == nil {
			t.Errorf("%s %q debía rechazarse", mal[0], mal[1])
		}
	}
	if len(p.ordenes) > 0 {
		t.Errorf("lo rechazado no lanza nada: %q", p.ordenes)
	}
}

// TestGuardarClaveSinSystemdCreds: sin systemd-creds, o con uno anterior a la 250, las claves van a claves.env (0600,
// de root), que claves.conf carga con EnvironmentFile=, y GuardarClave avisa con ErrClaveEnClaro; servidor estado lo
// dice también.
func TestGuardarClaveSinSystemdCreds(t *testing.T) {
	for _, creds := range []string{"", "systemd 249 (249.11-0ubuntu3)\n"} {
		p := nuevaPrueba(t)
		p.creds = creds
		p.conUnidad(t)
		for _, c := range [][2]string{{"OPENAI_API_KEY", "sk-otra-456"}, {"ANTHROPIC_API_KEY", "sk-vieja-000"}, {"ANTHROPIC_API_KEY", "sk-prueba-123"}} {
			p.ordenes = nil
			if err := GuardarClave(c[0], c[1]); !errors.Is(err, ErrClaveEnClaro) {
				t.Fatalf("%q: se esperaba ErrClaveEnClaro: %v", creds, err)
			}
			mismas(t, "órdenes", p.ordenes, "systemd-creds --version", "systemctl daemon-reload", "systemctl restart tramtai")
		}
		if env := leido(t, p.r.ClavesEnv); env != "ANTHROPIC_API_KEY=sk-prueba-123\nOPENAI_API_KEY=sk-otra-456\n" {
			t.Errorf("claves.env:\n%s", env)
		}
		if info, err := os.Stat(p.r.ClavesEnv); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("claves.env debe ser 0600: %v %v", info, err)
		}
		if conf := leido(t, p.r.ClavesConf); conf != cabeceraClaves+"[Service]\nEnvironmentFile=-"+p.r.ClavesEnv+"\n" {
			t.Errorf("claves.conf:\n%s", conf)
		}
		ejecutar0 := ejecutar
		ejecutar = func(string, ...string) ([]byte, error) { return []byte("active\n"), nil }
		if e, err := Estado(context.Background()); err != nil || !e.ClavesEnClaro {
			t.Errorf("servidor estado avisa de las claves en claro: %+v %v", e, err)
		}
		ejecutar = ejecutar0
		for _, nombre := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
			if err := BorrarClave(nombre); err != nil {
				t.Fatal(err)
			}
		}
		if existe(p.r.ClavesEnv) || existe(p.r.ClavesConf) {
			t.Error("sin claves no quedan claves.env ni claves.conf")
		}
	}
}

// TestDesinstalarBorraClaves: las claves sobreviven a desinstalar sin borrar los datos; con borrarDatos se borran las
// cifradas, claves.conf y claves.env.
func TestDesinstalarBorraClaves(t *testing.T) {
	p := nuevaPrueba(t)
	p.instalar(t)
	p.creds = "systemd 255 (255.4-1ubuntu8)\n"
	if err := GuardarClave("ANTHROPIC_API_KEY", "sk-prueba-123"); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p.r.ClavesEnv), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.r.ClavesEnv, []byte("OPENAI_API_KEY=sk-otra-456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cifrada := filepath.Join(p.r.Cifradas, "tramtai.ANTHROPIC_API_KEY")
	if err := Desinstalar(false); err != nil {
		t.Fatal(err)
	}
	if !existe(cifrada) || !existe(p.r.ClavesConf) || !existe(p.r.ClavesEnv) {
		t.Fatal("sin borrar los datos, las claves se conservan para reinstalar")
	}
	p.instalar(t)
	if err := Desinstalar(true); err != nil {
		t.Fatal(err)
	}
	if existe(cifrada) || existe(p.r.ClavesConf) || existe(p.r.ClavesEnv) {
		t.Error("con borrarDatos se borran las claves")
	}
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

// TestOrdenEntrar: el inicio de sesión del CLI va como la cuenta del servicio con un entorno limpio (env -i: HOME, el
// PATH del servicio, TERM y LANG), sin depender de la política de sudo; sin sudo, lo dice.
func TestOrdenEntrar(t *testing.T) {
	anterior := buscarPrograma
	t.Cleanup(func() { buscarPrograma = anterior })
	buscarPrograma = func(nombre string) (string, error) { return "/usr/bin/" + nombre, nil }
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LANG", "es_CL.UTF-8")
	orden, err := OrdenEntrar("codex", "codex", []string{"login", "--device-auth"})
	quiere := "/usr/bin/sudo -u tramtai -H env -i HOME=" + Rutas().Datos +
		" PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin TERM=xterm-256color LANG=es_CL.UTF-8 codex login --device-auth"
	if err != nil || strings.Join(append([]string{orden.Path}, orden.Args[1:]...), " ") != quiere {
		t.Fatalf("%v\n%v\nse esperaba\n%s", err, orden, quiere)
	}
	buscarPrograma = func(nombre string) (string, error) { return "", &exec.Error{Name: nombre, Err: exec.ErrNotFound} }
	if _, err := OrdenEntrar("claude", "claude", []string{"auth", "login"}); err == nil || !strings.Contains(err.Error(), "sudo") {
		t.Fatalf("sin sudo: %v", err)
	}
}
