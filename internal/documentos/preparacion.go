package documentos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"tramtai/internal/carpetas"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/procesos"
)

// plazoPreparacion acota una preparación: una descarga colgada (un proxy que no responde) termina en error.
const plazoPreparacion = 30 * time.Minute

// vigenciaUbicacion es lo que vale el conversor localizado antes de volver a mirar el disco: el estado se consulta en
// cada sondeo y en cada petición con archivos.
const vigenciaUbicacion = 10 * time.Second

// htmlComprobacion es lo que convierte el conversor recién preparado antes de darlo por listo.
const htmlComprobacion = "<html><body><p>Tramtai comprueba</p></body></html>"

// RaizConversor es la carpeta donde se prepara el conversor: la de la portable si lo es; si no, la local del usuario
// (%LOCALAPPDATA%\tramtai en Windows, para no llenar un perfil móvil con ~300 MB de Python; $XDG_DATA_HOME o
// ~/.local/share/tramtai en Linux, fuera de ~/.config) y la de Tramtai en macOS.
func RaizConversor() (string, error) {
	if carpetas.Portable() || runtime.GOOS == "darwin" {
		return carpetas.RaizUsuario()
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "tramtai"), nil
		}
		return carpetas.RaizUsuario()
	}
	if datos := os.Getenv("XDG_DATA_HOME"); datos != "" {
		return filepath.Join(datos, "tramtai"), nil
	}
	casa, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(casa, ".local", "share", "tramtai"), nil
}

// Preparacion prepara markitdown en <raiz>/conversor con el uv incluido y lleva su estado. Hay una por servicio:
// todas las instancias la comparten.
type Preparacion struct {
	raiz, comando, uv string
	plazo             time.Duration
	// Registros recibe el inicio, el fin y el error de cada preparación (nil: no se anotan).
	Registros io.Writer

	base     context.Context // se cancela al cerrar el servicio: uv no lo sobrevive
	cancelar context.CancelFunc
	// uso lo leen las conversiones en curso y lo escribe el reemplazo: no se mueve la carpeta de un python vivo.
	uso sync.RWMutex

	mu        sync.Mutex
	corriendo bool
	hecho     chan struct{} // se cierra al terminar la preparación en curso
	etapa     string
	progreso  int
	error     string // de la última preparación
	alIniciar bool   // conversion.preparar_al_iniciar, para mostrarlo con el estado
	ubicacion Ubicacion
	hay       bool
	visto     time.Time // cuándo se localizó; cero: hay que volver a mirar
}

// NuevaPreparacion recoge lo que dejó una preparación cortada (conversor.nuevo a medias, conversor.viejo de un
// reemplazo sin terminar). uv vacío: se busca al preparar (BuscarUV).
func NuevaPreparacion(raiz, comando, uv string) *Preparacion {
	final := filepath.Join(raiz, "conversor")
	if _, err := os.Stat(final); os.IsNotExist(err) {
		_ = os.Rename(final+".viejo", final)
	}
	_ = os.RemoveAll(final + ".nuevo")
	_ = os.RemoveAll(final + ".viejo")
	base, cancelar := context.WithCancel(context.Background())
	return &Preparacion{raiz: raiz, comando: comando, uv: uv, plazo: plazoPreparacion, base: base, cancelar: cancelar}
}

// Cerrar corta la preparación en curso (al detener el servicio).
func (p *Preparacion) Cerrar() { p.cancelar() }

// FijarAlIniciar guarda si el servicio prepara el conversor solo al iniciar (el estado lo muestra).
func (p *Preparacion) FijarAlIniciar(preparar bool) {
	p.mu.Lock()
	p.alIniciar = preparar
	p.mu.Unlock()
}

// Estado es el del conversor al momento: listo (también mientras se prepara uno nuevo: sigue sirviendo el anterior, y
// la etapa y el progreso dicen que se está preparando; con el error de la última preparación si falló), preparando,
// error o falta.
func (p *Preparacion) Estado() contrato.EstadoConversor {
	p.mu.Lock()
	defer p.mu.Unlock()
	e := p.estado()
	e.PrepararAlIniciar = p.alIniciar
	return e
}

// hayPaquete dice si hay paquete del conversor para este sistema: publicar.yml solo lo publica para Windows x64.
const hayPaquete = runtime.GOOS == "windows" && runtime.GOARCH == "amd64"

func (p *Preparacion) estado() contrato.EstadoConversor {
	u, ok := p.localizar()
	switch {
	case ok && p.corriendo:
		return contrato.EstadoConversor{Estado: "listo", Origen: string(u.Origen), Version: u.Version, Etapa: p.etapa, Progreso: p.progreso}
	case p.corriendo:
		return contrato.EstadoConversor{Estado: "preparando", Etapa: p.etapa, Progreso: p.progreso}
	case ok:
		return contrato.EstadoConversor{Estado: "listo", Origen: string(u.Origen), Version: u.Version, Mensaje: p.error}
	case p.error != "":
		return contrato.EstadoConversor{Estado: "error", Mensaje: p.error, Paquete: hayPaquete}
	}
	return contrato.EstadoConversor{Estado: "falta"}
}

