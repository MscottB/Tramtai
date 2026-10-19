package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

// Lo que firma este programa lo verifica la misma clave pública; otra clave, unas sumas cambiadas o una semilla rota no.
func TestFirmarYComprobar(t *testing.T) {
	publica, privada, _ := ed25519.GenerateKey(nil)
	semilla := base64.StdEncoding.EncodeToString(privada.Seed())
	clave := base64.StdEncoding.EncodeToString(publica)
	sumas := []byte("abc  Tramtai-v1.0.0+1-instalador-windows-x64.exe\n")
	firma, err := firmar(sumas, semilla+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := comprobar(sumas, []byte(firma+"\n"), clave); err != nil {
		t.Fatalf("la firma propia no se verifica: %v", err)
	}
	otra, _, _ := ed25519.GenerateKey(nil)
	if comprobar(sumas, []byte(firma), base64.StdEncoding.EncodeToString(otra)) == nil {
		t.Fatal("se verificó con otra clave")
	}
	if comprobar(append(sumas, 'x'), []byte(firma), clave) == nil {
		t.Fatal("se verificaron unas sumas cambiadas")
	}
	for _, mala := range []string{"", "no-es-base64", base64.StdEncoding.EncodeToString([]byte("corta"))} {
		if _, err := firmar(sumas, mala); err == nil {
			t.Fatalf("se firmó con la semilla %q", mala)
		}
	}
}
