package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// politica clasifica los parámetros de una fachada.
type politica struct {
	soportados   map[string]bool
	ignorables   map[string]bool // se ignoran con aviso en la cabecera Tramtai-Advertencia
	noSoportados map[string]bool // cambian la semántica: error si traen un valor efectivo
}

func conjunto(claves ...string) map[string]bool {
	m := make(map[string]bool, len(claves))
	for _, c := range claves {
		m[c] = true
	}
	return m
}

// sinEfecto indica que un parámetro no soportado llega con un valor que no cambia nada (null, false, [], n=1).
func sinEfecto(clave string, valor json.RawMessage) bool {
	v := strings.TrimSpace(string(valor))
	return v == "null" || v == "false" || v == "[]" || v == "{}" || (clave == "n" && v == "1")
}

// revisar devuelve los parámetros ignorados o el error del primero que no puede respetarse.
func (p politica) revisar(campos map[string]json.RawMessage, tolerante bool) ([]string, error) {
	claves := make([]string, 0, len(campos))
	for clave := range campos {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	var avisos []string
	for _, clave := range claves {
		switch {
		case p.soportados[clave]:
		case p.ignorables[clave]:
			avisos = append(avisos, clave)
		case p.noSoportados[clave]:
			if !sinEfecto(clave, campos[clave]) {
				return nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("el parámetro %s no se puede respetar con los agentes CLI", clave))
			}
		case tolerante:
			avisos = append(avisos, clave)
		default:
			return nil, nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("parámetro desconocido: %s (activa compatibilidad.modo_tolerante para ignorarlo)", clave))
		}
	}
	return avisos, nil
}

// leerFachada lee el cuerpo, aplica la política de parámetros y lo decodifica en destino.
func (s *Servidor) leerFachada(w http.ResponseWriter, r *http.Request, p politica, destino any) ([]string, error) {
	datos, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maximoCuerpo))
	if err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("cuerpo demasiado grande o ilegible"))
	}
	var campos map[string]json.RawMessage
	if err := json.Unmarshal(datos, &campos); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("cuerpo JSON no válido: %s", err.Error()))
	}
	avisos, err := p.revisar(campos, s.Config.Booleano("compatibilidad.modo_tolerante"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(datos, destino); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("cuerpo JSON no válido: %s", err.Error()))
	}
	return avisos, nil
}

func avisar(w http.ResponseWriter, avisos []string) {
	if len(avisos) > 0 {
		w.Header().Add("Tramtai-Advertencia", idioma.Tf("parametros ignorados: %s", strings.Join(avisos, ", ")))
	}
}

// avisarUso marca uso_no_verificado cuando el agente no informó tokens: se envían ceros.
func avisarUso(w http.ResponseWriter, t nucleo.Trabajo) {
	if t.Uso.TokensEntrada == 0 && t.Uso.TokensSalida == 0 {
		w.Header().Add("Tramtai-Advertencia", "uso_no_verificado")
	}
}

// solicitudFachada traduce el campo model: agente/modelo, perfil/nombre, el nombre de un agente o un
// identificador sin prefijo que solo exista en el catálogo de un agente.
func (s *Servidor) solicitudFachada(model string) (nucleo.SolicitudEjecucion, error) {
	if s.Punto != nil && s.Punto.ForzarModelo {
		return nucleo.SolicitudEjecucion{}, nil // el punto fija el modelo e ignora el de la solicitud
	}
	model = s.traducirModelo(model)
	if model == "" {
		return nucleo.SolicitudEjecucion{}, nil
	}
	if perfil, ok := strings.CutPrefix(model, "perfil/"); ok {
		return nucleo.SolicitudEjecucion{PerfilModelo: perfil}, nil
	}
	if agente, resto, ok := strings.Cut(model, "/"); ok {
		if _, existe := s.Registro[agente]; existe {
			return nucleo.SolicitudEjecucion{Agente: agente, Modelo: resto}, nil
		}
	}
	if _, existe := s.Registro[model]; existe {
		return nucleo.SolicitudEjecucion{Agente: model}, nil
	}
	var agentes []string
	for _, porAgente := range s.Config.Perfiles() {
		for agente, m := range porAgente {
			if _, existe := s.Registro[agente]; existe && m == model && !contiene(agentes, agente) {
				agentes = append(agentes, agente)
			}
		}
	}
	switch len(agentes) {
	case 1:
		return nucleo.SolicitudEjecucion{Agente: agentes[0], Modelo: model}, nil
	case 0:
		return nucleo.SolicitudEjecucion{}, nucleo.NuevoError(nucleo.CodigoModeloNoDisponible,
			idioma.Tf("modelo desconocido %s: usa agente/modelo (consulta /openai/v1/models)", model))
	}
	sort.Strings(agentes)
	return nucleo.SolicitudEjecucion{}, nucleo.NuevoError(nucleo.CodigoModeloAmbiguo,
		idioma.Tf("el modelo %s existe en varios agentes (%s): usa agente/modelo", model, strings.Join(agentes, ", ")))
}

