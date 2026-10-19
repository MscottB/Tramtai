// Package servicio es el servicio único de Tramtai: un proceso y un puerto con varias instancias,
// global y por proyecto, separadas por token, más los puntos de acceso con puerto o ruta propios.
package servicio

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	Datos     string                // directorio de datos del usuario
	Config    *nucleo.Configuracion // configuración global (sin capas de proyecto)
	Registro  map[string]adaptadores.Adaptador
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
	instancias    map[string]*Instancia
	abriendo      map[string]*apertura // instancias que se están abriendo, fuera de mu
	cerrado       bool                 // Cerrar ya empezó: no se publican instancias nuevas
	puntos        map[string]*puntoActivo
	erroresPuntos []string
	descriptor    bool

	muSesiones sync.Mutex
	sesiones   map[string]*sesion
	codigos    map[string]codigoEntrada

	detener    chan struct{}
	detenerUna sync.Once

	actualizaciones actualizacion.Comprobador
	muAvance        sync.Mutex
	avance          ProgresoActualizacion // la actualización en curso, para la Web UI
	ordenInstalar   chan bool             // en la fase listo: true instala ya, false cancela
	buscar          atomic.Bool

	fondo sync.WaitGroup // tareas en segundo plano que usan las bases (purgas): Cerrar las espera
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
	return s, nil
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
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.T("indica una instancia activa en la cabecera Tramtai-Instancia")))
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
		return nil, false, nucleo.NuevoError(nucleo.CodigoOrigenNoPermitido, idioma.T("falta la cabecera X-CSRF-Token de la sesión"))
	}
	s.mu.RLock()
	inst = s.instancias[ses.instancia]
	s.mu.RUnlock()
	if inst == nil {
		return nil, false, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("la instancia de la sesión (%s) no está activa", ses.instancia))
	}
	return inst, true, nil
}

// RotarTokenInstancia genera la clave nueva de una instancia y la aplica en marcha: la anterior deja de valer al
// momento, sin reiniciar. Una clave que llega de una variable de entorno se cambia en esa variable.
func (s *Servicio) RotarTokenInstancia(inst *Instancia) (string, error) {
	if strings.HasPrefix(inst.ArchivoToken, "variable ") {
		return "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("la clave de %s viene de la %s: cámbiala allí", inst.ID, inst.ArchivoToken))
	}
	// El archivo y la memoria cambian juntos: dos rotaciones a la vez no pueden dejarlos distintos.
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := RotarToken(inst.ArchivoToken)
	if err != nil {
		return "", err
	}
	inst.token = token
	return token, nil
}

// tokenDe lee la clave de una instancia, que puede cambiar en marcha (RotarTokenInstancia).
func (s *Servicio) tokenDe(inst *Instancia) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return inst.token
}

// instanciaPorToken compara el token con el de todas las instancias, sin revelar por tiempos cuál existe.
func (s *Servicio) instanciaPorToken(token string) *Instancia {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var elegida *Instancia
	for _, inst := range s.instancias {
		if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(inst.token)) == 1 {
			elegida = inst
		}
	}
	return elegida
}

// IDProyecto deriva un identificador estable del nombre del proyecto y de su ruta.
func IDProyecto(raiz string, c *nucleo.Configuracion) string {
	nombre := c.Texto("proyecto.nombre")
	if nombre == "" {
		nombre = filepath.Base(raiz)
	}
	var b strings.Builder
	for _, r := range strings.ToLower(nombre) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteByte('_')
		}
	}
	slug := strings.Trim(b.String(), "_")
	if len(slug) > 30 {
		slug = slug[:30]
	}
	if slug == "" {
		slug = "proyecto"
	}
	clave := filepath.Clean(raiz)
	if runtime.GOOS != "linux" { // Windows y macOS no distinguen mayúsculas en las rutas; Linux sí
		clave = strings.ToLower(clave)
	}
	suma := sha256.Sum256([]byte(clave))
	return slug + "_" + hex.EncodeToString(suma[:4])
}

// ActivarGlobal activa la instancia global con la configuración global.
func (s *Servicio) ActivarGlobal() (*Instancia, error) {
	return s.activar("global", "global", "", s.opc.Config)
}

