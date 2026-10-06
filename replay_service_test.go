package main

import (
	"testing"
	"time"
)

func TestEventDecision(t *testing.T) {
	l := newLedger(time.Minute)
	cases := []struct {
		event    event
		accepted bool
		invalid  bool
	}{
		{event{"build-42", "build"}, true, false},
		{event{"build-42", "build"}, false, false},
		{event{"release-7", "release"}, true, false},
		{event{"diag-9", "diagnostic"}, true, false},
		{event{"other-1", "unknown"}, false, true},
	}
	for _, tc := range cases {
		got, err := l.accept(tc.event)
		if got != tc.accepted || (err != nil) != tc.invalid {
			t.Fatalf("%+v: accepted=%v err=%v", tc.event, got, err)
		}
	}
	if l.counts["build"] != 1 || l.counts["release"] != 1 || l.counts["diagnostic"] != 1 {
		t.Fatalf("counts: %+v", l.counts)
	}
}

func TestRedriveInterval(t *testing.T) {
	l := newLedger(time.Minute)
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }
	for _, tc := range []struct {
		queue   string
		advance time.Duration
		allowed bool
	}{
		{"events", 0, true}, {"events", 0, false}, {"other", 0, true}, {"events", time.Minute, true},
	} {
		now = now.Add(tc.advance)
		if got := l.reserve(tc.queue); got != tc.allowed {
			t.Fatalf("queue %s: got %v, want %v", tc.queue, got, tc.allowed)
		}
	}
}
