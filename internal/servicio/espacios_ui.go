package servicio

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tramtai/internal/api"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
)

// EspacioUI es un espacio tal como lo ven la página «Espacios» y la CLI.
type EspacioUI = contrato.Espacio

// Espacios devuelve todos los espacios conocidos con su estado, si son confiables y cuántas APIs y peticiones tienen
// (también los detenidos: sus APIs salen de su configuración y sus peticiones, de su base sin abrirla).
func (s *Servicio) Espacios(ctx context.Context) []EspacioUI {
	global, _ := nucleo.CargarConfiguracion("")
	globales := map[string]int{} // APIs de la configuración global, por espacio
	if global != nil {
		if puntos, err := LeerPuntos(global.Lista("puntos_acceso")); err == nil {
			for _, p := range puntos {
				globales[cmp.Or(p.Instancia, "global")]++
			}
		}
	}
	lista := []EspacioUI{}
	for _, e := range s.registro().lista() {
		u := EspacioUI{ID: e.ID, Ambito: e.Ambito, Ruta: e.Ruta, Nombre: e.Nombre, Activo: e.Activo, UltimoUso: e.UltimoUso,
			UltimoError: e.UltimoError, APIs: globales[e.ID]}
		inst := s.instancia(e.ID)
		switch {
		case inst != nil:
			u.Estado = "activo"
		case e.Ambito == "proyecto" && (e.Ruta == "" || !esProyecto(e.Ruta)):
			u.Estado = "no_encontrado"
		default:
			u.Estado = "detenido"
		}
		u.Confiado = e.Ambito == "global" || e.Ruta != "" && global != nil && global.ProyectoConfiable(e.Ruta)
		if e.Ambito == "proyecto" && u.Estado != "no_encontrado" {
			u.APIs += len(puntosLocales(e.Ruta))
			if !u.Confiado {
				u.Claves, _ = nucleo.ClavesAdministradas(filepath.Join(e.Ruta, ".tramtai", "local.yaml"))
			}
		}
		u.Peticiones = s.peticionesDe(ctx, e.ID, inst)
		lista = append(lista, u)
	}
	return lista
}

// puntosLocales son las APIs del local.yaml de un proyecto (las de uno no confiable no cuentan: no se abren).
func puntosLocales(raiz string) []ConfigPunto {
	capa, err := nucleo.CargarCapa(filepath.Join(raiz, ".tramtai", "local.yaml"), nucleo.OrigenLocal)
	if err != nil {
		return nil
	}
	lista, _ := capa.Valores["puntos_acceso"].([]any)
	puntos, _ := LeerPuntos(lista)
	return puntos
}

// peticionesDe cuenta los trabajos del espacio: del planificador si está activo, de su base si no (2 s como mucho).
func (s *Servicio) peticionesDe(ctx context.Context, id string, inst *Instancia) int {
	if inst != nil {
		ctx, cancelar := context.WithTimeout(ctx, 2*time.Second)
		defer cancelar()
		total := 0
		if porEstado, err := inst.Planificador.Contar(ctx); err == nil {
			for _, n := range porEstado {
				total += n
			}
		}
		return total
	}
	base := filepath.Join(s.opc.Datos, "instancias", id+".db")
	if _, err := os.Stat(base); err != nil {
		return 0
	}
	n, _ := persistencia.ContarEn(base)
	return n
}

// ActivarEspacio vuelve a abrir un espacio detenido por su id.
func (s *Servicio) ActivarEspacio(id string) (*Instancia, error) {
	e, ok := s.registro().buscar(id)
	if !ok {
		return nil, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no existe el espacio %s", id))
	}
	var inst *Instancia
	var err error
	switch {
	case e.Ambito == "global":
		inst, err = s.ActivarGlobal()
	case e.Ruta == "" || !esProyecto(e.Ruta):
		err = nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no se encuentra la carpeta del espacio %s: búscala de nuevo", e.Nombre))
	default:
		inst, err = s.ActivarProyecto(e.Ruta)
	}
	if err != nil {
		return nil, err
	}
	return inst, s.persistirEstado()
}

