package servicio

import (
	"net"
	"net/http"
	"strconv"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/autoarranque"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Rutas de /ui/ que cambian el servicio o el equipo; todas exigen una sesión local (rutasDeUI).

func (s *Servicio) activarGlobalUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	inst, err := s.ActivarGlobal()
	if err == nil {
		err = s.guardarActivas()
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.responderSesion(w, r, s.derivarSesion(w, inst.ID, ses))
}

func (s *Servicio) abrirProyectoUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	var peticion struct {
		Raiz string `json:"raiz"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	inst, err := s.AbrirProyecto(peticion.Raiz)
	if err == nil {
		err = s.guardarActivas()
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.responderSesion(w, r, s.derivarSesion(w, inst.ID, ses))
}

func (s *Servicio) autoarranqueUI(w http.ResponseWriter, r *http.Request, ses *sesion) {
	var peticion struct {
		Activo bool `json:"activo"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	err := autoarranque.Desactivar()
	if peticion.Activo {
		var escritorio string
		if escritorio, err = autoarranque.Escritorio(); err == nil {
			err = autoarranque.Activar(escritorio)
		}
	}
	if err != nil {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, err.Error()))
		return
	}
	s.responderSesion(w, r, ses)
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
	if err := s.opc.Reiniciar(); err != nil {
		api.EscribirError(w, r, err)
		return
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
	ruta, err := nucleo.RutaGlobal()
	if err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "adaptadores."+agente+".autenticacion", modo); err != nil {
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
		if err := FijarValor(ruta, origen, clave, valor); err != nil {
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
// libre (o es el que ya usa este servicio). Se aplica al reiniciar.
func (s *Servicio) fijarPuerto(puerto int, automatico bool) error {
	if puerto < 1024 || puerto > 65535 {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el puerto debe estar entre 1024 y 65535"))
	}
	if puerto != s.opc.Puerto && !PuertoLibre(s.opc.Host, puerto) {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("el puerto %d está ocupado por otro programa: elige otro", puerto))
	}
	ruta, err := nucleo.RutaGlobal()
	if err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto", puerto); err != nil {
		return err
	}
	if err := FijarValor(ruta, nucleo.OrigenGlobal, "servicio.puerto_automatico", automatico); err != nil {
		return err
	}
	s.mu.Lock()
	s.opc.PuertoConfigurado, s.opc.PuertoAutomatico = puerto, automatico
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
