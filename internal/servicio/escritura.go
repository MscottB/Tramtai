package servicio

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"

	"tramtai/internal/contrato"
	"tramtai/internal/version"
)

// EscribirDescriptor publica el descriptor y lo mantiene actualizado al cambiar instancias o puntos.
func (s *Servicio) EscribirDescriptor() error {
	s.mu.Lock()
	s.descriptor = true
	s.mu.Unlock()
	return s.actualizarDescriptor()
}

func (s *Servicio) actualizarDescriptor() error {
	s.mu.RLock()
	if !s.descriptor {
		s.mu.RUnlock()
		return nil
	}
	d := Descriptor{Identificador: s.ID, PID: os.Getpid(), FirmaInicio: strconv.Itoa(os.Getpid()) + "@" + s.IniciadoEn.UTC().Format(time.RFC3339Nano),
		IniciadoEn: s.IniciadoEn, Host: s.opc.Host, Puerto: s.opc.Puerto, Base: s.Base(), Version: version.Version,
		Instancias: []string{}, Puntos: []contrato.PuntoDescriptor{}}
	for id := range s.instancias {
		d.Instancias = append(d.Instancias, id)
	}
	sort.Strings(d.Instancias)
	for _, p := range s.estadoPuntos() {
		if p.Activo {
			d.Puntos = append(d.Puntos, contrato.PuntoDescriptor{Nombre: p.Nombre, URL: p.URL})
		}
	}
	s.mu.RUnlock()
	return escribirJSONArchivo(RutaDescriptor(s.opc.Datos), d)
}

// BorrarDescriptor elimina el descriptor si sigue siendo el de este servicio.
func (s *Servicio) BorrarDescriptor() {
	if d, err := LeerDescriptor(s.opc.Datos); err == nil && d.Identificador == s.ID {
		_ = os.Remove(RutaDescriptor(s.opc.Datos))
	}
}

// escribirJSONArchivo escribe de forma atómica (temporal y renombrado) con permisos del usuario.
func escribirJSONArchivo(ruta string, v any) error {
	datos, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return escribirAtomico(ruta, datos, 0o600)
}

// escritores serializa, dentro del proceso, las escrituras de cada ruta: en Windows dos renombrados a la vez sobre el
// mismo archivo pueden fallar.
var escritores sync.Map // ruta → *sync.Mutex

// escribirAtomico escribe datos en un temporal propio de la misma carpeta y lo renombra: quien lee a la vez (un Recargar,
// la CLI, la TUI) ve el archivo anterior o el nuevo, nunca uno a medias, y un corte a mitad no deja el archivo truncado.
func escribirAtomico(ruta string, datos []byte, permisos os.FileMode) error {
	if real, err := filepath.EvalSymlinks(ruta); err == nil {
		ruta = real // un config.yaml enlazado (dotfiles) sigue siendo un enlace: se escribe el archivo al que apunta
	}
	m, _ := escritores.LoadOrStore(filepath.Clean(ruta), &sync.Mutex{})
	m.(*sync.Mutex).Lock()
	defer m.(*sync.Mutex).Unlock()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(ruta), filepath.Base(ruta)+".*.tmp")
	if err != nil {
		return err
	}
	temporal := f.Name()
	_, err = f.Write(datos)
	if err == nil {
		err = f.Sync()
	}
	if cerrar := f.Close(); err == nil {
		err = cerrar
	}
	if err == nil {
		err = os.Chmod(temporal, permisos)
	}
	if err == nil {
		err = renombrar(temporal, ruta)
	}
	if err != nil {
		_ = os.Remove(temporal)
	}
	return err
}

// renombrar reintenta en Windows durante unos 600 ms: allí Rename falla mientras otro proceso tiene abierto el destino
// (un Recargar que lo lee, la CLI, la terminal o el antivirus). En los demás sistemas no hace falta.
func renombrar(origen, destino string) error {
	err := os.Rename(origen, destino)
	for espera := 5 * time.Millisecond; err != nil && runtime.GOOS == "windows" && espera <= 320*time.Millisecond; espera *= 2 {
		time.Sleep(espera)
		err = os.Rename(origen, destino)
	}
	return err
}
