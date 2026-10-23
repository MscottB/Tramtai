// Package configuracion carga la configuración por capas (interna, global, proyecto, local y entorno) con su esquema
// y edita los archivos YAML (config.yaml y local.yaml) conservando el resto del archivo y sus comentarios: cada cambio
// se valida con el esquema antes de escribirse de forma atómica.
package configuracion

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"go.yaml.in/yaml/v3"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Origen identifica la capa de la que procede un valor: a mayor valor, mayor prioridad.
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
	tipoDecimal // entero o decimal (YAML da int para 50 y float64 para 12.5)
)

// esquema enumera las claves nativas admitidas; un segmento "*" acepta cualquier nombre.
// shortcut: valida las claves conocidas; las subclaves de compatibilidad y de puntos de acceso se validan en sus paquetes.
var esquema = map[string]tipoValor{
	"version":                                     tipoEntero,
	"proyecto.nombre":                             tipoTexto,
	"instancia.ambito":                            tipoTexto, // obsoleta: no hace nada; se acepta para no romper configuraciones antiguas
	"servicio.host":                               tipoTexto,
	"servicio.puerto":                             tipoEntero,
	"servicio.puerto_automatico":                  tipoBooleano,
	"servicio.plazo_drenaje_segundos":             tipoEntero,
	"servicio.entrada_local":                      tipoBooleano,
	"servicio.buscar_actualizaciones":             tipoBooleano,
	"servicio.requerir_token":                     tipoBooleano,
	"servicio.permitir_remoto":                    tipoBooleano,
	"instancia_global.token_variable_entorno":     tipoTexto,
	"instancia_global.raices_permitidas":          tipoLista,
	"proyectos_confiables":                        tipoLista,
	"comandos_agentes.*":                          tipoTexto,
	"adaptadores.*.autenticacion":                 tipoTexto,
	"adaptadores.*.desconectado":                  tipoBooleano,
	"adaptadores.*.permitir_apps":                 tipoBooleano,
	"concurrencia_servicio.adaptadores.*":         tipoEntero,
	"puntos_acceso":                               tipoLista,
	"predeterminados.agente":                      tipoTexto,
	"predeterminados.perfil_modelo":               tipoTexto,
	"predeterminados.modelo":                      tipoTexto,
	"predeterminados.esfuerzo":                    tipoTexto,
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
	"retencion.max_trabajos":                      tipoEntero,
	"retencion.horas_idempotencia":                tipoEntero,
	"retencion.horas_carga_huerfana":              tipoEntero,
	"retencion.cache_conversion_habilitada":       tipoBooleano,
	"retencion.eliminar_entradas_al_terminar":     tipoBooleano,
	"retencion.cifrado_aplicacion":                tipoBooleano,
	"limites.contexto_tokens":                     tipoLista,
	"limites.reserva_salida_tokens":               tipoEntero,
	"limites.memoria_proceso_mb":                  tipoEntero,
	"conversion.motor":                            tipoTexto,
	"conversion.habilitar_plugins":                tipoBooleano,
	"conversion.preparar_al_iniciar":              tipoBooleano,
	"conversion.transformacion_semantica":         tipoBooleano,
	"conversion.comando":                          tipoTexto,
	"conversion.tiempo_limite_segundos":           tipoEntero,
	"conversion.tiempo_total_segundos":            tipoEntero,
	"conversion.cuota_cargas_mb":                  tipoEntero,
	"conversion.maximo_carga_mb":                  tipoEntero,
	"conversion.maximo_salida_mb":                 tipoEntero,
	"conversion.maximo_archivos_zip":              tipoEntero, // retirada en la v0.50.2 (sin ZIP): se acepta y no hace nada
	"conversion.maximo_descomprimido_mb":          tipoEntero,
	"conversion.profundidad_maxima_zip":           tipoEntero, // retirada en la v0.50.2
	"compatibilidad.modo_tolerante":               tipoBooleano,
	"compatibilidad.cancelar_al_desconectar":      tipoBooleano,
	"modelos.perfiles.*.*":                        tipoTexto,
	"modelos.fallback.habilitado":                 tipoBooleano,
	"presupuesto.diario.peticiones":               tipoEntero,
	"presupuesto.diario.tokens":                   tipoEntero,
	"presupuesto.diario.usd":                      tipoDecimal,
	"presupuesto.mensual.peticiones":              tipoEntero,
	"presupuesto.mensual.tokens":                  tipoEntero,
	"presupuesto.mensual.usd":                     tipoDecimal,
}

