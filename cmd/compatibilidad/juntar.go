package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"tramtai/internal/adaptadores"
	"tramtai/internal/preparar"
)

// salidaFallos es la salida de juntar cuando algo no pasó.
const salidaFallos = 2

func juntar(fichas []adaptadores.Ficha, rutaAnterior, rutaNueva string, pares []string, salida io.Writer) (int, error) {
	var anterior lista
	if datos, err := os.ReadFile(rutaAnterior); err == nil {
		if err := json.Unmarshal(datos, &anterior); err != nil {
			return 0, fmt.Errorf("%s: %w", rutaAnterior, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	porSistema, err := leerResultados(pares)
	if err != nil {
		return 0, err
	}
	nueva := lista{Version: anterior.Version, Agentes: map[string]map[string][]string{}}
	for id, acciones := range anterior.Agentes {
		for accion, versiones := range acciones {
			sumar(&nueva, id, accion, firmada, versiones...)
		}
	}
	if anterior.Version == 0 { // la primera lista parte de lo comprobado en las recetas (el M0)
		for _, f := range fichas {
			for accion, versiones := range f.Receta.Comprobadas {
				sumar(&nueva, f.Nombre, accion, firmada, versiones...)
			}
		}
	}
	var fallos []string
	for _, f := range fichas {
		version, pasan, fallosAgente, notas := juntarAgente(f, porSistema)
		fallos = append(fallos, fallosAgente...)
		for _, nota := range notas {
			fmt.Fprintln(os.Stderr, "sin concluir:", nota)
		}
		for accion, paso := range pasan {
			if paso {
				sumar(&nueva, f.Nombre, accion, preparar.VersionValida, version)
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

// leerResultados lee un resultado por sistema («linux=ruta»): de un sistema de la matriz, uno solo por sistema, con ese
// Sistema dentro y solo acciones conocidas. Así un runner no puede hablar por otro.
func leerResultados(pares []string) (map[string]resultado, error) {
	porSistema := map[string]resultado{}
	for _, par := range pares {
		sistema, ruta, ok := strings.Cut(par, "=")
		if !ok || !slices.Contains(sistemas, sistema) {
			return nil, fmt.Errorf("%q: se esperaba <sistema>=<resultado.json>, con el sistema entre %v", par, sistemas)
		}
		if _, repetido := porSistema[sistema]; repetido {
			return nil, fmt.Errorf("hay dos resultados de %s", sistema)
		}
		var r resultado
		datos, err := os.ReadFile(ruta)
		if err == nil {
			err = json.Unmarshal(datos, &r)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ruta, err)
		}
		if r.Sistema != sistema {
			return nil, fmt.Errorf("%s dice ser de %q, no de %s", ruta, r.Sistema, sistema)
		}
		for id, c := range r.Agentes {
			for _, accion := range slices.Concat(slices.Collect(maps.Keys(c.Acciones)), c.SinConcluir) {
				if !slices.Contains(acciones, accion) {
					return nil, fmt.Errorf("%s: acción desconocida en %s: %q", ruta, id, accion)
				}
			}
		}
		porSistema[sistema] = r
	}
	return porSistema, nil
}

// juntarAgente dice con qué versión y para qué acciones pasó el agente en todos los sistemas, lo que no pasó (para el
// issue) y lo que quedó sin concluir por el límite de la API de GitHub (ni se añade ni se avisa: Rulings 32 y 33).
// Sin la última versión por ese límite, un sistema pudo instalar la de la receta: si las versiones no coinciden por eso,
// también queda sin concluir. Cualquier otro motivo de no saber la última versión (404, etiqueta no válida, sin red) se
// avisa.
func juntarAgente(f adaptadores.Ficha, porSistema map[string]resultado) (version string, pasan map[string]bool, fallos, notas []string) {
	limite, consultada, errorUltima := false, false, ""
	for _, s := range sistemas {
		if c, ok := porSistema[s].Agentes[f.Nombre]; ok {
			consultada = consultada || c.Ultima != ""
			limite = limite || c.UltimaLimite
			errorUltima = cmp.Or(errorUltima, c.UltimaFallo)
		}
	}
	completo := true
	pasan = map[string]bool{}
	pendientes := map[string]bool{}
	for _, s := range sistemas {
		c, ok := porSistema[s].Agentes[f.Nombre]
		if !ok {
			fallos = append(fallos, fmt.Sprintf("- **%s** (%s): sin resultado; mira el job de ese sistema", f.Nombre, s))
			completo = false
			continue
		}
		if c.Fallo != "" {
			fallos = append(fallos, fmt.Sprintf("- **%s %s** (%s):\n\n%s", f.Nombre, mostrar(c.Version, c.Ultima), s, sangrar(bloque(c.Fallo))))
		}
		for _, accion := range c.SinConcluir {
			pendientes[accion] = true
		}
		switch {
		case c.Version == "":
			completo = false
		case !preparar.VersionValida(c.Version):
			fallos = append(fallos, fmt.Sprintf("- **%s** (%s): la versión instalada no tiene una forma válida", f.Nombre, s))
			completo = false
		case version == "":
			version = c.Version
		case c.Version != version:
			completo = false
			texto := fmt.Sprintf("**%s**: versiones distintas según el sistema (%s, %s en %s)", f.Nombre, version, c.Version, s)
			if limite {
				notas = append(notas, texto)
			} else {
				fallos = append(fallos, "- "+texto)
			}
		}
		for accion, paso := range c.Acciones {
			if antes, visto := pasan[accion]; visto {
				paso = paso && antes
			}
			pasan[accion] = paso
		}
	}
	if !consultada && f.Receta.Ultima.Tipo != "" {
		texto := fmt.Sprintf("**%s**: no se pudo consultar la última versión en ningún sistema (%s %s)", f.Nombre, f.Receta.Ultima.Tipo, f.Receta.Ultima.Nombre)
		switch {
		case errorUltima != "":
			fallos = append(fallos, fmt.Sprintf("- %s: %s", texto, tapar(errorUltima, "")))
		case limite:
			notas = append(notas, texto)
		default:
			fallos = append(fallos, "- "+texto)
		}
	}
	for accion := range pendientes {
		notas = append(notas, fmt.Sprintf("**%s** %s: %s", f.Nombre, mostrar(version, ""), accion))
		delete(pasan, accion)
	}
	if !completo {
		return "", nil, fallos, notas
	}
	return version, pasan, fallos, notas
}

// mostrar es la versión instalada o, si no llegó a instalarse, la última publicada; «?» si no tiene forma de versión.
func mostrar(version, ultima string) string {
	if version == "" {
		version = ultima
	}
	if version != "" && !preparar.VersionValida(version) {
		return "?"
	}
	return version
}

// bloque pone el texto entre vallas más largas que cualquier serie de comillas invertidas que lleve.
func bloque(texto string) string {
	larga, serie := 0, 0
	for _, r := range texto {
		if r == '`' {
			serie++
			larga = max(larga, serie)
		} else {
			serie = 0
		}
	}
	valla := strings.Repeat("`", max(3, larga+1))
	return valla + "\n" + texto + "\n" + valla
}

func sangrar(texto string) string { return "  " + strings.ReplaceAll(texto, "\n", "\n  ") }

// firmada dice si una versión de la lista anterior (ya firmada) o de una receta vale: exacta o con comodín de parche.
func firmada(v string) bool {
	base, _ := strings.CutSuffix(v, ".*")
	return preparar.VersionValida(base)
}

// sumar añade a las de la acción las versiones que valen, ordenadas y sin repetir; sin ninguna, no toca nada (ni crea
// el agente). La primera lista publicada traía «1.0.111.», con el punto que Copilot pone tras su versión: así se cae.
func sumar(l *lista, id, accion string, vale func(string) bool, versiones ...string) {
	versiones = slices.DeleteFunc(slices.Clone(versiones), func(v string) bool { return !vale(v) })
	if len(versiones) == 0 {
		return
	}
	if l.Agentes[id] == nil {
		l.Agentes[id] = map[string][]string{}
	}
	todas := slices.Concat(l.Agentes[id][accion], versiones)
	slices.SortFunc(todas, preparar.CompararVersiones)
	l.Agentes[id][accion] = slices.Compact(todas)
}