// localizar es Localizar con una vigencia corta (con p.mu tomado).
func (p *Preparacion) localizar() (Ubicacion, bool) {
	if time.Since(p.visto) > vigenciaUbicacion {
		p.ubicacion, p.hay = Localizar(p.comando, p.raiz)
		p.visto = time.Now()
	}
	return p.ubicacion, p.hay
}

// Desactualizado indica que el conversor preparado es de otro lock: una versión nueva de Tramtai cambió markitdown.
func (p *Preparacion) Desactualizado() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	u, ok := p.localizar()
	return ok && u.Origen == OrigenPreparado && u.Lock != HuellaLock()
}

// Preparar lanza la preparación en segundo plano si no hay una en curso y devuelve el estado.
func (p *Preparacion) Preparar() contrato.EstadoConversor {
	p.mu.Lock()
	if !p.corriendo {
		p.corriendo, p.hecho, p.etapa, p.progreso = true, make(chan struct{}), "python", 0
		go p.correr(p.hecho)
	}
	p.mu.Unlock()
	return p.Estado()
}

// Esperar espera a que termine la preparación en curso (o a ctx) y devuelve el estado.
func (p *Preparacion) Esperar(ctx context.Context) contrato.EstadoConversor {
	p.mu.Lock()
	hecho := p.hecho
	p.mu.Unlock()
	if hecho != nil {
		select {
		case <-hecho:
		case <-ctx.Done():
		}
	}
	return p.Estado()
}

func (p *Preparacion) anotar(formato string, args ...any) {
	if p.Registros != nil {
		fmt.Fprintf(p.Registros, formato+"\n", args...)
	}
}

func (p *Preparacion) correr(hecho chan struct{}) {
	ctx, cancelar := context.WithTimeout(p.base, p.plazo)
	defer cancelar()
	p.anotar("preparando el conversor de archivos (markitdown) en segundo plano")
	final := filepath.Join(p.raiz, "conversor")
	uv := p.uv
	if uv == "" {
		uv, _ = BuscarUV()
	}
	err := PrepararEn(ctx, final+".nuevo", uv, func(etapa string, progreso int) {
		p.mu.Lock()
		p.etapa, p.progreso = etapa, progreso
		p.mu.Unlock()
	})
	if err == nil {
		p.uso.Lock() // espera a las conversiones en curso: no se mueve la carpeta de un python vivo
		err = reemplazar(final+".nuevo", final)
		p.uso.Unlock()
	}
	_ = os.RemoveAll(final + ".nuevo")
	p.mu.Lock()
	p.corriendo, p.error, p.visto = false, "", time.Time{}
	if err != nil {
		p.error = err.Error()
	}
	p.mu.Unlock()
	if err != nil {
		p.anotar("no se pudo preparar el conversor de archivos: %s", err)
	} else {
		p.anotar("conversor de archivos listo")
	}
	close(hecho)
}

// reemplazar pone nuevo en el lugar de final; en Windows un antivirus puede retener la carpeta unos instantes.
func reemplazar(nuevo, final string) error {
	viejo := final + ".viejo"
	_ = os.RemoveAll(viejo)
	if _, err := os.Stat(final); err == nil {
		if err := renombrar(final, viejo); err != nil {
			return err
		}
	}
	if err := renombrar(nuevo, final); err != nil {
		_ = renombrar(viejo, final)
		return err
	}
	_ = os.RemoveAll(viejo)
	return nil
}

