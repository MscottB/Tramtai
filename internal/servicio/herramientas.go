package servicio

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/apuntar"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Herramientas de terceros (Claude Code, Codex, Continue, opencode, Kilo) apuntadas a Tramtai desde un espacio. Cada
// una usa su propia API, de nombre su id y en el grupo Herramientas, porque su clave solo vale en /p/<id>/…; su archivo
// lo edita internal/apuntar. La clave va al almacén del sistema (y el archivo lleva la orden «tramtai herramientas clave
// <id>») o, si la herramienta no sabe pedirla con una orden, dentro de su archivo.

const grupoHerramientas = "Herramientas"

// errSinAPI: la herramienta no tiene API en ese archivo (nunca se activó, o el usuario la borró en Mis APIs).
var errSinAPI = errors.New("sin API")

// NombreClaveHerramienta es el nombre de la clave de la herramienta en el almacén: TRAMTAI_HERRAMIENTA_CLAUDE_CODE.
func NombreClaveHerramienta(id string) string {
	return "TRAMTAI_HERRAMIENTA_" + strings.ToUpper(strings.ReplaceAll(id, "-", "_"))
}

// mismaRuta compara dos rutas de archivo como lo hace el sistema: en Windows, sin distinguir mayúsculas.
func mismaRuta(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (s *Servicio) puertoActual() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.opc.Puerto
}

// rutaHerramientas atiende /api/v1/herramientas…; en el modo servidor no hay herramientas: la cuenta del servicio no es
// la del usuario y no se escribe nada en ella.
func (s *Servicio) rutaHerramientas(w http.ResponseWriter, r *http.Request, inst *Instancia) {
	if s.entorno() == "servidor" {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoDisponibleEnServidor, idioma.T("las herramientas no están disponibles en el modo servidor: apúntalas desde el equipo de cada persona")))
		return
	}
	resto := strings.TrimPrefix(r.URL.Path, "/api/v1/herramientas")
	id, accion, _ := strings.Cut(strings.TrimPrefix(resto, "/"), "/")
	var err error
	switch {
	case resto == "" && r.Method == http.MethodGet:
	case resto == "/activar" && r.Method == http.MethodPost:
		var p contrato.ActivarHerramientas
		if err = api.LeerJSON(w, r, &p); err == nil {
			err = s.activarHerramientas(inst, p)
		}
	case resto == "/desactivar" && r.Method == http.MethodPost:
		var p struct {
			IDs   []string `json:"ids"`
			Todas bool     `json:"todas"`
		}
		if err = api.LeerJSON(w, r, &p); err == nil {
			err = s.desactivarHerramientas(inst, p.IDs, p.Todas)
		}
	case (accion == "olvidar" || accion == "restaurar") && r.Method == http.MethodPost:
		h, err := buscarHerramienta(id)
		if err != nil {
			api.EscribirError(w, r, err)
			return
		}
		var copia string
		if accion == "olvidar" {
			if err = s.olvidarHerramienta(h); err == nil {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		} else if copia, err = s.restaurarHerramienta(h); err == nil {
			api.EscribirJSON(w, http.StatusOK, map[string]any{"copia": copia})
			return
		}
		api.EscribirError(w, r, errorHerramienta(h, err))
		return
	default:
		err = nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, r.URL.Path))
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	lista := []contrato.EstadoHerramienta{}
	for _, h := range apuntar.Lista() {
		lista = append(lista, s.estadoHerramienta(inst, h))
	}
	api.EscribirJSON(w, http.StatusOK, map[string]any{"herramientas": lista})
}

func buscarHerramienta(id string) (apuntar.Herramienta, error) {
	for _, h := range apuntar.Lista() {
		if h.ID == id {
			return h, nil
		}
	}
	return apuntar.Herramienta{ID: id, Nombre: id}, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("herramienta desconocida: %s", id))
}

