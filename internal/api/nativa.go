package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/version"
)

// enviar completa la solicitud con los predeterminados y la acepta con la Idempotency-Key del cliente.
func (s *Servidor) enviar(r *http.Request, sol nucleo.SolicitudEjecucion) (nucleo.Trabajo, bool, error) {
	clave := r.Header.Get("Idempotency-Key")
	if len(clave) > 255 {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("Idempotency-Key supera 255 caracteres"))
	}
	sol, err := s.ajustar(sol)
	if err != nil {
		return nucleo.Trabajo{}, false, err
	}
	return s.aceptar(r.Context(), sol, clave)
}

func ubicacion(id string) string { return "/api/v1/trabajos/" + id }

// crearTrabajo responde 202 tras registrar el trabajo, o 200 si la clave de idempotencia ya apuntaba a él.
func (s *Servidor) crearTrabajo(w http.ResponseWriter, r *http.Request) {
	var sol nucleo.SolicitudEjecucion
	if err := leerJSON(w, r, &sol); err != nil {
		EscribirError(w, r, err)
		return
	}
	t, reutilizado, err := s.enviar(r, sol)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	w.Header().Set("Location", ubicacion(t.ID))
	estado := http.StatusAccepted
	if reutilizado {
		estado = http.StatusOK
	}
	EscribirJSON(w, estado, t)
}

func (s *Servidor) obtenerTrabajo(w http.ResponseWriter, r *http.Request) {
	t, err := s.trabajo(r, r.PathValue("id"))
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	EscribirJSON(w, http.StatusOK, t)
}

func (s *Servidor) eliminarTrabajo(w http.ResponseWriter, r *http.Request) {
	if _, err := s.trabajo(r, r.PathValue("id")); err != nil {
		EscribirError(w, r, err)
		return
	}
	if err := s.Planificador.Eliminar(r.Context(), r.PathValue("id")); err != nil {
		EscribirError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// eliminarTrabajos vacía el historial: borra los trabajos terminados y las cargas que ya no necesita ninguno.
func (s *Servidor) eliminarTrabajos(w http.ResponseWriter, r *http.Request) {
	n, err := s.Planificador.EliminarTerminados(r.Context())
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	if s.Cargas != nil {
		enUso, err := s.Planificador.CargasEnUso(r.Context())
		if err != nil {
			EscribirError(w, r, err)
			return
		}
		// Con un "ahora" lejano toda carga cuenta como vencida: se borran todas las que no están en uso.
		if _, err := s.Cargas.Purgar(enUso, time.Now().AddDate(100, 0, 0)); err != nil {
			EscribirError(w, r, err)
			return
		}
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"eliminados": n})
}

// salidaTrabajo entrega la salida con el tipo de su formato; 410 si la retención ya la borró.
func (s *Servidor) salidaTrabajo(w http.ResponseWriter, r *http.Request) {
	t, err := s.trabajo(r, r.PathValue("id"))
	switch {
	case err != nil:
		EscribirError(w, r, err)
	case !t.Estado.Terminal():
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoTrabajoNoTerminado, idioma.Tf("el trabajo está en %s", t.Estado)))
	case t.Disponibilidad == nucleo.SalidaExpirada:
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoResultadoExpirado, idioma.T("el resultado expiró por la retención (retencion.horas_resultado); vuelve a ejecutar la solicitud")))
	case t.Estado != nucleo.Completado:
		EscribirError(w, r, errorDeTrabajo(t))
	default:
		tipo := map[string]string{"json": "application/json", "texto": "text/plain; charset=utf-8"}[t.Solicitud.Salida.Formato]
		if tipo == "" {
			tipo = "text/markdown; charset=utf-8"
		}
		w.Header().Set("Content-Type", tipo)
		_, _ = io.WriteString(w, t.Salida)
	}
}

// eventosTrabajo emite los eventos por SSE desde Last-Event-ID (o ?desde=). Reconectar no inicia ni cancela
// el trabajo; al terminar envía un evento "fin" sin identificador.
func (s *Servidor) eventosTrabajo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cursor := r.Header.Get("Last-Event-ID")
	if cursor == "" {
		cursor = r.URL.Query().Get("desde")
	}
	desde := 0
	if cursor != "" {
		n, err := strconv.Atoi(cursor)
		if err != nil || n < 0 {
			EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("Last-Event-ID no válido")))
			return
		}
		desde = n
	}
	t, err := s.trabajo(r, id)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	f := abrirFlujo(w)
	if t.Disponibilidad == nucleo.SalidaExpirada {
		// Solo quedan los eventos de estado: se avisa en lugar de dar a entender que no hubo contenido.
		_ = f.json("aviso", map[string]any{"codigo": nucleo.CodigoResultadoExpirado, "mensaje": idioma.T("los eventos con contenido expiraron con el resultado")})
	}
	final, err := s.seguir(r.Context(), id, desde, func(e persistencia.Evento) error {
		return f.enviar(strconv.Itoa(e.Numero), e.Tipo, e.Datos)
	}, f.latido)
	if err != nil {
		return // cliente desconectado: el trabajo sigue
	}
	_ = f.json("fin", map[string]any{"estado": final.Estado, "codigo_error": final.CodigoError})
}

