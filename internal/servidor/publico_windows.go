//go:build windows

package servidor

import "tramtai/internal/carpetas"

// prepararPublica crea la carpeta publico, o la vuelve a dejar, con su propia DACL: los usuarios leen el puerto y nada
// más de la raíz del servidor. Lleva siempre el SID del servicio, calculado, también si la crea un administrador.
func prepararPublica(dir string) error {
	return carpetas.ProtegerPublica(dir, sidServicio(nombreServicio))
}