// errorHerramienta da a un error de internal/apuntar su código y el nombre de la herramienta.
func errorHerramienta(h apuntar.Herramienta, err error) error {
	switch {
	case errors.Is(err, apuntar.ErrCambioMientras):
		return nucleo.NuevoError(nucleo.CodigoHerramientaEnConflicto, idioma.Tf("%s: su archivo cambió mientras Tramtai lo escribía: vuelve a intentarlo", h.Nombre))
	case errors.Is(err, apuntar.ErrOtraActivacion):
		return nucleo.NuevoError(nucleo.CodigoHerramientaEnConflicto, h.Nombre+": "+err.Error())
	case nucleo.CodigoDe(err) != "":
		return nucleo.NuevoError(nucleo.CodigoDe(err), h.Nombre+": "+nucleo.MensajeDe(err))
	}
	return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, h.Nombre+": "+err.Error())
}

// unirErrores devuelve el primero con su código y los mensajes de todos.
func unirErrores(errs []error) error {
	if len(errs) <= 1 {
		return errors.Join(errs...)
	}
	mensajes := []string{}
	for _, err := range errs {
		mensajes = append(mensajes, nucleo.MensajeDe(err))
	}
	return nucleo.NuevoError(nucleo.CodigoDe(errs[0]), strings.Join(mensajes, "; "))
}

// alcanceDe es dónde se apunta la herramienta desde el espacio: en un proyecto, su archivo del proyecto; soloGlobal si
// no tiene (entonces solo se activa desde el espacio global).
func alcanceDe(inst *Instancia, h apuntar.Herramienta) (alcance apuntar.Alcance, proyecto string, soloGlobal bool) {
	if inst.Ambito == "proyecto" {
		return apuntar.AlcanceProyecto, inst.Raiz, !h.EnProyecto
	}
	return apuntar.AlcanceGlobal, "", false
}

// deEsteEspacio dice si la herramienta se apunta desde este espacio y no está activa en otro archivo: «todas» solo
// llega a esas.
func (s *Servicio) deEsteEspacio(inst *Instancia, h apuntar.Herramienta) bool {
	alcance, proyecto, soloGlobal := alcanceDe(inst, h)
	if soloGlobal {
		return false
	}
	ruta, err := h.Ruta(alcance, proyecto)
	a, activa, errLeer := apuntar.Leer(s.opc.Datos, h.ID)
	return err == nil && errLeer == nil && (!activa || mismaRuta(a.Ruta, ruta))
}

// elegirHerramientas son las de ids o, con todas, las que cumplen incluir.
func elegirHerramientas(ids []string, todas bool, incluir func(apuntar.Herramienta) bool) ([]apuntar.Herramienta, error) {
	if (len(ids) == 0) == !todas {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("indica las herramientas o todas, no ambas"))
	}
	var lista []apuntar.Herramienta
	for _, h := range apuntar.Lista() {
		if todas && incluir(h) {
			lista = append(lista, h)
		}
	}
	for _, id := range ids {
		h, err := buscarHerramienta(id)
		if err != nil {
			return nil, err
		}
		lista = append(lista, h)
	}
	return lista, nil
}

// modeloPorDefecto es el primer agente conectado y habilitado para aplicaciones, en el orden de sus fichas, con el modelo
// predeterminado de su CLI.
func (s *Servicio) modeloPorDefecto(inst *Instancia) (string, error) {
	for _, f := range agentes.Todos() {
		if s.opc.Registro[f.Nombre] != nil && !s.opc.Conexiones.Desconectado(f.Nombre) && agentes.PermitidoEnApps(inst.Config, f.Nombre) {
			return f.Nombre, nil
		}
	}
	return "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("no hay ningún agente conectado: conecta uno en Agentes o elige el modelo"))
}

