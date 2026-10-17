// Package planificador admite trabajos, aplica colas y límites y los ejecuta con los adaptadores.
package planificador

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/procesos"
	"tramtai/internal/salidas"
)

// Preparador convierte las cargas de un trabajo en texto para el agente (documentos.Preparador).
type Preparador interface {
	Preparar(ctx context.Context, cargas []string) (nucleo.Documentos, error)
}

// Opciones son los límites de una instancia.
type Opciones struct {
	Instancia string
	// MaximoCola son los trabajos aceptados sin empezar; al llenarse se rechaza con cola_llena (HTTP 429).
	MaximoCola int
	// MaximoTrabajos son los trabajos simultáneos de la instancia.
	MaximoTrabajos int
	// TiempoMaximoCola es el plazo en cola, distinto del de ejecución; 0 lo desactiva.
	TiempoMaximoCola time.Duration
	// TiempoLimitePredeterminado se aplica si la solicitud no fija tiempo_limite_segundos.
	TiempoLimitePredeterminado time.Duration
	// Perfiles traduce perfil_modelo a un identificador por agente (perfil -> agente -> modelo).
	Perfiles map[string]map[string]string
	// MaximoConversiones limita las conversiones simultáneas, aparte de las ejecuciones.
	MaximoConversiones int
	// Preparador convierte las cargas; sin él, un trabajo con cargas falla con entrada_no_disponible.
	Preparador Preparador
	// RaizConfiable es la raíz del proyecto si figura en proyectos_confiables; vacía = solo modo aislado.
	RaizConfiable string
	// Registros recibe una línea al aceptar y al terminar cada trabajo: la terminal del servicio en primer
	// plano o servicio.log en segundo plano. nil = sin registro.
	Registros io.Writer
	// Contextos es limites.contexto_tokens («agente» o «agente/modelo» → tokens) y ReservaSalida los tokens que se
	// dejan para la respuesta al comprobar si la entrada cabe.
	Contextos     map[string]int
	ReservaSalida int
	// PermitirAntigravityApps es adaptadores.antigravity.permitir_apps: Antigravity también para apps y APIs.
	PermitirAntigravityApps bool
}

// OpcionesDesde toma los límites de la configuración efectiva.
func OpcionesDesde(c *nucleo.Configuracion, instancia string) Opciones {
	return Opciones{
		Instancia:                  instancia,
		MaximoCola:                 c.Entero("concurrencia.maximo_tamano_cola"),
		MaximoTrabajos:             c.Entero("concurrencia.maximo_trabajos_instancia"),
		TiempoMaximoCola:           time.Duration(c.Entero("concurrencia.tiempo_maximo_en_cola_segundos")) * time.Second,
		TiempoLimitePredeterminado: 300 * time.Second,
		Perfiles:                   c.Perfiles(),
		MaximoConversiones:         c.Entero("concurrencia.maximo_conversiones_instancia"),
		Contextos:                  c.Contextos(),
		PermitirAntigravityApps:    c.Booleano("adaptadores.antigravity.permitir_apps"),
		ReservaSalida:              c.Entero("limites.reserva_salida_tokens"),
	}
}

// Semaforos limita las ejecuciones simultáneas de cada adaptador en todo el servicio, porque todas
// las instancias comparten la sesión del CLI.
type Semaforos map[string]chan struct{}

// NuevosSemaforos crea un semáforo por adaptador con su límite (mínimo 1).
func NuevosSemaforos(limites map[string]int) Semaforos {
	s := Semaforos{}
	for nombre, limite := range limites {
		s[nombre] = make(chan struct{}, max(limite, 1))
	}
	return s
}

// Planificador ejecuta los trabajos de una instancia.
type Planificador struct {
	opc         Opciones
	almacen     *persistencia.Almacen
	adaptadores map[string]adaptadores.Adaptador
	semaforos   Semaforos
	// conversiones es el semáforo de conversiones de la instancia.
	conversiones chan struct{}
	// ranuras limita los trabajos de la instancia en ejecución (MaximoTrabajos).
	ranuras chan struct{}

	mu            sync.Mutex
	cancelaciones map[string]context.CancelFunc
	// semPuntos limita las ejecuciones simultáneas de cada punto de acceso, dentro de las de la instancia.
	semPuntos map[string]chan struct{}
	// presupuestos son los vigentes por API y de la instancia (ver FijarPresupuestos).
	presupuestos atomic.Pointer[map[string]nucleo.Presupuesto]
	cola         chan string
	// ctx gobierna la admisión y la toma de trabajos de la cola; ejec, las ejecuciones en curso. Separarlos
	// permite el drenaje: se deja de aceptar sin cortar lo que ya se ejecuta.
	ctx    context.Context
	parar  context.CancelFunc
	ejec   context.Context
	cortar context.CancelFunc
	grupo  sync.WaitGroup
}

// Nuevo crea un planificador; hay que llamar a Iniciar antes de enviar trabajos.
func Nuevo(almacen *persistencia.Almacen, registro map[string]adaptadores.Adaptador, semaforos Semaforos, opc Opciones) *Planificador {
	opc.MaximoCola, opc.MaximoTrabajos = max(opc.MaximoCola, 1), max(opc.MaximoTrabajos, 1)
	p := &Planificador{opc: opc, almacen: almacen, adaptadores: registro, semaforos: semaforos,
		conversiones: make(chan struct{}, max(opc.MaximoConversiones, 1)), ranuras: make(chan struct{}, opc.MaximoTrabajos),
		cancelaciones: map[string]context.CancelFunc{}}
	almacen.Topes = p.topes
	return p
}

// Iniciar recupera el estado tras un reinicio, reencola lo pendiente y arranca el despachador.
func (p *Planificador) Iniciar(ctx context.Context) error {
	if err := p.terminarHuerfanos(ctx); err != nil {
		return err
	}
	if _, _, err := p.almacen.Recuperar(ctx, p.opc.Instancia); err != nil {
		return err
	}
	pendientes, err := p.almacen.ListarPorEstado(ctx, p.opc.Instancia, nucleo.EnCola)
	if err != nil {
		return err
	}
	// Capacidad inicial: los pendientes más la cola admitida; encolar no bloquea si se llena.
	p.cola = make(chan string, p.opc.MaximoCola+len(pendientes))
	for _, id := range pendientes {
		p.cola <- id
	}
	p.ctx, p.parar = context.WithCancel(context.Background())
	p.ejec, p.cortar = context.WithCancel(context.Background())
	p.grupo.Add(1)
	go p.despachar()
	return nil
}

