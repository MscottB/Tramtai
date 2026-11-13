// Command compatibilidad es el job semanal que comprueba las versiones nuevas de los CLI de los agentes
// (.github/workflows/agentes-compatibilidad.yml; spec de la v0.60 › «Versiones comprobadas»):
//
//	compatibilidad comprobar <resultado.json>
//	compatibilidad juntar <anterior.json> <nueva.json> <resultado.json>…
//
// comprobar, en cada sistema, prepara cada CLI con su receta como lo haría el servicio (internal/preparar): instala
// la última versión, lanza su inicio de sesión hasta que el lector encuentra el enlace y el código (y la petición de
// pegar, si la hay), sin cuenta, lo cancela, lo actualiza y lo desinstala; anota qué acciones pasaron.
//
// juntar añade a la lista anterior (la publicada; sin ella, la de las recetas) cada versión para las acciones que
// pasaron en todos los sistemas y, si algo cambió, escribe la nueva con version+1 (si no, no escribe nada y no se
// publica). Lo que no pasó sale por la salida estándar, en Markdown, y termina con 2: el workflow abre o comenta un
// issue.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/agentes"
	"tramtai/internal/contrato"
	"tramtai/internal/preparar"
)

// salidaFallos es la salida de juntar cuando algo no pasó.
const salidaFallos = 2

// plazoPegar es lo que se espera, tras el enlace y el código, a que el CLI pida pegar (Claude lo pide justo después).
const plazoPegar = 30 * time.Second

// sistemas son los de la matriz del workflow (runtime.GOOS): una versión se añade solo si pasó en todos.
var sistemas = []string{"linux", "darwin", "windows"}

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
	// Ultima es la última versión publicada (vacía si no se pudo consultar); Version, la que quedó instalada.
	Ultima  string `json:"ultima,omitempty"`
	Version string `json:"version,omitempty"`
	// Acciones dice, por acción intentada, si pasó; Fallo es lo que no pasó, con las últimas líneas ya tapadas.
	Acciones map[string]bool `json:"acciones"`
	Fallo    string          `json:"fallo,omitempty"`
}

func main() {
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
	case len(args) >= 4 && args[0] == "juntar":
		codigo, err = juntar(fichas, args[1], args[2], args[3:], salida)
	default:
		err = errors.New("uso: compatibilidad comprobar <resultado.json> | juntar <anterior.json> <nueva.json> <resultado.json>…")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "compatibilidad:", err)
		return 1
	}
	return codigo
}

func comprobar(ctx context.Context, fichas []adaptadores.Ficha, ultima func(context.Context, string) (string, error), destino string, salida io.Writer) error {
	datos, err := os.MkdirTemp("", "compatibilidad")
	if err != nil {
		return err
	}
	defer os.RemoveAll(datos)
	ids := make([]string, len(fichas))
	for i, f := range fichas {
		ids[i] = f.Nombre
	}
	p := preparar.Nuevo(preparar.Opciones{
		Datos: datos,
		Ficha: func(id string) (adaptadores.Ficha, bool) {
			if i := slices.Index(ids, id); i >= 0 {
				return fichas[i], true
			}
			return adaptadores.Ficha{}, false
		},
		Agentes:       ids,
		UltimaVersion: ultima,
		// Aquí es donde se comprueba: todas las versiones se dejan preparar.
		Comprobada: func(string, string, string) bool { return true },
	})
	defer p.Cerrar()
	ultimas := map[string]string{}
	semilla := map[string]map[string][]string{}
	for _, v := range p.Versiones(ctx, true) {
		ultimas[v.Agente] = v.Ultima
		if v.Ultima != "" {
			semilla[v.Agente] = map[string][]string{"instalar": {v.Ultima}}
		}
	}
	// Un instalador que fija la versión instala la comprobada más reciente de la receta y de la lista guardada: con la
	// última publicada en esa lista, instala la última.
	if err := escribirJSON(filepath.Join(datos, "agentes", "compatibilidad.json"), map[string]lista{"lista": {Agentes: semilla}}); err != nil {
		return err
	}
	r := resultado{Sistema: runtime.GOOS, Agentes: map[string]comprobacion{}}
	for _, f := range fichas {
		c := comprobarAgente(ctx, p, f)
		c.Ultima = ultimas[f.Nombre]
		r.Agentes[f.Nombre] = c
		fmt.Fprintf(salida, "%s %s (última %q): %v\n", f.Nombre, c.Version, c.Ultima, c.Acciones)
		if c.Fallo != "" {
			fmt.Fprintln(salida, c.Fallo)
		}
	}
	return escribirJSON(destino, r)
}

