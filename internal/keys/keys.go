package keys

import (
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	// PrefixTimeout is how long a lone g or d waits for its partner.
	PrefixTimeout = 800 * time.Millisecond
	// DigitTimeout is how long a typed digit waits to be joined into a longer number.
	DigitTimeout = 300 * time.Millisecond
	// TimeDigitTimeout is how long the first digit of a two-digit time entry waits.
	TimeDigitTimeout = time.Second
)

// EventKind tells the caller what a key press resolved to.
type EventKind int

const (
	// Nothing means the key started a sequence and was swallowed.
	Nothing EventKind = iota
	// Pass means the key belongs to no sequence; handle it as itself.
	Pass
	// GoTop is gg.
	GoTop
	// Delete is dd.
	Delete
	// Jump is a digit run; Target carries the number typed.
	Jump
)

// Event is the outcome of feeding a key to a Machine.
type Event struct {
	Kind   EventKind
	Key    tea.KeyMsg
	Target int
	// Wait is how long until the pending sequence expires, when there is one.
	Wait time.Duration
}

// Machine resolves the multi-key sequences of select mode: the g and d
// prefixes, and runs of digits that jump to an item number.
type Machine struct {
	prefix   rune
	prefixAt time.Time
	digits   string
	digitAt  time.Time
}

// Feed interprets one key press. A digit jumps immediately and keeps listening
// for a second digit to widen the target, so reaching item 12 costs no delay on
// the way to item 1.
func (m *Machine) Feed(k tea.KeyMsg, now time.Time) Event {
	m.expire(now)

	if r, ok := SingleRune(k); ok {
		switch {
		case r >= '0' && r <= '9':
			if m.digits == "" {
				m.digits = string(r)
			} else {
				m.digits += string(r)
			}
			m.digitAt = now
			target, _ := strconv.Atoi(m.digits)
			return Event{Kind: Jump, Target: target, Wait: DigitTimeout}

		case r == 'g' || r == 'd':
			if m.prefix == r {
				m.prefix = 0
				kind := GoTop
				if r == 'd' {
					kind = Delete
				}
				return Event{Kind: kind}
			}
			m.prefix, m.prefixAt = r, now
			return Event{Kind: Nothing, Wait: PrefixTimeout}
		}
	}

	m.prefix = 0
	return Event{Kind: Pass, Key: k}
}

// Pending is the remaining wait before the oldest unfinished sequence lapses,
// or zero when nothing is in flight. The caller uses it to schedule a tick.
func (m *Machine) Pending(now time.Time) time.Duration {
	m.expire(now)

	wait := time.Duration(0)
	if m.prefix != 0 {
		wait = PrefixTimeout - now.Sub(m.prefixAt)
	}
	if m.digits != "" {
		if remain := DigitTimeout - now.Sub(m.digitAt); wait == 0 || remain < wait {
			wait = remain
		}
	}
	if wait < 0 {
		return 0
	}
	return wait
}

// Prefix is the unfinished g or d, for showing in the status line.
func (m *Machine) Prefix(now time.Time) string {
	m.expire(now)
	if m.prefix == 0 {
		return ""
	}
	return string(m.prefix)
}

// Count is the digits typed so far, for showing in the status line.
func (m *Machine) Count(now time.Time) string {
	m.expire(now)
	return m.digits
}

// Reset drops any half-typed sequence, used when leaving select mode.
func (m *Machine) Reset() {
	m.prefix, m.digits = 0, ""
}

func (m *Machine) expire(now time.Time) {
	if m.prefix != 0 && now.Sub(m.prefixAt) > PrefixTimeout {
		m.prefix = 0
	}
	if m.digits != "" && now.Sub(m.digitAt) > DigitTimeout {
		m.digits = ""
	}
}

// SingleRune extracts the one character a key press typed, if it was one.
func SingleRune(k tea.KeyMsg) (rune, bool) {
	if k.Type != tea.KeyRunes || len(k.Runes) != 1 || k.Alt {
		return 0, false
	}
	return k.Runes[0], true
}

// TimeEntry collects the two digits of an hour or minute field.
type TimeEntry struct {
	first int
	at    time.Time
}

// Digit takes a typed 0-9. It reports the resolved value and whether entry is
// finished: a second digit completes immediately, a first one only shows a
// provisional value until Flush or the timeout lands.
func (t *TimeEntry) Digit(d int, now time.Time) (value int, done bool) {
	if t.first < 0 {
		t.first, t.at = d, now
		return d, false
	}
	value, t.first = t.first*10+d, -1
	return value, true
}

// Flush commits a lone first digit once its timeout has passed.
func (t *TimeEntry) Flush(now time.Time) (value int, ok bool) {
	if t.first < 0 || now.Sub(t.at) <= TimeDigitTimeout {
		return 0, false
	}
	value, t.first = t.first, -1
	return value, true
}

// Provisional is the half-typed digit, or -1.
func (t *TimeEntry) Provisional(now time.Time) int {
	if t.first < 0 || now.Sub(t.at) > TimeDigitTimeout {
		return -1
	}
	return t.first
}

// Pending is the wait left on a half-typed digit.
func (t *TimeEntry) Pending(now time.Time) time.Duration {
	if t.first < 0 {
		return 0
	}
	if remain := TimeDigitTimeout - now.Sub(t.at); remain > 0 {
		return remain
	}
	return 0
}

// Reset abandons a half-typed digit.
func (t *TimeEntry) Reset() { t.first = -1 }

// NewTimeEntry is an idle TimeEntry.
func NewTimeEntry() TimeEntry { return TimeEntry{first: -1} }
