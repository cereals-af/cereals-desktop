Unicode true
RequestExecutionLevel user
SetCompressor /SOLID lzma
ManifestDPIAware true

!include "MUI2.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!include "FileFunc.nsh"

!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\cn.creals.desktop"
!define WEBVIEW_KEY "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TEXT "安装 cereals 本地 Kubernetes 工作台。$\r$\n$\r$\n更新前请从托盘选择“退出 cereals”；关闭窗口仅会隐藏应用。$\r$\n$\r$\n如果系统缺少 WebView2 运行时，安装程序会联网下载安装。"
!define MUI_FINISHPAGE_RUN "$INSTDIR\cereals.exe"
!define MUI_FINISHPAGE_RUN_TEXT "启动 cereals"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"

Name "cereals ${PRODUCT_VERSION}"
OutFile "${INSTALLER_OUTPUT}"
InstallDir "$LOCALAPPDATA\Programs\cereals"
ShowInstDetails show
ShowUninstDetails show
VIProductVersion "${PRODUCT_VERSION}.0"
VIAddVersionKey /LANG=2052 "ProductName" "cereals"
VIAddVersionKey /LANG=2052 "ProductVersion" "${PRODUCT_VERSION}"
VIAddVersionKey /LANG=2052 "CompanyName" "cereals"
VIAddVersionKey /LANG=2052 "FileDescription" "cereals x86-64 安装程序"
VIAddVersionKey /LANG=2052 "FileVersion" "${PRODUCT_VERSION}"
VIAddVersionKey /LANG=2052 "LegalCopyright" "cereals"

Function .onInit
  ${IfNot} ${AtLeastWin10}
    MessageBox MB_OK|MB_ICONSTOP "需要 Windows 10 或更新版本。" /SD IDOK
    SetErrorLevel 64
    Quit
  ${EndIf}
  ${IfNot} ${IsNativeAMD64}
    MessageBox MB_OK|MB_ICONSTOP "此安装包适用于 Intel/AMD x86-64 Windows。" /SD IDOK
    SetErrorLevel 65
    Quit
  ${EndIf}
  SetShellVarContext current
  SetRegView 64
  ReadRegStr $0 HKCU "${UNINSTALL_KEY}" "InstallLocation"
  ${If} $0 != ""
    StrCpy $INSTDIR $0
  ${EndIf}
FunctionEnd

# Microsoft documents machine registration in the 32-bit registry view,
# and user registration in HKCU. Treat 0.0.0.0 as missing.
Function DetectWebView
  SetRegView 32
  ReadRegStr $0 HKLM "${WEBVIEW_KEY}" "pv"
  SetRegView 64
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    ReadRegStr $0 HKCU "${WEBVIEW_KEY}" "pv"
  ${EndIf}
  ${If} $0 == "0.0.0.0"
    StrCpy $0 ""
  ${EndIf}
FunctionEnd

Function EnsureWebView
  Call DetectWebView
  ${If} $0 != ""
    Return
  ${EndIf}
  DetailPrint "正在安装 Microsoft Edge WebView2 运行时（需要网络连接）…"
  InitPluginsDir
  SetOutPath "$PLUGINSDIR"
  File /oname=MicrosoftEdgeWebview2Setup.exe "${WEBVIEW_BOOTSTRAPPER}"
  ClearErrors
  ExecWait '"$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install' $1
  Call DetectWebView
  ${If} $0 == ""
    MessageBox MB_OK|MB_ICONSTOP "WebView2 运行时安装未完成（退出码 $1）。请检查网络，或手动安装 Microsoft WebView2 Runtime 后重新运行此安装程序。" /SD IDOK
    SetErrorLevel 66
    Abort
  ${EndIf}
FunctionEnd

Section "cereals"
  Call EnsureWebView
  SetOutPath "$INSTDIR"
  # Upgrade the old product in the existing installation directory.
  IfFileExists "$INSTDIR\creals.exe" 0 legacy_removed
  ClearErrors
  Delete "$INSTDIR\creals.exe"
  ${If} ${Errors}
    MessageBox MB_OK|MB_ICONSTOP "请先从托盘退出旧版 creals，再安装 cereals。" /SD IDOK
    Abort
  ${EndIf}
  legacy_removed:
  SetOverwrite on
  File /oname=cereals.exe "${APP_BINARY}"
  CreateShortcut "$SMPROGRAMS\cereals.lnk" "$INSTDIR\cereals.exe"
  CreateShortcut "$DESKTOP\cereals.lnk" "$INSTDIR\cereals.exe"
  Delete "$SMPROGRAMS\creals.lnk"
  Delete "$DESKTOP\creals.lnk"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "cereals"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${PRODUCT_VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "cereals"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\cereals.exe"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '$\"$INSTDIR\uninstall.exe$\"'
  WriteRegStr HKCU "${UNINSTALL_KEY}" "QuietUninstallString" '$\"$INSTDIR\uninstall.exe$\" /S'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "EstimatedSize" $0
SectionEnd

Section "Uninstall"
  SetShellVarContext current
  SetRegView 64
  ClearErrors
  Delete "$INSTDIR\cereals.exe"
  ${If} ${Errors}
    MessageBox MB_OK|MB_ICONSTOP "无法删除应用文件。请从托盘退出 cereals 后重试。" /SD IDOK
    SetErrorLevel 67
    Abort
  ${EndIf}
  Delete "$SMPROGRAMS\cereals.lnk"
  Delete "$DESKTOP\cereals.lnk"
  Delete "$INSTDIR\uninstall.exe"
  # Do not recursively delete the chosen install folder or user configuration.
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
SectionEnd
