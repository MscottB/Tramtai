package main

import (
	"strings"
	"testing"
)

func TestConvertir(t *testing.T) {
	rtf := convertir("# Términos «de» uso\n\n**Aviso** y ***fuerte***: [licencia](LICENSE.md) <https://x.y> {a}\\b\n" +
		"sigue el párrafo\n\n- uno\n   - dentro\n1. **Uno.** primero\n\n> Nota\n\n---\n")
	for _, esperado := range []string{
		`{\rtf1`, `T\u233?rminos \u171?de\u187? uso`, // acentos y comillas con escape: no dependen de la codificación
		`{\b Aviso}`, `{\b\i fuerte}`, // negrita y negrita cursiva
		` licencia https://x.y \{a\}\\b sigue el p\u225?rrafo`, // enlaces como texto, llaves y barras escapadas, líneas unidas
		`\bullet\tab uno`, `\li720\fi-360\bullet\tab dentro`, `1.\tab {\b Uno.} primero`, `{\i Nota}`,
	} {
		if !strings.Contains(rtf, esperado) {
			t.Errorf("falta %q en:\n%s", esperado, rtf)
		}
	}
	for _, sobra := range []string{"**", "](", "# ", "<https", "---"} {
		if strings.Contains(rtf, sobra) {
			t.Errorf("queda Markdown sin convertir (%q) en:\n%s", sobra, rtf)
		}
	}
	for _, r := range rtf {
		if r > 0x7e {
			t.Fatalf("el RTF debe ser ASCII puro; contiene %q", r)
		}
	}
}
