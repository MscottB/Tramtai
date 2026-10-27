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
