package contrato

import "time"

// EstadoAgente es lo que el servicio concluye de un agente al comprobarlo. Lo calcula el servicio (una sola regla) y
// la Web UI y la terminal solo lo pintan.
type EstadoAgente string

const (
	AgenteListo        EstadoAgente = "listo"
	AgenteDesconectado EstadoAgente = "desconectado"
	// AgenteNoInstalado gana a AgenteDesconectado: sin su CLI, lo primero es instalarlo (docs/equipo/decisiones).
	AgenteNoInstalado      EstadoAgente = "no_instalado"
	AgenteFaltaClave       EstadoAgente = "falta_clave" // con clave de API: falta la clave
	AgenteSinSesion        EstadoAgente = "sin_sesion"  // con suscripción: su CLI no tiene sesión
	AgenteUsaClave         EstadoAgente = "usa_clave"
	AgenteSesionNoResponde EstadoAgente = "sesion_no_responde"
	AgenteRevisar          EstadoAgente = "revisar"
)

// EstadosAgente son todos los estados, en este orden; el generador de tipos de la Web UI los convierte en una unión.
var EstadosAgente = []EstadoAgente{AgenteListo, AgenteDesconectado, AgenteNoInstalado, AgenteFaltaClave, AgenteSinSesion,
	AgenteUsaClave, AgenteSesionNoResponde, AgenteRevisar}

// CapacidadesAgente es lo que garantiza un agente, según su ficha.
type CapacidadesAgente struct {
	Texto                   bool   `json:"texto"`
	Streaming               string `json:"streaming"` // tokens o final
	SalidaEstructurada      bool   `json:"salida_estructurada"`
	UsoTokens               string `json:"uso_tokens"`
	ModeloEfectivoInformado bool   `json:"modelo_efectivo_informado"`
	ToolCallingCompatible   bool   `json:"tool_calling_compatible"`
	Imagenes                bool   `json:"imagenes"`
	SesionesPersistentes    bool   `json:"sesiones_persistentes"`
}

// Adaptador es un agente tal como lo cuenta GET /api/v1/capacidades: su detección y el estado que el servicio
// deduce de ella.
type Adaptador struct {
	Agente        string       `json:"agente"`
	Estado        EstadoAgente `json:"estado"`
	Desconectado  bool         `json:"desconectado"` // el usuario lo desconectó en Agentes
	Version       string       `json:"version_detectada,omitempty"`
	Autenticacion string       `json:"autenticacion,omitempty"` // suscripcion o clave_api
	Terminos      string       `json:"terminos,omitempty"`
	Cuenta        string       `json:"cuenta,omitempty"` // la de la suscripción, si el CLI la informa
	Problema      string       `json:"problema,omitempty"`
	// ProblemaCodigo es no_instalado, falta_clave, usa_clave_api, sesion_no_responde, aviso (no impide usarlo) o vacío.
	ProblemaCodigo string            `json:"problema_codigo,omitempty"`
	Capacidades    CapacidadesAgente `json:"capacidades"`
}

// Capacidades es lo que devuelve GET /api/v1/capacidades.
type Capacidades struct {
	Version              string `json:"version"`
	FormatoSerializacion string `json:"formato_serializacion"`
	// Entradas son las extensiones de archivo que se admiten: las de texto y las que pasan por el conversor.
	Entradas    []string         `json:"entradas"`
	Adaptadores []Adaptador      `json:"adaptadores"`
	Limites     Limites          `json:"limites"`
	Conversor   *EstadoConversor `json:"conversor,omitempty"`
}

// EventoTrabajo es lo que el flujo de la instancia (GET /api/v1/eventos, evento «trabajo») cuenta de cada trabajo:
// dónde va, nunca qué lleva (ni el mensaje, ni la respuesta, ni los archivos).
type EventoTrabajo struct {
	ID          string    `json:"id"`
	Estado      string    `json:"estado"`
	Etapa       string    `json:"etapa" ts:"'cola' | 'preparacion' | 'ejecucion' | 'validacion' | 'llegada'"` // cola, preparacion, ejecucion, validacion o llegada (nucleo.Estado.Etapa)
	Agente      string    `json:"agente"`
	Modelo      string    `json:"modelo"`
	API         string    `json:"api"`
	Origen      string    `json:"origen"`
	Intento     int       `json:"intento"`
	CodigoError string    `json:"codigo_error"`
	Creado      time.Time `json:"creado"`
	Actualizado time.Time `json:"actualizado"`
}

// ResumenInstancia es lo que cuenta GET /api/v1/instancia para Inicio: el estado de la instancia, la dirección del
// servicio y las peticiones de apps con la clave del espacio en las últimas 24 h.
type ResumenInstancia struct {
	EstadoInstancia
	Base           string `json:"base"`
	AppConClave24h int    `json:"app_con_clave_24h"`
	// SugerirApiPropia: con muchas de esas peticiones conviene dar a esas apps su propia API.
	SugerirApiPropia bool `json:"sugerir_api_propia"`
}

// Paso es uno de los Primeros pasos de Inicio: la página que lo resuelve y si ya está hecho.
type Paso struct {
	Pagina string `json:"pagina"` // agentes, ejecutar, conectar o configuracion
	Hecho  bool   `json:"hecho"`
	// Listos y Total solo cuentan en el de agentes: cuántos están listos de los que hay.
	Listos int `json:"listos"`
	Total  int `json:"total"`
}

// PrimerosPasos es lo que devuelve GET /api/v1/instancia/pasos. Lleva los agentes que comprobó para calcularlos: Inicio
// los pinta sin pedir otra comprobación (cada una ejecuta los CLI).
type PrimerosPasos struct {
	Pasos       []Paso      `json:"pasos"`
	Adaptadores []Adaptador `json:"adaptadores"`
}

// Modelo es una entrada del catálogo informativo (GET /api/v1/modelos).
type Modelo struct {
	ID     string `json:"id"`
	Agente string `json:"agente,omitempty"`
	Modelo string `json:"modelo,omitempty"`
	Perfil string `json:"perfil,omitempty"`
	Origen string `json:"origen"`
	// Solo en /api/v1/modelos: el agente/modelo al que lleva un perfil, la versión real que informó el agente la
	// última vez (por ejemplo claude-sonnet-5-5 para "sonnet") y si es el modelo predeterminado de la instancia.
	Resuelto       string `json:"resuelto,omitempty"`
	Version        string `json:"version,omitempty"`
	Predeterminado bool   `json:"predeterminado,omitempty"`
	// Esfuerzos son los niveles de esfuerzo que admite su agente; vacío si no se puede elegir.
	Esfuerzos []string `json:"esfuerzos,omitempty"`
	// EsfuerzoPredeterminado es el nivel que usa el agente si no se elige; vacío si no se conoce con certeza.
	EsfuerzoPredeterminado string `json:"esfuerzo_predeterminado,omitempty"`
	// ContextoTokens es el contexto conocido del modelo (configurado o informado por el agente) y ContextoOrigen de
	// dónde sale; sin dato, no se comprueba si una entrada cabe.
	ContextoTokens int    `json:"contexto_tokens,omitempty"`
	ContextoOrigen string `json:"contexto_origen,omitempty"`
	// EsfuerzoEspacio, solo en el modelo predeterminado, es el esfuerzo que el espacio usa con él
	// (predeterminados.esfuerzo); vacío, el de su agente.
	EsfuerzoEspacio string `json:"esfuerzo_espacio,omitempty"`
}
