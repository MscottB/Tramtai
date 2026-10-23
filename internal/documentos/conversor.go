package documentos

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"tramtai/internal/configuracion"
	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// extensionesTexto se entregan tal como llegan: solo se valida su codificación.
var extensionesTexto = map[string]bool{".txt": true, ".md": true, ".markdown": true, ".json": true, ".jsonl": true,
	".xml": true, ".csv": true, ".tsv": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".log": true}

// extensionesConvertibles son las que markitdown convierte y /api/v1/capacidades anuncia; el resto se rechaza.
var extensionesConvertibles = map[string]bool{".html": true, ".htm": true, ".pdf": true, ".docx": true, ".pptx": true, ".xlsx": true}

// ExtensionAdmitida indica si Tramtai acepta un archivo con esa extensión: de texto o convertible.
func ExtensionAdmitida(extension string) bool {
	extension = strings.ToLower(extension)
	return extensionesTexto[extension] || extensionesConvertibles[extension]
}

// Formatos son las extensiones admitidas, ordenadas: las de texto pasan tal cual y las del conversor, por markitdown.
// Es la lista que anuncian /limites y /capacidades y que usan la Web UI, la CLI y los errores.
func Formatos() (texto, conversor []string) {
	return slices.Sorted(maps.Keys(extensionesTexto)), slices.Sorted(maps.Keys(extensionesConvertibles))
}

// AdmitirArchivos falla con ErrFormato en el primer archivo que Tramtai no admite: la CLI y la terminal lo avisan
// antes de subir nada.
func AdmitirArchivos(rutas []string) error {
	for _, ruta := range rutas {
		if !ExtensionAdmitida(filepath.Ext(ruta)) {
			return ErrFormato(filepath.Base(ruta))
		}
	}
	return nil
}

// ErrFormato es el error de un archivo que Tramtai no admite: dice cuáles sí (o, con un ZIP, que se descomprima).
func ErrFormato(nombre string) error {
	if strings.EqualFold(filepath.Ext(nombre), ".zip") {
		return nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.Tf(
			"%s: los ZIP no se admiten; descomprímelo y envía los archivos sueltos (hasta %d por petición)", nombre, nucleo.MaximoCargas))
	}
	texto, conversor := Formatos()
	return nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.Tf("%s: formato no admitido. Se admiten documentos (%s) y texto (%s)",
		nombre, strings.Join(conversor, ", "), strings.Join(texto, ", ")))
}

// Entrada es un archivo que hay que preparar para el agente.
type Entrada struct {
	Ruta   string
	Nombre string
	SHA256 string
}

// Limites acotan lo que se entrega a markitdown (conversion.*).
type Limites struct {
	MaximoBytes int64 // descomprimido de un Word, PowerPoint o Excel
}

// Conversor convierte entradas a Markdown con la herramienta markitdown.
type Conversor struct {
	Comando      string // nombre o ruta de markitdown (conversion.comando)
	Raiz         string // raíz de usuario: ahí está el conversor preparado
	Plugins      bool   // conversion.habilitar_plugins
	TiempoLimite time.Duration
	MaximoSalida int64    // bytes de texto ensamblado
	MemoriaMB    int      // tope de memoria de markitdown (limites.memoria_proceso_mb): un documento hostil no agota la del equipo
	SinVariables []string // variables que markitdown no hereda además de los secretos (la del token de la instancia)
	Limites      Limites
}

// Desde crea el conversor con la configuración efectiva.
func Desde(c *configuracion.Configuracion) *Conversor {
	raiz, _ := RaizConversor()
	return &Conversor{
		Comando:      c.Texto("conversion.comando"),
		Raiz:         raiz,
		Plugins:      c.Booleano("conversion.habilitar_plugins"),
		TiempoLimite: time.Duration(c.Limite("conversion.tiempo_limite_segundos")) * time.Second,
		MaximoSalida: int64(c.Limite("conversion.maximo_salida_mb")) << 20,
		MemoriaMB:    c.Limite("limites.memoria_proceso_mb"),
		SinVariables: []string{c.Texto("instancia_global.token_variable_entorno")},
		Limites:      Limites{MaximoBytes: int64(c.Limite("conversion.maximo_descomprimido_mb")) << 20},
	}
}

