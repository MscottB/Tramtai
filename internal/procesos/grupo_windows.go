//go:build windows

package procesos

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// preparar crea el proceso suspendido: se asigna al Job Object antes de que pueda lanzar hijos (validado en M0).
func preparar(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW}
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
