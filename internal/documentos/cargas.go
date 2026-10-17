// Package documentos prepara las entradas de un trabajo: guarda las cargas, las convierte a Markdown con
// markitdown y las ensambla con delimitadores explícitos.
package documentos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// Carga es un archivo recibido, identificado por su contenido.
type Carga struct {
	ID       string    `json:"identificador_carga"`
	Punto    string    `json:"punto_acceso,omitempty"` // la API que la subió; vacío, la instancia
	Nombre   string    `json:"nombre"`
	SHA256   string    `json:"sha256"`
	Bytes    int64     `json:"tamano_bytes"`
	CreadoEn time.Time `json:"creado_en"`
	VenceEn  time.Time `json:"vence_en"`
}

// Cargas guarda las cargas de una instancia en un directorio privado: el contenido en <id> y los metadatos en <id>.json.
// Una carga vence a las horas_carga_huerfana salvo que un trabajo la necesite todavía (Purgar).
type Cargas struct {
	dir         string
	vigencia    time.Duration
	maximoBytes int64
	cuota       int64      // bytes que pueden ocupar todas las cargas guardadas (0: sin cuota)
	mu          sync.Mutex // comprobar la cuota y guardar van juntos: dos subidas a la vez no la superan
}

// AbrirCargas crea el directorio si no existe.
func AbrirCargas(dir string, vigencia time.Duration, maximoBytes, cuota int64) (*Cargas, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Cargas{dir: dir, vigencia: vigencia, maximoBytes: maximoBytes, cuota: cuota}, nil
}

// Guardar copia el contenido calculando su hash. El identificador sale del contenido y de la API que lo sube (punto,
// vacío para la instancia): el mismo contenido de la misma API da el mismo identificador y otra API no puede
// citar ni pisar el suyo.
func (c *Cargas) Guardar(nombre, punto string, r io.Reader) (Carga, error) {
	temporal, err := os.CreateTemp(c.dir, "subida-*")
	if err != nil {
		return Carga{}, err
	}
	defer os.Remove(temporal.Name()) // sin efecto si ya se renombró
	resumen := sha256.New()
	n, err := io.Copy(io.MultiWriter(temporal, resumen), io.LimitReader(r, c.maximoBytes+1))
	if cerrar := temporal.Close(); err == nil {
		err = cerrar
	}
	if err != nil {
		return Carga{}, err
	}
	if n > c.maximoBytes {
		return Carga{}, nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.T("el archivo supera el tamaño máximo de carga"))
	}
	suma := hex.EncodeToString(resumen.Sum(nil))
	propio := sha256.Sum256([]byte(punto + "\x00" + suma))
	ahora := time.Now().UTC()
	carga := Carga{ID: "carga_" + hex.EncodeToString(propio[:])[:32], Punto: punto, Nombre: nombreSeguro(nombre), SHA256: suma, Bytes: n, CreadoEn: ahora, VenceEn: ahora.Add(c.vigencia)}
	c.mu.Lock()
	defer c.mu.Unlock()
	if anterior, _, err := c.Obtener(carga.ID); err == nil {
		// Ya estaba: solo se renueva su plazo; el nombre (y con él el formato) es el de la primera subida.
		anterior.VenceEn = carga.VenceEn
		carga = anterior
	} else if c.cuota > 0 && c.ocupado()+n > c.cuota {
		return Carga{}, nucleo.NuevoError(nucleo.CodigoCuotaCargasAgotada, idioma.T("las cargas guardadas llegan a conversion.cuota_cargas_mb: espera a que venzan o borra el historial"))
	}
	// Los metadatos van antes que el contenido: una purga concurrente nunca ve contenido sin metadatos.
	metadatos, _ := json.Marshal(carga)
	if err := c.escribir(carga.ID+".json", metadatos); err != nil {
		return Carga{}, err
	}
	return carga, os.Rename(temporal.Name(), filepath.Join(c.dir, carga.ID))
}

// ocupado suma lo que ocupan las cargas guardadas (sin metadatos ni subidas a medias).
func (c *Cargas) ocupado() int64 {
	entradas, _ := os.ReadDir(c.dir)
	var total int64
	for _, e := range entradas {
		if nombre := e.Name(); strings.HasPrefix(nombre, "carga_") && !strings.HasSuffix(nombre, ".json") {
			if info, err := e.Info(); err == nil {
				total += info.Size()
			}
		}
	}
	return total
}