// sinHTTPPropio: con el HTTPS propio, el puerto principal solo habla TLS y el oyente HTTP de este equipo cambia de puerto
// en cada arranque, así que la URL http://127.0.0.1:<puerto> que se escribe en la herramienta no funcionaría.
func (s *Servicio) sinHTTPPropio() string {
	if s.opc.TLS == nil {
		return ""
	}
	return idioma.T("con HTTPS propio activado, las herramientas no pueden usar Tramtai en este equipo: desactiva el HTTPS propio o pon Tramtai detrás de un proxy")
}

func (s *Servicio) activarHerramientas(inst *Instancia, p contrato.ActivarHerramientas) error {
	s.muHerramientas.Lock()
	defer s.muHerramientas.Unlock()
	if motivo := s.sinHTTPPropio(); motivo != "" {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, motivo)
	}
	modelo := strings.TrimSpace(p.Modelo)
	var err error
	if modelo == "" {
		modelo, err = s.modeloPorDefecto(inst)
	} else if err = s.modeloDePunto(inst, &ConfigPunto{}, modelo); err == nil { // solo valida: un agente que existe o un perfil
		// las peticiones de la herramienta llegan por su API, como las de una aplicación
		if agente, _, _ := strings.Cut(modelo, "/"); !agentes.PermitidoEnApps(inst.Config, agente) {
			err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s no está habilitado para aplicaciones: todavía no ha pasado su prueba de seguridad. Úsalo desde Probar o la terminal", agentes.NombreVisible(agente)))
		}
	}
	if err != nil {
		return err
	}
	lista, err := elegirHerramientas(p.IDs, p.Todas, func(h apuntar.Herramienta) bool { return apuntar.Detectada(h) && s.deEsteEspacio(inst, h) })
	if err != nil {
		return err
	}
	var errs []error
	for _, h := range lista {
		if err := s.activarHerramienta(inst, h, modelo); err != nil {
			errs = append(errs, errorHerramienta(h, err))
		}
	}
	s.Recargar()
	_ = s.persistirEstado() // anotado; las APIs ya están en su archivo
	return unirErrores(errs)
}

// activarHerramienta crea (o reanuda) la API de la herramienta, le da una clave nueva y apunta su archivo a ella.
func (s *Servicio) activarHerramienta(inst *Instancia, h apuntar.Herramienta, modelo string) error {
	alcance, proyecto, soloGlobal := alcanceDe(inst, h)
	if soloGlobal {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("solo se activa desde el espacio global"))
	}
	ruta, err := h.Ruta(alcance, proyecto)
	if err != nil {
		return err
	}
	// Antes de crear la API: un rechazo no deja nada a medias. Aplicar lo vuelve a comprobar.
	if a, activa, err := apuntar.Leer(s.opc.Datos, h.ID); err != nil {
		return err
	} else if activa && !mismaRuta(a.Ruta, ruta) {
		return fmt.Errorf("%w (%s)", apuntar.ErrOtraActivacion, a.Ruta)
	}
	if h.Previa != nil {
		_, err := os.Stat(ruta)
		if err := h.Previa(ruta, err == nil); err != nil {
			return err
		}
	}
	instancia := ""
	if inst.ID != "global" {
		instancia = inst.ID
	}
	if err := prepararAPI(instancia, h.ID, modelo); err != nil {
		return err
	}
	clave, err := NuevaClave(RutaTokenPunto(s.opc.Datos, h.ID))
	if err != nil {
		_ = detenerAPI(h.ID)
		return err
	}
	puerto := s.puertoActual() // sin HTTPS propio (sinHTTPPropio): el puerto principal habla HTTP en 127.0.0.1
	base := "http://127.0.0.1:" + strconv.Itoa(puerto) + "/p/" + h.ID
	v := apuntar.Valores{URLAnthropic: base + "/anthropic", URLOpenAI: base + "/openai/v1", Modelo: modelo, Puerto: puerto}
	if h.ClaveEnArchivo {
		v.Clave = clave
	} else {
		exe, err := s.ejecutable()
		if err == nil {
			err = s.guardarClave(NombreClaveHerramienta(h.ID), clave)
		}
		if err != nil {
			_ = detenerAPI(h.ID)
			return err
		}
		v.Orden = []string{exe, "herramientas", "clave", h.ID}
	}
	if _, err := apuntar.Aplicar(s.opc.Datos, h, alcance, proyecto, v); err != nil {
		_ = detenerAPI(h.ID) // sin archivo que la use, la API no queda en marcha
		return err
	}
	return nil
}

