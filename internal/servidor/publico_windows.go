//go:build windows

package servidor

import "tramtai/internal/carpetas"

// prepararPublica crea la carpeta publico, o la vuelve a dejar, con su propia DACL: los usuarios leen el puerto y nada
// más de la raíz del servidor. Lleva siempre el SID del servicio, calculado, también si la crea un administrador.
func prepararPublica(dir string) error {
	return carpetas.ProtegerPublica(dir, sidServicio(nombreServicio))
}

// leerPublico lee el puerto solo si su dueño es de confianza (SYSTEM, Administradores, el servicio o el propio usuario)
// y sin seguir enlaces: sin el servidor instalado, cualquiera puede crear el archivo en %ProgramData%. Si no lo es, no se
// avisa (no hay registro de depuración en quien lo lee: la app, la terminal o la CLI).
func leerPublico(ruta string, maximo int64) ([]byte, error) {
	return carpetas.LeerDeConfianza(ruta, sidServicio(nombreServicio), maximo)
}
