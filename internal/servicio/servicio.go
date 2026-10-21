// Package servicio es el servicio único de Tramtai: un proceso y un puerto con varias instancias,
// global y por proyecto, separadas por token, más los puntos de acceso con puerto o ruta propios.
package servicio

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/adaptadores"
	"tramtai/internal/api"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
	"tramtai/internal/version"
)

// Opciones configura el servicio.
type Opciones struct {
	Datos    string                // directorio de datos del usuario
	Config   *nucleo.Configuracion // configuración global (sin capas de proyecto)
	Registro map[string]adaptadores.Adaptador
	// Conexiones dice qué agentes están desconectados; Nuevo la crea desde la configuración si falta.
	Conexiones *adaptadores.Conexiones
	// Salud recuerda los agentes cuya última petición falló por autenticación; Nuevo la crea si falta.
	Salud     *adaptadores.Salud
	Semaforos planificador.Semaforos // compartidos por todas las instancias
	Host      string
	Puerto    int
	Hosts     []string
	Registros io.Writer // mensajes de operación; nil los descarta
	Web       fs.FS     // Web UI compilada (internal/recursos); nil la desactiva
	// PlazoDrenaje es lo que Drenar espera a los trabajos en curso antes de interrumpirlos.
	PlazoDrenaje time.Duration
	// EntradaLocal deja entrar en la Web UI sin token al mismo usuario del sistema (servicio.entrada_local).
	EntradaLocal bool
	// BuscarActualizaciones consulta cada hora si hay una versión nueva (servicio.buscar_actualizaciones).
	BuscarActualizaciones bool
	// PuertoConfigurado y PuertoAutomatico son servicio.puerto y servicio.puerto_automatico; Puerto es el que se
	// usa, que puede ser otro si el configurado estaba ocupado. Se cambian desde la Web UI (con s.mu).
	PuertoConfigurado int
	PuertoAutomatico  bool
	// Reiniciar lanza un servicio nuevo que espera a que este termine; nil = sin reinicio desde la Web UI.
	Reiniciar func() error
	// RaizConversor es la carpeta donde se prepara el conversor de archivos (documentos.RaizConversor; vacía: sin
	// conversor en el servicio, como en las pruebas que no lo usan).
	RaizConversor string
	// PrepararConversor prepara el conversor al iniciar si falta o es de otra versión (conversion.preparar_al_iniciar).
	PrepararConversor bool
	// UV es la ruta de uv; vacía, la del que viene con Tramtai (documentos.BuscarUV).
	UV string
}

// intervaloPurga es cada cuánto se aplica la retención a cada instancia activa.
const intervaloPurga = 5 * time.Minute

// Instancia es un ámbito lógico con configuración, token, cola, base de datos y cargas propios.
type Instancia struct {
	ID           string
	Ambito       string // global o proyecto
	Raiz         string
	Config       *nucleo.Configuracion
	Planificador *planificador.Planificador
	Cargas       *documentos.Cargas
	ArchivoToken string
	token        string
	almacen      *persistencia.Almacen
	rutas        http.Handler
	pararPurga   context.CancelFunc
	purgaParada  chan struct{}
	purgarYa     chan struct{} // pide al bucle de purga una vuelta ahora (por ejemplo, tras cambiar la retención)
}

// Servicio atiende todas las instancias en un único puerto; el token de cada petición elige la instancia.
type Servicio struct {
	opc        Opciones
	ID         string
	IniciadoEn time.Time
	tokenAdmin string
	handler    http.Handler
	admin      http.Handler

	muRecargar    sync.Mutex // una recarga a la vez: cada una lee la configuración y la aplica entera
	mu            sync.RWMutex
	muConfianza   sync.Mutex // proyectos_confiables se lee y se reescribe entero: una confianza a la vez
	instancias    map[string]*Instancia
	abriendo      map[string]*apertura // instancias que se están abriendo, fuera de mu
	cerrado       bool                 // Cerrar ya empezó: no se publican instancias nuevas
	puntos        map[string]*puntoActivo
	erroresPuntos []string
	descriptor    bool

	unaVezRegistro sync.Once
	espacios       *registro // todos los espacios conocidos; se carga al usarlo (registro())
	muSesiones     sync.Mutex
	sesiones       map[string]*sesion
	codigos        map[string]codigoEntrada

	detener    chan struct{}
	detenerUna sync.Once

	actualizaciones actualizacion.Comprobador
	muAvance        sync.Mutex
	avance          ProgresoActualizacion // la actualización en curso, para la Web UI
	ordenInstalar   chan bool             // en la fase listo: true instala ya, false cancela
	buscar          atomic.Bool

	fondo sync.WaitGroup // tareas en segundo plano que usan las bases (purgas): Cerrar las espera

	conversor *documentos.Preparacion // markitdown, compartido por todas las instancias; nil sin RaizUsuario
}