// ActivarProyecto activa la instancia del proyecto cuya raíz contiene .tramtai/config.yaml.
func (s *Servicio) ActivarProyecto(raiz string) (*Instancia, error) {
	raiz, err := filepath.Abs(raiz)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(raiz, ".tramtai", "config.yaml")); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s no tiene .tramtai/config.yaml: ejecuta tramtai iniciar en el proyecto", raiz))
	}
	c, err := nucleo.CargarConfiguracion(raiz)
	if err != nil {
		return nil, err
	}
	return s.activar(IDProyecto(raiz, c), "proyecto", raiz, c)
}

// activar abre la instancia si no está activa. La base se abre (migraciones, huérfanos, recuperación) fuera de mu: así
// no se detienen las peticiones de las demás instancias, y quien pida la misma a la vez espera y recibe la misma.
func (s *Servicio) activar(id, ambito, raiz string, c *nucleo.Configuracion) (*Instancia, error) {
	s.mu.Lock()
	if inst, activa := s.instancias[id]; activa {
		s.mu.Unlock()
		return inst, nil
	}
	if a, enCurso := s.abriendo[id]; enCurso {
		s.mu.Unlock()
		<-a.listo
		return a.inst, a.err
	}
	a := &apertura{listo: make(chan struct{})}
	s.abriendo[id] = a
	s.mu.Unlock()

	a.inst, a.err = s.abrirInstancia(id, ambito, raiz, c)
	var sobra *Instancia // abierta mientras el servicio se cerraba: se cierra fuera de mu
	s.mu.Lock()
	delete(s.abriendo, id)
	if a.err == nil && s.cerrado {
		sobra = a.inst
		a.inst, a.err = nil, nucleo.NuevoError(nucleo.CodigoServicioDetenido, idioma.T("el servicio se está deteniendo"))
	}
	if a.err == nil {
		s.instancias[id] = a.inst
	}
	s.mu.Unlock()
	if sobra != nil {
		sobra.cerrar(false)
	}
	close(a.listo)
	if a.err != nil {
		return nil, a.err
	}
	s.Recargar() // abre los puntos de acceso de la instancia
	return a.inst, nil
}

