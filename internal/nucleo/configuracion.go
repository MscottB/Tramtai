package nucleo

import (
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"go.yaml.in/yaml/v3"
)

// Origen identifica la capa de la que procede un valor (sección 12.2): a mayor valor, mayor prioridad.
type Origen int

const (
	OrigenInterno Origen = iota + 1
	OrigenGlobal
	OrigenProyecto
	OrigenLocal
	OrigenEntorno
	OrigenPuntoAcceso
)

func (o Origen) String() string {
	switch o {
	case OrigenInterno:
		return "interno"
	case OrigenGlobal:
		return "global"
	case OrigenProyecto:
		return "proyecto"
	case OrigenLocal:
		return "local"
	case OrigenEntorno:
		return "entorno"
	case OrigenPuntoAcceso:
		return "punto_acceso"
	}
	return "desconocido"
}

type tipoValor int

const (
	tipoTexto tipoValor = iota
	tipoEntero
	tipoBooleano
	tipoLista
)

// esquema enumera las claves nativas admitidas; un segmento "*" acepta cualquier nombre.
// shortcut: cubre las claves de M1; compatibilidad (M4) y puntos de acceso (M5) detallarán sus subclaves.
var esquema = map[string]tipoValor{
	"version":                                     tipoEntero,
	"proyecto.nombre":                             tipoTexto,
	"instancia.ambito":                            tipoTexto,
	"servicio.host":                               tipoTexto,
	"servicio.puerto":                             tipoEntero,
	"servicio.puerto_automatico":                  tipoBooleano,
	"servicio.plazo_drenaje_segundos":             tipoEntero,
	"servicio.entrada_local":                      tipoBooleano,
	"servicio.requerir_token":                     tipoBooleano,
	"servicio.permitir_remoto":                    tipoBooleano,
	"instancia_global.token_variable_entorno":     tipoTexto,
	"instancia_global.raices_permitidas":          tipoLista,
	"proyectos_confiables":                        tipoLista,
	"comandos_agentes.*":                          tipoTexto,
	"adaptadores.*.autenticacion":                 tipoTexto,
	"concurrencia_servicio.adaptadores.*":         tipoEntero,
	"puntos_acceso":                               tipoLista,
	"predeterminados.agente":                      tipoTexto,
	"predeterminados.perfil_modelo":               tipoTexto,
	"predeterminados.modelo":                      tipoTexto,
	"servicio.registro_mb":                        tipoEntero,
	"servicio.registro_copias":                    tipoEntero,
	"predeterminados.modo_contexto":               tipoTexto,
	"predeterminados.formato_salida":              tipoTexto,
	"concurrencia.maximo_trabajos_instancia":      tipoEntero,
	"concurrencia.maximo_conversiones_instancia":  tipoEntero,
	"concurrencia.maximo_tamano_cola":             tipoEntero,
	"concurrencia.tiempo_maximo_en_cola_segundos": tipoEntero,
	"retencion.modo":                              tipoTexto,
	"retencion.guardar_historial":                 tipoBooleano,
	"retencion.horas_resultado":                   tipoEntero,
	"retencion.horas_metadatos":                   tipoEntero,
	"retencion.horas_idempotencia":                tipoEntero,
	"retencion.horas_carga_huerfana":              tipoEntero,
	"retencion.cache_conversion_habilitada":       tipoBooleano,
	"retencion.eliminar_entradas_al_terminar":     tipoBooleano,
	"retencion.cifrado_aplicacion":                tipoBooleano,
	"conversion.motor":                            tipoTexto,
	"conversion.habilitar_plugins":                tipoBooleano,
	"conversion.transformacion_semantica":         tipoBooleano,
	"conversion.comando":                          tipoTexto,
	"conversion.tiempo_limite_segundos":           tipoEntero,
	"conversion.maximo_carga_mb":                  tipoEntero,
	"conversion.maximo_salida_mb":                 tipoEntero,
	"conversion.maximo_archivos_zip":              tipoEntero,
	"conversion.maximo_descomprimido_mb":          tipoEntero,
	"conversion.profundidad_maxima_zip":           tipoEntero,
	"compatibilidad.modo_tolerante":               tipoBooleano,
	"compatibilidad.cancelar_al_desconectar":      tipoBooleano,
	"modelos.perfiles.*.*":                        tipoTexto,
	"modelos.fallback.habilitado":                 tipoBooleano,
}

// prefijosAdministrados son límites de seguridad: nunca pueden venir de la configuración versionada del repositorio.
var prefijosAdministrados = []string{"servicio", "instancia_global", "proyectos_confiables", "comandos_agentes", "concurrencia_servicio", "puntos_acceso", "adaptadores", "conversion.comando"}

// Capa es un origen de configuración aplanado a claves con puntos.
type Capa struct {
	Origen  Origen
	Fuente  string
	Valores map[string]any
}

