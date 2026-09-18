package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"orgmaid/internal/keys"
	"orgmaid/internal/store"
)

type dateState struct {
	dates     []string // canonical, descending: today first
	counts    map[string]int
	matches   []int // indices into dates, narrowed by the search query
	cursor    int
	offset    int
	searching bool
	search    textinput.Model

	// Calendar view
	calMode   bool   // true = calendar view, false = list view
	calYear   int    // displayed year
	calMonth  int    // displayed month (1-12)
	calCursor string // selected date in calendar mode
}

func newDateState() dateState {
	search := textinput.New()
	search.Prompt = ""
	return dateState{counts: map[string]int{}, search: search, calMode: true}
}

// reset rebuilds the list from the journal: every day that has a header, plus
// today so the current date is always reachable.
func (d *dateState) reset(j store.Journal, current string) {
	d.counts = make(map[string]int, len(j.Days)+1)
	for _, day := range j.Days {
		d.counts[day.Date] = len(day.Items)
	}
	today := store.Today()
	if _, ok := d.counts[today]; !ok {
		d.counts[today] = 0
	}

	d.dates = make([]string, 0, len(d.counts))
	for date := range d.counts {
		d.dates = append(d.dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(d.dates)))

	d.matches = make([]int, len(d.dates))
	for i := range d.matches {
		d.matches[i] = i
	}

	d.searching = false
	d.search.Blur()
	d.search.SetValue("")
	d.offset = 0
	d.cursor = 0
	for i, date := range d.dates {
		if date == current {
			d.cursor = i
			break
		}
	}

	// Initialize calendar to current date's month
	if current != "" {
		d.calYear, d.calMonth, _ = parseDateParts(current)
	} else {
		d.calYear, d.calMonth, _ = parseDateParts(today)
	}
	d.calCursor = current
	if d.calCursor == "" {
		d.calCursor = today
	}
}

func (a *App) updateDates(k tea.KeyMsg) tea.Cmd {
	a.status = ""
	d := &a.dates

	if d.searching {
		return a.updateDateSearch(k)
	}

	// Calendar mode navigation
	if d.calMode {
		if r, ok := keys.SingleRune(k); ok {
			switch r {
			case 'h':
				d.moveCalendar(-1)
				return nil
			case 'l':
				d.moveCalendar(1)
				return nil
			case 'j':
				d.moveCalendar(7)
				return nil
			case 'k':
				d.moveCalendar(-7)
				return nil
			case 'H':
				d.moveCalendarMonth(-1)
				return nil
			case 'L':
				d.moveCalendarMonth(1)
				return nil
			case 'K':
				d.moveCalendarMonth(-1)
				return nil
			case 'J':
				d.moveCalendarMonth(1)
				return nil
			case 's':
				// Jump to today
				today := store.Today()
				d.calCursor = today
				d.calYear, d.calMonth, _ = parseDateParts(today)
				return nil
			case 'q':
				return tea.Quit
			case '?':
				a.page = pageHelp
				a.helpFrom = helpFromDate
				return nil
			case 'c':
				a.enterDay(d.calCursor)
				return nil
			}
		}
		switch k.Type {
		case tea.KeyTab:
			d.calMode = false
			return nil
		case tea.KeyEnter:
			a.enterDay(d.calCursor)
			return nil
		case tea.KeyEsc:
			a.enterDay(a.date)
			return nil
		case tea.KeyCtrlC:
			return tea.Quit
		}
		return nil
	}

	// List mode navigation
	if r, ok := keys.SingleRune(k); ok {
		switch r {
		case 'j':
			a.moveDate(1)
			return nil
		case 'k':
			a.moveDate(-1)
			return nil
		case 'h':
			a.pageDate(-1)
			return nil
		case 'l':
			a.pageDate(1)
			return nil
		case '/':
			return a.beginSearch()
		case 'q':
			return tea.Quit
		case '?':
			a.page = pageHelp
			a.helpFrom = helpFromDate
			return nil
		case 'c':
			a.enterDay(a.date)
			return nil
		}
	}

	switch k.Type {
	case tea.KeyTab:
		d.calMode = true
		return nil
	case tea.KeyDown:
		a.moveDate(1)
	case tea.KeyUp:
		a.moveDate(-1)
	case tea.KeyRight, tea.KeyPgDown:
		a.pageDate(1)
	case tea.KeyLeft, tea.KeyPgUp:
		a.pageDate(-1)
	case tea.KeyEnter:
		return a.openSelectedDate("")
	case tea.KeyEsc:
		a.enterDay(a.date)
	case tea.KeyHome:
		a.moveDate(-len(d.matches))
	case tea.KeyEnd:
		a.moveDate(len(d.matches))
	case tea.KeyCtrlC:
		return tea.Quit
	}
	return nil
}

