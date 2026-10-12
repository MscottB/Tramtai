//go:build windows

package procesos

import "golang.org/x/sys/windows"

func vivo(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	evento, _ := windows.WaitForSingleObject(h, 0)
	return evento == uint32(windows.WAIT_TIMEOUT)
}
