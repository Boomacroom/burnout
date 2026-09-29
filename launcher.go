package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// The launcher is a small settings window: pick a screen and what to run,
// press Start, and the fullscreen session takes over. When the session ends
// the launcher comes back with a summary.

const (
	idStart      = 1 // IDOK, so Enter starts
	idQuit       = 2 // IDCANCEL, so Esc quits
	idGuide      = 100
	idMonitor    = 101
	idRun        = 102
	idRounds     = 103
	idDuration   = 104
	idBrightness = 105
	idDesc       = 106
	idWarn       = 107
	idStatus     = 108
	idTitle      = 109
	idSubtitle   = 110
	idLabel      = 120 // + row index

	wmAppStart = wmApp + 1

	clientW = 520 // client size at 96 DPI
	clientH = 488
)

type runItem struct {
	label   string
	program string
	mode    mode
}

var runItems = []runItem{
	{"Routine: standard", "standard", modeNoise},
	{"Routine: gentle", "gentle", modeCycle},
	{"Routine: quick", "quick", modeNoise},
	{"Noise", "", modeNoise},
	{"Color cycle", "", modeCycle},
	{"White", "", modeWhite},
	{"Sweeping bars", "", modeBars},
	{"Check: grey", "", modeGrey},
	{"Check: gradient", "", modeGradient},
	{"Check: checkerboard", "", modeChecker},
}

var durationItems = []struct {
	label string
	d     time.Duration
}{
	{"Until I press Esc", 0},
	{"15 minutes", 15 * time.Minute},
	{"30 minutes", 30 * time.Minute},
	{"1 hour", time.Hour},
	{"2 hours", 2 * time.Hour},
	{"3 hours", 3 * time.Hour},
}

var brightnessItems = []struct {
	label string
	level int
}{
	{"100%", 100},
	{"90%", 90},
	{"80%", 80},
	{"70%", 70},
	{"Don't change", 0},
}

var roundItems = []string{"1", "2", "3", "4", "5", "6"}

var modeAbout = map[mode]string{
	modeNoise:    "Random full-on / full-off pixels every frame. The most effective exercise for LCD image retention.",
	modeCycle:    "Solid red, green, blue, white and black in turn.",
	modeWhite:    "Solid white. Gentle, and useful between noise sessions.",
	modeBars:     "Black and white bars sweeping across the screen.",
	modeGrey:     "Mid-grey check screen: ghosting is easiest to see here. Use ↑ / ↓ to change the level.",
	modeGradient: "Black-to-white gradient for spotting uneven areas.",
	modeChecker:  "Black and white checkerboard for spotting ghost outlines.",
}

const oledGuide = "Is your screen LCD or OLED?\n\n" +
	"LCD — IPS, VA, TN, and Mini-LED (which is also LCD). The Legion 5 Pro 16ITH6H is an IPS LCD.\n" +
	"Ghosting on an LCD is usually image retention: stuck crystals and trapped charge. It is normally reversible.\n" +
	"  1. Rest the panel first: screen off or lid closed overnight.\n" +
	"  2. Run the standard routine with brightness at 100%.\n" +
	"  3. Check on the grey screen, and repeat on later days if needed.\n" +
	"  4. Remove the cause: hide Chrome's side panel, auto-hide the taskbar, use a short screen-off timeout.\n\n" +
	"OLED — Legion 9i and some Legion Pro, Slim and Yoga models.\n" +
	"OLED burn-in is uneven wear of the pixels themselves. It is permanent, and flashing noise or colors won't repair it; bright white only ages the whole panel faster.\n" +
	"  • Ghosting only minutes or hours old: turn the screen off or show varied content.\n" +
	"  • Let the panel's own compensation run: leave the laptop asleep and plugged in, and turn on any OLED care or pixel refresh option in Lenovo Vantage.\n" +
	"  • Prevent it with dark mode, an auto-hidden taskbar and a short screen-off timeout.\n" +
	"  • Burn-in that survives all that needs a warranty claim or a new panel.\n" +
	"  • On OLED, use only the grey and gradient check screens, with brightness set to \"Don't change\".\n\n" +
	"Not sure? Look up your model number on Lenovo PSREF (psref.lenovo.com)."

type launcher struct {
	hwnd     uintptr
	dpi      int
	font     uintptr
	bold     uintptr
	mons     []monitor
	ctl      map[int]uintptr
	status   string
	textGrey uintptr // COLORREF for secondary text
}

