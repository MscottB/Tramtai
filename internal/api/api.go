// Package api expone el servicio por HTTP: la API nativa /api/v1 y las fachadas /openai/v1 y /anthropic/v1
// (secciones 7 y 8). Valida Host, Origin y token antes de atender cualquier ruta (sección 16.1).
package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/documentos"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
	"tramtai/internal/version"
)

const maximoCuerpo = 16 << 20

// Servidor atiende una instancia, o un punto de acceso de una instancia si Punto no es nil.
type Servidor struct {
	Planificador *planificador.Planificador
	Cargas       *documentos.Cargas
	Registro     map[string]adaptadores.Adaptador
	Config       *nucleo.Configuracion
	// Punto es la política del punto de acceso que atiende este servidor (sección 5.4).
	Punto *Punto
	// Token es el token de la instancia para Handler; vacío solo si servicio.requerir_token es false.
	Token string
	// Hosts son los nombres aceptados en la cabecera Host (protección frente a DNS rebinding).
	Hosts []string
	// Sondeo es el intervalo con que se leen los eventos nuevos de un trabajo.
	Sondeo time.Duration
}

// Handler devuelve las rutas con las protecciones aplicadas y el token propio del servidor.
func (s *Servidor) Handler() http.Handler {
	var autorizar func(*http.Request) bool
	if s.Token != "" {
		autorizar = func(r *http.Request) bool { return TokenValido(r, s.Token) }
	}
	return Proteger(s.Hosts, autorizar, s.Rutas())
}

// Rutas devuelve el enrutador sin protecciones, para montarlo detrás de Proteger o del servicio, que elige la
// instancia por token. En un punto de acceso solo se registran sus contratos habilitados y nunca la administración.
func (s *Servidor) Rutas() http.Handler {
	mux := http.NewServeMux()
	contrato := func(nombre string) bool { return s.Punto == nil || s.Punto.Contratos[nombre] }
	mux.HandleFunc("GET /healthz", s.salud)
	if contrato("nativa") {
		mux.HandleFunc("POST /api/v1/trabajos", s.crearTrabajo)
		mux.HandleFunc("GET /api/v1/trabajos", s.listarTrabajos)
		mux.HandleFunc("GET /api/v1/trabajos/{id}", s.obtenerTrabajo)
		mux.HandleFunc("DELETE /api/v1/trabajos/{id}", s.eliminarTrabajo)
		mux.HandleFunc("GET /api/v1/trabajos/{id}/salida", s.salidaTrabajo)
		mux.HandleFunc("GET /api/v1/trabajos/{id}/eventos", s.eventosTrabajo)
		mux.HandleFunc("POST /api/v1/trabajos/{id}/cancelar", s.cancelarTrabajo)
		mux.HandleFunc("POST /api/v1/trabajos/{id}/reintentar", s.reintentarTrabajo)
		mux.HandleFunc("POST /api/v1/ejecutar", s.ejecutarTrabajo)
		mux.HandleFunc("POST /api/v1/cargas", s.crearCarga)
		mux.HandleFunc("GET /api/v1/capacidades", s.capacidades)
		mux.HandleFunc("GET /api/v1/modelos", s.modelosNativos)
	}
	if contrato("openai") {
		mux.HandleFunc("POST /openai/v1/chat/completions", s.chatCompletions)
		mux.HandleFunc("POST /openai/v1/responses", s.respuestas)
		mux.HandleFunc("GET /openai/v1/models", s.modelosOpenAI)
	}
	if contrato("anthropic") {
		mux.HandleFunc("POST /anthropic/v1/messages", s.mensajesAnthropic)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "ruta no encontrada: "+r.Method+" "+r.URL.Path))
	})
	return mux
}

// Proteger valida Host (antes de autenticar), Origin (sin CORS: solo el mismo origen) y, si autorizar no es
// nil, el token. /healthz no exige token.
func Proteger(hosts []string, autorizar func(*http.Request) bool, siguiente http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostPermitido(hosts, r.Host) {
			EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoOrigenNoPermitido, "cabecera Host no permitida"))
			return
		}
		if origen := r.Header.Get("Origin"); origen != "" {
			if u, err := url.Parse(origen); err != nil || u.Host != r.Host {
				EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoOrigenNoPermitido, "origen no permitido"))
				return
			}
		}
		if autorizar != nil && r.URL.Path != "/healthz" && !autorizar(r) {
			NoAutorizado(w, r)
			return
		}
		siguiente.ServeHTTP(w, r)
	})
}

// NoAutorizado responde 401 con el formato del contrato de la ruta.
func NoAutorizado(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoNoAutorizado, "falta el token o no es válido"))
}

