; Instalador de Tramtai para Windows: solo para el usuario actual, sin permisos de administrador.
; Lo genera scripts/compilar.ps1 -Instalador, que pasa /DVERSION y /DSALIDA y antes compila bin\.
; Las imágenes salen de imagenes.ps1. El PATH lo edita tramtai.exe (NSIS corta las cadenas a 1024 caracteres).
Unicode true
ManifestDPIAware true
RequestExecutionLevel user
SetCompressor /SOLID lzma

!ifndef VERSION
  !define VERSION "v0.0.0"
!endif
!ifndef SALIDA
  !define SALIDA "..\..\dist\Tramtai-instalador.exe"
!endif
!define DESINSTALAR "Software\Microsoft\Windows\CurrentVersion\Uninstall\Tramtai"
!define RAIZ "..\.."

Name "Tramtai"
OutFile "${SALIDA}"
InstallDir "$LOCALAPPDATA\Programs\Tramtai"
InstallDirRegKey HKCU "${DESINSTALAR}" "InstallLocation"
BrandingText "Tramtai ${VERSION}"

!include MUI2.nsh
!include LogicLib.nsh
!include FileFunc.nsh

!define MUI_ICON "${RAIZ}\escritorio\icono\tramtai.ico"
!define MUI_UNICON "${RAIZ}\escritorio\icono\tramtai.ico"
!define MUI_WELCOMEFINISHPAGE_BITMAP "lateral.bmp"
!define MUI_UNWELCOMEFINISHPAGE_BITMAP "lateral.bmp"
!define MUI_WELCOMEFINISHPAGE_BITMAP_STRETCH AspectFitHeight
!define MUI_UNWELCOMEFINISHPAGE_BITMAP_STRETCH AspectFitHeight
!define MUI_HEADERIMAGE
!define MUI_HEADERIMAGE_RIGHT
!define MUI_HEADERIMAGE_BITMAP "cabecera.bmp"
!define MUI_HEADERIMAGE_UNBITMAP "cabecera.bmp"
!define MUI_HEADERIMAGE_BITMAP_STRETCH AspectFitHeight
!define MUI_HEADERIMAGE_UNBITMAP_STRETCH AspectFitHeight
!define MUI_ABORTWARNING
!define MUI_COMPONENTSPAGE_SMALLDESC

!define MUI_WELCOMEPAGE_TITLE "$(bienvenidaTitulo)"
!define MUI_WELCOMEPAGE_TEXT "$(bienvenidaTexto)"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "${RAIZ}\LICENSE.md"
!define MUI_PAGE_HEADER_TEXT "$(terminosTitulo)"
!define MUI_PAGE_HEADER_SUBTEXT "$(terminosSubtitulo)"
!insertmacro MUI_PAGE_LICENSE "${RAIZ}\TERMINOS.md"
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_TITLE "$(finalTitulo)"
!define MUI_FINISHPAGE_TEXT "$(finalTexto)"
!define MUI_FINISHPAGE_RUN "$INSTDIR\tramtai-escritorio.exe"
!define MUI_FINISHPAGE_RUN_TEXT "$(abrirAhora)"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; El primero es el predeterminado; el asistente elige solo el idioma de Windows si está en la lista.
!insertmacro MUI_LANGUAGE "Spanish"
!insertmacro MUI_LANGUAGE "English"

