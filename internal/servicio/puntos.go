package servicio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/salidas"
)

// ConfigPunto es un punto de acceso tal como se escribe en la configuración global o local.
type ConfigPunto struct {
	Nombre string `json:"nombre" yaml:"nombre"`
	Grupo  string `json:"grupo,omitempty" yaml:"grupo,omitempty"`
	// Instrucciones se anteponen a las de cada solicitud: por ejemplo, "Responde siempre en español".
	Instrucciones     string            `json:"instrucciones,omitempty" yaml:"instrucciones,omitempty"`
	Puerto            int               `json:"puerto,omitempty" yaml:"puerto,omitempty"`
	RutaBase          string            `json:"ruta_base,omitempty" yaml:"ruta_base,omitempty"`
	Instancia         string            `json:"instancia,omitempty" yaml:"instancia,omitempty"`
	Contratos         []string          `json:"contratos,omitempty" yaml:"contratos,omitempty"`
	Agente            string            `json:"agente,omitempty" yaml:"agente,omitempty"`
	Modelo            string            `json:"modelo,omitempty" yaml:"modelo,omitempty"`
	PerfilModelo      string            `json:"perfil_modelo,omitempty" yaml:"perfil_modelo,omitempty"`
	Esfuerzo          string            `json:"esfuerzo,omitempty" yaml:"esfuerzo,omitempty"`
	ModelosPermitidos []string          `json:"modelos_permitidos,omitempty" yaml:"modelos_permitidos,omitempty"`
	ForzarModelo      bool              `json:"forzar_modelo,omitempty" yaml:"forzar_modelo,omitempty"`
	AliasModelos      map[string]string `json:"alias_modelos,omitempty" yaml:"alias_modelos,omitempty"`
	// Esquema es el JSON Schema de la receta (escrito como YAML); con él la salida es JSON validado.
	Esquema map[string]any `json:"esquema,omitempty" yaml:"esquema,omitempty"`
	// Respaldo son los agentes (agente o agente/modelo) que siguen si el principal falla antes de producir texto.
	Respaldo []string `json:"respaldo,omitempty" yaml:"respaldo,omitempty"`
	// ReintentosSalida es cuántas veces se pide al agente que corrija una respuesta JSON que no cumple (0 a 3).
	ReintentosSalida int `json:"reintentos_salida,omitempty" yaml:"reintentos_salida,omitempty"`
	// Presupuesto limita el consumo diario y mensual de la API (peticiones, tokens, USD).
	Presupuesto *nucleo.Presupuesto `json:"presupuesto,omitempty" yaml:"presupuesto,omitempty"`
	Activo      *bool               `json:"activo,omitempty" yaml:"activo,omitempty"`
	Ejecucion   *EjecucionPunto     `json:"ejecucion,omitempty" yaml:"ejecucion,omitempty"`
}

// EjecucionPunto son los valores de ejecución propios del punto, dentro de los límites de la instancia.
type EjecucionPunto struct {
	ModoContexto         string `json:"modo_contexto,omitempty" yaml:"modo_contexto,omitempty"`
	FormatoSalida        string `json:"formato_salida,omitempty" yaml:"formato_salida,omitempty"`
	TiempoLimiteSegundos int    `json:"tiempo_limite_segundos,omitempty" yaml:"tiempo_limite_segundos,omitempty"`
	// MaximoTrabajos limita las ejecuciones simultáneas del punto; no amplía las de su instancia.
	MaximoTrabajos int `json:"maximo_trabajos,omitempty" yaml:"maximo_trabajos,omitempty"`
}

// NombrePuntoValido indica si el nombre sirve para un punto de acceso: también impide que una ruta derivada de él
// (como la de su clave) salga de su carpeta.
func NombrePuntoValido(nombre string) bool { return reNombrePunto.MatchString(nombre) }

// EstaActivo indica si el punto está habilitado (lo está salvo activo: false).
func (p ConfigPunto) EstaActivo() bool { return p.Activo == nil || *p.Activo }

