package servicio

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// rutaInstancia atiende lo que la Web UI necesita además de la API de la instancia: su estado y, con una sesión
// de la Web UI o el token de servicio (no con el token que usan las aplicaciones), su configuración efectiva y sus
// puntos de acceso. Devuelve false si la ruta no es suya.
func (s *Servicio) rutaInstancia(w http.ResponseWriter, r *http.Request, inst *Instancia, privilegiada bool) bool {
	ruta := r.URL.Path
	if ruta == "/api/v1/instancia" && r.Method == http.MethodGet {
		n, _ := inst.Planificador.ContarOrigenDesde(r.Context(), nucleo.DesdeApp, true, time.Now().Add(-24*time.Hour))
		e := inst.estado(r.Context())
		api.EscribirJSON(w, http.StatusOK, contrato.ResumenInstancia{EstadoInstancia: e, Base: s.Base(),
			AppConClave24h: n, SugerirApiPropia: sugerirApiPropia(n), Contadores: contadores(e.Trabajos)})
		return true
	}
	if !strings.HasPrefix(ruta, "/api/v1/instancia/") && ruta != "/api/v1/puntos" && !strings.HasPrefix(ruta, "/api/v1/puntos/") {
		return false
	}
	if !privilegiada {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado, idioma.T("requiere una sesión de la Web UI o la clave del servicio")))
		return true
	}
	switch {
	case ruta == "/api/v1/instancia/pasos" && r.Method == http.MethodGet:
		api.EscribirJSON(w, http.StatusOK, s.primerosPasos(r.Context(), inst, mismoUsuarioLocal(r)))
	case ruta == "/api/v1/instancia/token" && r.Method == http.MethodGet:
		// Para mostrar en «Conectar» la clave que usan las aplicaciones; solo con sesión o token de servicio.
		api.EscribirJSON(w, http.StatusOK, map[string]any{"token": s.tokenDe(inst), "archivo": inst.ArchivoToken})
	case ruta == "/api/v1/instancia/token" && r.Method == http.MethodPost:
		// Regenera la clave que usan las aplicaciones (por si se filtró); vale al momento, sin reiniciar.
		token, err := s.RotarTokenInstancia(inst)
		if err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		api.EscribirJSON(w, http.StatusOK, map[string]any{"token": token, "archivo": inst.ArchivoToken})
	case ruta == "/api/v1/instancia/configuracion" && r.Method == http.MethodGet:
		entradas := []EntradaConfiguracion{}
		for _, e := range inst.Config.Efectiva() {
			entradas = append(entradas, EntradaConfiguracion{Clave: e.Clave, Valor: e.Valor, Origen: e.Origen.String()})
		}
		api.EscribirJSON(w, http.StatusOK, map[string]any{"configuracion": entradas})
	case ruta == "/api/v1/instancia/heredar" && r.Method == http.MethodPost:
		// «Volver al de Global»: quita del local.yaml del proyecto un valor que la app deja cambiar por espacio.
		var peticion struct {
			Clave string `json:"clave"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		if err := s.heredar(inst, peticion.Clave); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	case ruta == "/api/v1/instancia/predeterminado" && r.Method == http.MethodPost:
		// esfuerzo es opcional: sin él no cambia; "" vuelve al del agente.
		var peticion struct {
			Modelo   string  `json:"modelo"`
			Esfuerzo *string `json:"esfuerzo"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		if err := s.fijarPredeterminado(inst, peticion.Modelo, peticion.Esfuerzo); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	case ruta == "/api/v1/puntos" && r.Method == http.MethodGet:
		uso, _ := inst.Planificador.UsoPorPunto(r.Context()) // sin uso, la lista se muestra igual
		puntos := s.puntosDe(inst.ID)
		presupuestos := map[string]nucleo.EstadoPresupuesto{}
		for _, p := range puntos {
			if estado, ok, err := inst.Planificador.EstadoPresupuesto(r.Context(), p.Nombre); ok && err == nil {
				presupuestos[p.Nombre] = estado
			}
		}
		api.EscribirJSON(w, http.StatusOK, map[string]any{"puntos_acceso": puntos, "uso": uso, "presupuestos": presupuestos})
	case ruta == "/api/v1/instancia/retencion" && r.Method == http.MethodGet:
		uso, err := inst.almacen.UsoHistorial(r.Context(), inst.ID)
		if err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		resultado, metadatos, maximo, _ := inst.retencion()
		api.EscribirJSON(w, http.StatusOK, map[string]any{"horas_resultado": resultado, "horas_metadatos": metadatos,
			"max_trabajos": maximo, "uso": uso})
	case ruta == "/api/v1/instancia/retencion" && r.Method == http.MethodPut:
		// Punteros: un campo que falta no puede valer 0, que borraría el historial en la purga inmediata.
		var peticion struct {
			HorasResultado *int `json:"horas_resultado"`
			HorasMetadatos *int `json:"horas_metadatos"`
			MaxTrabajos    *int `json:"max_trabajos"`
		}
		if err := api.LeerJSON(w, r, &peticion); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		if peticion.HorasResultado == nil || peticion.HorasMetadatos == nil || peticion.MaxTrabajos == nil {
			api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("faltan horas_resultado, horas_metadatos o max_trabajos")))
			return true
		}
		if err := s.fijarRetencion(inst, *peticion.HorasResultado, *peticion.HorasMetadatos, *peticion.MaxTrabajos); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	case ruta == "/api/v1/instancia/presupuesto" && r.Method == http.MethodGet:
		estado, ok, err := inst.Planificador.EstadoPresupuesto(r.Context(), persistencia.TodaLaInstancia)
		switch {
		case err != nil:
			api.EscribirError(w, r, err)
		case ok:
			api.EscribirJSON(w, http.StatusOK, map[string]any{"presupuesto": estado})
		default:
			api.EscribirJSON(w, http.StatusOK, map[string]any{})
		}
	case ruta == "/api/v1/instancia/presupuesto/reiniciar" && r.Method == http.MethodPost:
		if err := inst.Planificador.ReiniciarPresupuesto(r.Context(), persistencia.TodaLaInstancia); err != nil {
			api.EscribirError(w, r, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
	case ruta == "/api/v1/puntos" && r.Method == http.MethodPost:
		s.crearPuntoUI(w, r, inst)
	case strings.HasPrefix(ruta, "/api/v1/puntos/"):
		s.puntoUI(w, r, inst, strings.TrimPrefix(ruta, "/api/v1/puntos/"))
	default:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, ruta)))
	}
	return true
}

