package servicio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tramtai/internal/agentes"
	"tramtai/internal/carpetas"
	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

const plantillaProyecto = `# Configuración de Tramtai para este proyecto (versionada). Las claves de seguridad, los puertos y los
# puntos de acceso van en la configuración global, o en .tramtai/local.yaml (no versionado) si la carpeta del
# proyecto figura en proyectos_confiables de la configuración global.
version: 1
proyecto:
  nombre: %q
predeterminados:
  agente: claude
  perfil_modelo: equilibrado
  modo_contexto: aislado
  formato_salida: markdown
`

// PrepararProyecto crea .tramtai/ en dir: la configuración versionada, su .gitignore (local.yaml no se versiona)
// y un perfil de ejemplo con su esquema.
func PrepararProyecto(dir string) error {
	ruta := filepath.Join(dir, ".tramtai", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		return err
	}
	if err := escribirSiFalta(ruta, fmt.Sprintf(plantillaProyecto, filepath.Base(dir))); err != nil {
		return err
	}
	if err := escribirSiFalta(filepath.Join(dir, ".tramtai", ".gitignore"), gitignoreProyecto); err != nil {
		return err
	}
	// Un perfil de ejemplo con su esquema: tramtai ejecutar --perfil-trabajo resumen "…"
	for nombre, contenido := range map[string]string{
		filepath.Join("profiles", "resumen.yaml"):       perfilEjemplo,
		filepath.Join("schemas", "resumen.schema.json"): esquemaEjemplo,
	} {
		destino := filepath.Join(dir, ".tramtai", nombre)
		if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
			return err
		}
		if err := escribirSiFalta(destino, contenido); err != nil {
			return err
		}
	}
	_, err := configuracion.CargarCapa(ruta, configuracion.OrigenProyecto)
	return err
}

// escribirSiFalta crea ruta con contenido solo si no existe: lo que una carpeta ya traía no se pisa.
func escribirSiFalta(ruta, contenido string) error {
	f, err := os.OpenFile(ruta, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := f.WriteString(contenido); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// AbrirOpciones son las respuestas a lo que pregunta AbrirProyecto: si se confía en una carpeta que ya traía .tramtai/
// (nil: aún no se preguntó) y si se crea un proyecto en una carpeta que está dentro de otro.
type AbrirOpciones struct {
	Confiar   *bool
	CrearAqui bool
}

// ErrorConfianza: la carpeta ya traía .tramtai/ (por ejemplo, un repositorio clonado) y no es confiable; Claves son
// las de seguridad de su local.yaml, para enseñarlas antes de decidir. Movido es dónde estaba antes si su marca es la de
// un proyecto registrado que ya no está: la confianza no viaja sola, porque una marca se puede copiar.
type ErrorConfianza struct {
	Raiz   string
	Claves []string
	Movido string
}

func (e *ErrorConfianza) Error() string { return e.Unwrap().Error() }

func (e *ErrorConfianza) Unwrap() error {
	if e.Movido != "" {
		return nucleo.NuevoError(nucleo.CodigoConfianzaRequerida, idioma.Tf("%s parece el proyecto que estaba en %s: decide si confías en esta carpeta", e.Raiz, e.Movido))
	}
	return nucleo.NuevoError(nucleo.CodigoConfianzaRequerida, idioma.Tf("%s ya tiene configuración de Tramtai: decide si confías en esta carpeta", e.Raiz))
}

// ErrorDentroDeProyecto: la carpeta elegida está dentro de otro proyecto (Proyecto); hay que elegir cuál abrir.
type ErrorDentroDeProyecto struct {
	Proyecto string
}

func (e *ErrorDentroDeProyecto) Error() string { return e.Unwrap().Error() }

func (e *ErrorDentroDeProyecto) Unwrap() error {
	return nucleo.NuevoError(nucleo.CodigoDentroDeProyecto, idioma.Tf("la carpeta está dentro del proyecto %s", e.Proyecto))
}

// AbrirProyecto activa el espacio del proyecto en raiz, una carpeta existente con ruta completa. Si no tenía .tramtai/,
// lo crea y queda confiado (Tramtai lo escribió entero). Si ya traía .tramtai/ (aunque le falte config.yaml) y no es
// confiable, pregunta (ErrorConfianza) salvo que o.Confiar lo responda. Si está dentro de otro proyecto, pregunta
// (ErrorDentroDeProyecto) salvo que o.CrearAqui.
func (s *Servicio) AbrirProyecto(raiz string, o AbrirOpciones) (*Instancia, error) {
	raiz = carpetas.ExpandirCasa(strings.TrimSpace(raiz))
	if !filepath.IsAbs(raiz) {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.T("indica la ruta completa de la carpeta del proyecto"))
	}
	info, err := os.Stat(raiz)
	if err != nil || !info.IsDir() {
		return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("no existe la carpeta %s", raiz))
	}
	// La ruta real (sin nombres cortos como ADMINI~1 ni enlaces): la misma carpeta da siempre el mismo espacio.
	if real, err := filepath.EvalSymlinks(raiz); err == nil {
		raiz = real
	}
	existente, esProyecto := configuracion.BuscarProyecto(raiz)
	if esProyecto && !carpetas.MismaRuta(existente, raiz) && !o.CrearAqui {
		return nil, &ErrorDentroDeProyecto{Proyecto: existente}
	}
	nuevo := !esProyecto || !carpetas.MismaRuta(existente, raiz)
	if !nuevo {
		raiz = existente
	}
	_, err = os.Stat(filepath.Join(raiz, ".tramtai"))
	creada := os.IsNotExist(err)
	if !creada {
		if g, err := configuracion.CargarConfiguracion("", agentes.Internos()); err == nil && !g.ProyectoConfiable(raiz) {
			if o.Confiar == nil {
				claves, _ := configuracion.ClavesAdministradas(filepath.Join(raiz, ".tramtai", "local.yaml"))
				return nil, &ErrorConfianza{Raiz: raiz, Claves: claves, Movido: s.rutaMovida(raiz)}
			}
			if *o.Confiar {
				if err := s.FijarConfianza(raiz, true); err != nil {
					return nil, err
				}
			}
		}
	}
	if nuevo {
		if err := PrepararProyecto(raiz); err != nil {
			return nil, nucleo.NuevoError(nucleo.CodigoSolicitudInvalida, idioma.Tf("no se pudo preparar el proyecto: %s", err.Error()))
		}
		if creada {
			if err := s.FijarConfianza(raiz, true); err != nil {
				return nil, err
			}
		}
	}
	return s.ActivarProyecto(raiz)
}