var (
	reNombrePunto    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)
	contratosValidos = map[string]bool{"nativa": true, "openai": true, "anthropic": true}
)

// LeerPuntos decodifica la lista puntos_acceso; un campo desconocido o mal tipado es un error.
func LeerPuntos(lista []any) ([]ConfigPunto, error) {
	if len(lista) == 0 {
		return nil, nil
	}
	datos, err := json.Marshal(lista)
	if err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.T("puntos_acceso no válido"))
	}
	d := json.NewDecoder(bytes.NewReader(datos))
	d.DisallowUnknownFields()
	var puntos []ConfigPunto
	if err := d.Decode(&puntos); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, "puntos_acceso: "+err.Error())
	}
	return puntos, nil
}

// Normalizar completa los valores predeterminados (instancia, ruta /p/<nombre> y los tres contratos) y valida el punto.
func (p ConfigPunto) Normalizar(instancia string, puertoServicio int) (ConfigPunto, error) {
	invalido := func(motivo string) (ConfigPunto, error) {
		return p, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("punto de acceso %s: %s", p.Nombre, motivo))
	}
	if !reNombrePunto.MatchString(p.Nombre) {
		return invalido(idioma.T("nombre no válido (minúsculas, números, _ y -; hasta 40)"))
	}
	if p.Instancia == "" {
		p.Instancia = instancia
	}
	if utf8.RuneCountInString(p.Grupo) > 40 {
		return invalido(idioma.T("grupo de más de 40 caracteres"))
	}
	switch {
	case p.Puerto != 0 && p.RutaBase != "":
		return invalido(idioma.T("usa puerto o ruta_base, no ambos"))
	case p.Puerto != 0 && (p.Puerto < 1024 || p.Puerto > 65535 || p.Puerto == puertoServicio):
		return invalido(idioma.T("puerto fuera de 1024-65535 o igual al del servicio"))
	case p.Puerto == 0 && p.RutaBase == "":
		p.RutaBase = "/p/" + p.Nombre
	case p.Puerto == 0 && p.RutaBase != "/p/"+p.Nombre:
		return invalido(idioma.Tf("ruta_base debe ser /p/%s", p.Nombre))
	}
	if len(p.Contratos) == 0 {
		p.Contratos = []string{"nativa", "openai", "anthropic"}
	}
	for _, c := range p.Contratos {
		if !contratosValidos[c] {
			return invalido(idioma.Tf("contrato desconocido %s (nativa, openai, anthropic)", c))
		}
	}
	if p.Modelo != "" && p.PerfilModelo != "" {
		return invalido(idioma.T("modelo y perfil_modelo son excluyentes"))
	}
	if p.Esfuerzo != "" && !slices.Contains(nucleo.NivelesEsfuerzo, p.Esfuerzo) {
		return invalido(idioma.Tf("esfuerzo desconocido %s (bajo, medio, alto, muy_alto, maximo)", p.Esfuerzo))
	}
	if p.ForzarModelo && p.Agente == "" && p.Modelo == "" && p.PerfilModelo == "" {
		return invalido(idioma.T("forzar_modelo exige agente, modelo o perfil_modelo"))
	}
	if p.ReintentosSalida < 0 || p.ReintentosSalida > nucleo.MaximoReintentosSalida {
		return invalido(idioma.Tf("reintentos_salida va de 0 a %d", nucleo.MaximoReintentosSalida))
	}
	if p.ReintentosSalida > 0 && len(p.Esquema) == 0 && (p.Ejecucion == nil || p.Ejecucion.FormatoSalida != "json") {
		return invalido(idioma.T("reintentos_salida exige un esquema o ejecucion.formato_salida json"))
	}
	if p.Ejecucion != nil && p.Ejecucion.MaximoTrabajos < 0 {
		return invalido(idioma.T("ejecucion.maximo_trabajos no puede ser negativo"))
	}
	if len(p.Esquema) > 0 {
		esquema, err := json.Marshal(p.Esquema)
		if err == nil {
			_, err = salidas.Compilar(esquema)
		}
		if err != nil {
			return invalido(nucleo.MensajeDe(err)) // ya dice «esquema no válido» o «el esquema no es JSON válido»
		}
	}
	if p.Presupuesto != nil {
		if err := p.Presupuesto.Validar(); err != nil {
			return invalido(nucleo.MensajeDe(err))
		}
		if p.Presupuesto.Vacio() {
			p.Presupuesto = nil
		}
	}
	if err := nucleo.ValidarRespaldo(p.Respaldo); err != nil {
		return invalido(nucleo.MensajeDe(err))
	}
	return p, nil
}

