package main

import (
	"testing"
	"time"
)

func TestRoutineAt(t *testing.T) {
	r, err := newRoutine("standard", 2, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.steps) != 4 || r.total() != 2*time.Hour {
		t.Fatalf("got %d steps / %v total, want 4 / 2h", len(r.steps), r.total())
	}
	cases := []struct {
		elapsed time.Duration
		idx     int
		left    time.Duration
	}{
		{0, 0, 50 * time.Minute},
		{49 * time.Minute, 0, time.Minute},
		{50 * time.Minute, 1, 10 * time.Minute},
		{61 * time.Minute, 2, 49 * time.Minute},
		{119 * time.Minute, 3, time.Minute},
		{120 * time.Minute, 4, 0},
		{5 * time.Hour, 4, 0},
	}
	for _, c := range cases {
		idx, left := r.at(c.elapsed)
		if idx != c.idx || left != c.left {
			t.Errorf("at(%v) = %d, %v; want %d, %v", c.elapsed, idx, left, c.idx, c.left)
		}
	}
}

func TestUnknownProgram(t *testing.T) {
	if _, err := newRoutine("nope", 1, time.Time{}); err == nil {
		t.Fatal("expected error for unknown program")
	}
}