LangString bienvenidaTitulo ${LANG_SPANISH} "Te damos la bienvenida a Tramtai"
LangString bienvenidaTitulo ${LANG_ENGLISH} "Welcome to Tramtai"
LangString bienvenidaTexto ${LANG_SPANISH} "Tramtai convierte Claude Code, Codex, Gemini y Antigravity en una API local para tus aplicaciones.$\r$\n$\r$\nSe instala solo para tu usuario y no pide permisos de administrador. Si ya lo tienes instalado, se actualiza y conserva tu configuración, tus APIs y tu historial."
LangString bienvenidaTexto ${LANG_ENGLISH} "Tramtai turns Claude Code, Codex, Gemini and Antigravity into a local API for your applications.$\r$\n$\r$\nIt installs for your user only and needs no administrator rights. If it is already installed, it is updated and keeps your settings, your APIs and your history."
LangString finalTitulo ${LANG_SPANISH} "Tramtai está listo"
LangString finalTitulo ${LANG_ENGLISH} "Tramtai is ready"
LangString finalTexto ${LANG_SPANISH} "Al abrirlo queda en la bandeja del sistema con el servicio en marcha. En una terminal nueva puedes usar el comando tramtai."
LangString finalTexto ${LANG_ENGLISH} "Once opened it stays in the system tray with the service running. In a new terminal you can use the tramtai command."
LangString abrirAhora ${LANG_SPANISH} "Abrir Tramtai"
LangString abrirAhora ${LANG_ENGLISH} "Open Tramtai"
LangString secPrograma ${LANG_SPANISH} "Tramtai (necesario)"
LangString secPrograma ${LANG_ENGLISH} "Tramtai (required)"
LangString secRuta ${LANG_SPANISH} "Comando tramtai en la terminal"
LangString secRuta ${LANG_ENGLISH} "tramtai command in the terminal"
LangString secArranque ${LANG_SPANISH} "Iniciar con Windows"
LangString secArranque ${LANG_ENGLISH} "Start with Windows"
LangString secEscritorio ${LANG_SPANISH} "Acceso directo en el escritorio"
LangString secEscritorio ${LANG_ENGLISH} "Desktop shortcut"
LangString descPrograma ${LANG_SPANISH} "El servicio, la aplicación de escritorio con su icono en la bandeja y el acceso en el menú Inicio."
LangString descPrograma ${LANG_ENGLISH} "The service, the desktop app with its tray icon and the Start menu entry."
LangString descRuta ${LANG_SPANISH} "Añade la carpeta de Tramtai al PATH de tu usuario."
LangString descRuta ${LANG_ENGLISH} "Adds the Tramtai folder to your user PATH."
LangString descArranque ${LANG_SPANISH} "Al iniciar sesión, Tramtai arranca oculto en la bandeja con el servicio en marcha."
LangString descArranque ${LANG_ENGLISH} "When you sign in, Tramtai starts hidden in the tray with the service running."
LangString descEscritorio ${LANG_SPANISH} "Un acceso directo a Tramtai en el escritorio."
LangString descEscritorio ${LANG_ENGLISH} "A Tramtai shortcut on the desktop."
LangString terminosTitulo ${LANG_SPANISH} "Términos de uso"
LangString terminosTitulo ${LANG_ENGLISH} "Terms of use"
LangString terminosSubtitulo ${LANG_SPANISH} "Léelos antes de instalar: explican tu responsabilidad con los servicios de terceros."
LangString terminosSubtitulo ${LANG_ENGLISH} "Read them before installing: they explain your responsibility with third-party services."
LangString cerrarAbierta ${LANG_SPANISH} "Tramtai está abierto. Para continuar se cerrará y se detendrá su servicio; las peticiones en curso terminan antes.$\r$\n$\r$\nAl terminar de instalar, Tramtai volverá a abrirse."
LangString cerrarAbierta ${LANG_ENGLISH} "Tramtai is open. To continue it will be closed and its service stopped; requests in progress finish first.$\r$\n$\r$\nWhen the installation finishes, Tramtai will open again."
LangString cerrarAbiertaDesinstalar ${LANG_SPANISH} "Tramtai está abierto. Para desinstalarlo se cerrará y se detendrá su servicio; las peticiones en curso terminan antes."
LangString cerrarAbiertaDesinstalar ${LANG_ENGLISH} "Tramtai is open. To uninstall it, it will be closed and its service stopped; requests in progress finish first."
LangString borrarDatos ${LANG_SPANISH} "¿Borrar también tu configuración y tus datos de Tramtai (APIs, tokens e historial)?$\r$\n$\r$\nSi vas a volver a instalarlo, elige No."
LangString borrarDatos ${LANG_ENGLISH} "Also delete your Tramtai settings and data (APIs, tokens and history)?$\r$\n$\r$\nIf you plan to reinstall it, choose No."

