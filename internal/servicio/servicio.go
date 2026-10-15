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
	// BuscarActualizaciones consulta una vez al día si hay una versión nueva (servicio.buscar_actualizaciones).
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
}

// Servicio atiende todas las instancias en un único puerto; el token de cada petición elige la instancia.
type Servicio struct {
	opc        Opciones
	ID         string
	IniciadoEn time.Time
	tokenAdmin string
	handler    http.Handler
	admin      http.Handler

	mu            sync.RWMutex
	instancias    map[string]*Instancia
	puntos        map[string]*puntoActivo
	erroresPuntos []string
	descriptor    bool

	muSesiones sync.Mutex
	sesiones   map[string]*sesion
	codigos    map[string]codigoEntrada

	detener    chan struct{}
	detenerUna sync.Once

	actualizaciones actualizacion.Comprobador
	buscar          atomic.Bool
}

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
		instancias: map[string]*Instancia{}, puntos: map[string]*puntoActivo{}, detener: make(chan struct{}),
		sesiones: map[string]*sesion{}, codigos: map[string]codigoEntrada{}}
	s.buscar.Store(opc.BuscarActualizaciones)
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
	inst.rutas.ServeHTTP(w, r)
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

func (s *Servicio) activar(id, ambito, raiz string, c *nucleo.Configuracion) (*Instancia, error) {
	s.mu.Lock()
	if inst, activa := s.instancias[id]; activa {
		s.mu.Unlock()
		return inst, nil
	}
	inst, err := s.abrirInstancia(id, ambito, raiz, c)
	if err == nil {
		s.instancias[id] = inst
	}
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	s.Recargar() // abre los puntos de acceso de la instancia
	return inst, nil
}

func (s *Servicio) abrirInstancia(id, ambito, raiz string, c *nucleo.Configuracion) (*Instancia, error) {
	if err := os.MkdirAll(filepath.Join(s.opc.Datos, "instancias"), 0o700); err != nil {
		return nil, err
	}
	alm, err := persistencia.Abrir(filepath.Join(s.opc.Datos, "instancias", id+".db"))
	if err != nil {
		return nil, err
	}
	if horas := c.Entero("retencion.horas_idempotencia"); horas > 0 {
		alm.VentanaIdempotencia = time.Duration(horas) * time.Hour
	}
	cargas, err := documentos.AbrirCargas(filepath.Join(s.opc.Datos, "cargas", id),
		time.Duration(c.Entero("retencion.horas_carga_huerfana"))*time.Hour, int64(c.Entero("conversion.maximo_carga_mb"))<<20)
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
	srv := &api.Servidor{Planificador: p, Cargas: cargas, Registro: s.opc.Registro, Config: c}
	inst := &Instancia{ID: id, Ambito: ambito, Raiz: raiz, Config: c, Planificador: p, Cargas: cargas,
		ArchivoToken: archivo, token: token, almacen: alm, rutas: srv.Rutas()}
	inst.iniciarPurga(s.opc.Registros)
	return inst, nil
}

