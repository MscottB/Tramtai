package servicio

import (
	"bytes"
	"context"
	"encoding/json"
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
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"tramtai/internal/api"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// ConfigPunto es un punto de acceso tal como se escribe en la configuración global o local (sección 5.4).
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
	Activo            *bool             `json:"activo,omitempty" yaml:"activo,omitempty"`
	Ejecucion         *EjecucionPunto   `json:"ejecucion,omitempty" yaml:"ejecucion,omitempty"`
}

// EjecucionPunto son los valores de ejecución propios del punto, dentro de los límites de la instancia.
type EjecucionPunto struct {
	ModoContexto         string `json:"modo_contexto,omitempty" yaml:"modo_contexto,omitempty"`
	FormatoSalida        string `json:"formato_salida,omitempty" yaml:"formato_salida,omitempty"`
	TiempoLimiteSegundos int    `json:"tiempo_limite_segundos,omitempty" yaml:"tiempo_limite_segundos,omitempty"`
	// MaximoTrabajos limita las ejecuciones simultáneas del punto; no amplía las de su instancia.
	MaximoTrabajos int `json:"maximo_trabajos,omitempty" yaml:"maximo_trabajos,omitempty"`
}

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
	if p.Ejecucion != nil && p.Ejecucion.MaximoTrabajos < 0 {
		return invalido(idioma.T("ejecucion.maximo_trabajos no puede ser negativo"))
	}
	return p, nil
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
	url       string
	err       string
}

func (p *puntoActivo) cerrar() {
	if p.srv == nil {
		return
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	if p.srv.Shutdown(ctx) != nil {
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
	Esfuerzo      string   `json:"esfuerzo,omitempty"`
	Activo        bool     `json:"activo"`
	Error         string   `json:"error,omitempty"`
	Origen        string   `json:"origen"`
}

// candidato es un punto leído de la configuración con el archivo del que procede.
type candidato struct {
	cfg    ConfigPunto
	origen string
}

// puntosDeseados lee los puntos de la configuración global y de la local de cada proyecto activo; nunca de la
// versionada, que la configuración ya rechaza (sección 5.4).
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
// deja intactos los que no cambiaron, sin afectar a los trabajos en curso (sección 5.4).
func (s *Servicio) Recargar() []EstadoPunto {
	deseados, errores := s.puntosDeseados()
	s.mu.Lock()
	nuevos := map[string]*puntoActivo{}
	for _, c := range deseados {
		token, err := LeerOCrearToken(RutaTokenPunto(s.opc.Datos, c.cfg.Nombre))
		if err != nil {
			errores = append(errores, err.Error())
			continue
		}
		firma := firmaPunto(c.cfg, token)
		actual := s.puntos[c.cfg.Nombre]
		if actual != nil && actual.firma == firma && actual.err == "" && actual.instancia == s.instancias[c.cfg.Instancia] {
			nuevos[c.cfg.Nombre] = actual
			continue
		}
		if actual != nil {
			actual.cerrar()
		}
		nuevos[c.cfg.Nombre] = s.abrirPunto(c, token, firma)
	}
	for nombre, actual := range s.puntos {
		if _, sigue := nuevos[nombre]; !sigue {
			actual.cerrar()
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
	for id, inst := range s.instancias {
		inst.Planificador.LimitarPuntos(limites[id])
	}
	estado := s.estadoPuntos()
	s.mu.Unlock()
	for _, p := range estado {
		if p.Error != "" && p.Error != "desactivado" {
			fmt.Fprintf(s.opc.Registros, "punto de acceso %s: %s\n", p.Nombre, p.Error)
		}
	}
	return estado
}

func firmaPunto(cfg ConfigPunto, token string) string {
	datos, _ := json.Marshal(cfg)
	return string(datos) + "|" + token
}

// abrirPunto prepara el punto; si tiene puerto propio y está ocupado no se activa ni se elige otro (sección 5.4).
// Se llama con s.mu tomado.
func (s *Servicio) abrirPunto(c candidato, token, firma string) *puntoActivo {
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
	handler := api.Proteger(s.opc.Hosts, func(r *http.Request) bool { return api.TokenValido(r, token) }, srv.Rutas())
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
	p.handler, p.url = handler, "http://"+direccion
	p.srv = &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	go p.srv.Serve(oyente)
	return p
}

// estadoPuntos se llama con s.mu tomado (lectura o escritura).
func (s *Servicio) estadoPuntos() []EstadoPunto {
	lista := []EstadoPunto{}
	for _, p := range s.puntos {
		lista = append(lista, EstadoPunto{Nombre: p.cfg.Nombre, Grupo: p.cfg.Grupo, Instrucciones: p.cfg.Instrucciones, Instancia: p.cfg.Instancia, URL: p.url, Contratos: p.cfg.Contratos,
			Agente: p.cfg.Agente, Modelo: p.cfg.Modelo, Esfuerzo: p.cfg.Esfuerzo, Activo: p.err == "", Error: p.err, Origen: p.origen})
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

// editarYAML aplica editar a la raíz de un archivo YAML y lo guarda; si el resultado no es una configuración
// válida, deja el archivo como estaba.
func editarYAML(ruta string, origen nucleo.Origen, editar func(raiz *yaml.Node) error) error {
	original, err := os.ReadFile(ruta)
	existia := err == nil
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
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(ruta, salida, 0o600); err != nil {
		return err
	}
	if _, err := nucleo.CargarCapa(ruta, origen); err != nil {
		if existia {
			_ = os.WriteFile(ruta, original, 0o600)
		} else {
			_ = os.Remove(ruta)
		}
		return err
	}
	return nil
}