func (s *Servicio) puntosDe(instancia string) []EstadoPunto {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lista := []EstadoPunto{}
	for _, p := range s.estadoPuntos() {
		if p.Instancia == instancia {
			lista = append(lista, p)
		}
	}
	return lista
}

// heredables son los prefijos de las claves que la app deja tener propias en un proyecto y devolver a las de Global.
var heredables = []string{"predeterminados.", "retencion.", "presupuesto."}

// heredar quita clave del local.yaml de un proyecto y le vuelve a aplicar su configuración: vale la de Global.
func (s *Servicio) heredar(inst *Instancia, clave string) error {
	if inst.Ambito != "proyecto" {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("solo un proyecto hereda valores del espacio global"))
	}
	if !slices.ContainsFunc(heredables, func(p string) bool { return strings.HasPrefix(clave, p) }) {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s no se puede devolver al valor de Global desde aquí", clave))
	}
	if err := configuracion.QuitarValor(filepath.Join(inst.Raiz, ".tramtai", "local.yaml"), configuracion.OrigenLocal, clave); err != nil {
		return err
	}
	c, err := configuracion.CargarConfiguracion(inst.Raiz, agentes.Internos())
	if err != nil {
		return err
	}
	inst.Config.Reemplazar(c)
	return nil
}

// archivoPuntos es donde se guardan los puntos nuevos de la instancia: la configuración global para la global y
// .tramtai/local.yaml para un proyecto (nunca la versionada).
func archivoPuntos(inst *Instancia) (string, configuracion.Origen, error) {
	if inst.Ambito == "global" {
		ruta, err := configuracion.RutaGlobal()
		return ruta, configuracion.OrigenGlobal, err
	}
	return filepath.Join(inst.Raiz, ".tramtai", "local.yaml"), configuracion.OrigenLocal, nil
}