// iniciarPurga aplica la retención al abrir la instancia y después cada intervaloPurga.
func (i *Instancia) iniciarPurga(registros io.Writer) {
	ctx, parar := context.WithCancel(context.Background())
	i.pararPurga, i.purgaParada = parar, make(chan struct{})
	go func() {
		defer close(i.purgaParada)
		reloj := time.NewTicker(intervaloPurga)
		defer reloj.Stop()
		for {
			if err := i.Purgar(ctx); err != nil && ctx.Err() == nil {
				fmt.Fprintf(registros, "purga de %s: %s\n", i.ID, nucleo.MensajeDe(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-reloj.C:
			}
		}
	}()
}

// Purgar aplica la retención de la instancia: vence resultados y entradas, borra metadatos antiguos y las
// cargas vencidas que ningún trabajo necesita.
func (i *Instancia) Purgar(ctx context.Context) error {
	horas := func(clave string) time.Duration { return time.Duration(i.Config.Entero(clave)) * time.Hour }
	r := persistencia.Retencion{Resultado: horas("retencion.horas_resultado"), Metadatos: horas("retencion.horas_metadatos"),
		EliminarEntradas: i.Config.Booleano("retencion.eliminar_entradas_al_terminar")}
	if _, _, err := i.almacen.Purgar(ctx, i.ID, r); err != nil {
		return err
	}
	enUso, err := i.almacen.CargasEnUso(ctx, i.ID)
	if err != nil {
		return err
	}
	_, err = i.Cargas.Purgar(enUso, time.Now())
	return err
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
	i.pararPurga()
	<-i.purgaParada
	if vaciarCola {
		ctx := context.Background()
		if ids, err := i.almacen.ListarPorEstado(ctx, i.ID, nucleo.EnCola); err == nil {
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
	if datos, err := os.ReadFile(filepath.Join(s.opc.Datos, "instancias.json")); err == nil {
		_ = json.Unmarshal(datos, &a)
	}
	var errores []error
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

// EstadoInstancia resume una instancia para la CLI y la TUI.
type EstadoInstancia struct {
	ID           string         `json:"id"`
	Ambito       string         `json:"ambito"`
	Raiz         string         `json:"raiz,omitempty"`
	ArchivoToken string         `json:"archivo_token"`
	Agente       string         `json:"agente_predeterminado"`
	Perfil       string         `json:"perfil_predeterminado,omitempty"`
	Trabajos     map[string]int `json:"trabajos"`
}

// Estado es la vista del servicio que devuelve GET /api/v1/servicio.
type Estado struct {
	Identificador string            `json:"identificador_servicio"`
	Version       string            `json:"version"`
	PID           int               `json:"pid"`
	IniciadoEn    time.Time         `json:"iniciado_en"`
	Base          string            `json:"base"`
	Instancias    []EstadoInstancia `json:"instancias"`
	Puntos        []EstadoPunto     `json:"puntos_acceso"`
	Errores       []string          `json:"errores_configuracion,omitempty"`
	// Actualizacion es la versión publicada más nueva que la actual, si la hay (la bandeja la ofrece).
	Actualizacion *actualizacion.Version `json:"actualizacion,omitempty"`
}

// Estado devuelve el estado actual.
func (s *Servicio) Estado() Estado {
	var nueva *actualizacion.Version
	if v, ok := s.versionNueva(context.Background(), false); ok {
		nueva = &v
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := Estado{Identificador: s.ID, Version: version.Texto(), PID: os.Getpid(), IniciadoEn: s.IniciadoEn, Base: s.Base(),
		Instancias: []EstadoInstancia{}, Puntos: s.estadoPuntos(), Errores: s.erroresPuntos, Actualizacion: nueva}
	for _, inst := range s.instancias {
		e.Instancias = append(e.Instancias, inst.estado())
	}
	sort.Slice(e.Instancias, func(i, j int) bool { return e.Instancias[i].ID < e.Instancias[j].ID })
	return e
}

func (i *Instancia) estado() EstadoInstancia {
	conteo := map[string]int{}
	if porEstado, err := i.Planificador.Contar(context.Background()); err == nil {
		for estado, n := range porEstado {
			conteo[string(estado)] = n
		}
	}
	return EstadoInstancia{ID: i.ID, Ambito: i.Ambito, Raiz: i.Raiz, ArchivoToken: i.ArchivoToken,
		Agente: i.Config.Texto("predeterminados.agente"), Perfil: i.Config.Texto("predeterminados.perfil_modelo"), Trabajos: conteo}
}

// Descriptor identifica el servicio en marcha.
type Descriptor struct {
	Identificador string           `json:"identificador_servicio"`
	PID           int              `json:"pid"`
	FirmaInicio   string           `json:"firma_inicio_proceso"`
	IniciadoEn    time.Time        `json:"iniciado_en"`
	Host          string           `json:"host"`
	Puerto        int              `json:"puerto"`
	Base          string           `json:"base"`
	Version       string           `json:"version"`
	Instancias    []string         `json:"instancias_activas"`
	Puntos        []map[string]any `json:"puntos_acceso"`
}

// RutaDescriptor es el archivo del descriptor en el directorio de datos.
func RutaDescriptor(datos string) string { return filepath.Join(datos, "servicio.json") }

// LeerDescriptor lee el descriptor; no comprueba que el servicio siga vivo (lo hace el cliente con /healthz).
func LeerDescriptor(datos string) (Descriptor, error) {
	var d Descriptor
	contenido, err := os.ReadFile(RutaDescriptor(datos))
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(contenido, &d)
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
	temporal := ruta + ".tmp"
	if err := os.WriteFile(temporal, datos, 0o600); err != nil {
		return err
	}
	return os.Rename(temporal, ruta)
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
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return "", err
	}
	token := "tt_" + aleatorio(32)
	return token, os.WriteFile(ruta, []byte(token+"\n"), 0o600)
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
