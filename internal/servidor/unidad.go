package servidor

import (
	"cmp"
	"fmt"

	"tramtai/internal/configuracion"
)

// UnidadOpciones son lo que cambia en la unidad systemd.
type UnidadOpciones struct {
	PlazoDrenaje int  // servicio.plazo_drenaje_segundos; 0 es el predeterminado
	LetsEncrypt  bool // da CAP_NET_BIND_SERVICE
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
	plazo := cmp.Or(o.PlazoDrenaje, configuracion.Resolver().Entero("servicio.plazo_drenaje_segundos"))
	capacidades := "CapabilityBoundingSet=\n"
	if o.LetsEncrypt || o.PuertoBajo {
		capacidades = "CapabilityBoundingSet=CAP_NET_BIND_SERVICE\nAmbientCapabilities=CAP_NET_BIND_SERVICE\n"
	}
	return fmt.Sprintf(`# Generada por «tramtai servidor instalar»: se reescribe al instalar y al actualizar.
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
`, plazo+30, capacidades)
}
