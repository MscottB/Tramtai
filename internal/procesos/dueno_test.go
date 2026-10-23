package procesos

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"testing"
)

// pcblist arma una tabla como la de net.inet.tcp.pcblist_n de macOS: cabecera, por cada conexión un xinpcb_n y un
// xsocket_n (más largos que lo que se lee y sin alinear, como en el sistema) y la cabecera de cierre.
func pcblist(conexiones ...struct {
	local, remota netip.AddrPort
	uid           uint32
}) []byte {
	le := binary.LittleEndian
	cabecera := func() []byte { b := make([]byte, 24); le.PutUint32(b, 24); return b }
	elemento := func(tipo uint32, largo int) []byte {
		b := make([]byte, (largo+7)&^7)
		le.PutUint32(b, uint32(largo))
		le.PutUint32(b[4:], tipo)
		return b
	}
	datos := cabecera()
	for _, c := range conexiones {
		pcb := elemento(xsoInpcb, 105)
		binary.BigEndian.PutUint16(pcb[pcbPuertos:], c.remota.Port())
		binary.BigEndian.PutUint16(pcb[pcbPuertos+2:], c.local.Port())
		escribir := func(union []byte, d netip.Addr) {
			if d.Is4() {
				a := d.As4()
				copy(union[12:], a[:])
			} else {
				a := d.As16()
				copy(union, a[:])
			}
		}
		pcb[pcbVflag] = 0x1
		if c.local.Addr().Is6() {
			pcb[pcbVflag] = 0x2
		}
		escribir(pcb[pcbRemota:pcbRemota+16], c.remota.Addr())
		escribir(pcb[pcbLocal:pcbLocal+16], c.local.Addr())
		so := elemento(xsoSocket, 101)
		le.PutUint32(so[socketUID:], c.uid)
		datos = append(append(append(datos, pcb...), so...), elemento(0x002, 50)...) // y un elemento de otro tipo
	}
	return append(datos, cabecera()...)
}

func TestConexionesDePcblist(t *testing.T) {
	cliente, servidor := netip.MustParseAddrPort("127.0.0.1:50082"), netip.MustParseAddrPort("127.0.0.1:17271")
	cliente6, servidor6 := netip.MustParseAddrPort("[::1]:50083"), netip.MustParseAddrPort("[::1]:17271")
	type c = struct {
		local, remota netip.AddrPort
		uid           uint32
	}
	// El lado del servidor (local = servidor) es del servicio: no debe confundirse con el del cliente.
	tabla := pcblist(c{servidor, cliente, 0}, c{cliente, servidor, 501}, c{cliente6, servidor6, 502})
	for _, caso := range []struct {
		cliente, servidor netip.AddrPort
		uid               int
	}{{cliente, servidor, 501}, {cliente6, servidor6, 502}} {
		if uid, ok := uidDePcblist(tabla, caso.cliente, caso.servidor); !ok || uid != caso.uid {
			t.Errorf("%v: uid %d %v", caso.cliente, uid, ok)
		}
	}
	if _, ok := uidDePcblist(tabla, netip.MustParseAddrPort("127.0.0.1:1"), servidor); ok {
		t.Error("una conexión ajena no coincide")
	}
	if _, ok := uidDePcblist(tabla[:30], cliente, servidor); ok {
		t.Error("una tabla cortada no debe leerse fuera de sus límites")
	}
}

