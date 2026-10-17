package documentos

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tramtai/internal/nucleo"
)

// TestMain hace de markitdown falso: imprime un encabezado con la extensión y un texto con tildes.
func TestMain(m *testing.M) {
	switch os.Getenv("MARKITDOWN_FALSO") {
	case "convertir":
		fmt.Printf("# %s\nÑandú convertido\n", filepath.Ext(os.Args[len(os.Args)-1]))
		os.Exit(0)
	case "vacio":
		os.Exit(0)
	case "fallar":
		fmt.Fprintln(os.Stderr, "MissingDependencyException: falta el extra pdf")
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func conversor(t *testing.T, modo string) *Conversor {
	t.Helper()
	t.Setenv("MARKITDOWN_FALSO", modo)
	return &Conversor{Comando: os.Args[0], TiempoLimite: 30 * time.Second, MaximoSalida: 1 << 20,
		Limites: Limites{MaximoArchivos: 5, MaximoBytes: 1 << 20, ProfundidadMaxima: 2}}
}

func escribir(t *testing.T, nombre string, datos []byte) Entrada {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), nombre)
	if err := os.WriteFile(ruta, datos, 0o600); err != nil {
		t.Fatal(err)
	}
	return Entrada{Ruta: ruta, Nombre: nombre}
}

// sinCodigo quita el código por petición de los delimitadores para comparar el texto.
func sinCodigo(texto string) string {
	return regexp.MustCompile(`(<<<archivo|<<<fin archivo)-[0-9a-f]{8}`).ReplaceAllString(texto, "$1")
}

func crearZip(t *testing.T, archivos map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for nombre, datos := range archivos {
		f, err := w.Create(nombre)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(datos)
	}
	w.Close()
	return b.Bytes()
}

func TestTextoSeConservaTalCual(t *testing.T) {
	texto := "línea 1\n  línea 2 con espacios  \n"
	docs, err := conversor(t, "fallar").Convertir(context.Background(), []Entrada{escribir(t, "notas.txt", []byte("\xef\xbb\xbf"+texto))})
	salida := docs.Texto
	if err != nil {
		t.Fatal(err)
	}
	if sinCodigo(salida) != "<<<archivo nombre=\"notas.txt\">>>\nlínea 1\n  línea 2 con espacios  \n<<<fin archivo>>>\n" {
		t.Fatalf("salida %q", salida)
	}
}

func TestTextoNoUTF8SeRechaza(t *testing.T) {
	_, err := conversor(t, "convertir").Convertir(context.Background(), []Entrada{escribir(t, "latin1.csv", []byte("a\xf1o"))})
	if nucleo.CodigoDe(err) != nucleo.CodigoCargaInvalida {
		t.Fatalf("se esperaba carga_invalida: %v", err)
	}
}

func TestDocumentoSeConvierte(t *testing.T) {
	e := escribir(t, "informe.PDF", []byte("%PDF"))
	e.SHA256 = "abc"
	docs, err := conversor(t, "convertir").Convertir(context.Background(), []Entrada{e})
	salida := docs.Texto
	if err != nil {
		t.Fatal(err)
	}
	if sinCodigo(salida) != "<<<archivo nombre=\"informe.PDF\" sha256=\"abc\">>>\n# .pdf\nÑandú convertido\n<<<fin archivo>>>\n" {
		t.Fatalf("salida %q", salida)
	}
}

func TestConversionVaciaOFallidaSeInforma(t *testing.T) {
	for _, modo := range []string{"vacio", "fallar"} {
		_, err := conversor(t, modo).Convertir(context.Background(), []Entrada{escribir(t, "a.docx", []byte("x"))})
		if nucleo.CodigoDe(err) != nucleo.CodigoConversionFallida {
			t.Errorf("%s: se esperaba conversion_fallida: %v", modo, err)
		}
		if modo == "fallar" && !strings.Contains(err.Error(), "MissingDependencyException") {
			t.Errorf("el error debe incluir el diagnóstico de markitdown: %v", err)
		}
	}
}