var ui launcher

func runLauncher() {
	ui = launcher{ctl: map[int]uintptr{}, textGrey: 0x00606060}

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("BurnoutLauncher")
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)
	icon, _, _ := procLoadIconW.Call(hInst, 1) // resource 1 from winres/burnout.rc
	wc := wndClassEx{
		LpfnWndProc:   syscall.NewCallback(launcherProc),
		HInstance:     hInst,
		HIcon:         icon,
		HIconSm:       icon,
		HCursor:       cursor,
		HbrBackground: colorWindow + 1,
		LpszClassName: className,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	title, _ := syscall.UTF16PtrFromString("Burnout")
	const style = wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox
	hwnd, _, _ := procCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), style,
		cwUseDefault, cwUseDefault, 100, 100, 0, 0, hInst, 0)
	if hwnd == 0 {
		return
	}
	ui.hwnd = hwnd
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	ui.dpi = max(int(dpi), 96)

	ui.createControls()
	ui.fillMonitors()
	ui.layout()
	ui.centerOnScreen(style)
	ui.refresh()
	procShowWindow.Call(hwnd, swShow)
	procSetForegroundWindow.Call(hwnd)

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		if m.Message == wmAppStart {
			ui.start()
			continue
		}
		if r, _, _ := procIsDialogMessageW.Call(hwnd, uintptr(unsafe.Pointer(&m))); r != 0 {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (l *launcher) scale(v int) int { return v * l.dpi / 96 }

func (l *launcher) add(id int, class, text string, style uintptr) uintptr {
	c, _ := syscall.UTF16PtrFromString(class)
	t, _ := syscall.UTF16PtrFromString(text)
	hInst, _, _ := procGetModuleHandleW.Call(0)
	h, _, _ := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(t)),
		wsChild|wsVisible|style, 0, 0, 10, 10, l.hwnd, uintptr(id), hInst, 0)
	l.ctl[id] = h
	return h
}

func (l *launcher) combo(id int, items []string, sel int) {
	h := l.add(id, "COMBOBOX", "", cbsDropDownList|wsVScroll|wsTabStop)
	for _, s := range items {
		p, _ := syscall.UTF16PtrFromString(s)
		procSendMessageW.Call(h, cbAddString, 0, uintptr(unsafe.Pointer(p)))
	}
	procSendMessageW.Call(h, cbSetCurSel, uintptr(sel), 0)
}

func (l *launcher) selected(id int) int {
	r, _, _ := procSendMessageW.Call(l.ctl[id], cbGetCurSel, 0, 0)
	return max(int(int32(r)), 0)
}

func (l *launcher) setText(id int, s string) {
	p, _ := syscall.UTF16PtrFromString(s)
	procSetWindowTextW.Call(l.ctl[id], uintptr(unsafe.Pointer(p)))
}

func (l *launcher) enable(id int, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	procEnableWindow.Call(l.ctl[id], v)
}

var rowLabels = []string{"Screen", "Run", "Rounds", "Stop after", "Brightness"}

func (l *launcher) createControls() {
	l.add(idTitle, "STATIC", "Burnout", ssLeft|ssNoPrefix)
	l.add(idSubtitle, "STATIC", "Clears LCD image retention by exercising every pixel, edge to edge.", ssLeft|ssNoPrefix)
	for i, s := range rowLabels {
		l.add(idLabel+i, "STATIC", s, ssLeft|ssNoPrefix)
	}

	l.combo(idMonitor, nil, 0)
	runs := make([]string, len(runItems))
	for i, r := range runItems {
		runs[i] = r.label
	}
	l.combo(idRun, runs, 0)
	l.combo(idRounds, roundItems, 2)
	durs := make([]string, len(durationItems))
	for i, d := range durationItems {
		durs[i] = d.label
	}
	l.combo(idDuration, durs, 3)
	brights := make([]string, len(brightnessItems))
	for i, b := range brightnessItems {
		brights[i] = b.label
	}
	l.combo(idBrightness, brights, 0)

	l.add(idDesc, "STATIC", "", ssLeft|ssNoPrefix)
	l.add(idWarn, "STATIC", "Noise, color cycle, bars and the checkerboard flash or are high-contrast: don't watch if you are photosensitive. "+
		"While running, press Esc to stop and H for help.", ssLeft|ssNoPrefix)
	l.add(idStatus, "STATIC", "", ssLeft|ssNoPrefix)

	l.add(idGuide, "BUTTON", "LCD vs OLED…", bsPushButton|wsTabStop)
	l.add(idQuit, "BUTTON", "Quit", bsPushButton|wsTabStop)
	l.add(idStart, "BUTTON", "Start", bsDefPushButton|wsTabStop)
}