func (s *Servicio) abrirInstancia(id, ambito, raiz string, c *nucleo.Configuracion) (*Instancia, error) {
	if err := os.MkdirAll(filepath.Join(s.opc.Datos, "instancias"), 0o700); err != nil {
		return nil, err
	}
	alm, err := abrirBase(filepath.Join(s.opc.Datos, "instancias", id+".db"))
	if err != nil {
		return nil, err
	}
	if alm.Compactada {
		fmt.Fprintf(s.opc.Registros, "se compactó la base de %s: desde ahora devuelve al disco lo que libera la purga\n", id)
	}
	if horas := c.Entero("retencion.horas_idempotencia"); horas > 0 {
		alm.VentanaIdempotencia = time.Duration(horas) * time.Hour
	}
	cargas, err := documentos.AbrirCargas(filepath.Join(s.opc.Datos, "cargas", id),
		time.Duration(c.Entero("retencion.horas_carga_huerfana"))*time.Hour, int64(c.Limite("conversion.maximo_carga_mb"))<<20,
		int64(c.Limite("conversion.cuota_cargas_mb"))<<20)
	if err != nil {
		alm.Cerrar()
		return nil, err
	}
	token, archivo, err := TokenInstancia(s.opc.Datos, id, c)
	if err != nil {
		alm.Cerrar()
		return nil, err
	}
	opc := planificador.OpcionesDesde(c, id)
	opc.Registros = s.opc.Registros
	if c.ProyectoConfiable(raiz) {
		opc.RaizConfiable = raiz
	}
	opc.Preparador = documentos.Preparador{Cargas: cargas, Conversor: documentos.Desde(c)}
	p := planificador.Nuevo(alm, s.opc.Registro, s.opc.Semaforos, opc)
	if err := p.Iniciar(context.Background()); err != nil {
		alm.Cerrar()
		return nil, err
	}
	srv := &api.Servidor{Planificador: p, Cargas: cargas, Registro: s.opc.Registro, Config: c,
		PuntosDeGrupo: func(grupo string) []string {
			var nombres []string
			for _, punto := range s.puntosDe(id) {
				if punto.Grupo == grupo {
					nombres = append(nombres, punto.Nombre)
				}
			}
			return nombres
		}}
	inst := &Instancia{ID: id, Ambito: ambito, Raiz: raiz, Config: c, Planificador: p, Cargas: cargas,
		ArchivoToken: archivo, token: token, almacen: alm, rutas: srv.Rutas()}
	inst.iniciarPurga(s.opc.Registros)
	return inst, nil
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

// iniciarPurga aplica la retención al abrir la instancia y después cada intervaloPurga.
func (i *Instancia) iniciarPurga(registros io.Writer) {
	ctx, parar := context.WithCancel(context.Background())
	i.pararPurga, i.purgaParada, i.purgarYa = parar, make(chan struct{}), make(chan struct{}, 1)
	go func() {
		defer close(i.purgaParada)
		reloj := time.NewTicker(intervaloPurga)
		defer reloj.Stop()
		avisado := false
		for {
			if err := i.Purgar(ctx); err != nil && ctx.Err() == nil {
				fmt.Fprintf(registros, "purga de %s: %s\n", i.ID, nucleo.MensajeDe(err))
			}
			if r, m, x, ajustada := i.retencion(); ajustada && !avisado {
				fmt.Fprintf(registros, "retencion de %s fuera de rango; se usa horas_resultado %d, horas_metadatos %d y max_trabajos %d\n", i.ID, r, m, x)
				avisado = true
			}
			select {
			case <-ctx.Done():
				return
			case <-reloj.C:
			case <-i.purgarYa:
			}
		}
	}()
}

// purgarPronto pide una purga sin esperarla. La hace el bucle de la instancia, así que cerrar (Desactivar, Cerrar)
// la espera y nunca corre sobre una base cerrada.
func (i *Instancia) purgarPronto() {
	select {
	case i.purgarYa <- struct{}{}:
	default: // ya había una pedida, o la instancia no tiene bucle de purga
	}
}

// Purgar aplica la retención de la instancia: vence resultados y entradas, borra metadatos antiguos y las
// cargas vencidas que ningún trabajo necesita.
func (i *Instancia) Purgar(ctx context.Context) error {
	return PurgarBase(ctx, i.almacen, i.Cargas, i.Config, i.ID)
}

// PurgarBase aplica la retención de c a las instancias de una base y a sus cargas. La usan cada instancia del servicio
// y tramtai ejecutar --directo, cuya base guarda una instancia por ejecución.
func PurgarBase(ctx context.Context, alm *persistencia.Almacen, cargas *documentos.Cargas, c *nucleo.Configuracion, instancias ...string) error {
	resultado, metadatos, maximo, _ := nucleo.AjustarRetencion(c.Entero("retencion.horas_resultado"), c.Entero("retencion.horas_metadatos"),
		c.Entero("retencion.max_trabajos"))
	r := persistencia.Retencion{Resultado: time.Duration(resultado) * time.Hour, Metadatos: time.Duration(metadatos) * time.Hour,
		Maximo: maximo, EliminarEntradas: c.Booleano("retencion.eliminar_entradas_al_terminar")}
	enUso := map[string]bool{}
	for _, id := range instancias {
		if _, _, err := alm.Purgar(ctx, id, r); err != nil {
			return err
		}
		usadas, err := alm.CargasEnUso(ctx, id)
		if err != nil {
			return err
		}
		maps.Copy(enUso, usadas)
	}
	_, err := cargas.Purgar(enUso, time.Now())
	return err
}

// retencion da los plazos (horas) y el tope de la instancia, llevados a su rango; ajustada indica que la
// configuración traía algún valor fuera de él.
func (i *Instancia) retencion() (resultado, metadatos, maximo int, ajustada bool) {
	return nucleo.AjustarRetencion(i.Config.Entero("retencion.horas_resultado"), i.Config.Entero("retencion.horas_metadatos"),
		i.Config.Entero("retencion.max_trabajos"))
}

// Desactivar detiene una instancia sin afectar a las demás: vacía su cola (cancela lo que no empezó), lo que
// estaba en ejecución queda interrumpido y se cierran sus puntos de acceso.
func (s *Servicio) Desactivar(id string) error {
	s.mu.Lock()
	inst, activa := s.instancias[id]
	delete(s.instancias, id)
	s.mu.Unlock()
	if !activa {
		return nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("la instancia %s no está activa", id))
	}
	s.Recargar()
	inst.cerrar(true)
	return nil
}

