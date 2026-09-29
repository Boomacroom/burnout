package main

import "syscall"

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	procRegisterClassExW              = user32.NewProc("RegisterClassExW")
	procCreateWindowExW               = user32.NewProc("CreateWindowExW")
	procDefWindowProcW                = user32.NewProc("DefWindowProcW")
	procPeekMessageW                  = user32.NewProc("PeekMessageW")
	procPostMessageW                  = user32.NewProc("PostMessageW")
	procTranslateMessage              = user32.NewProc("TranslateMessage")
	procDispatchMessageW              = user32.NewProc("DispatchMessageW")
	procPostQuitMessage               = user32.NewProc("PostQuitMessage")
	procDestroyWindow                 = user32.NewProc("DestroyWindow")
	procGetDC                         = user32.NewProc("GetDC")
	procSetCursor                     = user32.NewProc("SetCursor")
	procShowCursor                    = user32.NewProc("ShowCursor")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procEnumDisplayMonitors           = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	procEnumDisplayDevicesW           = user32.NewProc("EnumDisplayDevicesW")
	procGetCursorPos                  = user32.NewProc("GetCursorPos")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procDrawTextW                     = user32.NewProc("DrawTextW")
	procMessageBeep                   = user32.NewProc("MessageBeep")
	procGetDisplayConfigBufferSizes   = user32.NewProc("GetDisplayConfigBufferSizes")
	procQueryDisplayConfig            = user32.NewProc("QueryDisplayConfig")
	procDisplayConfigGetDeviceInfo    = user32.NewProc("DisplayConfigGetDeviceInfo")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procGdiFlush           = gdi32.NewProc("GdiFlush")
	procCreateFontW        = gdi32.NewProc("CreateFontW")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procSetTextColor       = gdi32.NewProc("SetTextColor")

	procGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")
	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")

	procDwmFlush = dwmapi.NewProc("DwmFlush")
)

const (
	wsPopup         = 0x80000000
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	csOwnDC         = 0x0020
	pmRemove        = 0x0001
	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoActivate   = 0x0010
	swpShowWindow   = 0x0040
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmSetCursor     = 0x0020
	wmDisplayChange = 0x007E
	wmKeyDown       = 0x0100
	wmSysCommand    = 0x0112
	wmQuit          = 0x0012
	scScreenSave    = 0xF140
	scMonitorPower  = 0xF170
	srcCopy         = 0x00CC0020
	dtCenter        = 0x0001
	dtWordBreak     = 0x0010
	dtNoClip        = 0x0100
	mInfoPrimary    = 0x0001
	mbOK            = 0x0040

	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
	esContinuous      = 0x80000000

	qdcOnlyActivePaths = 0x00000002
	dcGetSourceName    = 1
	dcGetTargetName    = 2

	// DISPLAYCONFIG_VIDEO_OUTPUT_TECHNOLOGY values that mean "built-in panel".
	outputInternal    = 0x80000000
	outputDPEmbedded  = 11
	outputUDIEmbedded = 13
)

var (
	hwndTopmost    = ^uintptr(0) // (HWND)-1
	dpiPerMonitor2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == (HANDLE)-4
)

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

func (r rect) w() int32 { return r.Right - r.Left }
func (r rect) h() int32 { return r.Bottom - r.Top }

type wndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type msg struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
	SzDevice  [32]uint16
}

type displayDevice struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

type bitmapInfoHeader struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

type luid struct {
	Low  uint32
	High int32
}

// DISPLAYCONFIG_PATH_INFO (72 bytes).
type dcPathInfo struct {
	SrcAdapter      luid
	SrcID           uint32
	SrcModeIdx      uint32
	SrcFlags        uint32
	TgtAdapter      luid
	TgtID           uint32
	TgtModeIdx      uint32
	OutputTech      uint32
	Rotation        uint32
	Scaling         uint32
	RefreshNum      uint32
	RefreshDen      uint32
	ScanLine        uint32
	TargetAvailable int32
	TgtFlags        uint32
	Flags           uint32
}

type dcHeader struct {
	Type    uint32
	Size    uint32
	Adapter luid
	ID      uint32
}

// DISPLAYCONFIG_SOURCE_DEVICE_NAME.
type dcSourceName struct {
	Header  dcHeader
	GdiName [32]uint16
}

// DISPLAYCONFIG_TARGET_DEVICE_NAME.
type dcTargetName struct {
	Header            dcHeader
	Flags             uint32
	OutputTech        uint32
	EdidMfg           uint16
	EdidProduct       uint16
	ConnectorInstance uint32
	FriendlyName      [64]uint16
	DevicePath        [128]uint16
}