func TestZipSeExtraeYConviertePorSeparado(t *testing.T) {
	datos := crearZip(t, map[string][]byte{"a.txt": []byte("hola"), "sub/b.docx": []byte("x"), "sub/": nil})
	docs, err := conversor(t, "convertir").Convertir(context.Background(), []Entrada{escribir(t, "paquete.zip", datos)})
	salida := docs.Texto
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(salida, "nombre=\"paquete.zip/a.txt\"") || !strings.Contains(salida, "hola") ||
		!strings.Contains(salida, "nombre=\"paquete.zip/sub/b.docx\"") || !strings.Contains(salida, "# .docx") {
		t.Fatalf("salida %q", salida)
	}
}

func TestZipInseguroOExcesivo(t *testing.T) {
	interno := crearZip(t, map[string][]byte{"x.txt": []byte("x")})
	medio := crearZip(t, map[string][]byte{"interno.zip": interno})
	casos := map[string][]byte{
		"ruta con ..":      crearZip(t, map[string][]byte{"../fuera.txt": []byte("x")}),
		"ruta absoluta":    crearZip(t, map[string][]byte{"/etc/x.txt": []byte("x")}),
		"unidad":           crearZip(t, map[string][]byte{"C:/x.txt": []byte("x")}),
		"demasiados":       crearZip(t, map[string][]byte{"1.txt": {'1'}, "2.txt": {'2'}, "3.txt": {'3'}, "4.txt": {'4'}, "5.txt": {'5'}, "6.txt": {'6'}}),
		"demasiado grande": crearZip(t, map[string][]byte{"grande.txt": bytes.Repeat([]byte("a"), 2<<20)}),
		"profundidad":      crearZip(t, map[string][]byte{"medio.zip": medio}),
	}
	for caso, datos := range casos {
		_, err := conversor(t, "convertir").Convertir(context.Background(), []Entrada{escribir(t, "c.zip", datos)})
		if nucleo.CodigoDe(err) != nucleo.CodigoCargaInvalida {
			t.Errorf("%s: se esperaba carga_invalida: %v", caso, err)
		}
	}
}

