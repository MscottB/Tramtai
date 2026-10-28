package servicio

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"tramtai/internal/agentes"
	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/salidas"
)

var (
	contratosValidos = map[string]bool{"nativa": true, "openai": true, "anthropic": true}
)

// normalizarPunto completa los valores predeterminados (instancia, ruta /p/<nombre> y los tres contratos) y valida el punto.
func normalizarPunto(p ConfigPunto, instancia string, puertoServicio int) (ConfigPunto, error) {
	invalido := func(motivo string) (ConfigPunto, error) {
		return p, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("API %s: %s", p.Nombre, motivo))
	}
	if !contrato.NombrePuntoValido(p.Nombre) {
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
// modelos_permitidos. normalizarPunto solo ve el formato; existe pregunta al registro del servicio.
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
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("API %s: %s", p.Nombre, motivo))
	}
	return nil
}

// validarParaApps rechaza al guardar una API que use (como agente o en el respaldo) un agente que no se permite en apps
// (adaptadores.<nombre>.permitir_apps): sus peticiones se rechazarían todas.
func validarParaApps(p ConfigPunto) error {
	c, _ := configuracion.CargarConfiguracion("", agentes.Internos()) // sin configuración, solo cuenta la ficha de cada agente
	usados := []string{p.Agente}
	for _, e := range p.Respaldo {
		agente, _, _ := nucleo.DividirRespaldo(e)
		usados = append(usados, agente)
	}
	for _, agente := range usados {
		if agente != "" && !agentes.PermitidoEnApps(c, agente) {
			return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("API %s: %s", p.Nombre,
				idioma.Tf("%s no está habilitado para aplicaciones: todavía no ha pasado su prueba de seguridad. Úsalo desde Probar o la terminal", agentes.NombreVisible(agente))))
		}
	}
	return nil
}

// existe dice si el agente está en el registro del servicio.
func (s *Servicio) existe(agente string) bool { return s.opc.Registro[agente] != nil }

// validarPunto aplica las reglas para guardar una API: un respaldo con el que funcione y solo agentes habilitados para
// apps. El formato lo comprueba normalizarPunto.
func validarPunto(p ConfigPunto, existe func(string) bool) error {
	if err := validarRespaldo(p, existe); err != nil {
		return err
	}
	return validarParaApps(p)
}

// eliminarPunto quita un punto de acceso del archivo y borra su clave y su consumo, antes de recargar: otro con el mismo
// nombre no hereda ni la clave ni el presupuesto gastado. El consumo vive en la base de su espacio: si no está activo,
// se queda.
func (s *Servicio) eliminarPunto(ruta string, origen configuracion.Origen, nombre string) error {
	err := configuracion.EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
		i := slices.IndexFunc(lista, func(p ConfigPunto) bool { return p.Nombre == nombre })
		if i < 0 {
			return nil, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no existe la API %s en este archivo", nombre))
		}
		return slices.Delete(lista, i, i+1), nil
	})
	if err != nil {
		return err
	}
	EliminarTokenPunto(s.opc.Datos, nombre)
	s.mu.RLock()
	p := s.puntos[nombre]
	s.mu.RUnlock()
	if p != nil && p.instancia != nil {
		if err := p.instancia.Planificador.EliminarConsumo(context.Background(), nombre); err != nil {
			fmt.Fprintf(s.opc.Registros, "API %s: no se pudo borrar su consumo: %v\n", nombre, err)
		}
	}
	return nil
}

