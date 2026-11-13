package preparar

import (
	"cmp"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/adaptadores"
	"tramtai/internal/configuracion"
)

// URLLista es la lista firmada de versiones comprobadas: la sube el job semanal del CI al release fijo «compatibilidad»,
// con su firma (ed25519, la clave de las actualizaciones) en URLLista+".sig".
var URLLista = "https://github.com/MscottB/Tramtai/releases/download/compatibilidad/compatibilidad.json"

// listaFirmada es la lista que publica el CI: por agente y acción, versiones exactas o «X.Y.*».
type listaFirmada struct {
	Version int                            `json:"version"`
	Agentes map[string]map[string][]string `json:"agentes"`
}

// compatibilidad es datos/agentes/compatibilidad.json: la última lista firmada aceptada, las versiones marcadas como no
// compatibles en este equipo, la última versión publicada de cada CLI y cuándo se consultó todo (una vez al día).
type compatibilidad struct {
	Lista         listaFirmada        `json:"lista"`
	NoCompatibles map[string][]string `json:"no_compatibles,omitempty"`
	Ultimas       map[string]string   `json:"ultimas,omitempty"`
	Leida         time.Time           `json:"leida,omitzero"`
}

func (p *Preparador) rutaCompatibilidad() string {
	return filepath.Join(p.o.Datos, "agentes", "compatibilidad.json")
}

// compatibilidad lee el archivo; si no está o está dañado, uno vacío (la próxima escritura lo rehace).
func (p *Preparador) compatibilidad() compatibilidad {
	p.mc.Lock()
	defer p.mc.Unlock()
	return p.leerCompatibilidad()
}

func (p *Preparador) leerCompatibilidad() compatibilidad {
	var c compatibilidad
	if datos, err := os.ReadFile(p.rutaCompatibilidad()); err == nil {
		_ = json.Unmarshal(datos, &c)
	}
	return c
}

// cambiar lee, cambia y escribe el archivo con su cerrojo: Marcar, LeerLista y Versiones no se pisan.
func (p *Preparador) cambiar(cambio func(c *compatibilidad)) error {
	p.mc.Lock()
	defer p.mc.Unlock()
	c := p.leerCompatibilidad()
	cambio(&c)
	datos, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return configuracion.EscribirAtomico(p.rutaCompatibilidad(), datos, 0o600)
}

// Comprobada dice si la versión del agente está comprobada para la acción: está en la receta o en la lista firmada
// (exacta o con «X.Y.*») y no está marcada como no compatible en este equipo. Una versión vacía (no se sabe) no lo está.
func (p *Preparador) Comprobada(agente, accion, version string) bool {
	f, ok := p.o.Ficha(agente)
	if !ok || f.Receta == nil || version == "" {
		return false
	}
	c := p.compatibilidad()
	if slices.Contains(c.NoCompatibles[agente], version) {
		return false
	}
	return cubre(f.Receta.Comprobadas[accion], version) || cubre(c.Lista.Agentes[agente][accion], version)
}

// cubre dice si alguna de la lista es la versión o un comodín de parche que la incluye («2.1.*» cubre 2.1.300, no 2.2.0).
func cubre(lista []string, version string) bool {
	return slices.ContainsFunc(lista, func(v string) bool {
		base, comodin := strings.CutSuffix(v, "*")
		return v == version || comodin && strings.HasSuffix(base, ".") && len(version) > len(base) && strings.HasPrefix(version, base)
	})
}

// Marcar anota en este equipo que esa versión del agente no es compatible (su inicio de sesión no se reconoció): deja de
// estar comprobada para todo hasta que una lista firmada más nueva la vuelva a traer.
func (p *Preparador) Marcar(agente, version string) error {
	if version == "" {
		return nil
	}
	return p.cambiar(func(c *compatibilidad) {
		if c.NoCompatibles == nil {
			c.NoCompatibles = map[string][]string{}
		}
		if !slices.Contains(c.NoCompatibles[agente], version) {
			c.NoCompatibles[agente] = append(c.NoCompatibles[agente], version)
		}
	})
}

// LeerLista baja la lista firmada y, si su firma vale y su version es mayor que la guardada, la guarda y quita las marcas
// de las versiones que vuelve a traer. Una firma que no vale o una lista igual o más vieja no cambian nada. Con
// servicio.buscar_versiones_agentes=false no hace nada.
func (p *Preparador) LeerLista(ctx context.Context) error {
	if !p.buscaVersiones() {
		return nil
	}
	datos, err := p.pedir(ctx, URLLista, 1<<20)
	if err != nil {
		return err
	}
	firma, err := p.pedir(ctx, URLLista+".sig", 1<<10)
	if err != nil {
		return err
	}
	if err := actualizacion.VerificarFirma(datos, firma); err != nil {
		return err
	}
	var nueva listaFirmada
	if err := json.Unmarshal(datos, &nueva); err != nil {
		return err
	}
	return p.cambiar(func(c *compatibilidad) {
		if nueva.Version <= c.Lista.Version {
			return
		}
		c.Lista = nueva
		for agente, marcadas := range c.NoCompatibles {
			marcadas = slices.DeleteFunc(marcadas, func(v string) bool {
				for _, lista := range nueva.Agentes[agente] {
					if cubre(lista, v) {
						return true
					}
				}
				return false
			})
			if len(marcadas) == 0 {
				delete(c.NoCompatibles, agente)
			} else {
				c.NoCompatibles[agente] = marcadas
			}
		}
	})
}

// masReciente es la versión exacta comprobada más reciente para instalar (por número, sin las marcadas), de la receta o
// de la lista: la que se fija con un instalador que lo admite. Vacía si no hay ninguna.
func (p *Preparador) masReciente(agente string, r *adaptadores.Receta) string {
	c := p.compatibilidad()
	mejor := ""
	for _, v := range slices.Concat(r.Comprobadas["instalar"], c.Lista.Agentes[agente]["instalar"]) {
		if reVersion.MatchString(v) && !slices.Contains(c.NoCompatibles[agente], v) && (mejor == "" || compararVersiones(v, mejor) > 0) {
			mejor = v
		}
	}
	return mejor
}

// compararVersiones compara por número cada parte (1.10.0 > 1.9.0); una parte que no es un número, como texto.
func compararVersiones(a, b string) int {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := range min(len(x), len(y)) {
		n, errN := strconv.Atoi(x[i])
		m, errM := strconv.Atoi(y[i])
		c := strings.Compare(x[i], y[i])
		if errN == nil && errM == nil {
			c = cmp.Compare(n, m)
		}
		if c != 0 {
			return c
		}
	}
	return cmp.Compare(len(x), len(y))
}