// escribir crea o reemplaza un archivo de golpe (temporal + renombrado): nunca se lee a medias.
func (c *Cargas) escribir(nombre string, datos []byte) error {
	temporal, err := os.CreateTemp(c.dir, "subida-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporal.Name()) // sin efecto si ya se renombró
	_, err = temporal.Write(datos)
	if cerrar := temporal.Close(); err == nil {
		err = cerrar
	}
	if err != nil {
		return err
	}
	return os.Rename(temporal.Name(), filepath.Join(c.dir, nombre))
}

// Obtener devuelve la carga y la ruta de su contenido, o entrada_no_disponible si no existe. No mira VenceEn:
// un trabajo ya aceptado protege sus cargas hasta que vence su resultado (Purgar); Vigente lo comprueba al aceptar.
func (c *Cargas) Obtener(id string) (Carga, string, error) {
	noDisponible := nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.Tf("la carga %s no existe o venció; vuelve a enviarla", id))
	if !nucleo.ReCarga.MatchString(id) {
		return Carga{}, "", noDisponible
	}
	datos, err := os.ReadFile(filepath.Join(c.dir, id+".json"))
	if err != nil {
		return Carga{}, "", noDisponible
	}
	var carga Carga
	if json.Unmarshal(datos, &carga) != nil {
		return Carga{}, "", noDisponible
	}
	ruta := filepath.Join(c.dir, id)
	if _, err := os.Stat(ruta); err != nil {
		return Carga{}, "", noDisponible
	}
	return carga, ruta, nil
}

// Vigente comprueba que la carga exista y no haya vencido; se usa al aceptar un trabajo nuevo.
func (c *Cargas) Vigente(id string) error {
	carga, _, err := c.Obtener(id)
	if err == nil && time.Now().After(carga.VenceEn) {
		err = nucleo.NuevoError(nucleo.CodigoEntradaNoDisponible, idioma.Tf("la carga %s venció; vuelve a enviarla", id))
	}
	return err
}

// gracia protege lo recién subido: un cliente puede citar la carga en un trabajo poco después de subirla.
const gracia = time.Hour

// Purgar borra las cargas vencidas que ningún trabajo necesita (enUso) y las subidas a medias de más de una hora.
// Devuelve cuántas cargas borró.
func (c *Cargas) Purgar(enUso map[string]bool, ahora time.Time) (int, error) {
	return c.purgar(enUso, func(carga Carga) bool { return !ahora.Before(carga.VenceEn) })
}

// PurgarSinUso borra las cargas que ningún trabajo necesita aunque no hayan vencido, salvo las subidas en la
// última hora (gracia): otro cliente puede estar a punto de enviar el trabajo que las cita.
func (c *Cargas) PurgarSinUso(enUso map[string]bool) (int, error) {
	return c.purgar(enUso, func(carga Carga) bool { return time.Since(carga.CreadoEn) > gracia })
}

// purgar borra las cargas sin uso para las que borrable es cierto. Lo que no se puede leer (metadatos dañados
// o ausentes) solo se borra pasada la gracia, contada desde la fecha del archivo.
func (c *Cargas) purgar(enUso map[string]bool, borrable func(Carga) bool) (int, error) {
	entradas, err := os.ReadDir(c.dir)
	if err != nil {
		return 0, err
	}
	antiguo := func(e os.DirEntry) bool {
		info, err := e.Info()
		return err == nil && time.Since(info.ModTime()) > gracia
	}
	borradas := 0
	for _, e := range entradas {
		nombre := e.Name()
		ruta := filepath.Join(c.dir, nombre)
		switch {
		case strings.HasPrefix(nombre, "subida-"):
			if antiguo(e) {
				_ = os.Remove(ruta)
			}
		case strings.HasSuffix(nombre, ".json"):
			id := strings.TrimSuffix(nombre, ".json")
			if enUso[id] {
				continue
			}
			var carga Carga
			datos, err := os.ReadFile(ruta)
			legible := err == nil && json.Unmarshal(datos, &carga) == nil
			if (legible && !borrable(carga)) || (!legible && !antiguo(e)) {
				continue
			}
			// Primero el contenido: si falla, los metadatos siguen y la próxima purga lo reintenta.
			if err := os.Remove(filepath.Join(c.dir, id)); err != nil && !os.IsNotExist(err) {
				return borradas, err
			}
			if err := os.Remove(ruta); err != nil {
				return borradas, err
			}
			borradas++
		default:
			if _, err := os.Stat(ruta + ".json"); os.IsNotExist(err) && !enUso[nombre] && antiguo(e) {
				_ = os.Remove(ruta) // contenido sin metadatos: una subida que no terminó de registrarse
			}
		}
	}
	return borradas, nil
}

// nombreSeguro conserva solo el nombre base, sin caracteres de control y con longitud acotada.
func nombreSeguro(nombre string) string {
	nombre = filepath.Base(strings.ReplaceAll(nombre, "\\", "/"))
	nombre = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, nombre)
	if runas := []rune(nombre); len(runas) > 200 {
		nombre = string(runas[len(runas)-200:])
	}
	if nombre == "" || nombre == "." || nombre == "/" {
		return "archivo"
	}
	return nombre
}
