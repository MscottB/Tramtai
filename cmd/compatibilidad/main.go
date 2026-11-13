// Command compatibilidad es el job semanal que comprueba las versiones nuevas de los CLI de los agentes
// (.github/workflows/agentes-compatibilidad.yml; spec de la v0.60 › «Versiones comprobadas»):
//
//	compatibilidad comprobar <resultado.json>
//	compatibilidad juntar <anterior.json> <nueva.json> [<sistema>=<resultado.json>…]
//
// comprobar, en cada sistema, prepara cada CLI con su receta como lo haría el servicio (internal/preparar): instala
// la última versión, lanza su inicio de sesión hasta que el lector encuentra el enlace y el código (y la petición de
// pegar, si la hay), sin cuenta, lo cancela, lo actualiza y lo desinstala; anota qué acciones pasaron.
//
// juntar añade a la lista anterior (la publicada; sin ella, la de las recetas) cada versión para las acciones que
// pasaron en todos los sistemas y, si algo cambió, escribe la nueva con version+1 (si no, no escribe nada y no se
// publica). Lo que no pasó sale por la salida estándar, en Markdown, y termina con 2: el workflow abre o comenta un
// issue. Lo que quedó sin concluir por el límite de la API de GitHub no se añade ni se avisa (Rulings 32 y 33).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
)

// sistemas son los de la matriz del workflow (runtime.GOOS): una versión se añade solo si pasó en todos.
var sistemas = []string{"linux", "darwin", "windows"}

// acciones son las que puede traer un resultado.
var acciones = []string{"instalar", "actualizar", "iniciar_sesion", "desinstalar"}

// lista es la lista firmada, con el formato que lee preparar.LeerLista.
type lista struct {
	Version int                            `json:"version"`
	Agentes map[string]map[string][]string `json:"agentes"`
}

// resultado es lo que comprobar deja en un sistema.
type resultado struct {
	Sistema string                  `json:"sistema"`
	Agentes map[string]comprobacion `json:"agentes"`
}

type comprobacion struct {
	// Ultima es la última versión publicada; si no se pudo consultar, UltimaLimite dice que fue por el límite de la API
	// de GitHub (sin concluir) y UltimaFallo, si no, por qué (se avisa). Version es la que quedó instalada.
	Ultima       string `json:"ultima,omitempty"`
	UltimaLimite bool   `json:"ultima_limite,omitempty"`
	UltimaFallo  string `json:"ultima_fallo,omitempty"`
	Version      string `json:"version,omitempty"`
	// Acciones dice, por acción intentada, si pasó; Fallo es lo que no pasó, con las últimas líneas ya tapadas.
	Acciones map[string]bool `json:"acciones"`
	Fallo    string          `json:"fallo,omitempty"`
	// SinConcluir son las acciones que no se pudieron probar por el límite de la API de GitHub.
	SinConcluir []string `json:"sin_concluir,omitempty"`
}

func main() {
	// El token del job solo sirve para consultar la API de GitHub: fuera del entorno, ningún proceso hijo lo hereda.
	tokenGitHub = os.Getenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_TOKEN")
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt)
	defer parar()
	os.Exit(ejecutar(ctx, agentes.Todos(), nil, os.Args[1:], os.Stdout))
}

// ejecutar es main con lo que las pruebas cambian: las fichas y la consulta de la última versión (nil: la de la
// receta).
func ejecutar(ctx context.Context, fichas []adaptadores.Ficha, ultima func(context.Context, string) (string, error), args []string, salida io.Writer) int {
	fichas = slices.DeleteFunc(slices.Clone(fichas), func(f adaptadores.Ficha) bool { return f.Receta == nil })
	var err error
	codigo := 0
	switch {
	case len(args) == 2 && args[0] == "comprobar":
		err = comprobar(ctx, fichas, ultima, args[1], salida)
	case len(args) >= 3 && args[0] == "juntar":
		codigo, err = juntar(fichas, args[1], args[2], args[3:], salida)
	default:
		err = errors.New("uso: compatibilidad comprobar <resultado.json> | juntar <anterior.json> <nueva.json> [<sistema>=<resultado.json>…]")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "compatibilidad:", err)
		return 1
	}
	return codigo
}

func escribirJSON(ruta string, v any) error {
	datos, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	return os.WriteFile(ruta, append(datos, '\n'), 0o644)
}
