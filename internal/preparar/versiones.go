package preparar

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"tramtai/internal/adaptadores"
	"tramtai/internal/contrato"
	"tramtai/internal/idioma"
	"tramtai/internal/version"
)

// Las fuentes de la última versión de un CLI (FuenteVersion «npm» y «github»); variables para las pruebas.
var (
	baseNPM    = "https://registry.npmjs.org/"
	baseGitHub = "https://api.github.com/repos/"
)

const (
	// plazoVersion acota cada consulta de la última versión (y la lista firmada).
	plazoVersion = 10 * time.Second
	// unDia es lo que dura lo consultado: la última versión de cada CLI y la lista se piden como mucho una vez al día.
	unDia = 24 * time.Hour
)

// buscaVersiones es servicio.buscar_versiones_agentes: con false no se pide nada por la red.
func (p *Preparador) buscaVersiones() bool {
	return p.o.BuscarVersiones == nil || p.o.BuscarVersiones()
}

// pedir baja url por HTTPS con su plazo y solo la cabecera User-Agent (nada que identifique al usuario). Cualquier
// respuesta que no sea 200 es un error (GitHub sin token responde 403 o 429 al pasar de 60 por hora).
func (p *Preparador) pedir(ctx context.Context, url string, limite int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, errorSinHTTPS(url)
	}
	ctx, cancelar := context.WithTimeout(ctx, plazoVersion)
	defer cancelar()
	pedido, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	pedido.Header.Set("User-Agent", "Tramtai/"+version.Texto())
	r, err := p.cliente.Do(pedido)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return nil, errors.New(idioma.Tf("%s respondió %d", url, r.StatusCode))
	}
	datos, err := io.ReadAll(io.LimitReader(r.Body, limite+1))
	if err == nil && int64(len(datos)) > limite {
		err = errors.New(idioma.Tf("%s es más grande de lo esperado", url))
	}
	return datos, err
}

// ultimaPublicada es el gancho UltimaVersion del propio preparador: consulta la FuenteVersion de la receta (npm: el campo
// version de /latest; github: el tag_name del último release; url: el texto o el campo version de su JSON), sin su
// Prefijo. Con la búsqueda desactivada no pide nada.
func (p *Preparador) ultimaPublicada(ctx context.Context, agente string) (string, error) {
	f, ok := p.o.Ficha(agente)
	if !ok || f.Receta == nil || !p.buscaVersiones() {
		return "", errors.New(idioma.Tf("no se pudo consultar la última versión de %s", agente))
	}
	fuente := f.Receta.Ultima
	url := fuente.Nombre
	switch fuente.Tipo {
	case "npm":
		url = baseNPM + fuente.Nombre + "/latest"
	case "github":
		url = baseGitHub + fuente.Nombre + "/releases/latest"
	case "url":
	default:
		return "", errors.New(idioma.Tf("no se pudo consultar la última versión de %s", f.Programa))
	}
	datos, err := p.pedir(ctx, url, 1<<20)
	if err != nil {
		return "", err
	}
	ultima := strings.TrimSpace(string(datos))
	if fuente.Tipo != "url" || strings.HasPrefix(ultima, "{") {
		var j struct {
			Version  string `json:"version"`
			Etiqueta string `json:"tag_name"`
		}
		if err := json.Unmarshal(datos, &j); err != nil {
			return "", err
		}
		ultima = cmp.Or(j.Version, j.Etiqueta)
	}
	ultima = strings.TrimPrefix(ultima, fuente.Prefijo)
	if err := validarVersion(ultima); err != nil {
		return "", err
	}
	return ultima, nil
}

// versionPara es la versión que usaría la acción: la instalada al iniciar sesión o desinstalar; la última publicada al
// actualizar con la orden de la receta; al instalar (o reinstalar), la comprobada más reciente si el instalador la fija
// y, si no, la última publicada. ultima solo se llama si hace falta; vacía si no se sabe.
func (p *Preparador) versionPara(agente string, r *adaptadores.Receta, accion, instalada string, ultima func() string) string {
	_, hayOrden := r.Actualizar[runtime.GOOS]
	switch {
	case accion == "iniciar_sesion" || accion == "desinstalar":
		return instalada
	case accion == "actualizar" && hayOrden && instalada != "":
		return ultima()
	}
	if r.FijaVersion {
		if v := p.masReciente(agente, r); v != "" {
			return v
		}
	}
	return ultima()
}

