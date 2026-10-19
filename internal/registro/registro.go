// Package registro escribe el registro del servicio en segundo plano con un tamaño máximo: al superarlo, el
// archivo pasa a ser la copia .1 (la .1 pasa a .2, y así hasta servicio.registro_copias) y se empieza otro.
package registro

import (
	"fmt"
	"os"
	"sync"
)

// Rotativo es un io.Writer seguro entre goroutines que limita el tamaño del archivo de registro.
type Rotativo struct {
	mu     sync.Mutex
	ruta   string
	maximo int64 // bytes por archivo; 0 = sin límite
	copias int   // copias anteriores que se conservan; 0 = el archivo se vacía al llenarse
	f      *os.File
	tamano int64
}

// Abrir abre (o crea) el registro en ruta, añadiendo al final.
func Abrir(ruta string, maximo int64, copias int) (*Rotativo, error) {
	r := &Rotativo{ruta: ruta, maximo: maximo, copias: max(0, copias)}
	return r, r.abrir(false)
}

func (r *Rotativo) abrir(vaciar bool) error {
	modo := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	if vaciar {
		modo |= os.O_TRUNC
	}
	f, err := os.OpenFile(r.ruta, modo, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	r.f, r.tamano = f, info.Size()
	return nil
}

// Write escribe b y rota antes si el archivo superaría el máximo.
func (r *Rotativo) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.maximo > 0 && r.tamano > 0 && r.tamano+int64(len(b)) > r.maximo {
		if err := r.rotar(); err != nil {
			return 0, err
		}
	}
	n, err := r.f.Write(b)
	r.tamano += int64(n)
	return n, err
}

// rotar desplaza las copias y empieza un archivo vacío. Si no se puede renombrar (en Windows, cuando otro
// proceso lo tiene abierto, como «tramtai servicio registro -f»), el archivo actual se vacía.
func (r *Rotativo) rotar() error {
	r.f.Close()
	vaciar := r.copias == 0
	if !vaciar {
		_ = os.Remove(copia(r.ruta, r.copias))
		for i := r.copias - 1; i >= 1; i-- {
			_ = os.Rename(copia(r.ruta, i), copia(r.ruta, i+1))
		}
		vaciar = os.Rename(r.ruta, copia(r.ruta, 1)) != nil
	}
	err := r.abrir(vaciar)
	if err == nil {
		return nil
	}
	// Sin archivo nuevo el registro se perdería en silencio: se sigue en la copia más reciente y se anota por qué.
	f, err2 := os.OpenFile(copia(r.ruta, 1), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err2 != nil {
		return err
	}
	r.f, r.tamano = f, 0
	fmt.Fprintf(f, "no se pudo rotar el registro %s: %v\n", r.ruta, err)
	return nil
}

func copia(ruta string, n int) string { return fmt.Sprintf("%s.%d", ruta, n) }

// Close cierra el archivo.
func (r *Rotativo) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}