// cerrar detiene la purga y el planificador (interrumpe lo que aún se ejecuta) y cierra la base.
func (i *Instancia) cerrar(vaciarCola bool) {
	if i.pararPurga != nil {
		i.pararPurga()
		<-i.purgaParada
	}
	if vaciarCola {
		ctx := context.Background()
		// Esperando al agente también es cola: el trabajo aún no tiene plaza ni agente.
		if ids, err := i.almacen.ListarPorEstado(ctx, i.ID, nucleo.EnCola, nucleo.EsperandoAgente); err == nil {
			for _, id := range ids {
				_, _ = i.Planificador.Cancelar(ctx, id)
			}
		}
	}
	i.Planificador.Detener()
	i.almacen.Cerrar()
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

// activas es la lista de instancias que se restauran al reiniciar el servicio.
type activas struct {
	Global    bool     `json:"global"`
	Proyectos []string `json:"proyectos"`
}

func (s *Servicio) guardarActivas() error {
	s.mu.RLock()
	a := activas{Proyectos: []string{}}
	for _, inst := range s.instancias {
		if inst.Ambito == "global" {
			a.Global = true
		} else {
			a.Proyectos = append(a.Proyectos, inst.Raiz)
		}
	}
	s.mu.RUnlock()
	sort.Strings(a.Proyectos)
	return escribirJSONArchivo(filepath.Join(s.opc.Datos, "instancias.json"), a)
}

// Restaurar activa las instancias que estaban activas (la global, si no hay registro previo) y devuelve los
// errores de las que no pudieron abrirse, por ejemplo un proyecto movido.
func (s *Servicio) Restaurar() []error {
	a := activas{Global: true}
	ruta := filepath.Join(s.opc.Datos, "instancias.json")
	var errores []error
	if datos, err := os.ReadFile(ruta); err == nil {
		if err := json.Unmarshal(datos, &a); err != nil {
			// Dañado: se aparta (para poder mirarlo) y se abre la global, como sin registro previo.
			a = activas{Global: true}
			texto := idioma.Tf("%s no se pudo leer (%s): se apartó como instancias.json.dañado y se abre solo la instancia global", ruta, err.Error())
			if errApartar := os.Rename(ruta, ruta+".dañado"); errApartar != nil {
				texto = idioma.Tf("%s no se pudo leer (%s) ni apartar (%s): se abre solo la instancia global y se reescribirá", ruta, err.Error(), errApartar.Error())
			}
			errores = append(errores, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, texto))
		}
	}
	if a.Global {
		if _, err := s.ActivarGlobal(); err != nil {
			errores = append(errores, err)
		}
	}
	for _, raiz := range a.Proyectos {
		if _, err := s.ActivarProyecto(raiz); err != nil {
			errores = append(errores, err)
		}
	}
	return errores
}

// Estado devuelve el estado actual. Las bases se consultan sin s.mu: un Lock pendiente bloquearía todas las peticiones.
func (s *Servicio) Estado(ctx context.Context) Estado {
	var nueva *actualizacion.Version
	if v, ok := s.versionNueva(context.Background(), false); ok {
		nueva = &v
	}
	s.mu.RLock()
	e := Estado{Identificador: s.ID, Version: version.Texto(), PID: os.Getpid(), IniciadoEn: s.IniciadoEn, Base: s.Base(),
		Instancias: []EstadoInstancia{}, Puntos: s.estadoPuntos(), Errores: s.erroresPuntos, Actualizacion: nueva}
	instancias := make([]*Instancia, 0, len(s.instancias))
	for _, inst := range s.instancias {
		instancias = append(instancias, inst)
	}
	s.mu.RUnlock()
	for _, inst := range instancias {
		e.Instancias = append(e.Instancias, inst.estado(ctx))
	}
	sort.Slice(e.Instancias, func(i, j int) bool { return e.Instancias[i].ID < e.Instancias[j].ID })
	return e
}

