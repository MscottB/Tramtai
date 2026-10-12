# Versionado

Formato: **`vX.Y.Z+DDMMAAAA`**, por ejemplo `v0.1.0+12102026`.

- `X.Y.Z` es SemVer y se edita a mano en el archivo [`VERSION`](../VERSION):
  - `Z` (parche): correcciones.
  - `Y` (menor): funcionalidad nueva compatible, por ejemplo cerrar un hito.
  - `X` (mayor): cambios incompatibles de API o configuración.
- `+DDMMAAAA` es la fecha de la compilación o de la etiqueta. En SemVer es metadato de compilación: no cambia la precedencia, solo informa del día.

## Comandos

```powershell
./scripts/version.ps1              # muestra la versión de hoy
./scripts/compilar.ps1             # compila con esa versión incrustada
./scripts/version.ps1 -Etiquetar   # crea la etiqueta git anotada (exige árbol limpio)
git push origin v0.1.0+12102026    # sube la etiqueta
```

## Reglas

- Una etiqueta por número `X.Y.Z`: el script se niega a crear `v0.1.0+13102026` si ya existe `v0.1.0+12102026`. Para publicar otra, sube el número en `VERSION`.
- Flujo: editar `VERSION` → commit → `version.ps1 -Etiquetar` → push del commit y de la etiqueta.
- Los releases de GitHub se crearán a partir de estas etiquetas cuando exista el CI/CD (ver [pendientes.md](pendientes.md)).
- Nota: Go no admite `+metadatos` como versión de módulo. No afecta, porque Tramtai es una aplicación y no una biblioteca importable.