// comprobarAgente instala el CLI y, sin cuenta, sigue con su inicio de sesión hasta el enlace y el código y lo cancela;
// después lo actualiza y lo desinstala. Se para en lo primero que no pasa.
func comprobarAgente(ctx context.Context, p *preparar.Preparador, f adaptadores.Ficha) comprobacion {
	// Todas empiezan sin pasar: las que quedan por detrás de un fallo tampoco pasaron en este sistema. Actualizar, tras
	// instalar, siempre se puede (sin orden propia es reinstalar); desinstalar, si hay algo que borrar.
	l := f.Receta.Sesion
	c := comprobacion{Acciones: map[string]bool{"instalar": false, "actualizar": false}}
	if l != nil {
		c.Acciones["iniciar_sesion"] = false
	}
	if len(f.Receta.Borrar[runtime.GOOS]) > 0 {
		c.Acciones["desinstalar"] = false
	}
	fallo := func(accion, porque string, e contrato.EstadoPreparacion) {
		c.Acciones[accion] = false
		texto := strings.TrimSpace(fmt.Sprintf("%s: %s %s %s %s", accion, e.Estado, e.MotivoCodigo, e.Mensaje, porque))
		lineas := e.Lineas[max(len(e.Lineas)-10, 0):]
		c.Fallo = tapar(strings.Join(append([]string{texto}, lineas...), "\n"), e.Codigo)
	}
	var desde time.Time
	enEnlace := func(e contrato.EstadoPreparacion) bool {
		if l == nil || e.Estado != "esperando_usuario" {
			return false
		}
		if desde.IsZero() {
			desde = time.Now()
		}
		return l.PidePegar == nil || e.PidePegar || time.Since(desde) > plazoPegar
	}
	e, err := correr(ctx, p, f.Nombre, "instalar", enEnlace)
	if err != nil {
		fallo("instalar", err.Error(), e)
		return c
	}
	c.Version = e.VersionDespues // la escribe al encontrar el CLI instalado
	if c.Version == "" {
		fallo("instalar", "", e)
		return c
	}
	c.Acciones["instalar"] = true
	if l != nil {
		switch {
		case e.Estado == "esperando_usuario" && (l.PidePegar == nil || e.PidePegar):
			c.Acciones["iniciar_sesion"] = true
		case e.Estado == "esperando_usuario":
			fallo("iniciar_sesion", "dio el enlace y el código, pero no pidió pegar", e)
		default:
			fallo("iniciar_sesion", "no llegó al enlace y el código", e)
		}
		if e.Estado == "esperando_usuario" {
			_ = p.Cancelar(f.Nombre)
			if e, err = correr(ctx, p, f.Nombre, "", nil); err != nil || e.Estado != "cancelado" {
				fallo("iniciar_sesion", "no se canceló", e)
			}
		}
		if !c.Acciones["iniciar_sesion"] {
			return c
		}
	}
	for _, accion := range []string{"actualizar", "desinstalar"} {
		if _, toca := c.Acciones[accion]; !toca {
			continue
		}
		e, err := correr(ctx, p, f.Nombre, accion, nil)
		if err != nil {
			fallo(accion, err.Error(), e)
			return c
		}
		if e.Estado != "listo" {
			fallo(accion, "", e)
			return c
		}
		c.Acciones[accion] = true
	}
	return c
}

