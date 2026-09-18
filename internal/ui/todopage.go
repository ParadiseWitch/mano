package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"orgmaid/internal/keys"
	"orgmaid/internal/store"
)

type todoEntry struct {
	date  string
	index int
	item  *store.Item
}

type todoState struct {
	entries []todoEntry
	cursor  int
	offset  int
}

func (a *App) collectTodos() []todoEntry {
	var entries []todoEntry
	j := a.journal()
	for _, day := range j.Days {
		for i := range day.Items {
			if day.Items[i].Todo == "TODO" {
				entries = append(entries, todoEntry{
					date:  day.Date,
					index: i,
					item:  &day.Items[i],
				})
			}
		}
	}
	return entries
}

func (a *App) openTodos() {
	a.todos.entries = a.collectTodos()
	a.todos.cursor = 0
	a.todos.offset = 0
	a.page = pageTodos
}

func (a *App) updateTodos(k tea.KeyMsg) tea.Cmd {
	a.status = ""
	t := &a.todos

	if r, ok := keys.SingleRune(k); ok {
		switch r {
		case 'j':
			if len(t.entries) > 0 {
				t.cursor = clamp(t.cursor+1, 0, len(t.entries)-1)
			}
			return nil
		case 'k':
			if len(t.entries) > 0 {
				t.cursor = clamp(t.cursor-1, 0, len(t.entries)-1)
			}
			return nil
		case 't':
			if len(t.entries) > 0 && t.cursor < len(t.entries) {
				entry := t.entries[t.cursor]
				switch entry.item.Todo {
				case "TODO":
					entry.item.Todo = "DONE"
					a.status = "标记为完成"
				case "DONE":
					entry.item.Todo = ""
					a.status = "取消标记"
				}
				a.save()
				// Refresh the list
				a.todos.entries = a.collectTodos()
				if t.cursor >= len(t.entries) {
					t.cursor = len(t.entries) - 1
				}
			}
			return nil
		case 'q', '?':
			a.page = pageLog
			return nil
		}
	}

	switch k.Type {
	case tea.KeyDown:
		if len(t.entries) > 0 {
			t.cursor = clamp(t.cursor+1, 0, len(t.entries)-1)
		}
	case tea.KeyUp:
		if len(t.entries) > 0 {
			t.cursor = clamp(t.cursor-1, 0, len(t.entries)-1)
		}
	case tea.KeyEnter:
		if len(t.entries) > 0 && t.cursor < len(t.entries) {
			entry := t.entries[t.cursor]
			a.date = entry.date
			a.log.cursor = entry.index
			a.page = pageLog
		}
	case tea.KeyEsc:
		a.page = pageLog
	case tea.KeyCtrlC:
		return tea.Quit
	}
	return nil
}

func (a *App) viewTodos() string {
	t := &a.todos
	height := a.listHeight()

	rows := make([]string, 0, height)
	if len(t.entries) == 0 {
		hint := dimStyle.Render(strings.Repeat(" ", rowMargin) + "没有待办事项")
		rows = append(rows, lipgloss.NewStyle().Width(a.width).Render(fit(hint, a.width)))
	} else {
		for i := t.offset; i < len(t.entries) && len(rows) < height; i++ {
			rows = append(rows, a.renderTodoRow(i, t.entries[i]))
		}
	}
	for len(rows) < height {
		rows = append(rows, "")
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		spread(a.width, titleStyle.Render("\uf0ae 待办事项"), dimStyle.Render(fmt.Sprintf("%d 项", len(t.entries)))),
		divider(a.width),
		lipgloss.JoinVertical(lipgloss.Left, rows...),
		divider(a.width),
		statusStyle.Width(a.width).Render(fit("待办 | j/k 选择 | t 完成 | Enter 跳转 | Esc 返回 | q 退出", a.width)),
	)
}

func (a *App) renderTodoRow(i int, entry todoEntry) string {
	selected := i == a.todos.cursor

	ground := transparent
	if selected {
		ground = bg(pal.Row)
	}

	marker := "  "
	if selected {
		marker = "\uf0da "
	}

	dateFG := fg(pal.Text)
	contentFG := fg(pal.Selected)
	if !selected {
		dateFG = fg(pal.Dim)
		contentFG = fg(pal.Text)
	}

	content := entry.item.Content
	if content == "" {
		content = "（无内容）"
	}
	if len(entry.item.Tags) > 0 {
		content += "  :" + strings.Join(entry.item.Tags, ":") + ":"
	}

	row := run(strings.Repeat(" ", rowMargin), fg(pal.Text), ground) +
		run(marker, fg(pal.Warn), ground) +
		lipgloss.JoinHorizontal(lipgloss.Top,
			cell(entry.date, 10, lipgloss.Left, dateFG, ground),
			run(" ", fg(pal.Text), ground),
			cell(content, a.width-rowMargin-2-10-1, lipgloss.Left, contentFG, ground),
		)

	return cut(row, a.width)
}