// terminarHuerfanos mata los agentes que siguieran vivos de una ejecución anterior del servicio.
func (p *Planificador) terminarHuerfanos(ctx context.Context) error {
	ids, err := p.almacen.ListarPorEstado(ctx, p.opc.Instancia, nucleo.Ejecutando, nucleo.ValidandoSalida, nucleo.CancelacionSolicitada)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if t, err := p.almacen.Obtener(ctx, id); err == nil {
			procesos.TerminarHuerfano(t.PID, t.FirmaProceso)
		}
	}
	return nil
}

// Enviar valida y acepta una solicitud ya completada con los predeterminados (AplicarPredeterminados).
// Devuelve reutilizado = true si la clave de idempotencia ya apuntaba a un trabajo con la misma huella.
func (p *Planificador) Enviar(ctx context.Context, solicitud nucleo.SolicitudEjecucion, clave string) (nucleo.Trabajo, bool, error) {
	return p.aceptar(ctx, solicitud, clave, "", "")
}

// EnviarPorPunto acepta una solicitud recibida por un punto de acceso y la vincula a él.
func (p *Planificador) EnviarPorPunto(ctx context.Context, solicitud nucleo.SolicitudEjecucion, clave, punto string) (nucleo.Trabajo, bool, error) {
	return p.aceptar(ctx, solicitud, clave, "", punto)
}

// aceptar registra el trabajo; intentoDe vincula un reintento explícito con el trabajo anterior.
func (p *Planificador) aceptar(ctx context.Context, solicitud nucleo.SolicitudEjecucion, clave, intentoDe, punto string) (nucleo.Trabajo, bool, error) {
	if p.ctx == nil || p.ctx.Err() != nil {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoServicioDetenido, idioma.T("el planificador no está en marcha"))
	}
	if err := solicitud.Validar(); err != nil {
		return nucleo.Trabajo{}, false, err
	}
	for _, e := range solicitud.Respaldo {
		if agente, _, _ := nucleo.DividirRespaldo(e); p.adaptadores[agente] == nil {
			return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("agente de respaldo no disponible: %s", agente))
		}
	}
	if solicitud.ModoContexto == "consciente_proyecto" && p.opc.RaizConfiable == "" {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoContextoNoPermitido,
			idioma.T("consciente_proyecto solo se admite en la instancia de un proyecto incluido en proyectos_confiables de la configuración global"))
	}
	if len(solicitud.Salida.Esquema) > 0 {
		if _, err := salidas.Compilar(solicitud.Salida.Esquema); err != nil {
			return nucleo.Trabajo{}, false, err
		}
	}
	// Una solicitud nueva no puede citar cargas vencidas; un reintento usa las que su trabajo protegía.
	if v, ok := p.opc.Preparador.(interface{ Verificar([]string) error }); ok && intentoDe == "" && len(solicitud.Cargas) > 0 {
		if err := v.Verificar(solicitud.Cargas); err != nil {
			return nucleo.Trabajo{}, false, err
		}
	}
	modelo, err := p.resolverModelo(solicitud)
	if err != nil {
		return nucleo.Trabajo{}, false, err
	}
	t := nucleo.Trabajo{ID: nucleo.NuevoID(), Instancia: p.opc.Instancia, Solicitud: solicitud, Huella: solicitud.Huella(), ModeloEnviado: modelo, IntentoDe: intentoDe, PuntoAcceso: punto,
		Origen: nucleo.OrigenPeticion(ctx)}
	if punto != "" {
		t.Origen = nucleo.DesdeAPI // por una API extra, aunque la app mande otra etiqueta
	}
	if err := p.permitidoParaOrigen(t); err != nil {
		return nucleo.Trabajo{}, false, err
	}
	t, reutilizado, err := p.almacen.Aceptar(ctx, t, clave, p.opc.MaximoCola)
	if err != nil || reutilizado {
		return t, reutilizado, err
	}
	p.encolar(t.ID)
	p.registrar(t, "recibido")
	return t, false, nil
}

// permitidoParaOrigen rechaza Antigravity (principal o respaldo) en una petición de una app o una API: hasta comprobar
// con el CLI real que no puede usar herramientas, solo se usa desde Probar, la terminal y la CLI.
func (p *Planificador) permitidoParaOrigen(t nucleo.Trabajo) error {
	if p.opc.PermitirAntigravityApps || (t.Origen != nucleo.DesdeApp && t.Origen != nucleo.DesdeAPI) {
		return nil
	}
	for _, c := range candidatos(t) {
		if c.agente == "antigravity" {
			return nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, idioma.T("Antigravity no está habilitado para aplicaciones: todavía no se ha comprobado con el CLI real que no pueda usar herramientas. Úsalo desde Probar o la terminal"))
		}
	}
	return nil
}

// registrar escribe una línea legible por trabajo, por ejemplo:
// 10:01:22 global completado trabajo_dc76… claude/claude-haiku-4-5 · API soporte · 5.2s · 577→40 tokens
func (p *Planificador) registrar(t nucleo.Trabajo, evento string) {
	if p.opc.Registros == nil {
		return
	}
	linea := fmt.Sprintf("%s %s %-11s %s %s", time.Now().Format("15:04:05"), p.opc.Instancia, evento, t.ID, t.Solicitud.Agente)
	if modelo := cmp.Or(t.ModeloEfectivo, t.ModeloEnviado); modelo != "" {
		linea += "/" + modelo
	}
	if t.PuntoAcceso != "" {
		linea += " · API " + t.PuntoAcceso
	}
	if t.TerminadoEn != nil {
		linea += fmt.Sprintf(" · %.1fs", t.TerminadoEn.Sub(t.CreadoEn).Seconds())
	}
	if t.Uso.TokensEntrada+t.Uso.TokensSalida > 0 {
		linea += fmt.Sprintf(" · %d→%d tokens", t.Uso.TokensEntrada, t.Uso.TokensSalida)
	}
	if t.CodigoError != "" {
		linea += " · " + string(t.CodigoError) + ": " + t.MensajeError
	}
	fmt.Fprintln(p.opc.Registros, linea)
}