// validarRespaldo rechaza un respaldo con el que toda petición fallaría: un agente que no existe o una entrada fuera de
// modelos_permitidos. Normalizar solo ve el formato. El servicio pregunta a su registro; la CLI y la terminal, al
// catálogo (enCatalogo).
func validarRespaldo(p ConfigPunto, existe func(agente string) bool) error {
	for _, e := range p.Respaldo {
		agente, _, _ := nucleo.DividirRespaldo(e)
		motivo := ""
		switch {
		case !existe(agente):
			motivo = idioma.Tf("el agente de respaldo %s no existe", agente)
		case len(p.ModelosPermitidos) > 0 && !api.PermitidoRespaldo(p.ModelosPermitidos, e):
			motivo = idioma.Tf("el respaldo %s no está en modelos_permitidos", e)
		default:
			continue
		}
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("punto de acceso %s: %s", p.Nombre, motivo))
	}
	return nil
}

// ValidarParaApps rechaza al guardar una API que use (como agente o en el respaldo) un agente que no se permite en apps
// (adaptadores.<nombre>.permitir_apps): sus peticiones se rechazarían todas. La usan la Web UI y la terminal.
func validarParaApps(p ConfigPunto) error {
	c, _ := nucleo.CargarConfiguracion("") // sin configuración, solo cuenta la ficha de cada agente
	usados := []string{p.Agente}
	for _, e := range p.Respaldo {
		agente, _, _ := nucleo.DividirRespaldo(e)
		usados = append(usados, agente)
	}
	for _, agente := range usados {
		if agente != "" && !agentes.PermitidoEnApps(c, agente) {
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("punto de acceso %s: %s", p.Nombre,
				idioma.Tf("%s no está habilitado para aplicaciones: todavía no ha pasado su prueba de seguridad (canario). Úsalo desde Probar o la terminal", agentes.NombreVisible(agente))))
		}
	}
	return nil
}

func enCatalogo(agente string) bool {
	_, ok := agentes.Buscar(agente)
	return ok
}

// existe dice si el agente está en el registro del servicio.
func (s *Servicio) existe(agente string) bool { return s.opc.Registro[agente] != nil }

// ValidarPunto aplica las reglas para guardar una API que la Web UI, la CLI y la terminal comparten: un respaldo con el que
// funcione y solo agentes habilitados para apps. El formato lo comprueba Normalizar.
func ValidarPunto(p ConfigPunto) error { return validarPunto(p, enCatalogo) }

func validarPunto(p ConfigPunto, existe func(string) bool) error {
	if err := validarRespaldo(p, existe); err != nil {
		return err
	}
	return validarParaApps(p)
}

// ConReglas envuelve una edición de puntos de acceso con ValidarPunto en los puntos nuevos o que cambian algo más que
// activo: así se puede desactivar una API que ya tenía otro error, pero no guardar una nueva que fallaría siempre.
func ConReglas(editar func([]ConfigPunto) ([]ConfigPunto, error)) func([]ConfigPunto) ([]ConfigPunto, error) {
	// huella es el contenido del punto sin activo: se toma antes de editar, porque editar puede cambiar la lista (y lo
	// que apuntan sus punteros) en su sitio.
	huella := func(p ConfigPunto) string {
		p.Activo = nil
		datos, _ := json.Marshal(p)
		return string(datos)
	}
	return func(lista []ConfigPunto) ([]ConfigPunto, error) {
		antes := map[string]bool{}
		for _, p := range lista {
			antes[huella(p)] = true
		}
		nueva, err := editar(lista)
		if err != nil {
			return nil, err
		}
		for _, p := range nueva {
			if !antes[huella(p)] {
				if err := ValidarPunto(p); err != nil {
					return nil, err
				}
			}
		}
		return nueva, nil
	}
}