// apertura es una instancia que se está abriendo: quien la pida a la vez espera a listo y recibe la misma.
type apertura struct {
	listo chan struct{}
	inst  *Instancia
	err   error
}

// abrirBase abre la base de una instancia; las pruebas la sustituyen para simular una base lenta.
var abrirBase = persistencia.Abrir

// Nuevo crea el servicio sin instancias; Restaurar o Activar* las abren.
func Nuevo(opc Opciones) (*Servicio, error) {
	if opc.Registros == nil {
		opc.Registros = io.Discard
	}
	if opc.Salud == nil {
		opc.Salud = adaptadores.NuevaSalud()
	}
	if opc.Conexiones == nil {
		opc.Conexiones = adaptadores.NuevasConexiones(opc.Config, slices.Collect(maps.Keys(opc.Registro)))
	}
	token, err := LeerOCrearToken(filepath.Join(opc.Datos, "token-servicio"))
	if err != nil {
		return nil, err
	}
	s := &Servicio{opc: opc, ID: "servicio_" + aleatorio(8), IniciadoEn: time.Now(), tokenAdmin: token,
		instancias: map[string]*Instancia{}, abriendo: map[string]*apertura{}, puntos: map[string]*puntoActivo{}, detener: make(chan struct{}),
		sesiones: map[string]*sesion{}, codigos: map[string]codigoEntrada{}}
	s.buscar.Store(opc.BuscarActualizaciones)
	s.fondo.Go(func() { actualizacion.LimpiarDescargas(time.Now().Add(-10 * time.Minute)) }) // los instaladores que dejó la versión anterior
	s.admin = s.rutasAdmin()
	s.handler = api.Proteger(opc.Hosts, nil, http.HandlerFunc(s.enrutar))
	if opc.RaizConversor != "" {
		s.conversor = documentos.NuevaPreparacion(opc.RaizConversor, opc.Config.Texto("conversion.comando"), opc.UV)
		s.conversor.Registros = opc.Registros // cada preparación, también la que se pide a mano, queda anotada
		s.conversor.FijarAlIniciar(opc.PrepararConversor)
		if e := s.conversor.Estado(); opc.PrepararConversor && (e.Estado == "falta" || s.conversor.Desactualizado()) {
			s.conversor.Preparar()
		}
	}
	return s, nil
}

// fijarPrepararConversor guarda conversion.preparar_al_iniciar en la configuración global y lo muestra en el estado.
func (s *Servicio) fijarPrepararConversor(preparar bool) error {
	ruta, err := nucleo.RutaGlobal()
	if err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "conversion.preparar_al_iniciar", preparar); err != nil {
		return err
	}
	if s.conversor != nil {
		s.conversor.FijarAlIniciar(preparar)
	}
	return nil
}

// ServeHTTP implementa http.Handler.
func (s *Servicio) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.handler.ServeHTTP(w, r) }

// Base es la URL del puerto principal.
func (s *Servicio) Base() string {
	return "http://" + net.JoinHostPort(s.opc.Host, strconv.Itoa(s.opc.Puerto))
}

// Detenido se cierra cuando la administración pide detener el servicio.
func (s *Servicio) Detenido() <-chan struct{} { return s.detener }

// enrutar atiende la salud, la Web UI y su sesión, los puntos con ruta /p/<nombre>, la administración (token de
// servicio) y las instancias: su token, una sesión de la Web UI o el token de servicio con Tramtai-Instancia.
func (s *Servicio) enrutar(w http.ResponseWriter, r *http.Request) {
	ruta := r.URL.Path
	switch {
	case ruta == "/healthz":
		api.EscribirJSON(w, http.StatusOK, map[string]any{"estado": "ok", "servicio": s.ID, "version": version.Version})
		return
	case strings.HasPrefix(ruta, "/p/"):
		s.enrutarPunto(w, r)
		return
	case strings.HasPrefix(ruta, "/ui/"):
		s.rutasUI(w, r)
		return
	case s.esSitio(r):
		s.sitio(w, r)
		return
	}
	admin := api.TokenValido(r, s.tokenAdmin)
	if strings.HasPrefix(ruta, "/api/v1/servicio") {
		if !admin {
			api.NoAutorizado(w, r)
			return
		}
		s.admin.ServeHTTP(w, r)
		return
	}
	inst, porSesion, err := s.instanciaPara(r, admin)
	switch {
	case err != nil:
		api.EscribirError(w, r, err)
		return
	case inst == nil && admin:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("indica un espacio activo en la cabecera Tramtai-Instancia")))
		return
	case inst == nil:
		api.NoAutorizado(w, r)
		return
	}
	if s.rutaInstancia(w, r, inst, admin || porSesion) {
		return
	}
	inst.rutas.ServeHTTP(w, r.WithContext(nucleo.ConOrigenPeticion(r.Context(), origenPeticion(r, porSesion, mismoUsuarioLocal(r)))))
}

