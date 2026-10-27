//go:build unix

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// contextoDeParada se cancela con Ctrl+C o con SIGTERM, que es como systemd detiene el servicio: así drena igual que
// con «tramtai servicio detener».
func contextoDeParada(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
}
