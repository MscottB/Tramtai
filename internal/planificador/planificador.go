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
	"sync"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/procesos"
	"tramtai/internal/salidas"
)

// Preparador convierte las cargas de un trabajo en texto para el agente (documentos.Preparador).
type Preparador interface {
	Preparar(ctx context.Context, cargas []string) (string, error)
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

	mu            sync.Mutex
	cancelaciones map[string]context.CancelFunc
	// semPuntos limita las ejecuciones simultáneas de cada punto de acceso, dentro de las de la instancia.
	semPuntos map[string]chan struct{}
	cola      chan string
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
	return &Planificador{opc: opc, almacen: almacen, adaptadores: registro, semaforos: semaforos,
		conversiones: make(chan struct{}, max(opc.MaximoConversiones, 1)), cancelaciones: map[string]context.CancelFunc{}}
}

// Iniciar recupera el estado tras un reinicio, reencola lo pendiente y arranca los ejecutores.
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
	for range p.opc.MaximoTrabajos {
		p.grupo.Add(1)
		go p.ejecutor()
	}
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
	if solicitud.ModoContexto == "consciente_proyecto" && p.opc.RaizConfiable == "" {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoContextoNoPermitido,
			idioma.T("consciente_proyecto solo se admite en la instancia de un proyecto incluido en proyectos_confiables (configuración global o local.yaml)"))
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
	t := nucleo.Trabajo{ID: nucleo.NuevoID(), Instancia: p.opc.Instancia, Solicitud: solicitud, Huella: solicitud.Huella(), ModeloEnviado: modelo, IntentoDe: intentoDe, PuntoAcceso: punto}
	t, reutilizado, err := p.almacen.Aceptar(ctx, t, clave, p.opc.MaximoCola)
	if err != nil || reutilizado {
		return t, reutilizado, err
	}
	p.encolar(t.ID)
	p.registrar(t, "recibido")
	return t, false, nil
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
// Solo registra cancelado cuando la terminación está confirmada.
func (p *Planificador) Cancelar(ctx context.Context, id string) (nucleo.Trabajo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.Obtener(ctx, id)
	if err != nil || t.Estado.Terminal() || t.Estado == nucleo.CancelacionSolicitada {
		return t, err
	}
	if err := p.almacen.CambiarEstado(ctx, id, t.Estado, nucleo.CancelacionSolicitada, persistencia.Cambio{}); err != nil {
		return t, err
	}
	if cancelar, enCurso := p.cancelaciones[id]; enCurso {
		cancelar() // el ejecutor confirma la terminación y registra cancelado
	} else if err := p.almacen.CambiarEstado(ctx, id, nucleo.CancelacionSolicitada, nucleo.Cancelado, persistencia.Cambio{}); err != nil {
		return t, err
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

// Eventos devuelve los eventos de un trabajo de esta instancia posteriores al número desde.
func (p *Planificador) Eventos(ctx context.Context, id string, desde int) ([]persistencia.Evento, error) {
	if _, err := p.Obtener(ctx, id); err != nil {
		return nil, err
	}
	return p.almacen.Eventos(ctx, id, desde)
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
	t, _, err := p.aceptar(ctx, anterior.Solicitud, "", anterior.ID, anterior.PuntoAcceso)
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
func (p *Planificador) Listar(ctx context.Context, limite int, punto string) ([]nucleo.Trabajo, error) {
	return p.almacen.Listar(ctx, p.opc.Instancia, punto, limite)
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

func (p *Planificador) ejecutor() {
	defer p.grupo.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case id := <-p.cola:
			p.procesar(id)
		}
	}
}

// procesar lleva un trabajo de en_cola a un estado terminal. Las escrituras usan context.Background
// para que el estado quede consistente aunque el servicio se esté deteniendo.
func (p *Planificador) procesar(id string) {
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
	adaptador, registrado := p.adaptadores[t.Solicitud.Agente]
	if !registrado {
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
	if len(solicitud.Cargas) > 0 {
		if !p.cambiar(id, nucleo.EnCola, nucleo.PreparandoEntrada, persistencia.Cambio{}) {
			return
		}
		desde = nucleo.PreparandoEntrada
		documentos, err := p.preparar(ctxTrabajo, solicitud.Cargas)
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
		solicitud = conDocumentos(solicitud, documentos)
		if p.ctx.Err() != nil { // la conversión terminó durante un drenaje: no se lanza un agente nuevo
			p.cerrarDetenido(id, desde)
			return
		}
	}
	if !p.cambiar(id, desde, nucleo.EsperandoAgente, persistencia.Cambio{}) {
		return
	}
	// Primero el punto de acceso y después el adaptador, para no ocupar el CLI compartido mientras se espera.
	for _, semaforo := range []chan struct{}{p.semaforoPunto(t.PuntoAcceso), p.semaforos[t.Solicitud.Agente]} {
		if semaforo == nil {
			continue
		}
		select {
		case semaforo <- struct{}{}:
			defer func() { <-semaforo }()
		case <-ctxTrabajo.Done():
			p.cerrarDetenido(id, nucleo.EsperandoAgente)
			return
		case <-p.ctx.Done(): // en drenaje: nunca lanzó el agente, se reencola
			p.cerrarDetenido(id, nucleo.EsperandoAgente)
			return
		}
	}
	if !p.cambiar(id, nucleo.EsperandoAgente, nucleo.Ejecutando, persistencia.Cambio{}) {
		return
	}

	limite := time.Duration(t.Solicitud.Ejecucion.TiempoLimiteSegundos) * time.Second
	if limite == 0 {
		limite = p.opc.TiempoLimitePredeterminado
	}
	ctxEjecucion, cancelarEjecucion := context.WithTimeout(procesos.ConAlIniciar(ctxTrabajo, func(pid int, firma string) {
		_ = p.almacen.RegistrarProceso(fondo, id, pid, firma)
	}), limite)
	defer cancelarEjecucion()
	solicitud.Modelo, solicitud.PerfilModelo = t.ModeloEnviado, ""
	if solicitud.ModoContexto == "consciente_proyecto" {
		solicitud.RaizProyecto = p.opc.RaizConfiable
		// Se declara de forma visible qué contexto se carga antes de ejecutar.
		_ = p.almacen.AgregarEvento(fondo, id, "aviso", idioma.Tf("modo consciente_proyecto: el agente trabaja en %s"+
			" y carga las reglas del proyecto y de sus carpetas superiores (CLAUDE.md, AGENTS.md, GEMINI.md), sin MCP, herramientas"+
			" ni sesión persistente; con autenticación por suscripción el CLI carga también las reglas del usuario", p.opc.RaizConfiable))
	}
	resultado, err := adaptador.Ejecutar(ctxEjecucion, solicitud, func(e adaptadores.Evento) error {
		return p.almacen.AgregarEvento(fondo, id, e.Tipo, e.Datos)
	})
	switch {
	case ctxTrabajo.Err() != nil:
		p.cerrarDetenido(id, nucleo.Ejecutando)
		return
	case errors.Is(ctxEjecucion.Err(), context.DeadlineExceeded):
		p.cambiar(id, nucleo.Ejecutando, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoTiempoEjecucionAgotado, Mensaje: idioma.T("se superó el tiempo límite de ejecución")})
		return
	case err != nil:
		codigo := nucleo.CodigoDe(err)
		if codigo == "" {
			codigo = nucleo.CodigoErrorAgente
		}
		p.cambiar(id, nucleo.Ejecutando, nucleo.Fallido, persistencia.Cambio{Codigo: codigo, Mensaje: nucleo.MensajeDe(err)})
		return
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

// cerrarDetenido finaliza un trabajo cuyo contexto se canceló, por petición del usuario o por cierre del servicio.
func (p *Planificador) cerrarDetenido(id string, desde nucleo.Estado) {
	t, err := p.almacen.Obtener(context.Background(), id)
	switch {
	case err != nil:
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

// preparar convierte las cargas respetando el límite de conversiones de la instancia.
func (p *Planificador) preparar(ctx context.Context, cargas []string) (string, error) {
	if p.opc.Preparador == nil {
		return "", nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("esta instancia no admite cargas"))
	}
	select {
	case p.conversiones <- struct{}{}:
		defer func() { <-p.conversiones }()
	case <-ctx.Done():
		return "", ctx.Err()
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
