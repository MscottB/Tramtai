// Package contrato tiene los tipos que viajan entre el servicio y sus clientes (la CLI, la terminal y la app de
// escritorio). No depende del servicio: así un cliente no enlaza el planificador, la base ni los adaptadores.
package contrato

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/nucleo"
)

var reNombrePunto = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// EstadoInstancia resume una instancia para la CLI y la TUI.
type EstadoInstancia struct {
	ID           string         `json:"id"`
	Ambito       string         `json:"ambito"`
	Raiz         string         `json:"raiz,omitempty"`
	ArchivoToken string         `json:"archivo_token"`
	Agente       string         `json:"agente_predeterminado"`
	Perfil       string         `json:"perfil_predeterminado,omitempty"`
	Trabajos     map[string]int `json:"trabajos"`
	UltimoUso    time.Time      `json:"ultimo_uso,omitzero"`
}

// Espacio es un espacio de trabajo tal como lo ven la página «Espacios» y tramtai espacios: también los detenidos y
// los que ya no encuentran su carpeta.
type Espacio struct {
	ID          string    `json:"id"`
	Ambito      string    `json:"ambito"`
	Ruta        string    `json:"ruta,omitempty"`
	Nombre      string    `json:"nombre"`
	Activo      bool      `json:"activo"`
	UltimoUso   time.Time `json:"ultimo_uso,omitzero"`
	UltimoError string    `json:"ultimo_error,omitempty"`
	// Estado es activo, detenido o no_encontrado (falta la carpeta o su .tramtai/config.yaml).
	Estado     string `json:"estado"`
	Confiado   bool   `json:"confiado"`
	APIs       int    `json:"apis"`
	Peticiones int    `json:"peticiones"`
	// Claves son las de seguridad que trae el local.yaml de un proyecto no confiado: se enseñan antes de confiar.
	Claves []string `json:"claves,omitempty"`
}

// Estado es la vista del servicio que devuelve GET /api/v1/servicio.
type Estado struct {
	Identificador string            `json:"identificador_servicio"`
	Version       string            `json:"version"`
	PID           int               `json:"pid"`
	IniciadoEn    time.Time         `json:"iniciado_en"`
	Base          string            `json:"base"`
	Instancias    []EstadoInstancia `json:"instancias"`
	Puntos        []EstadoPunto     `json:"puntos_acceso"`
	Errores       []string          `json:"errores_configuracion,omitempty"`
	// Actualizacion es la versión publicada más nueva que la actual, si la hay (la bandeja la ofrece).
	Actualizacion *actualizacion.Version `json:"actualizacion,omitempty"`
}

// Descriptor identifica el servicio en marcha.
type Descriptor struct {
	Identificador string           `json:"identificador_servicio"`
	PID           int              `json:"pid"`
	FirmaInicio   string           `json:"firma_inicio_proceso"`
	IniciadoEn    time.Time        `json:"iniciado_en"`
	Host          string           `json:"host"`
	Puerto        int              `json:"puerto"`
	Base          string           `json:"base"`
	Version       string           `json:"version"`
	Instancias    []string         `json:"instancias_activas"`
	Puntos        []map[string]any `json:"puntos_acceso"`
}

// RutaDescriptor es el archivo del descriptor en el directorio de datos.
func RutaDescriptor(datos string) string { return filepath.Join(datos, "servicio.json") }

// LeerDescriptor lee el descriptor; no comprueba que el servicio siga vivo (lo hace el cliente con /healthz).
func LeerDescriptor(datos string) (Descriptor, error) {
	var d Descriptor
	contenido, err := os.ReadFile(RutaDescriptor(datos))
	if err != nil {
		return d, err
	}
	return d, json.Unmarshal(contenido, &d)
}

// PruebaIdentidad es lo que responde el servicio al reto de GET /healthz?reto=: el HMAC-SHA256 del token de servicio
// sobre el reto. Solo quien conoce el token puede calcularla, así que el cliente comprueba al servicio antes de enviarle
// el token.
func PruebaIdentidad(token, reto string) string {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte("tramtai-healthz:" + reto))
	return hex.EncodeToString(m.Sum(nil))
}

// ActivacionInstancia es la respuesta a la activación de una instancia.
type ActivacionInstancia struct {
	Instancia EstadoInstancia `json:"instancia"`
	Token     string          `json:"token"`
}

// EntradaConfiguracion es una fila de la configuración efectiva de una instancia.
type EntradaConfiguracion struct {
	Clave  string `json:"clave"`
	Valor  any    `json:"valor"`
	Origen string `json:"origen"`
}

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

// UsoPunto resume las peticiones de un punto de acceso que conserva la retención.
type UsoPunto struct {
	Peticiones int        `json:"peticiones"`
	Errores    int        `json:"errores"`
	Ultima     *time.Time `json:"ultima,omitempty"`
}

// ListaPuntos es lo que devuelve GET /api/v1/puntos: los puntos de la instancia con su uso y el consumo de su
// presupuesto (solo los que tienen presupuesto), por nombre.
type ListaPuntos struct {
	Puntos       []EstadoPunto                       `json:"puntos_acceso"`
	Uso          map[string]UsoPunto                 `json:"uso"`
	Presupuestos map[string]nucleo.EstadoPresupuesto `json:"presupuestos"`
}

// UsoHistorial es lo que ocupa el historial de una instancia: sus trabajos y el tamaño de su base.
type UsoHistorial struct {
	Trabajos int   `json:"trabajos"`
	Bytes    int64 `json:"bytes"`
}

// Retencion son los plazos y el tope del historial de una instancia (GET y PUT /api/v1/instancia/retencion). Uso solo
// se lee: al fijarla no se envía.
type Retencion struct {
	HorasResultado int          `json:"horas_resultado"`
	HorasMetadatos int          `json:"horas_metadatos"`
	MaxTrabajos    int          `json:"max_trabajos"`
	Uso            UsoHistorial `json:"uso"`
}

// Limites son los topes de una petición y de las cargas (GET /api/v1/limites).
type Limites struct {
	ExtensionesTexto     []string `json:"extensiones_texto"`
	ExtensionesConversor []string `json:"extensiones_conversor"`
	CargasPorPeticion    int      `json:"cargas_por_peticion"`
	CargaMB              int      `json:"carga_mb"`
	TextoConvertidoMB    int      `json:"texto_convertido_mb"`
	CuotaCargasMB        int      `json:"cuota_cargas_mb"`
	CuerpoMB             int      `json:"cuerpo_mb"`
	EsquemaKB            int      `json:"esquema_kb"`
	Mensajes             int      `json:"mensajes"`
	ReservaSalidaTokens  int      `json:"reserva_salida_tokens"`
}

// AjustesConversor es lo que se cambia del conversor de archivos (PUT /api/v1/conversor/ajustes).
type AjustesConversor struct {
	PrepararAlIniciar bool `json:"preparar_al_iniciar"`
}

// EstadoConversor es el del conversor de archivos (markitdown): listo, preparando, falta o error.
type EstadoConversor struct {
	Estado  string `json:"estado"`
	Origen  string `json:"origen,omitempty"`  // configurado, paquete, preparado o sistema
	Version string `json:"version,omitempty"` // la de markitdown, si la conoce Tramtai
	Etapa   string `json:"etapa,omitempty"`   // python, paquetes o comprobacion
	// Progreso aproximado de la preparación, de 0 a 100.
	Progreso int `json:"progreso,omitempty"`
	// Mensaje es el error de la última preparación, legible.
	Mensaje           string `json:"mensaje,omitempty"`
	PrepararAlIniciar bool   `json:"preparar_al_iniciar"`
}