func origenDeArchivo(ruta string) configuracion.Origen {
	if strings.HasSuffix(filepath.ToSlash(ruta), ".tramtai/local.yaml") {
		return configuracion.OrigenLocal
	}
	return configuracion.OrigenGlobal
}

func (s *Servicio) crearPuntoUI(w http.ResponseWriter, r *http.Request, inst *Instancia) {
	var p ConfigPunto
	if err := api.LeerJSON(w, r, &p); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	ruta, origen, err := archivoPuntos(inst)
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	p.Instancia, p.Activo = "", nil
	if origen == configuracion.OrigenGlobal && inst.ID != "global" {
		p.Instancia = inst.ID
	}
	if _, err := normalizarPunto(p, inst.ID, s.opc.Puerto); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if err := validarPunto(p, s.existe); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	err = configuracion.EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		if slices.ContainsFunc(lista, func(c ConfigPunto) bool { return c.Nombre == p.Nombre }) {
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("la API %s ya existe", p.Nombre))
		}
		return append(lista, p), nil
	})
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	// Clave nueva siempre (aunque quedara una de otro punto con el mismo nombre); solo se devuelve esta vez.
	token, err := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, p.Nombre))
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.Recargar()
	_ = s.persistirEstado() // anotado; el punto ya está en su archivo
	for _, e := range s.puntosDe(inst.ID) {
		if e.Nombre == p.Nombre {
			api.EscribirJSON(w, http.StatusCreated, map[string]any{"punto_acceso": e, "token": token})
			return
		}
	}
	api.EscribirJSON(w, http.StatusCreated, map[string]any{"token": token})
}

// puntoUI cambia los ajustes (PATCH), elimina (DELETE, con su clave) o regenera la clave (POST <nombre>/token) de
// un punto de la instancia de la sesión.
func (s *Servicio) puntoUI(w http.ResponseWriter, r *http.Request, inst *Instancia, resto string) {
	nombre, accion, _ := strings.Cut(resto, "/")
	var punto *EstadoPunto
	for _, p := range s.puntosDe(inst.ID) {
		if p.Nombre == nombre {
			punto = &p
			break
		}
	}
	if punto == nil {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("API no encontrada en este espacio: %s", nombre)))
		return
	}
	switch {
	case accion == "presupuesto/reiniciar" && r.Method == http.MethodPost:
		if err := inst.Planificador.ReiniciarPresupuesto(r.Context(), nombre); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case accion == "token" && r.Method == http.MethodPost:
		// Regenera la clave: solo se guarda su huella, así que esta es la única vez que se puede ver.
		token, err := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, nombre))
		if err != nil {
			api.EscribirError(w, r, err)
			return
		}
		s.Recargar()
		api.EscribirJSON(w, http.StatusOK, map[string]any{"token": token})
	case accion == "" && r.Method == http.MethodPatch:
		s.cambiarPuntoUI(w, r, inst, punto)
	case accion == "" && r.Method == http.MethodDelete:
		// Archivo, clave y consumo antes de recargar (404 si ya no estaba); la CLI y la terminal también borran por aquí.
		if err := s.eliminarPunto(punto.Origen, origenDeArchivo(punto.Origen), nombre); err != nil {
			api.EscribirError(w, r, err)
			return
		}
		s.Recargar()
		_ = s.persistirEstado() // anotado; el punto ya está en su archivo
		w.WriteHeader(http.StatusNoContent)
	default:
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ruta no encontrada: %s %s", r.Method, r.URL.Path)))
	}
}

