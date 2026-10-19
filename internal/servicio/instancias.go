package servicio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"tramtai/internal/actualizacion"
	"tramtai/internal/api"
	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/persistencia"
	"tramtai/internal/planificador"
	"tramtai/internal/version"
)

// IDProyecto deriva un identificador estable del nombre del proyecto y de su ruta.
func IDProyecto(raiz string, c *nucleo.Configuracion) string {
	nombre := c.Texto("proyecto.nombre")
	if nombre == "" {
		nombre = filepath.Base(raiz)
	}
	var b strings.Builder
	for _, r := range strings.ToLower(nombre) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteByte('_')
		}
	}
	slug := strings.Trim(b.String(), "_")
	if len(slug) > 30 {
		slug = slug[:30]
	}
	if slug == "" {
		slug = "proyecto"
	}
	clave := filepath.Clean(raiz)
	if runtime.GOOS != "linux" { // Windows y macOS no distinguen mayúsculas en las rutas; Linux sí
		clave = strings.ToLower(clave)
	}
	suma := sha256.Sum256([]byte(clave))
	return slug + "_" + hex.EncodeToString(suma[:4])
}

// ActivarGlobal activa la instancia global con la configuración global.
func (s *Servicio) ActivarGlobal() (*Instancia, error) {
	return s.activar("global", "global", "", s.opc.Config)
}

// ActivarProyecto activa la instancia del proyecto cuya raíz contiene .tramtai/config.yaml.
func (s *Servicio) ActivarProyecto(raiz string) (*Instancia, error) {
	raiz, err := filepath.Abs(raiz)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(raiz, ".tramtai", "config.yaml")); err != nil {
		return nil, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, idioma.Tf("%s no tiene .tramtai/config.yaml: ejecuta tramtai iniciar en el proyecto", raiz))
	}
	c, err := nucleo.CargarConfiguracion(raiz)
	if err != nil {
		return nil, err
	}
	return s.activar(IDProyecto(raiz, c), "proyecto", raiz, c)
}

// activar abre la instancia si no está activa. La base se abre (migraciones, huérfanos, recuperación) fuera de mu: así
// no se detienen las peticiones de las demás instancias, y quien pida la misma a la vez espera y recibe la misma.
func (s *Servicio) activar(id, ambito, raiz string, c *nucleo.Configuracion) (*Instancia, error) {
	s.mu.Lock()
	if inst, activa := s.instancias[id]; activa {
		s.mu.Unlock()
		return inst, nil
	}
	if a, enCurso := s.abriendo[id]; enCurso {
		s.mu.Unlock()
		<-a.listo
		return a.inst, a.err
	}
	a := &apertura{listo: make(chan struct{})}
	s.abriendo[id] = a
	s.mu.Unlock()

	a.inst, a.err = s.abrirInstancia(id, ambito, raiz, c)
	var sobra *Instancia // abierta mientras el servicio se cerraba: se cierra fuera de mu
	s.mu.Lock()
	delete(s.abriendo, id)
	if a.err == nil && s.cerrado {
		sobra = a.inst
		a.inst, a.err = nil, nucleo.NuevoError(nucleo.CodigoServicioDetenido, idioma.T("el servicio se está deteniendo"))
	}
	if a.err == nil {
		s.instancias[id] = a.inst
	}
	s.mu.Unlock()
	if sobra != nil {
		sobra.cerrar(false)
	}
	close(a.listo)
	if a.err != nil {
		return nil, a.err
	}
	s.Recargar() // abre los puntos de acceso de la instancia
	return a.inst, nil
}

func (s *Servicio) abrirInstancia(id, ambito, raiz string, c *nucleo.Configuracion) (*Instancia, error) {
	if err := os.MkdirAll(filepath.Join(s.opc.Datos, "instancias"), 0o700); err != nil {
		return nil, err
	}
	alm, err := abrirBase(filepath.Join(s.opc.Datos, "instancias", id+".db"))
	if err != nil {
		return nil, err
	}
	if alm.Compactada {
		fmt.Fprintf(s.opc.Registros, "se compactó la base de %s: desde ahora devuelve al disco lo que libera la purga\n", id)
	}
	if horas := c.Entero("retencion.horas_idempotencia"); horas > 0 {
		alm.VentanaIdempotencia = time.Duration(horas) * time.Hour
	}
	cargas, err := documentos.AbrirCargas(filepath.Join(s.opc.Datos, "cargas", id),
		time.Duration(c.Entero("retencion.horas_carga_huerfana"))*time.Hour, int64(c.Limite("conversion.maximo_carga_mb"))<<20,
		int64(c.Limite("conversion.cuota_cargas_mb"))<<20)
	if err != nil {
		alm.Cerrar()
		return nil, err
	}
	token, archivo, err := TokenInstancia(s.opc.Datos, id, c)
	if err != nil {
		alm.Cerrar()
		return nil, err
	}
	opc := planificador.OpcionesDesde(c, id)
	opc.Registros = s.opc.Registros
	if c.ProyectoConfiable(raiz) {
		opc.RaizConfiable = raiz
	}
	opc.Preparador = documentos.Preparador{Cargas: cargas, Conversor: documentos.Desde(c)}
	p := planificador.Nuevo(alm, s.opc.Registro, s.opc.Semaforos, opc)
	if err := p.Iniciar(context.Background()); err != nil {
		alm.Cerrar()
		return nil, err
	}
	srv := &api.Servidor{Planificador: p, Cargas: cargas, Registro: s.opc.Registro, Config: c,
		PuntosDeGrupo: func(grupo string) []string {
			var nombres []string
			for _, punto := range s.puntosDe(id) {
				if punto.Grupo == grupo {
					nombres = append(nombres, punto.Nombre)
				}
			}
			return nombres
		}}
	inst := &Instancia{ID: id, Ambito: ambito, Raiz: raiz, Config: c, Planificador: p, Cargas: cargas,
		ArchivoToken: archivo, token: token, almacen: alm, rutas: srv.Rutas()}
	inst.iniciarPurga(s.opc.Registros)
	return inst, nil
}