// prefijosAdministrados son límites de seguridad: nunca pueden venir de la configuración versionada del repositorio,
// y en local.yaml solo si la carpeta del proyecto es confiable según la configuración global (ver validar).
var prefijosAdministrados = []string{"servicio", "instancia_global", "proyectos_confiables", "comandos_agentes", "concurrencia_servicio", "puntos_acceso", "adaptadores", "conversion", "presupuesto", "limites.memoria_proceso_mb"}

// Capa es un origen de configuración aplanado a claves con puntos.
type Capa struct {
	Origen  Origen
	Fuente  string
	Valores map[string]any
	// Ignoradas son las claves de seguridad de un local.yaml de una carpeta no confiable: al cargar no se aplican.
	Ignoradas []string
}

// CargarCapa lee y valida un archivo YAML; si no existe devuelve una capa vacía. Las claves de seguridad de un
// local.yaml no confiable se ignoran (Ignoradas): el proyecto se abre igual, sin ellas.
func CargarCapa(ruta string, origen Origen) (Capa, error) {
	datos, err := os.ReadFile(ruta)
	if os.IsNotExist(err) {
		return Capa{Origen: origen, Fuente: ruta, Valores: map[string]any{}}, nil
	}
	if err != nil {
		return Capa{Origen: origen, Fuente: ruta, Valores: map[string]any{}}, errorConfiguracion(ruta, "", err.Error())
	}
	return cargarCapa(datos, ruta, origen, false)
}

// CargarCapaDe valida el contenido YAML de ruta sin leerlo del disco: así se comprueba un cambio antes de guardarlo.
// Es estricta: guardar una clave de seguridad en un local.yaml no confiable es un error.
func CargarCapaDe(datos []byte, ruta string, origen Origen) (Capa, error) {
	return cargarCapa(datos, ruta, origen, true)
}

// ValidarCambio valida el contenido nuevo de ruta antes de guardarlo, como CargarCapaDe, salvo por las claves de
// seguridad que ya estaban igual en antes: un local.yaml no confiado puede traerlas (se ignoran al aplicarlo), y
// guardar otro ajuste no debe fallar por ellas. Añadir o cambiar una sigue siendo un error.
func ValidarCambio(antes, despues []byte, ruta string, origen Origen) error {
	_, err := cargarCapa(despues, ruta, origen, true)
	if err == nil || !reflect.DeepEqual(administradas(antes), administradas(despues)) {
		return err
	}
	_, err = cargarCapa(despues, ruta, origen, false)
	return err
}

// administradas son los valores de las claves de seguridad de un YAML, aplanados.
func administradas(datos []byte) map[string]any {
	var crudo map[string]any
	_ = yaml.Unmarshal(datos, &crudo)
	valores := map[string]any{}
	aplanar("", crudo, valores)
	for clave := range valores {
		if !esAdministrada(clave) {
			delete(valores, clave)
		}
	}
	return valores
}

func cargarCapa(datos []byte, ruta string, origen Origen, estricta bool) (Capa, error) {
	capa := Capa{Origen: origen, Fuente: ruta, Valores: map[string]any{}}
	var crudo map[string]any
	if err := yaml.Unmarshal(datos, &crudo); err != nil {
		return capa, errorConfiguracion(ruta, "", idioma.Tf("YAML no válido: %s", err.Error()))
	}
	aplanar("", crudo, capa.Valores)
	err := capa.validar(estricta)
	return capa, err
}

