package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

type mode int

const (
	// Exercise modes.
	modeNoise mode = iota
	modeCycle
	modeWhite
	modeBars
	// Check patterns, for inspecting the panel.
	modeGrey
	modeGradient
	modeChecker
)

var modeNames = []string{"Noise", "Color cycle", "White", "Sweeping bars", "Grey", "Gradient", "Checkerboard"}
var modeFlagNames = []string{"noise", "cycle", "white", "bars", "grey", "gradient", "checker"}

func parseMode(s string) (mode, bool) {
	s = strings.ToLower(s)
	if s == "gray" {
		s = "grey"
	}
	for i, n := range modeFlagNames {
		if n == s {
			return mode(i), true
		}
	}
	return 0, false
}

// Pixel values in 32bpp DIB order: 0x00RRGGBB.
const (
	black = 0x000000
	white = 0xFFFFFF
	red   = 0xFF0000
	green = 0x00FF00
	blue  = 0x0000FF
)

// Every subpixel fully on or fully off: maximises liquid-crystal swing.
var noiseLUT = [8]uint32{black, blue, green, 0x00FFFF, red, 0xFF00FF, 0xFFFF00, white}

var cycleColors = []uint32{red, green, blue, white, black}

func greyPixel(level int) uint32 { return uint32(level) * 0x010101 }

func render(now time.Time) {
	procGdiFlush.Call() // GDI must finish with the surface before we write to it
	w, h := int(a.width), int(a.height)
	elapsed := now.Sub(a.start)

	switch a.mode {
	case modeNoise:
		fillNoise(a.pix, a.frame+1)
	case modeCycle:
		step := int(elapsed / a.interval)
		fillSolid(a.pix, cycleColors[step%len(cycleColors)])
	case modeWhite:
		fillSolid(a.pix, white)
	case modeBars:
		barW := max(w/16, 1)
		offset := int(elapsed.Seconds()*float64(w)/4) % (2 * barW) // one screen width every 4 s
		for x := range a.rowA {
			a.rowA[x] = black
			if ((x+offset)/barW)%2 == 0 {
				a.rowA[x] = white
			}
		}
		repeatRows(h, a.rowA)
	case modeGrey:
		fillSolid(a.pix, greyPixel(a.grey))
	case modeGradient:
		for x := range a.rowA {
			a.rowA[x] = greyPixel(x * 255 / max(w-1, 1))
		}
		repeatRows(h, a.rowA)
	case modeChecker:
		cell := max(h/10, 1)
		for x := range a.rowA {
			a.rowA[x], a.rowB[x] = white, black
			if (x/cell)%2 == 1 {
				a.rowA[x], a.rowB[x] = black, white
			}
		}
		repeatRows(cell, a.rowA, a.rowB)
	}

	if a.helpVisible() {
		drawHelp(now)
	}
}

// repeatRows fills the surface with the given row patterns, switching to the
// next pattern every bandH rows.
func repeatRows(bandH int, rows ...[]uint32) {
	w := int(a.width)
	parallel(int(a.height), func(lo, hi int) {
		for y := lo; y < hi; y++ {
			copy(a.pix[y*w:(y+1)*w], rows[(y/bandH)%len(rows)])
		}
	})
}

func fillSolid(pix []uint32, c uint32) {
	parallel(len(pix), func(lo, hi int) {
		p := pix[lo:hi]
		for i := range p {
			p[i] = c
		}
	})
}

func fillNoise(pix []uint32, seed uint64) {
	parallel(len(pix), func(lo, hi int) {
		x := (seed*0x9E3779B97F4A7C15 + uint64(lo)*0xBF58476D1CE4E5B9) | 1
		p := pix[lo:hi]
		for i := 0; i < len(p); {
			x ^= x << 13
			x ^= x >> 7
			x ^= x << 17
			r := x
			for k := 0; k < 21 && i < len(p); k++ { // 21 pixels x 3 bits per draw
				p[i] = noiseLUT[r&7]
				r >>= 3
				i++
			}
		}
	})
}

// parallel splits [0,n) across CPUs and waits for all parts.
func parallel(n int, fn func(lo, hi int)) {
	workers := runtime.NumCPU()
	size := max((n+workers-1)/workers, 1)
	var wg sync.WaitGroup
	for lo := 0; lo < n; lo += size {
		hi := min(lo+size, n)
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(lo, hi)
		}()
	}
	wg.Wait()
}

func drawHelp(now time.Time) {
	w, h := int(a.width), int(a.height)
	box := rect{Left: int32(w * 12 / 100), Top: int32(h * 22 / 100), Right: int32(w * 88 / 100), Bottom: int32(h * 64 / 100)}
	parallel(int(box.h()), func(lo, hi int) {
		for y := int(box.Top) + lo; y < int(box.Top)+hi; y++ {
			row := a.pix[y*w+int(box.Left) : y*w+int(box.Right)]
			for i := range row {
				row[i] = 0x101418
			}
		}
	})

	elapsed := now.Sub(a.start)
	m := a.monitors[a.monIdx]

	var status string
	switch {
	case a.prog != nil && a.prog.finished:
		status = fmt.Sprintf("Routine %q complete after %s. Look for ghosting on this grey screen (↑/↓ to change the level), then press Esc.",
			a.prog.name, fmtDur(a.prog.total()))
	case a.prog != nil:
		i, left := a.prog.at(now.Sub(a.prog.start))
		total := a.prog.total() - now.Sub(a.prog.start)
		status = fmt.Sprintf("Routine %q: step %d of %d (%s), %s left in step  ·  %s left in total",
			a.prog.name, i+1, len(a.prog.steps), modeNames[a.prog.steps[i].mode], fmtDur(left), fmtDur(total))
	case a.duration > 0:
		status = fmt.Sprintf("Elapsed %s  ·  %s left", fmtDur(elapsed), fmtDur(a.duration-elapsed))
	default:
		status = fmt.Sprintf("Elapsed %s  ·  runs until Esc", fmtDur(elapsed))
	}

	modeDetail := modeNames[a.mode]
	switch a.mode {
	case modeCycle:
		modeDetail += fmt.Sprintf(" (%v per color)", a.interval)
	case modeGrey:
		modeDetail += fmt.Sprintf(" (level %d)", a.grey)
	}

	text := fmt.Sprintf(
		"BURNOUT  —  LCD image-retention exerciser\n\n"+
			"Exercise:   [1] Noise    [2] Color cycle    [3] White    [4] Sweeping bars\n"+
			"Check:   [5] Grey (↑/↓ level)    [6] Gradient    [7] Checkerboard\n"+
			"[+] / [−] cycle speed    [M] next monitor    [H] show / hide panel    [Esc] quit\n\n"+
			"Mode: %s    ·    Monitor %d of %d: %s (%dx%d)    ·    %s\n"+
			"%s\n\n"+
			"Modes 1, 2, 4 and 7 flash or high-contrast. Don't watch if you are photosensitive.",
		modeDetail, a.monIdx+1, len(a.monitors), m.label(), m.bounds.w(), m.bounds.h(), a.bl.status(),
		status)

	pad := int32(h / 40)
	tr := rect{Left: box.Left + pad, Top: box.Top + pad, Right: box.Right - pad, Bottom: box.Bottom - pad}
	t, _ := syscall.UTF16FromString(text)
	procDrawTextW.Call(a.memDC, uintptr(unsafe.Pointer(&t[0])), uintptr(len(t)-1), uintptr(unsafe.Pointer(&tr)), dtCenter|dtWordBreak|dtNoClip)
}

func fmtDur(d time.Duration) string {
	d = max(d, 0).Round(time.Second)
	return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}
