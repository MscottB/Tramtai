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
	// variable del entorno heredado.
	Entorno []string
}

// FuenteVersion dice dónde se consulta la última versión de un CLI.
type FuenteVersion struct {
	// Tipo es "npm" (Nombre: el paquete), "github" (Nombre: «dueño/repositorio», último release) o "url" (Nombre: la URL
	// que devuelve la versión como texto o JSON).
	Tipo, Nombre string
	// Prefijo se quita del comienzo de la versión que da la fuente («rust-v» en el tag_name «rust-v0.167.2» de Codex);
	// vacío si la versión viene limpia.
	Prefijo string
}

// DescargaVerificada es un binario que Tramtai baja él mismo, sin instalador. "{version}" va en las dos URL; URLSumas
// es el SHA256SUMS del fabricante, con líneas «<sha256>  <archivo>».
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
}

// Receta reúne los datos de instalación y sesión de un CLI. Las órdenes de Instalar, Actualizar y Desinstalar y las
// demás tablas se indexan por runtime.GOOS; si un sistema no tiene entrada, esa acción no existe ahí.
type Receta struct {
	// Actualizar sin entrada: reinstalar. Desinstalar sin entrada: se borra Borrar. Windows de Copilot no tiene entrada en
	// Instalar porque va por Descarga.
	Instalar, Actualizar, Desinstalar map[string]OrdenReceta
	// Borrar son las rutas que se quitan al desinstalar un CLI que no tiene orden propia (Desinstalar sin entrada),
	// antes que DatosCLI; con "~" como en Rutas.
	Borrar map[string][]string
	// Requisito es lo que hace falta antes de instalar: "" | "npm" | "winget" | "brew".
	Requisito map[string]string
	// Rutas son dónde puede quedar el ejecutable tras instalar ("~" es la casa del usuario).
	Rutas map[string][]string
	// DatosCLI son las carpetas de sesión y ajustes del CLI, para «Borrar también».
	DatosCLI []string
	// FijaVersion indica que Instalar admite "{version}".
	FijaVersion bool
	// Descarga solo existe si Tramtai baja el binario él mismo en vez de usar un instalador.
	Descarga map[string]DescargaVerificada
	Ultima   FuenteVersion
	// Sesion es nil si el CLI no inicia sesión (antigravity).
	Sesion *LectorSesion
	// Comprobadas son las versiones probadas por acción, exactas o con comodín de parche ("2.1.*").
	Comprobadas map[string][]string
}