// iniciarPurga aplica la retención al abrir la instancia y después cada intervaloPurga.
func (i *Instancia) iniciarPurga(registros io.Writer) {
	ctx, parar := context.WithCancel(context.Background())
	i.pararPurga, i.purgaParada, i.purgarYa = parar, make(chan struct{}), make(chan struct{}, 1)
	go func() {
		defer close(i.purgaParada)
		reloj := time.NewTicker(intervaloPurga)
		defer reloj.Stop()
		avisado := false
		for {
			if err := i.Purgar(ctx); err != nil && ctx.Err() == nil {
				fmt.Fprintf(registros, "purga de %s: %s\n", i.ID, nucleo.MensajeDe(err))
			}
			if r, m, x, ajustada := i.retencion(); ajustada && !avisado {
				fmt.Fprintf(registros, "retencion de %s fuera de rango; se usa horas_resultado %d, horas_metadatos %d y max_trabajos %d\n", i.ID, r, m, x)
				avisado = true
			}
			select {
			case <-ctx.Done():
				return
			case <-reloj.C:
			case <-i.purgarYa:
			}
		}
	}()
}

// purgarPronto pide una purga sin esperarla. La hace el bucle de la instancia, así que cerrar (Desactivar, Cerrar)
// la espera y nunca corre sobre una base cerrada.
func (i *Instancia) purgarPronto() {
	select {
	case i.purgarYa <- struct{}{}:
	default: // ya había una pedida, o la instancia no tiene bucle de purga
	}
}

// Purgar aplica la retención de la instancia: vence resultados y entradas, borra metadatos antiguos y las
// cargas vencidas que ningún trabajo necesita.
func (i *Instancia) Purgar(ctx context.Context) error {
	return PurgarBase(ctx, i.almacen, i.Cargas, i.Config, i.ID)
}

// PurgarBase aplica la retención de c a las instancias de una base y a sus cargas. La usan cada instancia del servicio
// y tramtai ejecutar --directo, cuya base guarda una instancia por ejecución.
func PurgarBase(ctx context.Context, alm *persistencia.Almacen, cargas *documentos.Cargas, c *nucleo.Configuracion, instancias ...string) error {
	resultado, metadatos, maximo, _ := nucleo.AjustarRetencion(c.Entero("retencion.horas_resultado"), c.Entero("retencion.horas_metadatos"),
		c.Entero("retencion.max_trabajos"))
	r := persistencia.Retencion{Resultado: time.Duration(resultado) * time.Hour, Metadatos: time.Duration(metadatos) * time.Hour,
		Maximo: maximo, EliminarEntradas: c.Booleano("retencion.eliminar_entradas_al_terminar")}
	enUso := map[string]bool{}
	for _, id := range instancias {
		if _, _, err := alm.Purgar(ctx, id, r); err != nil {
			return err
		}
		usadas, err := alm.CargasEnUso(ctx, id)
		if err != nil {
			return err
		}
		maps.Copy(enUso, usadas)
	}
	_, err := cargas.Purgar(enUso, time.Now())
	return err
}

// retencion da los plazos (horas) y el tope de la instancia, llevados a su rango; ajustada indica que la
// configuración traía algún valor fuera de él.
func (i *Instancia) retencion() (resultado, metadatos, maximo int, ajustada bool) {
	return nucleo.AjustarRetencion(i.Config.Entero("retencion.horas_resultado"), i.Config.Entero("retencion.horas_metadatos"),
		i.Config.Entero("retencion.max_trabajos"))
}

// Desactivar detiene una instancia sin afectar a las demás: vacía su cola (cancela lo que no empezó), lo que
// estaba en ejecución queda interrumpido y se cierran sus puntos de acceso.
func (s *Servicio) Desactivar(id string) error {
	s.mu.Lock()
	inst, activa := s.instancias[id]
	delete(s.instancias, id)
	s.mu.Unlock()
	if !activa {
		return nucleo.NuevoError(nucleo.CodigoNoEncontrado, idioma.Tf("la instancia %s no está activa", id))
	}
	s.Recargar()
	inst.cerrar(true)
	return nil
}

