package ayuda

import "testing"

// ConNavegador cambia, durante la prueba, cómo se abre una URL: así se prueba sin navegador sin abrir uno de verdad.
func ConNavegador(t testing.TB, abrirURL func(string) error) {
	antes := abrir
	abrir = abrirURL
	t.Cleanup(func() { abrir = antes })
}
