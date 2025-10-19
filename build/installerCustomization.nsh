!include "Sections.nsh"

!define MUI_WELCOMEPAGE_TITLE "欢迎安装 PromptGen"
!define MUI_WELCOMEPAGE_TEXT "PromptGen 是一套 Electron + Go 的提示词工作台，将离线桌面与后台治理能力整合到一个客户端。请点击“下一步”继续安装，并阅读下页的产品简介与协议。"

!define MUI_DIRECTORYPAGE_TEXT_TOP "选择 PromptGen 的安装位置。建议安装在默认目录，或根据团队策略指定到共享磁盘。"

!define MUI_COMPONENTSPAGE_TEXT_TOP "选择要执行的附加任务："

!define MUI_FINISHPAGE_TITLE "PromptGen 安装完成"
!define MUI_FINISHPAGE_TEXT "PromptGen 已成功安装。您可以通过桌面或开始菜单快捷方式快速启动，也可以打开产品说明了解离线账号配置。"
!define PROMPTGEN_EXE_NAME "PromptGen.exe"
!define PROMPTGEN_SHORTCUT_NAME "PromptGen"
!define PROMPTGEN_STARTMENU_DIR "PromptGen"

Section /o "创建桌面快捷方式" SecDesktopShortcut
  SectionIn 1
  SetOutPath "$INSTDIR"
  CreateShortCut "$DESKTOP\\${PROMPTGEN_SHORTCUT_NAME}.lnk" "$INSTDIR\\${PROMPTGEN_EXE_NAME}"
SectionEnd

Section /o "创建开始菜单快捷方式" SecStartMenuShortcut
  SectionIn 1
  CreateDirectory "$SMPROGRAMS\\${PROMPTGEN_STARTMENU_DIR}"
  CreateShortCut "$SMPROGRAMS\\${PROMPTGEN_STARTMENU_DIR}\\${PROMPTGEN_SHORTCUT_NAME}.lnk" "$INSTDIR\\${PROMPTGEN_EXE_NAME}"
  CreateShortCut "$SMPROGRAMS\\${PROMPTGEN_STARTMENU_DIR}\\卸载 PromptGen.lnk" "$INSTDIR\\Uninstall ${PROMPTGEN_SHORTCUT_NAME}.exe"
SectionEnd

!macro customInit
  !insertmacro MUI_LANGDLL_DISPLAY
  SectionSetFlags ${SecDesktopShortcut} ${SF_SELECTED}
  SectionSetFlags ${SecStartMenuShortcut} ${SF_SELECTED}
!macroend

!macro customUnInstall
  Delete "$DESKTOP\\${PROMPTGEN_SHORTCUT_NAME}.lnk"
  Delete "$SMPROGRAMS\\${PROMPTGEN_STARTMENU_DIR}\\${PROMPTGEN_SHORTCUT_NAME}.lnk"
  Delete "$SMPROGRAMS\\${PROMPTGEN_STARTMENU_DIR}\\卸载 PromptGen.lnk"
  RMDir "$SMPROGRAMS\\${PROMPTGEN_STARTMENU_DIR}"
!macroend
