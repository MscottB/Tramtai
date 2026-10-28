package contrato

import "time"

// AjustesServicio son los del servicio y del equipo que se ven en Ajustes (GET /api/v1/servicio/ajustes): el puerto en
// uso, el configurado y si busca otro libre al iniciar, si se puede reiniciar desde la app y el arranque con el sistema
// (nil si el sistema no lo admite) con el modo en que arranca: la app en la bandeja o solo el servicio.
type AjustesServicio struct {
	Puerto            int    `json:"puerto"`
	PuertoConfigurado int    `json:"puerto_configurado"`
	PuertoAutomatico  bool   `json:"puerto_automatico"`
	Reiniciable       bool   `json:"reiniciable"`
	Autoarranque      *bool  `json:"autoarranque,omitempty"`
	ModoArranque      string `json:"modo_arranque,omitempty" ts:"'' | 'app' | 'servicio'"` // vacío si no arranca
}

// ClaveGuardada es una clave de proveedor en GET /api/v1/servicio/claves: si el servicio la tiene (en su almacén o en su
// entorno). Nunca lleva el valor.
type ClaveGuardada struct {
	Nombre   string `json:"nombre"`
	Guardada bool   `json:"guardada"`
}

// EstadoActualizacion es el de las actualizaciones: la versión actual, si se buscan, si hay otra nueva (con su página y
// sus notas) y si se instala desde aquí; Comprobado es cuándo terminó la última consulta a GitHub y Fallo si falló.
type EstadoActualizacion struct {
	Actual     string    `json:"actual"`
	Buscar     bool      `json:"buscar"`
	Disponible bool      `json:"disponible"`
	Version    string    `json:"version"`
	URL        string    `json:"url"`
	Notas      string    `json:"notas"`
	Instalable bool      `json:"instalable"`
	Comprobado time.Time `json:"comprobado,omitzero"`
	Fallo      bool      `json:"fallo,omitempty"`
}

// ProgresoActualizacion es el avance de la actualización en curso: la fase (descargando, verificando, listo, instalando
// o error; vacía si no hay ninguna), los bytes descargados y el total y, en listo, cuándo instalará.
type ProgresoActualizacion struct {
	Fase      string     `json:"fase" ts:"'' | 'descargando' | 'verificando' | 'listo' | 'instalando' | 'error'"`
	Version   string     `json:"version"`
	Recibidos int64      `json:"recibidos"`
	Total     int64      `json:"total"`
	InstalaEn *time.Time `json:"instala_en,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// Acerca son los datos de «Acerca de Tramtai»: versión, compilación, sistema y servicio. Las rutas solo van a quien es
// de este equipo, porque llevan el nombre del usuario del sistema. Nunca lleva claves ni tokens.
type Acerca struct {
	Version      string         `json:"version"`
	Commit       string         `json:"commit"`
	Modificado   bool           `json:"modificado"`
	Go           string         `json:"go"`
	Portable     bool           `json:"portable"`
	Entorno      string         `json:"entorno"`          // servidor, portable o escritorio
	Cuenta       string         `json:"cuenta,omitempty"` // la cuenta del sistema con la que corre el servicio del servidor
	Sistema      string         `json:"sistema"`
	Arquitectura string         `json:"arquitectura"`
	Almacen      string         `json:"almacen"`
	Servicio     ServicioAcerca `json:"servicio"`
	Rutas        *RutasAcerca   `json:"rutas,omitempty"`
}

// ServicioAcerca es el servicio en marcha: su dirección, su proceso y desde cuándo.
type ServicioAcerca struct {
	Direccion  string    `json:"direccion"`
	PID        int       `json:"pid"`
	IniciadoEn time.Time `json:"iniciado_en"`
}

// RutasAcerca son las carpetas de Tramtai en este equipo.
type RutasAcerca struct {
	Datos         string `json:"datos"`
	Configuracion string `json:"configuracion"`
	Registro      string `json:"registro"`
}

// EstadoTerminos es lo que la Web UI necesita para pedir la aceptación de los términos de uso: la versión vigente, si
// este entorno aún no la aceptó y los cinco riesgos que se enseñan antes de aceptar; Servidor dice si son los del modo
// servidor (la Web UI toma el texto de sus propias claves, con la redacción que corresponda).
type EstadoTerminos struct {
	Version   string   `json:"version"`
	Pendiente bool     `json:"pendiente"`
	Riesgos   []string `json:"riesgos"`
	Servidor  bool     `json:"servidor"`
}

// EntradaRemota es lo que la página de entrada de la Web UI necesita saber antes de pedir nada (GET /ui/entrada, sin
// sesión): si el servidor tiene administrador con contraseña, si pide además el código TOTP y si la petición llegó por
// HTTPS (sin él no se entra con contraseña).
type EntradaRemota struct {
	Contrasena bool `json:"contrasena"`
	TOTP       bool `json:"totp"`
	HTTPS      bool `json:"https"`
}

// EstadoServidor es lo que enseña Ajustes → Servidor (GET /ui/servidor y GET /api/v1/servicio/servidor, solo en el modo
// servidor): el modo de HTTPS (servicio.tls.modo), los dominios, los proxies de confianza, cuándo vence el certificado
// propio, si hay administrador con contraseña y con TOTP, si el servicio está abierto a la red (Expuesto: fuera de
// loopback o detrás de un proxy) y las suscripciones de los agentes. Solo se lee: cada dato se cambia en el servidor.
type EstadoServidor struct {
	TLS              string                `json:"tls" ts:"'ninguno' | 'archivos' | 'letsencrypt'"`
	Dominios         []string              `json:"dominios"`
	Confiables       []string              `json:"confiables"`
	VenceCertificado *time.Time            `json:"vence_certificado,omitempty"`
	Administrador    bool                  `json:"administrador"`
	TOTP             bool                  `json:"totp"`
	Expuesto         bool                  `json:"expuesto"`
	Suscripciones    []SuscripcionServidor `json:"suscripciones"`
}

// SuscripcionServidor es un agente que usa la suscripción en el servidor: si es la personal del administrador (con su
// aviso fijo) y cuántas peticiones a la vez admite (adaptadores.<agente>.suscripcion_simultaneas).
type SuscripcionServidor struct {
	Agente      string `json:"agente"`
	Personal    bool   `json:"personal"`
	Simultaneas int    `json:"simultaneas"`
}

// SecretoTOTP es el secreto nuevo que Ajustes → Seguridad enseña antes de guardarlo: se guarda al confirmarlo con un
// código de la app de autenticación.
type SecretoTOTP struct {
	Secreto string `json:"secreto"`
	URI     string `json:"uri"`
}

// CodigosRecuperacion son los 10 códigos de recuperación de un solo uso: solo se ven al crearlos (se guarda su huella).
type CodigosRecuperacion struct {
	Codigos []string `json:"codigos"`
}