// DetenerEspacio detiene un espacio (lo que estaba en cola se cancela); sus datos y su configuración se quedan.
func (s *Servicio) DetenerEspacio(id string) error {
	if err := s.Desactivar(id); err != nil {
		return err
	}
	return s.persistirEstado()
}

// Ubicar asocia un espacio cuya carpeta no se encuentra (movida antes de la v0.54, o a mano) con ruta, que tiene que
// ser un proyecto: conserva su id, su token y su historial, y lo activa.
func (s *Servicio) Ubicar(id, ruta string) (*Instancia, error) {
	e, ok := s.registro().buscar(id)
	if !ok || e.Ambito != "proyecto" {
		return nil, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no existe el espacio %s", id))
	}
	ruta = nucleo.ExpandirCasa(strings.TrimSpace(ruta))
	if !filepath.IsAbs(ruta) {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("indica la ruta completa de la carpeta del proyecto"))
	}
	if real, err := filepath.EvalSymlinks(ruta); err == nil {
		ruta = real
	}
	if !esProyecto(ruta) {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s no tiene .tramtai/config.yaml: elige la carpeta del proyecto", ruta))
	}
	for _, otro := range s.registro().lista() {
		if otro.ID != id && otro.Ruta != "" && mismaCarpeta(otro.Ruta, ruta) {
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s ya es la carpeta del espacio %s", ruta, otro.Nombre))
		}
	}
	s.registro().cambiar(id, func(e *Espacio) { e.Ruta = ruta })
	if err := escribirMarca(ruta, id); err != nil {
		return nil, err
	}
	inst, err := s.ActivarProyecto(ruta)
	if err != nil {
		return nil, err
	}
	return inst, s.persistirEstado()
}

// Olvidar borra un espacio y sus datos en Tramtai (base, cargas, su token y los de sus APIs) después de confirmar su
// nombre. Nunca toca la carpeta del proyecto, y solo borra dentro de la carpeta de datos. La global no se olvida.
func (s *Servicio) Olvidar(id, nombre string) error {
	if !idValido.MatchString(id) || id == "global" {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("ese espacio no se puede olvidar"))
	}
	e, ok := s.registro().buscar(id)
	if !ok {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("no existe el espacio %s", id))
	}
	if !strings.EqualFold(strings.TrimSpace(nombre), e.Nombre) {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("escribe el nombre del espacio para confirmar que quieres olvidarlo"))
	}
	// Mientras se olvida, el id ocupa su sitio en abriendo: quien lo active a la vez espera y recibe un error, y no
	// queda una instancia abierta sobre archivos borrados.
	olvido := &apertura{listo: make(chan struct{}), err: nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("el espacio %s se estaba olvidando", id))}
	s.mu.Lock()
	if _, enCurso := s.abriendo[id]; enCurso {
		s.mu.Unlock()
		return nucleo.NuevoError(nucleo.CodigoTransicionInvalida, idioma.Tf("el espacio %s se está abriendo: vuelve a intentarlo en un momento", e.Nombre))
	}
	s.abriendo[id] = olvido
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.abriendo, id)
		s.mu.Unlock()
		close(olvido.listo)
	}()
	if s.instancia(id) != nil {
		if err := s.Desactivar(id); err != nil {
			return err
		}
	}
	base := filepath.Join(s.opc.Datos, "instancias", id+".db")
	rutas := []string{base, base + "-wal", base + "-shm", filepath.Join(s.opc.Datos, "cargas", id), RutaTokenInstancia(s.opc.Datos, id)}
	tokens := filepath.Join(s.opc.Datos, "tokens")
	for _, p := range s.apisDe(id, e.Ruta) {
		if ruta := RutaTokenPunto(s.opc.Datos, p); contrato.NombrePuntoValido(p) && dentroDe(tokens, ruta) {
			rutas = append(rutas, ruta)
		}
	}
	for _, ruta := range rutas {
		if dentroDe(s.opc.Datos, ruta) {
			if err := os.RemoveAll(ruta); err != nil {
				return err
			}
		}
	}
	s.registro().quitar(id)
	return s.persistirEstado()
}