// Convertir prepara las entradas y devuelve el texto ensamblado: un bloque delimitado por archivo, con sus metadatos.
func (c *Conversor) Convertir(ctx context.Context, entradas []Entrada) (nucleo.Documentos, error) {
	temporal, err := os.MkdirTemp("", "tramtai-conversion-")
	if err != nil {
		return nucleo.Documentos{}, err
	}
	defer os.RemoveAll(temporal)
	// Los delimitadores llevan un código por petición: un documento que escriba «<<<fin archivo>>>» no cierra el bloque.
	codigo := make([]byte, 4)
	_, _ = rand.Read(codigo)
	t := &tarea{conversor: c, dir: temporal, codigo: hex.EncodeToString(codigo)}
	for _, e := range entradas {
		if err := t.archivo(ctx, e); err != nil {
			return nucleo.Documentos{}, err
		}
	}
	return nucleo.Documentos{Texto: t.salida.String(), Partes: t.partes}, nil
}

type tarea struct {
	conversor *Conversor
	dir       string
	salida    bytes.Buffer
	partes    []nucleo.ParteDocumento
	codigo    string
}

func (t *tarea) archivo(ctx context.Context, e Entrada) error {
	extension := strings.ToLower(filepath.Ext(e.Nombre))
	var texto string
	var err error
	switch {
	case extensionesTexto[extension]:
		texto, err = leerTexto(e)
	case extensionesConvertibles[extension]:
		if err := t.ooxml(e, extension); err != nil {
			return err
		}
		texto, err = t.conversor.markitdown(ctx, e, t.dir)
	default:
		return ErrFormato(e.Nombre)
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(texto) == "" {
		return nucleo.NuevoError(nucleo.CodigoConversionFallida, idioma.Tf("%s no produjo texto (¿documento escaneado o sin texto extraíble?)", e.Nombre))
	}
	atributos := fmt.Sprintf("nombre=%q", e.Nombre)
	if e.SHA256 != "" {
		atributos += fmt.Sprintf(" sha256=%q", e.SHA256)
	}
	texto = strings.TrimRight(texto, "\n")
	fmt.Fprintf(&t.salida, "<<<archivo-%s %s>>>\n%s\n<<<fin archivo-%s>>>\n", t.codigo, atributos, texto, t.codigo)
	t.partes = append(t.partes, nucleo.ParteDocumento{Nombre: e.Nombre, Caracteres: utf8.RuneCountInString(texto)})
	if max := t.conversor.MaximoSalida; max > 0 && int64(t.salida.Len()) > max {
		return nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.T("el texto convertido supera conversion.maximo_salida_mb"))
	}
	return nil
}

// leerTexto conserva el contenido tal cual; solo exige UTF-8 válido y quita la marca BOM.
// ooxml comprueba, antes de entregarlo a markitdown, que un Word, PowerPoint o Excel (que son un ZIP) no se
// descomprima en más de conversion.maximo_descomprimido_mb.
// Si no se abre como ZIP, decide markitdown.
func (t *tarea) ooxml(e Entrada, extension string) error {
	if extension != ".docx" && extension != ".pptx" && extension != ".xlsx" {
		return nil
	}
	lector, err := zip.OpenReader(e.Ruta)
	if err != nil {
		return nil
	}
	defer lector.Close()
	maximo := uint64(t.conversor.Limites.MaximoBytes)
	if maximo == 0 {
		return nil
	}
	var total uint64
	for _, f := range lector.File {
		// Se compara antes de sumar: dos tamaños declarados enormes no pueden dar la vuelta y sumar poco.
		if f.UncompressedSize64 > maximo-total {
			return nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.Tf("%s: descomprimido supera conversion.maximo_descomprimido_mb", e.Nombre))
		}
		total += f.UncompressedSize64
	}
	return nil
}

func leerTexto(e Entrada) (string, error) {
	datos, err := os.ReadFile(e.Ruta)
	if err != nil {
		return "", err
	}
	datos = bytes.TrimPrefix(datos, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(datos) {
		return "", nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.Tf("%s no está en UTF-8", e.Nombre))
	}
	return string(datos), nil
}

