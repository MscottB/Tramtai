//go:build windows

package procesos

import (
	"net/netip"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi            = windows.NewLazySystemDLL("iphlpapi.dll")
	getExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
)

const tcpTableOwnerPidAll = 5 // TCP_TABLE_OWNER_PID_ALL

// filaTCP4 es MIB_TCPROW_OWNER_PID; filaTCP6, MIB_TCP6ROW_OWNER_PID. Puertos en orden de red en los 16 bits bajos.
type filaTCP4 struct{ Estado, DirLocal, PuertoLocal, DirRemota, PuertoRemoto, PID uint32 }
type filaTCP6 struct {
	DirLocal     [16]byte
	AmbitoLocal  uint32
	PuertoLocal  uint32
	DirRemota    [16]byte
	AmbitoRemoto uint32
	PuertoRemoto uint32
	Estado       uint32
	PID          uint32
}

func puertoRed(p uint32) uint16 { return uint16(p&0xff)<<8 | uint16(p>>8&0xff) }

// MismoUsuario indica si el extremo local de la conexión TCP local→remoto es de un proceso del mismo usuario del
// sistema que este: busca ese proceso y compara su usuario (SID).
func MismoUsuario(local, remoto netip.AddrPort) bool {
	pid, ok := pidDeConexion(local, remoto)
	if !ok {
		return false
	}
	suyo, err := usuarioDe(pid)
	if err != nil {
		return false
	}
	propio, err := windows.GetCurrentProcessToken().GetTokenUser()
	return err == nil && windows.EqualSid(suyo, propio.User.Sid)
}

func pidDeConexion(cliente, servidor netip.AddrPort) (uint32, bool) {
	familia := uint32(windows.AF_INET)
	if cliente.Addr().Is6() {
		familia = windows.AF_INET6
	}
	var tamano uint32
	var datos []byte
	for intento := 0; ; intento++ {
		var puntero uintptr
		if len(datos) > 0 {
			puntero = uintptr(unsafe.Pointer(&datos[0]))
		}
		r, _, _ := getExtendedTcpTable.Call(puntero, uintptr(unsafe.Pointer(&tamano)), 0, uintptr(familia), tcpTableOwnerPidAll, 0)
		if r == 0 && len(datos) > 0 {
			break
		}
		if intento == 3 {
			return 0, false
		}
		datos = make([]byte, tamano+1024) // la tabla puede crecer entre llamadas
		tamano = uint32(len(datos))
	}
	n := *(*uint32)(unsafe.Pointer(&datos[0]))
	if familia == windows.AF_INET {
		for _, f := range unsafe.Slice((*filaTCP4)(unsafe.Pointer(&datos[4])), n) {
			local := netip.AddrFrom4([4]byte{byte(f.DirLocal), byte(f.DirLocal >> 8), byte(f.DirLocal >> 16), byte(f.DirLocal >> 24)})
			if local == cliente.Addr() && puertoRed(f.PuertoLocal) == cliente.Port() && puertoRed(f.PuertoRemoto) == servidor.Port() {
				return f.PID, true
			}
		}
		return 0, false
	}
	for _, f := range unsafe.Slice((*filaTCP6)(unsafe.Pointer(&datos[4])), n) {
		if netip.AddrFrom16(f.DirLocal) == cliente.Addr() && puertoRed(f.PuertoLocal) == cliente.Port() && puertoRed(f.PuertoRemoto) == servidor.Port() {
			return f.PID, true
		}
	}
	return 0, false
}

func usuarioDe(pid uint32) (*windows.SID, error) {
	proceso, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(proceso)
	var token windows.Token
	if err := windows.OpenProcessToken(proceso, windows.TOKEN_QUERY, &token); err != nil {
		return nil, err
	}
	defer token.Close()
	usuario, err := token.GetTokenUser()
	if err != nil {
		return nil, err
	}
	return usuario.User.Sid.Copy()
}
