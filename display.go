package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

type monitor struct {
	bounds   rect
	device   string // GDI name, e.g. \\.\DISPLAY1
	id       string // EDID vendor+product, e.g. CSO1606
	name     string // friendly name from EDID, often empty for laptop panels
	primary  bool
	internal bool // built-in laptop panel
}

func (m monitor) label() string {
	if m.name != "" {
		return m.name
	}
	if m.internal {
		return "Built-in display"
	}
	return m.id
}

type targetInfo struct {
	name     string
	internal bool
}

// targetsByDevice asks the display-config API which GDI device drives which
// physical output, so we can tell the built-in panel apart from externals.
func targetsByDevice() map[string]targetInfo {
	var nPaths, nModes uint32
	if r, _, _ := procGetDisplayConfigBufferSizes.Call(qdcOnlyActivePaths, uintptr(unsafe.Pointer(&nPaths)), uintptr(unsafe.Pointer(&nModes))); r != 0 || nPaths == 0 {
		return nil
	}
	paths := make([]dcPathInfo, nPaths)
	modes := make([]uint64, max(nModes, 1)*8) // DISPLAYCONFIG_MODE_INFO is 64 bytes
	if r, _, _ := procQueryDisplayConfig.Call(qdcOnlyActivePaths,
		uintptr(unsafe.Pointer(&nPaths)), uintptr(unsafe.Pointer(&paths[0])),
		uintptr(unsafe.Pointer(&nModes)), uintptr(unsafe.Pointer(&modes[0])), 0); r != 0 {
		return nil
	}

	out := map[string]targetInfo{}
	for _, p := range paths[:nPaths] {
		src := dcSourceName{Header: dcHeader{Type: dcGetSourceName, Adapter: p.SrcAdapter, ID: p.SrcID}}
		src.Header.Size = uint32(unsafe.Sizeof(src))
		if r, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&src))); r != 0 {
			continue
		}
		tgt := dcTargetName{Header: dcHeader{Type: dcGetTargetName, Adapter: p.TgtAdapter, ID: p.TgtID}}
		tgt.Header.Size = uint32(unsafe.Sizeof(tgt))
		procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&tgt)))

		tech := p.OutputTech
		out[syscall.UTF16ToString(src.GdiName[:])] = targetInfo{
			name:     syscall.UTF16ToString(tgt.FriendlyName[:]),
			internal: tech == outputInternal || tech == outputDPEmbedded || tech == outputUDIEmbedded,
		}
	}
	return out
}

func enumMonitors() []monitor {
	targets := targetsByDevice()
	var mons []monitor
	cb := syscall.NewCallback(func(hmon, hdc, lprc, lparam uintptr) uintptr {
		var mi monitorInfoEx
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi)))
		device := syscall.UTF16ToString(mi.SzDevice[:])

		id := "unknown"
		var dd displayDevice
		dd.Cb = uint32(unsafe.Sizeof(dd))
		if r, _, _ := procEnumDisplayDevicesW.Call(uintptr(unsafe.Pointer(&mi.SzDevice[0])), 0, uintptr(unsafe.Pointer(&dd)), 0); r != 0 {
			// DeviceID looks like MONITOR\CSO1606\{guid}\0001
			if parts := strings.Split(syscall.UTF16ToString(dd.DeviceID[:]), `\`); len(parts) > 1 {
				id = parts[1]
			}
		}
		t := targets[device]
		mons = append(mons, monitor{
			bounds:   mi.RcMonitor,
			device:   device,
			id:       id,
			name:     t.name,
			primary:  mi.DwFlags&mInfoPrimary != 0,
			internal: t.internal,
		})
		return 1
	})
	procEnumDisplayMonitors.Call(0, 0, cb, 0)
	return mons
}

func printMonitors(mons []monitor) {
	for i, m := range mons {
		var tags []string
		if m.internal {
			tags = append(tags, "internal")
		}
		if m.primary {
			tags = append(tags, "primary")
		}
		t := ""
		if len(tags) > 0 {
			t = "  (" + strings.Join(tags, ", ") + ")"
		}
		fmt.Printf("  [%d] %-18s %-8s %4dx%-4d at %d,%d%s\n",
			i, m.label(), m.id, m.bounds.w(), m.bounds.h(), m.bounds.Left, m.bounds.Top, t)
	}
}

func internalMonitor(mons []monitor) int {
	for i, m := range mons {
		if m.internal {
			return i
		}
	}
	return -1
}

func monitorUnderCursor(mons []monitor) int {
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	for i, m := range mons {
		b := m.bounds
		if pt.X >= b.Left && pt.X < b.Right && pt.Y >= b.Top && pt.Y < b.Bottom {
			return i
		}
	}
	return 0
}