// modeloDePunto fija el modelo de un punto desde un identificador del catálogo (agente, agente/modelo o
// perfil/nombre). "" lo quita para que valga el del espacio y deja de forzarlo, porque ya no hay qué forzar.
func (s *Servicio) modeloDePunto(inst *Instancia, c *ConfigPunto, id string) error {
	c.Agente, c.Modelo, c.PerfilModelo = "", "", ""
	agente, modelo, _ := strings.Cut(id, "/")
	switch {
	case id == "":
		c.ForzarModelo = false
	case agente == "perfil":
		if _, existe := inst.Config.Perfiles()[modelo]; !existe {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("perfil de modelo desconocido: %s", modelo))
		}
		c.PerfilModelo = modelo
	case s.opc.Registro[agente] == nil:
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("agente desconocido: %s", agente))
	default:
		c.Agente, c.Modelo = agente, modelo
	}
	return nil
}

// fijarPredeterminado guarda el modelo que se usa cuando la solicitud no indica agente ni modelo (un identificador
// del catálogo: agente, agente/modelo o perfil/nombre), y su esfuerzo si se indica (con id vacío, solo el esfuerzo), y
// lo aplica en marcha. En la
// instancia global va a la configuración global y alcanza también a los proyectos que no tengan el suyo; en un
// proyecto, a su local.yaml.
func (s *Servicio) fijarPredeterminado(inst *Instancia, id string, esfuerzo *string) error {
	valores := map[string]any{"predeterminados.modelo": "", "predeterminados.perfil_modelo": ""}
	if esfuerzo != nil {
		if *esfuerzo != "" && !slices.Contains(nucleo.NivelesEsfuerzo, *esfuerzo) {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("esfuerzo desconocido: %s", *esfuerzo))
		}
		if id == "" { // solo el esfuerzo: el modelo sigue siendo el que haya (en un proyecto, quizá el heredado)
			return s.guardarValores(inst, map[string]any{"predeterminados.esfuerzo": *esfuerzo})
		}
		valores["predeterminados.esfuerzo"] = *esfuerzo
	}
	agente, modelo, _ := strings.Cut(id, "/")
	switch {
	case agente == "perfil":
		if _, existe := inst.Config.Perfiles()[modelo]; !existe {
			return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("perfil de modelo desconocido: %s", modelo))
		}
		valores["predeterminados.perfil_modelo"] = modelo
	case s.opc.Registro[agente] == nil:
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("agente desconocido: %s", agente))
	default:
		valores["predeterminados.agente"], valores["predeterminados.modelo"] = agente, modelo
	}
	return s.guardarValores(inst, valores)
}

// fijarRetencion guarda los plazos y el tope del historial como fijarPredeterminado y purga al momento.
func (s *Servicio) fijarRetencion(inst *Instancia, resultado, metadatos, maximo int) error {
	if err := nucleo.ValidarRetencion(resultado, metadatos, maximo); err != nil {
		return err
	}
	err := s.guardarValores(inst, map[string]any{"retencion.horas_resultado": resultado, "retencion.horas_metadatos": metadatos,
		"retencion.max_trabajos": maximo})
	if err == nil {
		inst.purgarPronto()
	}
	return err
}

// editarPunto aplica f al archivo del punto y recarga; si falla, responde con el error y devuelve false.
func (s *Servicio) editarPunto(w http.ResponseWriter, r *http.Request, punto *EstadoPunto, f func([]ConfigPunto) ([]ConfigPunto, error)) bool {
	if err := configuracion.EditarPuntos(punto.Origen, origenDeArchivo(punto.Origen), f); err != nil {
		api.EscribirError(w, r, err)
		return false
	}
	s.Recargar()
	_ = s.persistirEstado() // anotado; el punto ya está en su archivo
	return true
}

