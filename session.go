package main

import (
	"errors"
	"fmt"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// config describes one fullscreen session, whether it came from command-line
// flags or the launcher window.
type config struct {
	monitor    int
	mode       mode
	duration   time.Duration // 0 = until Esc; ignored when program is set
	interval   time.Duration
	grey       int
	program    string
	rounds     int
	brightness int // 0 = leave brightness alone
}

type summary struct {
	what     string
	elapsed  time.Duration
	finished bool // ran to the end rather than being stopped early
}

type app struct {
	hwnd     uintptr
	windowDC uintptr
	memDC    uintptr
	dib      uintptr
	oldBmp   uintptr
	font     uintptr
	pix      []uint32
	rowA     []uint32 // scratch row patterns for striped modes
	rowB     []uint32
	width    int32
	height   int32

	monitors []monitor
	monIdx   int

	mode        mode
	interval    time.Duration
	grey        int
	start       time.Time
	duration    time.Duration
	prog        *routine
	bl          *backlight
	helpUntil   time.Time
	helpPinned  bool
	helpHidden  bool
	frame       uint64
	reposition  bool
	lastTopmost time.Time
	closed      bool
	err         error
}

var a app

// sessionHwnd lets the Ctrl+C handler close the fullscreen window.
var sessionHwnd atomic.Uintptr

var fullscreenClass *uint16

// runSession covers the chosen monitor until the user quits, the duration
// runs out, or the window is closed. It restores brightness, the cursor and
// sleep settings before returning.
func runSession(cfg config, mons []monitor) (summary, error) {
	if cfg.monitor < 0 || cfg.monitor >= len(mons) {
		return summary{}, fmt.Errorf("monitor %d out of range (0..%d)", cfg.monitor, len(mons)-1)
	}
	a = app{
		monitors: mons,
		monIdx:   cfg.monitor,
		mode:     cfg.mode,
		interval: cfg.interval,
		grey:     cfg.grey,
		start:    time.Now(),
	}
	a.helpUntil = a.start.Add(8 * time.Second)
	what := modeNames[a.mode]
	if cfg.program != "" {
		r, err := newRoutine(cfg.program, cfg.rounds, a.start)
		if err != nil {
			return summary{}, err
		}
		a.prog = r
		what = fmt.Sprintf("Routine %q", r.name)
	} else {
		a.duration = cfg.duration
	}

	if err := createWindow(); err != nil {
		return summary{}, err
	}
	sessionHwnd.Store(a.hwnd)
	defer sessionHwnd.Store(0)

	procSetThreadExecutionState.Call(esContinuous | esDisplayRequired | esSystemRequired)
	procShowCursor.Call(0)
	if cfg.brightness > 0 && a.monitors[a.monIdx].internal {
		a.bl = boostBrightness(cfg.brightness)
	}

	quit, quitCode := loop()

	a.bl.restore()
	procShowCursor.Call(1)
	procSetThreadExecutionState.Call(esContinuous)
	freeSurface()
	if quit {
		procPostQuitMessage.Call(quitCode)
	}

	s := summary{what: what, elapsed: time.Since(a.start)}
	switch {
	case a.prog != nil && a.prog.finished:
		s.elapsed, s.finished = a.prog.total(), true
	case a.prog == nil && a.duration > 0 && s.elapsed >= a.duration:
		s.elapsed, s.finished = a.duration, true
	case a.prog == nil && cfg.program != "":
		s.what += fmt.Sprintf(", then %s", modeNames[a.mode]) // routine was taken over by a key press
	}
	return s, a.err
}

func createWindow() error {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	if fullscreenClass == nil {
		className, _ := syscall.UTF16PtrFromString("BurnoutFullscreen")
		wc := wndClassEx{
			Style:         csOwnDC,
			LpfnWndProc:   syscall.NewCallback(wndProc),
			HInstance:     hInst,
			LpszClassName: className,
		}
		wc.CbSize = uint32(unsafe.Sizeof(wc))
		if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
			return fmt.Errorf("RegisterClassEx: %v", err)
		}
		fullscreenClass = className
	}

	title, _ := syscall.UTF16PtrFromString("Burnout")
	b := a.monitors[a.monIdx].bounds
	hwnd, _, err := procCreateWindowExW.Call(
		wsExTopmost|wsExToolWindow,
		uintptr(unsafe.Pointer(fullscreenClass)),
		uintptr(unsafe.Pointer(title)),
		wsPopup,
		uintptr(b.Left), uintptr(b.Top), uintptr(b.w()), uintptr(b.h()),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %v", err)
	}
	a.hwnd = hwnd
	a.windowDC, _, _ = procGetDC.Call(hwnd)
	a.memDC, _, _ = procCreateCompatibleDC.Call(a.windowDC)
	procSetBkMode.Call(a.memDC, 1) // TRANSPARENT
	procSetTextColor.Call(a.memDC, 0x00FFFFFF)

	if err := placeOnMonitor(); err != nil {
		procDestroyWindow.Call(hwnd)
		freeSurface()
		return err
	}
	procSetForegroundWindow.Call(hwnd)
	return nil
}

