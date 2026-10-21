package planificador

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
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

	// Lo que recibe el agente: con las herramientas descritas y la conversación escrita como texto; el trabajo guarda
	// la solicitud del cliente.
	solicitud, desde := herramientas.Preparar(t.Solicitud), nucleo.EnCola
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
		p.evento(id, "entrada", string(datos))
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
	if solicitud.ModoContexto == "consciente_proyecto" {
		solicitud.RaizProyecto = p.opc.RaizConfiable
		// Se declara de forma visible qué contexto se carga antes de ejecutar.
		p.evento(id, "aviso", idioma.Tf("modo consciente_proyecto: el agente trabaja en %s"+
			" y carga las reglas del proyecto y de sus carpetas superiores (CLAUDE.md, AGENTS.md, GEMINI.md o"+
			" .github/copilot-instructions.md, según el agente), con las mismas restricciones de herramientas que en modo aislado y"+
			" sin sesión persistente; con autenticación por suscripción algunos CLI cargan también las reglas del usuario", p.opc.RaizConfiable))
	}
	// El principal y después el respaldo: se salta al siguiente solo si el intento falló antes de emitir texto,
	// para no mezclar dos respuestas. El plazo en cola vale hasta el primer intento.
	lista, desconectados := p.conectados(candidatos(t))
	var resultado adaptadores.Resultado
	var elegido candidato    // el que respondió: las correcciones se le piden a él
	var usoPrevio nucleo.Uso // lo que gastaron los intentos que fallaron: también cuenta para el presupuesto
	ejecutado := false       // si algún candidato llegó a ejecutarse (no solo se saltó por tamaño)
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
			ejecutado = true
			res, huboTexto, fin, err = p.intentar(ctxTrabajo, id, t, solicitud, c, plazoCola, esperando, 0)
		}
		if fin {
			return
		}
		if err == nil {
			resultado, elegido = res, c
			resultado.Uso = sumarUso(resultado.Uso, usoPrevio)
			p.opc.Salud.Exito(c.agente)
			break
		}
		usoPrevio = sumarUso(usoPrevio, res.Uso)
		codigo := nucleo.CodigoDe(err)
		if codigo == "" {
			codigo = nucleo.CodigoErrorAgente
		}
		if codigo == nucleo.CodigoAutenticacionAgente {
			p.opc.Salud.Fallo(c.agente)
		}
		if !saltable(codigo) || huboTexto || i+1 == len(lista) {
			// Si no cabía en ninguno y nada se ejecutó, la petición no cuenta para el presupuesto.
			mensaje := nucleo.MensajeDe(err)
			if len(desconectados) > 0 && saltable(codigo) && !huboTexto { // el respaldo que no se probó: se dice por qué
				mensaje += idioma.Tf(" (respaldo desconectado: %s)", strings.Join(desconectados, ", "))
			}
			p.cambiar(id, estado, nucleo.Fallido, persistencia.Cambio{Codigo: codigo, Mensaje: mensaje, Uso: &usoPrevio,
				DevolverPeticion: codigo == nucleo.CodigoEntradaDemasiadoGrande && !ejecutado})
			return
		}
		siguiente := lista[i+1]
		// El uso acumulado queda en el trabajo: si se cancela durante el respaldo, también cuenta.
		if estado == nucleo.Ejecutando && !p.cambiar(id, nucleo.Ejecutando, nucleo.EsperandoAgente, persistencia.Cambio{Uso: &usoPrevio}) {
			return
		}
		if estado == nucleo.EsperandoAgente && (ctxTrabajo.Err() != nil || p.ctx.Err() != nil) {
			p.cerrarDetenido(id, nucleo.EsperandoAgente) // cancelado o en drenaje mientras se saltaba: no se cambia de agente
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
		p.evento(id, "aviso", aviso)
		if estado == nucleo.Ejecutando {
			plazoCola = nil // el plazo en cola vale hasta que algo se ejecuta; un salto por tamaño no ejecutó nada
		}
	}

	// Si la salida no cumple y quedan correcciones, se pide otra al mismo agente (nunca al respaldo) con la respuesta
	// y sus problemas; cada una suma su uso al trabajo.
	reintentos := min(t.Solicitud.Salida.Reintentos, nucleo.MaximoReintentosSalida)
	for correccion := 0; ; correccion++ {
		if !p.cambiar(id, nucleo.Ejecutando, nucleo.ValidandoSalida, persistencia.Cambio{}) {
			return
		}
		errSalida := validarSalida(t.Solicitud, resultado.Salida)
		if errSalida == nil {
			p.cambiar(id, nucleo.ValidandoSalida, nucleo.Completado, persistencia.Cambio{Salida: &resultado.Salida,
				Uso: &resultado.Uso, ModeloEfectivo: resultado.ModeloEfectivo})
			return
		}
		problemas := salidas.ProblemasDe(errSalida)
		if datos, errJSON := json.Marshal(validacion{Problemas: problemas, Correccion: correccion}); errJSON == nil {
			p.evento(id, "validacion", string(datos))
		}
		// fallar cierra el trabajo con los problemas de la última respuesta (y la respuesta, si se pidió conservarla).
		fallar := func(desde nucleo.Estado) {
			cambio := persistencia.Cambio{Codigo: nucleo.CodigoSalidaNoCumpleEsquema,
				Mensaje: nucleo.MensajeDe(errSalida), Uso: &resultado.Uso, ModeloEfectivo: resultado.ModeloEfectivo}
			if t.Solicitud.Salida.ConservarSalidaInvalida {
				cambio.Salida = &resultado.Salida // salida bruta conservada, con la retención del resultado
			}
			p.cambiar(id, desde, nucleo.Fallido, cambio)
		}
		// En drenaje no se pide otra corrección: el trabajo ya ejecutó y no puede volver a la cola.
		if correccion >= reintentos || p.ctx.Err() != nil {
			fallar(nucleo.ValidandoSalida)
			return
		}
		if !p.cambiar(id, nucleo.ValidandoSalida, nucleo.EsperandoAgente, persistencia.Cambio{Uso: &resultado.Uso, ModeloEfectivo: resultado.ModeloEfectivo}) {
			return
		}
		solicitud = conCorreccion(solicitud, resultado.Salida, problemas, nucleo.MensajeDe(errSalida))
		res, _, fin, err := p.intentar(ctxTrabajo, id, t, solicitud, elegido, nil, esperando, correccion+1)
		if errors.Is(err, errCorreccionDetenida) {
			fallar(nucleo.EsperandoAgente)
			return
		}
		if fin {
			return
		}
		uso := sumarUso(resultado.Uso, res.Uso)
		if err != nil {
			codigo := cmp.Or(nucleo.CodigoDe(err), nucleo.CodigoErrorAgente)
			p.cambiar(id, nucleo.Ejecutando, nucleo.Fallido, persistencia.Cambio{Codigo: codigo, Mensaje: nucleo.MensajeDe(err), Uso: &uso,
				ModeloEfectivo: cmp.Or(res.ModeloEfectivo, resultado.ModeloEfectivo)})
			return
		}
		resultado = res
		resultado.Uso = uso
	}
}