// Las APIs de las herramientas van todas en la configuración global, también la de un proyecto (con el campo instancia
// de puntos_acceso, que la ata a su espacio): el local.yaml de un proyecto solo admite APIs si el proyecto es de confianza.
func editarAPIs(f func([]ConfigPunto) ([]ConfigPunto, error)) error {
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	return configuracion.EditarPuntos(ruta, configuracion.OrigenGlobal, f)
}

// prepararAPI crea la API de la herramienta o la reanuda con el modelo y el espacio nuevos. Una API con ese nombre que no
// es de Herramientas es del usuario: no se toca.
func prepararAPI(instancia, id, modelo string) error {
	return editarAPIs(func(lista []ConfigPunto) ([]ConfigPunto, error) {
		i := slices.IndexFunc(lista, func(c ConfigPunto) bool { return c.Nombre == id })
		if i < 0 {
			return append(lista, ConfigPunto{Nombre: id, Grupo: grupoHerramientas, Instancia: instancia,
				Contratos: []string{"anthropic", "openai"}, ModelosPermitidos: []string{modelo}}), nil
		}
		if lista[i].Grupo != grupoHerramientas {
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("ya hay una API %s que no es de Herramientas: cámbiale el nombre en Mis APIs", id))
		}
		lista[i].Activo, lista[i].Instancia, lista[i].ModelosPermitidos = nil, instancia, []string{modelo}
		return lista, nil
	})
}

// apiHerramienta es la API de la herramienta (nil si no está o no es de Herramientas).
func apiHerramienta(id string) *ConfigPunto {
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return nil
	}
	lista, _ := leerArchivoPuntos(ruta, configuracion.OrigenGlobal)
	i := slices.IndexFunc(lista, func(c ConfigPunto) bool { return c.Nombre == id && c.Grupo == grupoHerramientas })
	if i < 0 {
		return nil
	}
	return &lista[i]
}

// detenerAPI desactiva la API de la herramienta; que no esté (el usuario la borró) no es un error.
func detenerAPI(id string) error {
	err := editarAPIs(func(lista []ConfigPunto) ([]ConfigPunto, error) {
		i := slices.IndexFunc(lista, func(c ConfigPunto) bool { return c.Nombre == id && c.Grupo == grupoHerramientas })
		if i < 0 {
			return nil, errSinAPI
		}
		lista[i].Activo = new(bool)
		return lista, nil
	})
	if errors.Is(err, errSinAPI) {
		return nil
	}
	return err
}

func (s *Servicio) desactivarHerramientas(inst *Instancia, ids []string, todas bool) error {
	s.muHerramientas.Lock()
	defer s.muHerramientas.Unlock()
	lista, err := elegirHerramientas(ids, todas, func(h apuntar.Herramienta) bool { return s.deEsteEspacio(inst, h) })
	if err != nil {
		return err
	}
	var errs []error
	for _, h := range lista {
		if err := s.desactivarHerramienta(h); err != nil {
			errs = append(errs, errorHerramienta(h, err))
		}
	}
	s.Recargar()
	_ = s.persistirEstado()
	return unirErrores(errs)
}

// quitarDelArchivo deshace lo de Tramtai en el archivo de la herramienta, si está activada.
func (s *Servicio) quitarDelArchivo(h apuntar.Herramienta) (apuntar.Anotacion, error) {
	a, activa, err := apuntar.Leer(s.opc.Datos, h.ID)
	if err == nil && activa {
		err = apuntar.Quitar(s.opc.Datos, h, a)
	}
	return a, err
}

