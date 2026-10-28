package adaptadores

import "tramtai/internal/configuracion"

// Ficha describe un agente para el catálogo (internal/agentes): cómo se llama y se ve, qué admite y cómo se crea.
// Cada módulo de internal/agentes/<nombre> declara la suya; el resto del código pregunta al catálogo en lugar de
// nombrar agentes.
type Ficha struct {
	// Nombre es el id del agente: el que va delante de agente/modelo.
	Nombre string
	// NombreVisible es como se muestra («Gemini API»); Programa, el CLI o servicio que hay detrás («Claude Code»).
	NombreVisible string
	Programa      string
	// Codigo son las dos letras de su línea en la Web UI; ColorClaro y ColorOscuro, su color en cada tema.
	Codigo      string
	ColorClaro  string
	ColorOscuro string
	// Orden es su lugar en las listas, de menor a mayor.
	Orden int
	// Streaming es "tokens" si el texto llega en vivo o "final" si llega al terminar.
	Streaming               string
	SalidaEstructurada      bool
	ModeloEfectivoInformado bool
	// ValoresEsfuerzo es el valor del CLI para cada nivel de nucleo.NivelesEsfuerzo, de menor a mayor y hasta su
	// máximo; vacío si no se puede elegir el esfuerzo.
	ValoresEsfuerzo []string
	// EsfuerzoPredeterminado da el nivel que usa el CLI con un modelo concreto si no se le indica otro ("" si no se
	// conoce con certeza); nil si nunca se conoce.
	EsfuerzoPredeterminado func(modelo string) string
	// ModelosSinEsfuerzo son los modelos con los que el CLI no admite elegir el esfuerzo ("" es su predeterminado).
	ModelosSinEsfuerzo []string
	// Autenticaciones son los modos que admite (AutenticacionSuscripcion, AutenticacionClaveAPI).
	Autenticaciones []string
	// VariablesClave son las variables de su clave. La primera es la que Agentes muestra para guardarla con tramtai claves
	// (debe estar en credenciales.Nombres); el adaptador decide en qué orden las lee.
	VariablesClave []string
	// Comando es el valor predeterminado de comandos_agentes.<nombre>; vacío si no es un CLI.
	Comando string
	// Concurrencia es el valor predeterminado de concurrencia_servicio.adaptadores.<nombre>.
	Concurrencia int
	// PermitidoEnApps indica que su canario pasó: un contenido hostil no le hace usar herramientas, así que se puede
	// usar desde apps y APIs (adaptadores.<nombre>.permitir_apps lo habilita igualmente).
	PermitidoEnApps bool
	// URLInstalacion es dónde se instala o se consigue su clave.
	URLInstalacion string
	// InicioSesion son los argumentos con que su CLI inicia sesión con la suscripción (tramtai --servidor agentes entrar);
	// URLTerminos, las condiciones del proveedor que se enseñan al activar la suscripción personal.
	InicioSesion []string
	URLTerminos  string
	// Crear construye el adaptador con la configuración efectiva.
	Crear func(c *configuracion.Configuracion) Adaptador
}
