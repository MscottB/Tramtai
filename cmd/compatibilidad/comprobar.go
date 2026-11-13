package main

import (
	"context"
	"fmt"
	"io"
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
	// El resultado se escribe tras cada agente: si el job se corta, lo hecho se sube igual.
	r := resultado{Sistema: runtime.GOOS, Agentes: map[string]comprobacion{}}
	for _, f := range fichas {
		c := comprobarAgente(ctx, p, f)
		c.Ultima = ultimas[f.Nombre]
		r.Agentes[f.Nombre] = c
		fmt.Fprintf(salida, "%s %s (última %q): %v; sin concluir: %v\n", f.Nombre, c.Version, c.Ultima, c.Acciones, c.SinConcluir)
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
