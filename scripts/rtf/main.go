// Command rtf convierte LICENSE.md y TERMINOS.md en bin/LICENCIA.rtf y bin/TERMINOS.rtf para las páginas de licencia
// del instalador: NSIS lee un texto plano como ANSI (los acentos de un UTF-8 salen rotos) y no entiende Markdown. Solo
// cubre el Markdown de esos dos archivos. Lo ejecuta scripts/compilar.ps1 -Instalador:
//
//	go run ./scripts/rtf
package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reEnlace     = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reAutomatico = regexp.MustCompile(`<(https?://[^>]+)>`)
	reNegrita    = regexp.MustCompile(`\*\*\*([^*]+)\*\*\*|\*\*([^*]+)\*\*`)
	reNumerada   = regexp.MustCompile(`^(\d+)\.\s+(.*)$`)
	reVineta     = regexp.MustCompile(`^(\s*)[-*]\s+(.*)$`)
)

func main() {
	for origen, destino := range map[string]string{"LICENSE.md": "LICENCIA.rtf", "TERMINOS.md": "TERMINOS.rtf"} {
		md, err := os.ReadFile(origen)
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("bin", destino), []byte(convertir(string(md))), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

// convertir pasa el Markdown a RTF: un párrafo por bloque (las líneas seguidas se unen), títulos en negrita y más
// grandes, listas con viñeta o número y sangría, citas en cursiva y enlaces como su texto.
func convertir(md string) string {
	var b strings.Builder
	b.WriteString(`{\rtf1\ansi\deff0{\fonttbl{\f0\fswiss Segoe UI;}}\fs18` + "\n")
	var parrafo []string
	formato := ""
	cerrar := func() {
		if len(parrafo) > 0 {
			b.WriteString(`\pard` + formato + " " + enLinea(strings.Join(parrafo, " ")) + `\par` + "\n")
		}
		parrafo, formato = nil, ""
	}
	for _, linea := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n") {
		recortada := strings.TrimSpace(linea)
		vineta, numerada := reVineta.FindStringSubmatch(linea), reNumerada.FindStringSubmatch(recortada)
		switch {
		case recortada == "" || recortada == "---":
			cerrar()
		case strings.HasPrefix(recortada, "#"):
			cerrar()
			nivel := len(recortada) - len(strings.TrimLeft(recortada, "#"))
			tamano := map[bool]string{true: `\fs28`, false: `\fs22`}[nivel == 1]
			b.WriteString(`\pard\sb240\sa120\keepn{\b` + tamano + " " + enLinea(strings.TrimSpace(recortada[nivel:])) + `}\par` + "\n")
		case vineta != nil:
			cerrar()
			sangria := 360 * (1 + len(vineta[1])/2)
			parrafo, formato = []string{vineta[2]}, `\li`+strconv.Itoa(sangria)+`\fi-360\bullet\tab`
		case numerada != nil:
			cerrar()
			parrafo, formato = []string{numerada[2]}, `\li360\fi-360 `+numerada[1]+`.\tab`
		case strings.HasPrefix(recortada, "> "):
			cerrar()
			b.WriteString(`\pard\li360{\i ` + enLinea(recortada[2:]) + `}\par` + "\n")
		default:
			if formato == "" && len(parrafo) == 0 {
				formato = `\sa120`
			}
			parrafo = append(parrafo, recortada)
		}
	}
	cerrar()
	b.WriteString("}\n")
	return b.String()
}

// enLinea escapa el texto para RTF y convierte enlaces, URL y negritas.
func enLinea(texto string) string {
	texto = reEnlace.ReplaceAllString(texto, "$1")
	texto = reAutomatico.ReplaceAllString(texto, "$1")
	return reNegrita.ReplaceAllStringFunc(escapar(texto), func(s string) string {
		if p := reNegrita.FindStringSubmatch(s); p[1] != "" {
			return `{\b\i ` + p[1] + `}`
		} else {
			return `{\b ` + p[2] + `}`
		}
	})
}

// escapar deja el RTF en ASCII: barras y llaves escapadas, y el resto de caracteres como \uN? (UTF-16 con signo).
func escapar(texto string) string {
	var b strings.Builder
	for _, r := range texto {
		switch {
		case r == '\\' || r == '{' || r == '}':
			b.WriteString(`\` + string(r))
		case r < 0x80:
			b.WriteRune(r)
		case r < 0x10000:
			b.WriteString(`\u` + strconv.Itoa(int(int16(r))) + "?")
		default: // fuera del plano básico: par sustituto
			r -= 0x10000
			b.WriteString(`\u` + strconv.Itoa(int(int16(0xD800+(r>>10)))) + `?\u` + strconv.Itoa(int(int16(0xDC00+(r&0x3FF)))) + "?")
		}
	}
	return b.String()
}
