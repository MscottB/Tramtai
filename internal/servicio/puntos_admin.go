package servicio

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"tramtai/internal/api"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Rutas de /api/v1/servicio/puntos, solo con el token de servicio. Trabajan sobre un archivo
// (la configuración global o, con ?proyecto=<raíz>, su .tramtai/local.yaml) y no sobre las instancias en marcha, así
// que también llegan a los puntos de una instancia inactiva o que no se pudieron abrir. Las reglas son las de la Web UI.

// archivoAdmin es el archivo de puntos que pide la petición.
func archivoAdmin(r *http.Request) (string, configuracion.Origen, error) {
	raiz := r.URL.Query().Get("proyecto")
	if raiz == "" {
		ruta, err := configuracion.RutaGlobal()
		return ruta, configuracion.OrigenGlobal, err
	}
	if !filepath.IsAbs(raiz) {
		return "", 0, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("proyecto debe ser una ruta absoluta"))
	}
	if _, err := os.Stat(filepath.Join(raiz, ".tramtai", "config.yaml")); err != nil {
		return "", 0, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("%s no es un proyecto de Tramtai (.tramtai/config.yaml)", raiz))
	}
	return filepath.Join(raiz, ".tramtai", "local.yaml"), configuracion.OrigenLocal, nil
}

// leerArchivoPuntos devuelve los puntos tal como están en el archivo (ninguno si no existe).
func leerArchivoPuntos(ruta string, origen configuracion.Origen) ([]ConfigPunto, error) {
	if _, err := os.Stat(ruta); os.IsNotExist(err) {
		return nil, nil
	}
	capa, err := configuracion.CargarCapa(ruta, origen)
	if err != nil {
		return nil, err
	}
	lista, _ := capa.Valores["puntos_acceso"].([]any)
	return configuracion.LeerPuntos(lista)
}

// guardadoAdmin recarga y responde con el archivo y los puntos del servicio, como tras recargar.
func (s *Servicio) guardadoAdmin(w http.ResponseWriter, ruta string, extra map[string]any) {
	respuesta := map[string]any{"archivo": ruta, "puntos_acceso": s.Recargar()}
	_ = s.persistirEstado() // anotado; los puntos ya están en su archivo
	for clave, valor := range extra {
		respuesta[clave] = valor
	}
	api.EscribirJSON(w, http.StatusOK, respuesta)
}

func noEstaEnArchivo(nombre string) error {
	return nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("no existe la API %s en este archivo", nombre))
}

// validarGuardado aplica las reglas para guardar un punto: el formato y que funcione (respaldo, agentes para apps).
func (s *Servicio) validarGuardado(p ConfigPunto) (ConfigPunto, error) {
	normalizado, err := normalizarPunto(p, "global", s.opc.Puerto)
	if err != nil {
		return p, err
	}
	p.Presupuesto = normalizado.Presupuesto // uno vacío se guarda como ninguno
	return p, validarPunto(p, s.existe)
}

func (s *Servicio) adminListarPuntos(w http.ResponseWriter, r *http.Request) {
	ruta, origen, err := archivoAdmin(r)
	if err == nil {
		var puntos []ConfigPunto
		if puntos, err = leerArchivoPuntos(ruta, origen); err == nil {
			api.EscribirJSON(w, http.StatusOK, map[string]any{"archivo": ruta, "puntos": append([]ConfigPunto{}, puntos...)})
			return
		}
	}
	api.EscribirError(w, r, err)
}

// adminCrearPunto añade el punto al archivo y le da una clave nueva, que solo se ve esta vez.
func (s *Servicio) adminCrearPunto(w http.ResponseWriter, r *http.Request) {
	var p ConfigPunto
	if err := api.LeerJSON(w, r, &p); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	ruta, origen, err := archivoAdmin(r)
	if err == nil {
		p.Activo = nil
		p, err = s.validarGuardado(p)
	}
	if err == nil {
		err = configuracion.EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
			if slices.ContainsFunc(lista, func(c ConfigPunto) bool { return c.Nombre == p.Nombre }) {
				return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("la API %s ya existe: usa tramtai acceso editar", p.Nombre))
			}
			return append(lista, p), nil
		})
	}
	var token string
	if err == nil {
		token, err = RotarTokenPunto(RutaTokenPunto(s.opc.Datos, p.Nombre))
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.guardadoAdmin(w, ruta, map[string]any{"token": token})
}

