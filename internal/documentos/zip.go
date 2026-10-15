package documentos

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"tramtai/internal/idioma"
	"tramtai/internal/nucleo"
)

// zip extrae el archivo con límites de cantidad, tamaño descomprimido (medido, no declarado) y profundidad,
// y rechaza rutas absolutas, con "..", con unidad o enlaces simbólicos (sección 10).
func (t *tarea) zip(ctx context.Context, e Entrada, profundidad int) error {
	limites := t.conversor.Limites
	invalida := func(motivo string) error {
		return nucleo.NuevoError(nucleo.CodigoCargaInvalida, e.Nombre+": "+motivo)
	}
	if limites.ProfundidadMaxima > 0 && profundidad > limites.ProfundidadMaxima {
		return invalida(idioma.T("supera conversion.profundidad_maxima_zip"))
	}
	lector, err := zip.OpenReader(e.Ruta)
	if err != nil {
		return invalida(idioma.T("no es un ZIP válido"))
	}
	defer lector.Close()
	for _, f := range lector.File {
		nombre := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if f.FileInfo().IsDir() || strings.HasPrefix(nombre, "__MACOSX/") {
			continue
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			return invalida(idioma.Tf("contiene un enlace simbólico: %s", f.Name))
		}
		if path.IsAbs(nombre) || nombre == ".." || strings.HasPrefix(nombre, "../") || strings.Contains(nombre, ":") {
			return invalida(idioma.Tf("contiene una ruta insegura: %s", f.Name))
		}
		t.archivos++
		if limites.MaximoArchivos > 0 && t.archivos > limites.MaximoArchivos {
			return invalida(idioma.T("supera conversion.maximo_archivos_zip"))
		}
		destino := filepath.Join(t.dir, fmt.Sprintf("zip-%04d%s", t.archivos, strings.ToLower(path.Ext(nombre))))
		suma, err := t.extraer(f, destino, invalida)
		if err != nil {
			return err
		}
		interna := Entrada{Ruta: destino, Nombre: e.Nombre + "/" + nombre, SHA256: suma}
		if err := t.archivo(ctx, interna, profundidad); err != nil {
			return err
		}
	}
	return nil
}

// extraer copia una entrada contando los bytes reales contra el límite global descomprimido.
func (t *tarea) extraer(f *zip.File, destino string, invalida func(string) error) (string, error) {
	origen, err := f.Open()
	if err != nil {
		return "", invalida(idioma.Tf("entrada ilegible: %s", f.Name))
	}
	defer origen.Close()
	salida, err := os.OpenFile(destino, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer salida.Close()
	restante := t.conversor.Limites.MaximoBytes - t.bytes
	if t.conversor.Limites.MaximoBytes <= 0 {
		restante = 1 << 62
	}
	resumen := sha256.New()
	n, err := io.Copy(io.MultiWriter(salida, resumen), io.LimitReader(origen, restante+1))
	if err != nil {
		return "", invalida(idioma.Tf("entrada corrupta: %s", f.Name))
	}
	t.bytes += n
	if n > restante {
		return "", invalida(idioma.T("supera conversion.maximo_descomprimido_mb"))
	}
	return hex.EncodeToString(resumen.Sum(nil)), nil
}
