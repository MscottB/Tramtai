# Inicio

## Requisitos

- Go 1.27 (instalado en `C:\Program Files\Go\bin`). Si `go` no se reconoce, añádelo al PATH del usuario:

  ```powershell
  [Environment]::SetEnvironmentVariable('Path', "$([Environment]::GetEnvironmentVariable('Path','User'));C:\Program Files\Go\bin;$env:USERPROFILE\go\bin", 'User')
  ```

  y abre una terminal nueva. `scripts/compilar.ps1` lo añade por su cuenta solo para esa ejecución.
- Git.
- Para M0b y los adaptadores reales: `claude`, `codex` y/o `agy` instalados y las claves de API (ver [M0B.md](M0B.md)).

## Compilar

```powershell
./scripts/compilar.ps1
```

Genera `bin/tramtai.exe` con la versión `vX.Y.Z+DDMMAAAA` incrustada. Sin el script (`go build ./cmd/tramtai`) la versión aparece como `dev`.

## Ejecutar

```powershell
./bin/tramtai.exe version
./bin/tramtai.exe configuracion validar
./bin/tramtai.exe configuracion mostrar --efectiva     # valor y capa de origen de cada clave
./bin/tramtai.exe configuracion explicar concurrencia.maximo_tamano_cola
```

O sin compilar: `go run ./cmd/tramtai configuracion mostrar --efectiva`.

La configuración se lee de (menor a mayor prioridad): valores internos, `%APPDATA%\tramtai\config.yaml`, `.tramtai/config.yaml`, `.tramtai/local.yaml` y variables `TRAMTAI_SECCION__CLAVE`. Ejemplo:

```powershell
$env:TRAMTAI_CONCURRENCIA__MAXIMO_TAMANO_COLA = 50
./bin/tramtai.exe configuracion explicar concurrencia.maximo_tamano_cola   # origen: entorno
```

## Probar

```powershell
go vet ./...
go test ./...                                   # núcleo, persistencia y planificador (agente simulado)
go test -tags m0b -v -count=1 ./pruebas/m0b     # requiere claves reales, ver M0B.md
```

## Qué se puede "ver" hoy

Aún no hay servidor HTTP ni interfaz: llegan en M2 (`tramtai servicio iniciar`, API `/api/v1`) y en el hito de la app de escritorio (Wails + Quasar). Hasta entonces el comportamiento visible es la CLI de configuración y las pruebas (`go test -v ./internal/planificador` muestra el ciclo completo de los trabajos).