func (s *Servicio) enrutarPunto(w http.ResponseWriter, r *http.Request) {
	nombre, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/p/"), "/")
	s.mu.RLock()
	p := s.puntos[nombre]
	s.mu.RUnlock()
	if p == nil || p.handler == nil || p.cfg.Puerto != 0 {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("punto de acceso no disponible: %s", nombre)))
		return
	}
	http.StripPrefix("/p/"+nombre, p.handler).ServeHTTP(w, r)
}

// instanciaPara elige la instancia: con el token de servicio, la de Tramtai-Instancia; con otro token, la suya;
// sin token, la de la sesión de la Web UI, que exige X-CSRF-Token en las peticiones que modifican. porSesion
// indica una sesión de la Web UI.
func (s *Servicio) instanciaPara(r *http.Request, admin bool) (inst *Instancia, porSesion bool, err error) {
	if admin {
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.instancias[r.Header.Get("Tramtai-Instancia")], false, nil
	}
	if token := api.TokenDe(r); token != "" {
		return s.instanciaPorToken(token), false, nil
	}
	_, ses := s.sesionDe(r)
	if ses == nil {
		return nil, false, nil
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && !csrfValido(r, ses) {
		return nil, false, nucleo.NuevoError(nucleo.CodigoCSRFInvalido, idioma.T("falta la cabecera X-CSRF-Token de la sesión"))
	}
	s.mu.RLock()
	inst = s.instancias[ses.instancia]
	s.mu.RUnlock()
	if inst == nil {
		return nil, false, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("el espacio de la sesión (%s) no está activo", ses.instancia))
	}
	return inst, true, nil
}

// origenPeticion dice de dónde llegó una petición a la instancia: la sesión de la Web UI es Probar; con la clave,
// X-Tramtai-Cliente distingue la terminal y la CLI y lo demás es una aplicación. La etiqueta solo vale desde este
// equipo y con el mismo usuario que el servicio (local): decide si se admite Antigravity, y una app remota no puede
// atribuírsela. Un programa local del mismo usuario podría, pero ese ya puede ejecutar los CLI por su cuenta.
func origenPeticion(r *http.Request, porSesion, local bool) string {
	if porSesion {
		return nucleo.DesdeProbar
	}
	switch c := r.Header.Get("X-Tramtai-Cliente"); c {
	case nucleo.DesdeTerminal, nucleo.DesdeCLI:
		if local {
			return c
		}
	}
	return nucleo.DesdeApp
}

// Drenar deja de aceptar trabajos en todas las instancias y espera hasta PlazoDrenaje a que terminen los que
// están en ejecución; los que sigan después se interrumpen. Las instancias siguen abiertas, así que los clientes
// pueden recoger los resultados hasta que se llame a Cerrar.
func (s *Servicio) Drenar() {
	_ = s.decidirInstalacion(false) // detener el servicio en la cuenta atrás de una actualización la cancela
	s.mu.RLock()
	instancias := make([]*Instancia, 0, len(s.instancias))
	enCurso := 0
	for _, inst := range s.instancias {
		instancias = append(instancias, inst)
		enCurso += inst.Planificador.EnCurso()
	}
	s.mu.RUnlock()
	if enCurso > 0 {
		fmt.Fprintf(s.opc.Registros, "esperando hasta %s a %d trabajo(s) en curso…\n", s.opc.PlazoDrenaje, enCurso)
	}
	var grupo sync.WaitGroup
	for _, inst := range instancias {
		grupo.Go(func() { inst.Planificador.Drenar(s.opc.PlazoDrenaje) })
	}
	grupo.Wait()
}

// Cerrar detiene todos los puntos de acceso y todas las instancias. Lo que estaba en cola se reanuda al volver
// a activar la instancia.
func (s *Servicio) Cerrar() {
	if s.conversor != nil {
		s.conversor.Cerrar() // uv no sobrevive al servicio
	}
	s.mu.Lock()
	s.cerrado = true
	s.mu.Unlock()
	s.fondo.Wait() // una purga en curso termina antes de cerrar su base
	s.mu.Lock()
	puntos, instancias := s.puntos, s.instancias
	s.puntos, s.instancias = map[string]*puntoActivo{}, map[string]*Instancia{}
	s.mu.Unlock()
	for _, p := range puntos {
		p.cerrar()
	}
	for _, inst := range instancias {
		inst.cerrar(false)
	}
}
