// Package planificador admite trabajos, aplica colas y límites y los ejecuta con los adaptadores.
package planificador

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/herramientas"
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
	// PermitidoEnApps dice si un agente se puede usar desde apps y APIs (su canario pasó o
	// adaptadores.<nombre>.permitir_apps lo habilita); nil usa solo la ficha de cada agente.
	PermitidoEnApps func(agente string) bool
	// TiempoConversion es el plazo total de la preparación de las cargas (conversion.tiempo_total_segundos).
	TiempoConversion time.Duration
	// Desconectado dice si un agente está desconectado (Agentes → Desconectar): no se acepta como principal y el
	// respaldo lo salta. nil = todos conectados.
	Desconectado func(agente string) bool
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
		PermitidoEnApps:            func(agente string) bool { return agentes.PermitidoEnApps(c, agente) },
		TiempoConversion:           time.Duration(c.Limite("conversion.tiempo_total_segundos")) * time.Second,
		ReservaSalida:              c.Limite("limites.reserva_salida_tokens"),
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
	// flujos son los flujos de eventos de la instancia abiertos (ver ReservarFlujo); se cuentan con mu.
	flujos int
	cola   chan string
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
	if err := herramientas.Validar(solicitud); err != nil {
		return nucleo.Trabajo{}, false, err
	}
	if p.desconectado(solicitud.Agente) {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoAgenteDesconectado,
			idioma.Tf("el agente %s está desconectado: conéctalo en Agentes o elige otro", solicitud.Agente))
	}
	for _, e := range solicitud.Respaldo {
		if agente, _, _ := nucleo.DividirRespaldo(e); p.adaptadores[agente] == nil {
			return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("agente de respaldo no disponible: %s", agente))
		}
	}
	if solicitud.ModoContexto == "consciente_proyecto" && p.opc.RaizConfiable == "" {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoContextoNoPermitido,
			idioma.T("consciente_proyecto solo se admite en el espacio de un proyecto incluido en proyectos_confiables de la configuración global"))
	}
	if len(solicitud.Salida.Esquema) > 0 {
		if err := salidas.Comprobar(solicitud.Salida.Esquema); err != nil {
			return nucleo.Trabajo{}, false, err
		}
	}
	// Una solicitud nueva no puede citar cargas vencidas; un reintento usa las que su trabajo protegía.
	if v, ok := p.opc.Preparador.(interface{ Verificar([]string, string) error }); ok && intentoDe == "" && len(solicitud.Cargas) > 0 {
		if err := v.Verificar(solicitud.Cargas, punto); err != nil {
			return nucleo.Trabajo{}, false, err
		}
	}
	// Un archivo que necesita el conversor sin conversor listo se rechaza ya (también al reintentar): no se acepta un
	// trabajo que fallaría al convertir.
	if v, ok := p.opc.Preparador.(interface{ ConversorListo([]string) error }); ok && len(solicitud.Cargas) > 0 {
		if err := v.ConversorListo(solicitud.Cargas); err != nil {
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

// permitidoParaOrigen rechaza en una petición de una app o una API un agente (principal o respaldo) que no se permite
// en apps: hasta comprobar con el CLI real que no puede usar herramientas, solo se usa desde Probar, la terminal y la
// CLI.
func (p *Planificador) permitidoParaOrigen(t nucleo.Trabajo) error {
	if t.Origen != nucleo.DesdeApp && t.Origen != nucleo.DesdeAPI {
		return nil
	}
	permitido := p.opc.PermitidoEnApps
	if permitido == nil {
		permitido = func(agente string) bool { return agentes.PermitidoEnApps(nil, agente) }
	}
	for _, c := range candidatos(t) {
		if !permitido(c.agente) {
			return nucleo.NuevoError(nucleo.CodigoAgenteNoDisponible, idioma.Tf("%s no está habilitado para aplicaciones: todavía no ha pasado su prueba de seguridad (canario). Úsalo desde Probar o la terminal", agentes.NombreVisible(c.agente)))
		}
	}
	return nil
}

// anotar escribe en el registro del servicio un problema que no tiene a quién devolverse (un error de la base dentro de
// la ejecución).
func (p *Planificador) anotar(formato string, args ...any) {
	if p.opc.Registros != nil {
		fmt.Fprintf(p.opc.Registros, "%s %s "+formato+"\n", append([]any{time.Now().Format("15:04:05"), p.opc.Instancia}, args...)...)
	}
}

// evento guarda un evento del trabajo; si la base no puede, lo anota en el registro en lugar de perderlo en silencio.
func (p *Planificador) evento(id, tipo, datos string) {
	if err := p.almacen.AgregarEvento(context.Background(), id, tipo, datos); err != nil {
		p.anotar("%s no se pudo guardar el evento %s: %v", id, tipo, err)
	}
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
		linea += " · " + string(t.CodigoError) + ": " + lineaSegura(t.MensajeError)
	}
	fmt.Fprintln(p.opc.Registros, linea)
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
