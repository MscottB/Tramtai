package api

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/documentos"
	"tramtai/internal/herramientas"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/salidas"
	"tramtai/internal/version"
)

// enviar completa la solicitud con los predeterminados y la acepta con la Idempotency-Key del cliente.
func (s *Servidor) enviar(r *http.Request, sol nucleo.SolicitudEjecucion) (nucleo.Trabajo, bool, error) {
	sol, err := s.ajustar(sol)
	if err != nil {
		return nucleo.Trabajo{}, false, err
	}
	return s.aceptar(r.Context(), sol, r.Header.Get("Idempotency-Key"))
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
	EscribirJSON(w, http.StatusOK, s.conErrores(r.Context(), t))
}

// vista es un trabajo como lo ve la API propia: con herramientas, la salida es el texto para el usuario y llamadas las
// que pidió el agente; si falló por el esquema, errores_salida. Un trabajo sin herramientas ni errores se ve igual.
type vista struct {
	nucleo.Trabajo
	Llamadas      []nucleo.Llamada   `json:"llamadas,omitempty"`
	ErroresSalida []salidas.Problema `json:"errores_salida,omitempty"`
	conSalida     bool               // con herramientas: salida va aunque esté vacía (el agente solo pidió llamadas)
}

// MarshalJSON escribe el trabajo tal cual y le suma llamadas, errores_salida y, con herramientas, salida aunque esté
// vacía. Un trabajo sin nada de eso sale exactamente como antes.
func (v vista) MarshalJSON() ([]byte, error) {
	datos, err := json.Marshal(v.Trabajo)
	if err != nil || (len(v.Llamadas) == 0 && len(v.ErroresSalida) == 0 && !v.conSalida) {
		return datos, err
	}
	var campos map[string]json.RawMessage
	if err := json.Unmarshal(datos, &campos); err != nil {
		return nil, err
	}
	extra := map[string]any{"salida": v.Salida}
	if !v.conSalida && v.Salida == "" {
		delete(extra, "salida")
	}
	if len(v.Llamadas) > 0 {
		extra["llamadas"] = v.Llamadas
	}
	if len(v.ErroresSalida) > 0 {
		extra["errores_salida"] = v.ErroresSalida
	}
	for clave, valor := range extra {
		if campos[clave], err = json.Marshal(valor); err != nil {
			return nil, err
		}
	}
	return json.Marshal(campos)
}

// vistaTrabajo interpreta la salida de un trabajo con herramientas que terminó bien (ya validada al terminar).
func vistaTrabajo(t nucleo.Trabajo) vista {
	v := vista{Trabajo: t}
	if t.Estado == nucleo.Completado && t.Disponibilidad != nucleo.SalidaExpirada && herramientas.Activas(t.Solicitud) {
		if texto, llamadas, err := herramientas.Interpretar(t.Solicitud, t.Salida, "llamada_"+t.ID); err == nil {
			v.Salida, v.Llamadas, v.conSalida = texto, llamadas, true
		}
	}
	return v
}

// conErrores es la vista del trabajo con errores_salida, los problemas de la última validación, si falló por el esquema.
func (s *Servidor) conErrores(ctx context.Context, t nucleo.Trabajo) vista {
	v := vistaTrabajo(t)
	if t.CodigoError == nucleo.CodigoSalidaNoCumpleEsquema {
		if problemas, err := s.Planificador.ProblemasSalida(ctx, t.ID); err == nil {
			v.ErroresSalida = problemas
		}
	}
	return v
}