func hostPermitido(hosts []string, host string) bool {
	if nombre, _, err := net.SplitHostPort(host); err == nil {
		host = nombre
	}
	return slices.Contains(hosts, strings.ToLower(strings.Trim(host, "[]")))
}

// TokenDe extrae el token de "Authorization: Bearer" (API nativa y SDK de OpenAI) o de "x-api-key" (SDK de Anthropic).
func TokenDe(r *http.Request) string {
	if valor, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return valor
	}
	return r.Header.Get("x-api-key")
}

// TokenValido compara en tiempo constante el token de la petición con el esperado.
func TokenValido(r *http.Request, esperado string) bool {
	token := TokenDe(r)
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(esperado)) == 1
}

func (s *Servidor) salud(w http.ResponseWriter, _ *http.Request) {
	EscribirJSON(w, http.StatusOK, map[string]any{"estado": "ok", "version": version.Version})
}

var estadosHTTP = map[nucleo.CodigoError]int{
	nucleo.CodigoSolicitudInvalida:      http.StatusBadRequest,
	nucleo.CodigoConfiguracionInvalida:  http.StatusBadRequest,
	nucleo.CodigoModeloNoConfigurado:    http.StatusBadRequest,
	nucleo.CodigoModeloAmbiguo:          http.StatusBadRequest,
	nucleo.CodigoModeloNoDisponible:     http.StatusBadRequest,
	nucleo.CodigoModeloNoPermitido:      http.StatusBadRequest,
	nucleo.CodigoParametroNoSoportado:   http.StatusBadRequest,
	nucleo.CodigoCargaInvalida:          http.StatusBadRequest,
	nucleo.CodigoNoAutorizado:           http.StatusUnauthorized,
	nucleo.CodigoOrigenNoPermitido:      http.StatusForbidden,
	nucleo.CodigoNoEncontrado:           http.StatusNotFound,
	nucleo.CodigoConflictoIdempotencia:  http.StatusConflict,
	nucleo.CodigoTrabajoNoTerminado:     http.StatusConflict,
	nucleo.CodigoTrabajoSinSalida:       http.StatusConflict,
	nucleo.CodigoTransicionInvalida:     http.StatusConflict,
	nucleo.CodigoEntradaNoDisponible:    http.StatusGone,
	nucleo.CodigoResultadoExpirado:      http.StatusGone,
	nucleo.CodigoSalidaNoCumpleEsquema:  http.StatusUnprocessableEntity,
	nucleo.CodigoConversionFallida:      http.StatusUnprocessableEntity,
	nucleo.CodigoColaLlena:              http.StatusTooManyRequests,
	nucleo.CodigoLimiteUsoProveedor:     http.StatusTooManyRequests,
	nucleo.CodigoErrorAgente:            http.StatusBadGateway,
	nucleo.CodigoAutenticacionAgente:    http.StatusBadGateway,
	nucleo.CodigoErrorProtocoloAgente:   http.StatusBadGateway,
	nucleo.CodigoAgenteNoDisponible:     http.StatusServiceUnavailable,
	nucleo.CodigoServicioDetenido:       http.StatusServiceUnavailable,
	nucleo.CodigoTiempoColaAgotado:      http.StatusServiceUnavailable,
	nucleo.CodigoTiempoEjecucionAgotado: http.StatusGatewayTimeout,
}

// EscribirError responde con el formato de error del contrato de la ruta: nativo, OpenAI o Anthropic.
func EscribirError(w http.ResponseWriter, r *http.Request, err error) {
	codigo, mensaje := nucleo.CodigoDe(err), nucleo.MensajeDe(err)
	if codigo == "" {
		log.Printf("error interno en %s %s: %v", r.Method, r.URL.Path, err)
		codigo, mensaje = nucleo.CodigoErrorInterno, "error interno del servicio"
	}
	estado, conocido := estadosHTTP[codigo]
	if !conocido {
		estado = http.StatusInternalServerError
	}
	if codigo == nucleo.CodigoColaLlena {
		w.Header().Set("Retry-After", "5")
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/openai/"):
		EscribirJSON(w, estado, map[string]any{"error": errorOpenAI(estado, codigo, mensaje)})
	case strings.HasPrefix(r.URL.Path, "/anthropic/"):
		EscribirJSON(w, estado, map[string]any{"type": "error", "error": map[string]any{"type": tipoAnthropic(estado), "message": mensaje}})
	default:
		EscribirJSON(w, estado, map[string]any{"error": map[string]any{"codigo": codigo, "mensaje": mensaje}})
	}
}

func errorOpenAI(estado int, codigo nucleo.CodigoError, mensaje string) map[string]any {
	tipos := map[int]string{400: "invalid_request_error", 401: "authentication_error", 403: "permission_error", 404: "not_found_error", 429: "rate_limit_error"}
	tipo, ok := tipos[estado]
	if !ok {
		tipo = "api_error"
	}
	return map[string]any{"message": mensaje, "type": tipo, "param": nil, "code": codigo}
}