// markitdown convierte un archivo en un proceso supervisado con plazo y límite de salida.
func (c *Conversor) markitdown(ctx context.Context, e Entrada, dir string) (string, error) {
	programa, prefijo, err := c.programa()
	if err != nil {
		return "", err
	}
	// markitdown deduce el formato por la extensión: la entrada se copia con su extensión original.
	copia, err := os.CreateTemp(dir, "entrada-*"+strings.ToLower(filepath.Ext(e.Nombre)))
	if err != nil {
		return "", err
	}
	origen, err := os.Open(e.Ruta)
	if err != nil {
		copia.Close()
		return "", err
	}
	_, err = io.Copy(copia, origen)
	origen.Close()
	if cerrar := copia.Close(); err == nil {
		err = cerrar
	}
	if err != nil {
		return "", err
	}
	args := append(prefijo, copia.Name())
	if c.Plugins {
		args = append(args, "--use-plugins")
	}
	padre := ctx
	if c.TiempoLimite > 0 {
		var cancelar context.CancelFunc
		ctx, cancelar = context.WithTimeout(ctx, c.TiempoLimite)
		defer cancelar()
	}
	var salida strings.Builder
	r, err := procesos.Ejecutar(ctx, procesos.Orden{Programa: programa, Argumentos: args, Directorio: dir, Entorno: entornoConversor(c.SinVariables...), MemoriaMB: c.MemoriaMB},
		func(linea string) error {
			if c.MaximoSalida > 0 && int64(salida.Len()) > c.MaximoSalida {
				return nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.Tf("%s: la conversión supera conversion.maximo_salida_mb", e.Nombre))
			}
			salida.WriteString(linea + "\n")
			return nil
		})
	// Solo es culpa del conversor si venció su plazo; si venció el del trabajo, ese error es el del trabajo.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) && padre.Err() == nil {
		return "", nucleo.NuevoError(nucleo.CodigoConversionFallida, idioma.Tf("%s: la conversión superó conversion.tiempo_limite_segundos", e.Nombre))
	}
	if err != nil {
		return "", err
	}
	if r.CodigoSalida != 0 {
		return "", nucleo.NuevoError(nucleo.CodigoConversionFallida, idioma.Tf("%s: markitdown terminó con código %d: %s", e.Nombre, r.CodigoSalida, ultimaLinea(r.Errores)))
	}
	return salida.String(), nil
}

// programa localiza el conversor (Localizar); sin conversor, conversor_no_disponible con cómo prepararlo.
func (c *Conversor) programa() (string, []string, error) {
	if u, ok := Localizar(c.Comando, c.Raiz); ok {
		return u.Programa, u.Argumentos, nil
	}
	return "", nil, ErrSinConversor()
}

// ErrSinConversor es el error de un archivo que necesita el conversor cuando no hay ninguno.
func ErrSinConversor() error {
	return nucleo.NuevoError(nucleo.CodigoConversorNoDisponible,
		idioma.T("no hay conversor de archivos: prepáralo en Ajustes → Archivos o con «tramtai conversor preparar»"))
}

// entornoConversor hereda el entorno sin secretos, sin las TRAMTAI_* ni las variables dadas (como la del token de la
// instancia), y fuerza UTF-8 en la salida de Python.
func entornoConversor(sin ...string) []string {
	var entorno []string
	for _, par := range os.Environ() {
		nombre, _, _ := strings.Cut(par, "=")
		if nucleo.EntornoSecreto(par) || strings.HasPrefix(strings.ToUpper(nombre), "TRAMTAI_") ||
			slices.ContainsFunc(sin, func(v string) bool { return strings.EqualFold(v, nombre) }) {
			continue
		}
		entorno = append(entorno, par)
	}
	return append(entorno, "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
}

func ultimaLinea(texto string) string {
	lineas := strings.Split(strings.TrimSpace(texto), "\n")
	ultima := strings.TrimSpace(lineas[len(lineas)-1])
	if len(ultima) > 500 {
		ultima = ultima[:500] + "…"
	}
	return ultima
}
