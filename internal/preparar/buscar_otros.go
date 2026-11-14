//go:build !windows

package preparar

// pathUsuario: fuera de Windows los instaladores tocan los archivos de arranque, no un PATH que se pueda leer; basta con
// las rutas de la receta.
func pathUsuario() []string { return nil }
