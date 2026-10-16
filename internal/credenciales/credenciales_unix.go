//go:build darwin || linux

package credenciales

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ejecutar lanza la herramienta del almacén del sistema con la entrada indicada (la clave viaja por la entrada
// estándar, nunca en la línea de órdenes, que cualquiera ve con ps). Devuelve la salida, lo que escribió por stderr
// y el código de salida; ni ese texto ni el error incluyen nunca el secreto.
func ejecutar(entrada, secreto, programa string, args ...string) (salida, errores string, codigo int, err error) {
	ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, programa, args...)
	cmd.Stdin = strings.NewReader(entrada)
	var bufSalida, bufErrores bytes.Buffer
	cmd.Stdout, cmd.Stderr = &bufSalida, &bufErrores
	err = cmd.Run()
	var finalizado *exec.ExitError
	if errors.As(err, &finalizado) {
		codigo = finalizado.ExitCode()
	}
	errores = strings.TrimSpace(bufErrores.String())
	if secreto != "" {
		errores = strings.ReplaceAll(errores, secreto, "[REDACTADO]")
	}
	if err != nil {
		mensaje := errores
		if mensaje == "" {
			mensaje = err.Error()
		}
		err = fmt.Errorf("%s: %s", filepath.Base(programa), mensaje)
	}
	return bufSalida.String(), errores, codigo, err
}
