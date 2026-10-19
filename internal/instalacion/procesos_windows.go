//go:build windows

package instalacion

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ProcesosEn devuelve los PID de los procesos exe (por ejemplo, tramtai.exe) que se ejecutan desde carpeta, sin contar
// este: el instalador solo cierra lo de su carpeta, nunca una portable u otra copia con el mismo nombre. Los procesos
// que no se pueden abrir (de otro usuario) no cuentan.
func ProcesosEn(carpeta, exe string) ([]uint32, error) {
	foto, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(foto)
	propio := uint32(os.Getpid())
	var pids []uint32
	e := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(foto, &e); err == nil; err = windows.Process32Next(foto, &e) {
		if e.ProcessID == propio || !strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), exe) {
			continue
		}
		if ruta, ok := rutaDe(e.ProcessID); ok && mismaCarpeta(filepath.Dir(ruta), carpeta) {
			pids = append(pids, e.ProcessID)
		}
	}
	return pids, nil
}

// Cerrar termina los procesos exe de carpeta (ver ProcesosEn).
func Cerrar(carpeta, exe string) error {
	pids, err := ProcesosEn(carpeta, exe)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
		if err != nil {
			continue
		}
		_ = windows.TerminateProcess(h, 1)
		windows.CloseHandle(h)
	}
	return nil
}

func rutaDe(pid uint32) (string, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", false
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", false
	}
	return windows.UTF16ToString(buf[:n]), true
}

// mismaCarpeta compara dos carpetas como Windows: sin distinguir mayúsculas y con los nombres cortos expandidos.
func mismaCarpeta(a, b string) bool {
	if real, err := filepath.EvalSymlinks(a); err == nil {
		a = real
	}
	if real, err := filepath.EvalSymlinks(b); err == nil {
		b = real
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
