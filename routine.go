package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type step struct {
	mode     mode
	d        time.Duration
	interval time.Duration // cycle-mode speed for this step; 0 keeps the current one
}

type program struct {
	about string
	round []step
}

var programs = map[string]program{
	"standard": {"50 min noise + 10 min white per round", []step{
		{mode: modeNoise, d: 50 * time.Minute},
		{mode: modeWhite, d: 10 * time.Minute},
	}},
	"gentle": {"25 min slow color cycle + 5 min white per round", []step{
		{mode: modeCycle, d: 25 * time.Minute, interval: 500 * time.Millisecond},
		{mode: modeWhite, d: 5 * time.Minute},
	}},
	"quick": {"20 min noise + 5 min white per round", []step{
		{mode: modeNoise, d: 20 * time.Minute},
		{mode: modeWhite, d: 5 * time.Minute},
	}},
}

func programList() string {
	names := make([]string, 0, len(programs))
	for n := range programs {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "      %-9s %s\n", n, programs[n].about)
	}
	return b.String()
}

// routine is a running program: its rounds expanded into one flat list of steps.
type routine struct {
	name     string
	steps    []step
	start    time.Time
	current  int
	finished bool
}

func newRoutine(name string, rounds int, start time.Time) (*routine, error) {
	p, ok := programs[name]
	if !ok {
		return nil, fmt.Errorf("unknown -program %q; choose one of:\n%s", name, programList())
	}
	r := &routine{name: name, start: start, current: -1}
	for range max(rounds, 1) {
		r.steps = append(r.steps, p.round...)
	}
	return r, nil
}

func (r *routine) total() time.Duration {
	var t time.Duration
	for _, s := range r.steps {
		t += s.d
	}
	return t
}

// at returns the index of the step active after elapsed time and how long is
// left in it. The index equals len(steps) once the routine has finished.
func (r *routine) at(elapsed time.Duration) (int, time.Duration) {
	for i, s := range r.steps {
		if elapsed < s.d {
			return i, s.d - elapsed
		}
		elapsed -= s.d
	}
	return len(r.steps), 0
}
