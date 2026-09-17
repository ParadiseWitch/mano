package store

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	dateHeaderRe = regexp.MustCompile(`^\* ([0-9]{4}-[0-9]{2}-[0-9]{2})$`)
	itemRe       = regexp.MustCompile(`^\*\* (.*)$`)
	fieldRe      = regexp.MustCompile(`(?i)^   - (START|END)\s*:\s*([0-9]{1,2}:[0-9]{2})\s*$`)
)

// ParseDate normalises a compact or dashed date to DateLayout.
func ParseDate(s string) (string, bool) {
	for _, layout := range []string{HeaderDateLayout, DateLayout} {
		if parsed, err := time.Parse(layout, s); err == nil {
			return parsed.Format(DateLayout), true
		}
	}
	return "", false
}

// Parse reads org-mode into a journal. Days come back in ascending date order
// and repeated date headers are merged. Lines the format does not describe are
// dropped, since orgmaid owns the file outright.
func Parse(data []byte) Journal {
	var j Journal
	dayIdx, itemIdx := -1, -1

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimRight(raw, "\r")

		if m := dateHeaderRe.FindStringSubmatch(line); m != nil {
			if _, ok := ParseDate(m[1]); !ok {
				dayIdx, itemIdx = -1, -1
				continue
			}
			dayIdx, itemIdx = j.appendDay(m[1]), -1
			continue
		}

		if dayIdx < 0 {
			continue
		}

		if m := itemRe.FindStringSubmatch(line); m != nil {
			j.Days[dayIdx].Items = append(j.Days[dayIdx].Items, Item{Content: m[1]})
			itemIdx = len(j.Days[dayIdx].Items) - 1
			continue
		}

		if m := fieldRe.FindStringSubmatch(line); m != nil && itemIdx >= 0 {
			t, ok := ParseTime(m[2])
			if !ok || !t.Valid() {
				continue
			}
			item := &j.Days[dayIdx].Items[itemIdx]
			if strings.EqualFold(m[1], "START") {
				item.Start = &t
			} else {
				item.End = &t
			}
		}
	}

	j.mergeDuplicateDays()
	return j
}

// appendDay returns the index of the day a header line belongs to. Days arrive
// in file order, so a header that continues the run is reused or appended; one
// that jumps backwards defers to mergeDuplicateDays.
func (j *Journal) appendDay(date string) int {
	if n := len(j.Days); n > 0 && j.Days[n-1].Date == date {
		return n - 1
	}
	j.Days = append(j.Days, Day{Date: date})
	return len(j.Days) - 1
}

func (j *Journal) mergeDuplicateDays() {
	sort.SliceStable(j.Days, func(a, b int) bool { return j.Days[a].Date < j.Days[b].Date })

	merged := j.Days[:0]
	for _, day := range j.Days {
		if n := len(merged); n > 0 && merged[n-1].Date == day.Date {
			merged[n-1].Items = append(merged[n-1].Items, day.Items...)
			continue
		}
		merged = append(merged, day)
	}
	j.Days = merged
}