// EliminarPunto quita un punto de acceso del archivo y después borra su clave, antes de recargar: así el servicio sabe al
// recargar que se eliminó (y no que una errata lo invalidó), y otro con el mismo nombre no hereda ni la clave ni el
// consumo de su presupuesto.
func EliminarPunto(datos, ruta string, origen nucleo.Origen, nombre string) error {
	err := EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		i := slices.IndexFunc(lista, func(p ConfigPunto) bool { return p.Nombre == nombre })
		if i < 0 {
			return nil, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no existe el punto de acceso %s en este archivo", nombre))
		}
		return slices.Delete(lista, i, i+1), nil
	})
	if err == nil {
		EliminarTokenPunto(datos, nombre)
	}
	return err
}

func (p ConfigPunto) politica() *api.Punto {
	pol := &api.Punto{Nombre: p.Nombre, Contratos: map[string]bool{}, Agente: p.Agente, Modelo: p.Modelo,
		PerfilModelo: p.PerfilModelo, Esfuerzo: p.Esfuerzo, Instrucciones: strings.TrimSpace(p.Instrucciones), ModelosPermitidos: p.ModelosPermitidos, ForzarModelo: p.ForzarModelo, Alias: p.AliasModelos}
	for _, c := range p.Contratos {
		pol.Contratos[c] = true
	}
	if e := p.Ejecucion; e != nil {
		pol.ModoContexto, pol.FormatoSalida, pol.TiempoLimite = e.ModoContexto, e.FormatoSalida, e.TiempoLimiteSegundos
	}
	if len(p.Esquema) > 0 {
		pol.Esquema, _ = json.Marshal(p.Esquema) // ya validado en Normalizar
	}
	pol.Respaldo, pol.ReintentosSalida = p.Respaldo, p.ReintentosSalida
	return pol
}

// puntoActivo es un punto de acceso abierto (o el motivo por el que no lo está).
type puntoActivo struct {
	cfg       ConfigPunto
	origen    string
	firma     string
	instancia *Instancia
	handler   http.Handler
	srv       *http.Server
	oyente    net.Listener
	url       string
	err       string
}

// soltar deja de aceptar conexiones y libera el puerto al momento, para reabrirlo con s.mu tomado.
func (p *puntoActivo) soltar() {
	if p.oyente != nil {
		_ = p.oyente.Close()
	}
}

// cerrar espera hasta 2 s a las peticiones en curso; se llama sin s.mu para no frenar al resto del servicio.
func (p *puntoActivo) cerrar() {
	if p.srv == nil {
		return
	}
	p.soltar()
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	// Shutdown también avisa de que el oyente ya estaba cerrado; solo se corta si vence el plazo.
	if err := p.srv.Shutdown(ctx); errors.Is(err, context.DeadlineExceeded) {
		p.srv.Close()
	}
}

