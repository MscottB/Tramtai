//go:build windows

package procesos

import (
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// preparar crea el proceso suspendido: se asigna al Job Object antes de que pueda lanzar hijos (validado en M0).
func preparar(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW}
}

// SinVentana evita que un programa de consola abra una ventana cuando quien lo lanza no tiene consola (el
// servicio en segundo plano o la app de escritorio).
func SinVentana(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW, HideWindow: true}
}

type grupo struct{ job windows.Handle }

// agrupar crea un Job Object con KILL_ON_JOB_CLOSE, le asigna el proceso y lo reanuda.
func agrupar(cmd *exec.Cmd) (*grupo, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	pid := uint32(cmd.Process.Pid)
	proceso, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(proceso)
	if err := windows.AssignProcessToJobObject(job, proceso); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	if err := reanudar(pid); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	return &grupo{job: job}, nil
}

// reanudar despierta los hilos del proceso suspendido; os/exec no expone el identificador de su hilo principal.
func reanudar(pid uint32) error {
	instantanea, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(instantanea)
	hilo := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(instantanea, &hilo); err == nil; err = windows.Thread32Next(instantanea, &hilo) {
		if hilo.OwnerProcessID != pid {
			continue
		}
		h, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, hilo.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(h)
		windows.CloseHandle(h)
		if err != nil {
			return err
		}
	}
	return nil
}

// terminar finaliza el árbol completo.
// shortcut: en Windows no hay parada cooperativa (CTRL_BREAK exige consola compartida); se termina el job directamente.
func (g *grupo) terminar() { _ = windows.TerminateJobObject(g.job, 1) }

// cerrar libera el job; KILL_ON_JOB_CLOSE elimina los descendientes que sigan vivos.
func (g *grupo) cerrar() { _ = windows.CloseHandle(g.job) }

// firmaInicio es la hora de creación del proceso (FILETIME); vacía si no existe o no se puede abrir.
func firmaInicio(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	var creacion, salida, nucleo, usuario windows.Filetime
	if windows.GetProcessTimes(h, &creacion, &salida, &nucleo, &usuario) != nil {
		return ""
	}
	return strconv.FormatInt(creacion.Nanoseconds(), 10)
}

// terminarArbol termina el proceso. Sus descendientes ya los cerró el sistema con el Job Object al caer el servicio.
func terminarArbol(pid int) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}
