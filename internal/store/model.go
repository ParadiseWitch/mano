package store

import (
	"fmt"
	"sort"
	"time"
)

// DateLayout is the canonical in-memory date form.
const DateLayout = "2006-01-02"

// HeaderDateLayout is the compact form used in markdown headers.
const HeaderDateLayout = "20060102"

// Time is a wall clock reading with no date attached.
type Time struct {
	Hour   int
	Minute int
}

func (t Time) String() string { return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute) }

func (t Time) minutes() int { return t.Hour*60 + t.Minute }

// Valid reports whether t is a real clock reading.
func (t Time) Valid() bool {
	return t.Hour >= 0 && t.Hour <= 23 && t.Minute >= 0 && t.Minute <= 59
}

// ParseTime reads a HH:MM string.
func ParseTime(s string) (Time, bool) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return Time{}, false
	}
	return Time{Hour: t.Hour(), Minute: t.Minute()}, true
}

// NowTime is the current local clock reading, truncated to the minute.
func NowTime() Time {
	n := time.Now()
	return Time{Hour: n.Hour(), Minute: n.Minute()}
}

// Today is the current local date in canonical form.
func Today() string { return time.Now().Format(DateLayout) }

// Item is one logged entry. Start and End are nil when unset.
// Todo holds the TODO keyword ("TODO", "DONE", or "" for none).
// Tags holds zero or more org-mode tags.
type Item struct {
	Content string
	Start   *Time
	End     *Time
	Todo    string
	Tags    []string
}

// Duration is the elapsed span of the item, and whether it crosses midnight.
// An End earlier than Start is read as spanning into the next day. It reports
// false when either bound is missing.
func (it Item) Duration() (time.Duration, bool) {
	if it.Start == nil || it.End == nil {
		return 0, false
	}
	start, end := it.Start.minutes(), it.End.minutes()
	crossed := end < start
	if crossed {
		end += 24 * 60
	}
	return time.Duration(end-start) * time.Minute, crossed
}

// AddDuration sets the item's end so it spans the given minutes from its start.
// The span wraps within a day, so a duration measured past midnight lands on the
// clock reading it would really show. It reports false when the item has no start
// time to measure from.
func (it *Item) AddDuration(minutes int) bool {
	if it.Start == nil {
		return false
	}
	if minutes < 0 {
		minutes = 0
	}
	clock := (it.Start.minutes() + minutes) % (24 * 60)
	end := Time{Hour: clock / 60, Minute: clock % 60}
	it.End = &end
	return true
}

// Clone is a deep copy: the time pointers are duplicated so editing one item
// cannot reach through into another. Tags slice is also copied.
func (it Item) Clone() Item {
	out := it
	if it.Start != nil {
		start := *it.Start
		out.Start = &start
	}
	if it.End != nil {
		end := *it.End
		out.End = &end
	}
	if it.Tags != nil {
		out.Tags = make([]string, len(it.Tags))
		copy(out.Tags, it.Tags)
	}
	return out
}

// FormatDuration renders d as 01h10m.
func FormatDuration(d time.Duration) string {
	total := int(d / time.Minute)
	return fmt.Sprintf("%02dh%02dm", total/60, total%60)
}

// Day is every item logged under one date.
type Day struct {
	Date  string
	Items []Item
}

// TotalDuration sums the items that have both bounds set.
func (d Day) TotalDuration() time.Duration {
	var total time.Duration
	for _, it := range d.Items {
		if dur, _ := it.Duration(); dur > 0 {
			total += dur
		}
	}
	return total
}

// Journal is the whole file: days held in ascending date order.
type Journal struct {
	Days []Day
}

// insertPoint is the index at which date belongs, whether or not it is present.
func (j *Journal) insertPoint(date string) int {
	return sort.Search(len(j.Days), func(i int) bool { return j.Days[i].Date >= date })
}

// FindDay returns the index of date, or -1 when the journal has no such day.
func (j *Journal) FindDay(date string) int {
	i := j.insertPoint(date)
	if i < len(j.Days) && j.Days[i].Date == date {
		return i
	}
	return -1
}

// EnsureDay returns the index of date, creating an empty day when absent.
func (j *Journal) EnsureDay(date string) int {
	i := j.insertPoint(date)
	if i < len(j.Days) && j.Days[i].Date == date {
		return i
	}
	j.Days = append(j.Days, Day{})
	copy(j.Days[i+1:], j.Days[i:])
	j.Days[i] = Day{Date: date}
	return i
}

// Weekday is the Chinese weekday name for a canonical date, or empty when the
// date does not parse.
func Weekday(date string) string {
	parsed, err := time.Parse(DateLayout, date)
	if err != nil {
		return ""
	}
	names := [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[parsed.Weekday()]
}

// Weekday is the Chinese weekday name for the day's date.
func (d Day) Weekday() string { return Weekday(d.Date) }

// CompactDate is the date rendered for a markdown header.
func (d Day) CompactDate() string {
	parsed, err := time.Parse(DateLayout, d.Date)
	if err != nil {
		return d.Date
	}
	return parsed.Format(HeaderDateLayout)
}
