package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/preparar"
)

// plazoPegar es lo que se espera, tras el enlace y el código, a que el CLI pida pegar (Claude lo pide justo después).
const plazoPegar = 30 * time.Second

// reLimite reconoce un CLI que chocó con el límite de la API de GitHub («API rate limit exceeded»): eso no dice nada de
// la receta y queda sin concluir (Ruling 32).
var reLimite = regexp.MustCompile(`(?i)rate limit`)

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
	// La última versión se consulta una vez por agente, antes de nada, y el preparador usa lo consultado (Descarga sin
	// versión fijada). Sin gancho de las pruebas, GitHub se consulta aquí, con el token del job (Ruling 33), y las demás
	// fuentes como el servicio.
	ultimas, errores := map[string]string{}, map[string]error{}
	var p *preparar.Preparador
	p = preparar.Nuevo(preparar.Opciones{
		Datos: datos,
		Ficha: func(id string) (adaptadores.Ficha, bool) {
			if i := slices.Index(ids, id); i >= 0 {
				return fichas[i], true
			}
			return adaptadores.Ficha{}, false
		},
		Agentes:       ids,
		UltimaVersion: func(_ context.Context, id string) (string, error) { return ultimas[id], errores[id] },
		// Aquí es donde se comprueba: todas las versiones se dejan preparar.
		Comprobada: func(string, string, string) bool { return true },
	})
	defer p.Cerrar()
	if ultima == nil {
		ultima = func(ctx context.Context, id string) (string, error) {
			if f := fichas[slices.Index(ids, id)]; f.Receta.Ultima.Tipo == "github" {
				return ultimaGitHub(ctx, f.Receta.Ultima, tokenGitHub)
			}
			return p.UltimaPublicada(ctx, id)
		}
	}
	semilla := map[string]map[string][]string{}
	for _, id := range ids {
		consulta, cancelar := context.WithTimeout(ctx, plazoUltima)
		ultimas[id], errores[id] = ultima(consulta, id)
		cancelar()
		if errores[id] == nil {
			semilla[id] = map[string][]string{"instalar": {ultimas[id]}}
		}
	}
	// Un instalador que fija la versión instala la comprobada más reciente de la receta y de la lista guardada: con la
	// última publicada en esa lista, instala la última.
	if err := escribirJSON(filepath.Join(datos, "agentes", "compatibilidad.json"), map[string]lista{"lista": {Agentes: semilla}}); err != nil {
		return err
	}
	// El resultado se escribe tras cada agente: si el job se corta, lo hecho se sube igual.
	r := resultado{Sistema: runtime.GOOS, Agentes: map[string]comprobacion{}}
	for _, f := range fichas {
		c := comprobarAgente(ctx, p, f)
		anotarUltima(&c, ultimas[f.Nombre], errores[f.Nombre])
		r.Agentes[f.Nombre] = c
		fmt.Fprintf(salida, "%s %s (última %q%s): %v; sin concluir: %v\n", f.Nombre, c.Version, c.Ultima, c.UltimaFallo, c.Acciones, c.SinConcluir)
		if c.Fallo != "" {
			fmt.Fprintln(salida, c.Fallo)
		}
		if err := escribirJSON(destino, r); err != nil {
			return err
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
		if reLimite.MatchString(strings.Join(append([]string{e.Mensaje, porque}, e.Lineas...), "\n")) {
			// Esta y las que quedan por probar, sin concluir.
			for a, paso := range c.Acciones {
				if !paso {
					delete(c.Acciones, a)
					c.SinConcluir = append(c.SinConcluir, a)
				}
			}
			slices.Sort(c.SinConcluir)
			return
		}
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
	// Sin inicio de sesión que lo siga, instalar tiene que terminar en listo.
	if c.Version == "" || l == nil && e.Estado != "listo" {
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

// plazoUltima acota cada consulta de la última versión, como en el servicio.
const plazoUltima = 10 * time.Second

// tokenGitHub es el GITHUB_TOKEN del job (solo lectura), que main quita del entorno al arrancar: solo va en las consultas
// a la API de GitHub, nunca a un instalador ni a un CLI.
var tokenGitHub string

// La API de GitHub y su cliente; variables para las pruebas.
var (
	baseGitHub    = "https://api.github.com/repos/"
	clienteGitHub = &http.Client{}
)

// errorLimite es un 403 o 429 de la API de GitHub: su límite de peticiones, que deja la comprobación sin concluir.
type errorLimite struct{ error }

// ultimaGitHub es el tag_name del último release (sin Prefijo), como la fuente «github» del servicio, pero con el token
// del job. Un 403/429 es errorLimite; un 404, un tag que no es una versión o un fallo de red son un error que se avisa.
func ultimaGitHub(ctx context.Context, fuente adaptadores.FuenteVersion, token string) (string, error) {
	url := baseGitHub + fuente.Nombre + "/releases/latest"
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	pedido.Header.Set("Accept", "application/vnd.github+json")
	pedido.Header.Set("User-Agent", "tramtai-compatibilidad")
	if token != "" {
		pedido.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := clienteGitHub.Do(pedido)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	switch r.StatusCode {
	case http.StatusOK:
	case http.StatusForbidden, http.StatusTooManyRequests:
		return "", errorLimite{fmt.Errorf("%s respondió %d", url, r.StatusCode)}
	default:
		return "", fmt.Errorf("%s respondió %d", url, r.StatusCode)
	}
	var j struct {
		Etiqueta string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&j); err != nil {
		return "", fmt.Errorf("%s: %w", url, err)
	}
	version := strings.TrimPrefix(j.Etiqueta, fuente.Prefijo)
	if !preparar.VersionValida(version) {
		return "", fmt.Errorf("%s: la etiqueta %q no es una versión válida", url, j.Etiqueta)
	}
	return version, nil
}

// anotarUltima guarda en la comprobación la última versión o por qué no se supo: el límite de GitHub aparte, porque
// queda sin concluir; lo demás se avisa.
func anotarUltima(c *comprobacion, ultima string, err error) {
	var limite errorLimite
	switch {
	case err == nil:
		c.Ultima = ultima
	case errors.As(err, &limite):
		c.UltimaLimite = true
	default:
		c.UltimaFallo = err.Error()
	}
}
