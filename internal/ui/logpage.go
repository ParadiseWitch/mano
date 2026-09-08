package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mano/internal/keys"
	"mano/internal/store"
)

// mode is which of the four list states the log page is in.
type mode int

const (
	modeSelect mode = iota
	modeStart
	modeEnd
	modeEdit
)

// hourField and minuteField index the time component under adjustment.
const (
	hourField = iota
	minuteField
)

type logState struct {
	mode      mode
	cursor    int
	offset    int
	machine   keys.Machine
	field     int
	timeEntry keys.TimeEntry
	editor    textinput.Model
	clipboard *store.Item
}

func newLogState() logState {
	editor := textinput.New()
	editor.Prompt = ""
	return logState{timeEntry: keys.NewTimeEntry(), editor: editor}
}

func (a *App) updateLog(k tea.KeyMsg) tea.Cmd {
	a.status = ""

	switch a.log.mode {
	case modeEdit:
		return a.updateLogEdit(k)
	case modeStart, modeEnd:
		return a.updateLogTime(k)
	default:
		return a.updateLogSelect(k)
	}
}

func (a *App) updateLogSelect(k tea.KeyMsg) tea.Cmd {
	l := &a.log
	now := time.Now()

	switch ev := l.machine.Feed(k, now); ev.Kind {
	case keys.Nothing:
		return tick(ev.Wait)
	case keys.Jump:
		a.gotoItem(ev.Target - 1)
		return tick(ev.Wait)
	case keys.GoTop:
		a.gotoItem(0)
		return nil
	case keys.Delete:
		a.deleteItem()
		return nil
	case keys.Pass:
		return a.selectKey(ev.Key)
	}
	return nil
}

func (a *App) selectKey(k tea.KeyMsg) tea.Cmd {
	if r, ok := keys.SingleRune(k); ok {
		switch r {
		case 'j':
			a.moveItem(1)
		case 'k':
			a.moveItem(-1)
		case 'G':
			a.gotoItem(len(a.items()) - 1)
		case 'i':
			return a.enterEdit(true)
		case 'a':
			return a.enterEdit(false)
		case 'o':
			return a.insertBelow()
		case 's':
			return a.pickTime(modeStart)
		case 'e':
			return a.pickTime(modeEnd)
		case 'y':
			a.copyItem()
		case 'p':
			a.pasteItem()
		case 'c':
			a.openDates()
		case 'q':
			return tea.Quit
		case '?':
			a.page = pageHelp
		}
		return nil
	}

	switch k.Type {
	case tea.KeyDown:
		a.moveItem(1)
	case tea.KeyUp:
		a.moveItem(-1)
	case tea.KeyPgDown:
		a.moveItem(a.listHeight())
	case tea.KeyPgUp:
		a.moveItem(-a.listHeight())
	case tea.KeyHome:
		a.gotoItem(0)
	case tea.KeyEnd:
		a.gotoItem(len(a.items()) - 1)
	}
	return nil
}

func (a *App) updateLogEdit(k tea.KeyMsg) tea.Cmd {
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEsc, tea.KeyEnter:
		return a.commitEdit()
	}
	var cmd tea.Cmd
	a.log.editor, cmd = a.log.editor.Update(k)
	return cmd
}