// resolverModelo fija al aceptar el identificador que recibirá el CLI (fotografía del trabajo).
// Un perfil sin modelo para el agente se rechaza: nunca se sustituye el modelo en silencio.
func (p *Planificador) resolverModelo(s nucleo.SolicitudEjecucion) (string, error) {
	if s.PerfilModelo == "" {
		return s.Modelo, nil
	}
	if modelo := p.opc.Perfiles[s.PerfilModelo][s.Agente]; modelo != "" {
		return modelo, nil
	}
	return "", nucleo.NuevoError(nucleo.CodigoModeloNoConfigurado,
		idioma.Tf("el perfil %q no define modelo para el agente %q (modelos.perfiles.%s.%s)", s.PerfilModelo, s.Agente, s.PerfilModelo, s.Agente))
}

// encolar entrega el trabajo a los ejecutores sin bloquear al cliente. Un trabajo cancelado en cola sigue
// ocupando hueco en el canal hasta que un ejecutor lo descarta, así que si el canal está lleno se espera aparte.
func (p *Planificador) encolar(id string) {
	select {
	case p.cola <- id:
	default:
		go func() {
			select {
			case p.cola <- id:
			case <-p.ctx.Done(): // en cierre el trabajo sigue en_cola y se reencola al reiniciar
			}
		}()
	}
}

// Cancelar es idempotente: en un estado terminal o ya en cancelación devuelve el trabajo sin cambios.
// Solo registra cancelado cuando la terminación está confirmada: al momento si el trabajo aún no lanzó nada
// (en cola o esperando al agente) y, si no, cuando su ejecutor confirma que terminó.
func (p *Planificador) Cancelar(ctx context.Context, id string) (nucleo.Trabajo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// El ejecutor cambia el estado sin este cerrojo: si avanzó entre la lectura y el cambio (de en_cola a
	// esperando_agente, por ejemplo), se vuelve a leer y se reintenta.
	var t nucleo.Trabajo
	for intento := 1; ; intento++ {
		var err error
		t, err = p.Obtener(ctx, id)
		if err != nil || t.Estado.Terminal() || t.Estado == nucleo.CancelacionSolicitada {
			return t, err
		}
		err = p.almacen.CambiarEstado(ctx, id, t.Estado, nucleo.CancelacionSolicitada, persistencia.Cambio{})
		if err == nil {
			break
		}
		if nucleo.CodigoDe(err) != nucleo.CodigoTransicionInvalida || intento == 5 {
			return t, err
		}
	}
	cancelar, enCurso := p.cancelaciones[id]
	if enCurso {
		cancelar() // con algo en marcha, el ejecutor confirma la terminación y registra cancelado
	}
	if !enCurso || t.Estado == nucleo.EnCola || t.Estado == nucleo.EsperandoAgente {
		// Si el ejecutor se adelantó al ver la cancelación, el trabajo ya está cancelado.
		if err := p.almacen.CambiarEstado(ctx, id, nucleo.CancelacionSolicitada, nucleo.Cancelado, persistencia.Cambio{}); err != nil &&
			nucleo.CodigoDe(err) != nucleo.CodigoTransicionInvalida {
			return t, err
		}
	}
	return p.almacen.Obtener(ctx, id)
}

// Obtener devuelve un trabajo de esta instancia; los de otras instancias no existen para ella.
func (p *Planificador) Obtener(ctx context.Context, id string) (nucleo.Trabajo, error) {
	t, err := p.almacen.Obtener(ctx, id)
	if err == nil && t.Instancia != p.opc.Instancia {
		return nucleo.Trabajo{}, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("trabajo no encontrado: %s", id))
	}
	return t, err
}

// Avisos devuelve un canal que se cierra en el próximo cambio de cualquier trabajo de la instancia.
func (p *Planificador) Avisos() <-chan struct{} { return p.almacen.Avisos() }

// Eventos devuelve el estado de un trabajo de esta instancia y sus eventos posteriores al número desde. El estado
// se lee antes que los eventos: si ya es terminal, los eventos devueltos incluyen el de su último cambio.
func (p *Planificador) Eventos(ctx context.Context, id string, desde int) ([]persistencia.Evento, nucleo.Estado, error) {
	instancia, estado, err := p.almacen.Estado(ctx, id)
	if err == nil && instancia != p.opc.Instancia {
		err = nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("trabajo no encontrado: %s", id))
	}
	if err != nil {
		return nil, "", err
	}
	eventos, err := p.almacen.Eventos(ctx, id, desde)
	return eventos, estado, err
}

// Registro devuelve los pasos de un trabajo sin el texto de la respuesta: estados, intentos y avisos.
func (p *Planificador) Registro(ctx context.Context, id string) ([]persistencia.Evento, error) {
	instancia, _, err := p.almacen.Estado(ctx, id)
	if err == nil && instancia != p.opc.Instancia {
		err = nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("trabajo no encontrado: %s", id))
	}
	if err != nil {
		return nil, err
	}
	return p.almacen.EventosDeRegistro(ctx, id)
}

// Reintentar crea un intento nuevo vinculado al anterior, que debe haber terminado; reutiliza sus entradas
// y puede consumir cuota otra vez.
func (p *Planificador) Reintentar(ctx context.Context, id string) (nucleo.Trabajo, error) {
	anterior, err := p.Obtener(ctx, id)
	if err != nil {
		return anterior, err
	}
	if !anterior.Estado.Terminal() {
		return anterior, nucleo.NuevoError(nucleo.CodigoTrabajoNoTerminado, idioma.T("solo se reintenta un trabajo terminado"))
	}
	if anterior.EntradasEliminadas {
		return anterior, nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("las entradas del trabajo ya vencieron o se eliminaron al terminar; vuelve a enviarlas"))
	}
	// El reintento es el mismo pedido otra vez: conserva su API y su origen.
	t, _, err := p.aceptar(nucleo.ConOrigenPeticion(ctx, anterior.Origen), anterior.Solicitud, "", anterior.ID, anterior.PuntoAcceso)
	return t, err
}

