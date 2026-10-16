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
// estándar, nunca en la línea de órdenes, que cualquiera ve con ps). Devuelve la salida y el código de salida; un
// error nunca incluye el secreto.
func ejecutar(entrada, secreto, programa string, args ...string) (string, int, error) {
	ctx, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	cmd := exec.CommandContext(ctx, programa, args...)
	cmd.Stdin = strings.NewReader(entrada)
	var salida, errores bytes.Buffer
	cmd.Stdout, cmd.Stderr = &salida, &errores
	err := cmd.Run()
	codigo := 0
	var finalizado *exec.ExitError
	if errors.As(err, &finalizado) {
		codigo = finalizado.ExitCode()
	}
	if err != nil {
		mensaje := strings.TrimSpace(errores.String())
		if secreto != "" {
			mensaje = strings.ReplaceAll(mensaje, secreto, "[REDACTADO]")
		}
		if mensaje == "" {
			mensaje = err.Error()
		}
		err = fmt.Errorf("%s: %s", filepath.Base(programa), mensaje)
	}
	return salida.String(), codigo, err
}
