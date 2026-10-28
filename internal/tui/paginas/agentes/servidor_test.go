package agentes_test

import (
	"maps"
	"strings"
	"testing"

	"tramtai/internal/adaptadores"
	"tramtai/internal/servicio"
	"tramtai/internal/tui/paginas/prueba"
)

// TestAgentesServidor: en la terminal del servidor no se ofrece «tu suscripción» con la tecla a: en local, el inicio de
// sesión del CLI con su orden; con el servicio abierto a la red, la suscripción personal con sus condiciones. La clave que
// falta se guarda con tramtai-servidor.
func TestAgentesServidor(t *testing.T) {
	falta := detectado(adaptadores.Estado{Autenticacion: "clave_api", Problema: "falta ANTHROPIC_API_KEY", ProblemaCodigo: adaptadores.ProblemaFaltaClave})
	for _, c := range []struct {
		expuesto bool
		orden    string
	}{{false, "tramtai-servidor agentes entrar claude"}, {true, "tramtai-servidor agentes suscripcion claude --personal"}} {
		cli, id := prueba.Ajustado(t, func(o *servicio.Opciones) {
			maps.Copy(o.Registro, map[string]adaptadores.Adaptador{"claude": falta})
			o.Entorno, o.Expuesto = "servidor", c.expuesto
		})
		ctx := contexto(t, cli, id)
		ctx.Entorno = "servidor"
		a := abrir(t, ctx)
		a.Pulsar("down") // el simulado, listo, va primero
		a.Pulsar("enter")
		v := strings.Join(strings.Fields(a.Vista()), " ")
		for _, quiero := range []string{c.orden, "tramtai-servidor claves guardar ANTHROPIC_API_KEY"} {
			if !strings.Contains(v, quiero) {
				t.Errorf("expuesto %v: falta %q:\n%s", c.expuesto, quiero, a.Vista())
			}
		}
		if strings.Contains(v, "tecla a") || strings.Contains(v, "tramtai claves guardar") {
			t.Errorf("expuesto %v: en el servidor no se ofrece la suscripción con a ni la orden del escritorio:\n%s", c.expuesto, a.Vista())
		}
		a.Pulsar("esc")
		fuera := a.Pulsar("a")
		if av, ok := aviso(fuera); !ok || !strings.Contains(av.Texto, c.orden) {
			t.Errorf("expuesto %v: a dice la orden en lugar de preguntar: %+v", c.expuesto, fuera)
		}
		if _, ok := pregunta(fuera); ok {
			t.Errorf("expuesto %v: a no pregunta por el riesgo en el servidor", c.expuesto)
		}
	}
}
