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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
	"tramtai/internal/procesos"
)

// EspecificacionMarkitdown es la versión fijada del convertidor.
const EspecificacionMarkitdown = "markitdown[pdf,docx,pptx,xlsx]==0.1.8"

// extensionesTexto se entregan tal como llegan: solo se valida su codificación.
var extensionesTexto = map[string]bool{".txt": true, ".md": true, ".markdown": true, ".json": true, ".jsonl": true,
	".xml": true, ".csv": true, ".tsv": true, ".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".log": true}

// extensionesConvertibles son las que markitdown convierte y /api/v1/capacidades anuncia; el resto se rechaza.
var extensionesConvertibles = map[string]bool{".html": true, ".htm": true, ".pdf": true, ".docx": true, ".pptx": true, ".xlsx": true}

// Entrada es un archivo que hay que preparar para el agente.
type Entrada struct {
	Ruta   string
	Nombre string
	SHA256 string
}

// Limites acotan la extracción de ZIP (conversion.*).
type Limites struct {
	MaximoArchivos    int
	MaximoBytes       int64
	ProfundidadMaxima int
}

// Conversor convierte entradas a Markdown con la herramienta markitdown.
type Conversor struct {
	Comando      string // nombre o ruta de markitdown (conversion.comando)
	Plugins      bool   // conversion.habilitar_plugins
	TiempoLimite time.Duration
	MaximoSalida int64    // bytes de texto ensamblado
	MemoriaMB    int      // tope de memoria de markitdown (limites.memoria_proceso_mb): un documento hostil no agota la del equipo
	SinVariables []string // variables que markitdown no hereda además de los secretos (la del token de la instancia)
	Limites      Limites
}

// Desde crea el conversor con la configuración efectiva.
func Desde(c *nucleo.Configuracion) *Conversor {
	return &Conversor{
		Comando:      c.Texto("conversion.comando"),
		Plugins:      c.Booleano("conversion.habilitar_plugins"),
		TiempoLimite: time.Duration(c.Limite("conversion.tiempo_limite_segundos")) * time.Second,
		MaximoSalida: int64(c.Limite("conversion.maximo_salida_mb")) << 20,
		MemoriaMB:    c.Limite("limites.memoria_proceso_mb"),
		SinVariables: []string{c.Texto("instancia_global.token_variable_entorno")},
		Limites: Limites{
			MaximoArchivos:    c.Limite("conversion.maximo_archivos_zip"),
			MaximoBytes:       int64(c.Limite("conversion.maximo_descomprimido_mb")) << 20,
			ProfundidadMaxima: c.Limite("conversion.profundidad_maxima_zip"),
		},
	}
}

// Convertir prepara las entradas y devuelve el texto ensamblado: un bloque delimitado por archivo, con sus metadatos.
// Los ZIP se extraen con límites y cada archivo interno se convierte por separado.
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
		if err := t.archivo(ctx, e, 0); err != nil {
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
	archivos  int
	bytes     int64
}

func (t *tarea) archivo(ctx context.Context, e Entrada, profundidad int) error {
	extension := strings.ToLower(filepath.Ext(e.Nombre))
	if extension == ".zip" {
		return t.zip(ctx, e, profundidad+1)
	}
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
		return nucleo.NuevoError(nucleo.CodigoCargaInvalida, idioma.Tf("%s: formato no admitido (texto, Markdown, JSON, XML, CSV, HTML, PDF, Word, PowerPoint, Excel o ZIP)", e.Nombre))
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
// descomprima en más de conversion.maximo_descomprimido_mb: los límites de ZIP de Tramtai no alcanzan su interior.
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

// programa localiza markitdown; si no está instalado recurre a uvx con la versión fijada.
func (c *Conversor) programa() (string, []string, error) {
	comando := c.Comando
	if comando == "" {
		comando = "markitdown"
	}
	if ruta, err := exec.LookPath(comando); err == nil {
		return ruta, nil, nil
	}
	if uvx, err := exec.LookPath("uvx"); err == nil {
		return uvx, []string{"--from", EspecificacionMarkitdown, "markitdown"}, nil
	}
	return "", nil, nucleo.NuevoError(nucleo.CodigoConversionFallida,
		idioma.Tf(`markitdown no está instalado: uv tool install "%s"`, EspecificacionMarkitdown))
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
