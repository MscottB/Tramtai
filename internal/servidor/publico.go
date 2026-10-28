package servidor

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"tramtai/internal/carpetas"
	"tramtai/internal/idioma"
)

// El aviso de los dos entornos: el servicio del sistema publica su puerto en un archivo que cualquier usuario del equipo
// puede leer, y el Tramtai del usuario (la app, la terminal y «tramtai servicio estado») avisa de que hay otro Tramtai,
// con sus propias APIs y claves.

// rutaPublica es el archivo con el puerto: /run/tramtai/puerto en Linux (el RuntimeDirectory de la unidad, 0755, que
// systemd borra al parar el servicio) y %ProgramData%\Tramtai\publico\puerto en Windows (su carpeta deja leer a
// Usuarios; la raíz del servidor, no). Las pruebas lo sustituyen.
var rutaPublica = func() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(carpetas.RaizServidor(), "publico", "puerto")
	}
	return "/run/tramtai/puerto"
}

// PublicarPuerto lo llama el servicio del sistema al arrancar: deja su puerto en rutaPublica, legible por todos y sin
// permiso de escritura para ellos. Lo escribe en un archivo temporal y lo cambia de nombre, para que nadie lea uno a medias.
func PublicarPuerto(puerto int) error {
	if puerto < 1 || puerto > 65535 {
		return errors.New(idioma.Tf("puerto no válido: %d", puerto))
	}
	ruta := rutaPublica()
	dir := filepath.Dir(ruta)
	if err := prepararPublica(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".puerto-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // ya no existe si se cambió de nombre
	_, err = tmp.WriteString(strconv.Itoa(puerto) + "\n")
	if err == nil && runtime.GOOS != "windows" {
		err = tmp.Chmod(0o644) // la unidad lleva UMask=0077
	}
	if cerrar := tmp.Close(); err == nil {
		err = cerrar
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), ruta)
}

// PuertoDelSistema lee el puerto del servicio del sistema de este equipo, si lo hay. Lo leído no se cree a ciegas: solo
// vale un número de puerto (1-65535), y nada más.
func PuertoDelSistema() (int, bool) {
	f, err := os.Open(rutaPublica())
	if err != nil {
		return 0, false
	}
	defer f.Close()
	datos, err := io.ReadAll(io.LimitReader(f, 16))
	if err != nil || len(datos) == 16 { // un puerto, con su salto de línea, cabe de sobra
		return 0, false
	}
	p, err := strconv.ParseUint(strings.TrimSpace(string(datos)), 10, 16)
	if err != nil || p == 0 {
		return 0, false
	}
	return int(p), true
}

// AvisoCruzado es lo que dicen la app, la terminal y «tramtai servicio estado» del usuario si en el equipo hay un
// Tramtai del sistema; vacío si no lo hay o si este proceso es el propio servidor.
func AvisoCruzado() string {
	if carpetas.Servidor() {
		return ""
	}
	p, ok := PuertoDelSistema()
	if !ok {
		return ""
	}
	return idioma.Tf("Hay otro Tramtai, del sistema, en el puerto %d: es otro entorno, con sus propias APIs y claves", p)
}