// estado resume la instancia; el conteo de trabajos se abandona a los 2 s para no colgar a quien pregunta.
func (i *Instancia) estado(ctx context.Context) EstadoInstancia {
	ctx, cancelar := context.WithTimeout(ctx, 2*time.Second)
	defer cancelar()
	conteo := map[string]int{}
	if porEstado, err := i.Planificador.Contar(ctx); err == nil {
		for estado, n := range porEstado {
			conteo[string(estado)] = n
		}
	}
	return EstadoInstancia{ID: i.ID, Ambito: i.Ambito, Raiz: i.Raiz, ArchivoToken: i.ArchivoToken,
		Agente: i.Config.Texto("predeterminados.agente"), Perfil: i.Config.Texto("predeterminados.perfil_modelo"), Trabajos: conteo}
}

// EscribirDescriptor publica el descriptor y lo mantiene actualizado al cambiar instancias o puntos.
func (s *Servicio) EscribirDescriptor() error {
	s.mu.Lock()
	s.descriptor = true
	s.mu.Unlock()
	return s.actualizarDescriptor()
}

func (s *Servicio) actualizarDescriptor() error {
	s.mu.RLock()
	if !s.descriptor {
		s.mu.RUnlock()
		return nil
	}
	d := Descriptor{Identificador: s.ID, PID: os.Getpid(), FirmaInicio: strconv.Itoa(os.Getpid()) + "@" + s.IniciadoEn.UTC().Format(time.RFC3339Nano),
		IniciadoEn: s.IniciadoEn, Host: s.opc.Host, Puerto: s.opc.Puerto, Base: s.Base(), Version: version.Version,
		Instancias: []string{}, Puntos: []map[string]any{}}
	for id := range s.instancias {
		d.Instancias = append(d.Instancias, id)
	}
	sort.Strings(d.Instancias)
	for _, p := range s.estadoPuntos() {
		if p.Activo {
			d.Puntos = append(d.Puntos, map[string]any{"nombre": p.Nombre, "url": p.URL})
		}
	}
	s.mu.RUnlock()
	return escribirJSONArchivo(RutaDescriptor(s.opc.Datos), d)
}

// BorrarDescriptor elimina el descriptor si sigue siendo el de este servicio.
func (s *Servicio) BorrarDescriptor() {
	if d, err := LeerDescriptor(s.opc.Datos); err == nil && d.Identificador == s.ID {
		_ = os.Remove(RutaDescriptor(s.opc.Datos))
	}
}

// escribirJSONArchivo escribe de forma atómica (temporal y renombrado) con permisos del usuario.
func escribirJSONArchivo(ruta string, v any) error {
	datos, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return escribirAtomico(ruta, datos, 0o600)
}

// escritores serializa, dentro del proceso, las escrituras de cada ruta: en Windows dos renombrados a la vez sobre el
// mismo archivo pueden fallar.
var escritores sync.Map // ruta → *sync.Mutex

// escribirAtomico escribe datos en un temporal propio de la misma carpeta y lo renombra: quien lee a la vez (un Recargar,
// la CLI, la TUI) ve el archivo anterior o el nuevo, nunca uno a medias, y un corte a mitad no deja el archivo truncado.
func escribirAtomico(ruta string, datos []byte, permisos os.FileMode) error {
	if real, err := filepath.EvalSymlinks(ruta); err == nil {
		ruta = real // un config.yaml enlazado (dotfiles) sigue siendo un enlace: se escribe el archivo al que apunta
	}
	m, _ := escritores.LoadOrStore(filepath.Clean(ruta), &sync.Mutex{})
	m.(*sync.Mutex).Lock()
	defer m.(*sync.Mutex).Unlock()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(ruta), filepath.Base(ruta)+".*.tmp")
	if err != nil {
		return err
	}
	temporal := f.Name()
	_, err = f.Write(datos)
	if err == nil {
		err = f.Sync()
	}
	if cerrar := f.Close(); err == nil {
		err = cerrar
	}
	if err == nil {
		err = os.Chmod(temporal, permisos)
	}
	if err == nil {
		err = renombrar(temporal, ruta)
	}
	if err != nil {
		_ = os.Remove(temporal)
	}
	return err
}

