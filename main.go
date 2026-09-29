// burnout: a true-fullscreen LCD image-retention exerciser for Windows.
//
// It creates a borderless, topmost popup window sized to the exact physical
// pixel rectangle of one monitor (covering the taskbar and every edge), then
// drives every pixel through full-range transitions until you quit.
//
// Run with no arguments for the launcher window, or with flags from a
// terminal (see -h).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

func init() {
	// Win32 windows belong to the thread that created them.
	runtime.LockOSThread()
}

const usageText = `burnout — true-fullscreen LCD image-retention exerciser

Usage:
  burnout.exe                  open the launcher window
  burnout.exe [flags]          run directly from a terminal

Examples:
  burnout.exe -list                          list monitors
  burnout.exe -mode noise                    noise on the laptop screen until Esc
  burnout.exe -duration 1h                   stop automatically after an hour
  burnout.exe -program standard -rounds 3    run a timed routine, then show a grey check screen
  burnout.exe -mode grey                     inspect the panel for ghosting

Flags:
`

const panelGuide = `
Programs:
%s
LCD vs OLED:
  LCD (IPS, VA, TN, and Mini-LED, which is also LCD) — e.g. Legion 5 Pro 16ITH6H:
    Ghosting is usually image retention: stuck crystals and trapped charge. It is
    normally reversible. Rest the panel first (screen off overnight), then use the
    exercise modes with brightness high. This is what burnout is built for.

  OLED (Legion 9i, some Legion Pro/Slim and Yoga models):
    True burn-in is uneven wear of the organic subpixels. It is permanent, and
    flashing noise or colors will NOT repair it — bright full-screen white just
    ages the whole panel faster. Instead:
      - Short-term ghosting that is only minutes or hours old: turn the screen off
        or show varied content. Check patterns (grey) are fine for inspecting it.
      - Let the panel's own compensation run: leave the laptop asleep and plugged
        in, and enable any OLED care / pixel refresh option in Lenovo Vantage.
      - Prevent it: dark mode, auto-hide taskbar, short screen-off timeout.
      - Real burn-in that stays after all that: warranty or panel replacement.
    On OLED, use -mode grey / gradient only, and use -brightness 0.

Check what you have: Lenovo PSREF (psref.lenovo.com) lists the panel for your
model number, or run "burnout.exe -list" and search the panel ID.
`

func main() {
	procSetProcessDpiAwarenessContext.Call(dpiPerMonitor2) // no-op when the manifest already set it

	if len(os.Args) == 1 {
		releaseOwnConsole()
		runLauncher()
		return
	}
	attachParentConsole()
	runCLI()
}

// releaseOwnConsole closes the console window Windows opens when the console
// build is double-clicked. A console shared with a terminal is left alone.
func releaseOwnConsole() {
	var ids [2]uint32
	if n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&ids[0])), 2); n == 1 {
		procFreeConsole.Call()
	}
}

// attachParentConsole lets the windowless build print to the terminal it was
// started from.
func attachParentConsole() {
	if h, _, _ := procGetConsoleWindow.Call(); h != 0 {
		return
	}
	if r, _, _ := procAttachConsole.Call(attachParentProcess); r == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
}

func runCLI() {
	modeFlag := flag.String("mode", "noise", "noise | cycle | white | bars | grey | gradient | checker")
	monFlag := flag.Int("monitor", -1, "monitor index to cover (see -list); default: the built-in laptop screen,\nor the monitor under the mouse if none is found")
	durFlag := flag.Duration("duration", 0, "stop automatically after this long, e.g. 30m or 2h (0 = until Esc)")
	intervalFlag := flag.Duration("interval", 100*time.Millisecond, "time per color in cycle mode")
	greyFlag := flag.Int("grey", 128, "grey level 0-255 for grey mode")
	progFlag := flag.String("program", "", "run a timed routine: standard | gentle | quick")
	roundsFlag := flag.Int("rounds", 3, "rounds of the -program routine")
	brightFlag := flag.Int("brightness", 100, "set the built-in screen to this brightness % while running, restored on exit\n(0 = don't change)")
	listFlag := flag.Bool("list", false, "list monitors and exit")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usageText)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, panelGuide, programList())
	}
	flag.Parse()

	mons := enumMonitors()
	if len(mons) == 0 {
		fatal("no monitors found")
	}
	printMonitors(mons)
	if *listFlag {
		return
	}

	m, ok := parseMode(*modeFlag)
	if !ok {
		fatal("unknown -mode %q", *modeFlag)
	}
	if *greyFlag < 0 || *greyFlag > 255 {
		fatal("-grey must be 0-255")
	}
	if *brightFlag < 0 || *brightFlag > 100 {
		fatal("-brightness must be 0-100")
	}
	if *progFlag != "" && *durFlag > 0 {
		fatal("use either -duration or -program, not both")
	}
	cfg := config{
		monitor:    *monFlag,
		mode:       m,
		duration:   *durFlag,
		interval:   *intervalFlag,
		grey:       *greyFlag,
		program:    *progFlag,
		rounds:     *roundsFlag,
		brightness: *brightFlag,
	}
	if cfg.monitor < 0 {
		cfg.monitor = defaultMonitor(mons)
	}
	if cfg.program != "" {
		r, err := newRoutine(cfg.program, cfg.rounds, time.Now())
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("Routine %q: %d steps, %s total.\n", r.name, len(r.steps), fmtDur(r.total()))
	}

	// Ctrl+C or closing the console: exit cleanly so brightness is restored.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range sig {
			if h := sessionHwnd.Load(); h != 0 {
				procPostMessageW.Call(h, wmClose, 0, 0)
			}
		}
	}()

	fmt.Printf("Covering monitor %d. Press Esc or Q on the fullscreen window to quit.\n", cfg.monitor)
	s, err := runSession(cfg, mons)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Println(s.describe())
}

func defaultMonitor(mons []monitor) int {
	if i := internalMonitor(mons); i >= 0 {
		return i
	}
	return monitorUnderCursor(mons)
}

func (s summary) describe() string {
	end := "stopped early"
	if s.finished {
		end = "completed"
	}
	return fmt.Sprintf("%s for %s (%s).", s.what, fmtDur(s.elapsed), end)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "burnout: "+format+"\n", args...)
	os.Exit(1)
}
