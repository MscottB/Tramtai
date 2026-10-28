package cliente

import (
	"context"
	"net/url"

	"tramtai/internal/contrato"
)

// Ajustes del servicio y del equipo, por la administración (token de servicio): lo que Ajustes de la terminal comparte
// con la Web UI sin pasar por /ui/.

// AjustesServicio devuelve el puerto (el que se usa y el configurado) y el arranque con el sistema.
func (c *Cliente) AjustesServicio(ctx context.Context) (contrato.AjustesServicio, error) {
	var a contrato.AjustesServicio
	return a, c.pedir(ctx, "GET", "/api/v1/servicio/ajustes", "", nil, "", &a)
}

// Autoarranque activa o desactiva que Tramtai arranque al iniciar sesión; modo es "app" (la app en la bandeja) o
// "servicio" (solo el servicio).
func (c *Cliente) Autoarranque(ctx context.Context, activo bool, modo string) error {
	return c.pedir(ctx, "PUT", "/api/v1/servicio/autoarranque", "", cuerpoJSON(map[string]any{"activo": activo, "modo": modo}), "application/json", nil)
}

// Claves dice qué claves de proveedor tiene el servicio, sin sus valores.
func (c *Cliente) Claves(ctx context.Context) ([]contrato.ClaveGuardada, error) {
	var lista []contrato.ClaveGuardada
	return lista, c.pedir(ctx, "GET", "/api/v1/servicio/claves", "", nil, "", &lista)
}

// GuardarClave entrega una clave al servicio, que la guarda en su propio almacén (el servidor de Windows: el almacén es
// el de la cuenta del servicio, no el de quien administra).
func (c *Cliente) GuardarClave(ctx context.Context, nombre, valor string) error {
	return c.pedir(ctx, "PUT", "/api/v1/servicio/claves/"+url.PathEscape(nombre), "", cuerpoJSON(map[string]string{"valor": valor}), "application/json", nil)
}

// BorrarClave quita una clave del almacén del servicio.
func (c *Cliente) BorrarClave(ctx context.Context, nombre string) error {
	return c.pedir(ctx, "DELETE", "/api/v1/servicio/claves/"+url.PathEscape(nombre), "", nil, "", nil)
}

// FijarTOTP entrega el secreto TOTP del administrador al servicio, que lo guarda en su propio almacén (Windows).
func (c *Cliente) FijarTOTP(ctx context.Context, secreto string) error {
	return c.pedir(ctx, "PUT", "/api/v1/servicio/totp", "", cuerpoJSON(map[string]string{"secreto": secreto}), "application/json", nil)
}

// QuitarTOTP quita el secreto TOTP del administrador (el servicio lo rechaza con suscripción personal).
func (c *Cliente) QuitarTOTP(ctx context.Context) error {
	return c.pedir(ctx, "DELETE", "/api/v1/servicio/totp", "", nil, "", nil)
}

// Actualizacion devuelve el estado de las actualizaciones; con ahora, el servicio consulta GitHub en el momento.
func (c *Cliente) Actualizacion(ctx context.Context, ahora bool) (contrato.EstadoActualizacion, error) {
	ruta := "/api/v1/servicio/actualizacion"
	if ahora {
		ruta += "?ahora=1"
	}
	var e contrato.EstadoActualizacion
	return e, c.pedir(ctx, "GET", ruta, "", nil, "", &e)
}

// BuscarActualizaciones activa o desactiva la búsqueda automática y devuelve el estado.
func (c *Cliente) BuscarActualizaciones(ctx context.Context, buscar bool) (contrato.EstadoActualizacion, error) {
	var e contrato.EstadoActualizacion
	return e, c.pedir(ctx, "PUT", "/api/v1/servicio/actualizacion", "", cuerpoJSON(map[string]bool{"buscar": buscar}), "application/json", &e)
}

// Actualizar empieza a descargar e instalar la versión nueva en el servicio (o se une a la que ya está en curso) y
// devuelve su número; el avance se sigue con ProgresoActualizacion.
func (c *Cliente) Actualizar(ctx context.Context) (string, error) {
	var r struct {
		Version string `json:"version"`
	}
	return r.Version, c.pedir(ctx, "POST", "/api/v1/servicio/actualizar", "", nil, "", &r)
}

// ProgresoActualizacion devuelve el avance de la actualización en curso (fase vacía si no hay ninguna).
func (c *Cliente) ProgresoActualizacion(ctx context.Context) (contrato.ProgresoActualizacion, error) {
	var p contrato.ProgresoActualizacion
	return p, c.pedir(ctx, "GET", "/api/v1/servicio/actualizacion/progreso", "", nil, "", &p)
}

// DecidirInstalacion instala ya (true) o cancela (false) una actualización que espera en la cuenta atrás.
func (c *Cliente) DecidirInstalacion(ctx context.Context, instalarYa bool) error {
	ruta := "/api/v1/servicio/actualizar/cancelar"
	if instalarYa {
		ruta = "/api/v1/servicio/actualizar/instalar"
	}
	return c.pedir(ctx, "POST", ruta, "", nil, "", nil)
}

// Novedades devuelve lo que trae cada versión después de desde y hasta hasta (sin hasta, la instalada).
func (c *Cliente) Novedades(ctx context.Context, desde, hasta string) ([]contrato.VersionNueva, error) {
	q := url.Values{"desde": {desde}}
	if hasta != "" {
		q.Set("hasta", hasta)
	}
	var r struct {
		Versiones []contrato.VersionNueva `json:"versiones"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/servicio/actualizacion/novedades?"+q.Encode(), "", nil, "", &r)
	return r.Versiones, err
}

// Reiniciar reinicia el servicio para aplicar el puerto (si AjustesServicio dice que es reiniciable) y devuelve la dirección
// en la que escuchará el nuevo; el descriptor de la carpeta de datos lleva a él.
func (c *Cliente) Reiniciar(ctx context.Context) (string, error) {
	var r struct {
		URL string `json:"url"`
	}
	return r.URL, c.pedir(ctx, "POST", "/api/v1/servicio/reiniciar", "", nil, "", &r)
}

// Acerca devuelve los datos de «Acerca de Tramtai», con las carpetas de este equipo.
func (c *Cliente) Acerca(ctx context.Context) (contrato.Acerca, error) {
	var a contrato.Acerca
	return a, c.pedir(ctx, "GET", "/api/v1/servicio/acerca", "", nil, "", &a)
}
