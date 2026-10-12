// Package planificador admite trabajos, aplica colas y límites y los ejecuta con los adaptadores (sección 17).
package planificador

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

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
}

// OpcionesDesde toma los límites de la configuración efectiva.
func OpcionesDesde(c *nucleo.Configuracion, instancia string) Opciones {
	return Opciones{
		Instancia:                  instancia,
		MaximoCola:                 c.Entero("concurrencia.maximo_tamano_cola"),
		MaximoTrabajos:             c.Entero("concurrencia.maximo_trabajos_instancia"),
		TiempoMaximoCola:           time.Duration(c.Entero("concurrencia.tiempo_maximo_en_cola_segundos")) * time.Second,
		TiempoLimitePredeterminado: 300 * time.Second,
	}
}

// Semaforos limita las ejecuciones simultáneas de cada adaptador en todo el servicio, porque todas
// las instancias comparten la sesión del CLI (sección 17).
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

	mu            sync.Mutex
	cancelaciones map[string]context.CancelFunc
	cola          chan string
	ctx           context.Context
	parar         context.CancelFunc
	grupo         sync.WaitGroup
}

// Nuevo crea un planificador; hay que llamar a Iniciar antes de enviar trabajos.
func Nuevo(almacen *persistencia.Almacen, registro map[string]adaptadores.Adaptador, semaforos Semaforos, opc Opciones) *Planificador {
	opc.MaximoCola, opc.MaximoTrabajos = max(opc.MaximoCola, 1), max(opc.MaximoTrabajos, 1)
	return &Planificador{opc: opc, almacen: almacen, adaptadores: registro, semaforos: semaforos, cancelaciones: map[string]context.CancelFunc{}}
}

// Iniciar recupera el estado tras un reinicio (sección 4.3), reencola lo pendiente y arranca los ejecutores.
func (p *Planificador) Iniciar(ctx context.Context) error {
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
	for range p.opc.MaximoTrabajos {
		p.grupo.Add(1)
		go p.ejecutor()
	}
	return nil
}