// EstadoPunto resume un punto de acceso.
type EstadoPunto struct {
	Nombre string `json:"nombre"`
	Grupo  string `json:"grupo,omitempty"`
	// Instrucciones del punto, para verlas y editarlas en la Web UI.
	Instrucciones string   `json:"instrucciones,omitempty"`
	Instancia     string   `json:"instancia"`
	URL           string   `json:"url,omitempty"`
	Contratos     []string `json:"contratos"`
	Agente        string   `json:"agente,omitempty"`
	Modelo        string   `json:"modelo,omitempty"`
	PerfilModelo  string   `json:"perfil_modelo,omitempty"`
	Esfuerzo      string   `json:"esfuerzo,omitempty"`
	Permitidos    []string `json:"modelos_permitidos,omitempty"`
	Forzar        bool     `json:"forzar_modelo,omitempty"`
	// Esquema y Respaldo son la receta del punto, para mostrarla y abrirla en Probar.
	Esquema  map[string]any `json:"esquema,omitempty"`
	Respaldo []string       `json:"respaldo,omitempty"`
	// ReintentosSalida son las correcciones de la receta si la respuesta no cumple el esquema.
	ReintentosSalida int `json:"reintentos_salida,omitempty"`
	// Presupuesto son los límites de la API; su consumo lo da GET /api/v1/puntos en «presupuestos».
	Presupuesto *nucleo.Presupuesto `json:"presupuesto,omitempty"`
	Activo      bool                `json:"activo"`
	Error       string              `json:"error,omitempty"`
	Origen      string              `json:"origen"`
}

// candidato es un punto leído de la configuración con el archivo del que procede.
type candidato struct {
	cfg    ConfigPunto
	origen string
}

// puntosDeseados lee los puntos de la configuración global y de la local de cada proyecto activo; nunca de la
// versionada, que la configuración ya rechaza.
func (s *Servicio) puntosDeseados() ([]candidato, []string) {
	var errores []string
	var salida []candidato
	vistos := map[string]bool{}
	agregar := func(lista []any, instancia, origen string) {
		puntos, err := LeerPuntos(lista)
		if err != nil {
			errores = append(errores, origen+": "+err.Error())
			return
		}
		for _, p := range puntos {
			normalizado, err := p.Normalizar(instancia, s.opc.Puerto)
			if err == nil {
				err = validarRespaldo(normalizado, s.existe)
			}
			switch {
			case err != nil:
				errores = append(errores, origen+": "+nucleo.MensajeDe(err))
			case vistos[normalizado.Nombre]:
				errores = append(errores, idioma.Tf("%s: punto de acceso duplicado %s", origen, normalizado.Nombre))
			default:
				vistos[normalizado.Nombre] = true
				salida = append(salida, candidato{cfg: normalizado, origen: origen})
			}
		}
	}
	rutaGlobal, _ := nucleo.RutaGlobal()
	if global, err := nucleo.CargarConfiguracion(""); err != nil {
		errores = append(errores, nucleo.MensajeDe(err))
	} else {
		agregar(global.Lista("puntos_acceso"), "global", rutaGlobal)
	}
	s.mu.RLock()
	var proyectos []*Instancia
	for _, inst := range s.instancias {
		if inst.Ambito == "proyecto" {
			proyectos = append(proyectos, inst)
		}
	}
	s.mu.RUnlock()
	sort.Slice(proyectos, func(i, j int) bool { return proyectos[i].ID < proyectos[j].ID })
	for _, inst := range proyectos {
		ruta := filepath.Join(inst.Raiz, ".tramtai", "local.yaml")
		capa, err := nucleo.CargarCapa(ruta, nucleo.OrigenLocal)
		if err != nil {
			errores = append(errores, nucleo.MensajeDe(err))
			continue
		}
		lista, _ := capa.Valores["puntos_acceso"].([]any)
		agregar(lista, inst.ID, ruta)
	}
	return salida, errores
}