// Versiones dice, por cada agente con receta, la versión instalada, la última publicada, si la versión que usaría cada
// acción está comprobada y qué acciones están habilitadas. La última versión de cada CLI y la lista firmada se consultan
// como mucho una vez al día (leida, en compatibilidad.json), salvo con forzar, y nunca con la búsqueda desactivada. Sin
// red, Ultima queda vacía con sin_red: no es un error.
func (p *Preparador) Versiones(ctx context.Context, forzar bool) []contrato.VersionesAgente {
	consultar := p.buscaVersiones()
	if desde := time.Since(p.compatibilidad().Leida); consultar && (forzar || desde < 0 || desde >= unDia) {
		_ = p.LeerLista(ctx) // una firma que no vale o sin red: sigue la lista anterior
		nuevas := map[string]string{}
		preguntadas := 0
		var mu sync.Mutex
		p.enParalelo(func(id string, f adaptadores.Ficha) {
			v, err := p.o.UltimaVersion(ctx, id) // cada consulta con su plazo (pedir)
			mu.Lock()
			defer mu.Unlock()
			if f.Receta.Ultima.Tipo != "" {
				preguntadas++
			}
			if err == nil {
				nuevas[id] = v
			}
		})
		// Un fallo pasajero (sin red, 429) no borra lo que ya se sabía, y si no respondió ninguna se vuelve a intentar
		// en la próxima llamada.
		_ = p.cambiar(func(c *compatibilidad) {
			if c.Ultimas == nil {
				c.Ultimas = map[string]string{}
			}
			maps.Copy(c.Ultimas, nuevas)
			if len(nuevas) > 0 || preguntadas == 0 {
				c.Leida = time.Now()
			}
		})
	}
	ultimas := map[string]string{}
	if consultar {
		ultimas = p.compatibilidad().Ultimas
	}
	instaladas := map[string]string{}
	var mu sync.Mutex
	p.enParalelo(func(id string, f adaptadores.Ficha) {
		instalada := ""
		if ruta := buscar(f); ruta != "" {
			ctx, cancelar := context.WithTimeout(ctx, 30*time.Second)
			defer cancelar()
			instalada = numeroVersion(adaptadores.VersionDe(ctx, ruta))
		}
		mu.Lock()
		defer mu.Unlock()
		instaladas[id] = instalada
	})
	lista := []contrato.VersionesAgente{}
	for _, id := range p.o.Agentes {
		if f, ok := p.o.Ficha(id); ok && f.Receta != nil {
			lista = append(lista, p.versionesDe(id, f, instaladas[id], ultimas[id], consultar))
		}
	}
	return lista
}

// enParalelo llama a hacer con cada agente con receta a la vez y espera a todos.
func (p *Preparador) enParalelo(hacer func(id string, f adaptadores.Ficha)) {
	var grupo sync.WaitGroup
	for _, id := range p.o.Agentes {
		if f, ok := p.o.Ficha(id); ok && f.Receta != nil {
			grupo.Go(func() { hacer(id, f) })
		}
	}
	grupo.Wait()
}

func (p *Preparador) versionesDe(id string, f adaptadores.Ficha, instalada, ultima string, consultar bool) contrato.VersionesAgente {
	r := f.Receta
	v := contrato.VersionesAgente{Agente: id, Instalada: instalada, Ultima: ultima, Comprobadas: map[string]bool{}, Acciones: []string{}}
	posibles := []string{"instalar"}
	if instalada != "" {
		posibles = []string{"actualizar", "iniciar_sesion", "desinstalar"}
	}
	aviso := ""
	for _, accion := range posibles {
		if !disponible(r, accion) {
			continue
		}
		usada := p.versionPara(id, r, accion, instalada, func() string { return ultima })
		comprobada := p.o.Comprobada(id, accion, usada)
		v.Comprobadas[accion] = comprobada
		switch {
		case comprobada || accion == "desinstalar": // desinstalar ya pide confirmar siempre
		case accion == "iniciar_sesion": // sin comprobar, desactivado: ni con confirmar
			v.MotivoCodigo, v.Motivo = CodigoVersionNoComprobada, idioma.Tf("%s aún no está comprobado con Tramtai: el inicio de sesión automático vuelve cuando lo esté. Mientras, %s",
				conVersionSi(f.Programa, usada), p.aMano(f))
			continue
		case aviso != "":
		case r.Sesion == nil:
			aviso = idioma.Tf("%s aún no está comprobado con Tramtai", conVersionSi(f.Programa, usada))
		case usada == "": // búsqueda desactivada o sin red
			aviso = idioma.T("No se sabe qué versión se instalará, así que no se sabe si está comprobada: si sigues y no lo está, el inicio de sesión automático quedará desactivado hasta que lo esté")
		case accion == "actualizar":
			aviso = idioma.T("La versión nueva aún no está comprobada: si actualizas, el inicio de sesión automático quedará desactivado hasta que lo esté")
		default:
			aviso = idioma.Tf("%s aún no está comprobado con Tramtai: si sigues, el inicio de sesión automático quedará desactivado hasta que lo esté",
				conVersionSi(f.Programa, usada))
		}
		v.Acciones = append(v.Acciones, accion)
	}
	switch {
	case v.MotivoCodigo != "":
	case consultar && r.Ultima.Tipo != "" && ultima == "":
		v.MotivoCodigo, v.Motivo = CodigoSinRed, idioma.Tf("no se pudo consultar la última versión de %s", f.Programa)
	case aviso != "":
		v.MotivoCodigo, v.Motivo = CodigoVersionNoComprobada, aviso
	}
	return v
}
