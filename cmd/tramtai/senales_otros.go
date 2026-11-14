//go:build !unix

package main

import (
	"context"
	"os"
	"os/signal"
)

// contextoDeParada se cancela con Ctrl+C (en Windows no hay SIGTERM).
func contextoDeParada(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt)
}

// contextoDeOrden es el de una orden larga: se cancela con Ctrl+C.
func contextoDeOrden(ctx context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(ctx, os.Interrupt)
}