// ClavesAdministradas devuelve las claves de seguridad (prefijosAdministrados) que define el YAML de ruta, ordenadas y
// sin repetir: lo que la app enseña antes de confiar en una carpeta. Sin archivo, nil.
func ClavesAdministradas(ruta string) ([]string, error) {
	datos, err := os.ReadFile(ruta)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var crudo map[string]any
	if err := yaml.Unmarshal(datos, &crudo); err != nil {
		return nil, errorConfiguracion(ruta, "", idioma.Tf("YAML no válido: %s", err.Error()))
	}
	valores := map[string]any{}
	aplanar("", crudo, valores)
	vistas := map[string]bool{}
	var claves []string
	for clave := range valores {
		for _, prefijo := range prefijosAdministrados {
			if (clave == prefijo || strings.HasPrefix(clave, prefijo+".")) && !vistas[prefijo] {
				vistas[prefijo] = true
				claves = append(claves, prefijo)
			}
		}
	}
	sort.Strings(claves)
	return claves, nil
}

// DescribirAdministradas dice, para quien usa la app, qué controlan unas claves de seguridad (o sus subclaves): cada
// prefijo administrado una vez, en el orden en que llega.
func DescribirAdministradas(claves []string) []string {
	vistos := map[string]bool{}
	var nombres []string
	for _, clave := range claves {
		for _, prefijo := range prefijosAdministrados {
			if (clave == prefijo || strings.HasPrefix(clave, prefijo+".")) && !vistos[prefijo] {
				vistos[prefijo] = true
				nombres = append(nombres, nombreAdministrada(prefijo))
			}
		}
	}
	return nombres
}

func nombreAdministrada(prefijo string) string {
	switch prefijo {
	case "servicio":
		return idioma.T("los ajustes del servicio")
	case "instancia_global":
		return idioma.T("los ajustes del espacio global")
	case "proyectos_confiables":
		return idioma.T("las carpetas confiables")
	case "comandos_agentes":
		return idioma.T("el programa de cada agente")
	case "concurrencia_servicio":
		return idioma.T("cuántas peticiones atiende a la vez cada agente")
	case "puntos_acceso":
		return idioma.T("las APIs de este espacio")
	case "adaptadores":
		return idioma.T("el acceso a cada agente")
	case "conversion":
		return idioma.T("la conversión de archivos")
	case "presupuesto":
		return idioma.T("el presupuesto")
	case "limites.memoria_proceso_mb":
		return idioma.T("la memoria máxima de cada agente")
	}
	return prefijo
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
			return capa, errorConfiguracion("entorno", nombre, idioma.T("clave desconocida"))
		}
		convertido, err := convertir(tipo, valor)
		if err != nil {
			return capa, errorConfiguracion("entorno", nombre, idioma.T("tipo incorrecto"))
		}
		capa.Valores[clave] = convertido
	}
	return capa, nil
}