// apisDe son los nombres de las APIs del espacio: las de su local.yaml y las de la global marcadas con su id.
func (s *Servicio) apisDe(id, raiz string) []string {
	var nombres []string
	if raiz != "" {
		for _, p := range puntosLocales(raiz) {
			nombres = append(nombres, p.Nombre)
		}
	}
	if global, err := nucleo.CargarConfiguracion(""); err == nil {
		if puntos, err := LeerPuntos(global.Lista("puntos_acceso")); err == nil {
			for _, p := range puntos {
				if p.Instancia == id {
					nombres = append(nombres, p.Nombre)
				}
			}
		}
	}
	return nombres
}

// dentroDe indica si ruta está dentro de base (y no es base): lo que se borra al olvidar nunca sale de los datos.
func dentroDe(base, ruta string) bool {
	rel, err := filepath.Rel(filepath.Clean(base), filepath.Clean(ruta))
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// confiarEspacio fija si se confía en el proyecto de un espacio; solo vale para el espacio de un proyecto con ruta.
func (s *Servicio) confiarEspacio(id string, confiar *bool) error {
	e, ok := s.registro().buscar(id)
	if !ok || e.Ambito != "proyecto" || e.Ruta == "" || confiar == nil {
		return nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("indica el espacio de un proyecto y si confías en él"))
	}
	return s.FijarConfianza(e.Ruta, *confiar)
}

// persistirEstado guarda el registro de espacios y el descriptor tras un cambio. Si no puede (disco lleno, el renombrado
// agotado en Windows), lo anota en el registro del servicio y en los errores del estado y lo devuelve: la acción del
// usuario no responde que salió bien, porque al reiniciar volvería a estar como antes.
func (s *Servicio) persistirEstado() error {
	err := errors.Join(s.guardarActivas(), s.actualizarDescriptor())
	texto := ""
	switch r := s.registro(); {
	case err != nil:
		texto = idioma.Tf("no se pudo guardar el estado de los espacios: %s", err.Error())
		fmt.Fprintln(s.opc.Registros, texto)
	case r.soloLectura && r.aviso != nil: // no se reescribe a propósito (no se pudo leer), pero que se vea
		texto = nucleo.MensajeDe(r.aviso)
	}
	s.mu.Lock()
	s.errorEstado = texto
	s.mu.Unlock()
	return err
}

func (s *Servicio) listarEspaciosUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	api.EscribirJSON(w, http.StatusOK, map[string]any{"espacios": s.Espacios(r.Context())})
}

// accionEspacioUI atiende /ui/espacios/{activar,detener,confiar,ubicar,olvidar} (el id va en el cuerpo) y responde la
// lista actualizada.
func (s *Servicio) accionEspacioUI(w http.ResponseWriter, r *http.Request, _ *sesion) {
	var peticion struct {
		ID      string `json:"id"`
		Confiar *bool  `json:"confiar"`
		Ruta    string `json:"ruta"`
		Nombre  string `json:"nombre"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	var err error
	switch filepath.Base(r.URL.Path) {
	case "activar":
		_, err = s.ActivarEspacio(peticion.ID)
	case "detener":
		err = s.DetenerEspacio(peticion.ID)
	case "confiar":
		err = s.confiarEspacio(peticion.ID, peticion.Confiar)
	case "ubicar":
		_, err = s.Ubicar(peticion.ID, peticion.Ruta)
	case "olvidar":
		err = s.Olvidar(peticion.ID, peticion.Nombre)
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.listarEspaciosUI(w, r, nil)
}
