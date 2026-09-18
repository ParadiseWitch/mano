package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"orgmaid/internal/keys"
	"orgmaid/internal/store"
)

// A row's cursor sits on one of eight stops. The three measured columns each
// split into an hour and a minute, so walking from the index to the content
// costs seven presses of Tab.
const (
	fIndex = iota
	fTodo
	fStartHour
	fStartMinute
	fEndHour
	fEndMinute
	fDurHour
	fDurMinute
	fContent
	stopCount
)

// stopNames labels every stop for the key-hint bar.
var stopNames = [stopCount]string{
	"序号", "待办", "开始 小时", "开始 分钟", "结束 小时", "结束 分钟",
	"耗时 小时", "耗时 分钟", "内容",
}

// hourStop reports whether a stop is the hour half of its column. The stops come
// in hour/minute pairs, so this is the one thing state needs to know about which
// stop is active.
func hourStop(field int) bool {
	return field == fStartHour || field == fEndHour || field == fDurHour
}

// timeStop reports whether a stop takes typed digits. The index, todo, and content
// leave digits to the jump machine instead.
func timeStop(field int) bool {
	return field > fTodo && field < fContent
}

type logState struct {
	field     int // stop the row cursor is on
	editing   bool
	cursor    int
	offset    int
	machine   keys.Machine
	editor    textinput.Model
	clipboard *store.Item

	// tensNext is whose turn the next typed digit is: the tens first, then the
	// ones, so two presses fill one reading.
	tensNext bool

	// Tag editing state
	tagEditing bool
	tagEditor  textinput.Model
}

// newLogState starts on the content stop, which is where a row is read from and
// where the row-level commands live.
func newLogState() logState {
	editor := textinput.New()
	editor.Prompt = ""
	tagEditor := textinput.New()
	tagEditor.Prompt = "标签: "
	return logState{field: fContent, tensNext: true, editor: editor, tagEditor: tagEditor}
}

func (a *App) updateLog(k tea.KeyMsg) tea.Cmd {
	a.status = ""

	switch {
	case a.log.tagEditing:
		return a.updateTagEdit(k)
	case a.log.editing:
		return a.updateLogEdit(k)
	case a.log.field != fContent:
		return a.updateLogStop(k)
	default:
		return a.updateLogContent(k)
	}
}

func (a *App) updateLogContent(k tea.KeyMsg) tea.Cmd {
	l := &a.log

	switch ev := l.machine.Feed(k, time.Now()); ev.Kind {
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
		return a.contentKey(ev.Key)
	}
	return nil
}

// contentKey handles a key on the content stop, where the list itself is moved
// about and every row-level command lives.
func (a *App) contentKey(k tea.KeyMsg) tea.Cmd {
	if r, ok := keys.SingleRune(k); ok {
		if a.rowKey(r) {
			return nil
		}
		if r == '0' {
			return a.selectStop(fIndex)
		}
		cmd, _ := a.rowCommand(r)
		return cmd
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
	case tea.KeyTab:
		return a.stepStop(1)
	case tea.KeyShiftTab:
		return a.stepStop(-1)
	case tea.KeyEnter:
		return a.enterEdit(false)
	}
	return nil
}

// rowKey handles the four keys that move the cursor or the row it stands on.
// They mean the same thing on every stop: j and k walk the cursor to the
// neighbouring item, which leaves the item itself where it was, while J and K
// carry the item the cursor is on up or down the list. Neither pair touches a
// reading, so no key here can change a clock.
func (a *App) rowKey(r rune) bool {
	switch r {
	case 'j':
		a.moveItem(1)
	case 'k':
		a.moveItem(-1)
	case 'J':
		a.reorderItem(1)
	case 'K':
		a.reorderItem(-1)
	default:
		return false
	}
	return true
}

// rowCommand handles the keys that act on the item under the cursor, wherever
// the cursor stands, and reports whether it took the key.
func (a *App) rowCommand(r rune) (tea.Cmd, bool) {
	switch r {
	case 'i':
		return a.enterEdit(true), true
	case 'a':
		return a.enterEdit(false), true
	case 'o':
		return a.insertBelow(), true
	case 'G':
		a.gotoItem(len(a.items()) - 1)
	case 'y':
		a.copyItem()
	case 'p':
		a.pasteItem()
	case 'c':
		a.openDates()
	case 't':
		a.cycleTodo()
	case 'T':
		a.openTodos()
	case ',':
		return a.editTags(), true
	case 'q':
		return tea.Quit, true
	case '?':
		a.page = pageHelp
		a.helpFrom = helpFromLog
	default:
		return nil, false
	}
	return nil, true
}