func (l *launcher) fillMonitors() {
	prev := ""
	if len(l.mons) > 0 {
		if i := l.selected(idMonitor); i < len(l.mons) {
			prev = l.mons[i].device
		}
	}
	l.mons = enumMonitors()
	h := l.ctl[idMonitor]
	procSendMessageW.Call(h, cbResetContent, 0, 0)
	sel := defaultMonitor(l.mons)
	for i, m := range l.mons {
		label := fmt.Sprintf("%s  —  %d × %d", m.label(), m.bounds.w(), m.bounds.h())
		if m.internal && m.name != "" {
			label += "  (built-in)"
		} else if !m.internal && m.primary {
			label += "  (primary)"
		}
		p, _ := syscall.UTF16PtrFromString(label)
		procSendMessageW.Call(h, cbAddString, 0, uintptr(unsafe.Pointer(p)))
		if m.device == prev {
			sel = i
		}
	}
	procSendMessageW.Call(h, cbSetCurSel, uintptr(sel), 0)
}

// layout positions every control for the current DPI and resets fonts.
func (l *launcher) layout() {
	if l.font != 0 {
		procDeleteObject.Call(l.font)
		procDeleteObject.Call(l.bold)
	}
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	mkFont := func(pt, weight int) uintptr {
		f, _, _ := procCreateFontW.Call(uintptr(-pt*l.dpi/72), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
		return f
	}
	l.font = mkFont(9, 400)
	l.bold = mkFont(16, 600)

	place := func(id, x, y, w, h int) {
		c := l.ctl[id]
		procMoveWindow.Call(c, uintptr(l.scale(x)), uintptr(l.scale(y)), uintptr(l.scale(w)), uintptr(l.scale(h)), 1)
		f := l.font
		if id == idTitle {
			f = l.bold
		}
		procSendMessageW.Call(c, wmSetFont, f, 1)
	}

	const m, fieldX = 24, 150
	place(idTitle, m, 16, clientW-2*m, 36)
	place(idSubtitle, m, 54, clientW-2*m, 20)
	for i := range rowLabels {
		y := 92 + i*36
		place(idLabel+i, m, y+4, fieldX-m, 20)
	}
	fieldW := clientW - m - fieldX
	place(idMonitor, fieldX, 92, fieldW, 240)
	place(idRun, fieldX, 128, fieldW, 320)
	place(idRounds, fieldX, 164, 110, 200)
	place(idDuration, fieldX, 200, fieldW, 220)
	place(idBrightness, fieldX, 236, 160, 200)
	place(idDesc, m, 280, clientW-2*m, 72)
	place(idWarn, m, 356, clientW-2*m, 40)
	place(idStatus, m, 400, clientW-2*m, 20)
	place(idGuide, m, 440, 130, 30)
	place(idQuit, clientW-m-96-8-96, 440, 96, 30)
	place(idStart, clientW-m-96, 440, 96, 30)
}

func (l *launcher) outerSize(style uintptr) (int32, int32) {
	r := rect{Right: int32(l.scale(clientW)), Bottom: int32(l.scale(clientH))}
	procAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&r)), style, 0, 0, uintptr(l.dpi))
	return r.w(), r.h()
}

func (l *launcher) centerOnScreen(style uintptr) {
	w, h := l.outerSize(style)
	b := l.mons[monitorUnderCursor(l.mons)].bounds
	x := b.Left + (b.w()-w)/2
	y := b.Top + (b.h()-h)/2
	procSetWindowPos.Call(l.hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0x0004) // SWP_NOZORDER
	// Moving onto another monitor may have changed the DPI.
	if dpi, _, _ := procGetDpiForWindow.Call(l.hwnd); int(dpi) != l.dpi && dpi != 0 {
		l.dpi = int(dpi)
		l.layout()
		w, h = l.outerSize(style)
		procSetWindowPos.Call(l.hwnd, 0, 0, 0, uintptr(w), uintptr(h), 0x0004|swpNoMove)
	}
}

