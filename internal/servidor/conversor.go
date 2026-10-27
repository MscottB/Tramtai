package servidor

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"tramtai/internal/documentos"
	"tramtai/internal/idioma"
)

// comprobarConversor es la comprobación de «Preparar archivos»; las pruebas la sustituyen (no hay Python en el .zip).
var comprobarConversor = documentos.ComprobarConversor

// instalarConversor prepara el conversor sin red desde su paquete (--conversor): descomprime su carpeta conversor en
// una temporal del administrador, la comprueba ahí y, si vale, la descomprime otra vez en <raíz>/conversor, que es
// documentos.RaizConversor() en el modo servidor. El Python se ejecuta solo en la temporal: la raíz es de la cuenta
// del servicio, que podría cambiarlo entre la comprobación y la ejecución.
func instalarConversor(r *os.Root, paquete string) error {
	z, err := zip.OpenReader(paquete)
	if err != nil {
		return err
	}
	defer z.Close()
	temporal, err := os.MkdirTemp("", "tramtai-conversor-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporal)
	t, err := os.OpenRoot(temporal)
	if err != nil {
		return err
	}
	defer t.Close()
	if err := extraerConversor(z, t, "conversor"); err != nil {
		return err
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelar()
	if err := comprobarConversor(ctx, filepath.Join(temporal, "conversor")); err != nil {
		return err
	}
	if err := r.RemoveAll("conversor.nuevo"); err != nil {
		return err
	}
	if err := extraerConversor(z, r, "conversor.nuevo"); err != nil {
		_ = r.RemoveAll("conversor.nuevo")
		return err
	}
	if err := r.RemoveAll("conversor"); err != nil {
		return err
	}
	return r.Rename("conversor.nuevo", "conversor")
}

// extraerConversor descomprime las entradas conversor/… del paquete en destino, dentro de r (el os.Root no deja salir
// de la raíz ni seguir enlaces). Compress-Archive puede dejar los nombres con \.
func extraerConversor(z *zip.ReadCloser, r *os.Root, destino string) error {
	hay := false
	for _, f := range z.File {
		rel, ok := strings.CutPrefix(strings.ReplaceAll(f.Name, `\`, "/"), "conversor/")
		if !ok || rel == "" {
			continue
		}
		if !filepath.IsLocal(rel) {
			return errors.New(idioma.Tf("el paquete del conversor tiene una entrada que sale de su carpeta: %s", f.Name))
		}
		hay = true
		ruta := path.Join(destino, rel)
		if f.FileInfo().IsDir() {
			if err := r.MkdirAll(ruta, 0o700); err != nil {
				return err
			}
			continue
		}
		if err := r.MkdirAll(path.Dir(ruta), 0o700); err != nil {
			return err
		}
		if err := extraerArchivo(f, r, ruta); err != nil {
			return err
		}
	}
	if !hay {
		return errors.New(idioma.T("el .zip no tiene la carpeta conversor: usa el paquete del conversor de Tramtai"))
	}
	return nil
}

// extraerArchivo escribe una entrada del paquete; 0700 para que el Python sea ejecutable también en Linux.
func extraerArchivo(f *zip.File, r *os.Root, ruta string) error {
	origen, err := f.Open()
	if err != nil {
		return err
	}
	defer origen.Close()
	destino, err := r.OpenFile(ruta, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(destino, origen)
	if cerrar := destino.Close(); err == nil {
		err = cerrar
	}
	return err
}
