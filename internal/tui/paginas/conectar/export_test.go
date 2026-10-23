package conectar

import (
	"strconv"

	"tramtai/internal/nucleo"
)

// Barras son las del presupuesto (título, texto y fracción con dos decimales), para compararlas con BarrasPresupuesto.vue.
func Barras(lengua string, e nucleo.EstadoPresupuesto) [][3]string {
	var r [][3]string
	for _, b := range barras(lengua, e) {
		r = append(r, [3]string{b.titulo, b.texto, strconv.FormatFloat(b.fraccion, 'f', 2, 64)})
	}
	return r
}