// placeOnMonitor sizes the window to the exact physical bounds of the chosen
// monitor and (re)allocates the backing surface to match.
func placeOnMonitor() error {
	b := a.monitors[a.monIdx].bounds
	procSetWindowPos.Call(a.hwnd, hwndTopmost,
		uintptr(b.Left), uintptr(b.Top), uintptr(b.w()), uintptr(b.h()), swpShowWindow)

	if b.w() == a.width && b.h() == a.height && a.dib != 0 {
		return nil
	}
	a.width, a.height = b.w(), b.h()

	if a.dib != 0 {
		procSelectObject.Call(a.memDC, a.oldBmp)
		procDeleteObject.Call(a.dib)
		a.dib = 0
	}
	bmi := bitmapInfo{Header: bitmapInfoHeader{
		BiWidth:    a.width,
		BiHeight:   -a.height, // top-down rows
		BiPlanes:   1,
		BiBitCount: 32,
	}}
	bmi.Header.BiSize = uint32(unsafe.Sizeof(bmi.Header))
	var bits unsafe.Pointer
	a.dib, _, _ = procCreateDIBSection.Call(a.windowDC, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if a.dib == 0 || bits == nil {
		a.dib = 0
		return fmt.Errorf("couldn't allocate a %dx%d drawing surface", a.width, a.height)
	}
	a.oldBmp, _, _ = procSelectObject.Call(a.memDC, a.dib)
	a.pix = unsafe.Slice((*uint32)(bits), int(a.width)*int(a.height))
	a.rowA = make([]uint32, a.width)
	a.rowB = make([]uint32, a.width)

	if a.font != 0 {
		procDeleteObject.Call(a.font)
	}
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	a.font, _, _ = procCreateFontW.Call(uintptr(a.height/36), 0, 0, 0, 600, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	procSelectObject.Call(a.memDC, a.font)
	return nil
}

func freeSurface() {
	if a.memDC == 0 {
		return
	}
	if a.dib != 0 {
		procSelectObject.Call(a.memDC, a.oldBmp)
		procDeleteObject.Call(a.dib)
	}
	if a.font != 0 {
		procDeleteObject.Call(a.font)
	}
	procDeleteDC.Call(a.memDC)
	a.memDC, a.dib, a.font, a.pix = 0, 0, 0, nil
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmKeyDown:
		handleKey(wParam)
		return 0
	case wmSetCursor:
		procSetCursor.Call(0)
		return 1
	case wmEraseBkgnd:
		return 1
	case wmSysCommand:
		if c := wParam & 0xFFF0; c == scScreenSave || c == scMonitorPower {
			return 0
		}
	case wmDisplayChange:
		a.reposition = true
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		a.closed = true
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

// setModeManually switches mode from a key press; it takes over from any
// running routine.
func setModeManually(m mode) {
	a.mode = m
	if a.prog != nil && !a.prog.finished {
		a.prog = nil
	}
	a.showHelp()
}

func handleKey(vk uintptr) {
	switch {
	case vk == 0x1B || vk == 'Q': // Esc, Q
		procDestroyWindow.Call(a.hwnd)
	case vk >= '1' && vk <= '7':
		setModeManually(mode(vk - '1'))
	case vk >= 0x61 && vk <= 0x67: // numpad 1-7
		setModeManually(mode(vk - 0x61))
	case vk == 0x26 || vk == 0x28: // Up / Down: grey level
		if a.mode != modeGrey {
			setModeManually(modeGrey)
		} else if vk == 0x26 {
			a.grey = min(a.grey+16, 255)
		} else {
			a.grey = max(a.grey-16, 0)
		}
		a.showHelp()
	case vk == 'M':
		a.monIdx = (a.monIdx + 1) % len(a.monitors)
		a.fail(placeOnMonitor())
		a.showHelp()
	case vk == 'H':
		if a.helpVisible() {
			a.helpHidden, a.helpPinned = true, false
		} else {
			a.helpHidden, a.helpPinned = false, true
		}
	case vk == 0xBB || vk == 0x6B: // + (main row / numpad): faster
		a.interval = max(a.interval/2, 10*time.Millisecond)
		a.showHelp()
	case vk == 0xBD || vk == 0x6D: // - : slower
		a.interval = min(a.interval*2, 5*time.Second)
		a.showHelp()
	}
}

// fail ends the session with err, if there is one.
func (a *app) fail(err error) {
	if err != nil && a.err == nil {
		a.err = err
		procDestroyWindow.Call(a.hwnd)
	}
}

func (a *app) showHelp() {
	a.helpHidden = false
	a.helpUntil = time.Now().Add(3 * time.Second)
}

func (a *app) helpVisible() bool {
	return !a.helpHidden && (a.helpPinned || time.Now().Before(a.helpUntil))
}

// advanceRoutine applies the routine's current step, and switches to the
// grey check screen with a beep when it finishes.
func advanceRoutine(now time.Time) {
	r := a.prog
	if r == nil || r.finished {
		return
	}
	i, _ := r.at(now.Sub(r.start))
	if i == r.current {
		return
	}
	r.current = i
	if i >= len(r.steps) {
		r.finished = true
		a.mode = modeGrey
		a.helpHidden = false
		a.helpUntil = now.Add(15 * time.Second)
		procMessageBeep.Call(mbOK)
		return
	}
	s := r.steps[i]
	a.mode = s.mode
	if s.interval > 0 {
		a.interval = s.interval
	}
}

var errNoSurface = errors.New("drawing surface lost")

// loop renders frames until the fullscreen window is destroyed. If a WM_QUIT
// arrives meanwhile, it reports it so the caller can re-post it.
func loop() (quit bool, code uintptr) {
	var m msg
	for !a.closed {
		for !a.closed {
			r, _, _ := procPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, pmRemove)
			if r == 0 {
				break
			}
			if m.Message == wmQuit {
				quit, code = true, m.WParam
				procDestroyWindow.Call(a.hwnd)
				break
			}
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		if a.closed {
			break
		}

		now := time.Now()
		if a.duration > 0 && now.Sub(a.start) >= a.duration {
			procDestroyWindow.Call(a.hwnd)
			continue
		}
		advanceRoutine(now)
		if a.reposition {
			a.reposition = false
			if mons := enumMonitors(); len(mons) > 0 {
				a.monitors = mons
			}
			a.monIdx = min(a.monIdx, len(a.monitors)-1)
			a.fail(placeOnMonitor())
			if a.closed {
				break
			}
		}
		if a.pix == nil {
			a.fail(errNoSurface)
			break
		}
		// Reassert topmost periodically in case another window claimed it.
		if now.Sub(a.lastTopmost) > 2*time.Second {
			a.lastTopmost = now
			procSetWindowPos.Call(a.hwnd, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		}

		render(now)
		procBitBlt.Call(a.windowDC, 0, 0, uintptr(a.width), uintptr(a.height), a.memDC, 0, 0, srcCopy)

		// Pace to the compositor's refresh; fall back to a short sleep.
		if r, _, _ := procDwmFlush.Call(); r != 0 {
			time.Sleep(8 * time.Millisecond)
		}
		a.frame++
	}
	return quit, code
}
