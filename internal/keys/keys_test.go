package keys

import (
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

var t0 = time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)

func char(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func special(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func TestPrefixPairs(t *testing.T) {
	cases := []struct {
		name string
		keys []rune
		want []EventKind
	}{
		{"gg jumps to top", []rune{'g', 'g'}, []EventKind{Nothing, GoTop}},
		{"dd deletes", []rune{'d', 'd'}, []EventKind{Nothing, Delete}},
		{"gd is not a pair", []rune{'g', 'd'}, []EventKind{Nothing, Nothing}},
		{"g then j passes j through", []rune{'g', 'j'}, []EventKind{Nothing, Pass}},
		{"three g's restart the pair", []rune{'g', 'g', 'g'}, []EventKind{Nothing, GoTop, Nothing}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m Machine
			for i, r := range c.keys {
				got := m.Feed(char(r), t0.Add(time.Duration(i)*10*time.Millisecond))
				if got.Kind != c.want[i] {
					t.Errorf("Feed(%q) at step %d = %v, want %v", r, i, got.Kind, c.want[i])
				}
			}
		})
	}
}

func TestPrefixExpires(t *testing.T) {
	var m Machine

	if got := m.Feed(char('g'), t0); got.Kind != Nothing || got.Wait != PrefixTimeout {
		t.Fatalf("first g = %+v, want Nothing with a %v wait", got, PrefixTimeout)
	}
	if got := m.Pending(t0.Add(100 * time.Millisecond)); got <= 0 {
		t.Errorf("Pending while waiting = %v, want positive", got)
	}
	if got := m.Prefix(t0.Add(100 * time.Millisecond)); got != "g" {
		t.Errorf("Prefix = %q, want %q", got, "g")
	}

	// Past the timeout the lone g is forgotten and the next key stands alone.
	got := m.Feed(char('g'), t0.Add(PrefixTimeout+time.Millisecond))
	if got.Kind != Nothing {
		t.Errorf("stale g followed by g = %v, want Nothing (a fresh prefix)", got.Kind)
	}
	if p := m.Prefix(t0.Add(PrefixTimeout + time.Millisecond)); p != "g" {
		t.Errorf("Prefix after restart = %q, want %q", p, "g")
	}

	m.Reset()
	if got := m.Feed(char('j'), t0.Add(2*PrefixTimeout)); got.Kind != Pass {
		t.Errorf("j after expiry = %v, want Pass", got.Kind)
	}
	if got := m.Pending(t0.Add(3 * PrefixTimeout)); got != 0 {
		t.Errorf("Pending after expiry = %v, want 0", got)
	}
}

func TestDigitsConcatenateWithinWindow(t *testing.T) {
	var m Machine

	steps := []struct {
		at     time.Duration
		key    rune
		target int
	}{
		{0, '1', 1},
		{100 * time.Millisecond, '2', 12},
		{200 * time.Millisecond, '3', 123},
	}

	for _, s := range steps {
		got := m.Feed(char(s.key), t0.Add(s.at))
		if got.Kind != Jump || got.Target != s.target {
			t.Errorf("Feed(%q) at %v = %+v, want Jump %d", s.key, s.at, got, s.target)
		}
	}

	if got := m.Count(t0.Add(250 * time.Millisecond)); got != "123" {
		t.Errorf("Count = %q, want %q", got, "123")
	}
}

func TestDigitsRestartAfterWindow(t *testing.T) {
	var m Machine

	if got := m.Feed(char('1'), t0); got.Target != 1 {
		t.Fatalf("first digit target = %d, want 1", got.Target)
	}
	got := m.Feed(char('2'), t0.Add(DigitTimeout+time.Millisecond))
	if got.Kind != Jump || got.Target != 2 {
		t.Errorf("digit after window = %+v, want Jump 2", got)
	}
	if c := m.Count(t0.Add(DigitTimeout + 2*time.Millisecond)); c != "2" {
		t.Errorf("Count = %q, want %q", c, "2")
	}
}

func TestZeroLeadingDigits(t *testing.T) {
	var m Machine

	if got := m.Feed(char('0'), t0); got.Kind != Jump || got.Target != 0 {
		t.Errorf("lone 0 = %+v, want Jump 0 for the caller to ignore", got)
	}
	if got := m.Feed(char('5'), t0.Add(50*time.Millisecond)); got.Target != 5 {
		t.Errorf("05 = %+v, want Jump 5", got)
	}
}

func TestNonSequenceKeysPassThrough(t *testing.T) {
	var m Machine

	passed := []tea.KeyMsg{
		char('G'), char('i'), char('q'), char('?'), char('j'), char('k'),
		special(tea.KeyUp), special(tea.KeyDown), special(tea.KeyEnter), special(tea.KeyEsc),
	}
	for _, k := range passed {
		if got := m.Feed(k, t0); got.Kind != Pass || !reflect.DeepEqual(got.Key, k) {
			t.Errorf("Feed(%v) = %+v, want Pass with the same key", k, got)
		}
	}

	alt := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}, Alt: true}
	if got := m.Feed(alt, t0); got.Kind != Pass {
		t.Errorf("alt+g = %+v, want Pass", got)
	}
}

