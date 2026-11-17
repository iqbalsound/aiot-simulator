Unicode true
!define APP "AIoT Simulator"
!define VER "0.0.1"
!define EXE "aiot-simulator.exe"
!define UNKEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\AIoTSimulator"
Name "${APP} ${VER}"
OutFile "AIoT-Simulator-${VER}-setup.exe"
InstallDir "$PROGRAMFILES64\${APP}"
InstallDirRegKey HKLM "Software\AIoTSimulator" "InstallDir"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
!include "MUI2.nsh"
!include "x64.nsh"
!define MUI_ICON "aiot-simulator.ico"
!define MUI_UNICON "aiot-simulator.ico"
!define MUI_ABORTWARNING
!define MUI_FINISHPAGE_RUN "$INSTDIR\${EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Start ${APP}"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "COPYING.txt"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
VIProductVersion "0.0.1.0"
VIAddVersionKey "ProductName" "${APP}"
VIAddVersionKey "FileDescription" "${APP} installer"
VIAddVersionKey "FileVersion" "${VER}"
VIAddVersionKey "LegalCopyright" "Educational use, all rights reserved"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP "This program requires 64-bit Windows."
    Abort
  ${EndIf}
  SetRegView 64
FunctionEnd

Section "Install"
  SetRegView 64
  SetOutPath "$INSTDIR"
  ; stop a running copy so the file can be replaced
  nsExec::Exec 'taskkill /F /IM ${EXE}'
  File "${EXE}"
  File "aiot-simulator.ico"
  File "README.txt"
  File "COPYING.txt"
  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateDirectory "$SMPROGRAMS\${APP}"
  CreateShortCut "$SMPROGRAMS\${APP}\${APP}.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\aiot-simulator.ico"
  CreateShortCut "$SMPROGRAMS\${APP}\Uninstall ${APP}.lnk" "$INSTDIR\Uninstall.exe"
  CreateShortCut "$DESKTOP\${APP}.lnk" "$INSTDIR\${EXE}" "" "$INSTDIR\aiot-simulator.ico"
  WriteRegStr HKLM "Software\AIoTSimulator" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${UNKEY}" "DisplayName" "${APP}"
  WriteRegStr HKLM "${UNKEY}" "DisplayVersion" "${VER}"
  WriteRegStr HKLM "${UNKEY}" "Publisher" "AI-4301 Course"
  WriteRegStr HKLM "${UNKEY}" "DisplayIcon" "$INSTDIR\aiot-simulator.ico"
  WriteRegStr HKLM "${UNKEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKLM "${UNKEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKLM "${UNKEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKLM "${UNKEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNKEY}" "NoRepair" 1
  SectionGetSize 0 $0
  WriteRegDWORD HKLM "${UNKEY}" "EstimatedSize" $0
SectionEnd

Section "Uninstall"
  SetRegView 64
  nsExec::Exec 'taskkill /F /IM ${EXE}'
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\aiot-simulator.ico"
  Delete "$INSTDIR\README.txt"
  Delete "$INSTDIR\COPYING.txt"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${APP}\${APP}.lnk"
  Delete "$SMPROGRAMS\${APP}\Uninstall ${APP}.lnk"
  RMDir "$SMPROGRAMS\${APP}"
  Delete "$DESKTOP\${APP}.lnk"
  DeleteRegKey HKLM "${UNKEY}"
  DeleteRegKey HKLM "Software\AIoTSimulator"
  ; the student workspace (%USERPROFILE%\AIoT-Simulator) is intentionally kept
SectionEnd
