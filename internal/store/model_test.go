package store

import (
	"testing"
	"time"
)

func TestAddDurationWritesTheEndTime(t *testing.T) {
	cases := []struct {
		name        string
		start       string
		minutes     int
		wantEnd     string
		wantDur     time.Duration
		wantCrossed bool
	}{
		{"a plain span", "09:00", 70, "10:10", 70 * time.Minute, false},
		{"no span at all", "09:00", 0, "09:00", 0, false},
		{"up to midnight", "23:00", 60, "00:00", time.Hour, true},
		{"past midnight", "23:00", 120, "01:00", 2 * time.Hour, true},
		{"a whole day lands back on the start", "09:00", 24 * 60, "09:00", 0, false},
		{"longer than a day wraps within it", "09:00", 1505, "10:05", 65 * time.Minute, false},
		{"a negative span is no span", "09:00", -30, "09:00", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			item := Item{Content: "记录", Start: mustTime(t, c.start)}

			if !item.AddDuration(c.minutes) {
				t.Fatalf("AddDuration(%d) = false, want true", c.minutes)
			}
			if item.End == nil {
				t.Fatal("no end time was written")
			}
			if got := item.End.String(); got != c.wantEnd {
				t.Errorf("end = %s, want %s", got, c.wantEnd)
			}
			dur, crossed := item.Duration()
			if dur != c.wantDur || crossed != c.wantCrossed {
				t.Errorf("Duration() = %v, %v; want %v, %v", dur, crossed, c.wantDur, c.wantCrossed)
			}
			if item.Start.String() != c.start {
				t.Errorf("the start moved to %s, want %s", item.Start, c.start)
			}
			if item.Content != "记录" {
				t.Errorf("content = %q, want it left alone", item.Content)
			}
			if item.Start == item.End {
				t.Error("the new end time shares its pointer with the start time")
			}
		})
	}
}

func TestAddDurationWithoutAStartTime(t *testing.T) {
	item := Item{Content: "还没开始", End: mustTime(t, "18:00")}

	if item.AddDuration(60) {
		t.Error("AddDuration(60) = true, want false when there is no start to measure from")
	}
	if got := item.End.String(); got != "18:00" {
		t.Errorf("the end time was rewritten to %s, want the row left alone", got)
	}

	// An end time is not invented either, so the row still reads as no span.
	var bare Item
	if bare.AddDuration(60) {
		t.Error("AddDuration(60) = true on an empty item, want false")
	}
	if bare.End != nil {
		t.Errorf("end = %s, want it left unset", bare.End)
	}
}

func mustTime(t *testing.T, s string) *Time {
	t.Helper()

	tm, ok := ParseTime(s)
	if !ok {
		t.Fatalf("ParseTime(%q) = false, want a valid reading", s)
	}
	return &tm
}
