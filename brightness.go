package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Built-in laptop panels expose their backlight through WMI; external
// monitors generally don't, so this only applies to the internal display.

func runPowerShell(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func getBrightness() (int, error) {
	out, err := runPowerShell(`(Get-CimInstance -Namespace root/wmi -ClassName WmiMonitorBrightness -ErrorAction Stop | Select-Object -First 1).CurrentBrightness`)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

func setBrightness(level int) error {
	_, err := runPowerShell(fmt.Sprintf(`$m = Get-CimInstance -Namespace root/wmi -ClassName WmiMonitorBrightnessMethods -ErrorAction Stop | Select-Object -First 1; Invoke-CimMethod -InputObject $m -MethodName WmiSetBrightness -Arguments @{Timeout=[uint32]0; Brightness=[byte]%d} -ErrorAction Stop | Out-Null`, level))
	return err
}

// backlight raises the panel brightness in the background (PowerShell takes
// a moment to start) and puts the original level back on restore.
type backlight struct {
	target int
	orig   int
	ok     bool
	err    error
	done   chan struct{}
}

func boostBrightness(level int) *backlight {
	b := &backlight{target: level, done: make(chan struct{})}
	go func() {
		defer close(b.done)
		if b.orig, b.err = getBrightness(); b.err != nil {
			return
		}
		if b.orig == level {
			return
		}
		if b.err = setBrightness(level); b.err == nil {
			b.ok = true
		}
	}()
	return b
}

func (b *backlight) restore() {
	if b == nil {
		return
	}
	<-b.done
	if b.ok {
		setBrightness(b.orig)
	}
}

func (b *backlight) status() string {
	if b == nil {
		return "Brightness: unchanged"
	}
	select {
	case <-b.done:
	default:
		return "Brightness: setting…"
	}
	switch {
	case b.err != nil:
		return "Brightness: couldn't change"
	case b.ok:
		return fmt.Sprintf("Brightness: %d%% (was %d%%)", b.target, b.orig)
	default:
		return fmt.Sprintf("Brightness: %d%%", b.orig)
	}
}