// CargarCapa lee y valida un archivo YAML; si no existe devuelve una capa vacía.
func CargarCapa(ruta string, origen Origen) (Capa, error) {
	capa := Capa{Origen: origen, Fuente: ruta, Valores: map[string]any{}}
	datos, err := os.ReadFile(ruta)
	if os.IsNotExist(err) {
		return capa, nil
	}
	if err != nil {
		return capa, errorConfiguracion(ruta, "", err.Error())
	}
	var crudo map[string]any
	if err := yaml.Unmarshal(datos, &crudo); err != nil {
		return capa, errorConfiguracion(ruta, "", "YAML no válido: "+err.Error())
	}
	aplanar("", crudo, capa.Valores)
	return capa, capa.validar()
}

// CapaEntorno lee variables TRAMTAI_<SECCION>__<CLAVE>, donde "__" separa niveles.
// Las TRAMTAI_* sin "__" (por ejemplo, las que guardan tokens) se ignoran.
func CapaEntorno(entorno []string) (Capa, error) {
	capa := Capa{Origen: OrigenEntorno, Fuente: "entorno", Valores: map[string]any{}}
	for _, par := range entorno {
		nombre, valor, _ := strings.Cut(par, "=")
		if !strings.HasPrefix(strings.ToUpper(nombre), "TRAMTAI_") || !strings.Contains(nombre, "__") {
			continue
		}
		clave := strings.ToLower(strings.ReplaceAll(nombre[len("TRAMTAI_"):], "__", "."))
		tipo, conocida := tipoDe(clave)
		if !conocida {
			return capa, errorConfiguracion("entorno", nombre, "clave desconocida")
		}
		convertido, err := convertir(tipo, valor)
		if err != nil {
			return capa, errorConfiguracion("entorno", nombre, "tipo incorrecto")
		}
		capa.Valores[clave] = convertido
	}
	return capa, nil
}

// Internos son los valores predeterminados de diseño (prioridad 1).
func Internos() Capa {
	return Capa{Origen: OrigenInterno, Fuente: "interno", Valores: map[string]any{
		"servicio.host":                                 "127.0.0.1",
		"servicio.puerto":                               17373,
		"servicio.puerto_automatico":                    true,
		"servicio.plazo_drenaje_segundos":               30,
		"servicio.entrada_local":                        true,
		"servicio.requerir_token":                       true,
		"servicio.permitir_remoto":                      false,
		"instancia_global.token_variable_entorno":       "TRAMTAI_TOKEN_GLOBAL",
		"compatibilidad.modo_tolerante":                 false,
		"compatibilidad.cancelar_al_desconectar":        true,
		"comandos_agentes.claude":                       "claude",
		"comandos_agentes.codex":                        "codex",
		"comandos_agentes.antigravity":                  "agy",
		"adaptadores.antigravity.autenticacion":         "clave_api",
		"concurrencia_servicio.adaptadores.gemini":      2,
		"concurrencia_servicio.adaptadores.antigravity": 1,
		"modelos.perfiles.rapido.gemini":                "gemini-flash-lite-latest",
		"modelos.perfiles.equilibrado.gemini":           "gemini-flash-latest",
		"modelos.perfiles.profundo.gemini":              "gemini-pro-latest",
		"adaptadores.claude.autenticacion":              "clave_api",
		"adaptadores.codex.autenticacion":               "clave_api",
		"concurrencia_servicio.adaptadores.claude":      1,
		"concurrencia_servicio.adaptadores.codex":       1,
		"predeterminados.agente":                        "claude",
		"servicio.registro_mb":                          5,
		"servicio.registro_copias":                      2,
		"modelos.perfiles.rapido.claude":                "haiku",
		"modelos.perfiles.equilibrado.claude":           "sonnet",
		"modelos.perfiles.profundo.claude":              "opus",
		"predeterminados.modo_contexto":                 "aislado",
		"predeterminados.formato_salida":                "markdown",
		"concurrencia.maximo_trabajos_instancia":        2,
		"concurrencia.maximo_conversiones_instancia":    1,
		"concurrencia.maximo_tamano_cola":               20,
		"concurrencia.tiempo_maximo_en_cola_segundos":   1800,
		"retencion.modo":                                "temporal",
		"retencion.guardar_historial":                   false,
		"retencion.horas_resultado":                     24,
		"retencion.horas_metadatos":                     168,
		"retencion.horas_idempotencia":                  24,
		"retencion.horas_carga_huerfana":                24,
		"retencion.cache_conversion_habilitada":         false,
		"retencion.eliminar_entradas_al_terminar":       false,
		"retencion.cifrado_aplicacion":                  false,
		"conversion.motor":                              "markitdown",
		"conversion.habilitar_plugins":                  false,
		"conversion.transformacion_semantica":           false,
		"conversion.comando":                            "markitdown",
		"conversion.tiempo_limite_segundos":             120,
		"conversion.maximo_carga_mb":                    50,
		"conversion.maximo_salida_mb":                   20,
		"conversion.maximo_archivos_zip":                50,
		"conversion.maximo_descomprimido_mb":            200,
		"conversion.profundidad_maxima_zip":             3,
	}}
}