// correr lanza la acción (ninguna, con accion vacía: sigue la que está en marcha) y espera a que termine o a que parar
// diga que basta; devuelve el estado de ese momento.
func correr(ctx context.Context, p *preparar.Preparador, agente, accion string, parar func(contrato.EstadoPreparacion) bool) (contrato.EstadoPreparacion, error) {
	if accion != "" {
		if _, err := p.Iniciar(agente, contrato.PedidoPreparacion{Accion: accion}); err != nil {
			return contrato.EstadoPreparacion{}, err
		}
	}
	for {
		e, _ := p.Estado(agente)
		if !slices.Contains([]string{"instalando", "actualizando", "desinstalando", "comprobando", "esperando_usuario"}, e.Estado) ||
			(parar != nil && parar(e)) {
			return e, nil
		}
		select {
		case <-ctx.Done():
			_ = p.Cancelar(agente)
			return e, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Lo que se tapa en lo que va a un issue: los códigos de dispositivo (Codex «ABCD-EFGH1», Copilot «ABCD-EFGH») y lo que
// va tras la ruta de un enlace (estado y desafío de OAuth).
var (
	reCodigo   = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4,5}\b`)
	reConsulta = regexp.MustCompile(`(https?://[^\s?#]+)[?#]\S*`)
)

func tapar(texto, codigo string) string {
	if codigo != "" {
		texto = strings.ReplaceAll(texto, codigo, "XXXX-XXXX")
	}
	texto = reCodigo.ReplaceAllString(texto, "XXXX-XXXX")
	return reConsulta.ReplaceAllString(texto, "$1?…")
}

func juntar(fichas []adaptadores.Ficha, rutaAnterior, rutaNueva string, rutas []string, salida io.Writer) (int, error) {
	var anterior lista
	if datos, err := os.ReadFile(rutaAnterior); err == nil {
		if err := json.Unmarshal(datos, &anterior); err != nil {
			return 0, fmt.Errorf("%s: %w", rutaAnterior, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	porSistema := map[string]resultado{}
	for _, ruta := range rutas {
		var r resultado
		datos, err := os.ReadFile(ruta)
		if err == nil {
			err = json.Unmarshal(datos, &r)
		}
		if err != nil {
			return 0, fmt.Errorf("%s: %w", ruta, err)
		}
		porSistema[r.Sistema] = r
	}
	nueva := lista{Version: anterior.Version, Agentes: map[string]map[string][]string{}}
	for id, acciones := range anterior.Agentes {
		for accion, versiones := range acciones {
			sumar(&nueva, id, accion, versiones...)
		}
	}
	if anterior.Version == 0 { // la primera lista parte de lo comprobado en las recetas (el M0)
		for _, f := range fichas {
			for accion, versiones := range f.Receta.Comprobadas {
				sumar(&nueva, f.Nombre, accion, versiones...)
			}
		}
	}
	var fallos []string
	for _, f := range fichas {
		version, completo, consultada := "", true, false
		pasan := map[string]bool{}
		for _, s := range sistemas {
			c, ok := porSistema[s].Agentes[f.Nombre]
			if !ok {
				fallos = append(fallos, fmt.Sprintf("- **%s** (%s): sin resultado; mira el job de ese sistema", f.Nombre, s))
				completo = false
				continue
			}
			consultada = consultada || c.Ultima != ""
			if c.Fallo != "" {
				fallos = append(fallos, fmt.Sprintf("- **%s %s** (%s):\n\n  ```\n  %s\n  ```", f.Nombre, cmpVersion(c), s,
					strings.ReplaceAll(c.Fallo, "\n", "\n  ")))
			}
			switch {
			case c.Version == "":
				completo = false
			case version == "":
				version = c.Version
			case c.Version != version:
				fallos = append(fallos, fmt.Sprintf("- **%s**: versiones distintas según el sistema (%s, %s en %s)", f.Nombre, version, c.Version, s))
				completo = false
			}
			for accion, paso := range c.Acciones {
				if antes, visto := pasan[accion]; visto {
					paso = paso && antes
				}
				pasan[accion] = paso
			}
		}
		if !consultada && f.Receta.Ultima.Tipo != "" {
			fallos = append(fallos, fmt.Sprintf("- **%s**: no se pudo consultar la última versión en ningún sistema (%s %s)",
				f.Nombre, f.Receta.Ultima.Tipo, f.Receta.Ultima.Nombre))
		}
		if !completo {
			continue
		}
		for accion, paso := range pasan {
			if paso {
				sumar(&nueva, f.Nombre, accion, version)
			}
		}
	}
	if !maps.EqualFunc(nueva.Agentes, anterior.Agentes, func(a, b map[string][]string) bool { return maps.EqualFunc(a, b, slices.Equal) }) {
		nueva.Version++
		if err := escribirJSON(rutaNueva, nueva); err != nil {
			return 0, err
		}
	}
	if len(fallos) > 0 {
		fmt.Fprintln(salida, strings.Join(fallos, "\n"))
		return salidaFallos, nil
	}
	return 0, nil
}

// cmpVersion es la versión instalada o, si no llegó a instalarse, la última publicada.
func cmpVersion(c comprobacion) string {
	if c.Version != "" {
		return c.Version
	}
	return c.Ultima
}

// reVersion es la forma de una versión de la lista: exacta (2.1.311, 1.0.0-beta) o con comodín de parche (2.1.*).
var reVersion = regexp.MustCompile(`^[0-9]+(\.[0-9A-Za-z-]+)*(\.\*)?$`)

// sumar añade versiones a las de la acción, ordenadas y sin repetir; una con otra forma no se firma (y se cae de la
// lista anterior: la primera lista publicada traía «1.0.111.», con el punto que Copilot pone tras su versión).
func sumar(l *lista, id, accion string, versiones ...string) {
	if l.Agentes[id] == nil {
		l.Agentes[id] = map[string][]string{}
	}
	todas := slices.DeleteFunc(slices.Concat(l.Agentes[id][accion], versiones), func(v string) bool { return !reVersion.MatchString(v) })
	if len(todas) == 0 {
		delete(l.Agentes[id], accion)
		return
	}
	slices.SortFunc(todas, preparar.CompararVersiones)
	l.Agentes[id][accion] = slices.Compact(todas)
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
