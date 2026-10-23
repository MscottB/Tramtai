package nucleo

import "strings"

var sinTildes = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")

// Plegar pasa a minúsculas y quita las tildes, para buscar como se escribe: «reunion» encuentra «Reunión».
func Plegar(s string) string { return sinTildes.Replace(strings.ToLower(s)) }