func contiene(lista []string, valor string) bool {
	for _, v := range lista {
		if v == valor {
			return true
		}
	}
	return false
}

// rolInterno traduce los roles de OpenAI y Anthropic.
func rolInterno(rol string) (string, error) {
	switch rol {
	case "system", "developer":
		return "sistema", nil
	case "user":
		return "usuario", nil
	case "assistant":
		return "asistente", nil
	}
	return "", nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("rol no soportado: %s", rol))
}

// textoDe acepta un texto o una lista de partes de texto; imágenes, audio o herramientas no se admiten.
func textoDe(contenido json.RawMessage) (string, error) {
	var texto string
	if json.Unmarshal(contenido, &texto) == nil {
		return texto, nil
	}
	var partes []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(contenido, &partes); err != nil {
		return "", nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("contenido de mensaje no válido"))
	}
	var b strings.Builder
	for i, p := range partes {
		switch p.Type {
		case "text", "input_text", "output_text":
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(p.Text)
		default:
			return "", nucleo.NuevoError(nucleo.CodigoParametroNoSoportado, idioma.Tf("contenido de tipo %s no soportado: solo texto", p.Type))
		}
	}
	return b.String(), nil
}

// enVivo indica si el texto emitido durante la ejecución es la respuesta. Con salida JSON o esquema el
// resultado final (por ejemplo, structured_output) puede diferir del texto emitido y se envía al terminar.
func enVivo(sol nucleo.SolicitudEjecucion) bool {
	return sol.Salida.Formato != "json" && len(sol.Salida.Esquema) == 0
}

// aceptarFachada completa la solicitud con los valores del punto de acceso y de la instancia y la acepta; si el
// punto fuerza el modelo, lo informa en la cabecera Tramtai-Modelo-Forzado.
func (s *Servidor) aceptarFachada(w http.ResponseWriter, r *http.Request, sol nucleo.SolicitudEjecucion) (nucleo.SolicitudEjecucion, nucleo.Trabajo, error) {
	sol, err := s.ajustar(sol)
	if err != nil {
		return sol, nucleo.Trabajo{}, err
	}
	if s.Punto != nil && s.Punto.ForzarModelo {
		w.Header().Set("Tramtai-Modelo-Forzado", descripcionModelo(sol))
	}
	t, _, err := s.aceptar(r.Context(), sol, r.Header.Get("Idempotency-Key"))
	return sol, t, err
}

// ejecutarFachada sigue el trabajo; si el cliente se desconecta y compatibilidad.cancelar_al_desconectar está
// activo, lo cancela, porque los clientes de estas API esperan que cerrar la conexión detenga la generación.
func (s *Servidor) ejecutarFachada(r *http.Request, t nucleo.Trabajo, alTexto func(string) error, latido func() error) (nucleo.Trabajo, error) {
	final, err := s.seguir(r.Context(), t.ID, 0, func(e persistencia.Evento) error {
		if e.Tipo == "texto" && alTexto != nil {
			return alTexto(e.Datos)
		}
		return nil
	}, latido)
	if err != nil && s.Config.Booleano("compatibilidad.cancelar_al_desconectar") {
		_, _ = s.Planificador.Cancelar(context.Background(), t.ID)
	}
	return final, err
}
