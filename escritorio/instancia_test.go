package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestUniqueID(t *testing.T) {
	if uniqueID(`C:\Programas\Tramtai`, false) != "cl.tramtai.escritorio" {
		t.Fatal("la instalada usa siempre el mismo")
	}
	a, b := uniqueID(`C:\Portable\Tramtai`, true), uniqueID(`D:\Otra\Tramtai`, true)
	if a == b || a == "cl.tramtai.escritorio" {
		t.Fatalf("cada portable el suyo: %s %s", a, b)
	}
	// Rutas de Windows: solo allí \ separa carpetas y no se distinguen las mayúsculas.
	if runtime.GOOS == "windows" && uniqueID(`c:\portable\tramtai\`, true) != a {
		t.Fatal("la misma carpeta escrita de otra forma da otro identificador")
	}
	// Válido para D-Bus: ninguna parte vacía ni empezando por un dígito, también con cualquier hash.
	for _, carpeta := range []string{"/a", "/b", "/c", "/d", "/e", "/f", "/g", "/h"} {
		for _, parte := range strings.Split(uniqueID(carpeta, true), ".") {
			if parte == "" || parte[0] >= '0' && parte[0] <= '9' {
				t.Fatalf("%q no vale para D-Bus", uniqueID(carpeta, true))
			}
		}
	}
}