// Internos son los valores predeterminados de diseño (prioridad 1). Los de cada agente no están aquí: los da
// agentes.Internos() como otra capa interna, que quien carga la configuración pasa a CargarConfiguracion.
func Internos() Capa {
	valores := map[string]any{
		"servicio.host":                               "127.0.0.1",
		"servicio.puerto":                             17373,
		"servicio.puerto_automatico":                  true,
		"servicio.plazo_drenaje_segundos":             30,
		"servicio.entrada_local":                      true,
		"servicio.buscar_actualizaciones":             true,
		"servicio.requerir_token":                     true,
		"servicio.permitir_remoto":                    false,
		"instancia_global.token_variable_entorno":     "TRAMTAI_TOKEN_GLOBAL",
		"compatibilidad.modo_tolerante":               false,
		"compatibilidad.cancelar_al_desconectar":      true,
		"limites.reserva_salida_tokens":               8192,
		"limites.memoria_proceso_mb":                  2048,
		"modelos.perfiles.rapido.gemini":              "gemini-flash-lite-latest",
		"modelos.perfiles.equilibrado.gemini":         "gemini-flash-latest",
		"modelos.perfiles.profundo.gemini":            "gemini-pro-latest",
		"predeterminados.agente":                      "claude",
		"servicio.registro_mb":                        5,
		"servicio.registro_copias":                    2,
		"modelos.perfiles.rapido.claude":              "haiku",
		"modelos.perfiles.equilibrado.claude":         "sonnet",
		"modelos.perfiles.profundo.claude":            "opus",
		"predeterminados.modo_contexto":               "aislado",
		"predeterminados.formato_salida":              "markdown",
		"concurrencia.maximo_trabajos_instancia":      2,
		"concurrencia.maximo_conversiones_instancia":  1,
		"concurrencia.maximo_tamano_cola":             20,
		"concurrencia.tiempo_maximo_en_cola_segundos": 1800,
		"retencion.modo":                              "temporal",
		"retencion.guardar_historial":                 false,
		"retencion.horas_resultado":                   24,
		"retencion.horas_metadatos":                   168,
		"retencion.max_trabajos":                      10000,
		"retencion.horas_idempotencia":                24,
		"retencion.horas_carga_huerfana":              24,
		"retencion.cache_conversion_habilitada":       false,
		"retencion.eliminar_entradas_al_terminar":     false,
		"retencion.cifrado_aplicacion":                false,
		"conversion.motor":                            "markitdown",
		"conversion.habilitar_plugins":                false,
		"conversion.preparar_al_iniciar":              true,
		"conversion.transformacion_semantica":         false,
		"conversion.comando":                          "markitdown",
		"conversion.tiempo_limite_segundos":           120,
		"conversion.tiempo_total_segundos":            600,
		"conversion.cuota_cargas_mb":                  2048,
		"conversion.maximo_carga_mb":                  50,
		"conversion.maximo_salida_mb":                 20,
		"conversion.maximo_descomprimido_mb":          200,
	}
	return Capa{Origen: OrigenInterno, Fuente: "interno", Valores: valores}
}

