package adaptadores

import (
	"regexp"

	"tramtai/internal/configuracion"
)

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
	// Receta es cómo Tramtai instala, actualiza, conecta y desinstala el CLI; nil si no es un CLI.
	Receta *Receta
	// Crear construye el adaptador con la configuración efectiva.
	Crear func(c *configuracion.Configuracion) Adaptador
}

// OrdenReceta es una orden fija de una receta. Es solo datos: quien la ejecuta (la preparación) decide los plazos y la
// entrada estándar. "{version}" se sustituye por la versión fijada; si no se fija ninguna, se omite el argumento o la
// variable que lo contiene.
type OrdenReceta struct {
	Programa   string
	Argumentos []string
	// Entorno son variables NOMBRE=valor que la orden necesita (CODEX_NON_INTERACTIVE=1…). «NOMBRE=» vacío quita esa
	// variable del entorno heredado. Omitir «CODEX_RELEASE={version}» sin versión fijada no limpia una variable heredada:
	// quien ejecuta la orden antepone «NOMBRE=» (vacío) por cada variable que la orden nombra con {version}.
	Entorno []string
}

// FuenteVersion dice dónde se consulta la última versión de un CLI.
type FuenteVersion struct {
	// Tipo es "npm" (Nombre: el paquete), "github" (Nombre: «dueño/repositorio», último release) o "url" (Nombre: la URL
	// que devuelve la versión: texto con la versión sola, como la de Claude, o JSON, en cuyo caso se lee su campo
	// «version», como el manifiesto de Antigravity).
	Tipo, Nombre string
	// Prefijo se quita del comienzo de la versión que da la fuente («rust-v» en el tag_name «rust-v0.167.2» de Codex);
	// vacío si la versión viene limpia.
	Prefijo string
}

// DescargaVerificada es un binario que Tramtai baja él mismo, sin instalador. "{version}" va en las dos URL y, sin versión
// fijada, se sustituye por el resultado de Ultima (ya sin Prefijo): la omisión de {version} solo vale para OrdenReceta.
// "{arq}" (solo en URL) es x64 o arm64 según runtime.GOARCH. URLSumas solo lleva {version}: es el SHA256SUMS único de la
// versión, con líneas «<sha256>  <archivo>».
type DescargaVerificada struct{ URL, URLSumas string }

// LectorSesion describe cómo iniciar sesión con un CLI y cómo leer lo que imprime. Las expresiones se aplican al texto
// acumulado y sin secuencias ANSI; el grupo 1 de Enlace y Codigo es el valor.
type LectorSesion struct {
	// Argumentos son los del inicio de sesión (Ficha.InicioSesion se rellena desde aquí).
	Argumentos []string
	// Entorno son variables NOMBRE=valor que el inicio de sesión necesita (Copilot: COPILOT_DEBUG_BROWSER, para que no
	// abra el navegador del sistema por su cuenta).
	Entorno []string
	PTY     bool
	// Enlace y Codigo son obligatorios si el CLI los imprime; Codigo es nil si no hay (Claude). PidePegar es nil si el
	// CLI no espera un código pegado.
	Enlace, Codigo, PidePegar *regexp.Regexp
	// Aviso es la clave del texto de lo que la cuenta del proveedor exige antes de iniciar sesión y no es un ajuste del
	// CLI (Codex: activar el código de dispositivo en ChatGPT); AvisoEnlace, dónde se hace. Vacíos si no hay.
	Aviso, AvisoEnlace string
}

// Receta reúne los datos de instalación y sesión de un CLI. Las órdenes de Instalar y Actualizar y las demás tablas se
// indexan por runtime.GOOS; si un sistema no tiene entrada, esa acción no existe ahí. Los Programa sueltos (claude, codex,
// copilot, agy) se resuelven por Rutas o comandos_agentes, no por el PATH del servicio.
type Receta struct {
	// Actualizar sin entrada: reinstalar. Windows de Copilot no tiene entrada en Instalar porque va por Descarga.
	Instalar, Actualizar map[string]OrdenReceta
	// Borrar son las rutas que se quitan al desinstalar (ningún CLI tiene orden propia), antes que DatosCLI; con "~" como
	// en Rutas.
	Borrar map[string][]string
	// Rutas son dónde puede quedar el ejecutable tras instalar ("~" es la casa del usuario).
	Rutas map[string][]string
	// DatosCLI son las carpetas de sesión y ajustes del CLI, para «Borrar también». No expresa CLAUDE_CONFIG_DIR, CODEX_HOME
	// ni COPILOT_HOME (el M0 lo deja así).
	DatosCLI []string
	// FijaVersion indica que Instalar o Descarga admiten "{version}".
	FijaVersion bool
	// Descarga solo existe si Tramtai baja el binario él mismo en vez de usar un instalador.
	Descarga map[string]DescargaVerificada
	Ultima   FuenteVersion
	// Sesion es nil si el CLI no inicia sesión (antigravity).
	Sesion *LectorSesion
	// Comprobadas son las versiones probadas por acción, exactas o con comodín de parche ("2.1.*").
	Comprobadas map[string][]string
}