// desactivarHerramienta quita lo de Tramtai del archivo de la herramienta y detiene su API; la clave sigue en el almacén.
// Si el archivo no se puede deshacer, no se detiene nada: la herramienta sigue funcionando.
func (s *Servicio) desactivarHerramienta(h apuntar.Herramienta) error {
	if _, err := s.quitarDelArchivo(h); err != nil {
		return err
	}
	return detenerAPI(h.ID)
}

// olvidarHerramienta es desactivar y, además, borrar su clave del almacén y la anotación: Tramtai deja de saber de ella.
// La API queda detenida, para que el usuario decida si borrarla en Mis APIs. Si el archivo no se pudo deshacer, se olvida
// igual y se avisa: lo que quede de Tramtai en él hay que quitarlo a mano (sin anotación, Restaurar ya no puede).
func (s *Servicio) olvidarHerramienta(h apuntar.Herramienta) error {
	s.muHerramientas.Lock()
	defer s.muHerramientas.Unlock()
	a, errQuitar := s.quitarDelArchivo(h)
	err := detenerAPI(h.ID)
	if err == nil {
		err = apuntar.Borrar(s.opc.Datos, h.ID)
	}
	if err == nil && !h.ClaveEnArchivo {
		err = s.borrarClave(NombreClaveHerramienta(h.ID))
	}
	s.Recargar()
	_ = s.persistirEstado()
	if err == nil && errQuitar != nil {
		err = nucleo.NuevoError(nucleo.CodigoHerramientaEnConflicto, idioma.Tf("Tramtai la olvidó, pero no pudo quitar lo suyo de %s: %s", a.Ruta, nucleo.MensajeDe(errQuitar)))
	}
	return err
}

// restaurarHerramienta devuelve el archivo de la herramienta a como estaba antes de activarla y detiene su API.
func (s *Servicio) restaurarHerramienta(h apuntar.Herramienta) (string, error) {
	s.muHerramientas.Lock()
	defer s.muHerramientas.Unlock()
	a, activa, err := apuntar.Leer(s.opc.Datos, h.ID)
	if err == nil && !activa {
		err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("no está activada: no hay nada que restaurar"))
	}
	if err != nil {
		return "", err
	}
	copia, err := apuntar.Restaurar(s.opc.Datos, a)
	if err != nil {
		return "", err
	}
	err = detenerAPI(h.ID)
	s.Recargar()
	_ = s.persistirEstado()
	return copia, err
}

// estadoHerramienta es la herramienta vista desde el espacio.
func (s *Servicio) estadoHerramienta(inst *Instancia, h apuntar.Herramienta) (e contrato.EstadoHerramienta) {
	e = contrato.EstadoHerramienta{ID: h.ID, Nombre: h.Nombre, ClaveEnArchivo: h.ClaveEnArchivo, Gestionado: h.Gestionado != nil && h.Gestionado()}
	defer func() {
		if texto := s.sinHTTPPropio(); texto != "" {
			if e.Estado == "activa" {
				e.Estado = "desactualizada"
			}
			ponerMotivo(&e, motivo{"sin_https_propio", nil, texto})
		} else if e.Gestionado && e.Motivo == "" {
			ponerMotivo(&e, motivo{"gestionado", nil, idioma.T("una política de la organización manda sobre su configuración: puede que no use lo que escriba Tramtai")})
		}
	}()
	alcance, proyecto, soloGlobal := alcanceDe(inst, h)
	if soloGlobal {
		alcance = apuntar.AlcanceGlobal
	}
	e.Ruta, _ = h.Ruta(alcance, proyecto)
	if soloGlobal {
		e.Estado = "solo_global"
		ponerMotivo(&e, motivo{"solo_global", nil, idioma.T("solo se activa desde el espacio global")})
		return e
	}
	a, activa, err := apuntar.Leer(s.opc.Datos, h.ID)
	if err != nil {
		e.Estado = "desactualizada"
		ponerMotivo(&e, motivo{"anotacion_ilegible", map[string]string{"detalle": err.Error()}, err.Error()})
		return e
	}
	if activa {
		e.Estado, e.Modelo, e.HayCopia = "activa", a.Modelo, !a.NoExistia
		if e.HayCopia {
			e.FechaCopia = a.Fecha.UTC().Format(time.RFC3339)
		}
		if m := s.motivoDesactualizada(h, a); m.codigo != "" {
			e.Estado = "desactualizada"
			ponerMotivo(&e, m)
		} else if !mismaRuta(a.Ruta, e.Ruta) {
			ponerMotivo(&e, motivo{"otro_archivo", map[string]string{"ruta": a.Ruta}, idioma.Tf("activada en otro archivo: %s", a.Ruta)})
		}
		e.Ruta = a.Ruta
		return e
	}
	e.Estado = "no_detectada"
	if p := apiHerramienta(h.ID); p != nil {
		e.Estado = "desactivada"
		if len(p.ModelosPermitidos) > 0 {
			e.Modelo = p.ModelosPermitidos[0]
		}
	}
	if e.Estado == "no_detectada" && apuntar.Detectada(h) {
		e.Estado = "detectada"
	}
	if h.Previa != nil {
		_, err := os.Stat(e.Ruta)
		if err := h.Previa(e.Ruta, err == nil); err != nil {
			codigo := "previa"
			if errors.Is(err, apuntar.ErrContinueJSON) {
				codigo = "continue_json"
			}
			ponerMotivo(&e, motivo{codigo, nil, err.Error()})
		}
	}
	return e
}

