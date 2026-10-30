// Claude Code simulado para las pruebas del servidor (scripts/servidor): responde como «claude -p --output-format
// stream-json» con cuatro trozos de texto separados por una pausa, sin red ni clave de verdad. Con «largo» en la petición
// la pausa es de 4 s (un trabajo de unos 16 s, para guardar una clave mientras corre); si no, de 1 s. Se compila para
// Linux y Windows y se configura como comandos_agentes.claude (o se deja como claude en el PATH del servicio).
package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
)

func main() {
	if slices.Contains(os.Args[1:], "--version") {
		fmt.Println("2.1.0 (Claude Code simulado)")
		return
	}
	peticion, _ := io.ReadAll(os.Stdin)
	pausa := time.Second
	if strings.Contains(string(peticion), "largo") {
		pausa = 4 * time.Second
	}
	fmt.Println(`{"type":"system","subtype":"init","model":"claude-simulado"}`)
	for _, trozo := range []string{"Hola", " desde", " el", " servidor"} {
		time.Sleep(pausa)
		fmt.Printf(`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":%q}}}`+"\n", trozo)
	}
	fmt.Println(`{"type":"result","subtype":"success","is_error":false,"result":"Hola desde el servidor","usage":{"input_tokens":1,"output_tokens":4}}`)
}
