package main

// Bindings used by the launcher window and console handling.

var (
	procGetMessageW              = user32.NewProc("GetMessageW")
	procIsDialogMessageW         = user32.NewProc("IsDialogMessageW")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procMoveWindow               = user32.NewProc("MoveWindow")
	procEnableWindow             = user32.NewProc("EnableWindow")
	procSetWindowTextW           = user32.NewProc("SetWindowTextW")
	procGetDlgCtrlID             = user32.NewProc("GetDlgCtrlID")
	procGetDpiForWindow          = user32.NewProc("GetDpiForWindow")
	procAdjustWindowRectExForDpi = user32.NewProc("AdjustWindowRectExForDpi")
	procGetSysColor              = user32.NewProc("GetSysColor")
	procGetSysColorBrush         = user32.NewProc("GetSysColorBrush")
	procMessageBoxW              = user32.NewProc("MessageBoxW")
	procLoadCursorW              = user32.NewProc("LoadCursorW")
	procLoadIconW                = user32.NewProc("LoadIconW")

	procSetBkColor = gdi32.NewProc("SetBkColor")
	procDeleteDC   = gdi32.NewProc("DeleteDC")

	procGetConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procAttachConsole         = kernel32.NewProc("AttachConsole")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
)

const (
	wsOverlapped  = 0x00000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000
	wsChild       = 0x40000000
	wsVisible     = 0x10000000
	wsTabStop     = 0x00010000
	wsVScroll     = 0x00200000

	bsPushButton    = 0x0000
	bsDefPushButton = 0x0001
	cbsDropDownList = 0x0003
	ssLeft          = 0x0000
	ssNoPrefix      = 0x0080

	cbAddString    = 0x0143
	cbGetCurSel    = 0x0147
	cbResetContent = 0x014B
	cbSetCurSel    = 0x014E
	cbnSelChange   = 1

	wmSetFont        = 0x0030
	wmCommand        = 0x0111
	wmCtlColorStatic = 0x0138
	wmDpiChanged     = 0x02E0
	wmApp            = 0x8000

	swHide = 0
	swShow = 5

	colorWindow  = 5
	cwUseDefault = 0x80000000
	idcArrow     = 32512
	mbIconInfo   = 0x0040

	attachParentProcess = ^uintptr(0) // (DWORD)-1
)