func (a *App) updateLogTime(k tea.KeyMsg) tea.Cmd {
	l := &a.log
	slot := a.timeSlot()
	if slot == nil {
		l.mode = modeSelect
		return nil
	}

	now := time.Now()
	step := 1
	if l.field == minuteField {
		step = 5
	}

	if r, ok := keys.SingleRune(k); ok {
		// h/l/k/j adjust time only in the end-time mode, per the spec.
		if l.mode == modeEnd {
			switch r {
			case 'h':
				return a.selectField(hourField)
			case 'l':
				return a.selectField(minuteField)
			case 'k':
				a.bumpTime(slot, l.field, step)
				a.save()
				return nil
			case 'j':
				a.bumpTime(slot, l.field, -step)
				a.save()
				return nil
			}
		}
		if r >= '0' && r <= '9' {
			value, done := l.timeEntry.Digit(int(r-'0'), now)
			if !done {
				return tick(l.timeEntry.Pending(now))
			}
			a.setTimeField(slot, l.field, value)
			a.save()
			return nil
		}
	}

	switch k.Type {
	case tea.KeyLeft:
		return a.selectField(hourField)
	case tea.KeyRight:
		return a.selectField(minuteField)
	case tea.KeyUp:
		a.bumpTime(slot, l.field, step)
		a.save()
	case tea.KeyDown:
		a.bumpTime(slot, l.field, -step)
		a.save()
	case tea.KeyEsc:
		l.mode = modeSelect
		l.timeEntry.Reset()
	case tea.KeyCtrlC:
		return tea.Quit
	}
	return nil
}

// selectField moves between hour and minute, dropping any half-typed digit so
// it cannot land in the field the user just left.
func (a *App) selectField(field int) tea.Cmd {
	a.log.field = field
	a.log.timeEntry.Reset()
	return nil
}

// onTick expires pending key sequences and flushes a half-typed time digit.
func (a *App) onTick(now time.Time) tea.Cmd {
	if a.page != pageLog {
		return nil
	}
	l := &a.log

	switch l.mode {
	case modeStart, modeEnd:
		if value, ok := l.timeEntry.Flush(now); ok {
			if slot := a.timeSlot(); slot != nil {
				a.setTimeField(slot, l.field, value)
				a.save()
			}
			return nil
		}
		return tick(l.timeEntry.Pending(now))

	case modeSelect:
		return tick(l.machine.Pending(now))
	}
	return nil
}

// timeSlot is the start or end time being adjusted, or nil when there is none.
func (a *App) timeSlot() *store.Time {
	day := a.editableDay()
	i := a.log.cursor
	if i < 0 || i >= len(day.Items) {
		return nil
	}
	if a.log.mode == modeEnd {
		return day.Items[i].End
	}
	return day.Items[i].Start
}

func (a *App) setTimeField(slot *store.Time, field, value int) {
	if field == hourField {
		slot.Hour = clamp(value, 0, 23)
	} else {
		slot.Minute = clamp(value, 0, 59)
	}
}

func (a *App) bumpTime(slot *store.Time, field, delta int) {
	if field == hourField {
		slot.Hour = wrap(slot.Hour+delta, 24)
	} else {
		slot.Minute = wrap(slot.Minute+delta, 60)
	}
}

func (a *App) enterEdit(atStart bool) tea.Cmd {
	items := a.items()
	i := a.log.cursor
	if i < 0 || i >= len(items) {
		return nil
	}

	l := &a.log
	l.machine.Reset()
	l.editor.SetValue(items[i].Content)
	l.editor.Width = a.contentWidth()
	if atStart {
		l.editor.CursorStart()
	} else {
		l.editor.CursorEnd()
	}
	l.mode = modeEdit
	return l.editor.Focus()
}

func (a *App) commitEdit() tea.Cmd {
	l := &a.log
	l.editor.Blur()

	day := a.editableDay()
	if i := l.cursor; i >= 0 && i < len(day.Items) {
		day.Items[i].Content = l.editor.Value()
		a.save()
	}

	l.mode = modeSelect
	return nil
}

func (a *App) insertBelow() tea.Cmd {
	items := a.items()
	at := a.log.cursor + 1
	if len(items) == 0 {
		at = 0
	}
	if at > len(items) {
		at = len(items)
	}

	day := a.editableDay()
	now := store.NowTime()
	day.Items = append(day.Items, store.Item{})
	copy(day.Items[at+1:], day.Items[at:])
	day.Items[at] = store.Item{Start: &now}
	a.save()

	a.log.cursor = at
	a.scrollToCursor()
	return a.enterEdit(false)
}

