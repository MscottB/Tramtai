package servicio

import (
	"cmp"
	"errors"
	"net"
	"net/http"
	"strconv"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/autoarranque"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Rutas de /ui/ que cambian el servicio o el equipo; todas exigen una sesión local (rutasDeUI).

func (s *Servicio) activarGlobalUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	inst, err := s.ActivarGlobal()
	if err == nil {
		err = s.persistirEstado()
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.responderSesion(w, r, s.derivarSesion(w, inst.ID, ses))
}

func (s *Servicio) abrirProyectoUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	var peticion struct {
		Raiz      string `json:"raiz"`
		Confiar   *bool  `json:"confiar"`
		CrearAqui bool   `json:"crear_aqui"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	inst, err := s.abrirYGuardar(peticion.Raiz, AbrirOpciones{Confiar: peticion.Confiar, CrearAqui: peticion.CrearAqui})
	if escribirErrorAbrir(w, r, err) {
		return
	}
	s.responderSesion(w, r, s.derivarSesion(w, inst.ID, ses))
}

// abrirYGuardar abre el proyecto y deja guardados el registro y el descriptor.
func (s *Servicio) abrirYGuardar(raiz string, o AbrirOpciones) (*Instancia, error) {
	inst, err := s.AbrirProyecto(raiz, o)
	if err == nil {
		err = s.persistirEstado()
	}
	return inst, err
}

// escribirErrorAbrir responde al error de abrir un proyecto y dice si lo hizo (err no era nil). Las dos preguntas
// (confianza y carpeta dentro de otro proyecto) son un 409 con lo que la app enseña para decidir.
func escribirErrorAbrir(w http.ResponseWriter, r *http.Request, err error) bool {
	var confianza *ErrorConfianza
	var dentro *ErrorDentroDeProyecto
	switch {
	case errors.As(err, &confianza):
		api.EscribirJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"codigo": nucleo.CodigoConfianzaRequerida,
			"mensaje": nucleo.MensajeDe(err)}, "claves": append([]string{}, confianza.Claves...), "movido": confianza.Movido})
	case errors.As(err, &dentro):
		api.EscribirJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"codigo": nucleo.CodigoDentroDeProyecto,
			"mensaje": nucleo.MensajeDe(err)}, "proyecto": dentro.Proyecto})
	case err != nil:
		api.EscribirError(w, r, err)
	}
	return err != nil
}

// peticionArranque es lo que piden /ui/autoarranque y PUT /api/v1/servicio/autoarranque: activar o desactivar el arranque
// y, al activarlo, el modo (app o servicio; sin modo, la app).
type peticionArranque struct {
	Activo bool   `json:"activo"`
	Modo   string `json:"modo"`
}

func (s *Servicio) autoarranqueUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	var peticion peticionArranque
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := s.fijarAutoarranque(peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.responderSesion(w, r, ses)
}

// fijarAutoarranque registra que Tramtai arranque al iniciar sesión, la app en la bandeja o solo el servicio, o lo quita
// (Opciones.CambiarAutoarranque lo cambia).
func (s *Servicio) fijarAutoarranque(p peticionArranque) error {
	var modo autoarranque.Modo
	if p.Activo {
		var err error
		if modo, err = autoarranque.ModoDe(p.Modo); err != nil {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, err.Error())
		}
	}
	if s.opc.CambiarAutoarranque != nil {
		return s.opc.CambiarAutoarranque(modo)
	}
	var err error
	if modo == "" {
		err = autoarranque.Desactivar()
	} else {
		err = autoarranque.Elegir(modo)
	}
	if err != nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, err.Error())
	}
	return nil
}

// ajustesServicio son el puerto y el arranque con el sistema, para Ajustes en la terminal y la sesión de la Web UI.
func (s *Servicio) ajustesServicio() contrato.AjustesServicio {
	s.mu.RLock()
	a := contrato.AjustesServicio{Puerto: s.opc.Puerto, PuertoConfigurado: cmp.Or(s.opc.PuertoConfigurado, s.opc.Puerto),
		PuertoAutomatico: s.opc.PuertoAutomatico, Reiniciable: s.opc.Reiniciar != nil}
	s.mu.RUnlock()
	if disponible, modo := s.estadoAutoarranque(); disponible {
		activo := modo != ""
		a.Autoarranque, a.ModoArranque = &activo, string(modo)
	}
	return a
}

func (s *Servicio) autenticacionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	var peticion struct {
		Agente string `json:"agente"`
		Modo   string `json:"modo"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := s.cambiarAutenticacion(peticion.Agente, peticion.Modo); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// conexionUI conecta o desconecta un agente: {agente, conectado}.
func (s *Servicio) conexionUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	var peticion struct {
		Agente    string `json:"agente"`
		Conectado bool   `json:"conectado"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := s.Conectar(peticion.Agente, peticion.Conectado); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Conectar conecta o desconecta un agente: lo guarda en la configuración global (adaptadores.<agente>.desconectado) y
// vale al momento en todos los espacios.
func (s *Servicio) Conectar(agente string, conectado bool) error {
	if s.opc.Registro[agente] == nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("no hay ningún agente %s", agente))
	}
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	if err := configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "adaptadores."+agente+".desconectado", !conectado); err != nil {
		return err
	}
	s.opc.Conexiones.Fijar(agente, !conectado)
	return nil
}

func (s *Servicio) puertoUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	var peticion struct {
		Puerto     int  `json:"puerto"`
		Automatico bool `json:"automatico"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := s.fijarPuerto(peticion.Puerto, peticion.Automatico); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.responderSesion(w, r, ses)
}

func (s *Servicio) reiniciarUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	if s.opc.Reiniciar == nil {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("este servicio no se puede reiniciar desde la Web UI")))
		return
	}
	// Un segundo «Reiniciar» (otra pestaña, la bandeja) no lanza otro proceso: responde lo mismo que el primero.
	if s.reiniciando.CompareAndSwap(false, true) {
		if err := s.opc.Reiniciar(); err != nil {
			s.reiniciando.Store(false)
			api.EscribirError(w, r, err)
			return
		}
	}
	s.mu.RLock()
	puerto := puertoTrasReiniciar(s.opc.Host, s.opc.Puerto, s.opc.PuertoConfigurado)
	s.mu.RUnlock()
	// El servicio nuevo espera a que este termine y escucha en el puerto configurado (o en otro libre).
	api.EscribirJSON(w, http.StatusAccepted, map[string]any{"url": "http://" + net.JoinHostPort(s.opc.Host, strconv.Itoa(puerto))})
	s.detenerUna.Do(func() { close(s.detener) })
}

