package servicio

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"time"

	"tramtai/internal/configuracion"
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
		IniciadoEn: s.IniciadoEn, Host: s.opc.Host, Puerto: s.opc.Puerto, Base: s.BaseLocal(), Version: version.Version,
		Instancias: []string{}, Puntos: []contrato.PuntoDescriptor{}}
	if publica := s.BasePublica(); publica != d.Base {
		d.BasePublica = publica
	}
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
	return configuracion.EscribirAtomico(ruta, datos, 0o600)
}
