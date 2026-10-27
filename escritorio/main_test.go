package main

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Un fallo de --solo-servicio queda en solo-servicio.log, con la hora, porque un binario windowsgui no muestra stderr; el
// siguiente fallo lo sobrescribe, y si la carpeta de datos aún no existe se crea.
func TestAnotarFalloSoloServicio(t *testing.T) {
	datos := filepath.Join(t.TempDir(), "datos")
	anotarFalloSoloServicio(datos, errors.New("primer fallo"))
	anotarFalloSoloServicio(datos, errors.New("segundo fallo"))
	contenido, err := os.ReadFile(filepath.Join(datos, "solo-servicio.log"))
	texto := string(contenido)
	if err != nil || !strings.Contains(texto, "segundo fallo") || strings.Contains(texto, "primer fallo") || strings.Count(texto, "\n") != 1 || !strings.Contains(texto, "T") {
		t.Fatalf("solo-servicio.log: %q %v", texto, err)
	}
}

// Los iconos de la bandeja están embebidos y se generan al tamaño pedido, en los dos temas, con algo dibujado.
func TestIconosMenu(t *testing.T) {
	dibujado := func(img image.Image) (n int) {
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
					n++
				}
			}
		}
		return n
	}
	for _, nombre := range []string{"abrir", "apis", "copiar", "iniciar", "reiniciar", "detener", "salir"} {
		for _, oscuro := range []bool{false, true} {
			img, err := png.Decode(bytes.NewReader(iconoMenu(nombre, 24, oscuro)))
			if err != nil || img.Bounds().Dx() != 24 || img.Bounds().Dy() != 24 {
				t.Fatalf("icono %s (oscuro=%v): %v", nombre, oscuro, err)
			}
			if dibujado(img) < 20 {
				t.Errorf("icono %s casi vacío", nombre)
			}
		}
	}
	img, err := png.Decode(bytes.NewReader(circulo(colorVerde, 20)))
	if err != nil || img.Bounds().Dx() != 20 || dibujado(img) < 20 {
		t.Fatalf("círculo: %v", err)
	}
}