// Eliminar borra un trabajo terminado de esta instancia.
func (p *Planificador) Eliminar(ctx context.Context, id string) error {
	if _, err := p.Obtener(ctx, id); err != nil {
		return err
	}
	return p.almacen.Eliminar(ctx, id)
}

// EliminarTerminados borra todos los trabajos terminados de esta instancia y devuelve cuántos borró.
func (p *Planificador) EliminarTerminados(ctx context.Context) (int, error) {
	return p.almacen.EliminarTerminados(ctx, p.opc.Instancia)
}

// UsoPorPunto resume las peticiones de cada punto de acceso de esta instancia.
func (p *Planificador) UsoPorPunto(ctx context.Context) (map[string]persistencia.UsoPunto, error) {
	return p.almacen.UsoPorPunto(ctx, p.opc.Instancia)
}

// ModelosEfectivos devuelve el último modelo efectivo por "agente/modelo enviado".
func (p *Planificador) ModelosEfectivos(ctx context.Context) (map[string]string, error) {
	return p.almacen.ModelosEfectivos(ctx)
}

// CargasEnUso devuelve las cargas que algún trabajo de esta instancia puede necesitar todavía.
func (p *Planificador) CargasEnUso(ctx context.Context) (map[string]bool, error) {
	return p.almacen.CargasEnUso(ctx, p.opc.Instancia)
}

// Listar devuelve los trabajos más recientes de la instancia; con punto, solo los de ese punto de acceso.
func (p *Planificador) Listar(ctx context.Context, limite int, f persistencia.FiltroTrabajos) ([]nucleo.Trabajo, error) {
	return p.almacen.Listar(ctx, p.opc.Instancia, f, limite)
}

// Contar devuelve cuántos trabajos de la instancia hay en cada estado.
func (p *Planificador) Contar(ctx context.Context) (map[nucleo.Estado]int, error) {
	return p.almacen.Contar(ctx, p.opc.Instancia)
}

// Detener deja de aceptar trabajos e interrumpe lo que se está ejecutando (Drenar sin plazo).
func (p *Planificador) Detener() { p.Drenar(0) }

// Drenar deja de aceptar trabajos y de tomarlos de la cola, espera hasta plazo a que terminen los que se
// están ejecutando y después interrumpe los que queden. Lo que seguía en cola se reanuda al reiniciar.
func (p *Planificador) Drenar(plazo time.Duration) {
	if p.parar == nil {
		return
	}
	p.parar()
	fin := make(chan struct{})
	go func() {
		p.grupo.Wait()
		close(fin)
	}()
	if plazo > 0 {
		select {
		case <-fin:
			return
		case <-time.After(plazo):
		}
	}
	p.cortar()
	<-fin
}

// LimitarPuntos fija cuántos trabajos de cada punto de acceso se ejecutan a la vez; sin límite (0), solo
// cuentan los de la instancia. Un límite que no cambia conserva su semáforo, así que recargar no libera plazas.
func (p *Planificador) LimitarPuntos(limites map[string]int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	nuevos := map[string]chan struct{}{}
	for nombre, limite := range limites {
		if limite <= 0 {
			continue
		}
		if actual := p.semPuntos[nombre]; actual != nil && cap(actual) == limite {
			nuevos[nombre] = actual
		} else {
			nuevos[nombre] = make(chan struct{}, limite)
		}
	}
	p.semPuntos = nuevos
}

func (p *Planificador) semaforoPunto(nombre string) chan struct{} {
	if nombre == "" {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.semPuntos[nombre]
}

// EnCurso devuelve cuántos trabajos se están procesando ahora (los que esperaría un drenaje).
func (p *Planificador) EnCurso() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.cancelaciones)
}

// despachar saca los trabajos de la cola en orden y procesa cada uno en su goroutine: un trabajo que espera
// a su agente o a su punto de acceso no impide empezar a otro listo. Los semáforos limitan cuántos se ejecutan.
// Antes de sacar el siguiente espera a que el anterior llegue a su espera: así cada semáforo, que atiende por
// orden de llegada, conserva el orden de la cola.
func (p *Planificador) despachar() {
	defer p.grupo.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case id := <-p.cola:
			esperando := make(chan struct{})
			p.grupo.Add(1)
			go func() {
				defer p.grupo.Done()
				p.procesar(id, sync.OnceFunc(func() { close(esperando) }))
			}()
			<-esperando
		}
	}
}