// updateLogStop is a stop other than content: the cursor sits on one place in the
// row, so a digit goes into the reading under it rather than to the jump machine.
func (a *App) updateLogStop(k tea.KeyMsg) tea.Cmd {
	l := &a.log

	// A digit belongs to the reading being typed, so it never reaches the jump
	// machine here. That includes 0, which is a perfectly good hour and minute.
	if timeStop(l.field) {
		if r, ok := keys.SingleRune(k); ok && r >= '0' && r <= '9' {
			return a.typeDigit(int(r - '0'))
		}
	}

	switch ev := l.machine.Feed(k, time.Now()); ev.Kind {
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
		return a.stopKey(ev.Key)
	}
	return nil
}

// stopKey handles a key on a stop other than the content one. The row commands
// mean the same thing here as there; what is left is the stop's own business:
// the arrows move the number under the cursor on a measured stop, and s writes
// the current clock into one of the two it can be written into.
func (a *App) stopKey(k tea.KeyMsg) tea.Cmd {
	if r, ok := keys.SingleRune(k); ok {
		if a.rowKey(r) {
			return nil
		}
		if r == 's' {
			return a.setNow()
		}
		cmd, _ := a.rowCommand(r)
		return cmd
	}

	measured := timeStop(a.log.field)
	switch k.Type {
	case tea.KeyUp:
		if a.log.field == fTodo {
			a.cycleTodo()
			return nil
		}
		if measured {
			return a.adjustStop(1)
		}
		a.moveItem(-1)
	case tea.KeyDown:
		if a.log.field == fTodo {
			a.cycleTodo()
			return nil
		}
		if measured {
			return a.adjustStop(-1)
		}
		a.moveItem(1)
	case tea.KeyTab:
		return a.stepStop(1)
	case tea.KeyShiftTab:
		return a.stepStop(-1)
	case tea.KeyEsc, tea.KeyEnter:
		return a.selectStop(fContent)
	}
	return nil
}

// stepStop walks along the row, wrapping at either end so the stops form a ring.
func (a *App) stepStop(delta int) tea.Cmd {
	return a.selectStop(wrap(a.log.field+delta, stopCount))
}

// selectStop parks on one stop, which restarts digit entry at the tens.
func (a *App) selectStop(field int) tea.Cmd {
	l := &a.log
	l.field = field
	l.machine.Reset()
	l.tensNext = true
	return nil
}

// adjustStop is up and down on a measured stop: the number the cursor stands on
// moves. direction is +1 for the up key, which each stop reads as its own kind
// of increase. Numbers wrap at their own ceiling rather than sticking, so holding
// a key sweeps the whole range.
func (a *App) adjustStop(direction int) tea.Cmd {
	item := a.focusItem()
	if item == nil {
		return nil
	}

	value, limit, step := a.stopReading(item)
	if !a.putStop(item, wrap(value+direction*step, limit+1)) {
		a.status = "耗时由起止时间算出，先填开始时间"
		return nil
	}
	a.save()
	return nil
}

// stopReading is the number the cursor stands on: what it holds, the most it may
// hold, and how far one press of up or down moves it.
func (a *App) stopReading(item *store.Item) (value, limit, step int) {
	limit, step = 59, 5
	if hourStop(a.log.field) {
		limit, step = 23, 1
	}
	hour := hourStop(a.log.field)

	switch a.log.field {
	case fStartHour, fStartMinute, fEndHour, fEndMinute:
		slot := a.clockSlot(item)
		if hour {
			return slot.Hour, limit, step
		}
		return slot.Minute, limit, step
	case fDurHour, fDurMinute:
		hours, minutes := splitDuration(item)
		if hour {
			return hours, limit, step
		}
		return minutes, limit, step
	}
	return 0, limit, step
}

