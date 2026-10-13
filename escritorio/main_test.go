package main

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

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
