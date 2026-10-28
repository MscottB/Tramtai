package servidor

import (
	"cmp"
	"fmt"

	"tramtai/internal/configuracion"
)

// UnidadOpciones son lo que cambia en la unidad systemd.
type UnidadOpciones struct {
	PlazoDrenaje int  // servicio.plazo_drenaje_segundos; 0 es el predeterminado
	LetsEncrypt  bool // da CAP_NET_BIND_SERVICE: letsencrypt exige el puerto 443, así que equivale a PuertoBajo
	PuertoBajo   bool // da CAP_NET_BIND_SERVICE
}

// Unidad es el texto de /etc/systemd/system/tramtai.service (spec §2). Es la misma para el paquete con
// UnidadOpciones{}.
//
// KillMode=mixed manda SIGTERM solo al servicio, que deja terminar las peticiones en curso, y no a los CLI de los
// agentes, que lo cortarían; lo que quede vivo recibe SIGKILL al terminar. La parada espera el drenaje más 30 s. Sin
// MemoryDenyWriteExecute, RestrictNamespaces, SystemCallFilter, ProcSubset ni PrivateUsers: rompen o arriesgan los CLI
// de los agentes (el JIT de Node de Claude Code y Copilot, sus procesos hijos).
func Unidad(o UnidadOpciones) string {
	return fmt.Sprintf(`%s
[Unit]
Description=Tramtai (servidor)
Documentation=https://github.com/MscottB/Tramtai/blob/main/docs/servidor.md
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=tramtai
Group=tramtai
Environment=TRAMTAI_SERVIDOR=1
StateDirectory=tramtai
StateDirectoryMode=0700
RuntimeDirectory=tramtai
RuntimeDirectoryMode=0755
ExecStart=/opt/tramtai/tramtai servicio iniciar --registro /var/lib/tramtai/datos/servicio.log
Restart=on-failure
RestartSec=5s
KillMode=mixed
TimeoutStopSec=%d
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes
ReadWritePaths=/var/lib/tramtai
RestrictSUIDSGID=yes
LockPersonality=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
ProtectHostname=yes
ProtectProc=invisible
RestrictRealtime=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
SystemCallArchitectures=native
RemoveIPC=yes
UMask=0077
%s
[Install]
WantedBy=multi-user.target
`, cabeceraUnidad, parada(o), capacidades(o))
}

// cabeceraUnidad encabeza la unidad: vale también para la del paquete, que reescriben apt o dnf. Instalar con el
// paquete la reconoce (con la de antes, cabeceraUnidadAntigua) para quitar la unidad que dejó el .tar.gz en /etc.
const (
	cabeceraUnidad        = "# Unidad de Tramtai (servidor), generada por Tramtai: no la edites; para cambiarla, usa un drop-in (systemctl edit tramtai)."
	cabeceraUnidadAntigua = "# Generada por «tramtai servidor instalar»"
)

// ajustesPaquete es el drop-in que pone la unidad del paquete .deb/.rpm (que no se reescribe: es de apt o dnf) al día
// con la configuración, como Unidad(o). CapabilityBoundingSet= vacío deja el conjunto vacío, como la unidad; con
// CAP_NET_BIND_SERVICE, systemd lo suma a ese conjunto vacío.
func ajustesPaquete(o UnidadOpciones) string {
	return fmt.Sprintf("# Generado por «tramtai-servidor instalar» y «reiniciar»: no lo edites a mano.\n[Service]\nTimeoutStopSec=%d\n%s",
		parada(o), capacidades(o))
}

// parada es TimeoutStopSec: el plazo de drenaje más 30 s.
func parada(o UnidadOpciones) int {
	return cmp.Or(o.PlazoDrenaje, configuracion.Resolver().Entero("servicio.plazo_drenaje_segundos")) + 30
}

func capacidades(o UnidadOpciones) string {
	if o.LetsEncrypt || o.PuertoBajo {
		return "CapabilityBoundingSet=CAP_NET_BIND_SERVICE\nAmbientCapabilities=CAP_NET_BIND_SERVICE\n"
	}
	return "CapabilityBoundingSet=\n"
}