// Configuracion es la vista efectiva, con el origen de cada valor.
type Configuracion struct {
	// estado se reemplaza entero al cambiar un valor en marcha (Fijar): los lectores nunca ven uno a medias.
	estado atomic.Pointer[estadoConfiguracion]
	// perfiles son las carpetas de perfiles de ejecución, de mayor a menor prioridad (AplicarPerfil).
	perfiles []carpetaPerfiles
}

type estadoConfiguracion struct {
	valores  map[string]any
	origenes map[string]Origen
}

func (c *Configuracion) e() *estadoConfiguracion { return c.estado.Load() }

// Fijar cambia valores con el servicio en marcha, como si la capa origen los tuviera. Un valor de una capa con
// más prioridad (por ejemplo, una variable de entorno) se respeta, igual que al cargar los archivos.
func (c *Configuracion) Fijar(origen Origen, valores map[string]any) {
	actual := c.e()
	nuevo := &estadoConfiguracion{valores: maps.Clone(actual.valores), origenes: maps.Clone(actual.origenes)}
	for clave, valor := range valores {
		if nuevo.origenes[clave] <= origen {
			nuevo.valores[clave], nuevo.origenes[clave] = valor, origen
		}
	}
	c.estado.Store(nuevo)
}

// Resolver aplica la precedencia única de la sección 12.2: gana la capa de mayor prioridad,
// sin depender del orden de los argumentos.
func Resolver(capas ...Capa) *Configuracion {
	ordenadas := append([]Capa{Internos()}, capas...)
	sort.SliceStable(ordenadas, func(i, j int) bool { return ordenadas[i].Origen < ordenadas[j].Origen })
	e := &estadoConfiguracion{valores: map[string]any{}, origenes: map[string]Origen{}}
	for _, capa := range ordenadas {
		for clave, valor := range capa.Valores {
			e.valores[clave], e.origenes[clave] = valor, capa.Origen
		}
	}
	c := &Configuracion{}
	c.estado.Store(e)
	return c
}

// Texto devuelve un valor de texto, o "" si no existe.
func (c *Configuracion) Texto(clave string) string {
	v, _ := c.e().valores[clave].(string)
	return v
}

// Entero devuelve un valor entero, o 0 si no existe.
func (c *Configuracion) Entero(clave string) int {
	v, _ := c.e().valores[clave].(int)
	return v
}

// Booleano devuelve un valor booleano, o false si no existe.
func (c *Configuracion) Booleano(clave string) bool {
	v, _ := c.e().valores[clave].(bool)
	return v
}

// Perfiles devuelve modelos.perfiles como perfil -> agente -> identificador de modelo (sección 9.2).
func (c *Configuracion) Perfiles() map[string]map[string]string {
	perfiles := map[string]map[string]string{}
	for clave, valor := range c.e().valores {
		resto, esPerfil := strings.CutPrefix(clave, "modelos.perfiles.")
		perfil, agente, completo := strings.Cut(resto, ".")
		modelo, esTexto := valor.(string)
		if !esPerfil || !completo || !esTexto {
			continue
		}
		if perfiles[perfil] == nil {
			perfiles[perfil] = map[string]string{}
		}
		perfiles[perfil][agente] = modelo
	}
	return perfiles
}

// Origen devuelve la capa que aportó el valor efectivo, o 0 si la clave no tiene valor.
func (c *Configuracion) Origen(clave string) Origen { return c.e().origenes[clave] }

// EntradaEfectiva es una fila de "tramtai configuracion mostrar --efectiva".
type EntradaEfectiva struct {
	Clave  string
	Valor  any
	Origen Origen
}

// Efectiva devuelve todos los valores ordenados por clave, con su origen.
func (c *Configuracion) Efectiva() []EntradaEfectiva {
	e := c.e()
	entradas := make([]EntradaEfectiva, 0, len(e.valores))
	for clave, valor := range e.valores {
		entradas = append(entradas, EntradaEfectiva{Clave: clave, Valor: valor, Origen: e.origenes[clave]})
	}
	sort.Slice(entradas, func(i, j int) bool { return entradas[i].Clave < entradas[j].Clave })
	return entradas
}