// putStop writes a number back where it came from, and reports whether there was
// anywhere to write it. A span has no number of its own, so its share of the
// reading lands on the end time — which needs a start time to measure from.
func (a *App) putStop(item *store.Item, value int) bool {
	hour := hourStop(a.log.field)

	switch a.log.field {
	case fStartHour, fStartMinute, fEndHour, fEndMinute:
		slot := a.clockSlot(item)
		if hour {
			slot.Hour = value
		} else {
			slot.Minute = value
		}
	case fDurHour, fDurMinute:
		hours, minutes := splitDuration(item)
		if hour {
			hours = value
		} else {
			minutes = value
		}
		return item.AddDuration(hours*60 + minutes)
	}
	return true
}

// clockSlot is the reading a start or end stop writes to. An empty one is
// created at 00:00, which is what the cursor then steps away from.
func (a *App) clockSlot(item *store.Item) *store.Time {
	if a.log.field == fEndHour || a.log.field == fEndMinute {
		if item.End == nil {
			item.End = &store.Time{}
		}
		return item.End
	}
	if item.Start == nil {
		item.Start = &store.Time{}
	}
	return item.Start
}

// splitDuration is the item's span in whole hours and minutes, zero when it has
// no span yet — an unset duration is where editing starts.
func splitDuration(item *store.Item) (hours, minutes int) {
	dur, _ := item.Duration()
	total := int(dur / time.Minute)
	return total / 60, total % 60
}

// typeDigit writes one numeral into the reading under the cursor. The tens go
// first and then the ones, and an accepted press hands the turn to the other
// half: two presses fill one reading, a third starts the next. A press whose
// number would fall outside the clock — 25 时, 69 分 — is dropped whole, the turn
// included, so a refused digit cannot be waiting in the other half.
func (a *App) typeDigit(d int) tea.Cmd {
	l := &a.log
	item := a.focusItem()
	if item == nil {
		return nil
	}

	value, limit, _ := a.stopReading(item)
	next := value/10*10 + d
	if l.tensNext {
		next = d*10 + value%10
	}
	if next > limit {
		a.status = fmt.Sprintf("%s 最大 %d", stopNames[l.field], limit)
		return nil
	}
	if !a.putStop(item, next) {
		a.status = "耗时由起止时间算出，先填开始时间"
		return nil
	}

	l.tensNext = !l.tensNext
	a.save()
	return nil
}

