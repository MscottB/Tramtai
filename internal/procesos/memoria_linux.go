package procesos

import "strconv"

// guionTope fija RLIMIT_AS con ulimit -v y se sustituye por el programa con exec: el PID y el grupo son los del
// programa. Si no se puede fijar, lo dice por stderr y sigue sin tope (como antes).
const guionTope = `ulimit -v "$1" || echo "tramtai: no se pudo fijar el tope de memoria" >&2; shift; exec "$@"`

// conTope lanza el programa a través de sh para que el tope de memoria rija desde el primer instante, para él y para
// todo lo que lance. Fijarlo con prlimit después de Start no alcanzaría a los hijos que ya hubiera creado (uvx lanza
// Python enseguida). Los argumentos van como posicionales de sh, nunca dentro del guion.
func conTope(programa string, args []string, memoriaMB int) (string, []string) {
	if memoriaMB <= 0 {
		return programa, args
	}
	return "/bin/sh", append([]string{"-c", guionTope, "sh", strconv.Itoa(memoriaMB << 10), programa}, args...)
}