func TestResetClearsBothSequences(t *testing.T) {
	var m Machine

	m.Feed(char('g'), t0)
	m.Feed(char('7'), t0)
	m.Reset()

	if got := m.Pending(t0.Add(time.Millisecond)); got != 0 {
		t.Errorf("Pending after Reset = %v, want 0", got)
	}
	if got := m.Prefix(t0.Add(time.Millisecond)); got != "" {
		t.Errorf("Prefix after Reset = %q, want empty", got)
	}
	if got := m.Count(t0.Add(time.Millisecond)); got != "" {
		t.Errorf("Count after Reset = %q, want empty", got)
	}
	if got := m.Feed(char('g'), t0.Add(2*time.Millisecond)); got.Kind != Nothing {
		t.Errorf("g after Reset = %v, want Nothing", got.Kind)
	}
}

func TestTimeEntryTwoDigits(t *testing.T) {
	e := NewTimeEntry()

	if v, done := e.Digit(1, t0); v != 1 || done {
		t.Errorf("first digit = %d, %v; want 1, false", v, done)
	}
	if p := e.Provisional(t0.Add(100 * time.Millisecond)); p != 1 {
		t.Errorf("Provisional = %d, want 1", p)
	}
	if w := e.Pending(t0.Add(100 * time.Millisecond)); w != TimeDigitTimeout-100*time.Millisecond {
		t.Errorf("Pending = %v, want %v", w, TimeDigitTimeout-100*time.Millisecond)
	}
	if v, done := e.Digit(5, t0.Add(200*time.Millisecond)); v != 15 || !done {
		t.Errorf("second digit = %d, %v; want 15, true", v, done)
	}
	if p := e.Provisional(t0.Add(300 * time.Millisecond)); p != -1 {
		t.Errorf("Provisional after completion = %d, want -1", p)
	}
}

func TestTimeEntryFlushesLoneDigit(t *testing.T) {
	e := NewTimeEntry()
	e.Digit(9, t0)

	if v, ok := e.Flush(t0.Add(TimeDigitTimeout / 2)); ok {
		t.Errorf("Flush inside the window = %d, %v; want not ok", v, ok)
	}
	if v, ok := e.Flush(t0.Add(TimeDigitTimeout + time.Millisecond)); v != 9 || !ok {
		t.Errorf("Flush after the window = %d, %v; want 9, true", v, ok)
	}
	if p := e.Provisional(t0.Add(TimeDigitTimeout + 2*time.Millisecond)); p != -1 {
		t.Errorf("Provisional after expiry = %d, want -1", p)
	}
	if v, ok := e.Flush(t0.Add(2 * TimeDigitTimeout)); ok {
		t.Errorf("second Flush = %d, %v; want not ok", v, ok)
	}
}

func TestTimeEntryResets(t *testing.T) {
	e := NewTimeEntry()
	e.Digit(2, t0)
	e.Reset()

	if p := e.Provisional(t0.Add(time.Millisecond)); p != -1 {
		t.Errorf("Provisional after Reset = %d, want -1", p)
	}
	if w := e.Pending(t0.Add(time.Millisecond)); w != 0 {
		t.Errorf("Pending after Reset = %v, want 0", w)
	}
	if v, done := e.Digit(3, t0.Add(2*time.Millisecond)); v != 3 || done {
		t.Errorf("digit after Reset = %d, %v; want 3, false", v, done)
	}
}

func TestSingleRune(t *testing.T) {
	if r, ok := SingleRune(char('x')); !ok || r != 'x' {
		t.Errorf("SingleRune(x) = %q, %v; want x, true", r, ok)
	}
	if _, ok := SingleRune(special(tea.KeyEnter)); ok {
		t.Error("SingleRune(enter) = ok, want not ok")
	}
	if _, ok := SingleRune(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a', 'b'}}); ok {
		t.Error("SingleRune(two runes) = ok, want not ok")
	}
	if _, ok := SingleRune(tea.KeyMsg{Type: tea.KeyRunes}); ok {
		t.Error("SingleRune(no runes) = ok, want not ok")
	}
}
