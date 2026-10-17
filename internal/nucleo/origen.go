package nucleo

import "context"

// Origen de una petición: de dónde llegó, para mostrarlo y filtrarlo en el historial. Lo marca el servicio al
// autenticar; X-Tramtai-Cliente (terminal o cli) es solo una etiqueta y no da ningún permiso.
const (
	DesdeProbar   = "probar"   // la Web UI, con su sesión
	DesdeTerminal = "terminal" // la terminal interactiva
	DesdeCLI      = "cli"      // los comandos de tramtai
	DesdeApp      = "app"      // una aplicación con la clave de la instancia
	DesdeAPI      = "api"      // una API extra (punto de acceso)
)

type claveOrigen struct{}

// ConOrigenPeticion devuelve ctx con el origen de la petición que lo lleva.
func ConOrigenPeticion(ctx context.Context, origen string) context.Context {
	return context.WithValue(ctx, claveOrigen{}, origen)
}

// OrigenPeticion devuelve el origen guardado en ctx, o "" si no hay.
func OrigenPeticion(ctx context.Context) string {
	o, _ := ctx.Value(claveOrigen{}).(string)
	return o
}
