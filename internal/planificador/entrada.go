package planificador

import (
	"context"
	"slices"
	"unicode/utf8"

	"tramtai/internal/adaptadores"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// medida es lo que ocupa la entrada que recibirá el agente: caracteres y tokens estimados (sin el tokenizador de cada
// proveedor, caracteres ÷ 4), y cada documento de mayor a menor. Es el evento «entrada»; no lleva contenido.
type medida struct {
	Caracteres int           `json:"caracteres"`
	Tokens     int           `json:"tokens_estimados"`
	Documentos []parteMedida `json:"documentos"`
}

type parteMedida struct {
	Nombre string `json:"nombre"`
	Tokens int    `json:"tokens_estimados"`
}

func tokensDe(caracteres int) int { return (caracteres + 3) / 4 }

// cabe dice si la entrada, más lo que se reserva para la respuesta, entra en el contexto del modelo.
func cabe(m medida, contexto, reserva int) bool { return m.Tokens+reserva <= contexto }

// noCabe explica cuánto mide la entrada, qué documento pesa más y cuánto admite el modelo.
func noCabe(m medida, c candidato, contexto, reserva int) error {
	mayor := ""
	if len(m.Documentos) > 0 {
		mayor = idioma.Tf("; %s ~%d", m.Documentos[0].Nombre, m.Documentos[0].Tokens)
	}
	return nucleo.NuevoError(nucleo.CodigoEntradaDemasiadoGrande, idioma.Tf(
		"la entrada (~%d tokens%s) no cabe en %s: su contexto es de %d tokens y se reservan %d para la respuesta", m.Tokens, mayor, c, contexto, reserva))
}

// medirEntrada mide las instrucciones y los mensajes, ya con los documentos dentro.
func medirEntrada(s nucleo.SolicitudEjecucion, docs nucleo.Documentos) medida {
	caracteres := utf8.RuneCountInString(s.Instrucciones)
	for _, m := range s.Mensajes {
		caracteres += utf8.RuneCountInString(m.Contenido)
	}
	m := medida{Caracteres: caracteres, Tokens: tokensDe(caracteres), Documentos: []parteMedida{}}
	for _, d := range docs.Partes {
		m.Documentos = append(m.Documentos, parteMedida{Nombre: d.Nombre, Tokens: tokensDe(d.Caracteres)})
	}
	slices.SortStableFunc(m.Documentos, func(a, b parteMedida) int { return b.Tokens - a.Tokens })
	return m
}

// Contexto es el contexto conocido de un agente y modelo, y de dónde sale (configurado o informado); 0 si no se sabe.
func (p *Planificador) Contexto(ctx context.Context, agente, modelo string) (int, string) {
	return p.contextoDe(ctx, candidato{agente: agente, modelo: modelo})
}

// contextoDe es el contexto (tokens) del modelo de un candidato y de dónde sale: lo configurado en
// limites.contexto_tokens (agente/modelo y después agente), lo que informa el agente (su API o lo que dijo su CLI en
// un intento anterior, para el modelo o la versión vista detrás de un alias) o 0 si no se conoce.
func (p *Planificador) contextoDe(ctx context.Context, c candidato) (int, string) {
	version := p.versionVista(c.agente, c.modelo)
	modelos := slices.DeleteFunc([]string{c.modelo, version}, func(m string) bool { return m == "" })
	for _, m := range modelos {
		if n := p.opc.Contextos[c.agente+"/"+m]; n > 0 {
			return n, "configurado"
		}
	}
	if n := p.opc.Contextos[c.agente]; n > 0 {
		return n, "configurado"
	}
	if contextual, ok := p.adaptadores[c.agente].(adaptadores.Contextual); ok {
		if n, err := contextual.Contexto(ctx, c.modelo); err == nil && n > 0 {
			return n, "informado"
		}
	}
	for _, m := range modelos {
		if n, err := p.almacen.Contexto(ctx, c.agente, m); err == nil && n > 0 {
			return n, "informado"
		}
	}
	return 0, ""
}