// intento es lo que guarda el evento intento: esfuerzo es el nivel que se pidió al CLI, ya ajustado a su máximo, o
// con DelAgente el que usa por su cuenta para ese modelo (vacío si no se conoce o el agente no tiene esfuerzo).
type intento struct {
	Agente    string `json:"agente"`
	Modelo    string `json:"modelo"`
	Esfuerzo  string `json:"esfuerzo"`
	DelAgente bool   `json:"del_agente,omitempty"`
	Contexto  int    `json:"contexto_tokens,omitempty"` // del modelo, si se conoce
	// Correccion es el número de corrección (desde 1) de Correcciones pedidas; 0 en un intento normal.
	Correccion   int `json:"correccion,omitempty"`
	Correcciones int `json:"correcciones,omitempty"`
}

// nuevoIntento arma el evento de un intento; versionVista resuelve un alias (o el modelo vacío) con la última versión
// que informó el agente, como el catálogo.
func nuevoIntento(c candidato, pedido string, versionVista func(agente, modelo string) string) intento {
	i := intento{Agente: c.agente, Modelo: c.modelo}
	niveles := agentes.NivelesDeModelo(c.agente, c.modelo)
	switch {
	case len(niveles) == 0:
	case pedido == "":
		i.Esfuerzo = agentes.EsfuerzoPredeterminado(c.agente, c.modelo)
		if i.Esfuerzo == "" {
			i.Esfuerzo = agentes.EsfuerzoPredeterminado(c.agente, versionVista(c.agente, c.modelo))
		}
		i.DelAgente = i.Esfuerzo != ""
	case slices.Contains(niveles, pedido):
		i.Esfuerzo = pedido
	default: // por encima de su máximo: el adaptador usa el máximo
		i.Esfuerzo = niveles[len(niveles)-1]
	}
	return i
}

// lineaSegura deja un texto del CLI en una sola línea y corta lo largo: un mensaje de error no puede falsificar
// líneas del registro del servicio.
func lineaSegura(texto string) string {
	texto = strings.NewReplacer("\r", " ", "\n", " ").Replace(texto)
	if r := []rune(texto); len(r) > 300 {
		texto = string(r[:300]) + "…"
	}
	return texto
}