// AplicarPredeterminados completa lo que la solicitud omite; sus campos explícitos prevalecen.
func (c *Configuracion) AplicarPredeterminados(s SolicitudEjecucion) SolicitudEjecucion {
	if s.Agente == "" {
		s.Agente = c.Texto("predeterminados.agente")
		// El modelo predeterminado va con su agente: si la solicitud eligió otro agente, usa el modelo de ese CLI.
		if s.Modelo == "" && s.PerfilModelo == "" {
			s.Modelo = c.Texto("predeterminados.modelo")
		}
	}
	if s.Modelo == "" && s.PerfilModelo == "" {
		s.PerfilModelo = c.Texto("predeterminados.perfil_modelo")
	}
	if s.ModoContexto == "" {
		s.ModoContexto = c.Texto("predeterminados.modo_contexto")
	}
	if s.Salida.Formato == "" && len(s.Salida.Esquema) > 0 {
		s.Salida.Formato = "json"
	}
	if s.Salida.Formato == "" {
		s.Salida.Formato = c.Texto("predeterminados.formato_salida")
	}
	return s
}

func (c Capa) validar() error {
	claves := make([]string, 0, len(c.Valores))
	for clave := range c.Valores {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	for _, clave := range claves {
		tipo, conocida := tipoDe(clave)
		if !conocida {
			return errorConfiguracion(c.Fuente, clave, "clave desconocida")
		}
		if c.Origen == OrigenProyecto && esAdministrada(clave) {
			return errorConfiguracion(c.Fuente, clave, "clave de seguridad no admitida en la configuración versionada; defínela en la local o en la global")
		}
		if !tipoCorrecto(tipo, c.Valores[clave]) {
			return errorConfiguracion(c.Fuente, clave, "tipo incorrecto")
		}
	}
	// Promesas que Tramtai aún no cumple: se rechazan en vez de aceptarlas sin efecto (sección 16.2).
	for _, clave := range []string{"retencion.guardar_historial", "retencion.cifrado_aplicacion", "retencion.cache_conversion_habilitada"} {
		if c.Valores[clave] == true {
			return errorConfiguracion(c.Fuente, clave, "no soportado todavía: no hay historial permanente, cifrado propio ni cache documental")
		}
	}
	if modo, existe := c.Valores["retencion.modo"]; existe && modo != "temporal" {
		return errorConfiguracion(c.Fuente, "retencion.modo", "solo se admite temporal")
	}
	if version, existe := c.Valores["version"]; existe && version != 1 {
		return errorConfiguracion(c.Fuente, "version", "versión de formato no soportada")
	}
	return nil
}

func aplanar(prefijo string, origen map[string]any, destino map[string]any) {
	for clave, valor := range origen {
		if prefijo != "" {
			clave = prefijo + "." + clave
		}
		if hijo, esMapa := valor.(map[string]any); esMapa {
			aplanar(clave, hijo, destino)
			continue
		}
		destino[clave] = valor
	}
}

func tipoDe(clave string) (tipoValor, bool) {
	if tipo, existe := esquema[clave]; existe {
		return tipo, true
	}
	partes := strings.Split(clave, ".")
	for patron, tipo := range esquema {
		segmentos := strings.Split(patron, ".")
		if len(segmentos) != len(partes) {
			continue
		}
		coincide := true
		for i, segmento := range segmentos {
			if segmento != "*" && segmento != partes[i] {
				coincide = false
				break
			}
		}
		if coincide {
			return tipo, true
		}
	}
	return 0, false
}

func esAdministrada(clave string) bool {
	for _, prefijo := range prefijosAdministrados {
		if clave == prefijo || strings.HasPrefix(clave, prefijo+".") {
			return true
		}
	}
	return false
}

func tipoCorrecto(tipo tipoValor, valor any) bool {
	var correcto bool
	switch tipo {
	case tipoTexto:
		_, correcto = valor.(string)
	case tipoEntero:
		_, correcto = valor.(int)
	case tipoBooleano:
		_, correcto = valor.(bool)
	case tipoLista:
		_, correcto = valor.([]any)
	}
	return correcto
}

func convertir(tipo tipoValor, texto string) (any, error) {
	switch tipo {
	case tipoEntero:
		return strconv.Atoi(texto)
	case tipoBooleano:
		return strconv.ParseBool(texto)
	case tipoLista:
		partes := strings.Split(texto, ",")
		lista := make([]any, len(partes))
		for i, parte := range partes {
			lista[i] = strings.TrimSpace(parte)
		}
		return lista, nil
	}
	return texto, nil
}

// errorConfiguracion indica archivo, propiedad y motivo sin revelar valores (sección 12.3).
func errorConfiguracion(fuente, clave, motivo string) error {
	if clave == "" {
		return NuevoError(CodigoConfiguracionInvalida, fuente+": "+motivo)
	}
	return NuevoError(CodigoConfiguracionInvalida, fmt.Sprintf("%s: %s: %s", fuente, clave, motivo))
}
