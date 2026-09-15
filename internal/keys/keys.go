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
// the way to item 1. A leading 0 is left alone: no item is number 0, and the UI
// wants that key for itself.
func (m *Machine) Feed(k tea.KeyMsg, now time.Time) Event {
	m.expire(now)

	if r, ok := SingleRune(k); ok {
		switch {
		case r >= '1' && r <= '9', r == '0' && m.digits != "":
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