// cambiarPuntoUI cambia solo lo que viene en el cuerpo (PATCH): lo que edita la Web UI.
func (s *Servicio) cambiarPuntoUI(w http.ResponseWriter, r *http.Request, inst *Instancia, punto *EstadoPunto) {
	nombre := punto.Nombre
	var cambio struct { // solo cambia lo que viene en el cuerpo
		Activo        *bool     `json:"activo"`
		Instrucciones *string   `json:"instrucciones"`
		Esfuerzo      *string   `json:"esfuerzo"` // "" vuelve al del agente
		Modelo        *string   `json:"modelo"`   // identificador del catálogo; "" vuelve al del espacio
		Forzar        *bool     `json:"forzar_modelo"`
		Permitidos    *[]string `json:"modelos_permitidos"`
		// Esquema: ausente no cambia; null o {} lo quita; un objeto lo fija.
		Esquema    json.RawMessage `json:"esquema"`
		Respaldo   *[]string       `json:"respaldo"`
		Reintentos *int            `json:"reintentos_salida"`
		// Presupuesto: ausente no cambia; null lo quita; un objeto lo fija.
		Presupuesto json.RawMessage `json:"presupuesto"`
	}
	if err := api.LeerJSON(w, r, &cambio); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if cambio.Esfuerzo != nil && *cambio.Esfuerzo != "" && !slices.Contains(nucleo.NivelesEsfuerzo, *cambio.Esfuerzo) {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("esfuerzo desconocido: %s", *cambio.Esfuerzo)))
		return
	}
	if s.editarPunto(w, r, punto, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		for i := range lista {
			if lista[i].Nombre != nombre {
				continue
			}
			if cambio.Activo != nil {
				lista[i].Activo = cambio.Activo
			}
			if cambio.Instrucciones != nil {
				lista[i].Instrucciones = strings.TrimSpace(*cambio.Instrucciones)
			}
			if cambio.Esfuerzo != nil {
				lista[i].Esfuerzo = *cambio.Esfuerzo
			}
			if cambio.Modelo != nil {
				if err := s.modeloDePunto(inst, &lista[i], strings.TrimSpace(*cambio.Modelo)); err != nil {
					return nil, err
				}
			}
			if cambio.Forzar != nil {
				lista[i].ForzarModelo = *cambio.Forzar
			}
			if cambio.Permitidos != nil {
				lista[i].ModelosPermitidos = slices.DeleteFunc(*cambio.Permitidos, func(m string) bool { return strings.TrimSpace(m) == "" })
			}
			if len(cambio.Esquema) > 0 {
				lista[i].Esquema = nil // null y {} lo quitan
				if err := json.Unmarshal(cambio.Esquema, &lista[i].Esquema); err != nil {
					return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el esquema debe ser un objeto JSON"))
				}
			}
			if cambio.Respaldo != nil {
				lista[i].Respaldo = *cambio.Respaldo
			}
			if cambio.Reintentos != nil {
				lista[i].ReintentosSalida = *cambio.Reintentos
			}
			if len(cambio.Presupuesto) > 0 {
				lista[i].Presupuesto = nil // null lo quita
				// Sin campos desconocidos: una errata dejaría el presupuesto vacío y lo quitaría sin avisar.
				leer := json.NewDecoder(bytes.NewReader(cambio.Presupuesto))
				leer.DisallowUnknownFields()
				if err := leer.Decode(&lista[i].Presupuesto); err != nil {
					return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("el presupuesto debe ser un objeto JSON"))
				}
			}
			// La receta se valida antes de escribir: un cambio no válido deja la configuración como estaba. Solo si
			// cambia, para no impedir desactivar un punto que ya tenía otro error.
			if len(cambio.Esquema) > 0 || cambio.Respaldo != nil || cambio.Permitidos != nil || len(cambio.Presupuesto) > 0 || cambio.Reintentos != nil {
				normalizado, err := normalizarPunto(lista[i], inst.ID, s.opc.Puerto)
				if err != nil {
					return nil, err
				}
				lista[i].Presupuesto = normalizado.Presupuesto // uno vacío se guarda como ninguno
				if err := validarRespaldo(lista[i], s.existe); err != nil {
					return nil, err
				}
			}
			if cambio.Modelo != nil || cambio.Respaldo != nil {
				if err := validarParaApps(lista[i]); err != nil {
					return nil, err
				}
			}
		}
		return lista, nil
	}) {
		api.EscribirJSON(w, http.StatusOK, map[string]any{"puntos_acceso": s.puntosDe(inst.ID)})
	}
}