// cerrar detiene la purga y el planificador (interrumpe lo que aún se ejecuta) y cierra la base.
func (i *Instancia) cerrar(vaciarCola bool) {
	if i.pararPurga != nil {
		i.pararPurga()
		<-i.purgaParada
	}
	if vaciarCola {
		ctx := context.Background()
		// Esperando al agente también es cola: el trabajo aún no tiene plaza ni agente.
		if ids, err := i.almacen.ListarPorEstado(ctx, i.ID, nucleo.EnCola, nucleo.EsperandoAgente); err == nil {
			for _, id := range ids {
				_, _ = i.Planificador.Cancelar(ctx, id)
			}
		}
	}
	i.Planificador.Detener()
	i.almacen.Cerrar()
}

// activas es la lista de instancias que se restauran al reiniciar el servicio.
type activas struct {
	Global    bool     `json:"global"`
	Proyectos []string `json:"proyectos"`
}

func (s *Servicio) guardarActivas() error {
	s.mu.RLock()
	a := activas{Proyectos: []string{}}
	for _, inst := range s.instancias {
		if inst.Ambito == "global" {
			a.Global = true
		} else {
			a.Proyectos = append(a.Proyectos, inst.Raiz)
		}
	}
	s.mu.RUnlock()
	sort.Strings(a.Proyectos)
	return escribirJSONArchivo(filepath.Join(s.opc.Datos, "instancias.json"), a)
}

// Restaurar activa las instancias que estaban activas (la global, si no hay registro previo) y devuelve los
// errores de las que no pudieron abrirse, por ejemplo un proyecto movido.
func (s *Servicio) Restaurar() []error {
	a := activas{Global: true}
	ruta := filepath.Join(s.opc.Datos, "instancias.json")
	var errores []error
	if datos, err := os.ReadFile(ruta); err == nil {
		if err := json.Unmarshal(datos, &a); err != nil {
			// Dañado: se aparta (para poder mirarlo) y se abre la global, como sin registro previo.
			a = activas{Global: true}
			texto := idioma.Tf("%s no se pudo leer (%s): se apartó como instancias.json.dañado y se abre solo la instancia global", ruta, err.Error())
			if errApartar := os.Rename(ruta, ruta+".dañado"); errApartar != nil {
				texto = idioma.Tf("%s no se pudo leer (%s) ni apartar (%s): se abre solo la instancia global y se reescribirá", ruta, err.Error(), errApartar.Error())
			}
			errores = append(errores, nucleo.NuevoError(nucleo.CodigoConfiguracionInvalida, texto))
		}
	}
	if a.Global {
		if _, err := s.ActivarGlobal(); err != nil {
			errores = append(errores, err)
		}
	}
	for _, raiz := range a.Proyectos {
		if _, err := s.ActivarProyecto(raiz); err != nil {
			errores = append(errores, err)
		}
	}
	return errores
}

// Estado devuelve el estado actual. Las bases se consultan sin s.mu: un Lock pendiente bloquearía todas las peticiones.
func (s *Servicio) Estado(ctx context.Context) Estado {
	var nueva *actualizacion.Version
	if v, ok := s.versionNueva(context.Background(), false); ok {
		nueva = &v
	}
	s.mu.RLock()
	e := Estado{Identificador: s.ID, Version: version.Texto(), PID: os.Getpid(), IniciadoEn: s.IniciadoEn, Base: s.Base(),
		Instancias: []EstadoInstancia{}, Puntos: s.estadoPuntos(), Errores: s.erroresPuntos, Actualizacion: nueva}
	instancias := make([]*Instancia, 0, len(s.instancias))
	for _, inst := range s.instancias {
		instancias = append(instancias, inst)
	}
	s.mu.RUnlock()
	for _, inst := range instancias {
		e.Instancias = append(e.Instancias, inst.estado(ctx))
	}
	sort.Slice(e.Instancias, func(i, j int) bool { return e.Instancias[i].ID < e.Instancias[j].ID })
	return e
}

// estado resume la instancia; el conteo de trabajos se abandona a los 2 s para no colgar a quien pregunta.
func (i *Instancia) estado(ctx context.Context) EstadoInstancia {
	ctx, cancelar := context.WithTimeout(ctx, 2*time.Second)
	defer cancelar()
	conteo := map[string]int{}
	if porEstado, err := i.Planificador.Contar(ctx); err == nil {
		for estado, n := range porEstado {
			conteo[string(estado)] = n
		}
	}
	return EstadoInstancia{ID: i.ID, Ambito: i.Ambito, Raiz: i.Raiz, ArchivoToken: i.ArchivoToken,
		Agente: i.Config.Texto("predeterminados.agente"), Perfil: i.Config.Texto("predeterminados.perfil_modelo"), Trabajos: conteo}
}
