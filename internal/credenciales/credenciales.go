// Package credenciales guarda las claves de API de los proveedores en el almacén de credenciales del sistema
// (sección 16.1), para no dejarlas en variables de entorno permanentes ni en archivos. Tramtai no lee ni guarda
// tokens OAuth de los CLIs: solo las claves que el usuario registra aquí.
package credenciales

import "slices"

// Nombres son las variables que admite el almacén; cada una se guarda como tramtai/<NOMBRE>.
var Nombres = []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "CODEX_API_KEY", "GEMINI_API_KEY"}

// Admitido indica si el nombre es una de las claves que Tramtai sabe usar.
func Admitido(nombre string) bool { return slices.Contains(Nombres, nombre) }

func objetivo(nombre string) string { return "tramtai/" + nombre }