// Configuracion es la vista efectiva, con el origen de cada valor.
type Configuracion struct {
	// estado se reemplaza entero al cambiar un valor en marcha (Fijar): los lectores nunca ven uno a medias.
	estado atomic.Pointer[estadoConfiguracion]
	// ignoradas son las claves de seguridad de un local.yaml no confiable que no se aplicaron.
	ignoradas atomic.Pointer[[]string]
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
// Dos Fijar a la vez no se pierden: si otro cambió la configuración entretanto, se vuelve a aplicar sobre la suya.
func (c *Configuracion) Fijar(origen Origen, valores map[string]any) {
	for {
		actual := c.e()
		nuevo := &estadoConfiguracion{valores: maps.Clone(actual.valores), origenes: maps.Clone(actual.origenes)}
		for clave, valor := range valores {
			if nuevo.origenes[clave] <= origen {
				nuevo.valores[clave], nuevo.origenes[clave] = valor, origen
			}
		}
		if c.estado.CompareAndSwap(actual, nuevo) {
			return
		}
	}
}

// Resolver aplica la precedencia de las capas: gana la capa de mayor prioridad,
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

// Ignoradas devuelve las claves de seguridad del local.yaml que no se aplicaron porque la carpeta no es confiable.
func (c *Configuracion) Ignoradas() []string {
	if p := c.ignoradas.Load(); p != nil {
		return *p
	}
	return nil
}

// Reemplazar pone en c los valores de otra (cargada de nuevo, por ejemplo al confiar en la carpeta): quien ya tenía
// c ve los nuevos sin volver a abrir el espacio.
func (c *Configuracion) Reemplazar(otra *Configuracion) {
	c.estado.Store(otra.e())
	c.ignoradas.Store(otra.ignoradas.Load())
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

// Decimal devuelve un valor decimal (escrito como entero o con decimales), o 0 si no existe.
func (c *Configuracion) Decimal(clave string) float64 {
	switch v := c.e().valores[clave].(type) {
	case int:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

// Booleano devuelve un valor booleano, o false si no existe.
func (c *Configuracion) Booleano(clave string) bool {
	v, _ := c.e().valores[clave].(bool)
	return v
}

// maximosLimites acota los límites de conversión y cargas: un valor mayor usa este.
var maximosLimites = map[string]int{
	"conversion.maximo_carga_mb": 1024, "conversion.maximo_salida_mb": 512,
	"conversion.maximo_descomprimido_mb": 8192, "conversion.tiempo_limite_segundos": 3600,
	"conversion.tiempo_total_segundos": 7200, "conversion.cuota_cargas_mb": 102400, "limites.memoria_proceso_mb": 65536,
}

// limitesConCero admiten 0 (la reserva de salida); por debajo cuentan como 0, no como el predeterminado.
var limitesConCero = map[string]bool{"limites.reserva_salida_tokens": true}

// Limite devuelve un límite de seguridad siempre positivo: 0 o negativo usa el predeterminado (no «sin límite») y
// un valor por encima de su máximo usa el máximo.
func (c *Configuracion) Limite(clave string) int {
	v := c.Entero(clave)
	switch {
	case limitesConCero[clave]:
		v = max(v, 0)
	case v <= 0:
		v, _ = Internos().Valores[clave].(int)
	}
	if maximo := maximosLimites[clave]; maximo > 0 && v > maximo {
		v = maximo
	}
	return v
}

// Contextos devuelve limites.contexto_tokens: «agente» o «agente/modelo» → tokens. Es una lista de entradas
// «clave=tokens» y no un mapa porque un modelo puede llevar puntos (gpt-5.5) y las claves de configuración los usan
// como separador. Una entrada mal escrita se ignora.
func (c *Configuracion) Contextos() map[string]int {
	contextos := map[string]int{}
	for _, e := range c.Lista("limites.contexto_tokens") {
		clave, valor, ok := strings.Cut(fmt.Sprint(e), "=")
		if n, err := strconv.Atoi(strings.TrimSpace(valor)); ok && err == nil && n > 0 {
			contextos[strings.TrimSpace(clave)] = n
		}
	}
	return contextos
}

// Perfiles devuelve modelos.perfiles como perfil -> agente -> identificador de modelo.
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
func (c *Configuracion) AplicarPredeterminados(s nucleo.SolicitudEjecucion) nucleo.SolicitudEjecucion {
	if s.Agente == "" {
		s.Agente = c.Texto("predeterminados.agente")
		// El modelo predeterminado va con su agente: si la solicitud eligió otro agente, usa el modelo de ese CLI.
		if s.Modelo == "" && s.PerfilModelo == "" {
			s.Modelo = c.Texto("predeterminados.modelo")
			// Su esfuerzo también: solo cuando la petición no eligió agente, modelo ni perfil. Un valor escrito a mano
			// que no es un nivel se ignora (el del agente) en lugar de rechazar todas las peticiones.
			if e := c.Texto("predeterminados.esfuerzo"); s.Esfuerzo == "" && slices.Contains(nucleo.NivelesEsfuerzo, e) {
				s.Esfuerzo = e
			}
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

func (c *Capa) validar(estricta bool) error {
	claves := make([]string, 0, len(c.Valores))
	for clave := range c.Valores {
		claves = append(claves, clave)
	}
	sort.Strings(claves)
	confiable := -1 // se averigua una sola vez y solo si local.yaml trae claves administradas
	for _, clave := range claves {
		tipo, conocida := tipoDe(clave)
		if !conocida {
			return errorConfiguracion(c.Fuente, clave, idioma.T("clave desconocida"))
		}
		if c.Origen == OrigenProyecto && esAdministrada(clave) {
			return errorConfiguracion(c.Fuente, clave, idioma.T("clave de seguridad no admitida en la configuración versionada; defínela en la global (o en local.yaml si el proyecto es confiable)"))
		}
		// Un local.yaml también puede llegar con el repositorio (git add -f): sus claves administradas solo valen si
		// el usuario ya confió en la carpeta desde la configuración global, y la confianza nunca se la da el propio proyecto.
		if c.Origen == OrigenLocal && esAdministrada(clave) {
			global, _ := RutaGlobal()
			if confiable < 0 {
				confiable = 0
				if g, err := CargarConfiguracion("", Capa{}); err == nil && g.ProyectoConfiable(filepath.Dir(filepath.Dir(c.Fuente))) {
					confiable = 1
				}
			}
			if clave == "proyectos_confiables" || confiable == 0 {
				if !estricta {
					// Al cargar no se aplica, pero el proyecto se abre: quien lo abrió ve el aviso y decide si confía.
					delete(c.Valores, clave)
					c.Ignoradas = append(c.Ignoradas, clave)
					continue
				}
				if clave == "proyectos_confiables" {
					return errorConfiguracion(c.Fuente, clave, idioma.Tf("solo se admite en la configuración global (%s)", global))
				}
				return errorConfiguracion(c.Fuente, clave, idioma.Tf("clave de seguridad: en local.yaml solo se admite si la carpeta del proyecto figura en proyectos_confiables de la configuración global (%s); añádela allí", global))
			}
		}
		if !tipoCorrecto(tipo, c.Valores[clave]) {
			return errorConfiguracion(c.Fuente, clave, idioma.T("tipo incorrecto"))
		}
		if strings.HasPrefix(clave, "presupuesto.") && limiteNoValido(c.Valores[clave]) {
			return errorConfiguracion(c.Fuente, clave, idioma.T("los límites del presupuesto no pueden ser negativos"))
		}
	}
	// Promesas que Tramtai aún no cumple: se rechazan en vez de aceptarlas sin efecto.
	for _, clave := range []string{"retencion.guardar_historial", "retencion.cifrado_aplicacion", "retencion.cache_conversion_habilitada"} {
		if c.Valores[clave] == true {
			return errorConfiguracion(c.Fuente, clave, idioma.T("no soportado todavía: no hay historial permanente, cifrado propio ni cache documental"))
		}
	}
	if modo, existe := c.Valores["retencion.modo"]; existe && modo != "temporal" {
		return errorConfiguracion(c.Fuente, "retencion.modo", idioma.T("solo se admite temporal"))
	}
	if version, existe := c.Valores["version"]; existe && version != 1 {
		return errorConfiguracion(c.Fuente, "version", idioma.T("versión de formato no soportada"))
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

// limiteNoValido rechaza los límites negativos y, en decimales, NaN e infinito.
func limiteNoValido(valor any) bool {
	switch n := valor.(type) {
	case int:
		return n < 0
	case float64:
		return !nucleo.FinitoNoNegativo(n)
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
	case tipoDecimal:
		switch valor.(type) {
		case int, float64:
			correcto = true
		}
	}
	return correcto
}

func convertir(tipo tipoValor, texto string) (any, error) {
	switch tipo {
	case tipoEntero:
		return strconv.Atoi(texto)
	case tipoBooleano:
		return strconv.ParseBool(texto)
	case tipoDecimal:
		return strconv.ParseFloat(texto, 64)
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

// errorConfiguracion indica archivo, propiedad y motivo sin revelar valores.
func errorConfiguracion(fuente, clave, motivo string) error {
	if clave == "" {
		return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, fuente+": "+motivo)
	}
	return nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, fmt.Sprintf("%s: %s: %s", fuente, clave, motivo))
}