// procesar lleva un trabajo de en_cola a un estado terminal. Las escrituras usan context.Background
// para que el estado quede consistente aunque el servicio se esté deteniendo. esperando se llama al llegar
// a la espera de los semáforos o de la conversión, o al terminar antes.
func (p *Planificador) procesar(id string, esperando func()) {
	defer esperando()
	fondo := context.Background()
	defer func() { // una línea en el registro si el trabajo terminó (no si quedó en cola por un cierre)
		if t, err := p.almacen.Obtener(fondo, id); err == nil && t.Estado.Terminal() {
			p.registrar(t, string(t.Estado))
		}
	}()
	if p.ctx.Err() != nil {
		return // en cierre: el trabajo sigue en_cola y se reencola al reiniciar
	}
	t, err := p.almacen.Obtener(fondo, id)
	if err != nil || t.Estado != nucleo.EnCola {
		return
	}
	if p.opc.TiempoMaximoCola > 0 && time.Since(t.CreadoEn) > p.opc.TiempoMaximoCola {
		p.cambiar(id, nucleo.EnCola, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoTiempoColaAgotado, Mensaje: idioma.T("se superó el tiempo máximo en cola")})
		return
	}
	if _, registrado := p.adaptadores[t.Solicitud.Agente]; !registrado {
		p.cambiar(id, nucleo.EnCola, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoAgenteNoDisponible, Mensaje: idioma.Tf("agente no registrado: %s", t.Solicitud.Agente)})
		return
	}

	ctxTrabajo, cancelar := context.WithCancel(p.ejec)
	defer cancelar()
	p.mu.Lock()
	p.cancelaciones[id] = cancelar
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.cancelaciones, id)
		p.mu.Unlock()
	}()

	solicitud, desde := t.Solicitud, nucleo.EnCola
	var documentos nucleo.Documentos
	if len(solicitud.Cargas) > 0 {
		esperando() // una conversión larga no frena el despacho de los demás
		if !p.cambiar(id, nucleo.EnCola, nucleo.PreparandoEntrada, persistencia.Cambio{}) {
			return
		}
		desde = nucleo.PreparandoEntrada
		var err error
		documentos, err = p.preparar(ctxTrabajo, solicitud.Cargas)
		if ctxTrabajo.Err() != nil {
			p.cerrarDetenido(id, desde)
			return
		}
		if err != nil {
			codigo := nucleo.CodigoDe(err)
			if codigo == "" {
				codigo = nucleo.CodigoConversionFallida
			}
			p.cambiar(id, desde, nucleo.Fallido, persistencia.Cambio{Codigo: codigo, Mensaje: nucleo.MensajeDe(err)})
			return
		}
		solicitud = conDocumentos(solicitud, documentos.Texto)
		if p.ctx.Err() != nil { // la conversión terminó durante un drenaje: no se lanza un agente nuevo
			p.cerrarDetenido(id, desde)
			return
		}
	}
	medida := medirEntrada(solicitud, documentos)
	if datos, err := json.Marshal(medida); err == nil {
		_ = p.almacen.AgregarEvento(fondo, id, "entrada", string(datos))
	}
	if !p.cambiar(id, desde, nucleo.EsperandoAgente, persistencia.Cambio{}) {
		return
	}
	// El plazo en cola cuenta hasta que el trabajo empieza a ejecutarse, también mientras espera al agente.
	var plazoCola <-chan time.Time
	if p.opc.TiempoMaximoCola > 0 {
		temporizador := time.NewTimer(time.Until(t.CreadoEn.Add(p.opc.TiempoMaximoCola)))
		defer temporizador.Stop()
		plazoCola = temporizador.C
	}
	// shortcut: el siguiente trabajo tarda en llegar a su espera (lee y escribe en la base) mucho más que este en
	// entrar en el select, así que en la práctica se respeta el orden; si no bastara, un turno por agente.
	esperando()
	if solicitud.ModoContexto == "consciente_proyecto" {
		solicitud.RaizProyecto = p.opc.RaizConfiable
		// Se declara de forma visible qué contexto se carga antes de ejecutar.
		_ = p.almacen.AgregarEvento(fondo, id, "aviso", idioma.Tf("modo consciente_proyecto: el agente trabaja en %s"+
			" y carga las reglas del proyecto y de sus carpetas superiores (CLAUDE.md, AGENTS.md, GEMINI.md), sin MCP, herramientas"+
			" ni sesión persistente; con autenticación por suscripción el CLI carga también las reglas del usuario", p.opc.RaizConfiable))
	}
	// El principal y después el respaldo: se salta al siguiente solo si el intento falló antes de emitir texto,
	// para no mezclar dos respuestas. El plazo en cola vale hasta el primer intento.
	lista := candidatos(t)
	var resultado adaptadores.Resultado
	var usoPrevio nucleo.Uso // lo que gastaron los intentos que fallaron: también cuenta para el presupuesto
	for i, c := range lista {
		// Lo que no cabe en el contexto del modelo no llega a su CLI: se salta sin ejecutarlo y el trabajo sigue
		// esperando agente.
		var res adaptadores.Resultado
		var huboTexto, fin bool
		var err error
		estado := nucleo.Ejecutando
		if contexto, _ := p.contextoDe(ctxTrabajo, c); contexto > 0 && !cabe(medida, contexto, p.opc.ReservaSalida) {
			estado, err = nucleo.EsperandoAgente, noCabe(medida, c, contexto, p.opc.ReservaSalida)
		} else {
			res, huboTexto, fin, err = p.intentar(ctxTrabajo, id, t, solicitud, c, plazoCola)
		}
		if fin {
			return
		}
		if err == nil {
			resultado = res
			resultado.Uso = sumarUso(resultado.Uso, usoPrevio)
			break
		}
		usoPrevio = sumarUso(usoPrevio, res.Uso)
		codigo := nucleo.CodigoDe(err)
		if codigo == "" {
			codigo = nucleo.CodigoErrorAgente
		}
		if !saltable(codigo) || huboTexto || i+1 == len(lista) {
			p.cambiar(id, estado, nucleo.Fallido, persistencia.Cambio{Codigo: codigo, Mensaje: nucleo.MensajeDe(err), Uso: &usoPrevio})
			return
		}
		siguiente := lista[i+1]
		// El uso acumulado queda en el trabajo: si se cancela durante el respaldo, también cuenta.
		if estado == nucleo.Ejecutando && !p.cambiar(id, nucleo.Ejecutando, nucleo.EsperandoAgente, persistencia.Cambio{Uso: &usoPrevio}) {
			return
		}
		// Lo guardado debe seguir valiendo al reintentar o reencolar: el agente nuevo con el esfuerzo que usa y el
		// respaldo restante con lo que heredaba del principal escrito, porque la solicitud guardada ya no lo tendrá
		// (sin esfuerzo en el principal, +agente: el de su propio CLI).
		esfuerzo := cmp.Or(siguiente.esfuerzo, t.Solicitud.Esfuerzo)
		if esfuerzo == nucleo.EsfuerzoDelAgente {
			esfuerzo = ""
		}
		herencia := t.Solicitud.Esfuerzo
		if herencia == "" && esfuerzo != "" {
			herencia = nucleo.EsfuerzoDelAgente
		}
		var restante []string
		for _, c := range lista[i+2:] {
			c.esfuerzo = cmp.Or(c.esfuerzo, herencia)
			restante = append(restante, c.String())
		}
		if err := p.almacen.CambiarAgente(fondo, id, siguiente.agente, siguiente.modelo, esfuerzo, restante); err != nil && p.opc.Registros != nil {
			// El respaldo sigue igual, pero el historial atribuiría la respuesta al agente anterior.
			fmt.Fprintf(p.opc.Registros, "%s no se pudo guardar el cambio a %s: %v\n", id, siguiente, err)
		}
		aviso := idioma.Tf("%s falló (%s); sigue con %s", c, codigo, siguiente)
		if codigo == nucleo.CodigoEntradaDemasiadoGrande {
			aviso = idioma.Tf("%s; sigue con %s", nucleo.MensajeDe(err), siguiente)
		}
		_ = p.almacen.AgregarEvento(fondo, id, "aviso", aviso)
		plazoCola = nil
	}

	if !p.cambiar(id, nucleo.Ejecutando, nucleo.ValidandoSalida, persistencia.Cambio{}) {
		return
	}
	if err := validarSalida(t.Solicitud.Salida, resultado.Salida); err != nil {
		cambio := persistencia.Cambio{Codigo: nucleo.CodigoSalidaNoCumpleEsquema,
			Mensaje: nucleo.MensajeDe(err), Uso: &resultado.Uso, ModeloEfectivo: resultado.ModeloEfectivo}
		if t.Solicitud.Salida.ConservarSalidaInvalida {
			cambio.Salida = &resultado.Salida // salida bruta conservada, con la retención del resultado
		}
		p.cambiar(id, nucleo.ValidandoSalida, nucleo.Fallido, cambio)
		return
	}
	p.cambiar(id, nucleo.ValidandoSalida, nucleo.Completado, persistencia.Cambio{Salida: &resultado.Salida,
		Uso: &resultado.Uso, ModeloEfectivo: resultado.ModeloEfectivo})
}

