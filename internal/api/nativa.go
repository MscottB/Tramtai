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
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/version"
)

// enviar completa la solicitud con los predeterminados y la acepta con la Idempotency-Key del cliente (sección 7.2).
func (s *Servidor) enviar(r *http.Request, sol nucleo.SolicitudEjecucion) (nucleo.Trabajo, bool, error) {
	clave := r.Header.Get("Idempotency-Key")
	if len(clave) > 255 {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "Idempotency-Key supera 255 caracteres")
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

// salidaTrabajo entrega la salida con el tipo de su formato; 410 si la retención ya la borró (sección 16.2).
func (s *Servidor) salidaTrabajo(w http.ResponseWriter, r *http.Request) {
	t, err := s.trabajo(r, r.PathValue("id"))
	switch {
	case err != nil:
		EscribirError(w, r, err)
	case !t.Estado.Terminal():
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoTrabajoNoTerminado, "el trabajo está en "+string(t.Estado)))
	case t.Disponibilidad == nucleo.SalidaExpirada:
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoResultadoExpirado, "el resultado expiró por la retención (retencion.horas_resultado); vuelve a ejecutar la solicitud"))
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
			EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "Last-Event-ID no válido"))
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
		_ = f.json("aviso", map[string]any{"codigo": nucleo.CodigoResultadoExpirado, "mensaje": "los eventos con contenido expiraron con el resultado"})
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
	rc := http.NewResponseController(w)
	_ = rc.Flush()
	final, err := s.seguir(r.Context(), t.ID, 0, nil, func() error {
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
		return rc.Flush()
	})
	if err != nil {
		return
	}
	_ = json.NewEncoder(w).Encode(final)
}

// crearCarga acepta multipart (campo "archivo") o el binario en el cuerpo con ?nombre= (sección 7.1).
func (s *Servidor) crearCarga(w http.ResponseWriter, r *http.Request) {
	if s.Cargas == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, "esta instancia no admite cargas"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.Config.Entero("conversion.maximo_carga_mb"))<<20+1<<20)
	var carga documentos.Carga
	var err error
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		carga, err = s.cargaMultipart(r)
	} else if nombre := r.URL.Query().Get("nombre"); nombre == "" {
		err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "envía multipart con el campo archivo, o el binario con ?nombre=archivo.ext")
	} else {
		carga, err = s.Cargas.Guardar(nombre, r.Body)
	}
	var demasiado *http.MaxBytesError
	if errors.As(err, &demasiado) {
		err = nucleo.NuevoError(nucleo.CodigoCargaInvalida, "el archivo supera conversion.maximo_carga_mb")
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
		return documentos.Carga{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "multipart no válido")
	}
	for {
		parte, err := lector.NextPart()
		if err == io.EOF {
			return documentos.Carga{}, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "falta el campo archivo")
		}
		if err != nil {
			return documentos.Carga{}, err
		}
		if parte.FormName() == "archivo" {
			return s.Cargas.Guardar(parte.FileName(), parte)
		}
	}
}

// capacidad es lo que cada adaptador garantiza (sección 8.1); lo no garantizado se declara false.
type capacidad struct {
	Streaming               string
	ModeloEfectivoInformado bool
}

var capacidadesConocidas = map[string]capacidad{
	"claude": {Streaming: "tokens", ModeloEfectivoInformado: true},
	"codex":  {Streaming: "final"},
	"gemini": {Streaming: "tokens", ModeloEfectivoInformado: true},
	// antigravity: final y sin modelo efectivo (M0); experimental.
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
				entrada["problema"] = e.Problema
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

// modelo es una entrada del catálogo informativo (sección 9.5).
type modelo struct {
	ID     string `json:"id"`
	Agente string `json:"agente,omitempty"`
	Modelo string `json:"modelo,omitempty"`
	Perfil string `json:"perfil,omitempty"`
	Origen string `json:"origen"`
}

// catalogo lista cada agente (con el modelo predeterminado de su CLI), los modelos configurados en los perfiles
// como agente/modelo y los perfiles como perfil/nombre.
func (s *Servidor) catalogo() []modelo {
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
	sort.Slice(lista, func(i, j int) bool { return lista[i].ID < lista[j].ID })
	return s.filtrarCatalogo(lista)
}

func (s *Servidor) modelosNativos(w http.ResponseWriter, _ *http.Request) {
	EscribirJSON(w, http.StatusOK, map[string]any{"modelos": s.catalogo()})
}
