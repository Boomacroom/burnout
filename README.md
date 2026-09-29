# burnout

True-fullscreen LCD image-retention exerciser for Windows. It uses a borderless, topmost Win32 window sized to the monitor's exact physical pixels. It covers the taskbar and every edge, blocks the screensaver and display sleep, and hides the cursor. It has no dependencies and is pure Go.

## Build

```
.\build.ps1              # vet, test, and build both executables
.\build.ps1 -Resources   # also rebuild the icon/manifest resources after editing winres\
```

This produces two executables from the same code:

- **`burnout-gui.exe`**: double-click this one or pin it to Start. It opens the launcher with no console window.
- **`burnout.exe`**: for the terminal. With no arguments it opens the launcher too, but a console window flashes briefly when you double-click it.

`winres\` holds the app icon and the Windows manifest. The manifest enables modern controls and sharp per-monitor scaling. They are compiled into `rsrc_windows_amd64.syso`, which `go build` picks up automatically. Rebuilding them needs `windres` from MSYS2.

## Launcher

Open `burnout-gui.exe`, then choose:

1. The **screen**. The built-in laptop screen is selected by default.
2. What to **run**: a routine, a single exercise mode, or a check pattern.
3. The **rounds** (routines) or when to **stop** (single modes).
4. The **brightness** to use while running. This only applies to the built-in screen.

Press **Start** or Enter to run. The launcher hides while the session runs and comes back with a summary when you press Esc. **LCD vs OLED…** explains which modes are safe for your kind of panel.

## Command line

```
burnout.exe -list                           # list monitors; the laptop screen is marked (internal)
burnout.exe -mode noise                     # noise on the laptop screen until Esc
burnout.exe -duration 1h                    # auto-stop after an hour
burnout.exe -program standard -rounds 3     # timed routine, then a grey check screen and a beep
burnout.exe -mode grey -grey 128            # inspect the panel for ghosting
burnout.exe -h                              # all flags, routines, and LCD vs OLED guidance
```

When no `-monitor` is given, the app finds the built-in laptop screen on its own. It falls back to the monitor under the mouse if there is no built-in screen.

On the built-in screen, brightness is raised to 100% while the app runs and restored when it exits, including after Ctrl+C. Use `-brightness 0` to leave it alone, or `-brightness 80` to pick a level.

### Keys

| Key | Action |
| --- | --- |
| `1` `2` `3` `4` | Exercise modes: noise, color cycle, white, sweeping bars |
| `5` `6` `7` | Check patterns: grey, gradient, checkerboard |
| `↑` / `↓` | Grey level, in steps of 16 |
| `+` / `-` | Color-cycle speed |
| `M` | Move to the next monitor |
| `H` | Show or hide the help panel |
| `Esc` / `Q` | Quit |

Pressing a mode key during a routine ends the routine and switches to manual control.

**Warning: the noise, cycle, bars and checkerboard modes flash or are high-contrast.** Don't watch them if you are photosensitive.

### Routines (`-program`)

| Name | Each round | Default 3 rounds |
| --- | --- | --- |
| `standard` | 50 min noise + 10 min white | 3 h |
| `gentle` | 25 min slow color cycle + 5 min white | 1.5 h |
| `quick` | 20 min noise + 5 min white | 75 min |

When a routine finishes, the screen switches to mid-grey and beeps, so you can check the result. Press ↑/↓ to try other grey levels, then press Esc.

## LCD vs OLED

**LCD** covers IPS, VA and TN panels. Mini-LED is also an LCD. This laptop, the Legion 5 Pro 16ITH6H, has a CSOT IPS panel. On an LCD, ghosting is usually *image retention*: stuck liquid crystals and trapped charge. It is normally reversible, and it's what this tool is for.

1. Rest the panel first: screen off or lid closed overnight.
2. Run `-program standard` with the brightness boost left on.
3. Check on grey, and repeat on later days if needed.
4. Remove the cause: hide Chrome's side panel, auto-hide the taskbar, and use a short screen-off timeout.

**OLED** is used in the Legion 9i and some Legion Pro, Slim and Yoga models. True OLED burn-in is uneven wear of the organic subpixels. It is **permanent**, and flashing noise or colors won't repair it. Bright full-screen white only ages the whole panel faster. On OLED:

- For short-term ghosting (minutes to hours old), turn the screen off or show varied content.
- Let the panel's built-in compensation run. Leave the laptop asleep and plugged in, and turn on any OLED care or pixel-refresh option in Lenovo Vantage.
- Prevent it with dark mode, an auto-hidden taskbar and a short screen-off timeout.
- If burn-in survives all of that, the fix is a warranty claim or a panel replacement.
- Use only `-mode grey` or `-mode gradient` with `-brightness 0`. They're for inspecting the panel.

To check which panel you have, look up your model number on Lenovo PSREF (psref.lenovo.com). You can also run `burnout.exe -list` and search the panel ID shown.