// intento es lo que guarda el evento intento: esfuerzo es el nivel que se pidió al CLI, ya ajustado a su máximo, o
// con DelAgente el que usa por su cuenta para ese modelo (vacío si no se conoce o el agente no tiene esfuerzo).
type intento struct {
	Agente    string `json:"agente"`
	Modelo    string `json:"modelo"`
	Esfuerzo  string `json:"esfuerzo"`
	DelAgente bool   `json:"del_agente,omitempty"`
}

// nuevoIntento arma el evento de un intento; versionVista resuelve un alias (o el modelo vacío) con la última versión
// que informó el agente, como el catálogo.
func nuevoIntento(c candidato, pedido string, versionVista func(agente, modelo string) string) intento {
	i := intento{Agente: c.agente, Modelo: c.modelo}
	niveles := adaptadores.NivelesEsfuerzo(c.agente)
	switch {
	case len(niveles) == 0:
	case pedido == "":
		i.Esfuerzo = adaptadores.EsfuerzoPredeterminado(c.agente, c.modelo)
		if i.Esfuerzo == "" {
			i.Esfuerzo = adaptadores.EsfuerzoPredeterminado(c.agente, versionVista(c.agente, c.modelo))
		}
		i.DelAgente = i.Esfuerzo != ""
	case slices.Contains(niveles, pedido):
		i.Esfuerzo = pedido
	default: // por encima de su máximo: el adaptador usa el máximo
		i.Esfuerzo = niveles[len(niveles)-1]
	}
	return i
}

// versionVista es el último modelo efectivo que informó el agente para ese modelo enviado ("" si no hay).
func (p *Planificador) versionVista(agente, modelo string) string {
	efectivos, _ := p.almacen.ModelosEfectivos(context.Background())
	return efectivos[agente+"/"+modelo]
}

// medida es lo que ocupa la entrada que recibirá el agente: caracteres y tokens estimados (sin el tokenizador de cada
// proveedor, caracteres ÷ 4), y cada documento de mayor a menor. Es el evento «entrada»; no lleva contenido.
type medida struct {
	Caracteres int           `json:"caracteres"`
	Tokens     int           `json:"tokens_estimados"`
	Documentos []parteMedida `json:"documentos"`
}

type parteMedida struct {
	Nombre string `json:"nombre"`
	Tokens int    `json:"tokens_estimados"`
}

func tokensDe(caracteres int) int { return (caracteres + 3) / 4 }

// cabe dice si la entrada, más lo que se reserva para la respuesta, entra en el contexto del modelo.
func cabe(m medida, contexto, reserva int) bool { return m.Tokens+reserva <= contexto }

// noCabe explica cuánto mide la entrada, qué documento pesa más y cuánto admite el modelo.
func noCabe(m medida, c candidato, contexto, reserva int) error {
	mayor := ""
	if len(m.Documentos) > 0 {
		mayor = idioma.Tf("; %s ~%d", m.Documentos[0].Nombre, m.Documentos[0].Tokens)
	}
	return nucleo.NuevoError(nucleo.CodigoEntradaDemasiadoGrande, idioma.Tf(
		"la entrada (~%d tokens%s) no cabe en %s: su contexto es de %d tokens y se reservan %d para la respuesta", m.Tokens, mayor, c, contexto, reserva))
}

// medirEntrada mide las instrucciones y los mensajes, ya con los documentos dentro.
func medirEntrada(s nucleo.SolicitudEjecucion, docs nucleo.Documentos) medida {
	caracteres := utf8.RuneCountInString(s.Instrucciones)
	for _, m := range s.Mensajes {
		caracteres += utf8.RuneCountInString(m.Contenido)
	}
	m := medida{Caracteres: caracteres, Tokens: tokensDe(caracteres), Documentos: []parteMedida{}}
	for _, d := range docs.Partes {
		m.Documentos = append(m.Documentos, parteMedida{Nombre: d.Nombre, Tokens: tokensDe(d.Caracteres)})
	}
	slices.SortStableFunc(m.Documentos, func(a, b parteMedida) int { return b.Tokens - a.Tokens })
	return m
}

// contextoDe es el contexto (tokens) del modelo de un candidato y de dónde sale: lo configurado en
// limites.contexto_tokens (agente/modelo y después agente), lo que informa el agente (su API o lo que dijo su CLI en
// un intento anterior, para el modelo o la versión vista detrás de un alias) o 0 si no se conoce.
func (p *Planificador) contextoDe(ctx context.Context, c candidato) (int, string) {
	version := p.versionVista(c.agente, c.modelo)
	modelos := slices.DeleteFunc([]string{c.modelo, version}, func(m string) bool { return m == "" })
	for _, m := range modelos {
		if n := p.opc.Contextos[c.agente+"/"+m]; n > 0 {
			return n, "configurado"
		}
	}
	if n := p.opc.Contextos[c.agente]; n > 0 {
		return n, "configurado"
	}
	if contextual, ok := p.adaptadores[c.agente].(adaptadores.Contextual); ok {
		if n, err := contextual.Contexto(ctx, c.modelo); err == nil && n > 0 {
			return n, "informado"
		}
	}
	for _, m := range modelos {
		if n, err := p.almacen.Contexto(ctx, c.agente, m); err == nil && n > 0 {
			return n, "informado"
		}
	}
	return 0, ""
}

