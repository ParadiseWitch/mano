package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mano/internal/store"
)

type page int

const (
	pageLog page = iota
	pageDates
	pageHelp
)

// tickMsg drives the expiry of half-typed key sequences.
type tickMsg time.Time

func tick(after time.Duration) tea.Cmd {
	if after <= 0 {
		return nil
	}
	return tea.Tick(after, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// App is the root model. It owns the journal and routes input to one page.
type App struct {
	store  *store.Store
	page   page
	date   string
	status string

	width, height int

	log        logState
	dates      dateState
	helpOffset int
}

// New builds an app showing date on the log page.
func New(s *store.Store, date string) *App {
	return &App{
		store: s,
		page:  pageLog,
		date:  date,
		log:   newLogState(),
		dates: newDateState(),
	}
}

func (a *App) Init() tea.Cmd { return nil }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.log.editor.Width = a.contentWidth()
		a.dates.search.Width = a.searchWidth()
		return a, nil

	case tickMsg:
		return a, a.onTick(time.Time(msg))

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return a, tea.Quit
		}
		switch a.page {
		case pageLog:
			return a, a.updateLog(msg)
		case pageDates:
			return a, a.updateDates(msg)
		default:
			return a, a.updateHelp(msg)
		}
	}

	// Anything else — a cursor blink — belongs to whichever input has focus.
	if a.page == pageLog && a.log.mode == modeEdit {
		var cmd tea.Cmd
		a.log.editor, cmd = a.log.editor.Update(msg)
		return a, cmd
	}
	if a.page == pageDates && a.dates.searching {
		var cmd tea.Cmd
		a.dates.search, cmd = a.dates.search.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) View() string {
	if a.width == 0 {
		return "正在启动"
	}
	switch a.page {
	case pageDates:
		return a.viewDates()
	case pageHelp:
		return a.viewHelp()
	default:
		return a.viewLog()
	}
}

func (a *App) journal() *store.Journal { return &a.store.Journal }

// day is the day being viewed, or nil when the journal holds no entry for it.
func (a *App) day() *store.Day {
	j := a.journal()
	if i := j.FindDay(a.date); i >= 0 {
		return &j.Days[i]
	}
	return nil
}

// editableDay is the day being modified. It is created only here, on the first
// mutation, so that merely browsing a date never writes an empty header.
func (a *App) editableDay() *store.Day {
	j := a.journal()
	return &j.Days[j.EnsureDay(a.date)]
}

// save writes the journal out. Every mutation lands on disk immediately, so a
// closed terminal or a crash cannot cost the session's edits.
func (a *App) save() {
	if err := a.store.Save(); err != nil {
		a.status = "保存失败：" + err.Error()
	}
}

func (a *App) openDates() {
	a.dates.reset(a.store.Journal, a.date)
	a.dates.scrollSelectedIntoView(a.listHeight())
	a.page = pageDates
}

// searchWidth bounds the date-page search field, leaving room on the status
// line for the hints that follow it.
func (a *App) searchWidth() int {
	return clamp(a.width-50, 8, 16)
}
