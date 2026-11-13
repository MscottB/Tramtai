package servicio

import (
	"context"
	"slices"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/planificador"
	"tramtai/internal/preparar"
)

// preparacion es lo que el servicio usa de preparar.Preparador (las pruebas ponen una falsa que no instala nada).
type preparacion interface {
	api.Preparacion
	Versiones(ctx context.Context, forzar bool) []contrato.VersionesAgente
	Cerrar()
}

// agentesPreparables son los ids del catálogo con receta, en su orden.
func agentesPreparables() []string {
	var ids []string
	for _, f := range agentes.Todos() {
		if f.Receta != nil {
			ids = append(ids, f.Nombre)
		}
	}
	return ids
}

// nuevaPreparacion crea la preparación de los CLI de los agentes y recoge lo que dejó el servicio anterior (una
// preparación a medias queda interrumpida y su proceso, cerrado). Ninguna de sus funciones toma un cerrojo que el
// servicio tenga mientras llama a la preparación.
func (s *Servicio) nuevaPreparacion() *preparar.Preparador {
	p := preparar.Nuevo(preparar.Opciones{
		Datos:   s.opc.Datos,
		Ficha:   agentes.Buscar,
		EnCurso: s.peticionesEnCurso,
		Guardar: s.guardarComando,
		Comprobar: func(ctx context.Context, agente string) adaptadores.SesionCLI {
			if c, ok := s.opc.Registro[agente].(adaptadores.ComprobadorSesion); ok {
				return c.ComprobarSesion(ctx)
			}
			return adaptadores.SesionCLI{Estado: adaptadores.SesionDesconocida}
		},
		Agentes:         agentesPreparables(),
		BuscarVersiones: func() bool { return s.opc.Config.Booleano("servicio.buscar_versiones_agentes") },
	})
	p.Recuperar()
	return p
}

// peticionesEnCurso cuenta las peticiones sin terminar del agente en todas las instancias abiertas.
func (s *Servicio) peticionesEnCurso(agente string) int {
	s.mu.RLock()
	planificadores := make([]*planificador.Planificador, 0, len(s.instancias))
	for _, inst := range s.instancias {
		planificadores = append(planificadores, inst.Planificador)
	}
	s.mu.RUnlock()
	total := 0
	for _, p := range planificadores {
		n, err := p.ContarEnCurso(context.Background(), agente)
		if err != nil {
			return 1 // sin poder contarlas no se toca el CLI: mejor esperar que cortar una petición
		}
		total += n
	}
	return total
}

// preparando dice si el CLI del agente se está instalando, actualizando o desinstalando: sus peticiones nuevas reciben
// agente_no_disponible (el inicio de sesión no las frena).
func (s *Servicio) preparando(agente string) bool {
	e, ok := s.preparar.Estado(agente)
	return ok && slices.Contains([]string{"instalando", "actualizando", "desinstalando"}, e.Estado)
}

// guardarComando fija (o quita, con "") comandos_agentes.<agente> en la configuración global cuando el CLI quedó fuera
// del PATH del servicio, y vale al momento: el adaptador, creado al arrancar con el nombre del programa, llega a esa ruta.
func (s *Servicio) guardarComando(agente, ruta string) error {
	f, ok := agentes.Buscar(agente)
	if !ok || f.Comando == "" {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("no hay ningún agente %s", agente))
	}
	archivo, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	clave := "comandos_agentes." + agente
	if ruta == "" {
		err = configuracion.QuitarValor(archivo, configuracion.OrigenGlobal, clave)
	} else {
		err = configuracion.FijarValor(archivo, configuracion.OrigenGlobal, clave, ruta)
	}
	if err != nil {
		return err
	}
	adaptadores.FijarRutaPrograma(f.Comando, ruta)
	return nil
}

// versionesAgentes da la última comprobación de las versiones de los CLI. La repite con forzar, si aún no hay ninguna o
// si alguna preparación terminó después (la versión instalada cambió): cada una lanza --version de cada CLI y, sin red,
// espera hasta 10 s, así que no se repite en cada sondeo.
func (s *Servicio) versionesAgentes(ctx context.Context, forzar bool) []contrato.VersionesAgente {
	s.muVersiones.Lock()
	lista, en := s.versiones, s.versionesEn
	s.muVersiones.Unlock()
	if lista != nil && !forzar && !s.preparadoDesde(en) {
		return lista
	}
	return s.calcularVersiones(ctx, forzar)
}

// calcularVersiones pide las versiones a la preparación y guarda el resultado, si no se cortó a medias.
func (s *Servicio) calcularVersiones(ctx context.Context, forzar bool) []contrato.VersionesAgente {
	en := time.Now()
	lista := s.preparar.Versiones(ctx, forzar)
	if ctx.Err() == nil {
		s.muVersiones.Lock()
		if en.After(s.versionesEn) {
			s.versiones, s.versionesEn = lista, en
		}
		s.muVersiones.Unlock()
	}
	return lista
}

// preparadoDesde dice si alguna preparación terminó después de en.
func (s *Servicio) preparadoDesde(en time.Time) bool {
	for _, id := range agentesPreparables() {
		if e, ok := s.preparar.Estado(id); ok && e.Fin != nil && e.Fin.After(en) {
			return true
		}
	}
	return false
}

// buscarVersionesAgentes comprueba las versiones de los CLI al arrancar y después una vez al día, en segundo plano. La
// consulta del día lee también la lista firmada de versiones comprobadas; con servicio.buscar_versiones_agentes en false,
// Versiones no consulta nada por la red.
func (s *Servicio) buscarVersionesAgentes(ctx context.Context) {
	s.calcularVersiones(ctx, false) // la última versión publicada, solo si la guardada es de otro día
	reloj := time.NewTicker(24 * time.Hour)
	defer reloj.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-reloj.C:
			s.calcularVersiones(ctx, true)
		}
	}
}
