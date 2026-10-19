package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

// Lo que firma este programa lo verifica su clave pública, y no otra ni unas sumas cambiadas; una semilla rota no firma.
func TestFirmar(t *testing.T) {
	publica, privada, _ := ed25519.GenerateKey(nil)
	sumas := []byte("abc  Tramtai-v1.0.0+1-instalador-windows-x64.exe\n")
	firma, err := firmar(sumas, base64.StdEncoding.EncodeToString(privada.Seed())+"\n")
	if err != nil {
		t.Fatal(err)
	}
	sig, _ := base64.StdEncoding.DecodeString(firma)
	if !ed25519.Verify(publica, sumas, sig) {
		t.Fatal("la firma no se verifica con su clave pública")
	}
	otra, _, _ := ed25519.GenerateKey(nil)
	if ed25519.Verify(otra, sumas, sig) || ed25519.Verify(publica, append(sumas, 'x'), sig) {
		t.Fatal("la firma vale con otra clave o con otras sumas")
	}
	for _, mala := range []string{"", "no-es-base64", base64.StdEncoding.EncodeToString([]byte("corta"))} {
		if _, err := firmar(sumas, mala); err == nil {
			t.Fatalf("se firmó con la semilla %q", mala)
		}
	}
}