Var abierta  ; la app de escritorio estaba abierta antes de instalar
Var servicio ; el servicio estaba en marcha

; HayProceso deja 0 en RESULTADO si el usuario tiene abierto un proceso con ese nombre (esté donde esté instalado).
!macro HayProceso NOMBRE RESULTADO
  nsExec::Exec 'cmd /c tasklist /FI "USERNAME eq %USERNAME%" /FI "IMAGENAME eq ${NOMBRE}" /NH | find /I "${NOMBRE}" >NUL'
  Pop ${RESULTADO}
!macroend

; Antes de copiar o borrar: cierra la aplicación y detiene el servicio de Tramtai, sea esta instalación, la portable o
; una copia compilada, porque los .exe en uso no se pueden reemplazar y la app nueva no debe quedarse conectada al
; servicio viejo. El servicio espera a que terminen los trabajos en curso (servicio.plazo_drenaje_segundos). CLI es el
; tramtai.exe que lo detiene: encuentra el servicio por su servicio.json, lo haya iniciado la copia que sea. Recuerda en
; $abierta y $servicio qué estaba en marcha para volver a abrirlo al terminar.
!macro DetenerTramtai CLI AVISO
  StrCpy $abierta 0
  StrCpy $servicio 0
  !insertmacro HayProceso "tramtai-escritorio.exe" $0
  !insertmacro HayProceso "tramtai.exe" $1
  ${If} $0 == 0
    StrCpy $abierta 1
  ${EndIf}
  ${If} $0 == 0
  ${OrIf} $1 == 0
    ${IfNot} ${Silent}
      MessageBox MB_OKCANCEL|MB_ICONINFORMATION "${AVISO}" /SD IDOK IDOK +2
      Abort
    ${EndIf}
    nsExec::Exec 'cmd /c taskkill /F /FI "USERNAME eq %USERNAME%" /IM tramtai-escritorio.exe'
    Pop $0
    nsExec::Exec '"${CLI}" servicio detener --si'
    Pop $0
    ${If} $0 == 0
      StrCpy $servicio 1
    ${EndIf}
    ; Espera hasta 10 s a que salgan los demás tramtai.exe (la CLI, una terminal interactiva…) y cierra los que queden.
    ${For} $2 1 20
      !insertmacro HayProceso "tramtai.exe" $1
      ${IfThen} $1 != 0 ${|} ${Break} ${|}
      Sleep 500
    ${Next}
    nsExec::Exec 'cmd /c taskkill /F /FI "USERNAME eq %USERNAME%" /IM tramtai.exe'
    Pop $0
  ${EndIf}
!macroend

Section "$(secPrograma)" SecPrograma
  SectionIn RO
  ; La CLI nueva detiene el servicio aunque esta carpeta aún no tenga Tramtai (primera instalación o portable).
  InitPluginsDir
  File "/oname=$PLUGINSDIR\tramtai.exe" "${RAIZ}\bin\tramtai.exe"
  !insertmacro DetenerTramtai "$PLUGINSDIR\tramtai.exe" "$(cerrarAbierta)"
  SetOutPath "$INSTDIR"
  File "${RAIZ}\bin\tramtai.exe"
  File "${RAIZ}\bin\tramtai-escritorio.exe"
  File "/oname=LICENCIA.md" "${RAIZ}\LICENSE.md"
  File "${RAIZ}\TERMINOS.md"
  WriteUninstaller "$INSTDIR\desinstalar.exe"
  CreateShortcut "$SMPROGRAMS\Tramtai.lnk" "$INSTDIR\tramtai-escritorio.exe"
  WriteRegStr HKCU "${DESINSTALAR}" "DisplayName" "Tramtai"
  WriteRegStr HKCU "${DESINSTALAR}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${DESINSTALAR}" "Publisher" "Michael Scott B."
  WriteRegStr HKCU "${DESINSTALAR}" "DisplayIcon" "$INSTDIR\tramtai-escritorio.exe"
  WriteRegStr HKCU "${DESINSTALAR}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${DESINSTALAR}" "URLInfoAbout" "https://github.com/MscottB/Tramtai"
  WriteRegStr HKCU "${DESINSTALAR}" "UninstallString" '"$INSTDIR\desinstalar.exe"'
  WriteRegStr HKCU "${DESINSTALAR}" "QuietUninstallString" '"$INSTDIR\desinstalar.exe" /S'
  WriteRegDWORD HKCU "${DESINSTALAR}" "NoModify" 1
  WriteRegDWORD HKCU "${DESINSTALAR}" "NoRepair" 1
