// Command firmar firma SHA256SUMS.txt para la actualización de un clic (internal/actualizacion la verifica).
//
//	go run ./scripts/firmar SHA256SUMS.txt            firma con la semilla de FIRMA_ACTUALIZACIONES
//	go run ./scripts/firmar -comprobar SHA256SUMS.txt comprueba SHA256SUMS.txt.sig con las claves de la app
//	go run ./scripts/firmar -generar                  par nuevo: la pública por stdout y la semilla por stderr
//
// La semilla (privada, ed25519 en base64) solo se lee del entorno: así no queda en el historial de órdenes ni en el disco.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"tramtai/internal/actualizacion"
)

func main() {
	generar := flag.Bool("generar", false, "genera un par nuevo")
	comprobar := flag.Bool("comprobar", false, "comprueba la firma con las claves públicas de la app")
	flag.Parse()
	if err := ejecutar(*generar, *comprobar, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "firmar:", err)
		os.Exit(1)
	}
}

func ejecutar(generar, comprobar bool, args []string) error {
	if generar {
		publica, privada, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		fmt.Println(base64.StdEncoding.EncodeToString(publica))
		fmt.Fprintln(os.Stderr, base64.StdEncoding.EncodeToString(privada.Seed()))
		return nil
	}
	if len(args) != 1 {
		return errors.New("falta el archivo a firmar")
	}
	datos, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	if comprobar {
		firma, err := os.ReadFile(args[0] + ".sig")
		if err != nil {
			return err
		}
		if err := actualizacion.VerificarFirma(datos, firma); err != nil {
			return err
		}
		fmt.Println("firma válida")
		return nil
	}
	firma, err := firmar(datos, os.Getenv("FIRMA_ACTUALIZACIONES"))
	if err != nil {
		return err
	}
	return os.WriteFile(args[0]+".sig", []byte(firma+"\n"), 0o644)
}

// firmar devuelve la firma de datos en base64 con la semilla (en base64).
func firmar(datos []byte, semilla string) (string, error) {
	s, err := base64.StdEncoding.DecodeString(strings.TrimSpace(semilla))
	if err != nil || len(s) != ed25519.SeedSize {
		return "", errors.New("FIRMA_ACTUALIZACIONES no tiene una semilla ed25519 válida")
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.NewKeyFromSeed(s), datos)), nil
}