// FijarConfianza añade raiz a proyectos_confiables de la configuración global (o la quita) y, si su espacio está
// activo, le aplica su configuración de nuevo: sus APIs y su presupuesto entran o salen sin reiniciar.
func (s *Servicio) FijarConfianza(raiz string, confiar bool) error {
	s.muConfianza.Lock()
	defer s.muConfianza.Unlock()
	ruta, err := configuracion.RutaGlobal()
	if err != nil {
		return err
	}
	g, err := configuracion.CargarConfiguracion("", agentes.Internos())
	if err != nil {
		return err
	}
	lista := []string{}
	for _, v := range g.Lista("proyectos_confiables") {
		if texto, ok := v.(string); ok && !carpetas.MismaRuta(texto, raiz) {
			lista = append(lista, texto)
		}
	}
	if confiar {
		lista = append(lista, raiz)
	}
	if err := configuracion.FijarValor(ruta, configuracion.OrigenGlobal, "proyectos_confiables", lista); err != nil {
		return err
	}
	var inst *Instancia
	s.mu.RLock()
	for _, i := range s.instancias {
		if i.Ambito == "proyecto" && carpetas.MismaRuta(i.Raiz, raiz) {
			inst = i
		}
	}
	s.mu.RUnlock()
	if inst != nil {
		c, err := configuracion.CargarConfiguracion(inst.Raiz, agentes.Internos())
		if err != nil {
			return err
		}
		inst.Config.Reemplazar(c)
		s.anotarIgnoradas(inst.ID, c)
		s.Recargar()
	}
	return nil
}

// anotarIgnoradas deja en el registro el aviso de las claves de seguridad que no se aplicaron (o lo quita).
func (s *Servicio) anotarIgnoradas(id string, c *configuracion.Configuracion) {
	texto := ""
	if ignoradas := c.Ignoradas(); len(ignoradas) > 0 {
		texto = idioma.Tf("local.yaml tiene claves que solo valen en carpetas confiables y no se aplicaron: %s", strings.Join(ignoradas, ", "))
	}
	s.registro().cambiar(id, func(e *Espacio) { e.UltimoError = texto })
	_ = s.registro().guardar()
}

const perfilEjemplo = `nombre: resumen
modo: perfil
instrucciones: |
  Resume el contenido en tres puntos y lista los pendientes. No inventes información.
salida:
  formato: json
  validacion_estricta: true
  esquema: ../schemas/resumen.schema.json
  conservar_salida_invalida: false
`

const esquemaEjemplo = `{
  "type": "object",
  "required": ["puntos", "pendientes"],
  "properties": {
    "puntos": {"type": "array", "items": {"type": "string"}, "maxItems": 3},
    "pendientes": {"type": "array", "items": {"type": "string"}}
  },
  "additionalProperties": false
}
`