SectionEnd

Section "$(secRuta)" SecRuta
  nsExec::Exec '"$INSTDIR\tramtai.exe" instalacion agregar-ruta "$INSTDIR"'
  Pop $0
SectionEnd

Section /o "$(secArranque)" SecArranque
  nsExec::Exec '"$INSTDIR\tramtai.exe" servicio autoarranque activar'
  Pop $0
SectionEnd

Section /o "$(secEscritorio)" SecEscritorio
  CreateShortcut "$DESKTOP\Tramtai.lnk" "$INSTDIR\tramtai-escritorio.exe"
SectionEnd

; Al terminar vuelve a abrir lo que estaba abierto. En silencio, la app si lo estaba o si se pide con /abrir (tramtai
; actualizar y «Actualizar ahora»); con el asistente, la casilla «Abrir Tramtai» de la última página decide. Si solo
; estaba el servicio (sin la app), se vuelve a iniciar en segundo plano.
Function .onInstSuccess
  ${GetParameters} $0
  ClearErrors
  ${GetOptions} $0 "/abrir" $1
  ${IfNot} ${Errors}
    StrCpy $abierta 1
  ${EndIf}
  ${If} ${Silent}
  ${AndIf} $abierta == 1
    Exec '"$INSTDIR\tramtai-escritorio.exe" -actualizado'
  ${ElseIf} $servicio == 1
  ${AndIf} $abierta != 1
    Exec '"$INSTDIR\tramtai.exe" servicio iniciar -d'
  ${EndIf}
FunctionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecPrograma} "$(descPrograma)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecRuta} "$(descRuta)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecArranque} "$(descArranque)"
  !insertmacro MUI_DESCRIPTION_TEXT ${SecEscritorio} "$(descEscritorio)"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "Uninstall"
  !insertmacro DetenerTramtai "$INSTDIR\tramtai.exe" "$(cerrarAbiertaDesinstalar)"
  nsExec::Exec '"$INSTDIR\tramtai.exe" servicio autoarranque desactivar'
  Pop $0
  nsExec::Exec '"$INSTDIR\tramtai.exe" instalacion quitar-ruta "$INSTDIR"'
  Pop $0
  Delete "$INSTDIR\tramtai.exe"
  Delete "$INSTDIR\tramtai-escritorio.exe"
  Delete "$INSTDIR\LICENCIA.md"
  Delete "$INSTDIR\TERMINOS.md"
  Delete "$INSTDIR\desinstalar.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\Tramtai.lnk"
  Delete "$DESKTOP\Tramtai.lnk"
  DeleteRegKey HKCU "${DESINSTALAR}"
  ; Los datos se conservan salvo que el usuario diga lo contrario; en modo silencioso, siempre.
  ${IfNot} ${Silent}
    MessageBox MB_YESNO|MB_ICONQUESTION|MB_DEFBUTTON2 "$(borrarDatos)" IDNO conservar
    RMDir /r "$APPDATA\tramtai"
    conservar:
  ${EndIf}
SectionEnd
