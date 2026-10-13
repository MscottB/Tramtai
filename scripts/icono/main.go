// Command icono dibuja el icono de Tramtai (un tranvía bajo su catenaria) en escritorio/icono/tramtai.png.
// Solo usa la biblioteca estándar; el .ico y los recursos de Windows los genera scripts/compilar.ps1 con wails3.
//
//	go run ./scripts/icono
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

const lado = 1024

type forma func(x, y float64) bool

type capa struct {
	dentro forma
	color  func(y float64) color.NRGBA
}

func rectRedondeado(x0, y0, x1, y1, r float64) forma {
	return func(x, y float64) bool {
		cx := math.Max(x0+r, math.Min(x, x1-r))
		cy := math.Max(y0+r, math.Min(y, y1-r))
		return x >= x0 && x <= x1 && y >= y0 && y <= y1 && math.Hypot(x-cx, y-cy) <= r
	}
}

func circulo(cx, cy, r float64) forma {
	return func(x, y float64) bool { return math.Hypot(x-cx, y-cy) <= r }
}

// segmento es una línea gruesa con extremos redondeados.
func segmento(ax, ay, bx, by, grosor float64) forma {
	return func(x, y float64) bool {
		dx, dy := bx-ax, by-ay
		t := math.Max(0, math.Min(1, ((x-ax)*dx+(y-ay)*dy)/(dx*dx+dy*dy)))
		return math.Hypot(x-(ax+t*dx), y-(ay+t*dy)) <= grosor/2
	}
}

func liso(c color.NRGBA) func(float64) color.NRGBA { return func(float64) color.NRGBA { return c } }

func main() {
	verdeClaro, verdeOscuro := color.NRGBA{0x14, 0xB8, 0xA6, 0xFF}, color.NRGBA{0x0F, 0x5F, 0x58, 0xFF}
	blanco := color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}
	degradado := func(y float64) color.NRGBA {
		t := y / lado
		mezcla := func(a, b uint8) uint8 { return uint8(float64(a)*(1-t) + float64(b)*t) }
		return color.NRGBA{mezcla(verdeClaro.R, verdeOscuro.R), mezcla(verdeClaro.G, verdeOscuro.G), mezcla(verdeClaro.B, verdeOscuro.B), 0xFF}
	}
	capas := []capa{
		{rectRedondeado(0, 0, lado, lado, 220), degradado},
		{segmento(150, 175, 874, 175, 26), liso(blanco)}, // catenaria
		{segmento(512, 335, 425, 182, 30), liso(blanco)}, // pantógrafo
		{segmento(512, 335, 599, 182, 30), liso(blanco)},
		{rectRedondeado(290, 320, 734, 770, 96), liso(blanco)},      // caja
		{rectRedondeado(345, 385, 679, 565, 44), liso(verdeOscuro)}, // parabrisas
		{circulo(392, 668, 38), liso(verdeOscuro)},                  // faros
		{circulo(632, 668, 38), liso(verdeOscuro)},
		{rectRedondeado(330, 750, 430, 838, 26), liso(blanco)}, // bogies
		{rectRedondeado(594, 750, 694, 838, 26), liso(blanco)},
		{segmento(210, 868, 814, 868, 24), liso(color.NRGBA{0xFF, 0xFF, 0xFF, 0xD0})}, // vía
	}
	img := image.NewNRGBA(image.Rect(0, 0, lado, lado))
	const muestras = 3 // supermuestreo 3×3 para suavizar los bordes
	for py := range lado {
		for px := range lado {
			var r, g, b, a float64
			for sy := range muestras {
				for sx := range muestras {
					x, y := float64(px)+(float64(sx)+0.5)/muestras, float64(py)+(float64(sy)+0.5)/muestras
					var c color.NRGBA
					for _, capa := range capas {
						if capa.dentro(x, y) {
							nuevo := capa.color(y)
							alfa := float64(nuevo.A) / 255
							c = color.NRGBA{uint8(float64(nuevo.R)*alfa + float64(c.R)*(1-alfa)), uint8(float64(nuevo.G)*alfa + float64(c.G)*(1-alfa)),
								uint8(float64(nuevo.B)*alfa + float64(c.B)*(1-alfa)), uint8(math.Max(float64(c.A), float64(nuevo.A)))}
						}
					}
					fa := float64(c.A) / 255
					r, g, b, a = r+float64(c.R)*fa, g+float64(c.G)*fa, b+float64(c.B)*fa, a+fa
				}
			}
			if a > 0 {
				n := float64(muestras * muestras)
				img.SetNRGBA(px, py, color.NRGBA{uint8(r / a), uint8(g / a), uint8(b / a), uint8(a / n * 255)})
			}
		}
	}
	destino := filepath.Join("escritorio", "icono", "tramtai.png")
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(destino)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	log.Println("escrito", destino)
}