// adminGuardarPunto reemplaza el punto entero. Si solo cambia activo no pasa por las reglas: así se puede desactivar
// una API que ya tenía otro error, pero no guardar una que fallaría siempre.
func (s *Servicio) adminGuardarPunto(w http.ResponseWriter, r *http.Request) {
	nombre := r.PathValue("nombre")
	var p ConfigPunto
	if err := api.LeerJSON(w, r, &p); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if p.Nombre != nombre {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("el nombre del cuerpo (%s) no coincide con el de la ruta (%s)", p.Nombre, nombre)))
		return
	}
	ruta, origen, err := archivoAdmin(r)
	if err == nil {
		err = configuracion.EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
			i := slices.IndexFunc(lista, func(c ConfigPunto) bool { return c.Nombre == nombre })
			if i < 0 {
				return nil, noEstaEnArchivo(nombre)
			}
			if huellaSinActivo(p) != huellaSinActivo(lista[i]) {
				var err error
				if p, err = s.validarGuardado(p); err != nil {
					return nil, err
				}
			}
			lista[i] = p
			return lista, nil
		})
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.guardadoAdmin(w, ruta, nil)
}

// huellaSinActivo es el contenido del punto sin activo.
func huellaSinActivo(p ConfigPunto) string {
	p.Activo = nil
	datos, _ := json.Marshal(p)
	return string(datos)
}

// adminActivarPuntos activa o desactiva de una vez los puntos indicados o los de un grupo.
func (s *Servicio) adminActivarPuntos(w http.ResponseWriter, r *http.Request) {
	var peticion struct {
		Nombres []string `json:"nombres"`
		Grupo   string   `json:"grupo"`
		Activo  bool     `json:"activo"`
	}
	if err := api.LeerJSON(w, r, &peticion); err != nil {
		api.EscribirError(w, r, err)
		return
	}
	if (len(peticion.Nombres) == 0) == (peticion.Grupo == "") {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("indica los nombres o --grupo, no ambos")))
		return
	}
	ruta, origen, err := archivoAdmin(r)
	if err == nil {
		err = configuracion.EditarPuntos(ruta, origen, func(lista []ConfigPunto) ([]ConfigPunto, error) {
			activo := peticion.Activo
			if peticion.Grupo != "" {
				n := 0
				for i := range lista {
					if lista[i].Grupo == peticion.Grupo {
						lista[i].Activo, n = &activo, n+1
					}
				}
				if n == 0 {
					return nil, nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("ninguna API del grupo %s en este archivo", peticion.Grupo))
				}
				return lista, nil
			}
			for _, nombre := range peticion.Nombres {
				i := slices.IndexFunc(lista, func(c ConfigPunto) bool { return c.Nombre == nombre })
				if i < 0 {
					return nil, noEstaEnArchivo(nombre)
				}
				lista[i].Activo = &activo
			}
			return lista, nil
		})
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.guardadoAdmin(w, ruta, nil)
}

// adminEliminarPunto quita el punto del archivo con su clave y su consumo.
func (s *Servicio) adminEliminarPunto(w http.ResponseWriter, r *http.Request) {
	ruta, origen, err := archivoAdmin(r)
	if err == nil {
		err = s.eliminarPunto(ruta, origen, r.PathValue("nombre"))
	}
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.guardadoAdmin(w, ruta, nil)
}

// adminRotarToken regenera la clave del punto (ver RotarTokenPunto).
func (s *Servicio) adminRotarToken(w http.ResponseWriter, r *http.Request) {
	nombre := r.PathValue("nombre")
	if !NombrePuntoValido(nombre) {
		api.EscribirError(w, r, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("nombre no válido (minúsculas, números, _ y -; hasta 40)")))
		return
	}
	token, err := RotarTokenPunto(RutaTokenPunto(s.opc.Datos, nombre))
	if err != nil {
		api.EscribirError(w, r, err)
		return
	}
	s.Recargar()
	api.EscribirJSON(w, http.StatusOK, map[string]any{"token": token})
}