// Recargar aplica la configuración de puntos de acceso sin reiniciar: abre los nuevos, cierra los retirados y
// deja intactos los que no cambiaron, sin afectar a los trabajos en curso.
func (s *Servicio) Recargar() []EstadoPunto {
	s.muRecargar.Lock()
	deseados, errores := s.puntosDeseados()
	var retirados []*puntoActivo // se cierran al final, sin s.mu ni muRecargar: cerrar uno puede tardar
	defer func() {
		s.muRecargar.Unlock()
		for _, p := range retirados {
			p.cerrar()
		}
	}()
	s.mu.Lock()
	nuevos := map[string]*puntoActivo{}
	for _, c := range deseados {
		huella, err := LeerHuellaPunto(RutaTokenPunto(s.opc.Datos, c.cfg.Nombre))
		if err != nil {
			errores = append(errores, err.Error())
			continue
		}
		firma := firmaPunto(c.cfg, huella)
		actual := s.puntos[c.cfg.Nombre]
		if actual != nil && actual.firma == firma && actual.err == "" && actual.instancia == s.instancias[c.cfg.Instancia] {
			nuevos[c.cfg.Nombre] = actual
			continue
		}
		if actual != nil {
			actual.soltar() // libera el puerto por si el punto nuevo usa el mismo
			retirados = append(retirados, actual)
		}
		nuevos[c.cfg.Nombre] = s.abrirPunto(c, huella, firma)
	}
	type eliminado struct {
		inst   *Instancia
		nombre string
	}
	var eliminados []eliminado
	for nombre, actual := range s.puntos {
		if _, sigue := nuevos[nombre]; !sigue {
			retirados = append(retirados, actual)
			// Un punto que sale de la configuración sin clave se eliminó (la UI y la CLI la borran antes de recargar): su
			// consumo deja de ser suyo para que otro con el mismo nombre no lo herede. Con la clave aún en su sitio
			// (una errata en el YAML lo invalidó) se conserva.
			if _, err := os.Stat(RutaTokenPunto(s.opc.Datos, nombre)); os.IsNotExist(err) && actual.instancia != nil &&
				s.instancias[actual.instancia.ID] == actual.instancia {
				eliminados = append(eliminados, eliminado{actual.instancia, nombre})
			}
		}
	}
	s.puntos, s.erroresPuntos = nuevos, errores
	limites := map[string]map[string]int{}
	for _, c := range deseados {
		if c.cfg.Ejecucion != nil && c.cfg.Ejecucion.MaximoTrabajos > 0 {
			if limites[c.cfg.Instancia] == nil {
				limites[c.cfg.Instancia] = map[string]int{}
			}
			limites[c.cfg.Instancia][c.cfg.Nombre] = c.cfg.Ejecucion.MaximoTrabajos
		}
	}
	presupuestos := map[string]map[string]nucleo.Presupuesto{}
	for _, c := range deseados {
		if c.cfg.Presupuesto != nil {
			if presupuestos[c.cfg.Instancia] == nil {
				presupuestos[c.cfg.Instancia] = map[string]nucleo.Presupuesto{}
			}
			presupuestos[c.cfg.Instancia][c.cfg.Nombre] = *c.cfg.Presupuesto
		}
	}
	for id, inst := range s.instancias {
		inst.Planificador.LimitarPuntos(limites[id])
		m := presupuestos[id]
		if m == nil {
			m = map[string]nucleo.Presupuesto{}
		}
		m[persistencia.TodaLaInstancia] = presupuestoInstancia(inst)
		inst.Planificador.FijarPresupuestos(m)
	}
	estado := s.estadoPuntos()
	instancias := make([]*Instancia, 0, len(s.instancias))
	for _, inst := range s.instancias {
		instancias = append(instancias, inst)
	}
	deseadosPor := map[string]map[string]bool{}
	for _, c := range deseados {
		if deseadosPor[c.cfg.Instancia] == nil {
			deseadosPor[c.cfg.Instancia] = map[string]bool{}
		}
		deseadosPor[c.cfg.Instancia][c.cfg.Nombre] = true
	}
	s.mu.Unlock()
	// Una API que se eliminó con el servicio detenido no pasó por el bucle de arriba: si tiene consumo, no está en
	// la configuración y ya no tiene clave, su consumo tampoco es de nadie.
	for _, inst := range instancias {
		puntos, err := inst.Planificador.PuntosConConsumo(context.Background())
		if err != nil {
			continue
		}
		for _, nombre := range puntos {
			if _, sigue := deseadosPor[inst.ID][nombre]; sigue {
				continue
			}
			if _, err := os.Stat(RutaTokenPunto(s.opc.Datos, nombre)); os.IsNotExist(err) {
				eliminados = append(eliminados, eliminado{inst, nombre})
			}
		}
	}
	for _, e := range eliminados {
		if err := e.inst.Planificador.EliminarConsumo(context.Background(), e.nombre); err != nil {
			fmt.Fprintf(s.opc.Registros, "punto de acceso %s: no se pudo borrar su consumo: %v\n", e.nombre, err)
		}
	}
	for _, p := range estado {
		if p.Error != "" && p.Error != "desactivado" {
			fmt.Fprintf(s.opc.Registros, "punto de acceso %s: %s\n", p.Nombre, p.Error)
		}
	}
	return estado
}

