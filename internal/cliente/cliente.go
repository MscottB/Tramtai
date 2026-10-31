// Package cliente habla con el servicio en marcha: lo descubre por su descriptor, comprueba que responde el
// mismo servicio y usa el token de servicio para administrar y operar las instancias.
package cliente

import (
	"bytes"
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/contrato"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// ErrServicioInactivo indica que no hay un servicio de Tramtai en marcha para este usuario.
var ErrServicioInactivo = errors.New(idioma.T("el servicio de Tramtai no está en marcha (tramtai servicio iniciar --segundo-plano)"))

// ErrServicioAnterior indica que responde el servicio de una versión anterior de Tramtai (la 0.55 o previa), que no
// prueba su identidad con el reto: no se le manda el token ni se lanza otro servicio sobre los mismos datos, porque
// no toma el cerrojo de la carpeta de datos. DetenerAnterior lo detiene.
var ErrServicioAnterior = errors.New(idioma.T("hay un servicio de una versión anterior de Tramtai en marcha"))

// ErrServidorOtraVersion indica que, en modo servidor, responde el servicio del sistema de otra versión. Ese servicio no
// es de quien ejecuta la orden: no se detiene ni se le manda el token ni se lanza otro sobre sus datos; se actualiza con
// tramtai-servidor actualizar.
var ErrServidorOtraVersion = errors.New(idioma.T("el servicio del sistema es de otra versión"))

// otraVersion es ErrServidorOtraVersion con la versión que responde (la del descriptor si /healthz no la dice).
func (c *Cliente) otraVersion(version string) error {
	return fmt.Errorf("%w (%s): %s", ErrServidorOtraVersion, cmp.Or(version, c.Descriptor.Version, "?"),
		idioma.T("actualízalo con tramtai-servidor actualizar"))
}

// duenoPropio dice si el extremo local de una conexión es de un proceso de este usuario; las pruebas lo sustituyen.
var duenoPropio = procesos.MismoUsuario

// salud es la respuesta de /healthz: la prueba del reto y, en un servicio anterior a la 0.56 (o hacia el mismo usuario,
// para la CLI de esa versión), el identificador del servicio.
type salud struct {
	Prueba   string `json:"prueba"`
	Servicio string `json:"servicio"`
	Version  string `json:"version"`
}

// Cliente es una conexión al servicio.
type Cliente struct {
	Base       string
	Descriptor contrato.Descriptor
	// Origen se manda en X-Tramtai-Cliente (terminal o cli) para que el historial diga de dónde llegó la petición.
	Origen string
	token  string
	http   *http.Client
}

// BasePublica es la dirección que se muestra o se copia: la que las apps usan para llegar al servicio. Con TLS propio
// no es Base (el oyente local, efímero, que usa el propio cliente); sin TLS o con un servicio anterior es la misma.
func (c *Cliente) BasePublica() string {
	if c.Descriptor.BasePublica != "" {
		return c.Descriptor.BasePublica
	}
	return c.Base
}

// Conectar lee el descriptor y el token de servicio y comprueba con un reto en /healthz que responde un servicio que
// conoce el token, no otro proceso que ocupe el puerto; el token no sale hasta entonces. Si responde, sin reto, el
// servicio del descriptor, es el de una versión anterior: ErrServicioAnterior (en modo servidor, ErrServidorOtraVersion).
func Conectar(datos string) (*Cliente, error) {
	c, err := clienteDe(datos)
	if err != nil {
		return nil, err
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	reto := rand.Text()
	var s salud
	if err := c.pedir(ctx, "GET", "/healthz?reto="+reto, "", nil, "", &s); err != nil {
		return nil, ErrServicioInactivo
	}
	switch {
	case hmac.Equal([]byte(s.Prueba), []byte(contrato.PruebaIdentidad(c.token, c.Base, reto))):
		return c, nil
	case s.Prueba == "" && s.Servicio != "" && s.Servicio == c.Descriptor.Identificador:
		if carpetas.Servidor() {
			return nil, c.otraVersion(s.Version)
		}
		return nil, ErrServicioAnterior
	}
	return nil, ErrServicioInactivo
}

// clienteDe prepara un cliente con el descriptor y el token de servicio, sin comprobar quién responde.
func clienteDe(datos string) (*Cliente, error) {
	d, err := contrato.LeerDescriptor(datos)
	if err != nil {
		return nil, ErrServicioInactivo
	}
	token, err := os.ReadFile(filepath.Join(datos, "token-servicio"))
	if err != nil {
		return nil, ErrServicioInactivo
	}
	return &Cliente{Base: d.Base, Descriptor: d, token: strings.TrimSpace(string(token)), http: &http.Client{}}, nil
}

// plazoComprobacion acota la comprobación y la orden de detener de DetenerAnterior; no la espera al drenaje.
var plazoComprobacion = 10 * time.Second

// DetenerAnterior detiene el servicio de una versión anterior que responde en el puerto del descriptor y espera a que
// termine, como «tramtai servicio detener» de esa versión: deja terminar las peticiones en curso hasta su plazo de
// drenaje. Ese servicio no prueba que conoce el token, así que el token solo sale si el proceso que escucha en el
// puerto es de este mismo usuario del sistema; si no se puede comprobar, el error dice cómo detenerlo a mano.
func DetenerAnterior(datos string) error {
	c, err := clienteDe(datos)
	if err != nil {
		return err
	}
	ctx, cancelar := context.WithTimeout(context.Background(), plazoComprobacion)
	defer cancelar()
	responde, propio, version := c.anterior(ctx)
	if !responde {
		return nil // ya no está
	}
	if carpetas.Servidor() { // el servicio del sistema no se detiene desde aquí
		return c.otraVersion(version)
	}
	if !propio {
		return fmt.Errorf("%w (PID %d): %s", ErrServicioAnterior, c.Descriptor.PID,
			idioma.T("no se pudo comprobar que sea de este usuario; detenlo con «tramtai servicio detener» de esa versión o termina ese proceso, y vuelve a intentarlo"))
	}
	if err := c.Detener(ctx); err != nil {
		return err
	}
	for limite := time.Now().Add(PlazoDrenaje() + 15*time.Second); time.Now().Before(limite); {
		// Sin el ctx de la comprobación: vencería antes que el drenaje y daría por detenido un servicio que aún responde.
		if responde, _, _ := c.anterior(context.Background()); !responde {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New(idioma.T("el servicio de la versión anterior no confirmó la detención"))
}

// anterior consulta /healthz como la versión anterior: responde dice que en el puerto está, sin reto, el servicio del
// descriptor; propio, que el proceso que escucha es de este usuario, comprobado sobre la misma conexión (en modo
// servidor lo es siempre: el servicio del sistema corre con su propia cuenta); version, la que dice /healthz.
func (c *Cliente) anterior(ctx context.Context) (responde, propio bool, version string) {
	ctx, cancelar := context.WithTimeout(ctx, 2*time.Second)
	defer cancelar()
	traza := &httptrace.ClientTrace{GotConn: func(i httptrace.GotConnInfo) {
		local, err1 := netip.ParseAddrPort(i.Conn.LocalAddr().String())
		remoto, err2 := netip.ParseAddrPort(i.Conn.RemoteAddr().String())
		// El extremo del servidor: su dirección local es la remota de esta conexión.
		// En modo servidor propio es siempre verdadero: no sirve para decidir si se envía el token.
		propio = carpetas.Servidor() || err1 == nil && err2 == nil && duenoPropio(netip.AddrPortFrom(remoto.Addr().Unmap(), remoto.Port()),
			netip.AddrPortFrom(local.Addr().Unmap(), local.Port()))
	}}
	var s salud
	err := c.pedir(httptrace.WithClientTrace(ctx, traza), "GET", "/healthz", "", nil, "", &s)
	return err == nil && s.Prueba == "" && s.Servicio != "" && s.Servicio == c.Descriptor.Identificador, propio, s.Version
}

// PlazoDrenaje es servicio.plazo_drenaje_segundos de la configuración global: lo que el servicio espera a las
// peticiones en curso al detenerse (30 s si no se puede leer).
func PlazoDrenaje() time.Duration {
	if c, err := configuracion.CargarConfiguracion("", configuracion.Capa{Origen: configuracion.OrigenInterno}); err == nil {
		return time.Duration(c.Entero("servicio.plazo_drenaje_segundos")) * time.Second
	}
	return 30 * time.Second
}

// Lanzar inicia "<ejecutable> <args>" desacoplado de la terminal, sin esperar. Su salida va a
// servicio-salida.log, que solo recoge un fallo grave; el del arranque anterior pasa a servicio-salida.log.1, para que
// reintentar no borre lo que lo explicaba.
func Lanzar(ejecutable, datos string, args ...string) error {
	if err := os.MkdirAll(datos, 0o700); err != nil {
		return err
	}
	ruta := filepath.Join(datos, "servicio-salida.log")
	if info, err := os.Stat(ruta); err == nil && info.Size() > 0 {
		_ = os.Rename(ruta, ruta+".1") // si no se puede (otro proceso lo tiene abierto), se vacía como antes
	}
	registro, err := os.OpenFile(ruta, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := ordenLanzar(ejecutable, datos, args)
	cmd.Stdout, cmd.Stderr = registro, registro
	err = cmd.Start()
	registro.Close()
	if err != nil {
		return err
	}
	return cmd.Process.Release()
}

// ordenLanzar arma el proceso de Lanzar. En modo servidor el hijo lleva TRAMTAI_SERVIDOR=1: sin eso calcularía su raíz
// como la del usuario y arrancaría un servicio sobre otros datos.
func ordenLanzar(ejecutable, datos string, args []string) *exec.Cmd {
	cmd := exec.Command(ejecutable, args...)
	cmd.Dir = datos
	if carpetas.Servidor() {
		cmd.Env = append(os.Environ(), "TRAMTAI_SERVIDOR=1")
	}
	desacoplar(cmd)
	return cmd
}

// ErrServidorNoEnMarcha indica que, en modo servidor, no responde el servicio del sistema y esta orden no lo inicia: lo
// gestiona el sistema.
var ErrServidorNoEnMarcha = errors.New(idioma.T("el servicio del sistema no está en marcha y lo gestiona el sistema: inícialo con tramtai-servidor reiniciar o con systemctl start tramtai (en Windows, con el administrador de servicios)"))

// IniciarEnSegundoPlano lanza "<ejecutable> servicio iniciar" desacoplado de la terminal y espera a que responda.
// El servicio escribe su registro en servicio.log, con tamaño máximo y copias. Si responde el servicio de una versión
// anterior (tras actualizar), primero lo detiene: no toma el cerrojo de la carpeta de datos y el nuevo arrancaría
// sobre sus peticiones.
func IniciarEnSegundoPlano(ejecutable, datos string) (*Cliente, error) {
	c, err := Conectar(datos)
	if errors.Is(err, ErrServidorOtraVersion) {
		return nil, err
	}
	if carpetas.Servidor() { // el servicio del sistema no lo lanza esta orden
		if err == nil {
			return c, nil
		}
		return nil, ErrServidorNoEnMarcha
	}
	if errors.Is(err, ErrServicioAnterior) {
		if err := DetenerAnterior(datos); err != nil {
			return nil, err
		}
	}
	rutaRegistro := filepath.Join(datos, "servicio.log")
	if err := Lanzar(ejecutable, datos, "servicio", "iniciar", "--registro", rutaRegistro); err != nil {
		return nil, err
	}
	limite := time.Now().Add(20 * time.Second)
	for time.Now().Before(limite) {
		if c, err := Conectar(datos); err == nil {
			return c, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf(idioma.T("el servicio no respondió; revisa %s y %s"), rutaRegistro, filepath.Join(datos, "servicio-salida.log"))
}

// pedir hace una petición con el token de servicio; instancia elige la instancia con Tramtai-Instancia.
func (c *Cliente) pedir(ctx context.Context, metodo, ruta, instancia string, cuerpo io.Reader, tipo string, destino any) error {
	req, err := http.NewRequestWithContext(ctx, metodo, c.Base+ruta, cuerpo)
	if err != nil {
		return err
	}
	// /healthz es pública y es la que comprueba la identidad: el token no sale hacia quien ocupe el puerto.
	if !strings.HasPrefix(ruta, "/healthz") {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if instancia != "" {
		req.Header.Set("Tramtai-Instancia", instancia)
	}
	if c.Origen != "" {
		req.Header.Set("X-Tramtai-Cliente", c.Origen)
	}
	if tipo != "" {
		req.Header.Set("Content-Type", tipo)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct {
			Error struct {
				Codigo  nucleo.CodigoError `json:"codigo"`
				Mensaje string             `json:"mensaje"`
			} `json:"error"`
			// Lo que traen las dos preguntas de abrir un proyecto.
			Claves   []string `json:"claves"`
			Movido   string   `json:"movido"`
			Proyecto string   `json:"proyecto"`
		}
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error.Codigo != "" {
			err := nucleo.NuevoError(e.Error.Codigo, e.Error.Mensaje)
			switch e.Error.Codigo {
			case nucleo.CodigoConfianzaRequerida:
				return &ErrorConfianza{Err: err, Claves: e.Claves, Movido: e.Movido}
			case nucleo.CodigoDentroDeProyecto:
				return &ErrorDentroDeProyecto{Err: err, Proyecto: e.Proyecto}
			}
			return err
		}
		return fmt.Errorf(idioma.T("el servicio respondió %s"), resp.Status)
	}
	if destino == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(destino)
}

func cuerpoJSON(v any) io.Reader {
	datos, _ := json.Marshal(v)
	return bytes.NewReader(datos)
}

// Estado devuelve el estado del servicio, sus instancias y sus puntos de acceso.
func (c *Cliente) Estado(ctx context.Context) (contrato.Estado, error) {
	var e contrato.Estado
	return e, c.pedir(ctx, "GET", "/api/v1/servicio", "", nil, "", &e)
}

// Activar activa la instancia global (raiz vacía) o la del proyecto en raiz.
func (c *Cliente) Activar(ctx context.Context, ambito, raiz string) (contrato.ActivacionInstancia, error) {
	var a contrato.ActivacionInstancia
	return a, c.pedir(ctx, "POST", "/api/v1/servicio/instancias", "", cuerpoJSON(map[string]string{"ambito": ambito, "raiz": raiz}), "application/json", &a)
}

// Desactivar detiene una instancia sin afectar a las demás.
func (c *Cliente) Desactivar(ctx context.Context, id string) error {
	return c.pedir(ctx, "DELETE", "/api/v1/servicio/instancias/"+url.PathEscape(id), "", nil, "", nil)
}

// ConectarAgente conecta o desconecta un agente en todos los espacios.
func (c *Cliente) ConectarAgente(ctx context.Context, agente string, conectado bool) error {
	return c.pedir(ctx, "POST", "/api/v1/servicio/agentes/"+url.PathEscape(agente)+"/conexion", "", cuerpoJSON(map[string]bool{"conectado": conectado}), "application/json", nil)
}

// Espacios devuelve todos los espacios de trabajo conocidos, también los detenidos.
func (c *Cliente) Espacios(ctx context.Context) ([]contrato.Espacio, error) {
	var r struct {
		Espacios []contrato.Espacio `json:"espacios"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/servicio/espacios", "", nil, "", &r)
	return r.Espacios, err
}

// Olvidar borra un espacio y sus datos en Tramtai (nunca la carpeta del proyecto); nombre lo confirma.
func (c *Cliente) Olvidar(ctx context.Context, id, nombre string) error {
	return c.pedir(ctx, "DELETE", "/api/v1/servicio/espacios/"+url.PathEscape(id), "", cuerpoJSON(map[string]string{"nombre": nombre}), "application/json", nil)
}

// ErrorConfianza es la pregunta de abrir un proyecto que ya traía .tramtai/: hay que decidir si se confía en él. Lleva
// las claves de seguridad de su local.yaml y, si parece uno que estaba en otra carpeta, dónde estaba.
type ErrorConfianza struct {
	Err    *nucleo.Error
	Claves []string
	Movido string
}

func (e *ErrorConfianza) Error() string { return e.Err.Error() }
func (e *ErrorConfianza) Unwrap() error { return e.Err }

// ErrorDentroDeProyecto es la pregunta de abrir una carpeta que está dentro de otro proyecto (Proyecto): hay que
// elegir cuál abrir.
type ErrorDentroDeProyecto struct {
	Err      *nucleo.Error
	Proyecto string
}

func (e *ErrorDentroDeProyecto) Error() string { return e.Err.Error() }
func (e *ErrorDentroDeProyecto) Unwrap() error { return e.Err }

// ActivarEspacio vuelve a abrir un espacio detenido por su id.
func (c *Cliente) ActivarEspacio(ctx context.Context, id string) (contrato.Espacio, error) {
	var e contrato.Espacio
	return e, c.pedir(ctx, "POST", "/api/v1/servicio/espacios/"+url.PathEscape(id)+"/activar", "", nil, "", &e)
}

// DetenerEspacio detiene un espacio; sus datos y su configuración se quedan.
func (c *Cliente) DetenerEspacio(ctx context.Context, id string) error {
	return c.pedir(ctx, "POST", "/api/v1/servicio/espacios/"+url.PathEscape(id)+"/detener", "", nil, "", nil)
}

// Confiar fija si se confía en el proyecto de un espacio: con confianza, su local.yaml puede traer las claves de
// seguridad.
func (c *Cliente) Confiar(ctx context.Context, id string, confiar bool) error {
	return c.pedir(ctx, "POST", "/api/v1/servicio/espacios/"+url.PathEscape(id)+"/confianza", "", cuerpoJSON(map[string]bool{"confiar": confiar}), "application/json", nil)
}

// Ubicar asocia un espacio cuya carpeta no se encuentra con ruta y lo activa; conserva su id, su clave y su historial.
func (c *Cliente) Ubicar(ctx context.Context, id, ruta string) (contrato.Espacio, error) {
	var e contrato.Espacio
	return e, c.pedir(ctx, "POST", "/api/v1/servicio/espacios/"+url.PathEscape(id)+"/ubicacion", "", cuerpoJSON(map[string]string{"ruta": ruta}), "application/json", &e)
}

// AbrirProyecto abre (y crea si falta) el proyecto de la carpeta raiz como un espacio activo. Si hay que decidir, el
// error es un *ErrorConfianza (confiar nil) o un *ErrorDentroDeProyecto (crearAqui false).
func (c *Cliente) AbrirProyecto(ctx context.Context, raiz string, confiar *bool, crearAqui bool) (contrato.Espacio, error) {
	var e contrato.Espacio
	cuerpo := cuerpoJSON(map[string]any{"raiz": raiz, "confiar": confiar, "crear_aqui": crearAqui})
	return e, c.pedir(ctx, "POST", "/api/v1/servicio/proyectos", "", cuerpo, "application/json", &e)
}

// CambiarAutenticacion pasa un agente que admite la suscripción a suscripcion o clave_api, en todos los espacios.
func (c *Cliente) CambiarAutenticacion(ctx context.Context, agente, modo string) error {
	return c.pedir(ctx, "PUT", "/api/v1/servicio/agentes/"+url.PathEscape(agente)+"/autenticacion", "", cuerpoJSON(map[string]string{"modo": modo}), "application/json", nil)
}

// Configuracion devuelve la configuración efectiva de una instancia con el origen de cada valor.
func (c *Cliente) Configuracion(ctx context.Context, id string) ([]contrato.EntradaConfiguracion, error) {
	var r struct {
		Configuracion []contrato.EntradaConfiguracion `json:"configuracion"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/servicio/instancias/"+url.PathEscape(id)+"/configuracion", "", nil, "", &r)
	return r.Configuracion, err
}

// RotarClave regenera la clave de una instancia activa; la anterior deja de valer al momento.
func (c *Cliente) RotarClave(ctx context.Context, instancia string) (string, error) {
	var r struct {
		Token string `json:"token"`
	}
	err := c.pedir(ctx, "POST", "/api/v1/instancia/token", instancia, nil, "", &r)
	return r.Token, err
}

// Recargar aplica los cambios de puntos de acceso sin reiniciar.
func (c *Cliente) Recargar(ctx context.Context) ([]contrato.EstadoPunto, error) {
	var r struct {
		Puntos []contrato.EstadoPunto `json:"puntos_acceso"`
	}
	err := c.pedir(ctx, "POST", "/api/v1/servicio/recargar", "", nil, "", &r)
	return r.Puntos, err
}

// ReiniciarPresupuesto pone a cero el consumo del día y del mes de un punto de acceso, o de la instancia sin nombre.
func (c *Cliente) ReiniciarPresupuesto(ctx context.Context, instancia, nombre string) error {
	ruta := "/api/v1/instancia/presupuesto/reiniciar"
	if nombre != "" {
		ruta = "/api/v1/puntos/" + url.PathEscape(nombre) + "/presupuesto/reiniciar"
	}
	return c.pedir(ctx, "POST", ruta, instancia, nil, "", nil)
}

// ActivarPunto activa o desactiva un punto de acceso; se guarda en su configuración y se aplica sin reiniciar.
func (c *Cliente) ActivarPunto(ctx context.Context, instancia, nombre string, activo bool) error {
	return c.pedir(ctx, "PATCH", "/api/v1/puntos/"+url.PathEscape(nombre), instancia, cuerpoJSON(map[string]bool{"activo": activo}), "application/json", nil)
}

// CrearPunto crea un punto de acceso y devuelve su estado y su clave, que solo se ve esta vez.
func (c *Cliente) CrearPunto(ctx context.Context, instancia string, p contrato.ConfigPunto) (contrato.EstadoPunto, string, error) {
	var r struct {
		Punto contrato.EstadoPunto `json:"punto_acceso"`
		Token string               `json:"token"`
	}
	err := c.pedir(ctx, "POST", "/api/v1/puntos", instancia, cuerpoJSON(p), "application/json", &r)
	return r.Punto, r.Token, err
}

// EliminarPunto elimina un punto de acceso con su clave y su consumo.
func (c *Cliente) EliminarPunto(ctx context.Context, instancia, nombre string) error {
	return c.pedir(ctx, "DELETE", "/api/v1/puntos/"+url.PathEscape(nombre), instancia, nil, "", nil)
}

// RotarTokenPunto regenera la clave de un punto de acceso; solo se ve esta vez.
func (c *Cliente) RotarTokenPunto(ctx context.Context, instancia, nombre string) (string, error) {
	var r struct {
		Token string `json:"token"`
	}
	err := c.pedir(ctx, "POST", "/api/v1/puntos/"+url.PathEscape(nombre)+"/token", instancia, nil, "", &r)
	return r.Token, err
}

// GuardadoEnArchivo es lo que devuelven los cambios de puntos en un archivo: dónde quedó y los puntos del servicio
// tras recargar (y la clave, al crear).
type GuardadoEnArchivo struct {
	Archivo string                 `json:"archivo"`
	Puntos  []contrato.EstadoPunto `json:"puntos_acceso"`
	Token   string                 `json:"token"`
}

// rutaArchivo es una ruta de /api/v1/servicio/puntos sobre el archivo global (proyecto vacío) o el .tramtai/local.yaml
// del proyecto en esa raíz.
func rutaArchivo(sufijo, proyecto string) string {
	ruta := "/api/v1/servicio/puntos" + sufijo
	if proyecto != "" {
		ruta += "?proyecto=" + url.QueryEscape(proyecto)
	}
	return ruta
}

// PuntosEnArchivo devuelve los puntos tal como están en el archivo, también los de una instancia inactiva.
func (c *Cliente) PuntosEnArchivo(ctx context.Context, proyecto string) (string, []contrato.ConfigPunto, error) {
	var r struct {
		Archivo string                 `json:"archivo"`
		Puntos  []contrato.ConfigPunto `json:"puntos"`
	}
	err := c.pedir(ctx, "GET", rutaArchivo("", proyecto), "", nil, "", &r)
	return r.Archivo, r.Puntos, err
}

// CrearPuntoEnArchivo añade un punto al archivo con las reglas del servicio; la clave nueva solo se ve esta vez.
func (c *Cliente) CrearPuntoEnArchivo(ctx context.Context, proyecto string, p contrato.ConfigPunto) (GuardadoEnArchivo, error) {
	var g GuardadoEnArchivo
	return g, c.pedir(ctx, "POST", rutaArchivo("", proyecto), "", cuerpoJSON(p), "application/json", &g)
}

// GuardarPuntoEnArchivo reemplaza el punto entero, con su nombre de siempre.
func (c *Cliente) GuardarPuntoEnArchivo(ctx context.Context, proyecto string, p contrato.ConfigPunto) (GuardadoEnArchivo, error) {
	var g GuardadoEnArchivo
	return g, c.pedir(ctx, "PUT", rutaArchivo("/"+url.PathEscape(p.Nombre), proyecto), "", cuerpoJSON(p), "application/json", &g)
}

// ActivarPuntosEnArchivo activa o desactiva de una vez los puntos indicados o, con grupo, los de ese grupo.
func (c *Cliente) ActivarPuntosEnArchivo(ctx context.Context, proyecto string, nombres []string, grupo string, activo bool) (GuardadoEnArchivo, error) {
	var g GuardadoEnArchivo
	cuerpo := cuerpoJSON(map[string]any{"nombres": nombres, "grupo": grupo, "activo": activo})
	return g, c.pedir(ctx, "POST", rutaArchivo("/activo", proyecto), "", cuerpo, "application/json", &g)
}

// EliminarPuntoEnArchivo quita el punto del archivo y borra su clave.
func (c *Cliente) EliminarPuntoEnArchivo(ctx context.Context, proyecto, nombre string) (GuardadoEnArchivo, error) {
	var g GuardadoEnArchivo
	return g, c.pedir(ctx, "DELETE", rutaArchivo("/"+url.PathEscape(nombre), proyecto), "", nil, "", &g)
}

// RotarTokenEnArchivo regenera la clave de un punto; solo se ve esta vez.
func (c *Cliente) RotarTokenEnArchivo(ctx context.Context, nombre string) (string, error) {
	var g GuardadoEnArchivo
	err := c.pedir(ctx, "POST", rutaArchivo("/"+url.PathEscape(nombre)+"/token", ""), "", nil, "", &g)
	return g.Token, err
}

// FijarPuerto guarda servicio.puerto (0 lo deja como está) y servicio.puerto_automatico a través del servicio en
// marcha; se aplica al reiniciar.
func (c *Cliente) FijarPuerto(ctx context.Context, puerto int, automatico bool) error {
	return c.pedir(ctx, "PUT", "/api/v1/servicio/puerto", "", cuerpoJSON(map[string]any{"puerto": puerto, "automatico": automatico}), "application/json", nil)
}

// Detener pide al servicio que se detenga con todas sus instancias.
func (c *Cliente) Detener(ctx context.Context) error {
	return c.pedir(ctx, "POST", "/api/v1/servicio/detener", "", nil, "", nil)
}

// Conversor devuelve el estado del conversor de archivos (markitdown).
func (c *Cliente) Conversor(ctx context.Context, instancia string) (contrato.EstadoConversor, error) {
	var e contrato.EstadoConversor
	return e, c.pedir(ctx, "GET", "/api/v1/conversor", instancia, nil, "", &e)
}

// PrepararConversor lanza la preparación del conversor (o devuelve la que está en curso).
func (c *Cliente) PrepararConversor(ctx context.Context, instancia string) (contrato.EstadoConversor, error) {
	var e contrato.EstadoConversor
	return e, c.pedir(ctx, "POST", "/api/v1/conversor/preparar", instancia, nil, "", &e)
}

// Herramientas devuelve las herramientas de terceros (Claude Code, Codex…) vistas desde el espacio.
func (c *Cliente) Herramientas(ctx context.Context, inst string) ([]contrato.EstadoHerramienta, error) {
	return c.herramientas(ctx, "GET", "/api/v1/herramientas", inst, nil)
}

// ActivarHerramientas apunta a Tramtai las herramientas indicadas (o todas las detectadas) y devuelve la lista.
func (c *Cliente) ActivarHerramientas(ctx context.Context, inst string, p contrato.ActivarHerramientas) ([]contrato.EstadoHerramienta, error) {
	return c.herramientas(ctx, "POST", "/api/v1/herramientas/activar", inst, cuerpoJSON(p))
}

// DesactivarHerramientas quita Tramtai de las herramientas indicadas (o de todas) y detiene sus APIs.
func (c *Cliente) DesactivarHerramientas(ctx context.Context, inst string, ids []string, todas bool) ([]contrato.EstadoHerramienta, error) {
	return c.herramientas(ctx, "POST", "/api/v1/herramientas/desactivar", inst, cuerpoJSON(map[string]any{"ids": ids, "todas": todas}))
}

func (c *Cliente) herramientas(ctx context.Context, metodo, ruta, inst string, cuerpo io.Reader) ([]contrato.EstadoHerramienta, error) {
	var r struct {
		Herramientas []contrato.EstadoHerramienta `json:"herramientas"`
	}
	err := c.pedir(ctx, metodo, ruta, inst, cuerpo, "application/json", &r)
	return r.Herramientas, err
}

// OlvidarHerramienta desactiva la herramienta y borra su clave, su API y lo que Tramtai sabía de ella.
func (c *Cliente) OlvidarHerramienta(ctx context.Context, inst, id string) error {
	return c.pedir(ctx, "POST", "/api/v1/herramientas/"+url.PathEscape(id)+"/olvidar", inst, nil, "", nil)
}

// RestaurarHerramienta devuelve el archivo de la herramienta a como estaba antes de activarla; copia es la copia usada.
func (c *Cliente) RestaurarHerramienta(ctx context.Context, inst, id string) (copia string, err error) {
	var r struct {
		Copia string `json:"copia"`
	}
	err = c.pedir(ctx, "POST", "/api/v1/herramientas/"+url.PathEscape(id)+"/restaurar", inst, nil, "", &r)
	return r.Copia, err
}

func (c *Cliente) Trabajo(ctx context.Context, instancia, id string) (nucleo.Trabajo, error) {
	var t nucleo.Trabajo
	return t, c.pedir(ctx, "GET", "/api/v1/trabajos/"+url.PathEscape(id), instancia, nil, "", &t)
}

// Crear envía una solicitud y devuelve el trabajo aceptado.
func (c *Cliente) Crear(ctx context.Context, instancia string, s nucleo.SolicitudEjecucion) (nucleo.Trabajo, error) {
	var t nucleo.Trabajo
	return t, c.pedir(ctx, "POST", "/api/v1/trabajos", instancia, cuerpoJSON(s), "application/json", &t)
}

// Accion ejecuta cancelar o reintentar sobre un trabajo.
func (c *Cliente) Accion(ctx context.Context, instancia, id, accion string) (nucleo.Trabajo, error) {
	var t nucleo.Trabajo
	return t, c.pedir(ctx, "POST", "/api/v1/trabajos/"+url.PathEscape(id)+"/"+accion, instancia, nil, "", &t)
}

// Corregir reintenta un trabajo que no cumplía el esquema pidiendo al mismo agente que corrija la respuesta que conservó
// (reintentar con {"corregir": true}); es otro trabajo. Sin salida que corregir, el servicio responde correccion_no_disponible.
func (c *Cliente) Corregir(ctx context.Context, instancia, id string) (nucleo.Trabajo, error) {
	var t nucleo.Trabajo
	return t, c.pedir(ctx, "POST", "/api/v1/trabajos/"+url.PathEscape(id)+"/reintentar", instancia, cuerpoJSON(map[string]bool{"corregir": true}), "application/json", &t)
}

// Eliminar borra un trabajo terminado.
func (c *Cliente) Eliminar(ctx context.Context, instancia, id string) error {
	return c.pedir(ctx, "DELETE", "/api/v1/trabajos/"+url.PathEscape(id), instancia, nil, "", nil)
}

func (c *Cliente) Catalogo(ctx context.Context, instancia string) ([]contrato.Modelo, error) {
	var r struct {
		Modelos []contrato.Modelo `json:"modelos"`
	}
	return r.Modelos, c.pedir(ctx, "GET", "/api/v1/modelos", instancia, nil, "", &r)
}

// Modelos devuelve los identificadores del catálogo de la instancia.
func (c *Cliente) Modelos(ctx context.Context, instancia string) ([]string, error) {
	lista, err := c.Catalogo(ctx, instancia)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range lista {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// Capacidades devuelve los agentes con su estado, los formatos y los límites de la instancia.
func (c *Cliente) Capacidades(ctx context.Context, instancia string) (contrato.Capacidades, error) {
	var r contrato.Capacidades
	return r, c.pedir(ctx, "GET", "/api/v1/capacidades", instancia, nil, "", &r)
}

// PrimerosPasos devuelve los Primeros pasos de Inicio, ya calculados, y los agentes que comprobó para ello.
func (c *Cliente) PrimerosPasos(ctx context.Context, instancia string) (contrato.PrimerosPasos, error) {
	var r contrato.PrimerosPasos
	return r, c.pedir(ctx, "GET", "/api/v1/instancia/pasos", instancia, nil, "", &r)
}

// SubirCarga sube un archivo a la instancia.
func (c *Cliente) SubirCarga(ctx context.Context, instancia, ruta string) (documentos.Carga, error) {
	var carga documentos.Carga
	archivo, err := os.Open(ruta)
	if err != nil {
		return carga, err
	}
	defer archivo.Close()
	lector, escritor := io.Pipe()
	m := multipart.NewWriter(escritor)
	go func() {
		parte, err := m.CreateFormFile("archivo", filepath.Base(ruta))
		if err == nil {
			_, err = io.Copy(parte, archivo)
		}
		if err == nil {
			err = m.Close()
		}
		escritor.CloseWithError(err)
	}()
	return carga, c.pedir(ctx, "POST", "/api/v1/cargas", instancia, lector, m.FormDataContentType(), &carga)
}

// URLWeb devuelve una URL de un solo uso (válida un minuto) que abre la Web UI de la instancia sin escribir su token.
func (c *Cliente) URLWeb(ctx context.Context, instancia string) (string, error) {
	var r struct {
		URL string `json:"url"`
	}
	err := c.pedir(ctx, "POST", "/api/v1/servicio/sesiones", "", cuerpoJSON(map[string]string{"instancia": instancia}), "application/json", &r)
	return r.URL, err
}