func (a *App) deleteItem() {
	items := a.items()
	i := a.log.cursor
	if i < 0 || i >= len(items) {
		return
	}

	day := a.editableDay()
	day.Items = append(day.Items[:i], day.Items[i+1:]...)
	a.save()
	a.gotoItem(i)
}

func (a *App) copyItem() {
	items := a.items()
	i := a.log.cursor
	if i < 0 || i >= len(items) {
		return
	}
	clone := items[i].Clone()
	a.log.clipboard = &clone
	a.status = "已复制第 " + strconv.Itoa(i+1) + " 项"
}

func (a *App) pasteItem() {
	if a.log.clipboard == nil {
		a.status = "剪贴板为空"
		return
	}
	day := a.editableDay()
	day.Items = append(day.Items, a.log.clipboard.Clone())
	a.save()
	a.gotoItem(len(day.Items) - 1)
	a.status = "已粘贴到第 " + strconv.Itoa(len(day.Items)) + " 项"
}

func (a *App) pickTime(m mode) tea.Cmd {
	items := a.items()
	i := a.log.cursor
	if i < 0 || i >= len(items) {
		return nil
	}

	day := a.editableDay()
	item := &day.Items[i]
	slot := item.Start
	if m == modeEnd {
		slot = item.End
	}
	if slot == nil {
		now := store.NowTime()
		if m == modeEnd {
			item.End = &now
		} else {
			item.Start = &now
		}
		a.save()
	}

	a.log.mode = m
	a.log.machine.Reset()
	return a.selectField(hourField)
}

func (a *App) items() []store.Item {
	if day := a.day(); day != nil {
		return day.Items
	}
	return nil
}

func (a *App) gotoItem(i int) {
	if n := len(a.items()); n > 0 {
		a.log.cursor = clamp(i, 0, n-1)
	} else {
		a.log.cursor = 0
	}
	a.scrollToCursor()
}

func (a *App) moveItem(delta int) { a.gotoItem(a.log.cursor + delta) }

func (a *App) scrollToCursor() {
	height := a.listHeight()
	l := &a.log
	l.offset = clamp(l.offset, 0, l.cursor)
	if l.cursor >= l.offset+height {
		l.offset = l.cursor - height + 1
	}
}

// enterDay points the log page at a date, resetting selection and scroll.
func (a *App) enterDay(date string) {
	a.date = date
	a.page = pageLog
	a.log.mode = modeSelect
	a.log.machine.Reset()
	a.log.timeEntry.Reset()
	a.log.editor.Blur()
	a.log.cursor = 0
	a.log.offset = 0
}

func (a *App) listHeight() int {
	if h := a.height - 4; h > 1 {
		return h
	}
	return 1
}

func (a *App) contentWidth() int {
	if w := a.width - fixedWidth; w > 4 {
		return w
	}
	return 4
}

func (a *App) viewLog() string {
	items := a.items()
	height := a.listHeight()

	rows := make([]string, 0, height)
	if len(items) == 0 {
		rows = append(rows, a.emptyRow())
	}
	for i := a.log.offset; i < len(items) && len(rows) < height; i++ {
		rows = append(rows, a.renderRow(i, items[i]))
	}
	for len(rows) < height {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		a.renderTitle(),
		divider(a.width),
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		divider(a.width),
		a.renderStatus(),
	)
}

func (a *App) emptyRow() string {
	hint := dimStyle.Render(strings.Repeat(" ", rowMargin) + "今日暂无记录，按 o 新建")
	return lipgloss.NewStyle().Width(a.width).Render(fit(hint, a.width))
}