// presupuestoInstancia relee el presupuesto de la configuración de la instancia, para que un cambio se aplique al
// recargar sin reiniciar el servicio; si no se puede leer, vale el de la configuración con que se abrió.
func presupuestoInstancia(inst *Instancia) nucleo.Presupuesto {
	raiz := ""
	if inst.Ambito == "proyecto" {
		raiz = inst.Raiz
	}
	if c, err := nucleo.CargarConfiguracion(raiz); err == nil {
		return nucleo.PresupuestoDe(c)
	}
	return nucleo.PresupuestoDe(inst.Config)
}

func firmaPunto(cfg ConfigPunto, huella string) string {
	datos, _ := json.Marshal(cfg)
	return string(datos) + "|" + huella
}

// abrirPunto prepara el punto; si tiene puerto propio y está ocupado no se activa ni se elige otro. huella es la de
// su clave: el servicio nunca conoce la clave de un punto. Se llama con s.mu tomado.
func (s *Servicio) abrirPunto(c candidato, huella, firma string) *puntoActivo {
	p := &puntoActivo{cfg: c.cfg, origen: c.origen, firma: firma, instancia: s.instancias[c.cfg.Instancia]}
	switch {
	case !c.cfg.EstaActivo():
		p.err = "desactivado"
		return p
	case p.instancia == nil:
		p.err = idioma.Tf("la instancia %s no está activa", c.cfg.Instancia)
		return p
	}
	inst := p.instancia
	srv := &api.Servidor{Planificador: inst.Planificador, Cargas: inst.Cargas, Registro: s.opc.Registro, Config: inst.Config, Punto: c.cfg.politica()}
	handler := api.Proteger(s.opc.Hosts, func(r *http.Request) bool { return api.HuellaValida(r, huella) }, srv.Rutas())
	if c.cfg.Puerto == 0 {
		p.handler, p.url = handler, s.Base()+c.cfg.RutaBase
		return p
	}
	direccion := net.JoinHostPort(s.opc.Host, strconv.Itoa(c.cfg.Puerto))
	oyente, err := net.Listen("tcp", direccion)
	if err != nil {
		p.err = idioma.Tf("no se pudo abrir %s (¿puerto en uso?)", direccion)
		return p
	}
	p.handler, p.url, p.oyente = handler, "http://"+direccion, oyente
	p.srv = &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 64 << 10,
		ConnContext: ContextoDeConexion}
	go p.srv.Serve(oyente)
	return p
}

// estadoPuntos se llama con s.mu tomado (lectura o escritura).
func (s *Servicio) estadoPuntos() []EstadoPunto {
	lista := []EstadoPunto{}
	for _, p := range s.puntos {
		lista = append(lista, EstadoPunto{Nombre: p.cfg.Nombre, Grupo: p.cfg.Grupo, Instrucciones: p.cfg.Instrucciones, Instancia: p.cfg.Instancia, URL: p.url, Contratos: p.cfg.Contratos,
			Agente: p.cfg.Agente, Modelo: p.cfg.Modelo, PerfilModelo: p.cfg.PerfilModelo, Esfuerzo: p.cfg.Esfuerzo,
			Permitidos: p.cfg.ModelosPermitidos, Forzar: p.cfg.ForzarModelo, Esquema: p.cfg.Esquema, Respaldo: p.cfg.Respaldo, ReintentosSalida: p.cfg.ReintentosSalida, Presupuesto: p.cfg.Presupuesto,
			Activo: p.err == "", Error: p.err, Origen: p.origen})
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Nombre < lista[j].Nombre })
	return lista
}