func (a *App) updateDateSearch(k tea.KeyMsg) tea.Cmd {
	d := &a.dates

	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEsc:
		d.searching = false
		d.search.Blur()
		d.search.SetValue("")
		a.refilterDates()
		return nil
	case tea.KeyEnter:
		query := d.search.Value()
		d.searching = false
		d.search.Blur()
		return a.openSelectedDate(query)
	}

	var cmd tea.Cmd
	d.search, cmd = d.search.Update(k)
	a.refilterDates()
	return cmd
}

func (a *App) beginSearch() tea.Cmd {
	a.dates.searching = true
	a.dates.search.Width = a.searchWidth()
	return a.dates.search.Focus()
}

func (a *App) openSelectedDate(query string) tea.Cmd {
	d := &a.dates

	// A query that spells a date the journal has never seen offers to start it.
	if missing := d.newDate(query); missing != "" {
		a.enterDay(missing)
		return nil
	}
	if len(d.matches) == 0 {
		a.status = "没有匹配的日期"
		return nil
	}

	a.enterDay(d.dates[d.matches[d.cursor]])
	return nil
}

// newDate is the date the query spells but the journal lacks, if any.
func (d *dateState) newDate(query string) string {
	date, ok := store.ParseDate(strings.TrimSpace(query))
	if !ok {
		return ""
	}
	for _, existing := range d.dates {
		if existing == date {
			return ""
		}
	}
	return date
}

func (a *App) refilterDates() {
	d := &a.dates
	query := strings.ToLower(strings.TrimSpace(d.search.Value()))

	d.matches = d.matches[:0]
	for i, date := range d.dates {
		if query == "" || a.dateMatches(date, query) {
			d.matches = append(d.matches, i)
		}
	}
	d.cursor = 0
	d.offset = 0
}

// dateMatches reports whether a date or anything logged under it contains the
// query. The dashed and compact spellings both match, so 2026-08 and 202608
// find the same days.
func (a *App) dateMatches(date, query string) bool {
	if strings.Contains(date, query) || strings.Contains(strings.ReplaceAll(date, "-", ""), query) {
		return true
	}
	j := a.journal()
	i := j.FindDay(date)
	if i < 0 {
		return false
	}
	for _, item := range j.Days[i].Items {
		if strings.Contains(strings.ToLower(item.Content), query) {
			return true
		}
	}
	return false
}

func (a *App) moveDate(delta int) {
	d := &a.dates
	if len(d.matches) == 0 {
		d.cursor = 0
		return
	}
	d.cursor = clamp(d.cursor+delta, 0, len(d.matches)-1)
	d.scrollSelectedIntoView(a.listHeight())
}

func (a *App) pageDate(direction int) { a.moveDate(direction * a.listHeight()) }

func (d *dateState) scrollSelectedIntoView(height int) {
	d.offset = clamp(d.offset, 0, d.cursor)
	if d.cursor >= d.offset+height {
		d.offset = d.cursor - height + 1
	}
}

func (a *App) viewDates() string {
	d := &a.dates

	// Calendar view
	if d.calMode {
		return lipgloss.JoinVertical(lipgloss.Left,
			a.viewCalendar(),
			divider(a.width),
			a.renderDateStatus(),
		)
	}

	// List view
	height := a.listHeight()

	rows := make([]string, 0, height)
	if len(d.matches) == 0 {
		rows = append(rows, a.emptyDateRow())
	}
	for i := d.offset; i < len(d.matches) && len(rows) < height; i++ {
		rows = append(rows, a.renderDateRow(i))
	}
	for len(rows) < height {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		spread(a.width, titleStyle.Render("\uf073 选择日期"), dimStyle.Render(a.dateCount())),
		divider(a.width),
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		divider(a.width),
		a.renderDateStatus(),
	)
}

