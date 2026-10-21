package servicio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Espacio es lo que el registro recuerda de un espacio de trabajo, también si está detenido o si su carpeta ya no
// existe. El id no cambia aunque el proyecto se renombre o se mueva.
type Espacio struct {
	ID          string    `json:"id"`
	Ambito      string    `json:"ambito"`
	Ruta        string    `json:"ruta,omitempty"`
	Nombre      string    `json:"nombre"`
	Activo      bool      `json:"activo"`
	UltimoUso   time.Time `json:"ultimo_uso,omitzero"`
	UltimoError string    `json:"ultimo_error,omitempty"`
}

// registro guarda en datos/espacios.json todos los espacios conocidos. Reemplaza a instancias.json (solo los activos),
// que se migra la primera vez.
type registro struct {
	mu        sync.Mutex
	muGuardar sync.Mutex // la foto y la escritura van juntas: una foto vieja no se escribe después de una nueva
	ruta      string
	espacios  map[string]*Espacio
	aviso     error // al cargar: el registro estaba dañado o no se pudo leer
	// soloLectura: el registro existe pero no se pudo leer; no se reescribe, para no perder lo que guarda.
	soloLectura bool
}

type archivoRegistro struct {
	Version  int       `json:"version"`
	Espacios []Espacio `json:"espacios"`
}

// idValido son los ids que puede tener un espacio: los que calcula IDProyecto (con sufijo -N si colisionan) y global.
var idValido = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// registro carga el registro la primera vez que se necesita: así lo ve igual quien restaura al arrancar y quien
// activa un espacio directamente.
func (s *Servicio) registro() *registro {
	s.unaVezRegistro.Do(func() { s.espacios = cargarRegistro(s.opc.Datos) })
	return s.espacios
}

func cargarRegistro(datos string) *registro {
	r := &registro{ruta: filepath.Join(datos, "espacios.json"), espacios: map[string]*Espacio{}}
	migrado := false
	contenido, err := os.ReadFile(r.ruta)
	switch {
	case err == nil:
		var a archivoRegistro
		if err := json.Unmarshal(contenido, &a); err != nil {
			r.aviso = apartarDanado(r.ruta, err, "espacios.json.dañado")
		}
		for _, e := range a.Espacios {
			if idValido.MatchString(e.ID) {
				e := e
				r.espacios[e.ID] = &e
			}
		}
	case os.IsNotExist(err):
		migrado = r.migrar(filepath.Join(datos, "instancias.json"))
	default:
		r.soloLectura = true
		r.aviso = nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida,
			idioma.Tf("%s no se pudo leer (%s): se abre solo el espacio global y no se reescribirá hasta reiniciar", r.ruta, err.Error()))
	}
	if _, ok := r.espacios["global"]; !ok {
		// Sin registro previo (o dañado) la global se abre, como siempre.
		r.espacios["global"] = &Espacio{ID: "global", Ambito: "global", Activo: !migrado}
	}
	r.espacios["global"].Nombre = "Global"
	// Los datos de un proyecto que el registro no conoce (movido antes de la v0.54) quedan a la vista como «no
	// encontrado», para reasociarlos u olvidarlos.
	bases, _ := filepath.Glob(filepath.Join(datos, "instancias", "*.db"))
	for _, base := range bases {
		id := strings.TrimSuffix(filepath.Base(base), ".db")
		if _, ok := r.espacios[id]; !ok && idValido.MatchString(id) {
			r.espacios[id] = &Espacio{ID: id, Ambito: "proyecto", Nombre: id}
		}
	}
	return r
}

// migrar pasa instancias.json al registro y lo borra; devuelve si había uno legible.
func (r *registro) migrar(ruta string) bool {
	contenido, err := os.ReadFile(ruta)
	if err != nil {
		return false
	}
	var a activas
	if err := json.Unmarshal(contenido, &a); err != nil {
		r.aviso = apartarDanado(ruta, err, "instancias.json.dañado")
		return false
	}
	r.espacios["global"] = &Espacio{ID: "global", Ambito: "global", Activo: a.Global}
	global, _ := nucleo.CargarConfiguracion("")
	for _, raiz := range a.Proyectos {
		c, err := nucleo.CargarConfiguracion(raiz)
		if err != nil {
			c = global // la carpeta ya no está: el id sale del nombre de la carpeta, como hacía IDProyecto
		}
		id := IDProyecto(raiz, c)
		r.espacios[id] = &Espacio{ID: id, Ambito: "proyecto", Ruta: raiz, Nombre: nombreProyecto(raiz, c), Activo: true}
	}
	if err := r.guardar(); err == nil {
		_ = os.Remove(ruta)
	}
	return true
}