// EditarPuntos modifica la lista puntos_acceso de un archivo YAML conservando el resto del archivo y sus
// comentarios. Si el resultado no es una configuración válida, deja el archivo como estaba.
func EditarPuntos(ruta string, origen nucleo.Origen, editar func([]ConfigPunto) ([]ConfigPunto, error)) error {
	return editarYAML(ruta, origen, func(raiz *yaml.Node) error {
		indice := -1
		var lista []any
		for i := 0; i+1 < len(raiz.Content); i += 2 {
			if raiz.Content[i].Value == "puntos_acceso" {
				indice = i + 1
				if err := raiz.Content[indice].Decode(&lista); err != nil {
					return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: puntos_acceso debe ser una lista", ruta))
				}
				break
			}
		}
		puntos, err := LeerPuntos(lista)
		if err != nil {
			return err
		}
		if puntos, err = editar(puntos); err != nil {
			return err
		}
		var valor yaml.Node
		if err := valor.Encode(puntos); err != nil {
			return err
		}
		if len(puntos) == 0 {
			valor = yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		}
		if indice >= 0 {
			raiz.Content[indice] = &valor
		} else {
			raiz.Content = append(raiz.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "puntos_acceso"}, &valor)
		}
		return nil
	})
}

// FijarValor escribe una clave con puntos (adaptadores.claude.autenticacion) en un archivo YAML, creando los
// mapas que falten y conservando el resto del archivo y sus comentarios.
func FijarValor(ruta string, origen nucleo.Origen, clave string, valor any) error {
	return editarYAML(ruta, origen, func(nodo *yaml.Node) error {
		partes := strings.Split(clave, ".")
		for i, parte := range partes {
			if nodo.Kind != yaml.MappingNode {
				return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: %s debe ser un mapa", ruta, strings.Join(partes[:i], ".")))
			}
			var hijo *yaml.Node
			for j := 0; j+1 < len(nodo.Content); j += 2 {
				if nodo.Content[j].Value == parte {
					hijo = nodo.Content[j+1]
					break
				}
			}
			if hijo == nil {
				hijo = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				nodo.Content = append(nodo.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: parte}, hijo)
			}
			if i == len(partes)-1 {
				var nuevo yaml.Node
				if err := nuevo.Encode(valor); err != nil {
					return err
				}
				*hijo = nuevo
			}
			nodo = hijo
		}
		return nil
	})
}

// muYAML serializa leer → editar → guardar de los archivos de configuración dentro del proceso: dos ediciones a la vez
// (dos pestañas de la Web UI, un ajuste y una API) no se pisan.
var muYAML sync.Mutex

// editarYAML aplica editar a la raíz de un archivo YAML y lo guarda; si el resultado no es una configuración
// válida, no toca el archivo. El resultado se valida antes de escribir y se escribe de forma atómica: quien lo lee a
// la vez nunca ve un YAML a medias.
func editarYAML(ruta string, origen nucleo.Origen, editar func(raiz *yaml.Node) error) error {
	muYAML.Lock()
	defer muYAML.Unlock()
	original, err := os.ReadFile(ruta)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var doc yaml.Node
	if len(bytes.TrimSpace(original)) > 0 {
		if err := yaml.Unmarshal(original, &doc); err != nil {
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: YAML no válido: %s", ruta, err.Error()))
		}
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	raiz := doc.Content[0]
	if raiz.Kind != yaml.MappingNode {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s: la raíz debe ser un mapa", ruta))
	}
	if err := editar(raiz); err != nil {
		return err
	}
	salida, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	if _, err := nucleo.CargarCapaDe(salida, ruta, origen); err != nil {
		return err
	}
	return escribirAtomico(ruta, salida, 0o600)
}