func (s *Servidor) cancelarTrabajo(w http.ResponseWriter, r *http.Request) {
	if _, err := s.trabajo(r, r.PathValue("id")); err != nil {
		EscribirError(w, r, err)
		return
	}
	t, err := s.Planificador.Cancelar(r.Context(), r.PathValue("id"))
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	EscribirJSON(w, http.StatusAccepted, t)
}

func (s *Servidor) reintentarTrabajo(w http.ResponseWriter, r *http.Request) {
	if _, err := s.trabajo(r, r.PathValue("id")); err != nil {
		EscribirError(w, r, err)
		return
	}
	t, err := s.Planificador.Reintentar(r.Context(), r.PathValue("id"))
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	w.Header().Set("Location", ubicacion(t.ID))
	EscribirJSON(w, http.StatusAccepted, t)
}

// ejecutarTrabajo crea el trabajo, envía Location de inmediato y espera el resultado. Un corte HTTP no lo
// cancela: el cliente lo recupera por Location. Mientras espera envía espacios (JSON válido) como latido.
func (s *Servidor) ejecutarTrabajo(w http.ResponseWriter, r *http.Request) {
	var sol nucleo.SolicitudEjecucion
	if err := leerJSON(w, r, &sol); err != nil {
		EscribirError(w, r, err)
		return
	}
	t, _, err := s.enviar(r, sol)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	w.Header().Set("Location", ubicacion(t.ID))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	f := &flujo{w: w, rc: http.NewResponseController(w)} // mismo plazo por escritura que el SSE
	_ = f.rc.Flush()
	final, err := s.seguir(r.Context(), t.ID, 0, nil, func() error { return f.escribir(" ") })
	if err != nil {
		return
	}
	datos, _ := json.Marshal(final)
	_ = f.escribir(string(datos) + "\n")
}

// crearCarga acepta multipart (campo "archivo") o el binario en el cuerpo con ?nombre=.
func (s *Servidor) crearCarga(w http.ResponseWriter, r *http.Request) {
	if s.Cargas == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("esta instancia no admite cargas")))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.Config.Entero("conversion.maximo_carga_mb"))<<20+1<<20)
	var carga documentos.Carga
	var err error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		carga, err = s.cargaMultipart(r)
	} else if nombre := r.URL.Query().Get("nombre"); nombre == "" {
		err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("envía multipart con el campo archivo, o el binario con ?nombre=archivo.ext"))
	} else {
		carga, err = s.Cargas.Guardar(nombre, r.Body)
	}
	var demasiado *http.MaxBytesError
	if errors.As(err, &demasiado) {
		err = nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.T("el archivo supera conversion.maximo_carga_mb"))
	}
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	EscribirJSON(w, http.StatusCreated, carga)
}

func (s *Servidor) cargaMultipart(r *http.Request) (documentos.Carga, error) {
	lector, err := r.MultipartReader()
	if err != nil {
		return documentos.Carga{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("multipart no válido"))
	}
	for {
		parte, err := lector.NextPart()
		if err == io.EOF {
			return documentos.Carga{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("falta el campo archivo"))
		}
		if err != nil {
			return documentos.Carga{}, err
		}
		if parte.FormName() == "archivo" {
			return s.Cargas.Guardar(parte.FileName(), parte)
		}
	}
}

// capacidad es lo que cada adaptador garantiza; lo no garantizado se declara false.
type capacidad struct {
	Streaming               string
	ModeloEfectivoInformado bool
}

var capacidadesConocidas = map[string]capacidad{
	"claude": {Streaming: "tokens", ModeloEfectivoInformado: true},
	"codex":  {Streaming: "final"},
	"gemini": {Streaming: "tokens", ModeloEfectivoInformado: true},
	// antigravity: final y sin modelo efectivo; experimental.
	"antigravity": {Streaming: "final"},
}

func (s *Servidor) capacidades(w http.ResponseWriter, r *http.Request) {
	nombres := make([]string, 0, len(s.Registro))
	for nombre := range s.Registro {
		nombres = append(nombres, nombre)
	}
	sort.Strings(nombres)
	lista := []map[string]any{}
	for _, nombre := range nombres {
		c, conocida := capacidadesConocidas[nombre]
		if !conocida {
			c = capacidad{Streaming: "final"}
		}
		entrada := map[string]any{"agente": nombre, "capacidades": map[string]any{
			"texto": true, "streaming": c.Streaming, "salida_estructurada": conocida, "uso_tokens": "informado_por_cli",
			"modelo_efectivo_informado": c.ModeloEfectivoInformado, "tool_calling_compatible": false, "imagenes": false, "sesiones_persistentes": false,
		}}
		if d, ok := s.Registro[nombre].(adaptadores.Detectable); ok {
			ctx, cancelar := context.WithTimeout(r.Context(), 10*time.Second)
			e := d.Detectar(ctx)
			cancelar()
			entrada["version_detectada"], entrada["autenticacion"], entrada["terminos"] = e.Version, e.Autenticacion, e.Terminos
			if e.Problema != "" {
				entrada["problema"], entrada["problema_codigo"] = e.Problema, e.ProblemaCodigo
			}
		}
		lista = append(lista, entrada)
	}
	EscribirJSON(w, http.StatusOK, map[string]any{
		"version":               version.Version,
		"formato_serializacion": adaptadores.FormatoSerializacion,
		"entradas":              []string{"texto", "markdown", "json", "xml", "csv", "html", "pdf", "docx", "pptx", "xlsx", "zip"},
		"adaptadores":           lista,
	})
}