func (a *App) dateCount() string {
	d := &a.dates
	if d.search.Value() != "" {
		return fmt.Sprintf("匹配 %d / %d 天", len(d.matches), len(d.dates))
	}
	return fmt.Sprintf("%d 天", len(d.dates))
}

func (a *App) emptyDateRow() string {
	hint := dimStyle.Render(strings.Repeat(" ", rowMargin) + "没有匹配的日期")
	return lipgloss.NewStyle().Width(a.width).Render(fit(hint, a.width))
}

func (a *App) renderDateRow(i int) string {
	d := &a.dates
	date := d.dates[d.matches[i]]
	selected := i == d.cursor

	// A date is read as plain text, so the only ground in the list belongs to the
	// row the cursor is on — and every stretch of that row has to be painted on
	// it, or the gaps would punch holes through the raised panel.
	ground := transparent
	if selected {
		ground = bg(pal.Row)
	}

	marker := "  "
	if selected {
		marker = "\uf0da "
	}

	dateFG, countFG := fg(pal.Text), fg(pal.Dim)
	if selected {
		dateFG, countFG = fg(pal.Selected), fg(pal.Title)
	}

	gap := run(" ", fg(pal.Text), ground)
	countText := cell(fmt.Sprintf("%3d 项", d.counts[date]), 6, lipgloss.Right, countFG, ground)
	if date == store.Today() {
		countText += gap + run("今天", fg(pal.Warn), ground)
	}

	row := run(strings.Repeat(" ", rowMargin), fg(pal.Text), ground) +
		run(marker, fg(pal.Warn), ground) +
		lipgloss.JoinHorizontal(lipgloss.Top,
			cell(date, 10, lipgloss.Left, dateFG, ground),
			gap,
			cell(store.Weekday(date), 4, lipgloss.Left, fg(pal.Dim), ground),
			gap,
			countText,
		)

	return cut(row, a.width)
}

func (a *App) renderDateStatus() string {
	d := &a.dates

	var hints string
	if d.calMode {
		hints = "日历 | h/l 前后天 | j/k 上下周 | H/L 上下月 | s 今天 | Tab 返回列表 | Enter 打开 | Esc 返回 | q 退出"
	} else {
		hints = "列表 | j/k 选择 | h/l 翻页 | / 搜索 | Tab 切换日历 | Enter 打开 | Esc 返回 | ? 帮助 | q 退出"
	}

	if d.searching {
		hints = "搜索 | " + d.search.View() + " | Enter 打开 | Esc 取消"
		if missing := d.newDate(d.search.Value()); missing != "" {
			hints = "搜索 | " + d.search.View() + " | " + missing + " 无记录，Enter 新建"
		}
	} else if a.status != "" {
		hints = a.status + " | " + hints
	}

	return statusStyle.Width(a.width).Render(fit(hints, a.width))
}

// parseDateParts extracts year, month, day from a date string like "2026-09-18"
func parseDateParts(date string) (year, month, day int) {
	if len(date) < 10 {
		return 2026, 1, 1
	}
	fmt.Sscanf(date, "%d-%d-%d", &year, &month, &day)
	return
}

// formatDate creates a date string from year, month, day
func formatDate(year, month, day int) string {
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// daysInMonth returns the number of days in a given month/year
func daysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	}
	return 30
}

// firstDayOfMonth returns the weekday (0=Sunday, 6=Saturday) of the first day of the month
func firstDayOfMonth(year, month int) int {
	// Using Zeller's formula simplified
	t := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return int(t.Weekday())
}

// monthName returns the Chinese name of a month
func monthName(month int) string {
	names := []string{"一月", "二月", "三月", "四月", "五月", "六月",
		"七月", "八月", "九月", "十月", "十一月", "十二月"}
	if month >= 1 && month <= 12 {
		return names[month-1]
	}
	return ""
}

// hasEntry checks if a date has any entries
func (d *dateState) hasEntry(date string) bool {
	return d.counts[date] > 0
}