// refresh enables the fields that apply and rewrites the description.
func (l *launcher) refresh() {
	item := runItems[l.selected(idRun)]
	routine := item.program != ""
	l.enable(idRounds, routine)
	l.enable(idDuration, !routine)

	internal := false
	if i := l.selected(idMonitor); i < len(l.mons) {
		internal = l.mons[i].internal
	}
	l.enable(idBrightness, internal)

	var desc string
	if routine {
		p := programs[item.program]
		rounds := l.selected(idRounds) + 1
		var parts []string
		for _, s := range p.round {
			parts = append(parts, fmt.Sprintf("%d min %s", int(s.d.Minutes()), strings.ToLower(modeNames[s.mode])))
		}
		r, _ := newRoutine(item.program, rounds, time.Time{})
		plural := "s"
		if rounds == 1 {
			plural = ""
		}
		desc = fmt.Sprintf("Each round: %s. %d round%s, %s in total. Ends on a grey check screen with a beep.",
			strings.Join(parts, ", then "), rounds, plural, fmtDurWords(r.total()))
	} else {
		desc = modeAbout[item.mode]
	}
	if !internal {
		desc += "  Brightness can only be changed on the built-in screen."
	}
	l.setText(idDesc, desc)
	l.setText(idStatus, l.status)
}

func (l *launcher) config() config {
	item := runItems[l.selected(idRun)]
	cfg := config{
		monitor:    min(l.selected(idMonitor), len(l.mons)-1),
		mode:       item.mode,
		interval:   100 * time.Millisecond,
		grey:       128,
		program:    item.program,
		rounds:     l.selected(idRounds) + 1,
		brightness: brightnessItems[l.selected(idBrightness)].level,
	}
	if item.program == "" {
		cfg.duration = durationItems[l.selected(idDuration)].d
	}
	if !l.mons[cfg.monitor].internal {
		cfg.brightness = 0
	}
	return cfg
}

func (l *launcher) start() {
	cfg := l.config()
	procShowWindow.Call(l.hwnd, swHide)
	s, err := runSession(cfg, l.mons)
	procShowWindow.Call(l.hwnd, swShow)
	procSetForegroundWindow.Call(l.hwnd)
	if err != nil {
		l.message("Couldn't run", err.Error())
		return
	}
	l.status = "Last session: " + s.describe()
	l.refresh()
}

func (l *launcher) message(title, text string) {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(title)
	procMessageBoxW.Call(l.hwnd, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), mbIconInfo)
}

func launcherProc(hwnd, message, wParam, lParam uintptr) uintptr {
	switch message {
	case wmCommand:
		id, code := int(wParam&0xFFFF), int(wParam>>16&0xFFFF)
		switch id {
		case idStart:
			procPostMessageW.Call(hwnd, wmAppStart, 0, 0)
		case idQuit:
			procDestroyWindow.Call(hwnd)
		case idGuide:
			ui.message("LCD vs OLED", oledGuide)
		case idMonitor, idRun, idRounds:
			if code == cbnSelChange {
				ui.refresh()
			}
		}
		return 0
	case wmCtlColorStatic:
		hdc := wParam
		id, _, _ := procGetDlgCtrlID.Call(lParam)
		switch int(id) {
		case idSubtitle, idDesc, idWarn, idStatus:
			procSetTextColor.Call(hdc, ui.textGrey)
		}
		bg, _, _ := procGetSysColor.Call(colorWindow)
		procSetBkColor.Call(hdc, bg)
		brush, _, _ := procGetSysColorBrush.Call(colorWindow)
		return brush
	case wmDpiChanged:
		ui.dpi = int(wParam >> 16 & 0xFFFF)
		r := (*rect)(unsafe.Add(nil, lParam)) // suggested window rect for the new DPI
		procSetWindowPos.Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.w()), uintptr(r.h()), 0x0004|swpNoActivate)
		ui.layout()
		return 0
	case wmDisplayChange:
		ui.fillMonitors()
		ui.refresh()
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func fmtDurWords(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h == 0:
		return fmt.Sprintf("%d min", m)
	case m == 0:
		return fmt.Sprintf("%d h", h)
	default:
		return fmt.Sprintf("%d h %d min", h, m)
	}
}