func tipoAnthropic(estado int) string {
	tipos := map[int]string{400: "invalid_request_error", 401: "authentication_error", 403: "permission_error", 404: "not_found_error", 429: "rate_limit_error", 503: "overloaded_error"}
	if tipo, ok := tipos[estado]; ok {
		return tipo
	}
	return "api_error"
}

// errorDeTrabajo convierte un trabajo terminado sin éxito en el error que se devuelve al cliente.
func errorDeTrabajo(t nucleo.Trabajo) error {
	if t.CodigoError != "" {
		return nucleo.NuevoError(t.CodigoError, t.MensajeError)
	}
	return nucleo.NuevoError(nucleo.CodigoTrabajoSinSalida, "el trabajo terminó como "+string(t.Estado))
}

// EscribirJSON responde con un cuerpo JSON.
func EscribirJSON(w http.ResponseWriter, estado int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(estado)
	_ = json.NewEncoder(w).Encode(v)
}

// LeerJSON decodifica el cuerpo con límite de tamaño; los campos desconocidos son un error.
func LeerJSON(w http.ResponseWriter, r *http.Request, destino any) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximoCuerpo))
	d.DisallowUnknownFields()
	if err := d.Decode(destino); err != nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "cuerpo JSON no válido: "+err.Error())
	}
	return nil
}

func leerJSON(w http.ResponseWriter, r *http.Request, destino any) error {
	return LeerJSON(w, r, destino)
}

// seguir entrega los eventos del trabajo posteriores a "desde" hasta que termina y devuelve el trabajo final.
// shortcut: sondea SQLite cada Sondeo; una notificación en memoria lo sustituirá si el sondeo pesa.
func (s *Servidor) seguir(ctx context.Context, id string, desde int, alEvento func(persistencia.Evento) error, latido func() error) (nucleo.Trabajo, error) {
	ultimoLatido := time.Now()
	entregar := func() (bool, error) {
		eventos, err := s.Planificador.Eventos(ctx, id, desde)
		if err != nil {
			return false, err
		}
		terminado := false
		for _, e := range eventos {
			desde = e.Numero
			if alEvento != nil {
				if err := alEvento(e); err != nil {
					return false, err
				}
			}
			terminado = terminado || (e.Tipo == "estado" && nucleo.Estado(e.Datos).Terminal())
		}
		return terminado, nil
	}
	for {
		terminado, err := entregar()
		if err != nil {
			return nucleo.Trabajo{}, err
		}
		t, err := s.Planificador.Obtener(ctx, id)
		if err != nil {
			return t, err
		}
		if t.Estado.Terminal() {
			if !terminado { // el último evento pudo escribirse entre las dos lecturas
				if _, err := entregar(); err != nil {
					return t, err
				}
			}
			return t, nil
		}
		if latido != nil && time.Since(ultimoLatido) > 15*time.Second {
			if err := latido(); err != nil {
				return t, err
			}
			ultimoLatido = time.Now()
		}
		select {
		case <-ctx.Done():
			return t, ctx.Err()
		case <-time.After(s.sondeo()):
		}
	}
}

func (s *Servidor) sondeo() time.Duration {
	if s.Sondeo > 0 {
		return s.Sondeo
	}
	return 100 * time.Millisecond
}

// flujo escribe eventos SSE.
type flujo struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func abrirFlujo(w http.ResponseWriter) *flujo {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	f := &flujo{w: w, rc: http.NewResponseController(w)}
	_ = f.rc.Flush()
	return f
}

// escribir envía con plazo: un cliente lento se desconecta en 30 s sin bloquear al agente (sección 7.3).
func (f *flujo) escribir(texto string) error {
	_ = f.rc.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := io.WriteString(f.w, texto); err != nil {
		return err
	}
	return f.rc.Flush()
}

// enviar escribe un evento; los saltos de línea del contenido van en varias líneas data (CR se normaliza a LF).
func (f *flujo) enviar(id, evento, datos string) error {
	var b strings.Builder
	if id != "" {
		b.WriteString("id: " + id + "\n")
	}
	if evento != "" {
		b.WriteString("event: " + evento + "\n")
	}
	datos = strings.ReplaceAll(strings.ReplaceAll(datos, "\r\n", "\n"), "\r", "\n")
	for _, linea := range strings.Split(datos, "\n") {
		b.WriteString("data: " + linea + "\n")
	}
	return f.escribir(b.String() + "\n")
}

func (f *flujo) json(evento string, v any) error {
	datos, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return f.enviar("", evento, string(datos))
}

func (f *flujo) latido() error { return f.escribir(": latido\n\n") }