// motivo es por qué una herramienta no va: un código que la Web UI traduce a su idioma (con los datos que lleva el texto) y
// el texto en el idioma del servicio, que sirve a la CLI y a la terminal y de respaldo si la Web UI no conoce el código.
type motivo struct {
	codigo string
	datos  map[string]string
	texto  string
}

func ponerMotivo(e *contrato.EstadoHerramienta, m motivo) {
	e.Motivo, e.MotivoCodigo, e.MotivoDatos = m.texto, m.codigo, m.datos
}

// motivoDesactualizada dice por qué una activación ya no funciona ("" si funciona).
func (s *Servicio) motivoDesactualizada(h apuntar.Herramienta, a apuntar.Anotacion) motivo {
	switch p, puerto := apiHerramienta(a.API), s.puertoActual(); {
	case p == nil:
		return motivo{"api_no_existe", map[string]string{"api": a.API}, idioma.Tf("la API %s ya no existe: vuelve a activarla", a.API)}
	case !p.EstaActivo():
		return motivo{"api_detenida", map[string]string{"api": a.API}, idioma.Tf("la API %s está detenida en Mis APIs", a.API)}
	case a.Puerto != puerto:
		return motivo{"puerto", map[string]string{"actual": strconv.Itoa(puerto), "anotado": strconv.Itoa(a.Puerto)},
			idioma.Tf("Tramtai usa ahora el puerto %d y la herramienta apunta al %d: vuelve a activarla", puerto, a.Puerto)}
	}
	if a.Orden != "" {
		if _, err := os.Stat(a.Orden); err != nil {
			return motivo{"orden_no_existe", map[string]string{"ruta": a.Orden}, idioma.Tf("ya no existe %s, el Tramtai que da la clave: vuelve a activarla", a.Orden)}
		}
	}
	if !h.ClaveEnArchivo {
		if clave, _ := s.leerClave(NombreClaveHerramienta(h.ID)); clave == "" {
			return motivo{"clave_no_almacen", nil, idioma.T("su clave ya no está en el almacén del sistema: vuelve a activarla")}
		}
	}
	if actual, _ := os.ReadFile(a.Ruta); !h.Formato.Contiene(actual, a) {
		return motivo{"editada_a_mano", map[string]string{"ruta": a.Ruta}, idioma.Tf("se cambió a mano lo que escribió Tramtai en %s", a.Ruta)}
	}
	return motivo{}
}
