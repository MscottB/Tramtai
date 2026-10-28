//go:build !windows

package servidor

// prepararPublica no hace nada: en Linux la carpeta es el RuntimeDirectory de la unidad (/run/tramtai, 0755, de la
// cuenta del servicio), que crea systemd al arrancar.
func prepararPublica(string) error { return nil }
