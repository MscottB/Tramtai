package main

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/servicio"
	"tramtai/internal/servidor"
)

// avisoCruzado es servidor.AvisoCruzado; las pruebas lo cambian para no depender de un servidor en el equipo.
var avisoCruzado = servidor.AvisoCruzado

func imprimirEstado(out io.Writer, e servicio.Estado) {
	fmt.Fprintf(out, idioma.T("servicio activo en %s · %s · pid %d · desde %s\n"), e.Base, e.Version, e.PID, e.IniciadoEn.Format("02-01-2006 15:04"))
	if aviso := avisoCruzado(); aviso != "" {
		fmt.Fprintln(out, aviso)
	}
	fmt.Fprintln(out, idioma.T("\nespacios:"))
	if len(e.Instancias) == 0 {
		fmt.Fprintln(out, idioma.T("  ninguna (tramtai global iniciar, o tramtai proyecto iniciar en un proyecto)"))
	}
	for _, i := range e.Instancias {
		fmt.Fprintf(out, "  %-28s %-9s %s\n", i.ID, i.Ambito, resumenTrabajos(i.Trabajos))
		if i.Raiz != "" {
			fmt.Fprintf(out, idioma.T("  %-28s raíz: %s\n"), "", i.Raiz)
		}
		fmt.Fprintf(out, "  %-28s token: %s\n", "", i.ArchivoToken)
	}
	fmt.Fprintln(out, idioma.T("\nAPIs:"))
	imprimirPuntos(out, e.Puntos)
	for _, err := range e.Errores {
		fmt.Fprintln(out, idioma.Tf("configuración: %s", err))
	}
}

func imprimirPuntos(out io.Writer, puntos []servicio.EstadoPunto) {
	if len(puntos) == 0 {
		fmt.Fprintln(out, idioma.T("  ninguno (tramtai acceso crear <nombre>)"))
	}
	for _, p := range puntos {
		estado := idioma.T("activo")
		if p.Error != "" {
			estado = p.Error
		}
		nombre := p.Nombre
		if p.Grupo != "" {
			nombre += " (" + p.Grupo + ")"
		}
		fmt.Fprintf(out, "  %-18s %-40s %-24s %s\n", nombre, valorO(p.URL, idioma.T("Sin dato")), "["+strings.Join(p.Contratos, ",")+"]", estado)
		if p.Presupuesto != nil {
			fmt.Fprintf(out, "  %-18s %s\n", "", textoPresupuesto(*p.Presupuesto))
		}
	}
}

// textoPresupuesto resume los límites: «presupuesto: diario 500 peticiones; mensual 50 USD».
func textoPresupuesto(p nucleo.Presupuesto) string {
	var partes []string
	for _, per := range []struct {
		nombre string
		l      nucleo.Limites
	}{{idioma.T("diario"), p.Diario}, {idioma.T("mensual"), p.Mensual}} {
		var limites []string
		if per.l.Peticiones > 0 {
			limites = append(limites, strconv.Itoa(per.l.Peticiones)+" "+idioma.T("peticiones"))
		}
		if per.l.Tokens > 0 {
			limites = append(limites, strconv.Itoa(per.l.Tokens)+" tokens")
		}
		if per.l.USD > 0 {
			limites = append(limites, strconv.FormatFloat(per.l.USD, 'f', -1, 64)+" USD")
		}
		if len(limites) > 0 {
			partes = append(partes, per.nombre+" "+strings.Join(limites, ", "))
		}
	}
	return idioma.T("presupuesto:") + " " + strings.Join(partes, "; ")
}

func resumenTrabajos(conteo map[string]int) string {
	if len(conteo) == 0 {
		return idioma.T("sin peticiones")
	}
	estados := make([]string, 0, len(conteo))
	for estado, n := range conteo {
		estados = append(estados, estado+" "+strconv.Itoa(n))
	}
	sort.Strings(estados)
	return strings.Join(estados, " · ")
}

func valorO(texto, alternativa string) string {
	if texto == "" {
		return alternativa
	}
	return texto
}