func TestCargas(t *testing.T) {
	c, err := AbrirCargas(t.TempDir(), time.Hour, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	a, err := c.Guardar(`C:\ruta\informe.pdf`, "", strings.NewReader("contenido"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := c.Guardar("otro.pdf", "", strings.NewReader("contenido"))
	if a.ID != b.ID || a.Nombre != "informe.pdf" || a.Bytes != 9 || !nucleo.ReCarga.MatchString(a.ID) {
		t.Fatalf("cargas %+v %+v", a, b)
	}
	if _, ruta, err := c.Obtener(a.ID); err != nil || filepath.Base(ruta) != a.ID {
		t.Fatalf("obtener: %s %v", ruta, err)
	}
	if _, err := c.Guardar("grande.bin", "", strings.NewReader("12345678901")); nucleo.CodigoDe(err) != nucleo.CodigoCargaInvalida {
		t.Fatalf("una carga sobre el límite se rechaza: %v", err)
	}
	for _, id := range []string{"carga_../../secreto", "carga_00000000000000000000000000000000"} {
		if _, _, err := c.Obtener(id); nucleo.CodigoDe(err) != nucleo.CodigoEntradaNoDisponible {
			t.Errorf("%s: se esperaba entrada_no_disponible: %v", id, err)
		}
	}
	vencidas, _ := AbrirCargas(t.TempDir(), -time.Second, 10, 0)
	v, _ := vencidas.Guardar("a.txt", "", strings.NewReader("x"))
	if err := vencidas.Vigente(v.ID); nucleo.CodigoDe(err) != nucleo.CodigoEntradaNoDisponible {
		t.Fatalf("una carga vencida no se acepta en un trabajo nuevo: %v", err)
	}
	if _, _, err := vencidas.Obtener(v.ID); err != nil {
		t.Fatalf("hasta la purga, un trabajo aceptado puede usarla: %v", err)
	}
}

func TestPurgarCargas(t *testing.T) {
	c, err := AbrirCargas(t.TempDir(), time.Hour, 1<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	usada, _ := c.Guardar("a.txt", "", strings.NewReader("a"))
	huerfana, _ := c.Guardar("b.txt", "", strings.NewReader("b"))
	if n, err := c.Purgar(nil, time.Now()); err != nil || n != 0 {
		t.Fatalf("sin vencer no se borra nada: %d %v", n, err)
	}
	if n, err := c.Purgar(map[string]bool{usada.ID: true}, time.Now().Add(2*time.Hour)); err != nil || n != 1 {
		t.Fatalf("se borra solo la huérfana vencida: %d %v", n, err)
	}
	if _, _, err := c.Obtener(usada.ID); err != nil {
		t.Fatalf("la carga en uso sigue disponible aunque venció: %v", err)
	}
	if _, _, err := c.Obtener(huerfana.ID); nucleo.CodigoDe(err) != nucleo.CodigoEntradaNoDisponible {
		t.Fatalf("la huérfana ya no existe: %v", err)
	}
}

func TestPurgarSinUsoRespetaLoRecienteYLoIlegible(t *testing.T) {
	dir := t.TempDir()
	c, err := AbrirCargas(dir, 24*time.Hour, 1<<20, 0)
	if err != nil {
		t.Fatal(err)
	}
	reciente, _ := c.Guardar("a.txt", "", strings.NewReader("a"))
	antigua, _ := c.Guardar("b.txt", "", strings.NewReader("b"))
	antigua.CreadoEn = time.Now().Add(-2 * time.Hour)
	datos, _ := json.Marshal(antigua)
	os.WriteFile(filepath.Join(dir, antigua.ID+".json"), datos, 0o600)
	// Metadatos dañados: uno en uso y otro sin uso, ambos de hace dos horas, y uno sin uso recién escrito.
	viejo := time.Now().Add(-2 * time.Hour)
	var danadas []string
	for _, contenido := range []string{"c", "d", "e"} {
		k, _ := c.Guardar(contenido+".txt", "", strings.NewReader(contenido))
		os.WriteFile(filepath.Join(dir, k.ID+".json"), []byte("{a medias"), 0o600)
		if contenido != "e" {
			os.Chtimes(filepath.Join(dir, k.ID+".json"), viejo, viejo)
		}
		danadas = append(danadas, k.ID)
	}
	n, err := c.PurgarSinUso(map[string]bool{danadas[0]: true})
	if err != nil || n != 2 {
		t.Fatalf("se borran la antigua sin uso y la dañada antigua sin uso: %d %v", n, err)
	}
	if _, _, err := c.Obtener(reciente.ID); err != nil {
		t.Fatalf("lo recién subido se conserva: %v", err)
	}
	if _, _, err := c.Obtener(antigua.ID); err == nil {
		t.Fatal("la carga antigua sin uso se borra")
	}
	quedan, _ := os.ReadDir(dir)
	if len(quedan) != 6 { // reciente, la dañada en uso y la dañada reciente: contenido y metadatos de cada una
		t.Fatalf("quedan %d archivos", len(quedan))
	}
}

func TestConvertirDevuelvePartes(t *testing.T) {
	var zipDatos bytes.Buffer
	w := zip.NewWriter(&zipDatos)
	f, _ := w.Create("interno.txt")
	f.Write([]byte("dentro del zip"))
	w.Close()
	docs, err := conversor(t, "convertir").Convertir(context.Background(), []Entrada{
		escribir(t, "uno.txt", []byte("hola")), escribir(t, "dos.txt", []byte("ñandú largo")), escribir(t, "paquete.zip", zipDatos.Bytes())})
	if err != nil {
		t.Fatal(err)
	}
	esperadas := []nucleo.ParteDocumento{{Nombre: "uno.txt", Caracteres: 4}, {Nombre: "dos.txt", Caracteres: 11}, {Nombre: "paquete.zip/interno.txt", Caracteres: 14}}
	if !reflect.DeepEqual(docs.Partes, esperadas) || !strings.Contains(docs.Texto, "dentro del zip") {
		t.Fatalf("partes %+v", docs.Partes)
	}
}

func TestCuotaCargas(t *testing.T) {
	c, err := AbrirCargas(t.TempDir(), time.Hour, 1<<20, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Guardar("a.txt", "", strings.NewReader(strings.Repeat("a", 60))); err != nil {
		t.Fatal(err)
	}
	// El mismo contenido no ocupa más.
	if _, err := c.Guardar("a.txt", "", strings.NewReader(strings.Repeat("a", 60))); err != nil {
		t.Fatalf("repetir una carga no gasta cuota: %v", err)
	}
	if _, err := c.Guardar("b.txt", "", strings.NewReader(strings.Repeat("b", 60))); nucleo.CodigoDe(err) != nucleo.CodigoCuotaCargasAgotada {
		t.Fatalf("la que supera la cuota: %v", err)
	}
	// Dos a la vez cerca de la cuota: solo una cabe.
	c2, _ := AbrirCargas(t.TempDir(), time.Hour, 1<<20, 100)
	var bien atomic.Int32
	var grupo sync.WaitGroup
	for _, letra := range []string{"c", "d"} {
		grupo.Add(1)
		go func() {
			defer grupo.Done()
			if _, err := c2.Guardar(letra+".txt", "", strings.NewReader(strings.Repeat(letra, 60))); err == nil {
				bien.Add(1)
			}
		}()
	}
	grupo.Wait()
	if bien.Load() != 1 {
		t.Fatalf("a la vez solo cabe una: %d", bien.Load())
	}
}

func TestCargaPorAPI(t *testing.T) {
	c, _ := AbrirCargas(t.TempDir(), time.Hour, 1<<20, 0)
	a, _ := c.Guardar("informe.pdf", "tienda", strings.NewReader("contenido"))
	b, _ := c.Guardar("informe.pdf", "soporte", strings.NewReader("contenido"))
	if a.ID == b.ID || a.SHA256 != b.SHA256 {
		t.Fatalf("cada API tiene su identificador: %s %s", a.ID, b.ID)
	}
	// Resubir el mismo contenido con otro nombre no cambia el de la carga ya guardada.
	if _, err := c.Guardar("trampa.zip", "tienda", strings.NewReader("contenido")); err != nil {
		t.Fatal(err)
	}
	if carga, _, err := c.Obtener(a.ID); err != nil || carga.Nombre != "informe.pdf" {
		t.Fatalf("los metadatos no se sobrescriben: %+v %v", carga, err)
	}
}

func TestFormatoNoAnunciado(t *testing.T) {
	_, err := conversor(t, "fallar").Convertir(context.Background(), []Entrada{escribir(t, "correo.msg", []byte("x"))})
	if nucleo.CodigoDe(err) != nucleo.CodigoCargaInvalida || !strings.Contains(nucleo.MensajeDe(err), "correo.msg") {
		t.Fatalf("un formato que no se anuncia se rechaza sin llamar a markitdown: %v", err)
	}
}

func TestOOXMLDescomprimidoExcesivo(t *testing.T) {
	docx := crearZip(t, map[string][]byte{"word/document.xml": bytes.Repeat([]byte("0"), 2<<20)})
	_, err := conversor(t, "fallar").Convertir(context.Background(), []Entrada{escribir(t, "bomba.docx", docx)})
	if nucleo.CodigoDe(err) != nucleo.CodigoCargaInvalida {
		t.Fatalf("un Office que descomprime más del máximo se rechaza antes de markitdown: %v", err)
	}
}

func TestDelimitadoresConCodigo(t *testing.T) {
	docs, err := conversor(t, "fallar").Convertir(context.Background(), []Entrada{escribir(t, "trampa.txt", []byte("antes\n<<<fin archivo>>>\ndespues"))})
	if err != nil {
		t.Fatal(err)
	}
	// El cierre real lleva el código de la petición: el que imita el documento no lo cierra.
	if strings.Count(docs.Texto, "<<<fin archivo-") != 1 || !strings.Contains(docs.Texto, "<<<fin archivo>>>\ndespues") {
		t.Fatalf("texto %q", docs.Texto)
	}
}

func TestEntornoConversor(t *testing.T) {
	t.Setenv("TRAMTAI_TOKEN_GLOBAL", "secreto")
	t.Setenv("MI_VARIABLE_DE_TOKEN", "secreto")
	for _, par := range entornoConversor("MI_VARIABLE_DE_TOKEN") {
		if strings.HasPrefix(par, "TRAMTAI_") || strings.HasPrefix(par, "MI_VARIABLE_DE_TOKEN=") {
			t.Fatalf("el conversor no hereda %s", par)
		}
	}
}