// sumarUso junta el uso de dos intentos del mismo trabajo.
func sumarUso(a, b nucleo.Uso) nucleo.Uso {
	return nucleo.Uso{TokensEntrada: a.TokensEntrada + b.TokensEntrada, TokensSalida: a.TokensSalida + b.TokensSalida, CostoUSD: a.CostoUSD + b.CostoUSD}
}

// candidato es un agente que puede ejecutar el trabajo: el principal o uno del respaldo. esfuerzo vacío usa el de la
// solicitud.
type candidato struct{ agente, modelo, esfuerzo string }

func (c candidato) String() string {
	texto := c.agente
	if c.modelo != "" {
		texto += "/" + c.modelo
	}
	if c.esfuerzo != "" {
		texto += "+" + c.esfuerzo
	}
	return texto
}

// candidatos son el agente principal (con el modelo ya resuelto) y después el respaldo. Una entrada que repite un
// candidato anterior se omite; una sin modelo repite cualquier intento anterior de su agente.
func candidatos(t nucleo.Trabajo) []candidato {
	lista := []candidato{{t.Solicitud.Agente, t.ModeloEnviado, ""}}
	for _, e := range t.Solicitud.Respaldo {
		agente, modelo, esfuerzo := nucleo.DividirRespaldo(e)
		if !slices.ContainsFunc(lista, func(c candidato) bool { return c.agente == agente && (modelo == "" || c.modelo == modelo) }) {
			lista = append(lista, candidato{agente, modelo, esfuerzo})
		}
	}
	return lista
}

// saltable indica los errores tras los que otro agente puede tener éxito: límite, credenciales, disponibilidad,
// fallo del agente o tiempo agotado. La cancelación y un esquema no cumplido nunca saltan.
func saltable(codigo nucleo.CodigoError) bool {
	switch codigo {
	case nucleo.CodigoLimiteUsoProveedor, nucleo.CodigoAutenticacionAgente, nucleo.CodigoEntradaDemasiadoGrande, nucleo.CodigoAgenteNoDisponible, nucleo.CodigoModeloNoDisponible,
		nucleo.CodigoErrorAgente, nucleo.CodigoErrorProtocoloAgente, nucleo.CodigoTiempoEjecucionAgotado:
		return true
	}
	return false
}

// intentar ejecuta un candidato: toma sus semáforos, lo lanza y los suelta al volver. Con fin = true el trabajo ya
// quedó cerrado (cancelado, reencolado por un drenaje o fuera de plazo en cola) y no hay nada más que hacer; si no,
// err es el fallo del intento y huboTexto indica si el agente llegó a emitir texto.
func (p *Planificador) intentar(ctxTrabajo context.Context, id string, t nucleo.Trabajo, solicitud nucleo.SolicitudEjecucion,
	c candidato, plazoCola <-chan time.Time) (resultado adaptadores.Resultado, huboTexto, fin bool, err error) {
	fondo := context.Background()
	// Primero el punto de acceso, después el adaptador y al final la plaza de la instancia: quien espera a su
	// agente no ocupa plaza, así que otro trabajo listo de otro agente puede empezar. Un canal atiende a sus
	// esperas por orden de llegada, así que cada agente conserva el orden de la cola.
	for _, semaforo := range []chan struct{}{p.semaforoPunto(t.PuntoAcceso), p.semaforos[c.agente], p.ranuras} {
		if semaforo == nil {
			continue
		}
		select {
		case semaforo <- struct{}{}:
			defer func() { <-semaforo }()
		case <-ctxTrabajo.Done():
			p.cerrarDetenido(id, nucleo.EsperandoAgente)
			return resultado, false, true, nil
		case <-p.ctx.Done(): // en drenaje: nunca lanzó el agente, se reencola
			p.cerrarDetenido(id, nucleo.EsperandoAgente)
			return resultado, false, true, nil
		case <-plazoCola:
			p.cambiar(id, nucleo.EsperandoAgente, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoTiempoColaAgotado, Mensaje: idioma.T("se superó el tiempo máximo en cola")})
			return resultado, false, true, nil
		}
	}
	if p.ctx.Err() != nil { // el drenaje empezó mientras se tomaban los semáforos
		p.cerrarDetenido(id, nucleo.EsperandoAgente)
		return resultado, false, true, nil
	}
	if !p.cambiar(id, nucleo.EsperandoAgente, nucleo.Ejecutando, persistencia.Cambio{}) {
		return resultado, false, true, nil
	}

	limite := time.Duration(t.Solicitud.Ejecucion.TiempoLimiteSegundos) * time.Second
	if limite == 0 {
		limite = p.opc.TiempoLimitePredeterminado
	}
	ctxEjecucion, cancelarEjecucion := context.WithTimeout(procesos.ConAlIniciar(ctxTrabajo, func(pid int, firma string) {
		_ = p.almacen.RegistrarProceso(fondo, id, pid, firma)
	}), limite)
	defer cancelarEjecucion()
	solicitud.Agente, solicitud.Modelo, solicitud.PerfilModelo = c.agente, c.modelo, ""
	switch c.esfuerzo {
	case "":
	case nucleo.EsfuerzoDelAgente:
		solicitud.Esfuerzo = ""
	default:
		solicitud.Esfuerzo = c.esfuerzo
	}
	// El registro dice con qué agente, modelo y esfuerzo se intentó; sin contenido del usuario.
	if datos, err := json.Marshal(nuevoIntento(c, solicitud.Esfuerzo, p.versionVista)); err == nil {
		_ = p.almacen.AgregarEvento(fondo, id, "intento", string(datos))
	}
	agrupador := &agrupador{guardar: func(tipo, datos string) error { return p.almacen.AgregarEvento(fondo, id, tipo, datos) }}
	var texto atomic.Bool // algunos adaptadores emiten desde la goroutine que lee la salida del CLI
	resultado, err = p.adaptadores[c.agente].Ejecutar(ctxEjecucion, solicitud, func(e adaptadores.Evento) error {
		if e.Tipo == "texto" && e.Datos != "" {
			texto.Store(true)
		}
		return agrupador.emitir(e)
	})
	if resultado.ContextoTokens > 0 && resultado.ModeloEfectivo != "" {
		_ = p.almacen.GuardarContexto(fondo, c.agente, resultado.ModeloEfectivo, resultado.ContextoTokens)
	}
	huboTexto = texto.Load()
	_ = agrupador.vaciar() // antes de cualquier cambio de estado: el texto queda completo y en orden
	switch {
	case ctxTrabajo.Err() != nil:
		p.cerrarDetenido(id, nucleo.Ejecutando)
		return resultado, huboTexto, true, nil
	case errors.Is(ctxEjecucion.Err(), context.DeadlineExceeded):
		return resultado, huboTexto, false, nucleo.NuevoError(nucleo.CodigoTiempoEjecucionAgotado, idioma.T("se superó el tiempo límite de ejecución"))
	}
	return resultado, huboTexto, false, err
}