// apartarDanado renombra un registro ilegible (para poder mirarlo) y devuelve el aviso.
func apartarDanado(ruta string, err error, apartado string) error {
	apartado += time.Now().Format("-20060102-150405") // otra copia dañada después no pisa esta
	texto := idioma.Tf("%s no se pudo leer (%s): se apartó como %s y se abre solo el espacio global", ruta, err.Error(), apartado)
	if errApartar := os.Rename(ruta, filepath.Join(filepath.Dir(ruta), apartado)); errApartar != nil {
		texto = idioma.Tf("%s no se pudo leer (%s) ni apartar (%s): se abre solo el espacio global y se reescribirá", ruta, err.Error(), errApartar.Error())
	}
	return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, texto)
}

// guardar escribe el registro de forma atómica. Quien lo llama no tiene r.mu.
func (r *registro) guardar() error {
	if r.soloLectura {
		return nil
	}
	r.muGuardar.Lock()
	defer r.muGuardar.Unlock()
	return escribirJSONArchivo(r.ruta, archivoRegistro{Version: 1, Espacios: r.lista()})
}

// lista devuelve una copia: la global primero y después por nombre.
func (r *registro) lista() []Espacio {
	r.mu.Lock()
	lista := make([]Espacio, 0, len(r.espacios))
	for _, e := range r.espacios {
		lista = append(lista, *e)
	}
	r.mu.Unlock()
	sort.Slice(lista, func(i, j int) bool {
		if (lista[i].Ambito == "global") != (lista[j].Ambito == "global") {
			return lista[i].Ambito == "global"
		}
		if lista[i].Nombre != lista[j].Nombre {
			return strings.ToLower(lista[i].Nombre) < strings.ToLower(lista[j].Nombre)
		}
		return lista[i].ID < lista[j].ID
	})
	return lista
}

func (r *registro) buscar(id string) (Espacio, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.espacios[id]; ok {
		return *e, true
	}
	return Espacio{}, false
}

// poner guarda los datos de un espacio conservando su último uso.
func (r *registro) poner(e Espacio) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if previo, ok := r.espacios[e.ID]; ok && e.UltimoUso.IsZero() {
		e.UltimoUso = previo.UltimoUso
	}
	r.espacios[e.ID] = &e
}

func (r *registro) quitar(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.espacios, id)
}

// cambiar aplica f al espacio id, si existe.
func (r *registro) cambiar(id string, f func(e *Espacio)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.espacios[id]; ok {
		f(e)
	}
}

// marcarUso anota que el espacio se acaba de usar: una sesión nueva vuelve al último.
func (s *Servicio) marcarUso(id string) {
	r := s.registro()
	r.cambiar(id, func(e *Espacio) { e.UltimoUso = time.Now().UTC() })
	if err := r.guardar(); err != nil {
		fmt.Fprintf(s.opc.Registros, "no se pudo guardar el registro de espacios: %s\n", err)
	}
}

// idDe resuelve el id de la carpeta raiz: el de su marca si es de un proyecto registrado (y la carpeta se movió, o
// es la misma), el del espacio registrado con esa ruta, o uno nuevo. Una marca ajena (global, un id que no está, o
// el de un proyecto que sigue en su sitio: una copia) no se adopta.
func (s *Servicio) idDe(raiz string, c *nucleo.Configuracion) (string, error) {
	r := s.registro()
	r.mu.Lock()
	defer r.mu.Unlock()
	id, adoptar := idPara(r.espacios, raiz, c)
	if adoptar {
		r.espacios[id].Ruta = raiz
	}
	return id, nil
}

