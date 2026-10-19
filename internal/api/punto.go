package api

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// Punto es la política de un punto de acceso: contratos, valores predeterminados, alias y
// restricciones de modelo. Hereda los límites de su instancia y no puede ampliarlos.
type Punto struct {
	Nombre            string
	Contratos         map[string]bool
	Agente            string
	Modelo            string
	PerfilModelo      string
	Esfuerzo          string // si la solicitud no indica uno
	Instrucciones     string // se anteponen a las de la solicitud
	ModelosPermitidos []string
	ForzarModelo      bool
	Alias             map[string]string
	ModoContexto      string
	FormatoSalida     string
	TiempoLimite      int
	Esquema           json.RawMessage // JSON Schema de la receta, si la solicitud no trae uno
	Respaldo          []string        // agentes que siguen si el principal falla, si la solicitud no trae los suyos
	ReintentosSalida  int             // correcciones de una salida JSON que no cumple, si la solicitud no pide otras
}

// aceptar registra el trabajo; en un punto de acceso queda vinculado a él para medir el uso por aplicación. La
// Idempotency-Key se comprueba aquí para que la cumplan por igual la API propia y las fachadas.
func (s *Servidor) aceptar(ctx context.Context, sol nucleo.SolicitudEjecucion, clave string) (nucleo.Trabajo, bool, error) {
	if len(clave) > 255 {
		return nucleo.Trabajo{}, false, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("Idempotency-Key supera 255 caracteres"))
	}
	if s.Punto != nil {
		return s.Planificador.EnviarPorPunto(ctx, sol, clave, s.Punto.Nombre)
	}
	return s.Planificador.Enviar(ctx, sol, clave)
}

// ajustar aplica los predeterminados del punto (o su modelo forzado), luego los de la instancia, y comprueba
// que el modelo resultante está permitido en el punto.
func (s *Servidor) ajustar(sol nucleo.SolicitudEjecucion) (nucleo.SolicitudEjecucion, error) {
	sol, err := s.Config.AplicarPerfil(sol)
	if err != nil {
		return sol, err
	}
	p := s.Punto
	if p == nil {
		return s.Config.AplicarPredeterminados(sol), nil
	}
	sinModelo := sol.Agente == "" && sol.Modelo == "" && sol.PerfilModelo == ""
	if (p.ForzarModelo || sinModelo) && (p.Agente != "" || p.Modelo != "" || p.PerfilModelo != "") {
		sol.Agente, sol.Modelo, sol.PerfilModelo = p.Agente, p.Modelo, p.PerfilModelo
	}
	if sol.Esfuerzo == "" {
		sol.Esfuerzo = p.Esfuerzo
	}
	if p.Instrucciones != "" {
		sol.Instrucciones = strings.TrimSpace(p.Instrucciones + "\n\n" + sol.Instrucciones)
	}
	// El modo de contexto lo decide la API (aislado si no dice otro): ni la petición ni un perfil lo amplían.
	modo := cmp.Or(p.ModoContexto, "aislado")
	if sol.ModoContexto != "" && sol.ModoContexto != modo {
		return sol, nucleo.NuevoError(nucleo.CodigoContextoNoPermitido,
			idioma.Tf("la API %s trabaja en modo %s: la petición no puede pedir %s", p.Nombre, modo, sol.ModoContexto))
	}
	sol.ModoContexto = modo
	if len(sol.Salida.Esquema) == 0 && len(p.Esquema) > 0 {
		sol.Salida.Formato, sol.Salida.Esquema = "json", p.Esquema
	}
	if sol.Salida.Formato == "" && len(sol.Salida.Esquema) == 0 {
		sol.Salida.Formato = p.FormatoSalida
	}
	if sol.Salida.Reintentos == 0 && sol.Salida.Formato == "json" {
		sol.Salida.Reintentos = p.ReintentosSalida
	}
	// Con forzar_modelo el dueño de la API decide los modelos: el cliente tampoco elige el respaldo.
	if len(sol.Respaldo) == 0 || p.ForzarModelo {
		sol.Respaldo = p.Respaldo
	}
	if sol.Ejecucion.TiempoLimiteSegundos == 0 {
		sol.Ejecucion.TiempoLimiteSegundos = p.TiempoLimite
	}
	sol = s.Config.AplicarPredeterminados(sol)
	if len(p.ModelosPermitidos) > 0 && !permitido(p.ModelosPermitidos, sol) {
		return sol, nucleo.NuevoError(nucleo.CodigoModeloNoPermitido,
			idioma.Tf("el modelo %s no está permitido en el punto de acceso %s", descripcionModelo(sol), p.Nombre))
	}
	for _, e := range sol.Respaldo {
		if len(p.ModelosPermitidos) > 0 && !PermitidoRespaldo(p.ModelosPermitidos, e) {
			return sol, nucleo.NuevoError(nucleo.CodigoModeloNoPermitido,
				idioma.Tf("el modelo %s no está permitido en el punto de acceso %s", e, p.Nombre))
		}
	}
	return sol, nil
}