// Enviar valida y acepta una solicitud ya completada con los predeterminados (AplicarPredeterminados).
// Devuelve reutilizado = true si la clave de idempotencia ya apuntaba a un trabajo con la misma huella.
func (p *Planificador) Enviar(ctx context.Context, solicitud nucleo.SolicitudEjecucion, clave string) (nucleo.Trabajo, bool, error) {
	if p.ctx == nil || p.ctx.Err() != nil {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoServicioDetenido, "el planificador no está en marcha")
	}
	if err := solicitud.Validar(); err != nil {
		return nucleo.Trabajo{}, false, err
	}
	t := nucleo.Trabajo{ID: nucleo.NuevoID(), Instancia: p.opc.Instancia, Solicitud: solicitud, Huella: solicitud.Huella()}
	t, reutilizado, err := p.almacen.Aceptar(ctx, t, clave, p.opc.MaximoCola)
	if err != nil || reutilizado {
		return t, reutilizado, err
	}
	p.encolar(t.ID)
	return t, false, nil
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
// Solo registra cancelado cuando la terminación está confirmada (sección 17.2).
func (p *Planificador) Cancelar(ctx context.Context, id string) (nucleo.Trabajo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	t, err := p.almacen.Obtener(ctx, id)
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

// Detener deja de aceptar trabajos, detiene los ejecutores y espera a que terminen (sección 17.3).
// shortcut: sin plazo de drenaje; lo que está ejecutándose queda interrumpido. El drenaje configurable llega en M7.
func (p *Planificador) Detener() {
	if p.parar != nil {
		p.parar()
	}
	p.grupo.Wait()
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
	if p.ctx.Err() != nil {
		return // en cierre: el trabajo sigue en_cola y se reencola al reiniciar
	}
	t, err := p.almacen.Obtener(fondo, id)
	if err != nil || t.Estado != nucleo.EnCola {
		return
	}
	if p.opc.TiempoMaximoCola > 0 && time.Since(t.CreadoEn) > p.opc.TiempoMaximoCola {
		p.cambiar(id, nucleo.EnCola, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoTiempoColaAgotado, Mensaje: "se superó el tiempo máximo en cola"})
		return
	}
	adaptador, registrado := p.adaptadores[t.Solicitud.Agente]
	if !registrado {
		p.cambiar(id, nucleo.EnCola, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoAgenteNoDisponible, Mensaje: "agente no registrado: " + t.Solicitud.Agente})
		return
	}

	ctxTrabajo, cancelar := context.WithCancel(p.ctx)
	defer cancelar()
	p.mu.Lock()
	p.cancelaciones[id] = cancelar
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.cancelaciones, id)
		p.mu.Unlock()
	}()

	if !p.cambiar(id, nucleo.EnCola, nucleo.EsperandoAgente, persistencia.Cambio{}) {
		return
	}
	if semaforo := p.semaforos[t.Solicitud.Agente]; semaforo != nil {
		select {
		case semaforo <- struct{}{}:
			defer func() { <-semaforo }()
		case <-ctxTrabajo.Done():
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
	ctxEjecucion, cancelarEjecucion := context.WithTimeout(ctxTrabajo, limite)
	defer cancelarEjecucion()
	resultado, err := adaptador.Ejecutar(ctxEjecucion, t.Solicitud, func(e adaptadores.Evento) error {
		return p.almacen.AgregarEvento(fondo, id, e.Tipo, e.Datos)
	})
	switch {
	case ctxTrabajo.Err() != nil:
		p.cerrarDetenido(id, nucleo.Ejecutando)
		return
	case errors.Is(ctxEjecucion.Err(), context.DeadlineExceeded):
		p.cambiar(id, nucleo.Ejecutando, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoTiempoEjecucionAgotado, Mensaje: "se superó el tiempo límite de ejecución"})
		return
	case err != nil:
		codigo := nucleo.CodigoDe(err)
		if codigo == "" {
			codigo = nucleo.CodigoErrorAgente
		}
		p.cambiar(id, nucleo.Ejecutando, nucleo.Fallido, persistencia.Cambio{Codigo: codigo, Mensaje: err.Error()})
		return
	}

	if !p.cambiar(id, nucleo.Ejecutando, nucleo.ValidandoSalida, persistencia.Cambio{}) {
		return
	}
	if err := validarSalida(t.Solicitud.Salida, resultado.Salida); err != nil {
		p.cambiar(id, nucleo.ValidandoSalida, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoSalidaNoCumpleEsquema, Mensaje: err.Error()})
		return
	}
	p.cambiar(id, nucleo.ValidandoSalida, nucleo.Completado, persistencia.Cambio{Salida: &resultado.Salida})
}

// cerrarDetenido finaliza un trabajo cuyo contexto se canceló, por petición del usuario o por cierre del servicio.
func (p *Planificador) cerrarDetenido(id string, desde nucleo.Estado) {
	t, err := p.almacen.Obtener(context.Background(), id)
	switch {
	case err != nil:
	case t.Estado == nucleo.CancelacionSolicitada:
		p.cambiar(id, nucleo.CancelacionSolicitada, nucleo.Cancelado, persistencia.Cambio{})
	case desde == nucleo.EsperandoAgente:
		p.cambiar(id, nucleo.EsperandoAgente, nucleo.EnCola, persistencia.Cambio{}) // nunca lanzó el agente: se reencola al reiniciar
	default:
		p.cambiar(id, desde, nucleo.Interrumpido, persistencia.Cambio{Mensaje: "el servicio se detuvo durante la ejecución"})
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

// validarSalida comprueba el formato pedido.
// shortcut: solo valida JSON bien formado; la validación contra JSON Schema llega con las salidas estructuradas (M3).
func validarSalida(config nucleo.ConfigSalida, salida string) error {
	if config.Formato == "json" && !json.Valid([]byte(salida)) {
		return errors.New("la salida no es JSON válido")
	}
	return nil
}