// idPara es la regla de idDe sobre los espacios conocidos; adoptar dice si la marca se adopta (su ruta pasa a raiz).
func idPara(espacios map[string]*Espacio, raiz string, c *nucleo.Configuracion) (id string, adoptar bool) {
	if marca := LeerMarca(raiz); marca != "" && marca != "global" {
		if e, ok := espacios[marca]; ok && e.Ambito == "proyecto" {
			if e.Ruta == "" || MismaRuta(e.Ruta, raiz) || !esProyecto(e.Ruta) {
				return marca, true
			}
		}
	}
	for _, e := range espacios {
		if e.Ambito == "proyecto" && e.Ruta != "" && MismaRuta(e.Ruta, raiz) {
			return e.ID, false
		}
	}
	base := IDProyecto(raiz, c)
	id = base
	for n := 2; ; n++ {
		e, ocupado := espacios[id]
		if !ocupado || e.Ruta == "" {
			return id, false
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

// rutaMovida devuelve dónde estaba antes el proyecto de raiz si es uno registrado que se movió (su marca es la de un
// espacio cuya carpeta ya no está), o "".
func (s *Servicio) rutaMovida(raiz string) string {
	marca := LeerMarca(raiz)
	e, ok := s.registro().buscar(marca)
	if !ok || e.Ambito != "proyecto" || e.Ruta == "" || MismaRuta(e.Ruta, raiz) || esProyecto(e.Ruta) {
		return ""
	}
	return e.Ruta
}

// nombreProyecto es el nombre que se muestra de un proyecto: el de su configuración o el de su carpeta.
func nombreProyecto(raiz string, c *nucleo.Configuracion) string {
	if nombre := strings.TrimSpace(c.Texto("proyecto.nombre")); nombre != "" {
		return nombre
	}
	return filepath.Base(raiz)
}

// mismaCarpeta compara dos rutas también por su forma real (sin nombres cortos como ADMINI~1 ni enlaces).
func mismaCarpeta(a, b string) bool {
	if MismaRuta(a, b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && MismaRuta(ra, rb)
}

// MismaRuta compara dos rutas como lo hace IDProyecto: sin distinguir mayúsculas en Windows y macOS.
func MismaRuta(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS != "linux" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func esProyecto(raiz string) bool {
	_, err := os.Stat(filepath.Join(raiz, ".tramtai", "config.yaml"))
	return err == nil
}

// LeerMarca devuelve el id guardado en .tramtai/espacio, o "" si no hay o no es un id.
func LeerMarca(raiz string) string {
	datos, err := os.ReadFile(filepath.Join(raiz, ".tramtai", "espacio"))
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(string(datos))
	if !idValido.MatchString(id) {
		return ""
	}
	return id
}

// escribirMarca deja el id en .tramtai/espacio y se asegura de que el .gitignore de .tramtai no lo versione.
func escribirMarca(raiz, id string) error {
	dir := filepath.Join(raiz, ".tramtai")
	if LeerMarca(raiz) != id {
		if err := os.WriteFile(filepath.Join(dir, "espacio"), []byte(id+"\n"), 0o644); err != nil {
			return err
		}
	}
	ignorar := filepath.Join(dir, ".gitignore")
	actual, err := os.ReadFile(ignorar)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for linea := range strings.SplitSeq(string(actual), "\n") {
		if strings.TrimSpace(linea) == "espacio" {
			return nil
		}
	}
	texto := string(actual)
	if texto == "" {
		texto = gitignoreProyecto
	} else {
		if !strings.HasSuffix(texto, "\n") {
			texto += "\n"
		}
		texto += "espacio\n"
	}
	return os.WriteFile(ignorar, []byte(texto), 0o644)
}

// IDDeCarpeta es el id del espacio de un proyecto visto desde fuera del servicio (CLI, terminal), con la misma regla
// que el servicio sobre el registro de datos (solo lo lee): una marca copiada en otra carpeta no se adopta.
func IDDeCarpeta(datos, raiz string, c *nucleo.Configuracion) string {
	espacios := map[string]*Espacio{}
	if contenido, err := os.ReadFile(filepath.Join(datos, "espacios.json")); err == nil {
		var a archivoRegistro
		if json.Unmarshal(contenido, &a) == nil {
			for _, e := range a.Espacios {
				e := e
				espacios[e.ID] = &e
			}
		}
	}
	id, _ := idPara(espacios, raiz, c)
	return id
}

// gitignoreProyecto es el .gitignore de .tramtai: lo propio de cada máquina no se versiona.
const gitignoreProyecto = "# configuración propia de cada máquina\nlocal.yaml\nespacio\n"
