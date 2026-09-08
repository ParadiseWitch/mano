package store

import (
	"reflect"
	"testing"
	"time"
)

func timePtr(h, m int) *Time {
	t := Time{Hour: h, Minute: m}
	return &t
}

func TestParseSpecExample(t *testing.T) {
	input := `# 20260801

1. 日志事项1内容

- START: 09:00
- END: 10:21

2. 日志事项2内容

- START: 10:30
- END: 11:21
`

	got := Parse([]byte(input))
	want := Journal{Days: []Day{{
		Date: "2026-08-01",
		Items: []Item{
			{Content: "日志事项1内容", Start: timePtr(9, 0), End: timePtr(10, 21)},
			{Content: "日志事项2内容", Start: timePtr(10, 30), End: timePtr(11, 21)},
		},
	}}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseToleratesVariants(t *testing.T) {
	input := "# 2026-08-01\r\n" +
		"\r\n" +
		"7.带空格的编号也被接受\r\n" +
		"\r\n" +
		"- start:9:05\r\n" +
		"- END : 17:45\r\n" +
		"这一行不属于格式，应当被丢弃\r\n" +
		"\r\n" +
		"8. 只有内容没有时间\r\n"

	got := Parse([]byte(input))
	want := Journal{Days: []Day{{
		Date: "2026-08-01",
		Items: []Item{
			{Content: "带空格的编号也被接受", Start: timePtr(9, 5), End: timePtr(17, 45)},
			{Content: "只有内容没有时间"},
		},
	}}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseRejectsImpossibleValues(t *testing.T) {
	input := `# 20261340

1. 日期不存在的段落整体忽略

# 20260801

- START: 09:00

1. 时间字段出现在条目之前，忽略

2. 非法时间

- START: 25:00
- END: 10:99
`

	got := Parse([]byte(input))
	want := Journal{Days: []Day{{
		Date: "2026-08-01",
		Items: []Item{
			{Content: "时间字段出现在条目之前，忽略"},
			{Content: "非法时间"},
		},
	}}}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestParseSortsAscendingAndMergesDuplicates(t *testing.T) {
	input := `# 20260803

1. 三天

# 20260801

1. 一天

# 20260803

2. 三天续

# 20260802

1. 两天
`

	got := Parse([]byte(input))

	var dates []string
	for _, d := range got.Days {
		dates = append(dates, d.Date)
	}
	if want := []string{"2026-08-01", "2026-08-02", "2026-08-03"}; !reflect.DeepEqual(dates, want) {
		t.Fatalf("dates = %v, want %v", dates, want)
	}

	merged := got.Days[2].Items
	if len(merged) != 2 || merged[0].Content != "三天" || merged[1].Content != "三天续" {
		t.Errorf("duplicate day not merged in order: %+v", merged)
	}
}

func TestParseEmpty(t *testing.T) {
	if got := Parse(nil); len(got.Days) != 0 {
		t.Errorf("Parse(nil) = %+v, want empty journal", got)
	}
	if got := Parse([]byte("随手写的散文\n没有日期头\n")); len(got.Days) != 0 {
		t.Errorf("Parse of headerless text = %+v, want empty journal", got)
	}
}

func TestSerializeMatchesSpecLayout(t *testing.T) {
	j := Journal{Days: []Day{{
		Date: "2026-08-01",
		Items: []Item{
			{Content: "日志事项1内容", Start: timePtr(9, 0), End: timePtr(10, 21)},
			{Content: "日志事项2内容", Start: timePtr(10, 30), End: timePtr(11, 21)},
		},
	}}}

	want := `# 20260801

1. 日志事项1内容

- START: 09:00
- END: 10:21

2. 日志事项2内容

- START: 10:30
- END: 11:21
`

	if got := string(j.Serialize()); got != want {
		t.Errorf("Serialize mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSerializeOmitsMissingFields(t *testing.T) {
	j := Journal{Days: []Day{
		{Date: "2026-08-01", Items: []Item{
			{Content: "只有开始", Start: timePtr(9, 0)},
			{Content: "只有结束", End: timePtr(18, 30)},
			{Content: "都没有"},
		}},
		{Date: "2026-08-02"},
	}}

	want := `# 20260801

1. 只有开始

- START: 09:00

2. 只有结束

- END: 18:30

3. 都没有

# 20260802
`

	if got := string(j.Serialize()); got != want {
		t.Errorf("Serialize mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRoundTripIsStable(t *testing.T) {
	inputs := []string{
		`# 20260801

1. 日志事项1内容

- START: 09:00
- END: 10:21

2. 日志事项2内容

- START: 10:30
- END: 11:21
`,
		`# 20260801

1. 只有开始

- START: 09:00

2. 只有结束

- END: 18:30

3. 都没有

# 20260802
`,
		`# 20260101

1. 跨年

- START: 23:00
- END: 01:30
`,
	}

	for _, input := range inputs {
		once := Parse([]byte(input))
		twice := Parse(once.Serialize())

		if !reflect.DeepEqual(once, twice) {
			t.Fatalf("round trip changed the model\nfirst:  %+v\nsecond: %+v", once, twice)
		}
		if got, want := string(once.Serialize()), string(twice.Serialize()); got != want {
			t.Errorf("round trip changed the bytes\nfirst:\n%s\nsecond:\n%s", got, want)
		}
	}
}

func TestDurationCrossesMidnight(t *testing.T) {
	cases := []struct {
		name       string
		start, end *Time
		want       time.Duration
		crossed    bool
		shown      bool
	}{
		{"same day", timePtr(9, 0), timePtr(10, 10), 70 * time.Minute, false, true},
		{"crosses midnight", timePtr(23, 0), timePtr(1, 0), 2 * time.Hour, true, true},
		{"zero length", timePtr(9, 0), timePtr(9, 0), 0, false, true},
		{"no start", nil, timePtr(9, 0), 0, false, false},
		{"no end", timePtr(9, 0), nil, 0, false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			item := Item{Start: c.start, End: c.end}
			got, crossed := item.Duration()

			if c.shown {
				if got != c.want || crossed != c.crossed {
					t.Errorf("Duration() = %v, %v; want %v, %v", got, crossed, c.want, c.crossed)
				}
			} else if got != 0 || crossed {
				t.Errorf("Duration() = %v, %v; want 0, false", got, crossed)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                            "00h00m",
		70 * time.Minute:             "01h10m",
		10 * time.Minute:             "00h10m",
		26*time.Hour + 5*time.Minute: "26h05m",
		2*time.Hour + 59*time.Minute: "02h59m",
	}
	for in, want := range cases {
		if got := FormatDuration(in); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestDayTotals(t *testing.T) {
	day := Day{Date: "2026-08-01", Items: []Item{
		{Content: "a", Start: timePtr(9, 0), End: timePtr(10, 0)},
		{Content: "b", Start: timePtr(23, 0), End: timePtr(1, 0)},
		{Content: "c", Start: timePtr(14, 0)},
	}}

	if got, want := day.TotalDuration(), 3*time.Hour; got != want {
		t.Errorf("TotalDuration() = %v, want %v", got, want)
	}
	if got, want := day.Weekday(), "周六"; got != want {
		t.Errorf("Weekday() = %q, want %q", got, want)
	}
	if got, want := day.CompactDate(), "20260801"; got != want {
		t.Errorf("CompactDate() = %q, want %q", got, want)
	}
}

func TestJournalEnsureDayKeepsOrder(t *testing.T) {
	var j Journal

	for _, date := range []string{"2026-08-05", "2026-08-01", "2026-08-03", "2026-08-01"} {
		j.EnsureDay(date)
	}

	var dates []string
	for _, d := range j.Days {
		dates = append(dates, d.Date)
	}
	want := []string{"2026-08-01", "2026-08-03", "2026-08-05"}
	if !reflect.DeepEqual(dates, want) {
		t.Errorf("days = %v, want %v", dates, want)
	}

	if got := j.FindDay("2026-08-03"); got != 1 {
		t.Errorf("FindDay(2026-08-03) = %d, want 1", got)
	}
	if got := j.FindDay("2026-09-09"); got != -1 {
		t.Errorf("FindDay(absent) = %d, want -1", got)
	}
}

func TestParseDate(t *testing.T) {
	valid := map[string]string{
		"20260801":   "2026-08-01",
		"2026-08-01": "2026-08-01",
	}
	for in, want := range valid {
		got, ok := ParseDate(in)
		if !ok || got != want {
			t.Errorf("ParseDate(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
	for _, in := range []string{"2026-0801", "20261301", "not-a-date", "", "2026-02-30"} {
		if got, ok := ParseDate(in); ok {
			t.Errorf("ParseDate(%q) = %q, true; want false", in, got)
		}
	}
}

func TestSaveAndReload(t *testing.T) {
	path := t.TempDir() + "/nested/mano.md"

	saved := &Store{Path: path}
	saved.Journal.EnsureDay("2026-08-02")
	day := saved.Day("2026-08-01")
	day.Items = append(day.Items, Item{Content: "写入再读回", Start: timePtr(9, 0), End: timePtr(9, 45)})

	if err := saved.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	reloaded, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !reflect.DeepEqual(reloaded.Journal, saved.Journal) {
		t.Errorf("reloaded journal differs\n got: %+v\nwant: %+v", reloaded.Journal, saved.Journal)
	}
}

func TestOpenMissingFileYieldsEmptyJournal(t *testing.T) {
	s, err := Open(t.TempDir() + "/does-not-exist.md")
	if err != nil {
		t.Fatalf("Open on missing file: %v", err)
	}
	if len(s.Journal.Days) != 0 {
		t.Errorf("journal = %+v, want empty", s.Journal)
	}
}
