package contrato

import "time"

// AjustesServicio son los del servicio y del equipo que se ven en Ajustes (GET /api/v1/servicio/ajustes): el puerto en
// uso, el configurado y si busca otro libre al iniciar, si se puede reiniciar desde la app y el arranque con el sistema
// (nil si el sistema no lo admite).
type AjustesServicio struct {
	Puerto            int   `json:"puerto"`
	PuertoConfigurado int   `json:"puerto_configurado"`
	PuertoAutomatico  bool  `json:"puerto_automatico"`
	Reiniciable       bool  `json:"reiniciable"`
	Autoarranque      *bool `json:"autoarranque,omitempty"`
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