// sumarUso junta el uso de dos intentos del mismo trabajo.
func sumarUso(a, b nucleo.Uso) nucleo.Uso {
	return nucleo.Uso{TokensEntrada: a.TokensEntrada + b.TokensEntrada, TokensSalida: a.TokensSalida + b.TokensSalida, CostoUSD: a.CostoUSD + b.CostoUSD}
}

// intentar ejecuta un candidato: toma sus semáforos, lo lanza y los suelta al volver. Con fin = true el trabajo ya
// quedó cerrado (cancelado, reencolado por un drenaje o fuera de plazo en cola) y no hay nada más que hacer; si no,
// err es el fallo del intento y huboTexto indica si el agente llegó a emitir texto.
func (p *Planificador) intentar(ctxTrabajo context.Context, id string, t nucleo.Trabajo, solicitud nucleo.SolicitudEjecucion,
	c candidato, plazoCola <-chan time.Time, esperando func(), correccion int) (resultado adaptadores.Resultado, huboTexto, fin bool, err error) {
	fondo := context.Background()
	// detenido cierra el trabajo que no llegó a lanzar el agente: se reencola. Una corrección no, porque el trabajo ya
	// ejecutó: el bucle de corrección la cierra con sus problemas (salvo que el usuario la cancelara).
	detenido := func() (adaptadores.Resultado, bool, bool, error) {
		if actual, err := p.almacen.Obtener(fondo, id); correccion > 0 && err == nil && actual.Estado == nucleo.EsperandoAgente {
			return resultado, false, false, errCorreccionDetenida
		}
		p.cerrarDetenido(id, nucleo.EsperandoAgente)
		return resultado, false, true, nil
	}
	// Primero el punto de acceso, después el adaptador y al final la plaza de la instancia: quien espera a su
	// agente no ocupa plaza, así que otro trabajo listo de otro agente puede empezar. Un canal atiende a sus
	// esperas por orden de llegada, así que cada agente conserva el orden de la cola.
	for _, semaforo := range []chan struct{}{p.semaforoPunto(t.PuntoAcceso), p.semaforos[c.agente], p.ranuras} {
		if semaforo == nil {
			continue
		}
		// Se avisa al despachador justo antes de la primera espera, no antes de preparar el intento: con la máquina
		// cargada, el siguiente trabajo podía llegar antes a su select y adelantarse en el semáforo del agente.
		// shortcut: queda la ventana entre este aviso y entrar en el select, mucho menor que lo que tarda el
		// siguiente trabajo en llegar a su espera (lee y escribe en la base); si no bastara, un turno por agente.
		// El coste: el despachador no saca otro trabajo mientras este prepara el intento, incluida la consulta del
		// contexto del modelo (hasta 5 s con Gemini la primera vez; luego queda en caché un día). Si eso frenara a
		// otros agentes, el turno por agente también lo resuelve.
		esperando()
		select {
		case semaforo <- struct{}{}:
			defer func() { <-semaforo }()
		case <-ctxTrabajo.Done():
			return detenido()
		case <-p.ctx.Done(): // en drenaje: nunca lanzó el agente, se reencola
			return detenido()
		case <-plazoCola:
			p.cambiar(id, nucleo.EsperandoAgente, nucleo.Fallido, persistencia.Cambio{Codigo: nucleo.CodigoTiempoColaAgotado, Mensaje: idioma.T("se superó el tiempo máximo en cola")})
			return resultado, false, true, nil
		}
	}
	if p.ctx.Err() != nil { // el drenaje empezó mientras se tomaban los semáforos
		return detenido()
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
	registro := nuevoIntento(c, solicitud.Esfuerzo, p.versionVista)
	registro.Contexto, _ = p.contextoDe(ctxTrabajo, c)
	if correccion > 0 {
		registro.Correccion, registro.Correcciones = correccion, max(t.Solicitud.Salida.Reintentos, correccion)
	}
	if datos, err := json.Marshal(registro); err == nil {
		p.evento(id, "intento", string(datos))
	}
	// Si el agente escribe una clave de proveedor (lo empujó un contenido hostil), se tapa antes de guardarla o enviarla.
	// shortcut: una clave partida entre dos fragmentos de texto solo se tapa en la salida final, no en los eventos.
	secretos := adaptadores.Secretos()
	agrupador := &agrupador{guardar: func(tipo, datos string) error {
		return p.almacen.AgregarEvento(fondo, id, tipo, adaptadores.Redactar(datos, secretos))
	}}
	var texto atomic.Bool                               // algunos adaptadores emiten desde la goroutine que lee la salida del CLI
	sinTextoEnVivo := herramientas.Activas(t.Solicitud) // el texto sería el sobre interno: la respuesta llega al terminar
	resultado, err = p.adaptadores[c.agente].Ejecutar(ctxEjecucion, solicitud, func(e adaptadores.Evento) error {
		if e.Tipo == "texto" && e.Datos != "" {
			texto.Store(true)
		}
		if e.Tipo == "texto" && sinTextoEnVivo {
			return nil
		}
		return agrupador.emitir(e)
	})
	if resultado.ContextoTokens > 0 && resultado.ModeloEfectivo != "" {
		_ = p.almacen.GuardarContexto(fondo, c.agente, resultado.ModeloEfectivo, resultado.ContextoTokens)
	}
	huboTexto = texto.Load()
	_ = agrupador.vaciar() // antes de cualquier cambio de estado: el texto queda completo y en orden
	resultado.Salida = adaptadores.Redactar(resultado.Salida, secretos)
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

// cambiar aplica una transición; si falla porque el usuario canceló entretanto, confirma la cancelación. Si la base no
// pudo guardarla (disco lleno, base ocupada o cerrada), lo registra y cierra el trabajo con error_interno, para que no
// quede en curso para siempre con sus clientes esperando.
func (p *Planificador) cambiar(id string, de, hacia nucleo.Estado, cambio persistencia.Cambio) bool {
	fondo := context.Background()
	err := p.almacen.CambiarEstado(fondo, id, de, hacia, cambio)
	if err == nil {
		return true
	}
	if nucleo.CodigoDe(err) != nucleo.CodigoTransicionInvalida {
		p.anotar("%s no se pudo pasar de %s a %s: %v", id, de, hacia, err)
		cierre := nucleo.Interrumpido
		if nucleo.ValidarTransicion(de, cierre) != nil {
			cierre = nucleo.Fallido
		}
		if hacia != cierre && nucleo.ValidarTransicion(de, cierre) == nil {
			fallo := persistencia.Cambio{Codigo: nucleo.CodigoErrorInterno, Mensaje: idioma.T("no se pudo guardar el estado del trabajo")}
			if err := p.almacen.CambiarEstado(fondo, id, de, cierre, fallo); err != nil {
				p.anotar("%s tampoco se pudo cerrar: %v", id, err)
			}
		}
		return false
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
		return nucleo.Documentos{}, nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("este espacio no admite cargas"))
	}
	select {
	case p.conversiones <- struct{}{}:
		defer func() { <-p.conversiones }()
	case <-ctx.Done():
		return nucleo.Documentos{}, ctx.Err()
	}
	if p.opc.TiempoConversion > 0 {
		var cancelar context.CancelFunc
		ctx, cancelar = context.WithTimeout(ctx, p.opc.TiempoConversion)
		defer cancelar()
	}
	docs, err := p.opc.Preparador.Preparar(ctx, cargas)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return docs, nucleo.NuevoError(nucleo.CodigoConversionFallida, idioma.T("la preparación de los documentos superó conversion.tiempo_total_segundos"))
	}
	return docs, err
}

