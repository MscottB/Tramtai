// Package cliente habla con el servicio en marcha: lo descubre por su descriptor, comprueba que responde el
// mismo servicio y usa el token de servicio para administrar y operar las instancias. Lo usan la CLI y la TUI.
package cliente

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/contrato"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// ErrServicioInactivo indica que no hay un servicio de Tramtai en marcha para este usuario.
var ErrServicioInactivo = errors.New(idioma.T("el servicio de Tramtai no está en marcha (tramtai servicio iniciar --segundo-plano)"))

// Cliente es una conexión al servicio.
type Cliente struct {
	Base       string
	Descriptor contrato.Descriptor
	// Origen se manda en X-Tramtai-Cliente (terminal o cli) para que el historial diga de dónde llegó la petición.
	Origen string
	token  string
	http   *http.Client
}

// Conectar lee el descriptor y el token de servicio y comprueba con /healthz que responde ese mismo servicio,
// no otro proceso que ocupe el puerto.
func Conectar(datos string) (*Cliente, error) {
	d, err := contrato.LeerDescriptor(datos)
	if err != nil {
		return nil, ErrServicioInactivo
	}
	token, err := os.ReadFile(filepath.Join(datos, "token-servicio"))
	if err != nil {
		return nil, ErrServicioInactivo
	}
	c := &Cliente{Base: d.Base, Descriptor: d, token: strings.TrimSpace(string(token)), http: &http.Client{}}
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelar()
	var salud struct {
		Servicio string `json:"servicio"`
	}
	if err := c.pedir(ctx, "GET", "/healthz", "", nil, "", &salud); err != nil || salud.Servicio != d.Identificador {
		return nil, ErrServicioInactivo
	}
	return c, nil
}

// Lanzar inicia "<ejecutable> <args>" desacoplado de la terminal, sin esperar. Su salida va a
// servicio-salida.log, que se vacía en cada arranque y solo recoge un fallo grave.
func Lanzar(ejecutable, datos string, args ...string) error {
	if err := os.MkdirAll(datos, 0o700); err != nil {
		return err
	}
	registro, err := os.OpenFile(filepath.Join(datos, "servicio-salida.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(ejecutable, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = datos, registro, registro
	desacoplar(cmd)
	err = cmd.Start()
	registro.Close()
	if err != nil {
		return err
	}
	return cmd.Process.Release()
}

// IniciarEnSegundoPlano lanza "<ejecutable> servicio iniciar" desacoplado de la terminal y espera a que responda.
// El servicio escribe su registro en servicio.log, con tamaño máximo y copias.
func IniciarEnSegundoPlano(ejecutable, datos string) (*Cliente, error) {
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
	return nil, fmt.Errorf(idioma.T("el servicio no respondió; revisa %s"), rutaRegistro)
}

// pedir hace una petición con el token de servicio; instancia elige la instancia con Tramtai-Instancia.
func (c *Cliente) pedir(ctx context.Context, metodo, ruta, instancia string, cuerpo io.Reader, tipo string, destino any) error {
	req, err := http.NewRequestWithContext(ctx, metodo, c.Base+ruta, cuerpo)
	if err != nil {
		return err
	}
	// /healthz es pública y es la que comprueba la identidad: el token no sale hacia quien ocupe el puerto.
	if ruta != "/healthz" {
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
		}
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error.Codigo != "" {
			return nucleo.NuevoError(e.Error.Codigo, e.Error.Mensaje)
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

// Trabajos lista los trabajos recientes de una instancia.
func (c *Cliente) Trabajos(ctx context.Context, instancia string, limite int) ([]nucleo.Trabajo, error) {
	var r struct {
		Trabajos []nucleo.Trabajo `json:"trabajos"`
	}
	err := c.pedir(ctx, "GET", "/api/v1/trabajos?limite="+strconv.Itoa(limite), instancia, nil, "", &r)
	return r.Trabajos, err
}

// Trabajo devuelve un trabajo.
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

// Eliminar borra un trabajo terminado.
func (c *Cliente) Eliminar(ctx context.Context, instancia, id string) error {
	return c.pedir(ctx, "DELETE", "/api/v1/trabajos/"+url.PathEscape(id), instancia, nil, "", nil)
}

// Modelos devuelve los identificadores del catálogo de la instancia.
func (c *Cliente) Modelos(ctx context.Context, instancia string) ([]string, error) {
	var r struct {
		Modelos []struct {
			ID string `json:"id"`
		} `json:"modelos"`
	}
	if err := c.pedir(ctx, "GET", "/api/v1/modelos", instancia, nil, "", &r); err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range r.Modelos {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// Capacidades devuelve la matriz de capacidades de los agentes.
func (c *Cliente) Capacidades(ctx context.Context, instancia string) (map[string]any, error) {
	var r map[string]any
	return r, c.pedir(ctx, "GET", "/api/v1/capacidades", instancia, nil, "", &r)
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

// Eventos sigue los eventos SSE de un trabajo desde el número desde hasta el evento fin.
func (c *Cliente) Eventos(ctx context.Context, instancia, id string, desde int, alEvento func(numero int, tipo, datos string) error) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.Base+"/api/v1/trabajos/"+url.PathEscape(id)+"/eventos", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Tramtai-Instancia", instancia)
	if desde > 0 {
		req.Header.Set("Last-Event-ID", strconv.Itoa(desde))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(idioma.T("eventos: el servicio respondió %s"), resp.Status)
	}
	lector := bufio.NewScanner(resp.Body)
	lector.Buffer(make([]byte, 64<<10), 16<<20)
	numero, tipo, datos := 0, "", []string{}
	for lector.Scan() {
		linea := lector.Text()
		switch {
		case linea == "":
			if tipo == "fin" {
				return nil
			}
			if len(datos) > 0 {
				if err := alEvento(numero, tipo, strings.Join(datos, "\n")); err != nil {
					return err
				}
			}
			numero, tipo, datos = 0, "", nil
		case strings.HasPrefix(linea, "id: "):
			numero, _ = strconv.Atoi(linea[4:])
		case strings.HasPrefix(linea, "event: "):
			tipo = linea[7:]
		case strings.HasPrefix(linea, "data: "):
			datos = append(datos, linea[6:])
		}
	}
	return lector.Err()
}

// URLWeb devuelve una URL de un solo uso (válida un minuto) que abre la Web UI de la instancia sin escribir su token.
func (c *Cliente) URLWeb(ctx context.Context, instancia string) (string, error) {
	var r struct {
		URL string `json:"url"`
	}
	err := c.pedir(ctx, "POST", "/api/v1/servicio/sesiones", "", cuerpoJSON(map[string]string{"instancia": instancia}), "application/json", &r)
	return r.URL, err
}
