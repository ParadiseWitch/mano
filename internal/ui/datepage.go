package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mano/internal/keys"
	"mano/internal/store"
)

type dateState struct {
	dates     []string // canonical, descending: today first
	counts    map[string]int
	matches   []int // indices into dates, narrowed by the search query
	cursor    int
	offset    int
	searching bool
	search    textinput.Model
}

func newDateState() dateState {
	search := textinput.New()
	search.Prompt = ""
	return dateState{counts: map[string]int{}, search: search}
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
}

func (a *App) updateDates(k tea.KeyMsg) tea.Cmd {
	a.status = ""
	d := &a.dates

	if d.searching {
		return a.updateDateSearch(k)
	}

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
			return nil
		case 'c':
			a.enterDay(a.date)
			return nil
		}
	}

	switch k.Type {
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
		spread(a.width, titleStyle.Render("选择日期"), dimStyle.Render(a.dateCount())),
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
		marker = "> "
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

	hints := "日期 | j/k 选择 | h/l 翻页 | / 搜索 | Enter 打开 | Esc 返回 | ? 帮助 | q 退出"
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