// conDocumentos antepone las entradas preparadas al último mensaje del usuario sin alterar su texto;
// cada archivo va en su bloque delimitado con metadatos. En una corrección hecha a mano van con la pregunta, no con
// el pedido de corrección.
func conDocumentos(s nucleo.SolicitudEjecucion, documentos string) nucleo.SolicitudEjecucion {
	mensajes := slices.Clone(s.Mensajes)
	fin := len(mensajes)
	for fin >= 3 && mensajes[fin-2].Rol == "asistente" && mensajes[fin-1].Rol == "usuario" && strings.HasPrefix(mensajes[fin-1].Contenido, inicioCorreccion) {
		fin -= 2
	}
	for i := fin - 1; i >= 0; i-- {
		if mensajes[i].Rol == "usuario" {
			mensajes[i].Contenido = documentos + "\n" + mensajes[i].Contenido
			s.Mensajes = mensajes
			return s
		}
	}
	s.Mensajes = append(mensajes, nucleo.Mensaje{Rol: "usuario", Contenido: documentos})
	return s
}

// validarSalida comprueba el formato pedido y, si hay esquema, que la salida lo cumpla. Con herramientas, que la
// salida sea un sobre válido y que cada llamada cumpla el esquema de su herramienta.
func validarSalida(sol nucleo.SolicitudEjecucion, salida string) error {
	if herramientas.Activas(sol) {
		_, _, err := herramientas.Interpretar(sol, salida, "")
		return err
	}
	config := sol.Salida
	if len(config.Esquema) > 0 {
		return salidas.Validar(config.Esquema, salida)
	}
	if config.Formato == "json" {
		return salidas.Validar([]byte("true"), salida) // el esquema true admite cualquier JSON: solo se comprueba que lo sea
	}
	return nil
}