func TestDireccionesDeProc(t *testing.T) {
	tabla := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 0100007F:4377 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1 1 0000000000000000 100 0 0 10 0
   1: 0100007F:C3A2 0100007F:4377 01 00000000:00000000 00:00000000 00000000  1001        0 2 1 0000000000000000 20 4 30 10 -1`
	uid, ok := uidDeConexion(tabla, netip.MustParseAddrPort("127.0.0.1:50082"), netip.MustParseAddrPort("127.0.0.1:17271"))
	if !ok || uid != 1001 {
		t.Fatalf("uid %d %v", uid, ok)
	}
	if _, ok := uidDeConexion(tabla, netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:17271")); ok {
		t.Fatal("una conexión ajena no coincide")
	}
	if d, ok := direccionProc("00000000000000000000000001000000:1F90"); !ok || d != netip.MustParseAddrPort("[::1]:8080") {
		t.Fatalf("IPv6: %v", d)
	}
}

func TestPcblistCasosLimite(t *testing.T) {
	cliente, servidor := netip.MustParseAddrPort("127.0.0.1:50082"), netip.MustParseAddrPort("127.0.0.1:17271")
	type c = struct {
		local, remota netip.AddrPort
		uid           uint32
	}
	// Un socket IPv6 que habla con IPv4 guarda las direcciones como ::ffff:127.0.0.1.
	mapeada := pcblist(c{netip.MustParseAddrPort("[::ffff:127.0.0.1]:50082"), netip.MustParseAddrPort("[::ffff:127.0.0.1]:17271"), 503})
	if uid, ok := uidDePcblist(mapeada, cliente, servidor); !ok || uid != 503 {
		t.Errorf("IPv4 mapeada: uid %d %v", uid, ok)
	}
	le := binary.LittleEndian
	normal := pcblist(c{cliente, servidor, 501})
	// Un xinpcb_n más corto de lo que se lee no se usa.
	corto := append([]byte(nil), normal...)
	le.PutUint32(corto[24:], 60)
	if _, ok := uidDePcblist(corto, cliente, servidor); ok {
		t.Error("un xinpcb_n corto no debe coincidir")
	}
	// Un xsocket_n sin su xinpcb_n delante no dice de quién es.
	sinPcb := append(append([]byte(nil), normal[:24]...), normal[24+104:]...)
	if _, ok := uidDePcblist(sinPcb, cliente, servidor); ok {
		t.Error("un xsocket_n suelto no debe coincidir")
	}
	// Una cabecera que dice ser enorme termina sin leer fuera.
	enorme := append([]byte(nil), normal...)
	le.PutUint32(enorme, 0xFFFFFFF0)
	if _, ok := uidDePcblist(enorme, cliente, servidor); ok {
		t.Error("con la cabecera rota no hay conexiones")
	}
}

// FuzzUidDePcblist: con cualquier tabla, el análisis no se sale de los límites.
func FuzzUidDePcblist(f *testing.F) {
	f.Add(pcblist(struct {
		local, remota netip.AddrPort
		uid           uint32
	}{netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:2"), 1}))
	f.Fuzz(func(t *testing.T, datos []byte) {
		uidDePcblist(datos, netip.MustParseAddrPort("127.0.0.1:1"), netip.MustParseAddrPort("127.0.0.1:2"))
	})
}

// Un xinpcb_n y un xsocket_n copiados de una conexión real en el CI de macOS (127.0.0.1:49253 aceptó a :49254; este
// es el lado del servidor, uid 501): fijan las posiciones de las estructuras con #pragma pack(4).
func TestPcblistDeMacOSReal(t *testing.T) {
	pcb, _ := hex.DecodeString("68000000100000002992cdd2abd4f18ec066c0650106e86fdc4b6db7b0010000000000000000800000000200014000000000000000000000000000007f0000010000000000000000000000007f000001000000000000000000000000000000005137139a12000000")
	so, _ := hex.DecodeString("6800000001000000ee3e416292e6c7aa010000000c000000000002012992cdd2abd4f18e06000000020000000000000000000000000000000000000000000000f50100005c07000000000000220f0000000000008000000000080001020000000000000000000000")
	cabecera := make([]byte, 24)
	binary.LittleEndian.PutUint32(cabecera, 24)
	tabla := append(append(append(cabecera, pcb...), so...), cabecera...)
	servidor, cliente := netip.MustParseAddrPort("127.0.0.1:49253"), netip.MustParseAddrPort("127.0.0.1:49254")
	if uid, ok := uidDePcblist(tabla, servidor, cliente); !ok || uid != 501 {
		t.Fatalf("uid %d %v", uid, ok)
	}
}