// validarEsquema compila un esquema sin ejecutar nada y explica por qué no vale, con la línea y la columna si no es
// JSON. El esquema llega como objeto o como el texto que escribe el usuario (así un JSON roto también se valida).
func (s *Servidor) validarEsquema(w http.ResponseWriter, r *http.Request) {
	var cuerpo struct {
		Esquema json.RawMessage `json:"esquema"`
	}
	if err := leerJSON(w, r, &cuerpo); err != nil {
		EscribirError(w, r, err)
		return
	}
	esquema := []byte(cuerpo.Esquema)
	var texto string
	if json.Unmarshal(cuerpo.Esquema, &texto) == nil {
		esquema = []byte(texto)
	}
	if len(strings.TrimSpace(string(esquema))) == 0 {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("falta el esquema")))
		return
	}
	if len(esquema) > nucleo.MaximoEsquemaPeticion {
		EscribirJSON(w, http.StatusOK, map[string]any{"valido": false, "error": idioma.T("el esquema supera 256 KB")})
		return
	}
	linea, columna, err := salidas.CompilarDetallado(esquema)
	if err != nil {
		respuesta := map[string]any{"valido": false, "error": nucleo.MensajeDe(err)}
		if linea > 0 {
			respuesta["linea"], respuesta["columna"] = linea, columna
		}
		EscribirJSON(w, http.StatusOK, respuesta)
		return
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"valido": true})
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
		// Las que no están en uso, salvo las recién subidas que otro cliente puede estar a punto de citar.
		if _, err := s.Cargas.PurgarSinUso(enUso); err != nil {
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
		_, _ = io.WriteString(w, vistaTrabajo(t).Salida)
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

// registroTrabajo devuelve los eventos sin texto de la respuesta (estados, intentos y avisos) con su hora, para ver
// cuánto duró cada etapa y con qué agente se intentó.
func (s *Servidor) registroTrabajo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.trabajo(r, id); err != nil {
		EscribirError(w, r, err)
		return
	}
	eventos, err := s.Planificador.Registro(r.Context(), id)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	type evento struct {
		Numero   int       `json:"numero"`
		Tipo     string    `json:"tipo"`
		Datos    string    `json:"datos"`
		CreadoEn time.Time `json:"creado_en"`
	}
	lista := []evento{}
	for _, e := range eventos {
		lista = append(lista, evento{e.Numero, e.Tipo, e.Datos, e.CreadoEn.UTC()})
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"eventos": lista})
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
	// El cuerpo es opcional: {"corregir": true} pide al agente que corrija la respuesta que no cumplía el esquema.
	var cuerpo struct {
		Corregir bool `json:"corregir"`
	}
	leer := json.NewDecoder(http.MaxBytesReader(w, r.Body, maximoCuerpo))
	leer.DisallowUnknownFields()
	if err := leer.Decode(&cuerpo); err != nil && !errors.Is(err, io.EOF) {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("cuerpo JSON no válido: %s", err.Error())))
		return
	}
	reintentar := s.Planificador.Reintentar
	if cuerpo.Corregir {
		reintentar = s.Planificador.Corregir
	}
	t, err := reintentar(r.Context(), r.PathValue("id"))
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
	datos, _ := json.Marshal(s.conErrores(context.Background(), final))
	_ = f.escribir(string(datos) + "\n")
}

// crearCarga acepta multipart (campo "archivo") o el binario en el cuerpo con ?nombre=.
func (s *Servidor) crearCarga(w http.ResponseWriter, r *http.Request) {
	if s.Cargas == nil {
		EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.T("este espacio no admite cargas")))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.Config.Limite("conversion.maximo_carga_mb"))<<20+1<<20)
	var carga documentos.Carga
	err := conPlazoDeLectura(w, plazoLecturaCarga, func() error {
		var err error
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			carga, err = s.cargaMultipart(r)
		} else if nombre := r.URL.Query().Get("nombre"); nombre == "" {
			err = nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("envía multipart con el campo archivo, o el binario con ?nombre=archivo.ext"))
		} else {
			carga, err = s.Cargas.Guardar(nombre, s.nombrePunto(), r.Body)
		}
		return err
	})
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

// nombrePunto es la API de la petición; vacío para la instancia.
func (s *Servidor) nombrePunto() string {
	if s.Punto == nil {
		return ""
	}
	return s.Punto.Nombre
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
			return s.Cargas.Guardar(parte.FileName(), s.nombrePunto(), parte)
		}
	}
}

// fichasAgentes devuelve la ficha de cada agente del registro, en su orden y sin comprobar los CLI: la Web UI saca de
// aquí sus nombres, colores y lo que admite cada uno. Un agente sin ficha (los simulados) va al final con su id.
func (s *Servidor) fichasAgentes(w http.ResponseWriter, _ *http.Request) {
	lista := []map[string]any{}
	vistos := map[string]bool{}
	agregar := func(f adaptadores.Ficha) {
		vistos[f.Nombre] = true
		lista = append(lista, map[string]any{
			"agente": f.Nombre, "nombre": cmp.Or(f.NombreVisible, f.Nombre), "programa": cmp.Or(f.Programa, f.Nombre),
			"codigo": f.Codigo, "color": map[string]string{"claro": f.ColorClaro, "oscuro": f.ColorOscuro}, "orden": f.Orden,
			"streaming": cmp.Or(f.Streaming, "final"), "salida_estructurada": f.SalidaEstructurada,
			"modelo_efectivo_informado": f.ModeloEfectivoInformado, "esfuerzos": noNulo(agentes.Niveles(f.Nombre)),
			"autenticaciones": noNulo(f.Autenticaciones), "variables_clave": noNulo(f.VariablesClave),
			"permitido_apps": agentes.PermitidoEnApps(s.Config, f.Nombre), "url_instalacion": f.URLInstalacion,
			"desconectado": s.Conexiones.Desconectado(f.Nombre),
		})
	}
	for _, f := range agentes.Todos() {
		if s.Registro[f.Nombre] != nil {
			agregar(f)
		}
	}
	otros := make([]string, 0, len(s.Registro))
	for nombre := range s.Registro {
		if !vistos[nombre] {
			otros = append(otros, nombre)
		}
	}
	sort.Strings(otros)
	for _, nombre := range otros {
		agregar(adaptadores.Ficha{Nombre: nombre})
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"agentes": lista})
}