// renombrar reintenta en Windows durante unos 600 ms: allí Rename falla mientras otro proceso tiene abierto el destino
// (un Recargar que lo lee, la CLI, la terminal o el antivirus). En los demás sistemas no hace falta.
func renombrar(origen, destino string) error {
	err := os.Rename(origen, destino)
	for espera := 5 * time.Millisecond; err != nil && runtime.GOOS == "windows" && espera <= 320*time.Millisecond; espera *= 2 {
		time.Sleep(espera)
		err = os.Rename(origen, destino)
	}
	return err
}

// LeerOCrearToken lee el token del archivo o crea uno aleatorio (permisos 0600, fuera del repositorio).
func LeerOCrearToken(ruta string) (string, error) {
	if datos, err := os.ReadFile(ruta); err == nil {
		if token := strings.TrimSpace(string(datos)); len(token) >= 32 {
			return token, nil
		}
	}
	return RotarToken(ruta)
}

// RotarToken genera un token nuevo y lo guarda; el anterior deja de valer al recargar.
func RotarToken(ruta string) (string, error) {
	token := "tt_" + aleatorio(32)
	return token, guardarSecreto(ruta, token)
}

// guardarSecreto escribe un token o su huella solo legible por el usuario, de forma atómica: quien lo lee a la vez (un
// Recargar, la CLI) nunca ve un archivo a medias que tomaría por vacío y regeneraría.
func guardarSecreto(ruta, valor string) error {
	return escribirAtomico(ruta, []byte(valor+"\n"), 0o600)
}

// RotarTokenPunto genera la clave nueva de un punto de acceso y guarda solo su huella: la clave se muestra una vez
// y no se puede recuperar; la anterior deja de valer al recargar.
func RotarTokenPunto(ruta string) (string, error) {
	token := "tt_" + aleatorio(32)
	return token, guardarSecreto(ruta, api.HuellaToken(token))
}

// LeerHuellaPunto devuelve la huella de la clave de un punto. Un archivo antiguo con la clave en claro se convierte
// a huella y las aplicaciones siguen funcionando; sin archivo se crea una clave que nadie conoce, así que hay que
// regenerarla para usar el punto.
func LeerHuellaPunto(ruta string) (string, error) {
	if datos, err := os.ReadFile(ruta); err == nil {
		contenido := strings.TrimSpace(string(datos))
		if strings.HasPrefix(contenido, api.PrefijoHuella) {
			return contenido, nil
		}
		if len(contenido) >= 32 {
			huella := api.HuellaToken(contenido)
			return huella, guardarSecreto(ruta, huella)
		}
	}
	token, err := RotarTokenPunto(ruta)
	return api.HuellaToken(token), err
}

// EliminarTokenPunto borra la clave de un punto eliminado: otro con el mismo nombre no hereda la anterior.
func EliminarTokenPunto(datos, nombre string) {
	_ = os.Remove(RutaTokenPunto(datos, nombre))
}

// RutaTokenInstancia es el archivo del token de una instancia (el global conserva su ruta histórica).
func RutaTokenInstancia(datos, id string) string {
	if id == "global" {
		return filepath.Join(datos, "token-global")
	}
	return filepath.Join(datos, "tokens", id)
}

// RutaTokenPunto es el archivo del token de un punto de acceso.
func RutaTokenPunto(datos, nombre string) string {
	return filepath.Join(datos, "tokens", "acceso_"+nombre)
}

// TokenInstancia devuelve el token de la instancia y su origen; el global puede venir de una variable de entorno.
func TokenInstancia(datos, id string, c *nucleo.Configuracion) (valor, origen string, err error) {
	if id == "global" {
		if variable := c.Texto("instancia_global.token_variable_entorno"); variable != "" {
			if v := os.Getenv(variable); v != "" {
				return v, "variable " + variable, nil
			}
		}
	}
	ruta := RutaTokenInstancia(datos, id)
	valor, err = LeerOCrearToken(ruta)
	return valor, ruta, err
}

func aleatorio(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