func politicaDe(p ConfigPunto) *api.Punto {
	pol := &api.Punto{Nombre: p.Nombre, Contratos: map[string]bool{}, Agente: p.Agente, Modelo: p.Modelo,
		PerfilModelo: p.PerfilModelo, Esfuerzo: p.Esfuerzo, Instrucciones: strings.TrimSpace(p.Instrucciones), ModelosPermitidos: p.ModelosPermitidos, ForzarModelo: p.ForzarModelo, Alias: p.AliasModelos}
	for _, c := range p.Contratos {
		pol.Contratos[c] = true
	}
	if e := p.Ejecucion; e != nil {
		pol.ModoContexto, pol.FormatoSalida, pol.TiempoLimite = e.ModoContexto, e.FormatoSalida, e.TiempoLimiteSegundos
	}
	if len(p.Esquema) > 0 {
		pol.Esquema, _ = json.Marshal(p.Esquema) // ya validado en normalizarPunto
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
		puntos, err := configuracion.LeerPuntos(lista)
		if err != nil {
			errores = append(errores, origen+": "+err.Error())
			return
		}
		for _, p := range puntos {
			normalizado, err := normalizarPunto(p, instancia, s.opc.Puerto)
			if err == nil {
				err = validarRespaldo(normalizado, s.existe)
			}
			switch {
			case err != nil:
				errores = append(errores, origen+": "+nucleo.MensajeDe(err))
			case vistos[normalizado.Nombre]:
				errores = append(errores, idioma.Tf("%s: API duplicada %s", origen, normalizado.Nombre))
			default:
				vistos[normalizado.Nombre] = true
				salida = append(salida, candidato{cfg: normalizado, origen: origen})
			}
		}
	}
	rutaGlobal, _ := configuracion.RutaGlobal()
	if global, err := configuracion.CargarConfiguracion("", agentes.Internos()); err != nil {
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
		capa, err := configuracion.CargarCapa(ruta, configuracion.OrigenLocal)
		if err != nil {
			errores = append(errores, nucleo.MensajeDe(err))
			continue
		}
		lista, _ := capa.Valores["puntos_acceso"].([]any)
		agregar(lista, inst.ID, ruta)
	}
	return salida, errores
}

// leerHuellaPunto es LeerHuellaPunto; las pruebas la sustituyen para simular una lectura lenta.
var leerHuellaPunto = LeerHuellaPunto