// setNow is the s key: the clock reading written as it is, so logging a span
// that starts now costs one keystroke. The span stops have no reading of their
// own, so the key is inert there.
func (a *App) setNow() tea.Cmd {
	l := &a.log
	item := a.focusItem()
	if item == nil {
		return nil
	}

	switch l.field {
	case fStartHour, fStartMinute:
		now := store.NowTime()
		item.Start = &now
	case fEndHour, fEndMinute:
		now := store.NowTime()
		item.End = &now
	default:
		return nil
	}

	l.tensNext = true
	a.save()
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

func (a *App) updateTagEdit(k tea.KeyMsg) tea.Cmd {
	switch k.Type {
	case tea.KeyCtrlC:
		return tea.Quit
	case tea.KeyEsc, tea.KeyEnter:
		a.commitTagEdit()
		return nil
	}
	var cmd tea.Cmd
	a.log.tagEditor, cmd = a.log.tagEditor.Update(k)
	return cmd
}

// onTick expires a half-typed gg or dd.
func (a *App) onTick(now time.Time) tea.Cmd {
	if a.page != pageLog || a.log.editing {
		return nil
	}
	return tick(a.log.machine.Pending(now))
}

func (a *App) enterEdit(atStart bool) tea.Cmd {
	items := a.items()
	i := a.log.cursor
	if i < 0 || i >= len(items) {
		return nil
	}

	l := &a.log
	l.machine.Reset()
	l.tensNext = true
	l.editor.SetValue(items[i].Content)
	l.editor.Width = a.contentWidth()
	if atStart {
		l.editor.CursorStart()
	} else {
		l.editor.CursorEnd()
	}
	l.field = fContent
	l.editing = true
	return l.editor.Focus()
}

func (a *App) commitEdit() tea.Cmd {
	l := &a.log
	l.editor.Blur()

	if item := a.focusItem(); item != nil {
		item.Content = l.editor.Value()
		a.save()
	}

	l.editing = false
	l.field = fContent
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

	// The new item starts now, so the one above it — the task logged until this
	// moment — stops now too, if it never got an end of its own. It gets its own
	// copy of the reading: one *store.Time shared by two items would let editing
	// either of them move the other.
	if at > 0 {
		if prev := &day.Items[at-1]; prev.Start != nil && prev.End == nil {
			stop := now
			prev.End = &stop
		}
	}

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

// reorderItem swaps the selected item with its neighbour and follows it, so an
// item being moved stays under the cursor the whole way. delta counts toward the
// end of the list: +1 moves the item down the screen.
func (a *App) reorderItem(delta int) {
	items := a.items()
	i := a.log.cursor
	j := i + delta
	if i < 0 || j < 0 || j >= len(items) {
		return
	}

	day := a.editableDay()
	day.Items[i], day.Items[j] = day.Items[j], day.Items[i]
	a.save()
	a.gotoItem(j)
}

func (a *App) copyItem() {
	items := a.items()
	i := a.log.cursor
	if i < 0 || i >= len(items) {
		return
	}
	clone := items[i].Clone()
	a.log.clipboard = &clone

	// Copy content to system clipboard
	if err := clipboard.WriteAll(clone.Content); err != nil {
		a.status = "已复制第 " + strconv.Itoa(i+1) + " 项（剪贴板写入失败）"
	} else {
		a.status = "已复制第 " + strconv.Itoa(i+1) + " 项到剪贴板"
	}
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

// cycleTodo cycles the TODO state of the current item: "" → "TODO" → "DONE" → "".
func (a *App) cycleTodo() {
	item := a.focusItem()
	if item == nil {
		return
	}
	switch item.Todo {
	case "":
		item.Todo = "TODO"
		a.status = "标记为待办"
	case "TODO":
		item.Todo = "DONE"
		a.status = "标记为完成"
	case "DONE":
		item.Todo = ""
		a.status = "取消标记"
	}
	a.save()
}

// editTags opens the tag editor for the current item.
func (a *App) editTags() tea.Cmd {
	item := a.focusItem()
	if item == nil {
		return nil
	}
	l := &a.log
	l.tagEditing = true
	l.tagEditor.SetValue(strings.Join(item.Tags, " "))
	l.tagEditor.Width = a.width - 10
	l.tagEditor.CursorEnd()
	return l.tagEditor.Focus()
}

// commitTagEdit saves the tags from the editor.
func (a *App) commitTagEdit() {
	l := &a.log
	l.tagEditor.Blur()
	if item := a.focusItem(); item != nil {
		raw := strings.Fields(l.tagEditor.Value())
		seen := map[string]bool{}
		var tags []string
		for _, t := range raw {
			if !seen[t] {
				seen[t] = true
				tags = append(tags, t)
			}
		}
		item.Tags = tags
		a.save()
	}
	l.tagEditing = false
}

// focusItem is the item the stops read and write, or nil when the day is empty.
func (a *App) focusItem() *store.Item {
	day := a.editableDay()
	i := a.log.cursor
	if i < 0 || i >= len(day.Items) {
		return nil
	}
	return &day.Items[i]
}

func (a *App) items() []store.Item {
	if day := a.day(); day != nil {
		return day.Items
	}
	return nil
}

// gotoItem moves the row cursor. A fill cycle belongs to the reading it started
// on, so landing on another row starts the next digit at the tens again.
func (a *App) gotoItem(i int) {
	if n := len(a.items()); n > 0 {
		a.log.cursor = clamp(i, 0, n-1)
	} else {
		a.log.cursor = 0
	}
	a.log.tensNext = true
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

// enterDay points the log page at a date, resetting the cursor and where it
// stands on the row.
func (a *App) enterDay(date string) {
	a.date = date
	a.page = pageLog
	a.log.field = fContent
	a.log.editing = false
	a.log.machine.Reset()
	a.log.tensNext = true
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

// contentWidth is the one column that stretches with the terminal. Its floor is
// a single cell so the row still lands exactly on the right edge: a row built
// wider than the terminal has to be cut, and a cut that lands in the middle of a
// wide glyph leaves a cell the canvas then fills with the program background.
func (a *App) contentWidth() int {
	if w := a.width - fixedWidth; w > 1 {
		return w
	}
	return 1
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
	hint := "\uf466 今日暂无记录，按 o 新建"
	lead := clamp(a.width-lipgloss.Width(hint), colMark, fixedWidth)
	return cell(strings.Repeat(" ", lead)+hint, a.width, lipgloss.Left, fg(pal.Dim), transparent)
}

// noDuration fills the duration column of an item that has no span yet. It is
// as wide as store.FormatDuration output, so the column keeps its shape.
const noDuration = "--h--m"

func (a *App) renderRow(i int, it store.Item) string {
	l := &a.log
	selected := i == l.cursor

	// Only the selected row has a stop under the cursor, and the editor owns the
	// screen while it is open.
	focus := -1
	if selected && !l.editing {
		focus = l.field
	}

	// A row is a line of readings in their own accents rather than a stack of
	// bands, so the only ground it carries is the one the selected row sits on.
	ground := transparent
	if selected {
		ground = bg(pal.Row)
	}

	mark := " "
	if selected {
		mark = "\uf0da"
	}

	row := cell(mark, colMark, lipgloss.Left, fg(pal.Warn), ground) +
		lipgloss.JoinHorizontal(lipgloss.Top,
			a.indexCell(i+1, ground, focus == fIndex),
			a.todoCell(it.Todo, ground, focus == fTodo),
			a.clockCell(it.Start, fg(pal.Start), ground, focus == fStartHour, focus == fStartMinute, " -")+
				a.clockCell(it.End, fg(pal.End), ground, focus == fEndHour, focus == fEndMinute, "  "),
			a.durationCell(it, ground, focus == fDurHour, focus == fDurMinute),
			a.contentCell(it, ground, selected, selected && l.editing),
		)

	return cut(row, a.width)
}

// indexCell is the item number. It is the one stop that takes the cursor's block
// whole, having no hour and minute to split it between.
func (a *App) indexCell(n int, ground lipgloss.TerminalColor, active bool) string {
	return cell(strconv.Itoa(n)+".", colIndex, lipgloss.Right,
		choose(active, fg(pal.Ink), fg(pal.Index)),
		choose(active, bg(pal.Field), ground))
}

// todoCell shows the TODO/DONE status in its own column.
func (a *App) todoCell(todo string, ground lipgloss.TerminalColor, active bool) string {
	text := ""
	col := fg(pal.Dim)
	var todoBg lipgloss.TerminalColor = ground
	switch todo {
	case "TODO":
		text = "TODO"
		col = fg(pal.Ink)
		if active {
			todoBg = bg(pal.Start)
		} else {
			todoBg = bg("#5b8abf")
		}
	case "DONE":
		text = "DONE"
		col = fg(pal.Ink)
		if active {
			todoBg = bg(pal.Crossed)
		} else {
			todoBg = bg("#6aaa7a")
		}
	default:
		return cell("      ", colTodo, lipgloss.Center, col, ground)
	}
	// Render the text with background, then pad to column width
	styled := lipgloss.NewStyle().
		Foreground(col).
		Background(todoBg).
		Render(text)
	return cell(styled, colTodo, lipgloss.Center, fg(pal.Text), ground)
}

// clockCell is one of the two time columns: the reading in its own accent with a
// cell of padding either side, and the half the cursor stands on lifted onto the
// block. An unset clock is a placeholder rather than a reading, so it takes the
// secondary colour; the block still lands on the half the cursor is on, because
// an empty clock is as fillable as a full one.
func (a *App) clockCell(t *store.Time, accent, ground lipgloss.TerminalColor, hourActive, minuteActive bool, trail string) string {
	ink := accent
	if t == nil {
		ink = fg(pal.Dim)
	}

	hour, sep, minute := "--", ":", "--"
	if t != nil {
		text := t.String()
		hour, sep, minute = text[0:2], text[2:3], text[3:5]
	}

	return run(" ", ink, ground) +
		run(hour, choose(hourActive, fg(pal.Ink), ink), choose(hourActive, bg(pal.Field), ground)) +
		run(sep, ink, ground) +
		run(minute, choose(minuteActive, fg(pal.Ink), ink), choose(minuteActive, bg(pal.Field), ground)) +
		run(trail, ink, ground)
}

// durationCell is the span column, editable through the end time standing behind
// it. The hour and the minute are separate stops, as in the clock columns, and a
// span that runs past midnight takes the accent that says so.
func (a *App) durationCell(it store.Item, ground lipgloss.TerminalColor, hourActive, minuteActive bool) string {
	ink, text := fg(pal.Duration), noDuration
	if it.Start != nil && it.End != nil {
		dur, crossed := it.Duration()
		text = store.FormatDuration(dur)
		if crossed {
			ink = fg(pal.Crossed)
		}
	} else {
		ink = fg(pal.Dim)
	}

	hour, unit, minute := text[0:2], text[2:3], text[3:5]

	return run(" ", ink, ground) +
		run(hour, choose(hourActive, fg(pal.Ink), ink), choose(hourActive, bg(pal.Field), ground)) +
		run(unit, ink, ground) +
		run(minute, choose(minuteActive, fg(pal.Ink), ink), choose(minuteActive, bg(pal.Field), ground)) +
		run(text[5:6], ink, ground) +
		run(" ", ink, ground)
}

// contentCell is the row's own text, and the reason the other columns keep their
// accents quiet: it is what the day is read for. A row the cursor is on lifts it
// to the brighter of the two text colours, the editor takes over while it is
// open, and a row with nothing logged says so in the secondary colour.
func (a *App) contentCell(it store.Item, ground lipgloss.TerminalColor, selected, editing bool) string {
	width := a.contentWidth()

	col := fg(pal.Text)
	switch {
	case editing:
		col = fg(pal.Editing)
	case it.Content == "":
		col = fg(pal.Dim)
	case selected:
		col = fg(pal.Selected)
	}

	if editing {
		return lipgloss.NewStyle().
			Width(width).
			Foreground(col).
			Background(ground).
			Render(a.log.editor.View())
	}

	text := it.Content
	if text == "" {
		text = "（无内容）"
	}

	// Append tags in dim color
	if len(it.Tags) > 0 {
		tagStr := "  :" + strings.Join(it.Tags, ":") + ":"
		text += tagStr
	}

	return cell(text, width, lipgloss.Left, col, ground)
}

func (a *App) renderTitle() string {
	left := titleStyle.Render("\uf073 ") + titleStyle.Render(a.date+" "+store.Weekday(a.date))

	right := ""
	if day := a.day(); day != nil && len(day.Items) > 0 {
		right = dimStyle.Render(fmt.Sprintf("%d 项 | %s",
			len(day.Items), store.FormatDuration(day.TotalDuration())))
	}

	return spread(a.width, left, right)
}

func (a *App) renderStatus() string {
	text := a.stopHints()

	if a.log.tagEditing {
		text = "\uf02c 标签编辑 | " + a.log.tagEditor.View() + " | Enter 或 Esc 保存"
	} else if pending := a.pendingText(); pending != "" {
		text = "[" + pending + "] " + text
	} else if a.status != "" {
		text = a.status + " | " + text
	}

	return statusStyle.Width(a.width).Render(fit(text, a.width))
}

// stopHints says what the keys do where the cursor currently stands, which is
// the one thing a row's eight stops disagree about. Each one has to fit an
// 80-column terminal: a hint cut at the edge loses the key the reader was
// reaching for, so the four row keys live on the two stops that have room for
// them and every bar ends with the way out.
func (a *App) stopHints() string {
	l := &a.log

	if l.editing {
		return "\uf044 编辑 | Enter 或 Esc 完成"
	}

	name := stopNames[l.field]
	switch l.field {
	case fContent:
		return "\uf044 内容 j/k 换项 J/K 挪 i 编辑 o 新建 t 待办 T 全局 , 标签 c 日期 q 退出"
	case fIndex:
		return "\uf0cb 序号 j/k 换项 J/K 挪本项 Tab 换列 ? 帮助 Esc 回内容"
	case fTodo:
		return "\uf0ae 待办 ↑/↓ 切换状态 Tab 换列 Esc 回内容"
	}

	// The clock stops take one half of a reading at a time and can be filled from
	// the clock; the span stops own no reading at all, so a digit there lands on
	// the end time and s has nothing to fill.
	step, digits := "每次 5", "十位→个位 s 现在"
	if hourStop(l.field) {
		step = "每次 1"
	}
	if l.field == fDurHour || l.field == fDurMinute {
		digits = "写回结束时间"
	}

	return name + " | ↑ 加 ↓ 减 " + step + " 循环 | 数字 " + digits +
		" | Tab 换列 Esc 回内容"
}

func (a *App) pendingText() string {
	l := &a.log
	if l.editing {
		return ""
	}
	now := time.Now()

	if prefix := l.machine.Prefix(now); prefix != "" {
		return prefix
	}
	return l.machine.Count(now)
}