func (a *App) renderRow(i int, it store.Item) string {
	l := &a.log
	selected := i == l.cursor
	now := time.Now()

	numFG, numBG := fgDim, bgBlock
	if selected {
		numFG, numBG = fgBright, bgSelect
	}

	durText, durFG, durBG := "------", fgDim, bgBlock
	if dur, crossed := it.Duration(); it.Start != nil && it.End != nil {
		durText, durFG = store.FormatDuration(dur), fgText
		if crossed {
			durFG, durBG = fgBright, bgCrossed
		}
	}
	if selected && durBG == bgBlock {
		durFG, durBG = fgBright, bgSelect
	}

	row := strings.Repeat(" ", rowMargin) + lipgloss.JoinHorizontal(lipgloss.Top,
		cell(strconv.Itoa(i+1), colIndex, lipgloss.Right, numFG, numBG),
		" ",
		a.timeCell(it.Start, selected, l.mode == modeStart, now),
		" ",
		a.timeCell(it.End, selected, l.mode == modeEnd, now),
		" ",
		cell(durText, colDuration, lipgloss.Center, durFG, durBG),
		" ",
		a.contentCell(it, selected),
	)

	return lipgloss.NewStyle().MaxWidth(a.width).Render(row)
}

func (a *App) timeCell(t *store.Time, selected, active bool, now time.Time) string {
	fg, bg := fgText, bgBlock
	if t == nil {
		fg = fgDim
	}
	if selected {
		fg, bg = fgBright, bgSelect
	}
	if active {
		fg, bg = fgInk, bgField
	}

	text := "--:--"
	if t != nil {
		text = t.String()
		if active {
			if p := a.log.timeEntry.Provisional(now); p >= 0 {
				if a.log.field == hourField {
					text = fmt.Sprintf("%d_:%02d", p, t.Minute)
				} else {
					text = fmt.Sprintf("%02d:%d_", t.Hour, p)
				}
			}
		}
	}

	return cell(text, colTime, lipgloss.Center, fg, bg)
}

func (a *App) contentCell(it store.Item, selected bool) string {
	width := a.contentWidth()

	if selected && a.log.mode == modeEdit {
		return lipgloss.NewStyle().Width(width).Foreground(fgBright).Render(a.log.editor.View())
	}

	fg := fgText
	text := it.Content
	if text == "" {
		text, fg = "（无内容）", fgDim
	} else if selected {
		fg = fgBright
	}
	return textCell(text, width, fg)
}

func textCell(s string, width int, fg lipgloss.Color) string {
	return lipgloss.NewStyle().Width(width).Foreground(fg).Render(fit(s, width))
}

func (a *App) renderTitle() string {
	left := titleStyle.Render(a.date + " " + store.Weekday(a.date))

	right := ""
	if day := a.day(); day != nil && len(day.Items) > 0 {
		right = dimStyle.Render(fmt.Sprintf("%d 项 | %s",
			len(day.Items), store.FormatDuration(day.TotalDuration())))
	}

	return spread(a.width, left, right)
}

func (a *App) renderStatus() string {
	text := a.modeHints()

	if pending := a.pendingText(); pending != "" {
		text = "[" + pending + "] " + text
	}
	if a.status != "" {
		text = a.status + " | " + text
	}

	return statusStyle.Width(a.width).Render(fit(text, a.width))
}

func (a *App) modeHints() string {
	switch a.log.mode {
	case modeEdit:
		return "编辑 | Enter 或 Esc 完成"
	case modeStart:
		return "开始时间 | 左右键 选时分 | 上下键 调整 | 数字直填 | Esc 完成"
	case modeEnd:
		return "结束时间 | h/l 或 左右键 选时分 | k/j 或 上下键 调整 | 数字直填 | Esc 完成"
	default:
		return "选择 | j/k 移动 | i/a 编辑 | o 新建 | s/e 时间 | dd 删 | y/p 粘贴 | c 日期 | ? 帮助 | q 退出"
	}
}

func (a *App) pendingText() string {
	if a.log.mode != modeSelect {
		return ""
	}
	now := time.Now()
	if prefix := a.log.machine.Prefix(now); prefix != "" {
		return prefix
	}
	return a.log.machine.Count(now)
}