// modelo es una entrada del catálogo informativo.
type modelo struct {
	ID     string `json:"id"`
	Agente string `json:"agente,omitempty"`
	Modelo string `json:"modelo,omitempty"`
	Perfil string `json:"perfil,omitempty"`
	Origen string `json:"origen"`
	// Solo en /api/v1/modelos: el agente/modelo al que lleva un perfil, la versión real que informó el agente la
	// última vez (por ejemplo claude-sonnet-5-5 para "sonnet") y si es el modelo predeterminado de la instancia.
	Resuelto       string `json:"resuelto,omitempty"`
	Version        string `json:"version,omitempty"`
	Predeterminado bool   `json:"predeterminado,omitempty"`
	// Esfuerzos son los niveles de esfuerzo que admite su agente; vacío si no se puede elegir.
	Esfuerzos []string `json:"esfuerzos,omitempty"`
}

// predeterminado es el identificador del catálogo que se usa cuando la solicitud no indica agente ni modelo.
func (s *Servidor) predeterminado() string {
	agente, modelo, perfil := s.Config.Texto("predeterminados.agente"), s.Config.Texto("predeterminados.modelo"), s.Config.Texto("predeterminados.perfil_modelo")
	switch {
	case modelo != "":
		return agente + "/" + modelo
	case perfil != "":
		return "perfil/" + perfil
	}
	return agente
}

// catalogo lista cada agente (con el modelo predeterminado de su CLI), los modelos configurados en los perfiles
// como agente/modelo, los perfiles como perfil/nombre y las versiones concretas que informaron los agentes
// (efectivos, de ModelosEfectivos) como agente/version, para poder pedir una en concreto.
func (s *Servidor) catalogo(efectivos map[string]string) []modelo {
	var lista []modelo
	for agente := range s.Registro {
		lista = append(lista, modelo{ID: agente, Agente: agente, Origen: "predeterminado_del_cli"})
	}
	vistos := map[string]bool{}
	for perfil, porAgente := range s.Config.Perfiles() {
		lista = append(lista, modelo{ID: "perfil/" + perfil, Perfil: perfil, Origen: "configurado"})
		for agente, m := range porAgente {
			if _, existe := s.Registro[agente]; existe && !vistos[agente+"/"+m] {
				vistos[agente+"/"+m] = true
				lista = append(lista, modelo{ID: agente + "/" + m, Agente: agente, Modelo: m, Origen: "configurado"})
			}
		}
	}
	if agente, m, _ := strings.Cut(s.predeterminado(), "/"); m != "" && agente != "perfil" && !vistos[agente+"/"+m] {
		vistos[agente+"/"+m] = true
		lista = append(lista, modelo{ID: agente + "/" + m, Agente: agente, Modelo: m, Origen: "configurado"})
	}
	for clave, version := range efectivos {
		agente, _, _ := strings.Cut(clave, "/")
		if _, existe := s.Registro[agente]; existe && !vistos[agente+"/"+version] {
			vistos[agente+"/"+version] = true
			lista = append(lista, modelo{ID: agente + "/" + version, Agente: agente, Modelo: version, Origen: "visto"})
		}
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].ID < lista[j].ID })
	return s.filtrarCatalogo(lista)
}

func (s *Servidor) modelosNativos(w http.ResponseWriter, r *http.Request) {
	efectivos, _ := s.Planificador.ModelosEfectivos(r.Context()) // sin historial, el catálogo va sin versiones
	lista := s.catalogo(efectivos)
	pred, agentePred, perfiles := s.predeterminado(), s.Config.Texto("predeterminados.agente"), s.Config.Perfiles()
	for i := range lista {
		m := &lista[i]
		m.Predeterminado = m.ID == pred
		agente, modelo := m.Agente, m.Modelo
		if m.Perfil != "" {
			if agente, modelo = agentePred, perfiles[m.Perfil][agentePred]; modelo != "" {
				m.Resuelto = agente + "/" + modelo
			}
		}
		if agente != "" {
			m.Version = efectivos[agente+"/"+modelo]
			m.Esfuerzos = adaptadores.NivelesEsfuerzo(agente)
		}
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"modelos": lista})
}