// Recargar aplica la configuración de puntos de acceso sin reiniciar: abre los nuevos, cierra los retirados y
// deja intactos los que no cambiaron, sin afectar a los trabajos en curso. Leer las claves y la configuración y abrir
// los oyentes va fuera de s.mu: un Recargar lento no frena las peticiones de los demás espacios. muRecargar excluye a
// otro Recargar, el único que cambia s.puntos (salvo Cerrar).
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
	s.mu.RLock()
	actuales, instancias := maps.Clone(s.puntos), maps.Clone(s.instancias)
	s.mu.RUnlock()
	nuevos := map[string]*puntoActivo{}
	for _, c := range deseados {
		huella, err := leerHuellaPunto(RutaTokenPunto(s.opc.Datos, c.cfg.Nombre))
		if err != nil {
			errores = append(errores, err.Error())
			continue
		}
		firma := firmaPunto(c.cfg, huella)
		inst, actual := instancias[c.cfg.Instancia], actuales[c.cfg.Nombre]
		if actual != nil && actual.firma == firma && actual.err == "" && actual.instancia == inst {
			nuevos[c.cfg.Nombre] = actual
			continue
		}
		if actual != nil {
			actual.soltar() // libera el puerto por si el punto nuevo usa el mismo
			retirados = append(retirados, actual)
		}
		nuevos[c.cfg.Nombre] = s.abrirPunto(c, huella, firma, inst)
	}
	// Un punto que sale de la configuración se cierra; su consumo se queda. Solo eliminarlo (EliminarPunto) lo borra:
	// que falte su clave (tokens/ borrado a mano) no lo da por eliminado.
	for nombre, actual := range actuales {
		if _, sigue := nuevos[nombre]; !sigue {
			retirados = append(retirados, actual)
		}
	}
	limites := map[string]map[string]int{}
	presupuestos := map[string]map[string]nucleo.Presupuesto{}
	for _, c := range deseados {
		if c.cfg.Ejecucion != nil && c.cfg.Ejecucion.MaximoTrabajos > 0 {
			if limites[c.cfg.Instancia] == nil {
				limites[c.cfg.Instancia] = map[string]int{}
			}
			limites[c.cfg.Instancia][c.cfg.Nombre] = c.cfg.Ejecucion.MaximoTrabajos
		}
		if c.cfg.Presupuesto != nil {
			if presupuestos[c.cfg.Instancia] == nil {
				presupuestos[c.cfg.Instancia] = map[string]nucleo.Presupuesto{}
			}
			presupuestos[c.cfg.Instancia][c.cfg.Nombre] = *c.cfg.Presupuesto
		}
	}
	for id, inst := range instancias {
		inst.Planificador.LimitarPuntos(limites[id])
		m := presupuestos[id]
		if m == nil {
			m = map[string]nucleo.Presupuesto{}
		}
		m[persistencia.TodaLaInstancia] = presupuestoInstancia(inst)
		inst.Planificador.FijarPresupuestos(m)
	}
	s.mu.Lock()
	if s.cerrado { // Cerrar ya se llevó los puntos: los que se abrieron aquí se cierran también
		s.mu.Unlock()
		for _, p := range nuevos {
			retirados = append(retirados, p)
		}
		return nil
	}
	s.puntos, s.erroresPuntos = nuevos, errores
	estado := s.estadoPuntos()
	s.mu.Unlock()
	for _, p := range estado {
		if p.Error != "" && p.Error != "desactivado" {
			fmt.Fprintf(s.opc.Registros, "API %s: %s\n", p.Nombre, p.Error)
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
	if c, err := configuracion.CargarConfiguracion(raiz, agentes.Internos()); err == nil {
		return configuracion.PresupuestoDe(c)
	}
	return configuracion.PresupuestoDe(inst.Config)
}

func firmaPunto(cfg ConfigPunto, huella string) string {
	datos, _ := json.Marshal(cfg)
	return string(datos) + "|" + huella
}

// abrirPunto prepara el punto; si tiene puerto propio y está ocupado no se activa ni se elige otro. huella es la de
// su clave: el servicio nunca conoce la clave de un punto. inst es el espacio del punto (nil si no está activo).
func (s *Servicio) abrirPunto(c candidato, huella, firma string, inst *Instancia) *puntoActivo {
	p := &puntoActivo{cfg: c.cfg, origen: c.origen, firma: firma, instancia: inst}
	switch {
	case !c.cfg.EstaActivo():
		p.err = "desactivado"
		return p
	case p.instancia == nil:
		p.err = idioma.Tf("el espacio %s no está activo", c.cfg.Instancia)
		return p
	}
	srv := &api.Servidor{Planificador: inst.Planificador, Cargas: inst.Cargas, Registro: s.opc.Registro, Config: inst.Config, Punto: politicaDe(c.cfg),
		Conversor: s.conversor}
	handler := s.frenarClaves(api.Proteger(s.opc.Hosts, func(r *http.Request) bool {
		valida := api.HuellaValida(r, huella)
		if !valida {
			s.claveFalsa(r)
		}
		return valida
	}, srv.Rutas()))
	if c.cfg.Puerto == 0 {
		p.handler, p.url = handler, s.BasePublica()+c.cfg.RutaBase
		return p
	}
	direccion := net.JoinHostPort(s.opc.Host, strconv.Itoa(c.cfg.Puerto))
	oyente, err := net.Listen("tcp", direccion)
	if err != nil {
		p.err = idioma.Tf("no se pudo abrir %s (¿puerto en uso?)", direccion)
		return p
	}
	if s.opc.TLS != nil { // las claves Bearer de las apps no viajan en claro por el puerto propio del punto
		oyente = tls.NewListener(oyente, s.opc.TLS)
		handler = s.conHSTS(handler)
	}
	p.handler, p.url, p.oyente = handler, s.urlPublica(c.cfg.Puerto), oyente
	p.srv = &http.Server{Handler: s.marcarProxy(handler), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute, MaxHeaderBytes: 64 << 10,
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
			ModelosPermitidos: p.cfg.ModelosPermitidos, ForzarModelo: p.cfg.ForzarModelo, Esquema: p.cfg.Esquema, Respaldo: p.cfg.Respaldo, ReintentosSalida: p.cfg.ReintentosSalida, Presupuesto: p.cfg.Presupuesto,
			Activo: p.err == "", Error: p.err, Origen: p.origen})
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Nombre < lista[j].Nombre })
	return lista
}