func renombrar(origen, destino string) error {
	var err error
	for intento := 0; intento < 5; intento++ {
		if err = os.Rename(origen, destino); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return errors.New(idioma.Tf("no se pudo reemplazar el conversor: %s", err.Error()))
}

// BuscarUV encuentra el uv que viene con Tramtai: junto al ejecutable, en ../lib/tramtai (paquetes de Linux, Homebrew
// e instalar.sh) o en el PATH.
func BuscarUV() (string, error) {
	nombre := "uv"
	if runtime.GOOS == "windows" {
		nombre = "uv.exe"
	}
	if dir := dirEjecutable(); dir != "" {
		for _, ruta := range []string{filepath.Join(dir, nombre), filepath.Join(dir, "..", "lib", "tramtai", nombre)} {
			if info, err := os.Stat(ruta); err == nil && info.Mode().IsRegular() {
				return ruta, nil
			}
		}
	}
	if ruta, err := exec.LookPath("uv"); err == nil {
		return ruta, nil
	}
	if runtime.GOOS == "windows" {
		return "", errors.New(idioma.T("no se encontró uv junto a Tramtai: reinstala Tramtai o descarga el paquete del conversor"))
	}
	return "", errors.New(idioma.T("no se encontró uv junto a Tramtai: reinstala Tramtai (el paquete o instalar.sh lo traen)"))
}

// PrepararEn deja en destino un Python independiente con markitdown y el lock fijado, comprueba que convierte y
// escribe listo.json. Sin venv: la carpeta se puede mover (paquete del conversor, portable en otro equipo). Solo usa
// una carpeta vacía o un conversor anterior: nunca borra una carpeta con otras cosas.
func PrepararEn(ctx context.Context, destino, uv string, avance func(etapa string, progreso int)) error {
	if entradas, err := os.ReadDir(destino); err == nil && len(entradas) > 0 {
		if _, err := os.Stat(filepath.Join(destino, "listo.json")); err != nil {
			return errors.New(idioma.Tf("%s no está vacía ni es un conversor: elige una carpeta nueva", destino))
		}
	}
	if uv == "" {
		_, err := BuscarUV()
		return err
	}
	if err := os.RemoveAll(destino); err != nil {
		return err
	}
	if err := os.MkdirAll(destino, 0o700); err != nil {
		return err
	}
	trabajo, err := os.MkdirTemp("", "tramtai-conversor-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(trabajo)
	descarga := filepath.Join(destino, "descarga")
	entorno := append(entornoConversor(), "UV_NO_CONFIG=1", "UV_NATIVE_TLS=1",
		"UV_PYTHON_INSTALL_DIR="+descarga, "UV_CACHE_DIR="+filepath.Join(trabajo, "cache"))

	avance("python", 0)
	args := []string{"python", "install", "3.12", "--install-dir", descarga, "--no-bin"}
	if runtime.GOOS == "windows" {
		args = append(args, "--no-registry")
	}
	if err := ejecutarUV(ctx, uv, args, trabajo, entorno); err != nil {
		return err
	}
	// La carpeta con la versión completa; uv deja además un acceso cpython-3.12-… por versión menor.
	instalados, _ := filepath.Glob(filepath.Join(descarga, "cpython-3.12.*"))
	if len(instalados) != 1 {
		return errors.New(idioma.T("uv no dejó un Python 3.12 en la carpeta del conversor"))
	}
	version := strings.Split(filepath.Base(instalados[0]), "-")[1]
	if err := os.Rename(instalados[0], filepath.Join(destino, "python")); err != nil {
		return err
	}
	_ = os.RemoveAll(descarga)

	avance("paquetes", 40)
	lock := filepath.Join(trabajo, "conversor.lock")
	if err := os.WriteFile(lock, LockConversor, 0o600); err != nil {
		return err
	}
	if err := ejecutarUV(ctx, uv, []string{"pip", "install", "--python", interprete(destino), "--break-system-packages",
		"--require-hashes", "--no-deps", "-r", lock}, trabajo, entorno); err != nil {
		return err
	}

	avance("comprobacion", 90)
	html := filepath.Join(trabajo, "comprobacion.html")
	if err := os.WriteFile(html, []byte(htmlComprobacion), 0o600); err != nil {
		return err
	}
	var salida strings.Builder
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: interprete(destino), Argumentos: []string{"-m", "markitdown", html},
		Directorio: trabajo, Entorno: entornoConversor()}, func(l string) error { salida.WriteString(l + "\n"); return nil })
	if ctx.Err() != nil {
		return errorDePlazo(ctx)
	}
	if err == nil && (r.CodigoSalida != 0 || !strings.Contains(salida.String(), "Tramtai comprueba")) {
		err = errors.New(idioma.Tf("el conversor preparado no convierte: %s", ultimaLinea(r.Errores+"\n"+salida.String())))
	}
	if err != nil {
		return err
	}
	datos, _ := json.MarshalIndent(Listo{Markitdown: "0.1.8", LockSHA256: HuellaLock(), Python: version, PreparadoEn: time.Now().UTC()}, "", "  ")
	if err := os.WriteFile(filepath.Join(destino, "listo.json"), datos, 0o600); err != nil {
		return err
	}
	avance("comprobacion", 100)
	return nil
}

// errorDePlazo dice por qué se cortó la preparación: venció su plazo o se cerró el servicio.
func errorDePlazo(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return errors.New(idioma.T("la preparación del conversor superó su plazo (¿red o proxy que no responde?)"))
	}
	return errors.New(idioma.T("la preparación del conversor se cortó al cerrar Tramtai"))
}

// ejecutarUV corre uv supervisado; su error es la última línea de stderr, o el plazo vencido.
func ejecutarUV(ctx context.Context, uv string, args []string, dir string, entorno []string) error {
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: uv, Argumentos: args, Directorio: dir, Entorno: entorno},
		func(string) error { return nil })
	if ctx.Err() != nil {
		return errorDePlazo(ctx)
	}
	if err != nil {
		return err
	}
	if r.CodigoSalida != 0 {
		return errors.New(idioma.Tf("uv terminó con código %d: %s", r.CodigoSalida, ultimaLinea(r.Errores)))
	}
	return nil
}