func (s *Servicio) detenerUI(w http.ResponseWriter, _ *http.Request, _ *sesion) {
	api.EscribirJSON(w, http.StatusAccepted, map[string]any{"estado": "deteniendo"})
	s.detenerUna.Do(func() { close(s.detener) })
}

// cambiarAutenticacion pasa un agente que admite la suscripción a suscripción o clave de API: lo guarda en la
// configuración global y lo aplica al servicio en marcha, sin reiniciarlo.
func (s *Servicio) cambiarAutenticacion(agente, modo string) error {
	if !agentes.AdmiteSuscripcion(agente) {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s no admite la suscripción", agente))
	}
	if modo != adaptadores.AutenticacionClaveAPI && modo != adaptadores.AutenticacionSuscripcion {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("modo de autenticación desconocido: %s", modo))
	}
	if ip := net.ParseIP(s.opc.Host); modo == adaptadores.AutenticacionSuscripcion && s.opc.Host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.T("la suscripción no se permite con el servicio abierto a otros equipos"))
	}
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	if err := configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "adaptadores."+agente+".autenticacion", modo); err != nil {
		return err
	}
	return adaptadores.CambiarAutenticacion(s.opc.Registro, agente, modo)
}

// guardarValores escribe valores en la configuración de la instancia (la global o el local.yaml del proyecto) y los
// aplica en marcha; en la global alcanzan a todas las instancias que no tengan los suyos.
func (s *Servicio) guardarValores(inst *Instancia, valores map[string]any) error {
	ruta, origen, err := archivoPuntos(inst)
	if err != nil {
		return err
	}
	for clave, valor := range valores {
		if err := configuracion.FijarValor(ruta, origen, clave, valor); err != nil {
			return err
		}
	}
	if inst.Ambito != "global" {
		inst.Config.Fijar(origen, valores)
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, otra := range s.instancias {
		otra.Config.Fijar(origen, valores)
	}
	return nil
}

// fijarPuerto guarda servicio.puerto y servicio.puerto_automatico en la configuración global si el puerto está
// libre (o es el que ya usa este servicio); con puerto 0 solo cambia el automático. Se aplica al reiniciar.
func (s *Servicio) fijarPuerto(puerto int, automatico bool) error {
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	if puerto == 0 {
		err = configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "servicio.puerto_automatico", automatico)
	} else {
		err = FijarPuertoPropio(ruta, s.opc.Host, puerto, s.opc.Puerto, automatico)
	}
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.opc.PuertoConfigurado, s.opc.PuertoAutomatico = cmp.Or(puerto, s.opc.PuertoConfigurado), automatico
	s.mu.Unlock()
	return nil
}

// puertoTrasReiniciar es donde escuchará el servicio nuevo: el configurado si cambió y está libre; si no, el actual. Si
// en el configurado escucha otro Tramtai (la instalada mientras esta es la portable), la Web UI no salta a él: el
// servicio nuevo, que lo encontrará ocupado, vuelve a buscar en orden y cae otra vez en este.
func puertoTrasReiniciar(host string, actual, configurado int) int {
	if configurado != 0 && configurado != actual && PuertoLibre(host, configurado) {
		return configurado
	}
	return actual
}