// noNulo devuelve una lista vacía en lugar de nil, para que el JSON lleve [] y no null.
func noNulo(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func (s *Servidor) capacidades(w http.ResponseWriter, r *http.Request) {
	nombres := make([]string, 0, len(s.Registro))
	for nombre := range s.Registro {
		nombres = append(nombres, nombre)
	}
	sort.Strings(nombres)
	// Cada agente se detecta a la vez que los demás (cada Detectar ejecuta su CLI y puede tardar); cada uno escribe en
	// su posición, así que la lista sale en orden alfabético.
	lista := make([]map[string]any, len(nombres))
	var esperar sync.WaitGroup
	for i, nombre := range nombres {
		// Lo que garantiza cada agente sale de su ficha; uno sin ficha (simulado) no garantiza nada.
		f, conocida := agentes.Buscar(nombre)
		if !conocida {
			f.Streaming = "final"
		}
		entrada := map[string]any{"agente": nombre, "desconectado": s.Conexiones.Desconectado(nombre), "capacidades": map[string]any{
			"texto": true, "streaming": f.Streaming, "salida_estructurada": f.SalidaEstructurada, "uso_tokens": "informado_por_cli",
			"modelo_efectivo_informado": f.ModeloEfectivoInformado, "tool_calling_compatible": false, "imagenes": false, "sesiones_persistentes": false,
		}}
		lista[i] = entrada
		if d, ok := s.Registro[nombre].(adaptadores.Detectable); ok {
			esperar.Go(func() {
				// Fuera de la goroutine de la petición, net/http no recupera un pánico: tiraría el servicio entero.
				defer func() {
					if v := recover(); v != nil {
						entrada["problema"], entrada["problema_codigo"] = fmt.Sprint(v), adaptadores.ProblemaAviso
					}
				}()
				ctx, cancelar := context.WithTimeout(r.Context(), 10*time.Second)
				e := d.Detectar(ctx)
				cancelar()
				entrada["version_detectada"], entrada["autenticacion"], entrada["terminos"] = e.Version, e.Autenticacion, e.Terminos
				if e.Cuenta != "" {
					entrada["cuenta"] = e.Cuenta
				}
				if e.Problema != "" {
					entrada["problema"], entrada["problema_codigo"] = e.Problema, e.ProblemaCodigo
				}
			})
		}
	}
	esperar.Wait()
	// Detección pasiva: si la última petición de un agente falló por autenticación y la detección no vio nada más
	// grave, se avisa (también de los agentes que no saben comprobar su sesión).
	for i, nombre := range nombres {
		if codigo, _ := lista[i]["problema_codigo"].(string); s.Salud.SinRespuesta(nombre) && (lista[i]["problema"] == nil || codigo == adaptadores.ProblemaAviso) {
			lista[i]["problema"] = idioma.T("la sesión del CLI no responde: la última petición falló por autenticación")
			lista[i]["problema_codigo"] = adaptadores.ProblemaSesion
		}
	}
	respuesta := map[string]any{
		"version":               version.Version,
		"formato_serializacion": adaptadores.FormatoSerializacion,
		"entradas":              []string{"texto", "markdown", "json", "xml", "csv", "html", "pdf", "docx", "pptx", "xlsx"},
		"adaptadores":           lista,
		"limites":               s.limites(),
	}
	if s.Conversor != nil {
		respuesta["conversor"] = s.Conversor.Estado()
	}
	EscribirJSON(w, http.StatusOK, respuesta)
}

// limites son los topes de una petición y de sus cargas, y los formatos admitidos, para que una app (y Probar) los
// conozca antes de enviar.
func (s *Servidor) limites() map[string]any {
	texto, conversor := documentos.Formatos()
	return map[string]any{
		"extensiones_texto":     texto,
		"extensiones_conversor": conversor,
		"cargas_por_peticion":   nucleo.MaximoCargas,
		"carga_mb":              s.Config.Limite("conversion.maximo_carga_mb"),
		"texto_convertido_mb":   s.Config.Limite("conversion.maximo_salida_mb"),
		"cuota_cargas_mb":       s.Config.Limite("conversion.cuota_cargas_mb"),
		"cuerpo_mb":             maximoCuerpo >> 20,
		"esquema_kb":            nucleo.MaximoEsquemaPeticion >> 10,
		"mensajes":              nucleo.MaximoMensajes,
		"reserva_salida_tokens": s.Config.Limite("limites.reserva_salida_tokens"),
	}
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
	// EsfuerzoPredeterminado es el nivel que usa el agente si no se elige; vacío si no se conoce con certeza.
	EsfuerzoPredeterminado string `json:"esfuerzo_predeterminado,omitempty"`
	// ContextoTokens es el contexto conocido del modelo (configurado o informado por el agente) y ContextoOrigen de
	// dónde sale; sin dato, no se comprueba si una entrada cabe.
	ContextoTokens int    `json:"contexto_tokens,omitempty"`
	ContextoOrigen string `json:"contexto_origen,omitempty"`
	// EsfuerzoEspacio, solo en el modelo predeterminado, es el esfuerzo que el espacio usa con él
	// (predeterminados.esfuerzo); vacío, el de su agente.
	EsfuerzoEspacio string `json:"esfuerzo_espacio,omitempty"`
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

// modelosConcretos pregunta a los agentes que saben decirlo (Claude) qué modelo concreto usarán para cada alias y
// para su predeterminado, en paralelo y con un plazo; lo que no llegue a tiempo queda en caché para la próxima vez.
func (s *Servidor) modelosConcretos(ctx context.Context, lista []modelo, destinos [][2]string) map[string]string {
	ctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	var mu sync.Mutex
	var grupo sync.WaitGroup
	concretos, pedidos := map[string]string{}, map[string]bool{}
	for i, d := range destinos {
		clave := d[0] + "/" + d[1]
		if d[0] == "" || lista[i].Origen == "visto" || pedidos[clave] {
			continue
		}
		pedidos[clave] = true
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			if concreto := adaptadores.ModeloConcreto(context.WithoutCancel(ctx), s.Registro[d[0]], d[0], d[1]); concreto != "" {
				mu.Lock()
				concretos[clave] = concreto
				mu.Unlock()
			}
		}()
	}
	listo := make(chan struct{})
	go func() { grupo.Wait(); close(listo) }()
	select {
	case <-listo:
	case <-ctx.Done():
	}
	mu.Lock()
	defer mu.Unlock()
	return maps.Clone(concretos)
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
	destinos := make([][2]string, len(lista)) // agente y modelo al que lleva cada entrada
	for i := range lista {
		m := &lista[i]
		m.Predeterminado = m.ID == pred
		if m.Predeterminado && slices.Contains(nucleo.NivelesEsfuerzo, s.Config.Texto("predeterminados.esfuerzo")) {
			m.EsfuerzoEspacio = s.Config.Texto("predeterminados.esfuerzo")
		}
		agente, modelo := m.Agente, m.Modelo
		if m.Perfil != "" {
			if agente, modelo = agentePred, perfiles[m.Perfil][agentePred]; modelo != "" {
				m.Resuelto = agente + "/" + modelo
			}
		}
		if agente != "" {
			destinos[i] = [2]string{agente, modelo}
			m.Esfuerzos = agentes.NivelesDeModelo(agente, modelo)
		}
	}
	concretos := s.modelosConcretos(r.Context(), lista, destinos)
	for i := range lista {
		if d := destinos[i]; d[0] != "" {
			// Lo que el agente dice que usará ahora manda sobre lo que informó la última vez que se usó.
			lista[i].Version = cmp.Or(concretos[d[0]+"/"+d[1]], efectivos[d[0]+"/"+d[1]])
			lista[i].EsfuerzoPredeterminado = agentes.EsfuerzoPredeterminado(d[0], cmp.Or(lista[i].Version, d[1]))
			lista[i].ContextoTokens, lista[i].ContextoOrigen = s.Planificador.Contexto(r.Context(), d[0], cmp.Or(lista[i].Version, d[1]))
		}
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"modelos": lista})
}