// PermitidoRespaldo indica si una entrada de respaldo (agente o agente/modelo) cabe en modelos_permitidos.
func PermitidoRespaldo(lista []string, entrada string) bool {
	agente, modelo, _ := nucleo.DividirRespaldo(entrada)
	return permitido(lista, nucleo.SolicitudEjecucion{Agente: agente, Modelo: modelo})
}

// permitido acepta "agente" (cualquier modelo del agente), "agente/modelo" o "perfil/nombre".
func permitido(lista []string, sol nucleo.SolicitudEjecucion) bool {
	for _, p := range lista {
		switch {
		case sol.Agente != "" && p == sol.Agente:
			return true
		case sol.Modelo != "" && p == sol.Agente+"/"+sol.Modelo:
			return true
		case sol.PerfilModelo != "" && p == "perfil/"+sol.PerfilModelo:
			return true
		}
	}
	return false
}

func descripcionModelo(sol nucleo.SolicitudEjecucion) string {
	switch {
	case sol.PerfilModelo != "":
		return "perfil/" + sol.PerfilModelo
	case sol.Modelo != "":
		return sol.Agente + "/" + sol.Modelo
	}
	return sol.Agente
}

// traducirModelo aplica alias_modelos: nombres fijos de una aplicación (por ejemplo gpt-4o-mini) a agente/modelo.
func (s *Servidor) traducirModelo(model string) string {
	if s.Punto != nil {
		if destino, ok := s.Punto.Alias[model]; ok {
			return destino
		}
	}
	return model
}

// trabajo devuelve un trabajo visible para este servidor: en un punto de acceso, solo los creados por él.
func (s *Servidor) trabajo(r *http.Request, id string) (nucleo.Trabajo, error) {
	t, err := s.Planificador.Obtener(r.Context(), id)
	if err == nil && s.Punto != nil && t.PuntoAcceso != s.Punto.Nombre {
		return nucleo.Trabajo{}, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("trabajo no encontrado: %s", id))
	}
	return t, err
}

// listarTrabajos devuelve los trabajos más recientes (?limite=, máximo 200); en un punto, solo los suyos.
func (s *Servidor) listarTrabajos(w http.ResponseWriter, r *http.Request) {
	limite := 20
	if valor := r.URL.Query().Get("limite"); valor != "" {
		n, err := strconv.Atoi(valor)
		if err != nil || n < 1 || n > 200 {
			EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("limite debe estar entre 1 y 200")))
			return
		}
		limite = n
	}
	q := r.URL.Query()
	f := persistencia.FiltroTrabajos{Punto: q.Get("punto")} // ?punto=nombre: solo las peticiones de ese punto de acceso
	if antes := q.Get("antes"); antes != "" {               // ?antes=: la página siguiente, por fecha de creación
		t, err := time.Parse(time.RFC3339Nano, antes)
		if err != nil {
			EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("antes debe ser una fecha RFC 3339")))
			return
		}
		f.Antes, f.AntesID = t, q.Get("antes_id") // antes_id desempata los creados en el mismo instante
	}
	if s.Punto != nil {
		f.Punto = s.Punto.Nombre // un punto de acceso solo ve las suyas: sin grupo ni origen
	} else {
		switch o := q.Get("origen"); o {
		case "", nucleo.DesdeProbar, nucleo.DesdeTerminal, nucleo.DesdeCLI, nucleo.DesdeApp:
			f.Origen = o
		default:
			EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("origen debe ser probar, terminal, cli o app")))
			return
		}
		if grupo := q.Get("grupo"); grupo != "" {
			if s.PuntosDeGrupo != nil {
				f.Puntos = s.PuntosDeGrupo(grupo)
			}
			if len(f.Puntos) == 0 { // un grupo sin APIs no tiene trabajos (nunca todos)
				EscribirJSON(w, http.StatusOK, map[string]any{"trabajos": []nucleo.Trabajo{}})
				return
			}
		}
	}
	lista, err := s.Planificador.Listar(r.Context(), limite, f)
	if err != nil {
		EscribirError(w, r, err)
		return
	}
	if lista == nil {
		lista = []nucleo.Trabajo{}
	}
	EscribirJSON(w, http.StatusOK, map[string]any{"trabajos": lista})
}

// filtrarCatalogo deja en un punto de acceso solo los modelos permitidos y añade sus alias.
func (s *Servidor) filtrarCatalogo(lista []modelo) []modelo {
	if s.Punto == nil {
		return lista
	}
	var salida []modelo
	for _, m := range lista {
		sol := nucleo.SolicitudEjecucion{Agente: m.Agente, Modelo: m.Modelo, PerfilModelo: m.Perfil}
		if len(s.Punto.ModelosPermitidos) == 0 || permitido(s.Punto.ModelosPermitidos, sol) {
			salida = append(salida, m)
		}
	}
	for alias, destino := range s.Punto.Alias {
		salida = append(salida, modelo{ID: alias, Origen: "alias:" + destino})
	}
	sort.Slice(salida, func(i, j int) bool { return salida[i].ID < salida[j].ID })
	return salida
}