// cerrarDetenido finaliza un trabajo cuyo contexto se canceló, por petición del usuario o por cierre del servicio.
func (p *Planificador) cerrarDetenido(id string, desde nucleo.Estado) {
	t, err := p.almacen.Obtener(context.Background(), id)
	switch {
	case err != nil, t.Estado.Terminal(): // Terminal: Cancelar ya lo dio por cancelado
	case t.Estado == nucleo.CancelacionSolicitada:
		p.cambiar(id, nucleo.CancelacionSolicitada, nucleo.Cancelado, persistencia.Cambio{})
	case desde == nucleo.EsperandoAgente || desde == nucleo.PreparandoEntrada:
		p.cambiar(id, desde, nucleo.EnCola, persistencia.Cambio{}) // nunca lanzó el agente: se reencola al reiniciar
	default:
		p.cambiar(id, desde, nucleo.Interrumpido, persistencia.Cambio{Mensaje: idioma.T("el servicio se detuvo durante la ejecución (sin plazo de drenaje o agotado)")})
	}
}

// cambiar aplica una transición; si falla porque el usuario canceló entretanto, confirma la cancelación.
func (p *Planificador) cambiar(id string, de, hacia nucleo.Estado, cambio persistencia.Cambio) bool {
	fondo := context.Background()
	if err := p.almacen.CambiarEstado(fondo, id, de, hacia, cambio); err == nil {
		return true
	}
	if t, err := p.almacen.Obtener(fondo, id); err == nil && t.Estado == nucleo.CancelacionSolicitada {
		_ = p.almacen.CambiarEstado(fondo, id, nucleo.CancelacionSolicitada, nucleo.Cancelado, persistencia.Cambio{})
	}
	return false
}

// agrupador junta los fragmentos de texto consecutivos antes de guardarlos: un evento por token costaba una
// transacción y despertaba a todos los que siguen la instancia. Guarda lo acumulado a los 50 ms del primer
// fragmento o al llegar a 1 KiB, y siempre antes de cualquier otro evento, así que el orden no cambia.
type agrupador struct {
	guardar func(tipo, datos string) error

	mu           sync.Mutex
	texto        strings.Builder
	temporizador *time.Timer
}

const (
	esperaAgrupado = 50 * time.Millisecond
	maximoAgrupado = 1 << 10
)

// emitir recibe los eventos del adaptador.
func (g *agrupador) emitir(e adaptadores.Evento) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if e.Tipo != "texto" {
		if err := g.vaciarConCerrojo(); err != nil {
			return err
		}
		return g.guardar(e.Tipo, e.Datos)
	}
	g.texto.WriteString(e.Datos)
	if g.texto.Len() >= maximoAgrupado {
		return g.vaciarConCerrojo()
	}
	if g.temporizador == nil {
		g.temporizador = time.AfterFunc(esperaAgrupado, func() { _ = g.vaciar() })
	}
	return nil
}

// vaciar guarda el texto acumulado.
func (g *agrupador) vaciar() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.vaciarConCerrojo()
}

func (g *agrupador) vaciarConCerrojo() error {
	if g.temporizador != nil {
		g.temporizador.Stop()
		g.temporizador = nil
	}
	if g.texto.Len() == 0 {
		return nil
	}
	datos := g.texto.String()
	g.texto.Reset()
	return g.guardar("texto", datos)
}

// preparar convierte las cargas respetando el límite de conversiones de la instancia.
func (p *Planificador) preparar(ctx context.Context, cargas []string) (nucleo.Documentos, error) {
	if p.opc.Preparador == nil {
		return nucleo.Documentos{}, nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("esta instancia no admite cargas"))
	}
	select {
	case p.conversiones <- struct{}{}:
		defer func() { <-p.conversiones }()
	case <-ctx.Done():
		return nucleo.Documentos{}, ctx.Err()
	}
	return p.opc.Preparador.Preparar(ctx, cargas)
}

// conDocumentos antepone las entradas preparadas al último mensaje del usuario sin alterar su texto;
// cada archivo va en su bloque delimitado con metadatos.
func conDocumentos(s nucleo.SolicitudEjecucion, documentos string) nucleo.SolicitudEjecucion {
	mensajes := slices.Clone(s.Mensajes)
	for i := len(mensajes) - 1; i >= 0; i-- {
		if mensajes[i].Rol == "usuario" {
			mensajes[i].Contenido = documentos + "\n" + mensajes[i].Contenido
			s.Mensajes = mensajes
			return s
		}
	}
	s.Mensajes = append(mensajes, nucleo.Mensaje{Rol: "usuario", Contenido: documentos})
	return s
}

// validarSalida comprueba el formato pedido y, si hay esquema, que la salida lo cumpla.
func validarSalida(config nucleo.ConfigSalida, salida string) error {
	if len(config.Esquema) > 0 {
		return salidas.Validar(config.Esquema, salida)
	}
	if config.Formato == "json" && !json.Valid([]byte(salida)) {
		return errors.New(idioma.T("la salida no es JSON válido"))
	}
	return nil
}
