package api

import (
	"context"
	"net/http"
	"sort"
	"strconv"

	"tramtai/internal/nucleo"
)

// Punto es la política de un punto de acceso (sección 5.4): contratos, valores predeterminados, alias y
// restricciones de modelo. Hereda los límites de su instancia y no puede ampliarlos.
type Punto struct {
	Nombre            string
	Contratos         map[string]bool
	Agente            string
	Modelo            string
	PerfilModelo      string
	ModelosPermitidos []string
	ForzarModelo      bool
	Alias             map[string]string
	ModoContexto      string
	FormatoSalida     string
	TiempoLimite      int
}

// aceptar registra el trabajo; en un punto de acceso queda vinculado a él para medir el uso por aplicación.
func (s *Servidor) aceptar(ctx context.Context, sol nucleo.SolicitudEjecucion, clave string) (nucleo.Trabajo, bool, error) {
	if s.Punto != nil {
		return s.Planificador.EnviarPorPunto(ctx, sol, clave, s.Punto.Nombre)
	}
	return s.Planificador.Enviar(ctx, sol, clave)
}

// ajustar aplica los predeterminados del punto (o su modelo forzado), luego los de la instancia, y comprueba
// que el modelo resultante está permitido en el punto.
func (s *Servidor) ajustar(sol nucleo.SolicitudEjecucion) (nucleo.SolicitudEjecucion, error) {
	p := s.Punto
	if p == nil {
		return s.Config.AplicarPredeterminados(sol), nil
	}
	sinModelo := sol.Agente == "" && sol.Modelo == "" && sol.PerfilModelo == ""
	if (p.ForzarModelo || sinModelo) && (p.Agente != "" || p.Modelo != "" || p.PerfilModelo != "") {
		sol.Agente, sol.Modelo, sol.PerfilModelo = p.Agente, p.Modelo, p.PerfilModelo
	}
	if sol.ModoContexto == "" {
		sol.ModoContexto = p.ModoContexto
	}
	if sol.Salida.Formato == "" && len(sol.Salida.Esquema) == 0 {
		sol.Salida.Formato = p.FormatoSalida
	}
	if sol.Ejecucion.TiempoLimiteSegundos == 0 {
		sol.Ejecucion.TiempoLimiteSegundos = p.TiempoLimite
	}
	sol = s.Config.AplicarPredeterminados(sol)
	if len(p.ModelosPermitidos) > 0 && !permitido(p.ModelosPermitidos, sol) {
		return sol, nucleo.NuevoError(nucleo.CodigoModeloNoPermitido,
			"el modelo "+descripcionModelo(sol)+" no está permitido en el punto de acceso "+p.Nombre)
	}
	return sol, nil
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
		return nucleo.Trabajo{}, nucleo.NuevoError(nucleo.CodigoNoEncontrado, "trabajo no encontrado: "+id)
	}
	return t, err
}

// listarTrabajos devuelve los trabajos más recientes (?limite=, máximo 200); en un punto, solo los suyos.
func (s *Servidor) listarTrabajos(w http.ResponseWriter, r *http.Request) {
	limite := 20
	if valor := r.URL.Query().Get("limite"); valor != "" {
		n, err := strconv.Atoi(valor)
		if err != nil || n < 1 || n > 200 {
			EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, "limite debe estar entre 1 y 200"))
			return
		}
		limite = n
	}
	punto := ""
	if s.Punto != nil {
		punto = s.Punto.Nombre
	}
	lista, err := s.Planificador.Listar(r.Context(), limite, punto)
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