// viewCalendar renders the calendar view
func (a *App) viewCalendar() string {
	d := &a.dates
	year, month := d.calYear, d.calMonth

	// Calendar width: 7 days * 3 chars (2 digits + 1 space) = 21 chars
	calWidth := 21
	calLeft := (a.width - calWidth) / 2
	if calLeft < 0 {
		calLeft = 0
	}
	leftPad := strings.Repeat(" ", calLeft)

	// Header with month/year - centered
	header := titleStyle.Render(fmt.Sprintf("\uf073 %d年 %s", year, monthName(month)))
	headerWidth := lipgloss.Width(header)
	headerLeft := (a.width - headerWidth) / 2
	if headerLeft < 0 {
		headerLeft = 0
	}
	titleRow := strings.Repeat(" ", headerLeft) + header

	// Weekday headers - centered
	weekdays := "日 一 二 三 四 五 六"
	weekdayRow := leftPad + dimStyle.Render(weekdays)

	// Calculate calendar grid
	firstDay := firstDayOfMonth(year, month)
	totalDays := daysInMonth(year, month)

	var rows []string
	rows = append(rows, titleRow)
	rows = append(rows, divider(a.width))
	rows = append(rows, weekdayRow)

	// Build weeks
	day := 1
	for week := 0; week < 6 && day <= totalDays; week++ {
		var weekLine strings.Builder
		weekLine.WriteString(leftPad)

		for dow := 0; dow < 7; dow++ {
			if (week == 0 && dow < firstDay) || day > totalDays {
				weekLine.WriteString("   ")
			} else {
				dateStr := formatDate(year, month, day)
				isToday := dateStr == store.Today()
				isSelected := dateStr == d.calCursor
				hasEntry := d.hasEntry(dateStr)

				var style lipgloss.Style
				var text string

				if isSelected {
					style = lipgloss.NewStyle().
						Background(bg(pal.Field)).
						Foreground(fg(pal.Ink)).
						Bold(true)
					text = fmt.Sprintf("%2d", day)
				} else if isToday {
					// Today: bold + underline
					style = lipgloss.NewStyle().
						Foreground(fg(pal.Warn)).
						Bold(true).
						Underline(true)
					text = fmt.Sprintf("%2d", day)
				} else if hasEntry {
					// Has entries: use combining dot below (U+0323)
					style = lipgloss.NewStyle().
						Foreground(fg(pal.Start))
					text = fmt.Sprintf("%2d\u0323", day)
				} else {
					style = lipgloss.NewStyle().
						Foreground(fg(pal.Text))
					text = fmt.Sprintf("%2d", day)
				}

				weekLine.WriteString(style.Render(text))
				weekLine.WriteString(" ")
				day++
			}
		}
		rows = append(rows, weekLine.String())
	}

	// Fill remaining rows to maintain height
	targetHeight := a.height - 4
	for len(rows) < targetHeight {
		rows = append(rows, "")
	}

	// Add info about selected date
	if d.calCursor != "" {
		count := d.counts[d.calCursor]
		weekday := store.Weekday(d.calCursor)
		info := fmt.Sprintf("%s %s | %d 项", d.calCursor, weekday, count)
		if d.calCursor == store.Today() {
			info += " (今天)"
		}
		rows = append(rows, divider(a.width))
		rows = append(rows, cell(info, a.width, lipgloss.Center, fg(pal.Dim), transparent))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// moveCalendar moves the calendar cursor by delta days
func (d *dateState) moveCalendar(deltaDays int) {
	t, _ := time.Parse("2006-01-02", d.calCursor)
	t = t.AddDate(0, 0, deltaDays)
	d.calCursor = t.Format("2006-01-02")
	d.calYear = t.Year()
	d.calMonth = int(t.Month())
}

// moveCalendarMonth moves the calendar by months
func (d *dateState) moveCalendarMonth(deltaMonths int) {
	t, _ := time.Parse("2006-01-02", d.calCursor)
	t = t.AddDate(0, deltaMonths, 0)
	d.calCursor = t.Format("2006-01-02")
	d.calYear = t.Year()
	d.calMonth = int(t.Month())
}
