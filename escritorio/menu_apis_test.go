package main

import (
	"fmt"
	"testing"

	"tramtai/internal/servicio"
)

func TestMenuApis(t *testing.T) {
	var puntos []servicio.EstadoPunto
	for i := range 40 {
		p := servicio.EstadoPunto{Nombre: fmt.Sprintf("api%02d", i), Activo: i%3 != 0}
		if i%3 == 0 {
			p.Error = "desactivado"
		}
		if i == 38 || i == 39 {
			p.Activo, p.Error = false, "puerto ocupado" // con error: van primero aunque su nombre sea el último
		}
		puntos = append(puntos, p)
	}
	if m, resto := menuApis(puntos[:3], maximoApisMenu); len(m) != 3 || resto != 0 {
		t.Fatalf("con 3: %d y %d restantes", len(m), resto)
	}
	if m, resto := menuApis(puntos[:15], maximoApisMenu); len(m) != 15 || resto != 0 {
		t.Fatalf("con 15: %d y %d restantes", len(m), resto)
	}
	m, resto := menuApis(puntos, maximoApisMenu)
	if len(m) != 15 || resto != 25 {
		t.Fatalf("con 40: %d y %d restantes", len(m), resto)
	}
	if m[0].Nombre != "api38" || m[1].Nombre != "api39" || !m[2].Activo {
		t.Fatalf("primero las que tienen error y después las activas: %s %s %v", m[0].Nombre, m[1].Nombre, m[2].Activo)
	}
	for _, p := range m[2:] {
		if !p.Activo {
			t.Fatalf("una desactivada (%s) ocupa el sitio de una activa", p.Nombre)
		}
	}
}
